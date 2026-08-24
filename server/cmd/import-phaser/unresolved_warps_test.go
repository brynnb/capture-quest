package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMarkUnresolvedLastMapWarpPlaceholdersPostgres(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE phaser_warp_events (
			id INTEGER PRIMARY KEY,
			map_name TEXT,
			map_id INTEGER,
			x INTEGER,
			y INTEGER,
			dest_map TEXT,
			dest_warp_index INTEGER
		)`,
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
			(10, 'SAFFRON_CITY'),
			(235, 'SILPH_CO_11F'),
			(236, 'SILPH_CO_ELEVATOR')`,
		`INSERT INTO phaser_warp_events (id, map_name, map_id, x, y, dest_map, dest_warp_index) VALUES
			(656, 'SilphCo11F', 235, 5, 5, 'LAST_MAP', 10),
			(657, 'SilphCo11F', 235, 3, 2, 'SILPH_CO_7F', 4),
			(900, 'SilphCoElevator', 236, 1, 3, 'LAST_MAP', 1)`,
		`INSERT INTO phaser_warps (
			id, source_map_id, x, y, destination_map_id, destination_x, destination_y, warp_type, warp_direction
		) VALUES
			(122, 235, 5, 5, 10, NULL, NULL, 'carpet', 'UP'),
			(123, 235, 3, 2, 212, 5, 7, 'door', NULL),
			(35, 236, 1, 3, NULL, NULL, NULL, 'elevator', NULL)`,
	)

	if err := markUnresolvedLastMapWarpPlaceholdersPostgres(db); err != nil {
		t.Fatalf("markUnresolvedLastMapWarpPlaceholdersPostgres: %v", err)
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

	if got[122].warpType != "inactive" || got[122].direction != "" {
		t.Fatalf("unresolved LAST_MAP warp = %#v, want inactive with no direction", got[122])
	}
	if got[123].warpType != "door" {
		t.Fatalf("complete fixed warp changed to %#v, want door", got[123])
	}
	if got[35].warpType != "elevator" {
		t.Fatalf("dynamic elevator placeholder changed to %#v, want elevator", got[35])
	}
}
