package world

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/content"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

// PhaserTile represents a single tile in the game
type PhaserTile struct {
	ID            int     `json:"id"`
	X             int     `json:"x"`
	Y             int     `json:"y"`
	TileImageID   int     `json:"tileImageId"`
	LocalX        *int    `json:"localX,omitempty"`
	LocalY        *int    `json:"localY,omitempty"`
	MapID         int     `json:"mapId"`
	SourceMapID   *int    `json:"sourceMapId,omitempty"`
	SourceMapName *string `json:"sourceMapName,omitempty"`
	CollisionType int     `json:"collisionType"`
	RawFootTileID *int    `json:"rawFootTileId,omitempty"`
	TalkOverTile  bool    `json:"talkOverTile"`
	// CoordinateOrigin describes whether this square existed in the extracted
	// game data. ContentOrigin describes who supplied its current appearance.
	// Keeping both prevents a user edit from erasing native provenance.
	IsNativeGameData bool   `json:"isNativeGameData"`
	CoordinateOrigin string `json:"coordinateOrigin"`
	ContentOrigin    string `json:"contentOrigin"`
}

// PhaserActor represents an actor or object in the game
type PhaserActor struct {
	ID                int     `json:"id"`
	InternalID        int     `json:"internalId,omitempty"` // For players, this is CharacterID
	X                 *int    `json:"x,omitempty"`
	Y                 *int    `json:"y,omitempty"`
	MapID             int     `json:"mapId"`
	ObjectType        string  `json:"objectType"`
	SpriteName        *string `json:"spriteName,omitempty"`
	Name              *string `json:"name,omitempty"`
	ActionType        *string `json:"actionType,omitempty"`
	ActionDirection   *string `json:"actionDirection,omitempty"`
	Frame             *int    `json:"frame,omitempty"`
	FlipX             *bool   `json:"flipX,omitempty"`
	MoveSpeed         int     `json:"moveSpeed"` // milliseconds per tile (default 200 for walking)
	MovementType      *string `json:"movementType,omitempty"`
	Text              *string `json:"text,omitempty"`              // TEXT_ constant for dialogue resolution
	TrainerClass      *string `json:"trainerClass,omitempty"`      // Trainer class constant (e.g. "BUG_CATCHER")
	TrainerPartyIndex *int    `json:"trainerPartyIndex,omitempty"` // Party index within trainer class
	ItemID            *int    `json:"itemId,omitempty"`            // For item objects: the item template ID
	MovementSeq       *int    `json:"movementSeq,omitempty"`       // Server-driven movement step sequence
	DbID              int     `json:"-"`                           // Original database ID (not sent to client)
}

func playerSpriteName(gender uint8, ridingBicycle bool, surfing bool) string {
	if surfing {
		return "SPRITE_RED_SURF"
	}
	if ridingBicycle {
		return "SPRITE_RED_BIKE"
	}
	switch gender {
	case 1:
		return "SPRITE_BEAUTY"
	case 2:
		return "SPRITE_BLUENB"
	default:
		return "SPRITE_BLUE"
	}
}

// PhaserWarp represents a warp point between maps
type PhaserWarp struct {
	ID                int     `json:"id"`
	SourceMapID       int     `json:"sourceMapId"`
	X                 int     `json:"x"`
	Y                 int     `json:"y"`
	DestinationMapID  *int    `json:"destinationMapId,omitempty"`
	DestinationMap    *string `json:"destinationMap,omitempty"`
	DestinationX      *int    `json:"destinationX,omitempty"`
	DestinationY      *int    `json:"destinationY,omitempty"`
	DestinationKind   string  `json:"destinationKind"`
	DestinationWarpID int     `json:"destinationWarpId"`
	WarpType          string  `json:"warpType"`
	WarpDirection     *string `json:"warpDirection,omitempty"`
}

// PhaserTilesRequest is the request payload
type PhaserTilesRequest struct {
	MapID     int    `json:"mapId"`
	RequestID string `json:"requestId,omitempty"`
	MinX      *int   `json:"minX,omitempty"`
	MinY      *int   `json:"minY,omitempty"`
	MaxX      *int   `json:"maxX,omitempty"`
	MaxY      *int   `json:"maxY,omitempty"`
	AfterID   *int   `json:"afterId,omitempty"`
	Limit     *int   `json:"limit,omitempty"`
}

type PhaserTilesResponse struct {
	MapID       int          `json:"mapId"`
	RequestID   string       `json:"requestId"`
	Tiles       []PhaserTile `json:"tiles"`
	NextAfterID int          `json:"nextAfterId"`
	HasMore     bool         `json:"hasMore"`
	Error       string       `json:"error,omitempty"`
}

// PhaserActorsRequest is the request payload
type PhaserActorsRequest struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	MapID       int    `json:"mapId"`
}
type PhaserActorsResponse struct {
	Success     bool          `json:"success" tstype:"true"`
	RequestID   string        `json:"requestId"`
	CharacterID int64         `json:"characterId"`
	MapID       int           `json:"mapId"`
	Actors      []PhaserActor `json:"actors"`
}

