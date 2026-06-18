package scriptsim

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDebugScenarioFixturesStartOnUsableTiles(t *testing.T) {
	db := openPokemonSQLiteForDebugFixtureTest(t)
	defer db.Close()

	scenarioDir := debugScenarioFixtureDir(t)
	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		t.Fatalf("read debug scenario dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "debug_") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(scenarioDir, entry.Name())
		scenario, err := LoadScenario(path)
		if err != nil {
			t.Fatalf("load scenario %s: %v", entry.Name(), err)
		}
		if scenario.Fixture.MapName == "" {
			continue
		}

		var mapID int
		if err := db.QueryRow(`SELECT id FROM maps WHERE name = ?`, scenario.Fixture.MapName).Scan(&mapID); err != nil {
			t.Fatalf("%s fixture map %q not found: %v", entry.Name(), scenario.Fixture.MapName, err)
		}

		var collisionType int
		if err := db.QueryRow(
			`SELECT collision_type FROM tiles WHERE map_id = ? AND x = ? AND y = ?`,
			mapID,
			scenario.Fixture.X,
			scenario.Fixture.Y,
		).Scan(&collisionType); err != nil {
			t.Fatalf(
				"%s fixture tile %s (%d,%d) not found: %v",
				entry.Name(),
				scenario.Fixture.MapName,
				scenario.Fixture.X,
				scenario.Fixture.Y,
				err,
			)
		}
		if collisionType == 0 {
			t.Fatalf(
				"%s fixture starts on blocked tile %s (%d,%d)",
				entry.Name(),
				scenario.Fixture.MapName,
				scenario.Fixture.X,
				scenario.Fixture.Y,
			)
		}
	}
}

func openPokemonSQLiteForDebugFixtureTest(t *testing.T) *sql.DB {
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

func debugScenarioFixtureDir(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "script_tests", "scenarios"),
		filepath.Join("script_tests", "scenarios"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}
	t.Fatal("script_tests/scenarios directory not found")
	return ""
}
