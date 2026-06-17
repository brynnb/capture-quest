package world

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRoute6GateSouthExitCanPathIntoOverworld(t *testing.T) {
	db := openWorldPokemonSQLiteForTest(t)
	defer db.Close()

	collisionMap := loadSQLiteOverworldCollisionForTest(t, db)
	got := findPathOnCollisionMap(collisionMap, nil, 190, -83, 190, -81)
	if len(got) == 0 {
		t.Fatalf("Route 6 Gate south exit path from (190,-83) to (190,-81) was not found")
	}
}

func openWorldPokemonSQLiteForTest(t *testing.T) *sql.DB {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "public", "phaser", "pokemon.db"),
		filepath.Join("..", "public", "phaser", "pokemon.db"),
		filepath.Join("public", "phaser", "pokemon.db"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		db, err := sql.Open("sqlite", candidate)
		if err != nil {
			continue
		}
		if err := db.Ping(); err == nil {
			return db
		}
		db.Close()
	}
	t.Fatal("could not open public/phaser/pokemon.db")
	return nil
}

func loadSQLiteOverworldCollisionForTest(t *testing.T, db *sql.DB) map[string]int {
	t.Helper()
	rows, err := db.Query(`
		SELECT t.x, t.y, t.collision_type
		FROM tiles t
		JOIN maps m ON m.id = t.map_id
		WHERE m.is_overworld = 1`)
	if err != nil {
		t.Fatalf("query overworld collision: %v", err)
	}
	defer rows.Close()

	collisionMap := make(map[string]int)
	for rows.Next() {
		var x, y, collisionType int
		if err := rows.Scan(&x, &y, &collisionType); err != nil {
			t.Fatalf("scan overworld collision: %v", err)
		}
		collisionMap[tileKey(x, y)] = collisionType
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read overworld collision: %v", err)
	}
	return collisionMap
}