// PhaserWarpsRequest is the request payload
type PhaserWarpsRequest struct {
	RequestID string `json:"requestId"`
	MapID     int    `json:"mapId"`
}

// HandlePhaserMapInfoRequest only reads catalog metadata. Legacy destination
// fields are rejected so an old caller cannot mistake a read for an arrival.
func HandlePhaserMapInfoRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserMapInfoRequest
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	var extra any
	if err == nil && decoder.Decode(&extra) != io.EOF {
		err = fmt.Errorf("metadata request must contain one JSON object")
	}
	if err != nil || req.RequestID == "" || len(req.RequestID) > 64 {
		ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Invalid map metadata request."}, opcodes.PhaserMapInfoResponse)
		return false
	}
	info, err := loadRuntimeMapInfo(ses.CommandContext(), wh.Content, req.MapID)
	if err != nil {
		log.Printf("[Phaser] Error querying map info for %d: %v", req.MapID, err)
		ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Could not load map information."}, opcodes.PhaserMapInfoResponse)
		return false
	}
	ses.SendStreamJSON(protocol.PhaserMapInfoResponse{PhaserMapInfo: info, Success: true, RequestID: req.RequestID}, opcodes.PhaserMapInfoResponse)
	return false
}

func loadRuntimeMapInfo(ctx context.Context, service *content.Service, mapID int) (protocol.PhaserMapInfo, error) {
	if mapID != UnifiedOverworldMapID {
		return service.MapInfo(ctx, mapID)
	}
	info, err := service.OverworldInfo(ctx)
	info.ID = UnifiedOverworldMapID
	return info, err
}

// HandlePhaserMapLoadRequest owns arrival/recovery/load effects separately from metadata.
func HandlePhaserMapLoadRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserMapLoadRequest
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	var extra any
	if err == nil && decoder.Decode(&extra) != io.EOF {
		err = fmt.Errorf("map load request must contain one JSON object")
	}
	if err != nil || req.RequestID == "" || len(req.RequestID) > 64 {
		ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Invalid map load request."}, opcodes.PhaserMapLoadResponse)
		return false
	}
	mapInfo, err := loadRuntimeMapInfo(ses.CommandContext(), wh.Content, req.MapID)
	if err != nil {
		log.Printf("[Phaser] Error querying map info for %d: %v", req.MapID, err)
		ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Could not load map information."}, opcodes.PhaserMapLoadResponse)
		return false
	}

	char := ses.Client.CharData()
	charID := int64(char.ID)
	normalizedID := normalizedVisiblePlayerMapID(wh, mapInfo.ID)
	x, y, ownedMapID := wh.ownedPlayerPosition(ses)
	previousMapID := normalizedVisiblePlayerMapID(wh, ownedMapID)
	if normalizedID != previousMapID {
		ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Map loading requires the current player location."}, opcodes.PhaserMapLoadResponse)
		return false
	}
	teleport := false
	direction := "DOWN"
	if isInvalidZeroPlayerPosition(x, y) {
		// Recovery uses the actual recovery map's effects, never those of a stale view.
		mapInfo, err = loadRuntimeMapInfo(ses.CommandContext(), wh.Content, RecoverySpawnMap)
		if err != nil {
			ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Could not load recovery destination."}, opcodes.PhaserMapLoadResponse)
			return false
		}
		normalizedID = normalizedVisiblePlayerMapID(wh, RecoverySpawnMap)
		x, y = int(RecoverySpawnX), int(RecoverySpawnY)
		direction = RecoverySpawnDirection
		teleport = true
	}
	effect, err := commitMapLoad(ses.CommandContext(), wh.database, charID, mapLoadArrival{
		MapID: normalizedID, X: x, Y: y,
		ApplyEffects: wh.EventFlags != nil,
	})
	if err != nil {
		log.Printf("[Phaser] Commit map load for %d: %v", charID, err)
		ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Could not finish map loading. Please try again."}, opcodes.PhaserMapLoadResponse)
		return false
	}
	if wh.EventFlags != nil {
		if err := wh.EventFlags.LoadFlagsContext(ses.CommandContext(), charID); err != nil {
			log.Printf("[Phaser] Refresh committed map-load flags for %d: %v", charID, err)
		}
	}
	if teleport {
		publishCommittedPlayerPosition(ses, wh, normalizedID, x, y, direction)
	} else {
		// A load of the current location persists the owned snapshot without
		// clearing its path, facing, surfing or previous-map state.
		publishCommittedPlayerLocation(ses, wh, normalizedID, x, y)
	}
	if effect.Changed() {
		log.Printf("[Phaser] Committed map effects for %d map %s", charID, effect.MapName)
	}
	ses.SendStreamJSON(protocol.PhaserMapLoadResponse{Success: true, RequestID: req.RequestID, MapID: normalizedID, X: x, Y: y}, opcodes.PhaserMapLoadResponse)

	return false
}

