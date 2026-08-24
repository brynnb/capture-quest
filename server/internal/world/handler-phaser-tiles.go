package world

import (
	"database/sql"
	"encoding/json"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
)

// --- Request/Response types ---

// TileEdit represents a single tile placement or erasure
type TileEdit struct {
	X             int  `json:"x"`
	Y             int  `json:"y"`
	TileImageID   int  `json:"tileImageId"`
	CollisionType int  `json:"collisionType"`
	RawFootTileID *int `json:"rawFootTileId,omitempty"`
	TalkOverTile  bool `json:"talkOverTile,omitempty"`
	Erased        bool `json:"erased,omitempty"`
}

// TileEditorPlaceReq is the request payload for placing tiles
type TileEditorPlaceReq struct {
	Tiles []TileEdit `json:"tiles"`
	MapID int        `json:"mapId"`
}

// TileEditorEraseReq is the request payload for erasing tiles
type TileEditorEraseReq struct {
	Tiles []struct {
		X int `json:"x"`
		Y int `json:"y"`
	} `json:"tiles"`
	MapID int `json:"mapId"`
}

// TileEditorFillReq is the request payload for flood-filling
type TileEditorFillReq struct {
	X           int `json:"x"`
	Y           int `json:"y"`
	TileImageID int `json:"tileImageId"`
	MapID       int `json:"mapId"`
}

// TileEditorUndoReq is the request payload for undoing a tile action
type TileEditorUndoReq struct {
	Tiles []TileEdit `json:"tiles"` // The old tile states to restore
	MapID int        `json:"mapId"`
}

// TileEditorBroadcastPayload is sent to all clients when tiles change
type TileEditorBroadcastPayload struct {
	Tiles []TileEdit `json:"tiles"`
	MapID int        `json:"mapId"`
}

// TileProperty represents a tile type's metadata
type TileProperty struct {
	TileImageID    int    `json:"tileImageId"`
	Name           string `json:"name"`
	CollisionType  int    `json:"collisionType"`
	IsUserEditable int    `json:"isUserEditable"`
	RawFootTileID  *int   `json:"rawFootTileId,omitempty"`
	TalkOverTile   bool   `json:"talkOverTile"`
}

type tileRuntimeProperties struct {
	CollisionType int
	RawFootTileID *int
	TalkOverTile  bool
}

