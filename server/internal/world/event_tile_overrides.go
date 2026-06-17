package world

import (
	"database/sql"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
)

type EventTileState struct {
	X             int
	Y             int
	TileImageID   int
	CollisionType int
	RawFootTileID *int
	TalkOverTile  bool
	Label         string
}

type eventTileOverride struct {
	X                  int
	Y                  int
	TileImageID        int
	CollisionType      int
	RequiresFlag       sql.NullString
	RequiresFlagAbsent sql.NullString
	Label              sql.NullString
}

func ApplyEventTileOverridesToTiles(charID int64, mapID int, efm *EventFlagManager, tiles []PhaserTile) []PhaserTile {
	if len(tiles) == 0 {
		return tiles
	}
	overrides, err := eventTileOverridesForMap(mapID)
	if err != nil {
		log.Printf("[EventTiles] Failed to load overrides for map %d: %v", mapID, err)
		return tiles
	}
	if len(overrides) == 0 {
		return tiles
	}

	byCoord := make(map[string]eventTileOverride)
	for _, override := range overrides {
		if override.eventTileEligible(charID, efm) {
			byCoord[tileKey(override.X, override.Y)] = override
		}
	}
	if len(byCoord) == 0 {
		return tiles
	}

	for i := range tiles {
		override, ok := byCoord[tileKey(tiles[i].X, tiles[i].Y)]
		if !ok {
			continue
		}
		tiles[i].TileImageID = override.TileImageID
		tiles[i].CollisionType = override.CollisionType
		props := tileRuntimePropertiesForTileImage(override.TileImageID)
		tiles[i].RawFootTileID = props.RawFootTileID
		tiles[i].TalkOverTile = props.TalkOverTile
	}
	return tiles
}

func EventTileCollisionOverrides(charID int64, mapID int, efm *EventFlagManager) map[string]int {
	overrides, err := eventTileOverridesForMap(mapID)
	if err != nil {
		log.Printf("[EventTiles] Failed to load collision overrides for map %d: %v", mapID, err)
		return nil
	}
	collisions := make(map[string]int)
	for _, override := range overrides {
		if override.eventTileEligible(charID, efm) {
			collisions[tileKey(override.X, override.Y)] = override.CollisionType
		}
	}
	if len(collisions) == 0 {
		return nil
	}
	return collisions
}

func EventTileRawFootTileOverrides(charID int64, mapID int, efm *EventFlagManager) map[string]*int {
	overrides, err := eventTileOverridesForMap(mapID)
	if err != nil {
		log.Printf("[EventTiles] Failed to load raw foot tile overrides for map %d: %v", mapID, err)
		return nil
	}
	rawFootTiles := make(map[string]*int)
	for _, override := range overrides {
		if override.eventTileEligible(charID, efm) {
			rawFootTiles[tileKey(override.X, override.Y)] = rawFootTileIDForTileImage(override.TileImageID)
		}
	}
	if len(rawFootTiles) == 0 {
		return nil
	}
	return rawFootTiles
}

func EventTileStatesForCharacter(charID int64, mapID int, efm *EventFlagManager) ([]EventTileState, error) {
	overrides, err := eventTileOverridesForMap(mapID)
	if err != nil {
		return nil, err
	}
	if len(overrides) == 0 {
		return nil, nil
	}

	states := make([]EventTileState, 0, len(overrides))
	seen := make(map[string]bool)
	for _, override := range overrides {
		key := tileKey(override.X, override.Y)
		if seen[key] {
			continue
		}
		seen[key] = true
		state, ok := currentEventTileState(charID, mapID, efm, override.X, override.Y, overrides)
		if !ok {
			continue
		}
		states = append(states, state)
	}
	return states, nil
}

func currentEventTileState(charID int64, mapID int, efm *EventFlagManager, x, y int, overrides []eventTileOverride) (EventTileState, bool) {
	for _, override := range overrides {
		if override.X != x || override.Y != y {
			continue
		}
		if override.eventTileEligible(charID, efm) {
			props := tileRuntimePropertiesForTileImage(override.TileImageID)
			return EventTileState{
				X:             override.X,
				Y:             override.Y,
				TileImageID:   override.TileImageID,
				CollisionType: override.CollisionType,
				RawFootTileID: props.RawFootTileID,
				TalkOverTile:  props.TalkOverTile,
				Label:         nullStringValue(override.Label),
			}, true
		}
	}

	base, err := baseEventTileState(mapID, x, y)
	if err != nil {
		log.Printf("[EventTiles] Failed to load base tile for map %d (%d,%d): %v", mapID, x, y, err)
		return EventTileState{}, false
	}
	return base, true
}

