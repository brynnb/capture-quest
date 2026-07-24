package world

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/session"
)

// PhaserMapInfo represents a map in the 2D Phaser game
type PhaserMapInfo struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	TilesetID   *int   `json:"tilesetId,omitempty"`
	IsOverworld int    `json:"isOverworld"`
}

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
	ID               int     `json:"id"`
	SourceMapID      int     `json:"sourceMapId"`
	X                int     `json:"x"`
	Y                int     `json:"y"`
	DestinationMapID *int    `json:"destinationMapId,omitempty"`
	DestinationMap   *string `json:"destinationMap,omitempty"`
	DestinationX     *int    `json:"destinationX,omitempty"`
	DestinationY     *int    `json:"destinationY,omitempty"`
	WarpType         string  `json:"warpType"`
	WarpDirection    *string `json:"warpDirection,omitempty"`
}

// PhaserMapInfoRequest is the request payload
type PhaserMapInfoRequest struct {
	MapID int  `json:"mapId"`
	DestX *int `json:"destX,omitempty"`
	DestY *int `json:"destY,omitempty"`
}

// PhaserTilesRequest is the request payload
type PhaserTilesRequest struct {
	MapID int `json:"mapId"`
}

// PhaserActorsRequest is the request payload
type PhaserActorsRequest struct {
	MapID int `json:"mapId"`
}

// PhaserWarpsRequest is the request payload
type PhaserWarpsRequest struct {
	MapID int `json:"mapId"`
}

// HandlePhaserMapInfoRequest returns info about a specific map
func HandlePhaserMapInfoRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserMapInfoRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid MapInfoRequest: %v", err)
		return false
	}

	var mapInfo PhaserMapInfo
	if req.MapID == UnifiedOverworldMapID {
		mapInfo = PhaserMapInfo{
			ID:          UnifiedOverworldMapID,
			Name:        "Unified Overworld",
			Width:       500, // Large enough to encompass everything
			Height:      500,
			IsOverworld: 1,
		}
	} else {
		err := db.GlobalWorldDB.DB.QueryRow(`
			SELECT id, name, width, height, tileset_id, is_overworld
			FROM phaser_maps WHERE id = $1`, req.MapID).Scan(
			&mapInfo.ID, &mapInfo.Name, &mapInfo.Width, &mapInfo.Height, &mapInfo.TilesetID, &mapInfo.IsOverworld)
		if err != nil {
			log.Printf("[Phaser] Error querying map info for %d: %v", req.MapID, err)
			ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserMapInfoResponse)
			return false
		}
	}

	ses.SendStreamJSON(StructToMap(mapInfo), opcodes.PhaserMapInfoResponse)

	// Normalize overworld maps to the unified ID for session tracking
	normalizedID := mapInfo.ID
	if wh.ActorManager.IsOverworld(normalizedID) {
		normalizedID = UnifiedOverworldMapID
	}
	previousVisibleMapID := ses.MapID
	if ses.HasValidClient() {
		if char := ses.Client.CharData(); char != nil {
			previousVisibleMapID = int(char.MapID)
			if wh.ActorManager.IsOverworld(previousVisibleMapID) {
				previousVisibleMapID = UnifiedOverworldMapID
			}
		}
	}
	ses.MapID = normalizedID

	// Keep char.MapID in sync so the server always knows the player's current map.
	// This prevents stale interior map IDs from being used for auto-registration
	// when the client loads a different map (e.g., overworld on login).
	if ses.HasValidClient() {
		char := ses.Client.CharData()
		if char != nil {
			if recoverInvalidCharacterPosition(ses, wh) {
				char = ses.Client.CharData()
			}
			if req.DestX != nil && req.DestY != nil {
				// Warp: update position atomically so fetchActors returns correct position
				log.Printf("[Phaser] Warp position update via MapInfo: player %d -> map %d (%d,%d)",
					char.ID, req.MapID, *req.DestX, *req.DestY)
				char.X = float64(*req.DestX)
				char.Y = float64(*req.DestY)
				char.MapID = uint32(normalizedID)
				ses.X = float32(*req.DestX)
				ses.Y = float32(*req.DestY)

				// Sync with movement manager
				wh.PlayerMovement.UpdatePosition(int(char.ID), *req.DestX, *req.DestY, normalizedID, "DOWN")
				wh.PlayerMovement.FlushPlayerPosition(int(char.ID))
				broadcastPlayerVisibleMapChange(ses, wh, previousVisibleMapID)
			} else {
				charMapID := int(char.MapID)
				if wh.ActorManager.IsOverworld(charMapID) {
					charMapID = UnifiedOverworldMapID
				}
				if charMapID == normalizedID {
					ses.X = float32(char.X)
					ses.Y = float32(char.Y)
				}
			}
			if wh.EventFlags != nil {
				effectMapName := ""
				if req.MapID == UnifiedOverworldMapID && req.DestX != nil && req.DestY != nil {
					effectMapName = OverworldMapLoadNameForPosition(*req.DestX, *req.DestY)
				}
				var (
					effect MapLoadEffect
					err    error
				)
				if effectMapName != "" {
					effect, err = ApplyMapLoadScriptEffectsForMapName(int64(char.ID), effectMapName, wh.EventFlags)
				} else {
					effect, err = ApplyMapLoadScriptEffects(int64(char.ID), req.MapID, wh.EventFlags)
				}
				if err != nil {
					log.Printf("[Phaser] Map-load effects failed for char %d map %d: %v", char.ID, req.MapID, err)
				} else if effect.Changed() {
					log.Printf("[Phaser] Applied map-load effects for char %d map %s: set=%v reset=%v affected=%v",
						char.ID, effect.MapName, effect.SetFlags, effect.ResetFlags, effect.AffectedMapNames)
				}
			}
		}
	}

	log.Printf("[Phaser] Sent map info for map %d (%s), updated session map ID", mapInfo.ID, mapInfo.Name)
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