func broadcastPlayerVisibleMapChange(ses *session.Session, wh *WorldHandler, previousMapID int) {
	broadcastPlayerActorVisibleMapChange(ses, wh, previousMapID, nil)
}

func broadcastPlayerActorVisibleMapChange(ses *session.Session, wh *WorldHandler, previousMapID int, playerActor *PhaserActor) {
	if ses == nil || wh == nil || wh.ActorManager == nil || !ses.HasValidClient() {
		return
	}
	if wh.ActorManager.IsOverworld(previousMapID) {
		previousMapID = UnifiedOverworldMapID
	}

	if playerActor == nil {
		playerActor = createPlayerActor(ses, wh)
		if playerActor == nil {
			return
		}
	}
	if previousMapID != 0 && previousMapID != playerActor.MapID {
		wh.ActorManager.broadcastActorDespawnExcept(playerActor.ID, previousMapID, ses.SessionID)
	}
	wh.ActorManager.broadcastActorUpdate(playerActor, ses.SessionID)
}

func normalizedVisiblePlayerMapID(wh *WorldHandler, mapID int) int {
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		return UnifiedOverworldMapID
	}
	return mapID
}

func currentPlayerVisibleMapID(ses *session.Session, wh *WorldHandler, charID int) int {
	if wh != nil && wh.PlayerMovement != nil {
		if _, _, mapID, ok := wh.PlayerMovement.GetPosition(charID); ok {
			return normalizedVisiblePlayerMapID(wh, mapID)
		}
	}
	if ses != nil && ses.HasValidClient() {
		if char := ses.Client.CharData(); char != nil {
			return normalizedVisiblePlayerMapID(wh, int(char.MapID))
		}
	}
	if ses != nil {
		return normalizedVisiblePlayerMapID(wh, ses.MapID)
	}
	return 0
}

func setServerTeleportedPlayerPosition(ses *session.Session, wh *WorldHandler, mapID, x, y int, direction string) (int, error) {
	if ses == nil || !ses.HasValidClient() {
		return 0, fmt.Errorf("teleport requires a character")
	}
	normalizedMapID := normalizedVisiblePlayerMapID(wh, mapID)
	if err := commitPlayerPosition(ses.CommandContext(), wh.database, int64(ses.Client.CharData().ID), normalizedMapID, x, y); err != nil {
		return 0, err
	}
	refreshSafariFlags(ses, wh, int64(ses.Client.CharData().ID))
	return publishCommittedPlayerPosition(ses, wh, mapID, x, y, direction), nil
}

// publishCommittedPlayerLocation refreshes location projections only after commit.
// It does not replace movement state or discard an in-progress path.
func publishCommittedPlayerLocation(ses *session.Session, wh *WorldHandler, mapID, x, y int) {
	char := ses.Client.CharData()
	ses.X, ses.Y, ses.MapID = float32(x), float32(y), mapID
	char.X, char.Y, char.MapID = float64(x), float64(y), uint32(mapID)
	if wh != nil && wh.PlayerMovement != nil {
		wh.PlayerMovement.markPositionCommitted(int(char.ID), x, y, mapID)
	}
}

// Publication accepts only a position saved by the caller's successful commit.
// It never writes storage or independently clears gameplay state.
func publishCommittedPlayerPosition(ses *session.Session, wh *WorldHandler, mapID, x, y int, direction string) int {
	normalizedDirection := normalizeWarpDirection(direction)
	if normalizedDirection == "" {
		normalizedDirection = "DOWN"
	}
	normalizedMapID := normalizedVisiblePlayerMapID(wh, mapID)

	if ses == nil || !ses.HasValidClient() {
		return normalizedMapID
	}
	char := ses.Client.CharData()
	if char == nil {
		return normalizedMapID
	}

	charID := int(char.ID)
	previousMapID := currentPlayerVisibleMapID(ses, wh, charID)

	if wh != nil && wh.PlayerMovement != nil {
		if _, _, _, ok := wh.PlayerMovement.GetPosition(charID); !ok {
			wh.PlayerMovement.RegisterPlayer(ses, charID, x, y, normalizedMapID, normalizedDirection)
		}
		wh.PlayerMovement.UpdatePosition(charID, x, y, normalizedMapID, normalizedDirection)
	}
	publishCommittedPlayerLocation(ses, wh, normalizedMapID, x, y)

	if wh == nil || wh.ActorManager == nil {
		return normalizedMapID
	}
	if previousMapID != 0 && previousMapID != normalizedMapID {
		broadcastPlayerVisibleMapChange(ses, wh, previousMapID)
		return normalizedMapID
	}

	playerActor := createPlayerActor(ses, wh)
	if playerActor != nil {
		playerActor.ActionDirection = &normalizedDirection
		wh.ActorManager.broadcastActorUpdate(playerActor, ses.SessionID)
	}
	return normalizedMapID
}

const (
	maxTileRegionCells = 20000
	maxTilePageSize    = 5000
)

