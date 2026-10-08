package world

import (
	"context"
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

// Ordered rules use the last eligible override, matching presentation and
// collision projection. Publishers must not choose the first matching rule.
func eligibleEventTileOverrides(charID int64, flags *EventFlagManager, overrides []eventTileOverride) map[string]eventTileOverride {
	selected := make(map[string]eventTileOverride)
	for _, override := range overrides {
		if override.eventTileEligible(charID, flags) {
			selected[tileKey(override.X, override.Y)] = override
		}
	}
	return selected
}

// Project one owned database snapshot. An unavailable override/image source is
// a failed read, never permission to publish plausible base tiles.
func applyEventTileOverridesContext(ctx context.Context, database db.ReadDBTX, charID int64, mapID int, tiles []PhaserTile) ([]PhaserTile, error) {
	if len(tiles) == 0 {
		return tiles, nil
	}
	flags, err := eventFlagSnapshotIn(database, charID)
	if err != nil {
		return nil, err
	}
	overrides, err := eventTileOverridesForMapContext(ctx, database, mapID)
	if err != nil {
		return nil, err
	}
	byCoord := eligibleEventTileOverrides(charID, flags, overrides)
	properties := make(map[int]tileRuntimeProperties)
	for i := range tiles {
		override, ok := byCoord[tileKey(tiles[i].X, tiles[i].Y)]
		if !ok {
			continue
		}
		props, ok := properties[override.TileImageID]
		if !ok {
			props, err = tileRuntimePropertiesForTileImageContext(ctx, database, override.TileImageID)
			if err != nil {
				return nil, fmt.Errorf("event tile map=%d coordinate=(%d,%d) image=%d: %w", mapID, override.X, override.Y, override.TileImageID, err)
			}
			properties[override.TileImageID] = props
		}
		tiles[i].TileImageID = override.TileImageID
		tiles[i].CollisionType = override.CollisionType
		tiles[i].RawFootTileID = props.RawFootTileID
		tiles[i].TalkOverTile = props.TalkOverTile
		tiles[i].ContentOrigin = "event"
	}
	return tiles, nil
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

func EventTileStatesForCharacter(ctx context.Context, database *sql.DB, charID int64, mapID int) ([]EventTileState, error) {
	return db.ReadSnapshot(ctx, database, func(ctx context.Context, q db.ReadDBTX) ([]EventTileState, error) {
		return eventTileStatesIn(ctx, q, charID, mapID)
	})
}
func eventTileStatesIn(ctx context.Context, q db.ReadDBTX, charID int64, mapID int) ([]EventTileState, error) {
	flags, err := eventFlagSnapshotIn(q, charID)
	if err != nil {
		return nil, err
	}
	overrides, err := eventTileOverridesForMapContext(ctx, q, mapID)
	if err != nil {
		return nil, err
	}
	selected := eligibleEventTileOverrides(charID, flags, overrides)
	states := make([]EventTileState, 0)
	seen := make(map[string]bool)
	for _, rule := range overrides {
		key := tileKey(rule.X, rule.Y)
		if seen[key] {
			continue
		}
		seen[key] = true
		var state EventTileState
		if chosen, ok := selected[key]; ok {
			props, err := tileRuntimePropertiesForTileImageContext(ctx, q, chosen.TileImageID)
			if err != nil {
				return nil, fmt.Errorf("event tile map=%d coordinate=(%d,%d) image=%d: %w", mapID, chosen.X, chosen.Y, chosen.TileImageID, err)
			}
			state = EventTileState{X: chosen.X, Y: chosen.Y, TileImageID: chosen.TileImageID, CollisionType: chosen.CollisionType, RawFootTileID: props.RawFootTileID, TalkOverTile: props.TalkOverTile, Label: nullStringValue(chosen.Label)}
		} else {
			state, err = baseEventTileStateIn(q, mapID, rule.X, rule.Y)
			if err != nil {
				return nil, fmt.Errorf("base event tile map=%d coordinate=(%d,%d): %w", mapID, rule.X, rule.Y, err)
			}
		}
		states = append(states, state)
	}
	return states, nil
}

func baseEventTileStateIn(database db.DBTX, mapID, x, y int) (EventTileState, error) {
	query := `SELECT tile_image_id, collision_type, raw_foot_tile_id, talk_over_tile FROM phaser_tiles WHERE map_id = $1 AND x = $2 AND y = $3 AND is_tile_erased = 0 LIMIT 1`
	args := []interface{}{mapID, x, y}
	if mapID == UnifiedOverworldMapID {
		query = `SELECT tile_image_id, collision_type, raw_foot_tile_id, talk_over_tile FROM phaser_tiles WHERE map_id IS NULL AND x = $1 AND y = $2 AND is_tile_erased = 0 LIMIT 1`
		args = []interface{}{x, y}
	}

	state := EventTileState{X: x, Y: y}
	var rawFootTileID sql.NullInt64
	if err := database.QueryRow(query, args...).Scan(&state.TileImageID, &state.CollisionType, &rawFootTileID, &state.TalkOverTile); err != nil {
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
	type publication struct {
		mapID  int
		states []EventTileState
	}
	view, err := db.ReadSnapshot(ses.CommandContext(), wh.database, func(ctx context.Context, q db.ReadDBTX) (publication, error) {
		mapID, err := eventTileMapIDContext(ctx, q, mapName, ses)
		if err != nil {
			return publication{}, err
		}
		states, err := eventTileStatesIn(ctx, q, charID, mapID)
		if err != nil {
			return publication{}, err
		}
		return publication{mapID: mapID, states: states}, nil
	})
	if err != nil {
		log.Printf("[EventTiles] Character %d map %q publication read: %v", charID, mapName, err)
		return
	}
	mapID, states := view.mapID, view.states
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

func eventTileMapIDContext(ctx context.Context, database db.ContextDBTX, mapName string, ses *session.Session) (int, error) {
	if mapName != "" {
		var mapID int
		if err := database.QueryRowContext(ctx, `SELECT id FROM phaser_maps WHERE name=$1`, mapName).Scan(&mapID); err != nil {
			return 0, fmt.Errorf("map %q: %w", mapName, err)
		}
		return mapID, nil
	}
	if ses == nil {
		return 0, fmt.Errorf("event tile map context is required")
	}
	return ses.MapID, nil
}

func eventTileOverridesForMap(mapID int) ([]eventTileOverride, error) {
	return eventTileOverridesForMapContext(context.Background(), db.GlobalWorldDB.DB, mapID)
}

func eventTileOverridesForMapContext(ctx context.Context, database db.ContextDBTX, mapID int) ([]eventTileOverride, error) {
	rows, err := database.QueryContext(ctx, `
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