func baseEventTileState(mapID, x, y int) (EventTileState, error) {
	query := `SELECT tile_image_id, collision_type, raw_foot_tile_id, talk_over_tile FROM phaser_tiles WHERE map_id = $1 AND x = $2 AND y = $3 LIMIT 1`
	args := []interface{}{mapID, x, y}
	if mapID == UnifiedOverworldMapID {
		query = `SELECT tile_image_id, collision_type, raw_foot_tile_id, talk_over_tile FROM phaser_tiles WHERE map_id IS NULL AND x = $1 AND y = $2 LIMIT 1`
		args = []interface{}{x, y}
	}

	state := EventTileState{X: x, Y: y}
	var rawFootTileID sql.NullInt64
	if err := db.GlobalWorldDB.DB.QueryRow(query, args...).Scan(&state.TileImageID, &state.CollisionType, &rawFootTileID, &state.TalkOverTile); err != nil {
		return EventTileState{}, err
	}
	if rawFootTileID.Valid {
		v := int(rawFootTileID.Int64)
		state.RawFootTileID = &v
	}
	return state, nil
}

func sendEventTileStatesForSession(ses *session.Session, charID int64, mapName string, wh *WorldHandler) {
	if ses == nil || wh == nil {
		return
	}
	mapID := eventTileMapID(mapName, ses)
	if mapID == 0 {
		return
	}
	states, err := EventTileStatesForCharacter(charID, mapID, wh.EventFlags)
	if err != nil {
		log.Printf("[EventTiles] Failed to resolve tile states for char %d map %d: %v", charID, mapID, err)
		return
	}
	if len(states) == 0 {
		return
	}

	edits := make([]TileEdit, 0, len(states))
	for _, state := range states {
		edits = append(edits, TileEdit{
			X:             state.X,
			Y:             state.Y,
			TileImageID:   state.TileImageID,
			CollisionType: state.CollisionType,
			RawFootTileID: state.RawFootTileID,
			TalkOverTile:  state.TalkOverTile,
		})
	}
	if wh.ActorManager != nil {
		wh.ActorManager.InvalidateCollisionMap(mapID)
	}
	ses.SendStreamJSON(TileEditorBroadcastPayload{Tiles: edits, MapID: mapID}, opcodes.TileEditorBroadcast)
}

func eventTileMapID(mapName string, ses *session.Session) int {
	if mapName != "" {
		var mapID int
		if err := db.GlobalWorldDB.DB.QueryRow(`SELECT id FROM phaser_maps WHERE name = $1`, mapName).Scan(&mapID); err == nil {
			return mapID
		}
	}
	if ses != nil {
		return ses.MapID
	}
	return 0
}

func eventTileOverridesForMap(mapID int) ([]eventTileOverride, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT x, y, tile_image_id, collision_type, requires_flag, requires_flag_absent, label
		FROM phaser_event_tile_overrides
		WHERE map_id = $1
		ORDER BY id`, mapID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overrides []eventTileOverride
	for rows.Next() {
		var override eventTileOverride
		if err := rows.Scan(
			&override.X,
			&override.Y,
			&override.TileImageID,
			&override.CollisionType,
			&override.RequiresFlag,
			&override.RequiresFlagAbsent,
			&override.Label,
		); err != nil {
			return nil, err
		}
		overrides = append(overrides, override)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return overrides, nil
}

func (o eventTileOverride) eventTileEligible(charID int64, efm *EventFlagManager) bool {
	if o.RequiresFlag.Valid && o.RequiresFlag.String != "" {
		if efm == nil || charID == 0 || !efm.CheckFlag(charID, o.RequiresFlag.String) {
			return false
		}
	}
	if o.RequiresFlagAbsent.Valid && o.RequiresFlagAbsent.String != "" {
		if efm != nil && charID > 0 && efm.CheckFlag(charID, o.RequiresFlagAbsent.String) {
			return false
		}
	}
	return true
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func eventTileStateSummary(state EventTileState) string {
	label := state.Label
	if label == "" {
		label = "event tile"
	}
	return fmt.Sprintf("%s (%d,%d) tile=%d collision=%d", label, state.X, state.Y, state.TileImageID, state.CollisionType)
}
