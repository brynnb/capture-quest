package pokebattle

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLoadPokemonFromDBResolvesDefaultMoveConstants(t *testing.T) {
	db := openDefaultMoveTestDB(t)
	seedDefaultMoveTestPokemon(t, db, "POISON_STING")

	pokemon, err := LoadPokemonFromDB(db, 13)
	if err != nil {
		t.Fatalf("LoadPokemonFromDB: %v", err)
	}
	if pokemon.Moves[0].ID != 40 || pokemon.Moves[0].Name != "POISON_STING" || pokemon.Moves[0].PP != 35 {
		t.Fatalf("default move = %#v, want Poison Sting id=40 pp=35", pokemon.Moves[0])
	}
}

func TestLoadPokemonFromDBRejectsUnknownDefaultMoveConstant(t *testing.T) {
	db := openDefaultMoveTestDB(t)
	seedDefaultMoveTestPokemon(t, db, "MISSING_MOVE")

	_, err := LoadPokemonFromDB(db, 13)
	if err == nil || !strings.Contains(err.Error(), `resolve default move "MISSING_MOVE"`) {
		t.Fatalf("LoadPokemonFromDB error = %v, want unresolved default move error", err)
	}
}

func TestRepairPokemonWithNoMovesRestoresLegalMoves(t *testing.T) {
	db := openDefaultMoveTestDB(t)
	seedDefaultMoveTestPokemon(t, db, "POISON_STING")
	if _, err := db.Exec(`
		UPDATE phaser_pokemon SET default_move_2_id = 'STRING_SHOT' WHERE id = 13;
		INSERT INTO character_pokemon (
			id, pokemon_id, level, move1_id, move1_pp, move2_id, move2_pp,
			move3_id, move3_pp, move4_id, move4_pp
		) VALUES (7, 13, 5, 0, 0, 0, 0, 0, 0, 0, 0);
	`); err != nil {
		t.Fatalf("seed persisted Weedle: %v", err)
	}

	repaired, err := RepairPokemonWithNoMoves(db)
	if err != nil {
		t.Fatalf("RepairPokemonWithNoMoves: %v", err)
	}
	if repaired != 1 {
		t.Fatalf("repaired = %d, want 1", repaired)
	}

	var move1ID, move1PP, move2ID, move2PP int
	if err := db.QueryRow(`
		SELECT move1_id, move1_pp, move2_id, move2_pp
		FROM character_pokemon WHERE id = 7
	`).Scan(&move1ID, &move1PP, &move2ID, &move2PP); err != nil {
		t.Fatal(err)
	}
	if move1ID != 40 || move1PP != 35 || move2ID != 81 || move2PP != 40 {
		t.Fatalf("repaired moves = (%d,%d), (%d,%d), want (40,35), (81,40)", move1ID, move1PP, move2ID, move2PP)
	}

	repaired, err = RepairPokemonWithNoMoves(db)
	if err != nil {
		t.Fatalf("second RepairPokemonWithNoMoves: %v", err)
	}
	if repaired != 0 {
		t.Fatalf("second repaired = %d, want idempotent 0", repaired)
	}
}

func openDefaultMoveTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE phaser_pokemon (
			id INTEGER PRIMARY KEY, name TEXT NOT NULL, type_1 TEXT NOT NULL, type_2 TEXT,
			hp INTEGER NOT NULL, atk INTEGER NOT NULL, def INTEGER NOT NULL,
			spd INTEGER NOT NULL, spc INTEGER NOT NULL, catch_rate INTEGER NOT NULL,
			base_exp INTEGER NOT NULL, growth_rate TEXT NOT NULL,
			default_move_1_id TEXT, default_move_2_id TEXT,
			default_move_3_id TEXT, default_move_4_id TEXT,
			base_cry INTEGER, cry_pitch INTEGER, cry_length INTEGER,
			evolve_level INTEGER, evolve_pokemon TEXT
		);
		CREATE TABLE phaser_moves (
			id INTEGER PRIMARY KEY, name TEXT NOT NULL, constant_name TEXT NOT NULL,
			short_name TEXT NOT NULL,
			type TEXT, power INTEGER, accuracy INTEGER, pp INTEGER, effect TEXT,
			battle_sound TEXT, battle_sound_pitch INTEGER, battle_sound_tempo INTEGER
		);
		CREATE TABLE phaser_pokemon_learnset (
			pokemon_id INTEGER NOT NULL, level INTEGER NOT NULL,
			move_id INTEGER NOT NULL, move_name TEXT NOT NULL
		);
		CREATE TABLE character_pokemon (
			id INTEGER PRIMARY KEY, pokemon_id INTEGER NOT NULL, level INTEGER NOT NULL,
			move1_id INTEGER NOT NULL DEFAULT 0, move1_pp INTEGER NOT NULL DEFAULT 0,
			move2_id INTEGER NOT NULL DEFAULT 0, move2_pp INTEGER NOT NULL DEFAULT 0,
			move3_id INTEGER NOT NULL DEFAULT 0, move3_pp INTEGER NOT NULL DEFAULT 0,
			move4_id INTEGER NOT NULL DEFAULT 0, move4_pp INTEGER NOT NULL DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO phaser_moves (
			id, name, constant_name, short_name, type, power, accuracy, pp
		) VALUES
			(40, 'POISON STING', 'POISON_STING', 'POISON_STING', 'POISON', 15, 255, 35),
			(81, 'STRING SHOT', 'STRING_SHOT', 'STRING_SHOT', 'BUG', 0, 242, 40);
	`); err != nil {
		t.Fatalf("create default move test db: %v", err)
	}
	return db
}

func seedDefaultMoveTestPokemon(t *testing.T, db *sql.DB, defaultMove string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO phaser_pokemon (
			id, name, type_1, type_2, hp, atk, def, spd, spc,
			catch_rate, base_exp, growth_rate, default_move_1_id
		) VALUES (13, 'WEEDLE', 'BUG', 'POISON', 40, 35, 30, 50, 20, 255, 52, 'MEDIUM_FAST', $1)
	`, defaultMove); err != nil {
		t.Fatalf("seed pokemon: %v", err)
	}
}
