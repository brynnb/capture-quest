package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLoadDungeonHoleWarpSeedsFromSQLite(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE script_event_dungeon_hole_warps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_map TEXT NOT NULL,
			source_x INTEGER NOT NULL,
			source_y INTEGER NOT NULL,
			destination_map TEXT NOT NULL,
			destination_x INTEGER NOT NULL,
			destination_y INTEGER NOT NULL,
			destination_warp_index INTEGER NOT NULL,
			source_file TEXT NOT NULL
		)`,
		`INSERT INTO script_event_dungeon_hole_warps (
			source_map, source_x, source_y,
			destination_map, destination_x, destination_y,
			destination_warp_index, source_file
		) VALUES
			('SEAFOAM_ISLANDS_B3F', 3, 16, 'SEAFOAM_ISLANDS_B4F', 4, 14, 1, 'scripts/SeafoamIslandsB3F.asm'),
			('VICTORY_ROAD_3F', 23, 15, 'VICTORY_ROAD_2F', 22, 16, 2, 'scripts/VictoryRoad3F.asm')`,
	)

	seeds, err := loadDungeonHoleWarpSeedsFromSQLite(db)
	if err != nil {
		t.Fatalf("loadDungeonHoleWarpSeedsFromSQLite: %v", err)
	}
	if len(seeds) != 2 {
		t.Fatalf("loaded %d dungeon hole warps, want 2: %#v", len(seeds), seeds)
	}
	if seeds[0] != (dungeonHoleWarpSeed{
		SourceMap:            "SEAFOAM_ISLANDS_B3F",
		X:                    3,
		Y:                    16,
		DestinationMap:       "SEAFOAM_ISLANDS_B4F",
		DestinationX:         4,
		DestinationY:         14,
		DestinationWarpIndex: 1,
		SourceFile:           "scripts/SeafoamIslandsB3F.asm",
	}) {
		t.Fatalf("first seed = %#v, want Seafoam B3F hole", seeds[0])
	}
}

func TestLoadDungeonHoleWarpSeedsFromSQLiteRequiresGeneratedTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	_, err = loadDungeonHoleWarpSeedsFromSQLite(db)
	if err == nil || !strings.Contains(err.Error(), "script_event_dungeon_hole_warps table missing") {
		t.Fatalf("loadDungeonHoleWarpSeedsFromSQLite error = %v, want missing generated table error", err)
	}
}

func TestSeedDungeonHoleWarpsPostgresRejectsEmptyGeneratedTable(t *testing.T) {
	sqlite, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqlite.Close()
	pg, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open pg fixture: %v", err)
	}
	defer pg.Close()

	execStatements(t, sqlite,
		`CREATE TABLE script_event_dungeon_hole_warps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_map TEXT NOT NULL,
			source_x INTEGER NOT NULL,
			source_y INTEGER NOT NULL,
			destination_map TEXT NOT NULL,
			destination_x INTEGER NOT NULL,
			destination_y INTEGER NOT NULL,
			destination_warp_index INTEGER NOT NULL,
			source_file TEXT NOT NULL
		)`,
	)

	err = seedDungeonHoleWarpsPostgres(sqlite, pg)
	if err == nil || !strings.Contains(err.Error(), "script_event_dungeon_hole_warps exists but has no rows") {
		t.Fatalf("seedDungeonHoleWarpsPostgres error = %v, want empty generated table error", err)
	}
}

func TestUpsertDungeonHoleWarpsPostgresDoesNotRewriteOrdinaryWarpRows(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE phaser_warps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_map_id INTEGER NOT NULL,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			destination_map_id INTEGER,
			destination_map TEXT,
			destination_x INTEGER,
			destination_y INTEGER,
			warp_type TEXT DEFAULT 'door',
			warp_direction TEXT
		)`,
		`INSERT INTO phaser_maps (id, name) VALUES
			(161, 'SEAFOAM_ISLANDS_B3F'),
			(162, 'SEAFOAM_ISLANDS_B4F')`,
		`INSERT INTO phaser_warps (
			source_map_id, x, y, destination_map_id, destination_map,
			destination_x, destination_y, warp_type
		) VALUES (161, 20, 17, 162, 'SEAFOAM_ISLANDS_B4F', 20, 17, 'door')`,
	)

	seeds := []dungeonHoleWarpSeed{
		{
			SourceMap:      "SEAFOAM_ISLANDS_B3F",
			X:              3,
			Y:              16,
			DestinationMap: "SEAFOAM_ISLANDS_B4F",
			DestinationX:   4,
			DestinationY:   14,
			SourceFile:     "scripts/SeafoamIslandsB3F.asm",
		},
	}
	if err := upsertDungeonHoleWarpsPostgres(db, seeds); err != nil {
		t.Fatalf("upsertDungeonHoleWarpsPostgres: %v", err)
	}

	var destinationX, destinationY int
	var warpType string
	if err := db.QueryRow(`
		SELECT destination_x, destination_y, warp_type
		FROM phaser_warps
		WHERE source_map_id = 161 AND x = 3 AND y = 16`).Scan(&destinationX, &destinationY, &warpType); err != nil {
		t.Fatalf("query inserted hole warp: %v", err)
	}
	if destinationX != 4 || destinationY != 14 || warpType != "carpet" {
		t.Fatalf("inserted hole warp = (%d,%d,%q), want (4,14,carpet)", destinationX, destinationY, warpType)
	}

	if err := db.QueryRow(`
		SELECT destination_x, destination_y, warp_type
		FROM phaser_warps
		WHERE source_map_id = 161 AND x = 20 AND y = 17`).Scan(&destinationX, &destinationY, &warpType); err != nil {
		t.Fatalf("query ordinary warp: %v", err)
	}
	if destinationX != 20 || destinationY != 17 || warpType != "door" {
		t.Fatalf("ordinary warp row was rewritten to (%d,%d,%q), want unchanged (20,17,door)", destinationX, destinationY, warpType)
	}
}
