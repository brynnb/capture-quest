package main

import (
	"database/sql"
	"fmt"
	"log"
)

type dungeonHoleWarpSeed struct {
	SourceMap            string
	X                    int
	Y                    int
	DestinationMap       string
	DestinationX         int
	DestinationY         int
	SourceFile           string
	DestinationWarpIndex int
}

func seedDungeonHoleWarpsPostgres(sqlite, pg *sql.DB) error {
	seeds, err := loadDungeonHoleWarpSeedsFromSQLite(sqlite)
	if err != nil {
		return err
	}
	if len(seeds) == 0 {
		return fmt.Errorf("script_event_dungeon_hole_warps exists but has no rows")
	}
	if err := resetPostgresIdentitySequence(pg, "phaser_warps"); err != nil {
		return err
	}
	return upsertDungeonHoleWarpsPostgres(pg, seeds)
}

func loadDungeonHoleWarpSeedsFromSQLite(sqlite *sql.DB) ([]dungeonHoleWarpSeed, error) {
	if sqlite == nil {
		return nil, fmt.Errorf("script_event_dungeon_hole_warps requires a SQLite source database")
	}
	exists, err := sqliteTableExists(sqlite, "script_event_dungeon_hole_warps")
	if err != nil {
		return nil, fmt.Errorf("check script_event_dungeon_hole_warps: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("script_event_dungeon_hole_warps table missing; run npm run bootstrap:assets")
	}

	rows, err := sqlite.Query(`
		SELECT source_map, source_x, source_y,
		       destination_map, destination_x, destination_y,
		       destination_warp_index, source_file
		FROM script_event_dungeon_hole_warps
		ORDER BY source_map, source_y, source_x`)
	if err != nil {
		return nil, fmt.Errorf("query script_event_dungeon_hole_warps: %w", err)
	}
	defer rows.Close()

	seeds := []dungeonHoleWarpSeed{}
	for rows.Next() {
		var seed dungeonHoleWarpSeed
		if err := rows.Scan(
			&seed.SourceMap,
			&seed.X,
			&seed.Y,
			&seed.DestinationMap,
			&seed.DestinationX,
			&seed.DestinationY,
			&seed.DestinationWarpIndex,
			&seed.SourceFile,
		); err != nil {
			return nil, fmt.Errorf("scan script_event_dungeon_hole_warps row: %w", err)
		}
		seeds = append(seeds, seed)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read script_event_dungeon_hole_warps rows: %w", err)
	}
	return seeds, nil
}

func upsertDungeonHoleWarpsPostgres(pg *sql.DB, seeds []dungeonHoleWarpSeed) error {
	mapIDs, err := loadPostgresMapIDsByName(pg)
	if err != nil {
		return err
	}

	updateStmt, err := pg.Prepare(`
		UPDATE phaser_warps
		SET destination_map_id = $1,
		    destination_map = $2,
		    destination_x = $3,
		    destination_y = $4,
		    warp_type = 'carpet',
		    warp_direction = NULL
		WHERE source_map_id = $5
		  AND x = $6
		  AND y = $7`)
	if err != nil {
		return fmt.Errorf("prepare dungeon hole warp update: %w", err)
	}
	defer updateStmt.Close()

	insertStmt, err := pg.Prepare(`
		INSERT INTO phaser_warps (
			source_map_id, x, y, destination_map_id, destination_map,
			destination_x, destination_y, warp_type, warp_direction
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'carpet', NULL)`)
	if err != nil {
		return fmt.Errorf("prepare dungeon hole warp insert: %w", err)
	}
	defer insertStmt.Close()

	inserted := int64(0)
	updated := int64(0)
	for _, seed := range seeds {
		sourceMapID, ok := mapIDs[normalizeMapName(seed.SourceMap)]
		if !ok {
			return fmt.Errorf("source map %s for dungeon hole warp %s (%d,%d) not found", seed.SourceMap, seed.SourceFile, seed.X, seed.Y)
		}
		destinationMapID, ok := mapIDs[normalizeMapName(seed.DestinationMap)]
		if !ok {
			return fmt.Errorf("destination map %s for dungeon hole warp %s (%d,%d) not found", seed.DestinationMap, seed.SourceFile, seed.X, seed.Y)
		}

		result, err := updateStmt.Exec(
			destinationMapID,
			seed.DestinationMap,
			seed.DestinationX,
			seed.DestinationY,
			sourceMapID,
			seed.X,
			seed.Y,
		)
		if err != nil {
			return fmt.Errorf("update dungeon hole warp %s (%d,%d): %w", seed.SourceMap, seed.X, seed.Y, err)
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			updated += affected
			continue
		}

		if _, err := insertStmt.Exec(
			sourceMapID,
			seed.X,
			seed.Y,
			destinationMapID,
			seed.DestinationMap,
			seed.DestinationX,
			seed.DestinationY,
		); err != nil {
			return fmt.Errorf("insert dungeon hole warp %s (%d,%d): %w", seed.SourceMap, seed.X, seed.Y, err)
		}
		inserted++
	}

	log.Printf("  -> Seeded %d dungeon hole warps (%d inserted, %d updated)", len(seeds), inserted, updated)
	return nil
}

func loadPostgresMapIDsByName(pg *sql.DB) (map[string]int, error) {
	rows, err := pg.Query(`SELECT id, name FROM phaser_maps`)
	if err != nil {
		return nil, fmt.Errorf("load map ids for dungeon hole warps: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan map id for dungeon hole warps: %w", err)
		}
		result[normalizeMapName(name)] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read map ids for dungeon hole warps: %w", err)
	}
	return result, nil
}
