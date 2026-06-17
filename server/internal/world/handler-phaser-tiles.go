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

	if len(req.Tiles) == 0 || len(req.Tiles) > maxTilesPerRequest {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid tile count"}, opcodes.TileEditorPlaceResponse)
		return false
	}

	// Only allow overworld edits for now
	if req.MapID != UnifiedOverworldMapID {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "only overworld edits allowed"}, opcodes.TileEditorPlaceResponse)
		return false
	}

	// Get character ID for tracking who placed tiles
	var charID int64
	if ses.HasValidClient() {
		charID = int64(ses.Client.CharData().ID)
	}

	// Look up runtime tile metadata from tile properties/images.
	propertiesCache := make(map[int]tileRuntimeProperties)

	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		log.Printf("[TileEditor] Failed to begin transaction: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorPlaceResponse)
		return false
	}

	placed := 0
	for _, tile := range req.Tiles {
		props, ok := propertiesCache[tile.TileImageID]
		if !ok {
			props = tileRuntimePropertiesForTileImage(tile.TileImageID)
			propertiesCache[tile.TileImageID] = props
		}

		// Upsert into phaser_tiles — overworld tiles have map_id = NULL
		_, err := tx.Exec(`
			INSERT INTO phaser_tiles (x, y, tile_image_id, map_id, collision_type, raw_foot_tile_id, talk_over_tile, is_user_placed, placed_by_char_id, placed_at)
			VALUES ($1, $2, $3, NULL, $4, $5, $6, 1, $7, CURRENT_TIMESTAMP)
			ON CONFLICT (x, y, COALESCE(map_id, -1)) DO UPDATE SET
				tile_image_id = EXCLUDED.tile_image_id,
				collision_type = EXCLUDED.collision_type,
				raw_foot_tile_id = EXCLUDED.raw_foot_tile_id,
				talk_over_tile = EXCLUDED.talk_over_tile,
				is_user_placed = 1,
				placed_by_char_id = EXCLUDED.placed_by_char_id,
				placed_at = CURRENT_TIMESTAMP`,
			tile.X, tile.Y, tile.TileImageID, props.CollisionType, props.RawFootTileID, props.TalkOverTile, charID)
		if err != nil {
			log.Printf("[TileEditor] Error upserting tile at (%d,%d): %v", tile.X, tile.Y, err)
			continue
		}

		placed++
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[TileEditor] Failed to commit transaction: %v", err)
		tx.Rollback()
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorPlaceResponse)
		return false
	}

	// Invalidate server collision map so pathfinding reloads from DB
	wh.ActorManager.InvalidateCollisionMap(req.MapID)

	// Build broadcast edits with collision types
	broadcastEdits := make([]TileEdit, 0, len(req.Tiles))
	for _, tile := range req.Tiles {
		broadcastEdits = append(broadcastEdits, TileEdit{
			X: tile.X, Y: tile.Y,
			TileImageID:   tile.TileImageID,
			CollisionType: propertiesCache[tile.TileImageID].CollisionType,
			RawFootTileID: propertiesCache[tile.TileImageID].RawFootTileID,
			TalkOverTile:  propertiesCache[tile.TileImageID].TalkOverTile,
		})
	}

	// Send success to the requesting client
	ses.SendStreamJSON(map[string]interface{}{"success": true, "placed": placed}, opcodes.TileEditorPlaceResponse)

	// Broadcast tile changes to all clients on this map
	broadcastTileChanges(wh, broadcastEdits, req.MapID, ses.SessionID)

	log.Printf("[TileEditor] Placed %d tiles on map %d by char %d", placed, req.MapID, charID)
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

	if len(req.Tiles) == 0 || len(req.Tiles) > maxTilesPerRequest {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid tile count"}, opcodes.TileEditorEraseResponse)
		return false
	}

	if req.MapID != UnifiedOverworldMapID {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "only overworld edits allowed"}, opcodes.TileEditorEraseResponse)
		return false
	}

	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		log.Printf("[TileEditor] Failed to begin transaction: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorEraseResponse)
		return false
	}

	erased := 0
	var broadcastEdits []TileEdit
	for _, tile := range req.Tiles {
		// Delete from phaser_tiles — overworld tiles have map_id IS NULL
		result, err := tx.Exec(`
			DELETE FROM phaser_tiles WHERE x = $1 AND y = $2 AND map_id IS NULL`,
			tile.X, tile.Y)
		if err != nil {
			log.Printf("[TileEditor] Error deleting tile at (%d,%d): %v", tile.X, tile.Y, err)
			continue
		}
		rows, _ := result.RowsAffected()
		if rows > 0 {
			erased++
			// TileImageID=0 signals erasure to clients
			broadcastEdits = append(broadcastEdits, TileEdit{X: tile.X, Y: tile.Y, TileImageID: 0})
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[TileEditor] Failed to commit erase transaction: %v", err)
		tx.Rollback()
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorEraseResponse)
		return false
	}

	// Invalidate server collision map so pathfinding reloads from DB
	wh.ActorManager.InvalidateCollisionMap(req.MapID)

	ses.SendStreamJSON(map[string]interface{}{"success": true, "erased": erased}, opcodes.TileEditorEraseResponse)

	if len(broadcastEdits) > 0 {
		broadcastTileChanges(wh, broadcastEdits, req.MapID, ses.SessionID)
	}

	log.Printf("[TileEditor] Erased %d tiles on map %d", erased, req.MapID)
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

	if req.MapID != UnifiedOverworldMapID {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "only overworld edits allowed"}, opcodes.TileEditorFillResponse)
		return false
	}

	// Determine what tile type is at the target position (or "empty" if none)
	var targetTileImageID int
	var isEmptyFill bool
	// Overworld tiles have map_id IS NULL
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT tile_image_id FROM phaser_tiles
		WHERE x = $1 AND y = $2 AND map_id IS NULL`,
		req.X, req.Y).Scan(&targetTileImageID)
	if err != nil {
		// No tile at this position — filling empty space
		isEmptyFill = true
		targetTileImageID = 0
	}

	// Don't fill if the target is already the same tile type
	if !isEmptyFill && targetTileImageID == req.TileImageID {
		ses.SendStreamJSON(map[string]interface{}{"success": true, "filled": 0}, opcodes.TileEditorFillResponse)
		return false
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
		// Load all overworld tiles (map_id IS NULL) in a bounding box
		rows, err := db.GlobalWorldDB.DB.Query(`
			SELECT x, y, tile_image_id FROM phaser_tiles
			WHERE map_id IS NULL AND x BETWEEN $1 AND $2 AND y BETWEEN $3 AND $4`,
			req.X-maxFillTiles, req.X+maxFillTiles, req.Y-maxFillTiles, req.Y+maxFillTiles)
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

		if isEmptyFill {
			// For empty fill: only fill positions that have no tile
			if _, hasTile := existingTiles[current]; hasTile {
				continue
			}
		} else {
			// For same-tile fill: only fill positions that match the target tile type
			var currentTileID int
			err := db.GlobalWorldDB.DB.QueryRow(`
				SELECT tile_image_id FROM phaser_tiles
				WHERE x = $1 AND y = $2 AND map_id IS NULL`,
				current.x, current.y).Scan(&currentTileID)
			if err != nil || currentTileID != targetTileImageID {
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

	// Get character ID
	var charID int64
	if ses.HasValidClient() {
		charID = int64(ses.Client.CharData().ID)
	}

	props := tileRuntimePropertiesForTileImage(req.TileImageID)

	// Execute the fill in a transaction
	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		log.Printf("[TileEditor] Failed to begin fill transaction: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorFillResponse)
		return false
	}

	var broadcastEdits []TileEdit
	for _, p := range fillPoints {
		_, err := tx.Exec(`
			INSERT INTO phaser_tiles (x, y, tile_image_id, map_id, collision_type, raw_foot_tile_id, talk_over_tile, is_user_placed, placed_by_char_id, placed_at)
			VALUES ($1, $2, $3, NULL, $4, $5, $6, 1, $7, CURRENT_TIMESTAMP)
			ON CONFLICT (x, y, COALESCE(map_id, -1)) DO UPDATE SET
				tile_image_id = EXCLUDED.tile_image_id,
				collision_type = EXCLUDED.collision_type,
				raw_foot_tile_id = EXCLUDED.raw_foot_tile_id,
				talk_over_tile = EXCLUDED.talk_over_tile,
				is_user_placed = 1,
				placed_by_char_id = EXCLUDED.placed_by_char_id,
				placed_at = CURRENT_TIMESTAMP`,
			p.x, p.y, req.TileImageID, props.CollisionType, props.RawFootTileID, props.TalkOverTile, charID)
		if err != nil {
			log.Printf("[TileEditor] Error filling tile at (%d,%d): %v", p.x, p.y, err)
			continue
		}

		broadcastEdits = append(broadcastEdits, TileEdit{
			X:             p.x,
			Y:             p.y,
			TileImageID:   req.TileImageID,
			CollisionType: props.CollisionType,
			RawFootTileID: props.RawFootTileID,
			TalkOverTile:  props.TalkOverTile,
		})
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[TileEditor] Failed to commit fill transaction: %v", err)
		tx.Rollback()
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorFillResponse)
		return false
	}

	// Invalidate server collision map so pathfinding reloads from DB
	wh.ActorManager.InvalidateCollisionMap(req.MapID)

	ses.SendStreamJSON(map[string]interface{}{"success": true, "filled": len(broadcastEdits)}, opcodes.TileEditorFillResponse)

	if len(broadcastEdits) > 0 {
		broadcastTileChanges(wh, broadcastEdits, req.MapID, ses.SessionID)
	}

	log.Printf("[TileEditor] Filled %d tiles on map %d by char %d", len(broadcastEdits), req.MapID, charID)
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

	if len(req.Tiles) == 0 || len(req.Tiles) > maxTilesPerRequest {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "invalid tile count"}, opcodes.TileEditorUndoResponse)
		return false
	}

	if req.MapID != UnifiedOverworldMapID {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "only overworld edits allowed"}, opcodes.TileEditorUndoResponse)
		return false
	}

	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		log.Printf("[TileEditor] Failed to begin undo transaction: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorUndoResponse)
		return false
	}

	restored := 0
	var broadcastEdits []TileEdit
	for _, tile := range req.Tiles {
		props := tileRuntimeProperties{}
		if tile.TileImageID == 0 {
			// Restore to empty — delete the tile (overworld: map_id IS NULL)
			tx.Exec(`DELETE FROM phaser_tiles WHERE x = $1 AND y = $2 AND map_id IS NULL`,
				tile.X, tile.Y)
		} else {
			// Restore to previous tile
			props = tileRuntimePropertiesForTileImage(tile.TileImageID)

			tx.Exec(`
				INSERT INTO phaser_tiles (x, y, tile_image_id, map_id, collision_type, raw_foot_tile_id, talk_over_tile, is_user_placed, placed_at)
				VALUES ($1, $2, $3, NULL, $4, $5, $6, 1, CURRENT_TIMESTAMP)
				ON CONFLICT (x, y, COALESCE(map_id, -1)) DO UPDATE SET
					tile_image_id = EXCLUDED.tile_image_id,
					collision_type = EXCLUDED.collision_type,
					raw_foot_tile_id = EXCLUDED.raw_foot_tile_id,
					talk_over_tile = EXCLUDED.talk_over_tile,
					placed_at = CURRENT_TIMESTAMP`,
				tile.X, tile.Y, tile.TileImageID, props.CollisionType, props.RawFootTileID, props.TalkOverTile)
		}
		broadcastEdits = append(broadcastEdits, TileEdit{
			X: tile.X, Y: tile.Y,
			TileImageID:   tile.TileImageID,
			CollisionType: props.CollisionType,
			RawFootTileID: props.RawFootTileID,
			TalkOverTile:  props.TalkOverTile,
		})
		restored++
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[TileEditor] Failed to commit undo transaction: %v", err)
		tx.Rollback()
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "database error"}, opcodes.TileEditorUndoResponse)
		return false
	}

	// Invalidate server collision map so pathfinding reloads from DB
	wh.ActorManager.InvalidateCollisionMap(req.MapID)

	ses.SendStreamJSON(map[string]interface{}{"success": true, "restored": restored}, opcodes.TileEditorUndoResponse)

	if len(broadcastEdits) > 0 {
		broadcastTileChanges(wh, broadcastEdits, req.MapID, ses.SessionID)
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
			`UPDATE phaser_tiles SET collision_type = $1 WHERE tile_image_id = $2`,
			*req.CollisionType, req.TileImageID)
		if err != nil {
			log.Printf("[TileEditor] Error bulk-updating tile collision: %v", err)
		}
	}

	ses.SendStreamJSON(map[string]interface{}{"success": true}, opcodes.TilePropertyUpdateResponse)
	log.Printf("[TileEditor] Updated properties for tile_image_id %d", req.TileImageID)
	return false
}

// --- Broadcast helper ---

// broadcastTileChanges sends tile updates to all clients on the same map (including the originator)
func broadcastTileChanges(wh *WorldHandler, tiles []TileEdit, mapID int, originSessionID int) {
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
		playerOnOverworld := ses.MapID == UnifiedOverworldMapID
		if ses.MapID == mapID || (isOverworld && playerOnOverworld) {
			ses.SendStreamJSON(data, opcodes.TileEditorBroadcast)
		}
	})
}
