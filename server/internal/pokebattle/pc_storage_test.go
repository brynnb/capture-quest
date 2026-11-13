package pokebattle

import (
	"capturequest/internal/db"
	"context"
	"database/sql"
	"testing"

	"capturequest/internal/testdb"
)

func TestDepositToPCMovesPokemonAndCompactsParty(t *testing.T) {
	db := openPCTestDB(t)
	seedPCPokemon(t, db, 42, 0, BoxParty, 0, 1)
	seedPCPokemon(t, db, 42, 1, BoxParty, 1, 4)
	seedPCPokemon(t, db, 42, 2, BoxParty, 2, 7)

	boxSlot, err := commitFixturePCDeposit(db, 42, fixturePokemonID(t, db, 42, 4), 0)
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

	partySlot, err := commitFixturePCWithdrawal(db, 42, fixturePokemonID(t, db, 42, 7), 0)
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

	if _, err := commitFixturePCWithdrawal(db, 42, fixturePokemonID(t, db, 42, 7), 0); err == nil {
		t.Fatal("expected full party error")
	}
}

func TestReleasePokemonRequiresExistingPCPokemon(t *testing.T) {
	db := openPCTestDB(t)
	seedPCPokemon(t, db, 42, 0, 0, 0, 7)

	target := fixturePokemonID(t, db, 42, 7)

	if err := commitFixturePCRelease(db, 42, target, 0); err != nil {
		t.Fatalf("ReleasePokemon failed: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id = 42 AND pokemon_id = 7`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("released pokemon count = %d, want 0", count)
	}

	if err := commitFixturePCRelease(db, 42, target, 0); err == nil {
		t.Fatal("expected missing pokemon error")
	}
}

func openPCTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'pc-storage');
 INSERT INTO phaser_pokemon(id,name,type_1,type_2,hp,atk,def,spd,spc,catch_rate,base_exp,growth_rate) VALUES
 (1,'BULBASAUR','GRASS','POISON',45,49,49,45,65,45,64,'MEDIUM_SLOW'),
 (4,'CHARMANDER','FIRE','FIRE',39,52,43,65,50,45,65,'MEDIUM_SLOW'),
 (7,'SQUIRTLE','WATER','WATER',44,48,65,43,50,45,66,'MEDIUM_SLOW')`)
	return database
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

// Fixture transactions exercise the stable-ID primitive; they never implement
// slot intent or resolve a live target after admission.
func fixturePokemonID(t *testing.T, database DBTX, charID int64, species int) int64 {
	t.Helper()
	var id int64
	if err := database.QueryRow(`SELECT id FROM character_pokemon WHERE character_id=$1 AND pokemon_id=$2 ORDER BY id LIMIT 1`, charID, species).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func commitFixturePCDeposit(database DBTX, charID, rowID int64, box int) (int, error) {
	slot := -1
	err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		var err error
		slot, err = DepositPokemonToPCInTransaction(tx, charID, rowID, box)
		return err
	})
	if err != nil {
		return -1, err
	}
	return slot, nil
}
func commitFixturePCWithdrawal(database DBTX, charID, rowID int64, box int) (int, error) {
	slot := -1
	err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		var err error
		slot, err = WithdrawPokemonFromPCInTransaction(tx, charID, rowID, box)
		return err
	})
	if err != nil {
		return -1, err
	}
	return slot, nil
}
func commitFixturePCRelease(database DBTX, charID, rowID int64, box int) error {
	return db.Transaction(context.Background(), database, func(tx db.DBTX) error { return ReleasePokemonFromPCInTransaction(tx, charID, rowID, box) })
}
