package content

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/protocol"
)

// readSnapshot prevents an aggregate from mixing publications across its queries.
// It owns one operation budget and returns no partial result on load/commit failure.
func readSnapshot[T any](ctx context.Context, database *sql.DB, read func(context.Context, db.ContextDBTX) (T, error)) (T, error) {
	var zero T
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return zero, err
	}
	defer tx.Rollback()
	result, err := read(ctx, tx)
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(); err != nil {
		return zero, err
	}
	return result, nil
}

// collect closes each result before the next query, including on scan failure.
// Empty successful lists are arrays, while any failure discards the partial list.
func collect[T any](ctx context.Context, database db.ContextDBTX, query string, scan func(*sql.Rows, *T) error, args ...any) ([]T, error) {
	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]T, 0)
	for rows.Next() {
		var value T
		if err := scan(rows, &value); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) MapScripts(ctx context.Context, mapName string) (protocol.PhaserMapScriptsResponse, error) {
	return readSnapshot(ctx, s.database, func(ctx context.Context, database db.ContextDBTX) (protocol.PhaserMapScriptsResponse, error) {
		result := protocol.PhaserMapScriptsResponse{MapName: mapName}
		var err error
		result.Scripts, err = collect(ctx, database, `SELECT script_index,script_label,script_constant,raw_asm FROM phaser_map_scripts WHERE map_name=$1 ORDER BY script_index,id`, func(rows *sql.Rows, v *protocol.PhaserMapScript) error {
			return rows.Scan(&v.ScriptIndex, &v.ScriptLabel, &v.ScriptConstant, &v.RawASM)
		}, mapName)
		if err != nil {
			return result, fmt.Errorf("map scripts: %w", err)
		}
		result.EventFlags, err = collect(ctx, database, `SELECT flag_name,operation,context_label FROM phaser_event_flags WHERE map_name=$1 ORDER BY id`, func(rows *sql.Rows, v *protocol.PhaserEventFlag) error {
			return rows.Scan(&v.FlagName, &v.Operation, &v.ContextLabel)
		}, mapName)
		if err != nil {
			return result, fmt.Errorf("map event flags: %w", err)
		}
		result.CoordinateTriggers, err = collect(ctx, database, `SELECT label,x,y FROM phaser_coordinate_triggers WHERE map_name=$1 ORDER BY id`, func(rows *sql.Rows, v *protocol.PhaserCoordinateTrigger) error {
			return rows.Scan(&v.Label, &v.X, &v.Y)
		}, mapName)
		if err != nil {
			return result, fmt.Errorf("map coordinate triggers: %w", err)
		}
		result.NPCMovements, err = collect(ctx, database, `SELECT label,movements FROM phaser_npc_movement_data WHERE map_name=$1 ORDER BY id`, func(rows *sql.Rows, v *protocol.PhaserNPCMovement) error { return rows.Scan(&v.Label, &v.Movements) }, mapName)
		if err != nil {
			return result, fmt.Errorf("map NPC movements: %w", err)
		}
		result.Success = true
		return result, nil
	})
}

func (s *Service) Learnset(ctx context.Context, pokemonID int) (protocol.PhaserLearnsetResponse, error) {
	return readSnapshot(ctx, s.database, func(ctx context.Context, database db.ContextDBTX) (protocol.PhaserLearnsetResponse, error) {
		result := protocol.PhaserLearnsetResponse{PokemonID: pokemonID}
		var err error
		result.Learnset, err = collect(ctx, database, `SELECT level,move_name,move_id FROM phaser_pokemon_learnset WHERE pokemon_id=$1 ORDER BY level,id`, func(rows *sql.Rows, v *protocol.PhaserLearnsetEntry) error {
			return rows.Scan(&v.Level, &v.MoveName, &v.MoveID)
		}, pokemonID)
		if err != nil {
			return result, fmt.Errorf("level-up learnset: %w", err)
		}
		result.TMHM, err = collect(ctx, database, `SELECT tm_hm_name,move_name,move_id,is_hm FROM phaser_pokemon_tmhm WHERE pokemon_id=$1 ORDER BY tm_hm_name,id`, func(rows *sql.Rows, v *protocol.PhaserTMHMEntry) error {
			return rows.Scan(&v.TMHMName, &v.MoveName, &v.MoveID, &v.IsHM)
		}, pokemonID)
		if err != nil {
			return result, fmt.Errorf("TM/HM compatibility: %w", err)
		}
		result.Success = true
		return result, nil
	})
}
