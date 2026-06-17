package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLoadSpinTileImportRowsNormalizesMapNames(t *testing.T) {
	sqlite := openSpinTileSQLite(t)
	defer sqlite.Close()

	rows, found, err := loadSpinTileImportRows(sqlite)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("loadSpinTileImportRows found=false, want true")
	}
	if len(rows) != 2 {
		t.Fatalf("loaded %d spin rows, want 2", len(rows))
	}

	want := []spinTileImportRow{
		{ID: 1, MapName: "ROCKET_HIDEOUT_B2F", X: 4, Y: 15, Movements: `[{"direction":"RIGHT","count":4},{"direction":"UP","count":4}]`},
		{ID: 2, MapName: "VIRIDIAN_GYM", X: 19, Y: 11, Movements: `[{"direction":"UP","count":9}]`},
	}
	for i, row := range rows {
		if row != want[i] {
			t.Fatalf("row %d = %#v, want %#v", i, row, want[i])
		}
	}
}

func TestLoadSpinTileImportRowsMissingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pokemon.db")
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	if _, err := sqlite.Exec(`CREATE TABLE placeholder (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	rows, found, err := loadSpinTileImportRows(sqlite)
	if err != nil {
		t.Fatal(err)
	}
	if found || len(rows) != 0 {
		t.Fatalf("found=%t rows=%#v, want missing table", found, rows)
	}
}

func openSpinTileSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.Exec(`
		CREATE TABLE spin_tiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			map_name TEXT NOT NULL,
			source_label TEXT NOT NULL,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			movement_label TEXT NOT NULL,
			movements TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		mapName, sourceLabel, movementLabel, movements string
		x, y                                           int
	}{
		{"RocketHideoutB2F", "RocketHideout2ArrowTilePlayerMovement", "RocketHideout2ArrowMovement3", `[{"direction":"RIGHT","count":4},{"direction":"UP","count":4}]`, 4, 15},
		{"ViridianGym", "ViridianGymArrowTilePlayerMovement", "ViridianGymArrowMovement1", `[{"direction":"UP","count":9}]`, 19, 11},
	}
	for _, row := range rows {
		if _, err := sqlite.Exec(`
			INSERT INTO spin_tiles (map_name, source_label, x, y, movement_label, movements)
			VALUES (?, ?, ?, ?, ?, ?)`,
			row.mapName, row.sourceLabel, row.x, row.y, row.movementLabel, row.movements,
		); err != nil {
			t.Fatal(err)
		}
	}
	return sqlite
}
