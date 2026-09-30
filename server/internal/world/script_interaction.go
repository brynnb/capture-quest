package world

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"capturequest/internal/session"
)

var errScriptInteractionDenied = errors.New("actor unavailable or out of reach")

// scriptInteractionTarget authorizes before callers mutate puzzles, doors or
// issue completion tokens. Match the client's adjacency/counter rule, while
// taking target position and visibility exclusively from server state.
func (wh *WorldHandler) scriptInteractionTarget(ses *session.Session, objectID int) (PhaserActor, string, error) {
	if wh == nil || wh.database == nil || wh.ActorManager == nil || ses == nil || !ses.HasValidClient() || ses.IsClosed() {
		return PhaserActor{}, "", fmt.Errorf("interaction state unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	actor, err := wh.ActorManager.loadPhaserObjectActorContext(ctx, wh.database, objectID)
	if err != nil {
		return PhaserActor{}, "", err
	}
	charID := int64(ses.Client.CharData().ID)
	positions, err := characterObjectPositionsContext(ctx, wh.database, charID)
	if err != nil {
		return PhaserActor{}, "", err
	}
	if position, ok := positions[objectID]; ok {
		actor.X, actor.Y = &position.X, &position.Y
	}
	rules, err := eventObjectVisibilityForMapContext(ctx, wh.database, actor.MapID)
	if err != nil {
		return PhaserActor{}, "", err
	}
	overrides, err := objectVisibilityOverridesForCharacterContext(ctx, wh.database, charID)
	if err != nil {
		return PhaserActor{}, "", err
	}
	name := ""
	if actor.Name != nil {
		name = *actor.Name
	}
	visible, label := currentEventObjectVisibility(charID, wh.EventFlags, name, rules)
	visible, _ = applyObjectVisibilityOverride(objectID, visible, label, overrides)
	if !visible || actor.X == nil || actor.Y == nil {
		return PhaserActor{}, "", errScriptInteractionDenied
	}
	var mapName string
	var overworld int
	if err := wh.database.QueryRowContext(ctx, `SELECT name, is_overworld FROM phaser_maps WHERE id = $1`, actor.MapID).Scan(&mapName, &overworld); err != nil {
		return PhaserActor{}, "", err
	}
	x, y, playerMap := wh.scriptPlayerPosition(ses)
	actorMap := actor.MapID
	if overworld != 0 {
		actorMap = UnifiedOverworldMapID
	}
	if playerMap == actor.MapID {
		playerMap = actorMap
	} else if playerMap != UnifiedOverworldMapID {
		var playerOverworld int
		if err := wh.database.QueryRowContext(ctx, `SELECT is_overworld FROM phaser_maps WHERE id = $1`, playerMap).Scan(&playerOverworld); err != nil {
			return PhaserActor{}, "", err
		}
		if playerOverworld != 0 {
			playerMap = UnifiedOverworldMapID
		}
	}
	dx, dy := *actor.X-x, *actor.Y-y
	if actorMap != playerMap {
		return PhaserActor{}, "", errScriptInteractionDenied
	}
	if (dx == 0 && (dy == -1 || dy == 1)) || (dy == 0 && (dx == -1 || dx == 1)) {
		return actor, mapName, nil
	}
	if !((dx == 0 && (dy == -2 || dy == 2)) || (dy == 0 && (dx == -2 || dx == 2))) {
		return PhaserActor{}, "", errScriptInteractionDenied
	}
	var talkOver bool
	err = wh.database.QueryRowContext(ctx, `SELECT talk_over_tile FROM phaser_tiles
		WHERE x = $1 AND y = $2 AND is_tile_erased = 0
		AND (($3::integer = $4::integer AND map_id IS NULL) OR map_id = $3::integer)`, x+dx/2, y+dy/2, playerMap, UnifiedOverworldMapID).Scan(&talkOver)
	if err != nil && err != sql.ErrNoRows {
		return PhaserActor{}, "", err
	}
	if err == nil {
		tileOverrides, err := eventTileOverridesForMapContext(ctx, wh.database, playerMap)
		if err != nil {
			return PhaserActor{}, "", err
		}
		// Presentation applies the last eligible override at each coordinate.
		for _, override := range tileOverrides {
			if override.X != x+dx/2 || override.Y != y+dy/2 || !override.eventTileEligible(charID, wh.EventFlags) {
				continue
			}
			props, err := tileRuntimePropertiesForTileImageContext(ctx, wh.database, override.TileImageID)
			if err != nil {
				return PhaserActor{}, "", err
			}
			talkOver = props.TalkOverTile
		}
	}
	if !talkOver {
		return PhaserActor{}, "", errScriptInteractionDenied
	}
	return actor, mapName, nil
}
