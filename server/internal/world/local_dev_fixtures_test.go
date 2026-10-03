package world

import (
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
	"context"
	"errors"
	"testing"
)

func TestLocalCreationPartyPreservesSavedBattleIdentity(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	current := battleTestStart(t, database, true, nil)
	// Repeated setup must retain damage and row IDs, without needing fixture data.
	if err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		return seedLocalDevPokemonPartyIn(tx, 42)
	}); err != nil {
		t.Fatal(err)
	}
	restored, err := pokebattle.LoadBattleState(database, 42)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.RestoreParty(database, 42); err != nil {
		t.Fatal(err)
	}
	if restored.BattleID != current.BattleID || restored.PlayerParty[0].CurHP != 1 || restored.PlayerParty[0].RowID != current.PlayerParty[0].RowID {
		t.Fatal("repeated creation setup changed the saved battle party")
	}
}

func TestLocalCreationFixturesCommitAndRollbackTogether(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	if err := seedLocalDevCreationFixturesIn(database, 42); err == nil {
		t.Fatal("creation fixtures accepted a nontransactional database")
	}
	testdb.Exec(t, database, `DELETE FROM character_pokemon WHERE character_id=42;
 INSERT INTO phaser_pokemon(id,name,type_1,hp,atk,def,spd,spc,catch_rate,base_exp,growth_rate)
 SELECT id,'TEST','NORMAL',35,55,30,90,50,190,82,'MEDIUM_FAST' FROM unnest(ARRAY[4,7,1,16,39]) id;`)
	rejected := errors.New("outer creation rejected")
	err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		if err := seedLocalDevCreationFixturesIn(tx, 42); err != nil {
			return err
		}
		return rejected
	})
	if !errors.Is(err, rejected) {
		t.Fatalf("setup failed before outer rejection: %v", err)
	}
	var party, inventory int
	readCounts := func() {
		t.Helper()
		if err := database.QueryRow(`SELECT (SELECT COUNT(*) FROM character_pokemon WHERE character_id=42), (SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42)`).Scan(&party, &inventory); err != nil {
			t.Fatal(err)
		}
	}
	readCounts()
	if party != 0 || inventory != 0 {
		t.Fatalf("partial creation escaped rollback: party=%d inventory=%d", party, inventory)
	}
	if err := db.Transaction(context.Background(), database, func(tx db.DBTX) error { return seedLocalDevCreationFixturesIn(tx, 42) }); err != nil {
		t.Fatal(err)
	}
	readCounts()
	if party != 6 || inventory == 0 {
		t.Fatalf("creation fixture incomplete: party=%d inventory=%d", party, inventory)
	}
}