// HandlePhaserTilesRequest returns tiles for a map. New clients use a bounded
// first request followed by id-keyset pages; requests without a request ID keep
// the legacy all-tiles response for stale deployed clients.
func HandlePhaserTilesRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	startedAt := time.Now()
	var req PhaserTilesRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid TilesRequest: %v", err)
		return false
	}
	sendError := func(err error) {
		if req.RequestID != "" {
			ses.SendStreamJSON(PhaserTilesResponse{
				MapID: req.MapID, RequestID: req.RequestID, Error: err.Error(),
			}, opcodes.PhaserTilesResponse)
			return
		}
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserTilesResponse)
	}

	hasAnyBound := req.MinX != nil || req.MinY != nil || req.MaxX != nil || req.MaxY != nil
	hasAllBounds := req.MinX != nil && req.MinY != nil && req.MaxX != nil && req.MaxY != nil
	if hasAnyBound && !hasAllBounds {
		err := fmt.Errorf("tile bounds require minX, minY, maxX, and maxY")
		log.Printf("[Phaser] Invalid bounded tiles request for map %d: %v", req.MapID, err)
		sendError(err)
		return false
	}
	if hasAllBounds {
		width := *req.MaxX - *req.MinX + 1
		height := *req.MaxY - *req.MinY + 1
		if width <= 0 || height <= 0 || width > maxTileRegionCells || height > maxTileRegionCells || width*height > maxTileRegionCells {
			err := fmt.Errorf("invalid tile bounds (%d x %d; max %d cells)", width, height, maxTileRegionCells)
			log.Printf("[Phaser] Invalid bounded tiles request for map %d: %v", req.MapID, err)
			sendError(err)
			return false
		}
	}
	pageSize := 0
	if req.AfterID != nil && *req.AfterID < 0 {
		err := fmt.Errorf("invalid tile page cursor %d", *req.AfterID)
		log.Printf("[Phaser] Invalid paged tiles request for map %d: %v", req.MapID, err)
		sendError(err)
		return false
	}
	if req.Limit != nil {
		pageSize = *req.Limit
		if pageSize <= 0 || pageSize > maxTilePageSize {
			err := fmt.Errorf("invalid tile page size %d (max %d)", pageSize, maxTilePageSize)
			log.Printf("[Phaser] Invalid paged tiles request for map %d: %v", req.MapID, err)
			sendError(err)
			return false
		}
	}

	query := `
		SELECT pt.id, pt.x, pt.y, pt.tile_image_id, pt.local_x, pt.local_y,
			pt.map_id, pt.source_map_id, pm.name AS source_map_name,
			pt.collision_type, pt.raw_foot_tile_id, pt.talk_over_tile,
			pt.is_native_game_data, pt.coordinate_origin, pt.content_origin
		FROM phaser_tiles pt
		LEFT JOIN phaser_maps pm ON pm.id = COALESCE(pt.source_map_id, pt.map_id)
		WHERE pt.map_id = $1 AND pt.is_tile_erased = 0`
	queryArgs := []interface{}{req.MapID}

	if req.MapID == UnifiedOverworldMapID {
		// Overworld tiles use global coordinates and have map_id IS NULL.
		query = `
			SELECT pt.id, pt.x, pt.y, pt.tile_image_id, pt.local_x, pt.local_y,
				pt.map_id, pt.source_map_id, pm.name AS source_map_name,
				pt.collision_type, pt.raw_foot_tile_id, pt.talk_over_tile,
				pt.is_native_game_data, pt.coordinate_origin, pt.content_origin
			FROM phaser_tiles pt
			LEFT JOIN phaser_maps pm ON pm.id = pt.source_map_id
			WHERE pt.map_id IS NULL AND pt.is_tile_erased = 0`
		queryArgs = nil
	}
	if hasAllBounds {
		first := len(queryArgs) + 1
		query += fmt.Sprintf(" AND pt.x BETWEEN $%d AND $%d AND pt.y BETWEEN $%d AND $%d",
			first, first+1, first+2, first+3)
		queryArgs = append(queryArgs, *req.MinX, *req.MaxX, *req.MinY, *req.MaxY)
	}
	if req.AfterID != nil {
		query += fmt.Sprintf(" AND pt.id > $%d", len(queryArgs)+1)
		queryArgs = append(queryArgs, *req.AfterID)
	}
	if pageSize > 0 {
		query += fmt.Sprintf(" ORDER BY pt.id LIMIT $%d", len(queryArgs)+1)
		queryArgs = append(queryArgs, pageSize)
	}

	rows, err := db.GlobalWorldDB.DB.Query(query, queryArgs...)
	if err != nil {
		log.Printf("[Phaser] Error querying tiles for map %d: %v", req.MapID, err)
		sendError(err)
		return false
	}
	defer rows.Close()

	// Keep empty correlated responses as [] rather than null. Chunk requests
	// routinely cover sparse parts of the overworld, and clients must be able to
	// treat an empty chunk as a completed request.
	tiles := make([]PhaserTile, 0)
	for rows.Next() {
		var t PhaserTile
		var mapID sql.NullInt64
		var sourceMapID sql.NullInt64
		var sourceMapName sql.NullString
		var rawFootTileID sql.NullInt64
		if err := rows.Scan(
			&t.ID, &t.X, &t.Y, &t.TileImageID, &t.LocalX, &t.LocalY,
			&mapID, &sourceMapID, &sourceMapName, &t.CollisionType,
			&rawFootTileID, &t.TalkOverTile, &t.IsNativeGameData,
			&t.CoordinateOrigin, &t.ContentOrigin,
		); err != nil {
			log.Printf("[Phaser] Error scanning tile: %v", err)
			continue
		}
		if rawFootTileID.Valid {
			v := int(rawFootTileID.Int64)
			t.RawFootTileID = &v
		}
		if mapID.Valid {
			t.MapID = int(mapID.Int64)
		} else {
			t.MapID = UnifiedOverworldMapID // Present as 9999 to clients
		}
		if sourceMapID.Valid {
			v := int(sourceMapID.Int64)
			t.SourceMapID = &v
		}
		if sourceMapName.Valid {
			v := sourceMapName.String
			t.SourceMapName = &v
		}
		tiles = append(tiles, t)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[Phaser] Error iterating tiles for map %d: %v", req.MapID, err)
		sendError(err)
		return false
	}
	loadedAt := time.Now()

	if ses.HasValidClient() && wh != nil && wh.EventFlags != nil {
		charID := int64(ses.Client.CharData().ID)
		tiles = ApplyEventTileOverridesToTiles(charID, req.MapID, wh.EventFlags, tiles)
	}

	// PhaserTile already has the exact camelCase JSON contract. Sending the
	// typed slice directly also honors omitempty for its nullable source fields.
	// Converting tens of thousands of overworld tiles into reflection-built
	// map[string]interface{} values doubled allocation pressure, inflated the
	// payload with null keys, and could push the response past the client timer.
	responsePayload := interface{}(tiles)
	if req.RequestID != "" {
		nextAfterID := 0
		if len(tiles) > 0 {
			nextAfterID = tiles[len(tiles)-1].ID
		}
		responsePayload = PhaserTilesResponse{
			MapID: req.MapID, RequestID: req.RequestID, Tiles: tiles,
			NextAfterID: nextAfterID, HasMore: pageSize > 0 && len(tiles) == pageSize,
		}
	}
	if err := ses.SendStreamJSON(responsePayload, opcodes.PhaserTilesResponse); err != nil {
		log.Printf("[Phaser] Failed sending %d tiles for map %d after %s (query/scan %s): %v",
			len(tiles), req.MapID, time.Since(startedAt).Round(time.Millisecond), loadedAt.Sub(startedAt).Round(time.Millisecond), err)
		return false
	}
	log.Printf("[Phaser] Sent %d tiles for map %d in %s (query/scan %s)",
		len(tiles), req.MapID, time.Since(startedAt).Round(time.Millisecond), loadedAt.Sub(startedAt).Round(time.Millisecond))
	return false
}