func tileRuntimePropertiesForTileImage(tileImageID int) tileRuntimeProperties {
	var (
		collisionType int
		rawFootTileID sql.NullInt64
		talkOverTile  bool
	)
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT COALESCE(tp.collision_type, 0), ti.raw_foot_tile_id, COALESCE(ti.talk_over_tile, FALSE)
		FROM phaser_tile_properties tp
		LEFT JOIN phaser_tile_images ti ON ti.id = tp.tile_image_id
		WHERE tp.tile_image_id = $1`,
		tileImageID,
	).Scan(&collisionType, &rawFootTileID, &talkOverTile); err != nil {
		return tileRuntimeProperties{}
	}
	props := tileRuntimeProperties{
		CollisionType: collisionType,
		TalkOverTile:  talkOverTile,
	}
	if rawFootTileID.Valid {
		v := int(rawFootTileID.Int64)
		props.RawFootTileID = &v
	}
	return props
}

func rawFootTileIDForTileImage(tileImageID int) *int {
	props := tileRuntimePropertiesForTileImage(tileImageID)
	return props.RawFootTileID
}

// TilePropertyUpdateReq is the request payload for updating tile properties
type TilePropertyUpdateReq struct {
	TileImageID   int    `json:"tileImageId"`
	Name          string `json:"name,omitempty"`
	CollisionType *int   `json:"collisionType,omitempty"`
}

// Maximum tiles per single request to prevent abuse
const maxTilesPerRequest = 500

// Maximum tiles affected by a single fill operation — if BFS exceeds this, abort entirely
const maxFillTiles = 250

// --- Handlers ---

// HandleTileEditorPlace handles placing/painting tiles on the map
func HandleTileEditorPlace(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req TileEditorPlaceReq
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[TileEditor] Invalid PlaceRequest: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.TileEditorPlaceResponse)
		return false
	}
	if !canAdminEditWorldTiles(ses) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "not authorized"}, opcodes.TileEditorPlaceResponse)
		return false
	}

	if len(req.Tiles) == 0 || len(req.Tiles) > maxTilesPerRequest {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid tile count"}, opcodes.TileEditorPlaceResponse)
		return false
	}

	result, err := applyWorldTilePlacements(ses, wh, req.MapID, req.Tiles, "admin_editor")
	if err != nil {
		log.Printf("[TileEditor] Place failed: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.TileEditorPlaceResponse)
		return false
	}

	ses.SendStreamJSON(map[string]interface{}{"success": true, "placed": result.Changed}, opcodes.TileEditorPlaceResponse)

	if len(result.Edits) > 0 {
		broadcastTileChanges(wh, result.Edits, result.MapID, ses.SessionID)
	}

	log.Printf("[TileEditor] Placed %d tiles on map %d by char %d", result.Changed, req.MapID, characterIDForTileMutation(ses))
	return false
}

// HandleTileEditorErase handles erasing tiles from the map
func HandleTileEditorErase(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req TileEditorEraseReq
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[TileEditor] Invalid EraseRequest: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.TileEditorEraseResponse)
		return false
	}
	if !canAdminEditWorldTiles(ses) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "not authorized"}, opcodes.TileEditorEraseResponse)
		return false
	}

	if len(req.Tiles) == 0 || len(req.Tiles) > maxTilesPerRequest {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid tile count"}, opcodes.TileEditorEraseResponse)
		return false
	}

	coords := make([]tileCoord, 0, len(req.Tiles))
	for _, tile := range req.Tiles {
		coords = append(coords, tileCoord{X: tile.X, Y: tile.Y})
	}
	result, err := applyWorldTileErasures(ses, wh, req.MapID, coords, "admin_editor")
	if err != nil {
		log.Printf("[TileEditor] Erase failed: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.TileEditorEraseResponse)
		return false
	}

	ses.SendStreamJSON(map[string]interface{}{"success": true, "erased": result.Changed}, opcodes.TileEditorEraseResponse)

	if len(result.Edits) > 0 {
		broadcastTileChanges(wh, result.Edits, result.MapID, ses.SessionID)
	}

	log.Printf("[TileEditor] Erased %d tiles on map %d", result.Changed, req.MapID)
	return false
}

// HandleTileEditorFill handles flood-filling a region with a tile type
func HandleTileEditorFill(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req TileEditorFillReq
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[TileEditor] Invalid FillRequest: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.TileEditorFillResponse)
		return false
	}
	if !canAdminEditWorldTiles(ses) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "not authorized"}, opcodes.TileEditorFillResponse)
		return false
	}

	ctx, err := newWorldTileMutationContext(ses, wh, req.MapID, "admin_editor")
	if err != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.TileEditorFillResponse)
		return false
	}
	if err := validateTileImagesExist([]int{req.TileImageID}); err != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.TileEditorFillResponse)
		return false
	}

	// Determine what tile type is at the target position (or "empty" if none)
	targetTileImageID, hasTargetTile, err := currentWorldTileImageID(ctx.MapID, req.X, req.Y)
	if err != nil {
		log.Printf("[TileEditor] Fill target lookup failed: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorFillResponse)
		return false
	}
	isEmptyFill := !hasTargetTile

	// Don't fill if the target is already the same tile type
	if !isEmptyFill && targetTileImageID == req.TileImageID {
		ses.SendStreamJSON(map[string]interface{}{"success": true, "filled": 0}, opcodes.TileEditorFillResponse)
		return false
	}

	var mapWidth, mapHeight int
	if ctx.MapID != UnifiedOverworldMapID {
		if err := db.GlobalWorldDB.DB.QueryRow(`SELECT width, height FROM phaser_maps WHERE id = $1`, ctx.MapID).Scan(&mapWidth, &mapHeight); err != nil {
			log.Printf("[TileEditor] Fill bounds lookup failed: %v", err)
			ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorFillResponse)
			return false
		}
	}

	// BFS flood fill
	type point struct{ x, y int }
	queue := []point{{req.X, req.Y}}
	visited := make(map[point]bool)
	visited[point{req.X, req.Y}] = true
	var fillPoints []point

	// Build a set of existing tiles for fast lookup during empty-space fill
	var existingTiles map[point]int
	if isEmptyFill {
		existingTiles = make(map[point]int)
		var rows *sql.Rows
		var err error
		if ctx.MapID == UnifiedOverworldMapID {
			rows, err = db.GlobalWorldDB.DB.Query(`
				SELECT x, y, tile_image_id FROM phaser_tiles
				WHERE map_id IS NULL
				  AND is_tile_erased = 0
				  AND x BETWEEN $1 AND $2 AND y BETWEEN $3 AND $4`,
				req.X-maxFillTiles, req.X+maxFillTiles, req.Y-maxFillTiles, req.Y+maxFillTiles)
		} else {
			rows, err = db.GlobalWorldDB.DB.Query(`
				SELECT x, y, tile_image_id FROM phaser_tiles
				WHERE map_id = $1
				  AND is_tile_erased = 0
				  AND x BETWEEN $2 AND $3 AND y BETWEEN $4 AND $5`,
				ctx.MapID, req.X-maxFillTiles, req.X+maxFillTiles, req.Y-maxFillTiles, req.Y+maxFillTiles)
		}
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var x, y, tid int
				rows.Scan(&x, &y, &tid)
				existingTiles[point{x, y}] = tid
			}
		}
	}

	fillAborted := false
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if ctx.MapID != UnifiedOverworldMapID &&
			(current.x < 0 || current.y < 0 || current.x >= mapWidth || current.y >= mapHeight) {
			continue
		}

		if isEmptyFill {
			// For empty fill: only fill positions that have no tile
			if _, hasTile := existingTiles[current]; hasTile {
				continue
			}
		} else {
			// For same-tile fill: only fill positions that match the target tile type
			currentTileID, ok, err := currentWorldTileImageID(ctx.MapID, current.x, current.y)
			if err != nil || !ok || currentTileID != targetTileImageID {
				continue
			}
		}

		fillPoints = append(fillPoints, current)

		// If we exceed the limit, abort the entire fill
		if len(fillPoints) > maxFillTiles {
			fillAborted = true
			break
		}

		// Check 4 neighbors
		for _, dir := range []point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
			next := point{current.x + dir.x, current.y + dir.y}
			if !visited[next] {
				visited[next] = true
				queue = append(queue, next)
			}
		}
	}

	if fillAborted {
		log.Printf("[TileEditor] Fill aborted: area exceeds %d tile limit", maxFillTiles)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "area too large (max 250 tiles)"}, opcodes.TileEditorFillResponse)
		return false
	}

	if len(fillPoints) == 0 {
		ses.SendStreamJSON(map[string]interface{}{"success": true, "filled": 0}, opcodes.TileEditorFillResponse)
		return false
	}

	placeTiles := make([]TileEdit, 0, len(fillPoints))
	for _, p := range fillPoints {
		placeTiles = append(placeTiles, TileEdit{X: p.x, Y: p.y, TileImageID: req.TileImageID})
	}

	result, err := applyWorldTilePlacements(ses, wh, ctx.MapID, placeTiles, "admin_editor")
	if err != nil {
		log.Printf("[TileEditor] Fill failed: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.TileEditorFillResponse)
		return false
	}

	ses.SendStreamJSON(map[string]interface{}{"success": true, "filled": result.Changed}, opcodes.TileEditorFillResponse)

	if len(result.Edits) > 0 {
		broadcastTileChanges(wh, result.Edits, result.MapID, ses.SessionID)
	}

	log.Printf("[TileEditor] Filled %d tiles on map %d by char %d", result.Changed, ctx.MapID, characterIDForTileMutation(ses))
	return false
}

// HandleTileEditorUndo handles undoing a tile action by restoring old tile states
func HandleTileEditorUndo(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req TileEditorUndoReq
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[TileEditor] Invalid UndoRequest: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.TileEditorUndoResponse)
		return false
	}
	if !canAdminEditWorldTiles(ses) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "not authorized"}, opcodes.TileEditorUndoResponse)
		return false
	}

	if len(req.Tiles) == 0 || len(req.Tiles) > maxTilesPerRequest {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid tile count"}, opcodes.TileEditorUndoResponse)
		return false
	}

	var placeTiles []TileEdit
	var eraseCoords []tileCoord
	for _, tile := range req.Tiles {
		if tile.TileImageID == 0 {
			eraseCoords = append(eraseCoords, tileCoord{X: tile.X, Y: tile.Y})
		} else {
			placeTiles = append(placeTiles, tile)
		}
	}

	var broadcastEdits []TileEdit
	restored := 0
	broadcastMapID := req.MapID
	if len(eraseCoords) > 0 {
		result, err := applyWorldTileErasures(ses, wh, req.MapID, eraseCoords, "admin_editor")
		if err != nil {
			log.Printf("[TileEditor] Undo erase failed: %v", err)
			ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.TileEditorUndoResponse)
			return false
		}
		broadcastEdits = append(broadcastEdits, result.Edits...)
		restored += result.Changed
		broadcastMapID = result.MapID
	}
	if len(placeTiles) > 0 {
		result, err := applyWorldTilePlacements(ses, wh, req.MapID, placeTiles, "admin_editor")
		if err != nil {
			log.Printf("[TileEditor] Undo place failed: %v", err)
			ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.TileEditorUndoResponse)
			return false
		}
		broadcastEdits = append(broadcastEdits, result.Edits...)
		restored += result.Changed
		broadcastMapID = result.MapID
	}

	ses.SendStreamJSON(map[string]interface{}{"success": true, "restored": restored}, opcodes.TileEditorUndoResponse)

	if len(broadcastEdits) > 0 {
		broadcastTileChanges(wh, broadcastEdits, broadcastMapID, ses.SessionID)
	}

	log.Printf("[TileEditor] Undid %d tiles on map %d", restored, req.MapID)
	return false
}

// HandleTilePropertiesRequest returns all tile properties for the palette
func HandleTilePropertiesRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT tp.tile_image_id, tp.name, tp.collision_type, tp.is_user_editable, ti.raw_foot_tile_id, COALESCE(ti.talk_over_tile, FALSE)
		FROM phaser_tile_properties tp
		LEFT JOIN phaser_tile_images ti ON ti.id = tp.tile_image_id
		ORDER BY tp.tile_image_id`)
	if err != nil {
		log.Printf("[TileEditor] Error querying tile properties: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.TilePropertiesResponse)
		return false
	}
	defer rows.Close()

	var props []TileProperty
	for rows.Next() {
		var p TileProperty
		var rawFootTileID sql.NullInt64
		if err := rows.Scan(&p.TileImageID, &p.Name, &p.CollisionType, &p.IsUserEditable, &rawFootTileID, &p.TalkOverTile); err != nil {
			log.Printf("[TileEditor] Error scanning tile property: %v", err)
			continue
		}
		if rawFootTileID.Valid {
			v := int(rawFootTileID.Int64)
			p.RawFootTileID = &v
		}
		props = append(props, p)
	}

	ses.SendStreamJSON(StructToMap(props), opcodes.TilePropertiesResponse)
	log.Printf("[TileEditor] Sent %d tile properties", len(props))
	return false
}

