package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLoadTilesetGrassTileIDsUsesNativeMetadata(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE tilesets (id INTEGER PRIMARY KEY, grass_tile_id INTEGER);
		INSERT INTO tilesets VALUES
			(0, 82),
			(1, NULL),
			(3, 32),
			(23, 69);`); err != nil {
		t.Fatal(err)
	}

	got, err := loadTilesetGrassTileIDs(db)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]int{0: 0x52, 3: 0x20, 23: 0x45}
	if len(got) != len(want) {
		t.Fatalf("grass tile count = %d, want %d (%v)", len(got), len(want), got)
	}
	for tilesetID, grassTileID := range want {
		if got[tilesetID] != grassTileID {
			t.Fatalf("tileset %d grass tile = %#x, want %#x", tilesetID, got[tilesetID], grassTileID)
		}
	}
}

func TestValidateEncounterAreaCoverageRejectsEmptyArea(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE phaser_encounter_areas (
			id INTEGER PRIMARY KEY, name TEXT, encounter_type TEXT
		);
		CREATE TABLE phaser_tiles (
			id INTEGER PRIMARY KEY, original_encounter_area_id INTEGER,
			is_tile_erased INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO phaser_encounter_areas VALUES
			(1, 'ROUTE_1_GRASS', 'grass'),
			(2, 'ROUTE_2_GRASS', 'grass');
		INSERT INTO phaser_tiles VALUES (1, 1, 0);`); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	err = validateEncounterAreaCoveragePostgres(tx)
	if err == nil || !strings.Contains(err.Error(), "ROUTE_2_GRASS") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateEncounterAreaCoverageAcceptsPopulatedAreas(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE phaser_encounter_areas (
			id INTEGER PRIMARY KEY, name TEXT, encounter_type TEXT
		);
		CREATE TABLE phaser_tiles (
			id INTEGER PRIMARY KEY, original_encounter_area_id INTEGER,
			is_tile_erased INTEGER NOT NULL DEFAULT 0
		);
		INSERT INTO phaser_encounter_areas VALUES (1, 'ROUTE_1_GRASS', 'grass');
		INSERT INTO phaser_tiles VALUES (1, 1, 0);`); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := validateEncounterAreaCoveragePostgres(tx); err != nil {
		t.Fatal(err)
	}
}

func TestLoadTilesetGrassTileIDsRejectsMissingMetadata(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE tilesets (id INTEGER PRIMARY KEY, grass_tile_id INTEGER)`); err != nil {
		t.Fatal(err)
	}

	if _, err := loadTilesetGrassTileIDs(db); err == nil {
		t.Fatal("loadTilesetGrassTileIDs accepted an empty metadata set")
	}
}