// HandlePhaserOverworldMapsRequest returns all overworld maps
func HandlePhaserOverworldMapsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	maps, err := wh.Content.OverworldMaps(ses.CommandContext())
	if err != nil {
		log.Printf("[Phaser] Error querying overworld maps: %v", err)
		ses.SendStreamJSON(protocol.ErrorResponse{Error: "Could not load overworld maps."}, opcodes.PhaserOverworldMapsResponse)
		return false
	}
	ses.SendStreamJSON(maps, opcodes.PhaserOverworldMapsResponse)
	return false
}

// HandlePhaserActorsRequest returns actors for a specific map
func HandlePhaserActorsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req PhaserActorsRequest
	charID := int64(ses.Client.CharData().ID)
	fail := func() {
		sendInventoryCommandError(ses, req.RequestID, opcodes.PhaserActorsResponse, "Actor view unavailable.")
	}
	if decodePlayerMovement(payload, &req) != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != charID || req.MapID <= 0 {
		fail()
		return false
	}
	isOverworldTarget := req.MapID == UnifiedOverworldMapID

	query := `
		SELECT po.id, COALESCE(po.x, po.local_x) as x, COALESCE(po.y, po.local_y) as y,
			po.map_id, po.object_type, po.sprite_name, po.name,
			po.action_type, po.action_direction, po.movement_type,
			po.text, po.trainer_class, po.trainer_party_index, po.item_id
		FROM phaser_objects po
		LEFT JOIN character_collected_items cci
			ON cci.object_id = po.id AND cci.character_id = $1
		WHERE po.map_id = $2 AND cci.object_id IS NULL`
	queryArgs := []interface{}{charID, req.MapID}

	if isOverworldTarget {
		// Overworld objects have global coordinates baked in by the importer.
		query = `
			SELECT po.id, po.x, po.y,
				po.map_id, po.object_type, po.sprite_name, po.name,
				po.action_type, po.action_direction, po.movement_type,
				po.text, po.trainer_class, po.trainer_party_index, po.item_id
			FROM phaser_objects po
			JOIN phaser_maps pm ON po.map_id = pm.id
			LEFT JOIN character_collected_items cci
				ON cci.object_id = po.id AND cci.character_id = $1
			WHERE pm.is_overworld = 1 AND cci.object_id IS NULL`
		queryArgs = []interface{}{charID}
	}

	var actors []PhaserActor
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		var locked int64
		if err := tx.QueryRow(`SELECT id FROM character_data WHERE id=$1 FOR UPDATE`, charID).Scan(&locked); err != nil {
			return err
		}
		rows, err := tx.Query(query, queryArgs...)
		if err != nil {
			return err
		}
		defer rows.Close()

		actors = make([]PhaserActor, 0)
		for rows.Next() {
			var n PhaserActor
			if err := rows.Scan(&n.ID, &n.X, &n.Y, &n.MapID, &n.ObjectType, &n.SpriteName, &n.Name, &n.ActionType, &n.ActionDirection, &n.MovementType, &n.Text, &n.TrainerClass, &n.TrainerPartyIndex, &n.ItemID); err != nil {
				return err
			}
			// Set default move speed (300ms per tile for walking)
			if n.ActionType != nil && *n.ActionType == "WALK" {
				n.MoveSpeed = 300
			} else {
				n.MoveSpeed = 0 // Static actors don't need move speed
			}

			// Store original DB ID before remapping
			n.DbID = n.ID

			// Use the registry to get a unified runtime ID
			n.ID = wh.ActorRegistry.GetPhaserID(ActorTypeNPC, n.ID)
			if wh.ActorManager != nil {
				wh.ActorManager.applyRuntimeActorState(&n)
			}

			actors = append(actors, n)
		}

		if err := rows.Err(); err != nil {
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		flags, err := eventFlagSnapshotIn(tx, charID)
		if err != nil {
			return err
		}
		actors, err = applyEventObjectVisibilityContext(ses.CommandContext(), tx.(db.ContextDBTX), charID, req.MapID, flags, actors)
		if err != nil {
			return err
		}
		actors, err = applyCharacterObjectPositionsContext(ses.CommandContext(), tx.(db.ContextDBTX), charID, actors)
		return err

	})
	if err != nil {
		fail()
		return false
	}
	// Add all players on this map (or overworld if target is overworld)
	wh.sessionManager.ForEachSession(func(otherSes *session.Session) {
		presence := otherSes.Presence()
		if presence.CharacterID == 0 {
			return
		}

		// Include if on the same map, or if both are in overworld (Map 9999)
		if presence.MapID == req.MapID || (isOverworldTarget && wh.ActorManager.IsOverworld(presence.MapID)) {
			otherActor := createPlayerActorFromPresence(presence, wh)
			if otherActor != nil {
				actors = append(actors, *otherActor)
			}
		}
	})

	ses.SendStreamJSON(PhaserActorsResponse{Success: true, RequestID: req.RequestID, CharacterID: charID, MapID: req.MapID, Actors: actors}, opcodes.PhaserActorsResponse)
	log.Printf("[Phaser] Sent %d actors (including players) for map %d", len(actors), req.MapID)

	return false
}