// HandleTilePropertyUpdate handles updating a tile property (name, collision type)
func HandleTilePropertyUpdate(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req TilePropertyUpdateReq
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[TileEditor] Invalid TilePropertyUpdateRequest: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid request"}, opcodes.TilePropertyUpdateResponse)
		return false
	}
	if !canAdminEditWorldTiles(ses) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "not authorized"}, opcodes.TilePropertyUpdateResponse)
		return false
	}

	if req.Name != "" {
		_, err := db.GlobalWorldDB.DB.Exec(
			`UPDATE phaser_tile_properties SET name = $1 WHERE tile_image_id = $2`,
			req.Name, req.TileImageID)
		if err != nil {
			log.Printf("[TileEditor] Error updating tile name: %v", err)
		}
	}

	if req.CollisionType != nil {
		_, err := db.GlobalWorldDB.DB.Exec(
			`UPDATE phaser_tile_properties SET collision_type = $1 WHERE tile_image_id = $2`,
			*req.CollisionType, req.TileImageID)
		if err != nil {
			log.Printf("[TileEditor] Error updating tile collision: %v", err)
		}

		// Also update all existing tiles with this image to the new collision type
		_, err = db.GlobalWorldDB.DB.Exec(
			`UPDATE phaser_tiles SET collision_type = $1 WHERE tile_image_id = $2 AND is_tile_erased = 0`,
			*req.CollisionType, req.TileImageID)
		if err != nil {
			log.Printf("[TileEditor] Error bulk-updating tile collision: %v", err)
		}
		_, err = db.GlobalWorldDB.DB.Exec(
			`UPDATE phaser_tiles SET original_collision_type = $1 WHERE original_tile_image_id = $2`,
			*req.CollisionType, req.TileImageID)
		if err != nil {
			log.Printf("[TileEditor] Error bulk-updating original tile collision: %v", err)
		}
	}

	ses.SendStreamJSON(map[string]interface{}{"success": true}, opcodes.TilePropertyUpdateResponse)
	log.Printf("[TileEditor] Updated properties for tile_image_id %d", req.TileImageID)
	return false
}

// --- Broadcast helper ---

// broadcastTileChanges sends tile updates to all clients on the same map (including the originator)
func broadcastTileChanges(wh *WorldHandler, tiles []TileEdit, mapID int, originSessionID int) {
	if wh == nil || wh.sessionManager == nil {
		return
	}

	payload := TileEditorBroadcastPayload{
		Tiles: tiles,
		MapID: mapID,
	}

	data := StructToMap(payload)

	isOverworld := mapID == UnifiedOverworldMapID

	wh.sessionManager.ForEachSession(func(ses *session.Session) {
		if !ses.Authenticated {
			return
		}

		// Check if the player is on the same map
		playerOnOverworld := ses.MapID == UnifiedOverworldMapID || (wh.ActorManager != nil && wh.ActorManager.IsOverworld(ses.MapID))
		if ses.MapID == mapID || (isOverworld && playerOnOverworld) {
			ses.SendStreamJSON(data, opcodes.TileEditorBroadcast)
		}
	})
}
