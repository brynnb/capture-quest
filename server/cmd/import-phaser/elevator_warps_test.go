package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMarkDynamicElevatorWarpPlaceholdersPostgres(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE phaser_elevator_floors (elevator_map_id INTEGER, floor_map_id INTEGER)`,
		`CREATE TABLE phaser_warps (
			id INTEGER PRIMARY KEY,
			source_map_id INTEGER,
			x INTEGER,
			y INTEGER,
			destination_map_id INTEGER,
			destination_x INTEGER,
			destination_y INTEGER,
			warp_type TEXT,
			warp_direction TEXT
		)`,
		`INSERT INTO phaser_maps (id, name) VALUES
			(127, 'CELADON_MART_ELEVATOR'),
			(181, 'SILPH_CO_1F'),
			(236, 'SILPH_CO_ELEVATOR'),
			(237, 'UNUSED_MAP_ED')`,
		`INSERT INTO phaser_elevator_floors (elevator_map_id, floor_map_id) VALUES
			(236, 181)`,
		`INSERT INTO phaser_warps (
			id, source_map_id, x, y, destination_map_id, destination_x, destination_y, warp_type, warp_direction
		) VALUES
			(35, 236, 1, 3, 237, NULL, NULL, 'carpet', 'DOWN'),
			(36, 236, 2, 3, 237, NULL, NULL, 'carpet', 'DOWN'),
			(194, 127, 1, 3, 181, 1, 1, 'carpet', 'DOWN'),
			(999, 127, 5, 5, 237, NULL, NULL, 'carpet', 'DOWN')`,
	)

	if err := markDynamicElevatorWarpPlaceholdersPostgres(db); err != nil {
		t.Fatalf("markDynamicElevatorWarpPlaceholdersPostgres: %v", err)
	}

	rows, err := db.Query(`SELECT id, warp_type, COALESCE(warp_direction, '') FROM phaser_warps ORDER BY id`)
	if err != nil {
		t.Fatalf("query warps: %v", err)
	}
	defer rows.Close()

	got := map[int]struct {
		warpType  string
		direction string
	}{}
	for rows.Next() {
		var id int
		var warpType, direction string
		if err := rows.Scan(&id, &warpType, &direction); err != nil {
			t.Fatalf("scan warp: %v", err)
		}
		got[id] = struct {
			warpType  string
			direction string
		}{warpType: warpType, direction: direction}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read warps: %v", err)
	}

	for _, id := range []int{35, 36} {
		if got[id].warpType != "elevator" || got[id].direction != "" {
			t.Fatalf("warp %d = %#v, want elevator with no direction", id, got[id])
		}
	}
	if got[194].warpType != "carpet" || got[194].direction != "DOWN" {
		t.Fatalf("fixed Celadon elevator warp changed to %#v, want original carpet DOWN", got[194])
	}
	if got[999].warpType != "carpet" || got[999].direction != "DOWN" {
		t.Fatalf("UNUSED_MAP_ED row without elevator floors changed to %#v, want original carpet DOWN", got[999])
	}
}
