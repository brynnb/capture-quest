package world

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSpinTileManagerLoadAndCheckTile(t *testing.T) {
	db := openSpinTileManagerTestDB(t)
	manager := NewSpinTileManager(db)
	manager.Load()

	tile := manager.CheckTile("ROCKET_HIDEOUT_B2F", 4, 15)
	if tile == nil {
		t.Fatal("expected spin tile at Rocket Hideout B2F 4,15")
	}
	got := ExpandMovements(tile.Movements)
	want := []string{"RIGHT", "RIGHT", "UP"}
	if len(got) != len(want) {
		t.Fatalf("expanded movements = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expanded movements = %#v, want %#v", got, want)
		}
	}

	if skipped := manager.CheckTile("ROCKET_HIDEOUT_B2F", 99, 99); skipped != nil {
		t.Fatalf("unexpected spin tile at missing coordinate: %#v", skipped)
	}
	if malformed := manager.CheckTile("VIRIDIAN_GYM", 19, 11); malformed != nil {
		t.Fatalf("malformed movement JSON should have been skipped: %#v", malformed)
	}
}

func TestExpandMovementsNormalizesDirections(t *testing.T) {
	got := ExpandMovements([]SpinMovement{
		{Direction: " left ", Count: 2},
		{Direction: "Down", Count: 1},
		{Direction: "UP", Count: 0},
	})
	want := []string{"LEFT", "LEFT", "DOWN"}
	if len(got) != len(want) {
		t.Fatalf("expanded movements = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expanded movements = %#v, want %#v", got, want)
		}
	}
}

func openSpinTileManagerTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE phaser_spin_tiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			map_name TEXT NOT NULL,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			movements TEXT NOT NULL,
			UNIQUE (map_name, x, y)
		);
		INSERT INTO phaser_spin_tiles (map_name, x, y, movements) VALUES
			('ROCKET_HIDEOUT_B2F', 4, 15, '[{"direction":"right","count":2},{"direction":"UP","count":1}]'),
			('VIRIDIAN_GYM', 19, 11, 'not-json');
	`); err != nil {
		t.Fatalf("seed spin tiles: %v", err)
	}
	return db
}
