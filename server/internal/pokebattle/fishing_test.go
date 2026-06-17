package pokebattle

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSelectFishingEncounterUsesRodTables(t *testing.T) {
	db := openFishingEncounterTestDB(t)

	tests := []struct {
		name          string
		mapID         int
		rodType       string
		wantPokemonID int
		wantLevel     int
	}{
		{name: "old rod always magikarp", mapID: 1, rodType: "old_rod", wantPokemonID: 129, wantLevel: 5},
		{name: "good rod uses global table", mapID: 1, rodType: "good_rod", wantPokemonID: 118, wantLevel: 10},
		{name: "super rod uses map table", mapID: 1, rodType: "super_rod", wantPokemonID: 119, wantLevel: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPokemonID, gotLevel, err := SelectFishingEncounter(db, tt.mapID, tt.rodType)
			if err != nil {
				t.Fatalf("SelectFishingEncounter failed: %v", err)
			}
			if gotPokemonID != tt.wantPokemonID || gotLevel != tt.wantLevel {
				t.Fatalf("SelectFishingEncounter = (%d,%d), want (%d,%d)", gotPokemonID, gotLevel, tt.wantPokemonID, tt.wantLevel)
			}
		})
	}
}

func openFishingEncounterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE phaser_pokemon (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
		CREATE TABLE phaser_wild_encounters (
			map_id INTEGER,
			pokemon_name TEXT NOT NULL,
			level INTEGER NOT NULL,
			encounter_type TEXT NOT NULL,
			slot_index INTEGER NOT NULL,
			version TEXT NOT NULL
		);
		CREATE TABLE phaser_encounter_slots (
			slot_index INTEGER NOT NULL,
			probability REAL NOT NULL
		);
		INSERT INTO phaser_pokemon (id, name) VALUES
			(118, 'GOLDEEN'),
			(119, 'SEAKING'),
			(129, 'MAGIKARP');
		INSERT INTO phaser_wild_encounters (
			map_id, pokemon_name, level, encounter_type, slot_index, version
		) VALUES
			(NULL, 'GOLDEEN', 10, 'good_rod', 1, 'red'),
			(1, 'SEAKING', 30, 'super_rod', 1, 'red');
		INSERT INTO phaser_encounter_slots (slot_index, probability) VALUES (1, 100.0);
	`); err != nil {
		t.Fatalf("seed fishing encounter db: %v", err)
	}

	return db
}