// HandlePhaserWarpsRequest returns warps for a specific map
type PhaserWarpsResponse struct {
	Success     bool         `json:"success" tstype:"true"`
	RequestID   string       `json:"requestId"`
	CharacterID int64        `json:"characterId"`
	MapID       int          `json:"mapId"`
	Warps       []PhaserWarp `json:"warps"`
}

func HandlePhaserWarpsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserWarpsRequest
	fail := func(message string) bool {
		ses.SendStreamJSON(protocol.PlayerStepError{RequestID: req.RequestID, Error: message}, opcodes.PhaserWarpsResponse)
		return false
	}
	if decodePlayerMovement(payload, &req) != nil || !validBattleRequestID(req.RequestID) || !ses.HasValidClient() {
		return fail("Invalid warp read")
	}
	warps, err := readPlayerWarps(ses.CommandContext(), wh.database, req.MapID, ses.PreviousMapID)
	if err != nil {
		return fail("Warp data unavailable; retry the read")
	}
	ses.SendStreamJSON(PhaserWarpsResponse{Success: true, RequestID: req.RequestID, CharacterID: int64(ses.Client.CharData().ID), MapID: req.MapID, Warps: warps}, opcodes.PhaserWarpsResponse)
	return false
}
func readPlayerWarps(ctx context.Context, database *sql.DB, mapID, previousMapID int) ([]PhaserWarp, error) {
	query := `
		SELECT id, source_map_id, x, y, destination_map_id, destination_map,
		       destination_x, destination_y, destination_kind,
		       destination_warp_id, COALESCE(warp_type,'door'), warp_direction
		FROM phaser_warps
		WHERE source_map_id = $1
		  AND COALESCE(warp_type, 'door') NOT IN ('elevator', 'inactive')
		  AND (
			destination_kind = 'last-map'
			OR (
				destination_map_id IS NOT NULL
				AND destination_x IS NOT NULL
				AND destination_y IS NOT NULL
			)
		  )`
	queryArgs := []interface{}{mapID}

	if mapID == UnifiedOverworldMapID {
		// Overworld warps have global coordinates baked in by the importer.
		query = `
			SELECT pw.id, pw.source_map_id, pw.x, pw.y, pw.destination_map_id,
			       pw.destination_map, pw.destination_x, pw.destination_y,
			       pw.destination_kind, pw.destination_warp_id,
			       COALESCE(pw.warp_type,'door'), pw.warp_direction
			FROM phaser_warps pw
			JOIN phaser_maps pm ON pw.source_map_id = pm.id
			WHERE pm.is_overworld = 1
			  AND pw.destination_map_id IS NOT NULL
			  AND pw.destination_x IS NOT NULL
			  AND pw.destination_y IS NOT NULL
			  AND COALESCE(pw.warp_type, 'door') NOT IN ('elevator', 'inactive')`
		queryArgs = nil
	}

	return db.ReadSnapshot(ctx, database, func(ctx context.Context, q db.ReadDBTX) ([]PhaserWarp, error) {
		rows, err := q.Query(query, queryArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		warps := make([]PhaserWarp, 0)
		for rows.Next() {
			var w PhaserWarp
			if err := rows.Scan(&w.ID, &w.SourceMapID, &w.X, &w.Y, &w.DestinationMapID, &w.DestinationMap, &w.DestinationX, &w.DestinationY, &w.DestinationKind, &w.DestinationWarpID, &w.WarpType, &w.WarpDirection); err != nil {
				return nil, err
			}
			warps = append(warps, w)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		// Resolve only after consuming/closing the catalog cursor, through the SAME
		// snapshot and deadline. Never open a second pool connection for LAST_MAP.
		for i, w := range warps {
			resolved, err := resolvePhaserWarpForPlayer(q, previousMapID, w)
			if err != nil {
				return nil, fmt.Errorf("resolve warp %d: %w", w.ID, err)
			}
			warps[i] = resolved
		}
		return warps, nil
	})
}

func shouldResolveLastMapForPlayer(w PhaserWarp) bool {
	return w.DestinationKind == "last-map" &&
		(w.DestinationMapID == nil || w.DestinationX == nil || w.DestinationY == nil)
}

func resolvePhaserWarpForPlayer(database db.DBTX, previousMapID int, w PhaserWarp) (PhaserWarp, error) {
	if !shouldResolveLastMapForPlayer(w) {
		return w, nil
	}
	mapID, mapName, x, y, err := resolveLastMapWarpForPlayer(
		database, previousMapID, w.DestinationWarpID,
	)
	if err != nil {
		return PhaserWarp{}, err
	}
	w.DestinationMapID = &mapID
	w.DestinationMap = &mapName
	w.DestinationX = &x
	w.DestinationY = &y
	return w, nil
}

func resolveLastMapWarpForPlayer(
	database db.DBTX, previousMapID, destinationWarpIndex int,
) (int, string, int, int, error) {
	if previousMapID < 0 {
		return 0, "", 0, 0, fmt.Errorf("player has no previous-map context")
	}
	var mapID, x, y int
	var mapName string
	err := database.QueryRow(`
		SELECT warp.source_map_id, map.name, warp.x, warp.y
		FROM phaser_warps AS warp
		JOIN phaser_maps AS map ON map.id = warp.source_map_id
		WHERE warp.source_map_id = $1 AND warp.source_warp_index = $2
		ORDER BY warp.id
		LIMIT 1`, previousMapID, destinationWarpIndex).Scan(
		&mapID, &mapName, &x, &y,
	)
	if err != nil {
		return 0, "", 0, 0, fmt.Errorf(
			"resolve previous map %d warp %d: %w",
			previousMapID, destinationWarpIndex, err,
		)
	}
	return mapID, mapName, x, y, nil
}

func broadcastCommittedPlayerStep(ses *session.Session, wh *WorldHandler, x, y, mapID int, direction string, previousMapID int) {
	char := ses.Client.CharData()
	ridingBicycle := wh.PlayerMovement != nil && wh.PlayerMovement.IsBicycleActive(int(char.ID))
	surfing := wh.PlayerMovement != nil && wh.PlayerMovement.IsSurfing(int(char.ID))
	spriteName := playerSpriteName(char.Gender, ridingBicycle, surfing)

	both := "BOTH"
	playerActor := PhaserActor{
		ID:              wh.ActorRegistry.GetPhaserID(ActorTypePlayer, int(char.ID)),
		InternalID:      int(char.ID),
		X:               &x,
		Y:               &y,
		MapID:           mapID,
		ObjectType:      "player",
		SpriteName:      &spriteName,
		Name:            &char.Name,
		ActionType:      nil,
		ActionDirection: &direction,
		MovementType:    &both,
		MoveSpeed:       wh.PlayerMovement.GetMoveSpeed(int(char.ID)),
	}

	if previousMapID != mapID {
		broadcastPlayerActorVisibleMapChange(ses, wh, previousMapID, &playerActor)
	} else {
		wh.ActorManager.broadcastActorUpdate(&playerActor, ses.SessionID)
	}
}

// SendPlayerSpawn sends the player's initial position and sprite to the client
// and broadcasts their presence to other players in the same map/overworld.
// Successful world entry publishes presence once. Reads only enumerate it;
// the entering player gets its own actor through the correlated map snapshot.
func SendPlayerSpawn(ses *session.Session, wh *WorldHandler) {
	if ses == nil || !ses.HasValidClient() || ses.IsClosed() || wh == nil || wh.ActorManager == nil {
		return
	}
	actor := createPlayerActor(ses, wh)
	if actor != nil {
		wh.ActorManager.broadcastActorSpawn(actor, ses.SessionID)
	}
}

// createPlayerActor creates a PhaserActor representation of the session's player
func createPlayerActor(ses *session.Session, wh *WorldHandler) *PhaserActor {
	return createPlayerActorFromPresence(ses.PublishPresence(), wh)
}

func createPlayerActorFromPresence(p session.Presence, wh *WorldHandler) *PhaserActor {
	if p.CharacterID == 0 {
		return nil
	}

	// Use stored position from character data
	spawnX := int(p.CharacterX)
	spawnY := int(p.CharacterY)
	storedMapID := int(p.CharacterMapID)
	mapID := storedMapID

	// Normalize overworld maps to 9999 for the client
	if wh.ActorManager.IsOverworld(mapID) {
		mapID = UnifiedOverworldMapID
	}

	if isInvalidZeroPlayerPosition(spawnX, spawnY) {
		spawnX = int(RecoverySpawnX)
		spawnY = int(RecoverySpawnY)
		storedMapID = RecoverySpawnMap
		mapID = RecoverySpawnMap
		if wh.ActorManager.IsOverworld(mapID) {
			mapID = UnifiedOverworldMapID
		}
	}

	ridingBicycle := wh.PlayerMovement != nil && wh.PlayerMovement.IsBicycleActive(int(p.CharacterID))
	surfing := wh.PlayerMovement != nil && wh.PlayerMovement.IsSurfing(int(p.CharacterID))
	if !surfing && wh.ActorManager != nil {
		if collisionType, exists := wh.ActorManager.CollisionTypeAt(mapID, spawnX, spawnY); exists && collisionType == collisionWater {
			surfing = true
		}
	}
	spriteName := playerSpriteName(p.Gender, ridingBicycle, surfing)

	objectType := "player"
	stay := "STAY"
	direction := initialPlayerDirection(storedMapID, spawnX, spawnY)
	both := "BOTH"

	return &PhaserActor{
		ID:              wh.ActorRegistry.GetPhaserID(ActorTypePlayer, int(p.CharacterID)),
		InternalID:      int(p.CharacterID),
		X:               &spawnX,
		Y:               &spawnY,
		MapID:           mapID,
		ObjectType:      objectType,
		SpriteName:      &spriteName,
		Name:            &p.Name,
		ActionType:      &stay,
		ActionDirection: &direction,
		MovementType:    &both,
		MoveSpeed:       wh.PlayerMovement.GetMoveSpeed(int(p.CharacterID)),
	}
}

func isInvalidZeroPlayerPosition(x, y int) bool {
	return x == 0 && y == 0
}

func recoverInvalidCharacterPosition(ses *session.Session, wh *WorldHandler) (bool, error) {
	if ses == nil || !ses.HasValidClient() {
		return false, nil
	}
	char := ses.Client.CharData()
	if char == nil || !isInvalidZeroPlayerPosition(int(char.X), int(char.Y)) {
		return false, nil
	}

	mapID := RecoverySpawnMap
	x := int(RecoverySpawnX)
	y := int(RecoverySpawnY)
	sessionMapID := mapID
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		sessionMapID = UnifiedOverworldMapID
	}

	if _, err := setServerTeleportedPlayerPosition(ses, wh, mapID, x, y, RecoverySpawnDirection); err != nil {
		return false, err
	}
	log.Printf("[Phaser] Recovered invalid saved position for player %d to map %d (%d,%d)", char.ID, sessionMapID, x, y)
	return true, nil
}

// RegisterPlayerForMovement registers a player with the movement manager when they spawn
func RegisterPlayerForMovement(ses *session.Session, wh *WorldHandler) error {
	if !ses.HasValidClient() {
		return nil
	}
	char := ses.Client.CharData()
	if char == nil {
		return nil
	}

	storedMapID := int(char.MapID)
	mapID := storedMapID
	if wh.ActorManager.IsOverworld(mapID) {
		mapID = UnifiedOverworldMapID
	}

	wh.PlayerMovement.RegisterPlayer(
		ses,
		int(char.ID),
		int(char.X),
		int(char.Y),
		mapID,
		initialPlayerDirection(storedMapID, int(char.X), int(char.Y)),
	)
	return wh.PlayerMovement.restoreMovementRoute(ses)
}
