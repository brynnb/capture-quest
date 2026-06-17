package pokebattle

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCheckEvolution_NoEvolution(t *testing.T) {
	// Pokémon with no evolution data
	p := &Pokemon{ID: 6, Name: "CHARIZARD", Level: 50, EvolveLevel: 0}
	id, name := CheckEvolution(nil, p)
	if id != 0 || name != "" {
		t.Errorf("expected no evolution, got id=%d name=%s", id, name)
	}
}

func TestCheckEvolution_BelowLevel(t *testing.T) {
	// Pokémon below evolution level
	p := &Pokemon{ID: 4, Name: "CHARMANDER", Level: 15, EvolveLevel: 16, EvolvePokemonName: "CHARMELEON"}
	id, name := CheckEvolution(nil, p)
	if id != 0 || name != "" {
		t.Errorf("expected no evolution at level 15, got id=%d name=%s", id, name)
	}
}

func TestCheckEvolution_AtLevelUsesSpeciesDB(t *testing.T) {
	db := openEvolutionTestDB(t)
	p := &Pokemon{ID: 4, Name: "CHARMANDER", Level: 16, EvolveLevel: 16, EvolvePokemonName: "CHARMELEON"}

	id, name := CheckEvolution(db, p)
	if id != 5 || name != "CHARMELEON" {
		t.Errorf("expected CHARMELEON evolution, got id=%d name=%s", id, name)
	}
}

func TestCheckEvolution_AtLevelWithoutDBDoesNotPanic(t *testing.T) {
	p := &Pokemon{ID: 4, Name: "CHARMANDER", Level: 16, EvolveLevel: 16, EvolvePokemonName: "CHARMELEON"}

	id, name := CheckEvolution(nil, p)
	if id != 0 || name != "" {
		t.Errorf("expected no DB-backed evolution, got id=%d name=%s", id, name)
	}
}

func TestEvolvePokemon_TransformsInPlace(t *testing.T) {
	db := openEvolutionTestDB(t)

	// Test that EvolvePokemon updates the Pokemon struct in-place
	p := &Pokemon{
		ID:    4,
		Name:  "CHARMANDER",
		Level: 16,
		Type1: TypeFire,
		Type2: TypeFire,
		BaseStats: BaseStats{
			HP: 39, Attack: 52, Defense: 43, Special: 50, Speed: 65,
		},
		IVs:   IVs{Attack: 10, Defense: 8, Speed: 12, Special: 9},
		EVs:   EVs{},
		CurHP: 30,
		MaxHP: 30,
		Moves: [4]MoveSlot{
			{ID: 10, Name: "SCRATCH", PP: 35, MaxPP: 35},
			{ID: 52, Name: "EMBER", PP: 25, MaxPP: 25},
			{},
			{},
		},
		EvolveLevel:       16,
		EvolvePokemonName: "CHARMELEON",
	}

	if err := EvolvePokemon(db, p, 5); err != nil {
		t.Fatalf("EvolvePokemon failed: %v", err)
	}

	if p.ID != 5 {
		t.Errorf("expected ID 5, got %d", p.ID)
	}
	if p.Name != "CHARMELEON" {
		t.Errorf("expected CHARMELEON, got %s", p.Name)
	}
	if p.MaxHP <= 30 {
		t.Errorf("expected MaxHP to increase from evolution, got %d", p.MaxHP)
	}
	if p.CurHP <= 30 {
		t.Errorf("expected CurHP to increase by HP gain, got %d", p.CurHP)
	}
	// Moves should be preserved
	if p.Moves[0].Name != "SCRATCH" || p.Moves[1].Name != "EMBER" {
		t.Errorf("expected moves to be preserved, got %s / %s", p.Moves[0].Name, p.Moves[1].Name)
	}
	// IVs/EVs should be preserved
	if p.IVs.Attack != 10 {
		t.Errorf("expected IVs preserved, got Attack IV=%d", p.IVs.Attack)
	}
	// Should now evolve into CHARIZARD at level 36
	if p.EvolveLevel != 36 || p.EvolvePokemonName != "CHARIZARD" {
		t.Errorf("expected next evolution at 36 CHARIZARD, got %d %s", p.EvolveLevel, p.EvolvePokemonName)
	}
}

func TestEventEvolutionType(t *testing.T) {
	if EventEvolution != "evolution" {
		t.Errorf("expected EventEvolution = 'evolution', got %q", EventEvolution)
	}
}

func openEvolutionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE phaser_pokemon (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			type_1 TEXT NOT NULL,
			type_2 TEXT,
			hp INTEGER NOT NULL,
			atk INTEGER NOT NULL,
			def INTEGER NOT NULL,
			spd INTEGER NOT NULL,
			spc INTEGER NOT NULL,
			catch_rate INTEGER NOT NULL,
			base_exp INTEGER NOT NULL,
			growth_rate TEXT NOT NULL,
			default_move_1_id TEXT,
			default_move_2_id TEXT,
			default_move_3_id TEXT,
			default_move_4_id TEXT,
			base_cry INTEGER,
			cry_pitch INTEGER,
			cry_length INTEGER,
			evolve_level INTEGER,
			evolve_pokemon TEXT
		);
		INSERT INTO phaser_pokemon (
			id, name, type_1, type_2, hp, atk, def, spd, spc,
			catch_rate, base_exp, growth_rate, evolve_level, evolve_pokemon
		) VALUES
			(5, 'CHARMELEON', 'FIRE', 'FIRE', 58, 64, 58, 80, 65, 45, 142, 'MEDIUM_SLOW', 36, 'CHARIZARD'),
			(6, 'CHARIZARD', 'FIRE', 'FLYING', 78, 84, 78, 100, 85, 45, 209, 'MEDIUM_SLOW', NULL, NULL);
	`)
	if err != nil {
		t.Fatalf("seed evolution db: %v", err)
	}
	return db
}
