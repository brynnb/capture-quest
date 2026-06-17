package pokebattle

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDepositToPCMovesPokemonAndCompactsParty(t *testing.T) {
	db := openPCTestDB(t)
	seedPCPokemon(t, db, 42, 0, BoxParty, 0, 1)
	seedPCPokemon(t, db, 42, 1, BoxParty, 1, 4)
	seedPCPokemon(t, db, 42, 2, BoxParty, 2, 7)

	boxSlot, err := DepositToPC(db, 42, 1, 0)
	if err != nil {
		t.Fatalf("DepositToPC failed: %v", err)
	}
	if boxSlot != 0 {
		t.Fatalf("box slot = %d, want 0", boxSlot)
	}

	assertPokemonStorage(t, db, 42, 4, sql.NullInt64{}, 0, 0)
	assertPokemonStorage(t, db, 42, 1, sql.NullInt64{Int64: 0, Valid: true}, BoxParty, 0)
	assertPokemonStorage(t, db, 42, 7, sql.NullInt64{Int64: 1, Valid: true}, BoxParty, 1)
}

func TestWithdrawFromPCUsesFirstOpenPartySlot(t *testing.T) {
	db := openPCTestDB(t)
	seedPCPokemon(t, db, 42, 0, BoxParty, 0, 1)
	seedPCPokemon(t, db, 42, 2, BoxParty, 2, 4)
	seedPCPokemon(t, db, 42, 0, 0, 0, 7)

	partySlot, err := WithdrawFromPC(db, 42, 0, 0)
	if err != nil {
		t.Fatalf("WithdrawFromPC failed: %v", err)
	}
	if partySlot != 1 {
		t.Fatalf("party slot = %d, want first open slot 1", partySlot)
	}
	assertPokemonStorage(t, db, 42, 7, sql.NullInt64{Int64: 1, Valid: true}, BoxParty, 1)
}

func TestWithdrawFromPCRejectsFullParty(t *testing.T) {
	db := openPCTestDB(t)
	for slot := 0; slot < 6; slot++ {
		seedPCPokemon(t, db, 42, slot, BoxParty, slot, 1)
	}
	seedPCPokemon(t, db, 42, 0, 0, 0, 7)

	if _, err := WithdrawFromPC(db, 42, 0, 0); err == nil {
		t.Fatal("expected full party error")
	}
}

