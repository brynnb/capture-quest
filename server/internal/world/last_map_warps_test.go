package world

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLastMapWarpResolutionIsPerPlayer(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	statements := []string{
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, source_warp_index INTEGER, x INTEGER, y INTEGER)`,
		`INSERT INTO phaser_maps VALUES (17, 'ROUTE_6'), (18, 'ROUTE_7')`,
		`INSERT INTO phaser_warps VALUES (1, 17, 2, 190, -83), (2, 18, 2, 205, -44)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	mapA, nameA, xA, yA, err := resolveLastMapWarpForPlayer(database, 17, 2)
	if err != nil {
		t.Fatal(err)
	}
	mapB, nameB, xB, yB, err := resolveLastMapWarpForPlayer(database, 18, 2)
	if err != nil {
		t.Fatal(err)
	}
	if mapA != 17 || nameA != "ROUTE_6" || xA != 190 || yA != -83 {
		t.Fatalf("player A destination = (%d,%s,%d,%d)", mapA, nameA, xA, yA)
	}
	if mapB != 18 || nameB != "ROUTE_7" || xB != 205 || yB != -44 {
		t.Fatalf("player B destination = (%d,%s,%d,%d)", mapB, nameB, xB, yB)
	}
}

func TestLastMapWarpRequiresPreviousMapContext(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, _, _, _, err := resolveLastMapWarpForPlayer(database, -1, 1); err == nil {
		t.Fatal("expected missing previous-map context to fail")
	}
}