func setServerTeleportedPlayerPosition(ses *session.Session, wh *WorldHandler, mapID, x, y int, direction string) int {
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
	if previousMapID != 0 {
		endSafariSessionIfLeavingMap(int64(char.ID), previousMapID, normalizedMapID, wh)
	}

	ses.X = float32(x)
	ses.Y = float32(y)
	ses.MapID = normalizedMapID
	char.X = float64(x)
	char.Y = float64(y)
	char.MapID = uint32(normalizedMapID)

	if wh != nil && wh.PlayerMovement != nil {
		if _, _, _, ok := wh.PlayerMovement.GetPosition(charID); !ok {
			wh.PlayerMovement.RegisterPlayer(ses, charID, x, y, normalizedMapID, normalizedDirection)
		}
		wh.PlayerMovement.UpdatePosition(charID, x, y, normalizedMapID, normalizedDirection)
		wh.PlayerMovement.FlushPlayerPosition(charID)
	} else if db.GlobalWorldDB != nil && db.GlobalWorldDB.DB != nil {
		if err := db_character.UpdateCharacterPosition(
			int32(char.ID),
			uint32(normalizedMapID),
			float64(x),
			float64(y),
			0,
			0,
		); err != nil {
			log.Printf("[Phaser] Failed to save teleported player %d at map %d (%d,%d): %v",
				char.ID, normalizedMapID, x, y, err)
		}
	} else {
		log.Printf("[Phaser] Skipped saving teleported player %d at map %d (%d,%d): database unavailable",
			int32(char.ID),
			normalizedMapID, x, y)
	}

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

// HandlePhaserTilesRequest returns all tiles for a map
func HandlePhaserTilesRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserTilesRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid TilesRequest: %v", err)
		return false
	}

	query := `
		SELECT pt.id, pt.x, pt.y, pt.tile_image_id, pt.local_x, pt.local_y,
			pt.map_id, pt.source_map_id, pm.name AS source_map_name,
			pt.collision_type, pt.raw_foot_tile_id, pt.talk_over_tile
		FROM phaser_tiles pt
		LEFT JOIN phaser_maps pm ON pm.id = COALESCE(pt.source_map_id, pt.map_id)
		WHERE pt.map_id = $1`
	queryArgs := []interface{}{req.MapID}

	if req.MapID == UnifiedOverworldMapID {
		// Overworld tiles use global coordinates and have map_id IS NULL.
		query = `
			SELECT pt.id, pt.x, pt.y, pt.tile_image_id, pt.local_x, pt.local_y,
				pt.map_id, pt.source_map_id, pm.name AS source_map_name,
				pt.collision_type, pt.raw_foot_tile_id, pt.talk_over_tile
			FROM phaser_tiles pt
			LEFT JOIN phaser_maps pm ON pm.id = pt.source_map_id
			WHERE pt.map_id IS NULL`
		queryArgs = nil
	}

	rows, err := db.GlobalWorldDB.DB.Query(query, queryArgs...)
	if err != nil {
		log.Printf("[Phaser] Error querying tiles for map %d: %v", req.MapID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserTilesResponse)
		return false
	}
	defer rows.Close()

	var tiles []PhaserTile
	for rows.Next() {
		var t PhaserTile
		var mapID sql.NullInt64
		var sourceMapID sql.NullInt64
		var sourceMapName sql.NullString
		var rawFootTileID sql.NullInt64
		if err := rows.Scan(&t.ID, &t.X, &t.Y, &t.TileImageID, &t.LocalX, &t.LocalY, &mapID, &sourceMapID, &sourceMapName, &t.CollisionType, &rawFootTileID, &t.TalkOverTile); err != nil {
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

	if ses.HasValidClient() && wh != nil && wh.EventFlags != nil {
		charID := int64(ses.Client.CharData().ID)
		tiles = ApplyEventTileOverridesToTiles(charID, req.MapID, wh.EventFlags, tiles)
	}

	ses.SendStreamJSON(StructToMap(tiles), opcodes.PhaserTilesResponse)
	log.Printf("[Phaser] Sent %d tiles for map %d", len(tiles), req.MapID)
	return false
}

// HandlePhaserOverworldMapsRequest returns all overworld maps
func HandlePhaserOverworldMapsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT id, name, width, height, tileset_id, is_overworld
		FROM phaser_maps WHERE is_overworld = 1`)
	if err != nil {
		log.Printf("[Phaser] Error querying overworld maps: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserOverworldMapsResponse)
		return false
	}
	defer rows.Close()

	var maps []PhaserMapInfo
	for rows.Next() {
		var m PhaserMapInfo
		if err := rows.Scan(&m.ID, &m.Name, &m.Width, &m.Height, &m.TilesetID, &m.IsOverworld); err != nil {
			log.Printf("[Phaser] Error scanning map: %v", err)
			continue
		}
		maps = append(maps, m)
	}

	ses.SendStreamJSON(StructToMap(maps), opcodes.PhaserOverworldMapsResponse)
	if len(maps) > 0 {
		ses.MapID = maps[0].ID
	}
	log.Printf("[Phaser] Sent %d overworld maps, updated session map ID to %d", len(maps), ses.MapID)
	return false
}

// HandlePhaserActorsRequest returns actors for a specific map
func HandlePhaserActorsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserActorsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid ActorsRequest: %v", err)
		return false
	}

	isOverworldTarget := req.MapID == UnifiedOverworldMapID

	// Get character ID for filtering collected items
	var charID int64
	if ses.HasValidClient() {
		charID = int64(ses.Client.CharData().ID)
	}

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

	rows, err := db.GlobalWorldDB.DB.Query(query, queryArgs...)
	if err != nil {
		log.Printf("[Phaser] Error querying actors for map %d: %v", req.MapID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserActorsResponse)
		return false
	}
	defer rows.Close()

	var actors []PhaserActor
	for rows.Next() {
		var n PhaserActor
		if err := rows.Scan(&n.ID, &n.X, &n.Y, &n.MapID, &n.ObjectType, &n.SpriteName, &n.Name, &n.ActionType, &n.ActionDirection, &n.MovementType, &n.Text, &n.TrainerClass, &n.TrainerPartyIndex, &n.ItemID); err != nil {
			log.Printf("[Phaser] Error scanning actor: %v", err)
			continue
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

	actors = ApplyEventObjectVisibilityToActors(charID, req.MapID, wh.EventFlags, actors)
	actors = ApplyCharacterObjectPositions(charID, actors)

	// Add all players on this map (or overworld if target is overworld)
	wh.sessionManager.ForEachSession(func(otherSes *session.Session) {
		if !otherSes.HasValidClient() {
			return
		}

		// Include if on the same map, or if both are in overworld (Map 9999)
		if otherSes.MapID == req.MapID || (isOverworldTarget && wh.ActorManager.IsOverworld(otherSes.MapID)) {
			otherActor := createPlayerActor(otherSes, wh)
			if otherActor != nil {
				actors = append(actors, *otherActor)
			}
		}
	})

	ses.SendStreamJSON(StructToMap(actors), opcodes.PhaserActorsResponse)
	log.Printf("[Phaser] Sent %d actors (including players) for map %d", len(actors), req.MapID)

	// Broadcast this player's presence to other sessions so they see the new player immediately
	if ses.HasValidClient() {
		playerActor := createPlayerActor(ses, wh)
		if playerActor != nil {
			wh.ActorManager.broadcastActorSpawn(playerActor, ses.SessionID)
			log.Printf("[Phaser] Broadcast player spawn for %s to other sessions", ses.Client.CharData().Name)
		}
	}

	return false
}

// HandlePhaserWarpsRequest returns warps for a specific map
func HandlePhaserWarpsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserWarpsRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Phaser] Invalid WarpsRequest: %v", err)
		return false
	}

	query := `
		SELECT id, source_map_id, x, y, destination_map_id, destination_map, destination_x, destination_y, warp_type, warp_direction
		FROM phaser_warps
		WHERE source_map_id = $1
		  AND destination_map_id IS NOT NULL
		  AND destination_x IS NOT NULL
		  AND destination_y IS NOT NULL
		  AND COALESCE(warp_type, 'door') NOT IN ('elevator', 'inactive')`
	queryArgs := []interface{}{req.MapID}

	if req.MapID == UnifiedOverworldMapID {
		// Overworld warps have global coordinates baked in by the importer.
		query = `
			SELECT pw.id, pw.source_map_id, pw.x, pw.y, pw.destination_map_id, pw.destination_map, pw.destination_x, pw.destination_y, pw.warp_type, pw.warp_direction
			FROM phaser_warps pw
			JOIN phaser_maps pm ON pw.source_map_id = pm.id
			WHERE pm.is_overworld = 1
			  AND pw.destination_map_id IS NOT NULL
			  AND pw.destination_x IS NOT NULL
			  AND pw.destination_y IS NOT NULL
			  AND COALESCE(pw.warp_type, 'door') NOT IN ('elevator', 'inactive')`
		queryArgs = nil
	}

	rows, err := db.GlobalWorldDB.DB.Query(query, queryArgs...)
	if err != nil {
		log.Printf("[Phaser] Error querying warps for map %d: %v", req.MapID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.PhaserWarpsResponse)
		return false
	}
	defer rows.Close()

	var warps []PhaserWarp
	for rows.Next() {
		var w PhaserWarp
		if err := rows.Scan(&w.ID, &w.SourceMapID, &w.X, &w.Y, &w.DestinationMapID, &w.DestinationMap, &w.DestinationX, &w.DestinationY, &w.WarpType, &w.WarpDirection); err != nil {
			log.Printf("[Phaser] Error scanning warp: %v", err)
			continue
		}
		warps = append(warps, w)
	}

	ses.SendStreamJSON(StructToMap(warps), opcodes.PhaserWarpsResponse)
	log.Printf("[Phaser] Sent %d warps for map %d", len(warps), req.MapID)
	return false
}

// PhaserPlayerPositionUpdateRequest is the request payload from client.
type PhaserPlayerPositionUpdateRequest struct {
	X         int    `json:"x"`
	Y         int    `json:"y"`
	MapID     int    `json:"mapId"`
	Direction string `json:"direction"`
}

// HandlePhaserPlayerPositionUpdate handles client-reported player position updates.
func HandlePhaserPlayerPositionUpdate(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PhaserPlayerPositionUpdateRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}

	mapID := req.MapID
	if wh.ActorManager.IsOverworld(mapID) {
		mapID = UnifiedOverworldMapID
	}

	prevX, prevY, prevMapID, hadPrevPosition := 0, 0, 0, false
	prevX = int(char.X)
	prevY = int(char.Y)
	prevMapID = int(char.MapID)
	if wh.ActorManager.IsOverworld(prevMapID) {
		prevMapID = UnifiedOverworldMapID
	}
	if wh.PlayerMovement != nil {
		if mx, my, mm, ok := wh.PlayerMovement.GetPosition(int(char.ID)); ok {
			prevX, prevY, prevMapID = mx, my, mm
			hadPrevPosition = true
		}
	}
	if !hadPrevPosition {
		hadPrevPosition = true
	}

	direction := normalizeWarpDirection(req.Direction)
	if direction == "" && wh.PlayerMovement != nil {
		if currentDirection, ok := wh.PlayerMovement.GetDirection(int(char.ID)); ok {
			direction = currentDirection
		}
	}
	if direction == "" {
		direction = initialPlayerDirection(int(char.MapID), int(char.X), int(char.Y))
	}

	sameTileFacing :=
		hadPrevPosition &&
			prevMapID == mapID &&
			prevX == req.X &&
			prevY == req.Y &&
			normalizeBoulderDirection(direction) != ""

	reportedMove :=
		hadPrevPosition &&
			(prevMapID != mapID || prevX != req.X || prevY != req.Y)
	mapChanged := prevMapID != 0 && prevMapID != mapID

	ses.X = float32(req.X)
	ses.Y = float32(req.Y)
	ses.MapID = mapID

	char.X = float64(req.X)
	char.Y = float64(req.Y)
	char.MapID = uint32(mapID)

	if wh.PlayerMovement != nil {
		if _, _, _, ok := wh.PlayerMovement.GetPosition(int(char.ID)); !ok {
			wh.PlayerMovement.RegisterPlayer(ses, int(char.ID), req.X, req.Y, mapID, direction)
		}
		wh.PlayerMovement.UpdateReportedPosition(int(char.ID), req.X, req.Y, mapID, direction)
		wh.PlayerMovement.FlushPlayerPosition(int(char.ID))
	}

	if sameTileFacing && wh.PlayerMovement != nil {
		charID := int(char.ID)
		if result, attempted := wh.PlayerMovement.tryPushBoulderFromFacingAttempt(charID, mapID, req.X, req.Y, direction); attempted {
			if result.Success {
				wh.PlayerMovement.queueStepAfterBoulderPush(charID, req.X, req.Y, mapID, result)
				log.Printf("[PlayerMovement] Player %d pushed boulder %s from facing %s",
					charID, result.ObjectName, direction)
			} else {
				log.Printf("[PlayerMovement] Boulder facing attempt for player %d failed: %s", charID, result.Message)
			}
		}
	}

	if reportedMove {
		handleClientReportedStepEffects(ses, wh, int64(char.ID), req.X, req.Y, mapID, direction, mapChanged, prevMapID)
	}

	ridingBicycle := wh.PlayerMovement != nil && wh.PlayerMovement.IsBicycleActive(int(char.ID))
	surfing := wh.PlayerMovement != nil && wh.PlayerMovement.IsSurfing(int(char.ID))
	spriteName := playerSpriteName(char.Gender, ridingBicycle, surfing)

	both := "BOTH"
	playerActor := PhaserActor{
		ID:              wh.ActorRegistry.GetPhaserID(ActorTypePlayer, int(char.ID)),
		InternalID:      int(char.ID),
		X:               &req.X,
		Y:               &req.Y,
		MapID:           mapID,
		ObjectType:      "player",
		SpriteName:      &spriteName,
		Name:            &char.Name,
		ActionType:      nil,
		ActionDirection: &direction,
		MovementType:    &both,
		MoveSpeed:       wh.PlayerMovement.GetMoveSpeed(int(char.ID)),
	}

	if mapChanged {
		broadcastPlayerActorVisibleMapChange(ses, wh, prevMapID, &playerActor)
	} else {
		wh.ActorManager.broadcastActorUpdate(&playerActor, ses.SessionID)
	}

	return false
}

func handleClientReportedStepEffects(ses *session.Session, wh *WorldHandler, charID int64, x, y, mapID int, direction string, mapChanged bool, previousMapID int) {
	if ses == nil || wh == nil {
		return
	}
	if mapChanged {
		endSafariSessionIfLeavingMap(charID, previousMapID, mapID, wh)
	}
	state := &PlayerMovementState{
		SessionID:   ses.SessionID,
		CharacterID: int(charID),
		CurrentX:    x,
		CurrentY:    y,
		MapID:       mapID,
		Direction:   direction,
	}

	if wh.TrainerEncounter != nil && wh.TrainerEncounter.CheckPlayerPosition(charID, x, y, mapID, ses) {
		return
	}

	TickDayCareStep(charID)

	if wh.Safari != nil && IsInSafariZone(mapID) {
		CheckSafariStep(charID, x, y, mapID, ses, wh)
		return
	}

	if wh.PlayerMovement != nil && wh.PlayerMovement.tryTriggerCoordinateCutscene(state, charID, ses) {
		return
	}

	testModeSuppressesRandomEncounters :=
		os.Getenv("CAPTUREQUEST_TEST_MODE") == "true" &&
			os.Getenv("CAPTUREQUEST_TEST_RANDOM_ENCOUNTERS") != "true"
	if wh.WildEncounter != nil &&
		!testModeSuppressesRandomEncounters &&
		(wh.PlayerMovement == nil || !wh.PlayerMovement.isWildEncounterSuppressed(state, charID)) {
		wh.WildEncounter.CheckPlayerStep(charID, x, y, mapID, ses)
	}
}

// SendPlayerSpawn sends the player's initial position and sprite to the client
// and broadcasts their presence to other players in the same map/overworld.
func SendPlayerSpawn(ses *session.Session, wh *WorldHandler) {
	if ses.Client == nil {
		return
	}
	char := ses.Client.CharData()
	if char == nil {
		return
	}
	recoverInvalidCharacterPosition(ses, wh)
	char = ses.Client.CharData()
	if char == nil {
		return
	}

	// Ensure the session's map ID is synced with the character's map position
	// before we create the actor or start broadcasting.
	mapID := int(char.MapID)
	if wh.ActorManager.IsOverworld(mapID) {
		mapID = UnifiedOverworldMapID
	}
	ses.MapID = mapID

	playerActor := createPlayerActor(ses, wh)
	if playerActor == nil {
		return
	}
	if playerActor.X != nil && playerActor.Y != nil {
		ses.X = float32(*playerActor.X)
		ses.Y = float32(*playerActor.Y)
		ses.MapID = playerActor.MapID
	}

	// 1. Send to the player themselves
	ses.SendStreamJSON(StructToMap([]PhaserActor{*playerActor}), opcodes.PhaserActorsResponse)
	log.Printf("[Phaser] Sent player spawn for %s at (%d, %d)", char.Name, *playerActor.X, *playerActor.Y)

	// 2. Broadcast to other players so they see this player immediately
	wh.ActorManager.broadcastActorSpawn(playerActor, ses.SessionID)
}

// createPlayerActor creates a PhaserActor representation of the session's player
func createPlayerActor(ses *session.Session, wh *WorldHandler) *PhaserActor {
	if ses.Client == nil {
		return nil
	}
	char := ses.Client.CharData()
	if char == nil {
		return nil
	}

	// Use stored position from character data
	spawnX := int(char.X)
	spawnY := int(char.Y)
	storedMapID := int(char.MapID)
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

	ridingBicycle := wh.PlayerMovement != nil && wh.PlayerMovement.IsBicycleActive(int(char.ID))
	surfing := wh.PlayerMovement != nil && wh.PlayerMovement.IsSurfing(int(char.ID))
	if !surfing && wh.ActorManager != nil {
		if collisionType, exists := wh.ActorManager.CollisionTypeAt(mapID, spawnX, spawnY); exists && collisionType == collisionWater {
			surfing = true
		}
	}
	spriteName := playerSpriteName(char.Gender, ridingBicycle, surfing)

	objectType := "player"
	stay := "STAY"
	direction := initialPlayerDirection(storedMapID, spawnX, spawnY)
	both := "BOTH"

	return &PhaserActor{
		ID:              wh.ActorRegistry.GetPhaserID(ActorTypePlayer, int(char.ID)),
		InternalID:      int(char.ID),
		X:               &spawnX,
		Y:               &spawnY,
		MapID:           mapID,
		ObjectType:      objectType,
		SpriteName:      &spriteName,
		Name:            &char.Name,
		ActionType:      &stay,
		ActionDirection: &direction,
		MovementType:    &both,
		MoveSpeed:       wh.PlayerMovement.GetMoveSpeed(int(char.ID)),
	}
}

func isInvalidZeroPlayerPosition(x, y int) bool {
	return x == 0 && y == 0
}

func recoverInvalidCharacterPosition(ses *session.Session, wh *WorldHandler) bool {
	if ses == nil || !ses.HasValidClient() {
		return false
	}
	char := ses.Client.CharData()
	if char == nil || !isInvalidZeroPlayerPosition(int(char.X), int(char.Y)) {
		return false
	}

	mapID := RecoverySpawnMap
	x := int(RecoverySpawnX)
	y := int(RecoverySpawnY)
	sessionMapID := mapID
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		sessionMapID = UnifiedOverworldMapID
	}

	log.Printf("[Phaser] Recovered invalid saved position for player %d to map %d (%d,%d)",
		char.ID, mapID, x, y)
	char.X = RecoverySpawnX
	char.Y = RecoverySpawnY
	char.Z = RecoverySpawnZ
	char.MapID = uint32(sessionMapID)
	ses.X = float32(x)
	ses.Y = float32(y)
	ses.MapID = sessionMapID

	if db.GlobalWorldDB != nil && db.GlobalWorldDB.DB != nil {
		if err := db_character.UpdateCharacterPosition(
			int32(char.ID),
			uint32(mapID),
			RecoverySpawnX,
			RecoverySpawnY,
			RecoverySpawnZ,
			0,
		); err != nil {
			log.Printf("[Phaser] Failed to persist recovered position for player %d: %v", char.ID, err)
		}
	}

	if wh != nil && wh.PlayerMovement != nil {
		wh.PlayerMovement.UpdatePosition(int(char.ID), x, y, sessionMapID, RecoverySpawnDirection)
	}
	return true
}

// RegisterPlayerForMovement registers a player with the movement manager when they spawn
func RegisterPlayerForMovement(ses *session.Session, wh *WorldHandler) {
	if !ses.HasValidClient() {
		return
	}
	char := ses.Client.CharData()
	if char == nil {
		return
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
}