func TestReleasePokemonRequiresExistingPCPokemon(t *testing.T) {
	db := openPCTestDB(t)
	seedPCPokemon(t, db, 42, 0, 0, 0, 7)

	if err := ReleasePokemon(db, 42, 0, 0); err != nil {
		t.Fatalf("ReleasePokemon failed: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id = 42 AND pokemon_id = 7`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("released pokemon count = %d, want 0", count)
	}

	if err := ReleasePokemon(db, 42, 0, 0); err == nil {
		t.Fatal("expected missing pokemon error")
	}
}

func openPCTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
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
		CREATE TABLE character_pokemon (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			character_id INTEGER NOT NULL,
			party_slot INTEGER,
			box INTEGER NOT NULL DEFAULT -1,
			box_slot INTEGER NOT NULL DEFAULT -1,
			pokemon_id INTEGER NOT NULL,
			nickname TEXT DEFAULT '',
			level INTEGER NOT NULL DEFAULT 5,
			exp INTEGER NOT NULL DEFAULT 0,
			growth_rate TEXT NOT NULL DEFAULT 'MEDIUM_FAST',
			cur_hp INTEGER NOT NULL,
			max_hp INTEGER NOT NULL,
			iv_atk INTEGER NOT NULL DEFAULT 0,
			iv_def INTEGER NOT NULL DEFAULT 0,
			iv_spd INTEGER NOT NULL DEFAULT 0,
			iv_spc INTEGER NOT NULL DEFAULT 0,
			ev_hp INTEGER NOT NULL DEFAULT 0,
			ev_atk INTEGER NOT NULL DEFAULT 0,
			ev_def INTEGER NOT NULL DEFAULT 0,
			ev_spd INTEGER NOT NULL DEFAULT 0,
			ev_spc INTEGER NOT NULL DEFAULT 0,
			move1_id INTEGER NOT NULL DEFAULT 0,
			move1_pp INTEGER NOT NULL DEFAULT 0,
			move1_pp_up INTEGER NOT NULL DEFAULT 0,
			move2_id INTEGER NOT NULL DEFAULT 0,
			move2_pp INTEGER NOT NULL DEFAULT 0,
			move2_pp_up INTEGER NOT NULL DEFAULT 0,
			move3_id INTEGER NOT NULL DEFAULT 0,
			move3_pp INTEGER NOT NULL DEFAULT 0,
			move3_pp_up INTEGER NOT NULL DEFAULT 0,
			move4_id INTEGER NOT NULL DEFAULT 0,
			move4_pp INTEGER NOT NULL DEFAULT 0,
			move4_pp_up INTEGER NOT NULL DEFAULT 0,
			status INTEGER NOT NULL DEFAULT 0,
			original_trainer_id INTEGER,
			UNIQUE (character_id, box, box_slot),
			UNIQUE (character_id, party_slot)
		);
		INSERT INTO phaser_pokemon (
			id, name, type_1, type_2, hp, atk, def, spd, spc, catch_rate, base_exp, growth_rate
		) VALUES
			(1, 'BULBASAUR', 'GRASS', 'POISON', 45, 49, 49, 45, 65, 45, 64, 'MEDIUM_SLOW'),
			(4, 'CHARMANDER', 'FIRE', 'FIRE', 39, 52, 43, 65, 50, 45, 65, 'MEDIUM_SLOW'),
			(7, 'SQUIRTLE', 'WATER', 'WATER', 44, 48, 65, 43, 50, 45, 66, 'MEDIUM_SLOW');
	`); err != nil {
		t.Fatalf("seed PC db: %v", err)
	}
	return db
}

func seedPCPokemon(t *testing.T, db *sql.DB, charID int64, partySlot int, box int, boxSlot int, speciesID int) {
	t.Helper()
	var partySlotValue interface{}
	if box == BoxParty {
		partySlotValue = partySlot
	} else {
		partySlotValue = nil
	}
	if _, err := db.Exec(`
		INSERT INTO character_pokemon (
			character_id, party_slot, box, box_slot, pokemon_id, nickname,
			level, exp, growth_rate, cur_hp, max_hp,
			iv_atk, iv_def, iv_spd, iv_spc,
			ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
			status, original_trainer_id
		)
		VALUES ($1, $2, $3, $4, $5, '', 10, 0, 'MEDIUM_SLOW', 20, 20, 10, 10, 10, 10, 0, 0, 0, 0, 0, 0, $1)`,
		charID, partySlotValue, box, boxSlot, speciesID,
	); err != nil {
		t.Fatalf("seed pokemon %d: %v", speciesID, err)
	}
}

func assertPokemonStorage(t *testing.T, db *sql.DB, charID int64, speciesID int, wantPartySlot sql.NullInt64, wantBox int, wantBoxSlot int) {
	t.Helper()
	var gotPartySlot sql.NullInt64
	var gotBox, gotBoxSlot int
	if err := db.QueryRow(`
		SELECT party_slot, box, box_slot
		FROM character_pokemon
		WHERE character_id = $1 AND pokemon_id = $2`, charID, speciesID,
	).Scan(&gotPartySlot, &gotBox, &gotBoxSlot); err != nil {
		t.Fatalf("query pokemon %d: %v", speciesID, err)
	}
	if gotPartySlot.Valid != wantPartySlot.Valid || gotPartySlot.Int64 != wantPartySlot.Int64 || gotBox != wantBox || gotBoxSlot != wantBoxSlot {
		t.Fatalf(
			"pokemon %d storage = party_slot(%d,%v) box %d slot %d, want party_slot(%d,%v) box %d slot %d",
			speciesID,
			gotPartySlot.Int64,
			gotPartySlot.Valid,
			gotBox,
			gotBoxSlot,
			wantPartySlot.Int64,
			wantPartySlot.Valid,
			wantBox,
			wantBoxSlot,
		)
	}
}
