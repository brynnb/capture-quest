package pokebattle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func partyDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'party'),(43,'other');
		INSERT INTO phaser_pokemon(id,name,type_1,hp,atk,def,spd,spc,catch_rate,base_exp,growth_rate) VALUES
		(1,'BULBASAUR','GRASS',45,49,49,45,65,45,64,'MEDIUM_SLOW'),
		(4,'CHARMANDER','FIRE',39,52,43,65,50,45,65,'MEDIUM_SLOW'),
		(7,'SQUIRTLE','WATER',44,48,65,43,50,45,66,'MEDIUM_SLOW');`)
	seedPCPokemon(t, database, 42, 0, BoxParty, 0, 1)
	seedPCPokemon(t, database, 42, 1, BoxParty, 1, 4)
	return database
}

func mustLoadParty(t *testing.T, database DBTX, charID int64) []*Pokemon {
	t.Helper()
	party, err := LoadParty(database, charID)
	if err != nil {
		t.Fatal(err)
	}
	return party
}

func TestPartySavePreservesIdentityNicknameAndStorage(t *testing.T) {
	database := partyDatabase(t)
	seedPCPokemon(t, database, 42, 0, 0, 0, 7)
	testdb.Exec(t, database, `UPDATE character_pokemon SET nickname='Buddy' WHERE character_id=42 AND party_slot=0;
		CREATE TABLE pokemon_reference(pokemon_id bigint REFERENCES character_pokemon(id));
		INSERT INTO pokemon_reference SELECT id FROM character_pokemon WHERE character_id=42;`)
	party := mustLoadParty(t, database, 42)
	firstID, secondID := party[0].RowID, party[1].RowID
	if firstID == 0 || party[0].Nickname != "Buddy" {
		t.Fatalf("loaded identity=%d nickname=%q", firstID, party[0].Nickname)
	}
	party[0].CurHP = 5
	party[0], party[1] = party[1], party[0]
	if err := SaveParty(database, 42, party); err != nil {
		t.Fatal(err)
	}
	saved := mustLoadParty(t, database, 42)
	if saved[0].RowID != secondID || saved[1].RowID != firstID || saved[1].CurHP != 5 || saved[1].Name != "Buddy" {
		t.Fatalf("reordered saved party: %+v / %+v", saved[0], saved[1])
	}
	box, err := LoadBox(database, 42, 0)
	if err != nil || len(box) != 1 || box[0].ID != 7 {
		t.Fatalf("PC changed: %v %v", box, err)
	}
}

func TestPartySaveRollsBackEveryRowAndUnpublishedIdentity(t *testing.T) {
	database := partyDatabase(t)
	party := mustLoadParty(t, database, 42)
	original := mustLoadParty(t, database, 42)
	testdb.Exec(t, database, `ALTER TABLE character_pokemon ADD CONSTRAINT reject_hp CHECK(cur_hp <> 999)`)
	newPokemon, err := LoadPokemonFromDB(database, 7)
	if err != nil {
		t.Fatal(err)
	}
	newPokemon.Level = 5
	newPokemon.RecalculateStats()
	newPokemon.CurHP = newPokemon.MaxHP
	party[0].CurHP = 1
	party[1].CurHP = 999
	party = append([]*Pokemon{newPokemon}, party...)
	if err := SaveParty(database, 42, party); err == nil {
		t.Fatal("accepted database failure")
	}
	if newPokemon.RowID != 0 {
		t.Fatalf("published uncommitted identity %d", newPokemon.RowID)
	}
	saved := mustLoadParty(t, database, 42)
	if len(saved) != 2 {
		t.Fatalf("party length=%d", len(saved))
	}
	for i := range saved {
		if saved[i].RowID != original[i].RowID || saved[i].CurHP != original[i].CurHP {
			t.Fatalf("row %d survived partial save: %+v", i, saved[i])
		}
	}
	party[2].CurHP = 10
	if err := SaveParty(database, 42, party); err != nil {
		t.Fatal(err)
	}
	if newPokemon.RowID == 0 {
		t.Fatal("successful commit did not publish new identity")
	}
	if err := SaveParty(database, 42, party); err != nil {
		t.Fatalf("repeated save recreated or lost identity: %v", err)
	}
}

func TestPartySaveRejectsStaleForeignDuplicateAndMissingRows(t *testing.T) {
	database := partyDatabase(t)
	seedPCPokemon(t, database, 43, 0, BoxParty, 0, 7)
	foreign := mustLoadParty(t, database, 43)[0]
	party := mustLoadParty(t, database, 42)
	for name, invalid := range map[string][]*Pokemon{
		"foreign":   {party[0], foreign},
		"duplicate": {party[0], party[0]},
		"missing":   {party[0]},
		"empty":     {},
		"nil":       {party[0], nil},
	} {
		t.Run(name, func(t *testing.T) {
			if err := SaveParty(database, 42, invalid); err == nil {
				t.Fatal("accepted invalid membership")
			}
		})
	}
	// A storage move after this snapshot makes the snapshot stale. It must never
	// pull the deposited row back into the party or overwrite its PC state.
	testdb.Exec(t, database, `UPDATE character_pokemon SET box=0,box_slot=0,party_slot=NULL WHERE id=$1`, party[1].RowID)
	if err := SaveParty(database, 42, party); err == nil {
		t.Fatal("accepted stale party")
	}
	assertPokemonStorage(t, database, 42, 4, sql.NullInt64{}, 0, 0)
}

func TestPartySaveParticipatesInOuterRollbackWithoutMutatingInput(t *testing.T) {
	database := partyDatabase(t)
	party := mustLoadParty(t, database, 42)
	added := *party[0]
	added.RowID = 0
	party = append(party, &added)
	if _, err := SavePartyInTransaction(database, 42, party); err == nil {
		t.Fatal("accepted unowned transaction")
	}
	err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		saved, err := SavePartyInTransaction(tx, 42, party)
		if err != nil {
			return err
		}
		if saved[2].RowID == 0 || added.RowID != 0 {
			t.Fatal("pending identities were not isolated")
		}
		return fmt.Errorf("later effect failed")
	})
	if err == nil || !strings.Contains(err.Error(), "later effect failed") {
		t.Fatalf("outer error=%v", err)
	}
	if len(mustLoadParty(t, database, 42)) != 2 || added.RowID != 0 {
		t.Fatal("outer rollback leaked the acquisition")
	}
}

func TestPartyLoadRejectsMalformedRowsInsteadOfDroppingMembers(t *testing.T) {
	database := partyDatabase(t)
	party := mustLoadParty(t, database, 42)
	rowID := party[1].RowID
	testdb.Exec(t, database, `UPDATE character_pokemon SET pokemon_id=999 WHERE id=$1`, rowID)
	if loaded, err := LoadParty(database, 42); loaded != nil || err == nil || !strings.Contains(err.Error(), fmt.Sprint(rowID)) {
		t.Fatalf("missing species silently dropped a party member: %v %v", loaded, err)
	}
	testdb.Exec(t, database, `UPDATE character_pokemon SET pokemon_id=4,move1_id=999 WHERE id=$1`, rowID)
	if loaded, err := LoadParty(database, 42); loaded != nil || err == nil || !strings.Contains(err.Error(), "move 999") {
		t.Fatalf("missing move silently disappeared: %v %v", loaded, err)
	}
	testdb.Exec(t, database, `UPDATE character_pokemon SET move1_id=0,party_slot=NULL WHERE id=$1`, rowID)
	if loaded, err := LoadParty(database, 42); loaded != nil || err == nil || !strings.Contains(err.Error(), "scan") {
		t.Fatalf("malformed row silently disappeared: %v %v", loaded, err)
	}
}

func TestStorageDepositRollsBackWhenCompactionFails(t *testing.T) {
	database := partyDatabase(t)
	seedPCPokemon(t, database, 42, 2, BoxParty, 2, 7)
	testdb.Exec(t, database, `ALTER TABLE character_pokemon ADD CONSTRAINT reject_compaction CHECK(NOT(pokemon_id=7 AND box=-1 AND party_slot=1))`)
	if slot, err := commitFixturePCDeposit(database, 42, fixturePokemonID(t, database, 42, 4), 0); err == nil || slot != -1 {
		t.Fatalf("reported partially committed deposit: slot=%d err=%v", slot, err)
	}
	assertPokemonStorage(t, database, 42, 4, sql.NullInt64{Int64: 1, Valid: true}, BoxParty, 1)
	assertPokemonStorage(t, database, 42, 7, sql.NullInt64{Int64: 2, Valid: true}, BoxParty, 2)
}

func TestConcurrentStorageCannotRemoveLastPartyPokemon(t *testing.T) {
	database := partyDatabase(t)
	target := fixturePokemonID(t, database, 42, 1)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := commitFixturePCDeposit(database, 42, target, 0); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 || len(mustLoadParty(t, database, 42)) != 1 {
		t.Fatalf("concurrent deposits: %d successes", successes.Load())
	}
	for _, box := range []int{BoxParty, BoxDayCare, 12} {
		if err := commitFixturePCRelease(database, 42, target, box); err == nil {
			t.Fatalf("PC release accepted box=%d", box)
		}
		if _, err := commitFixturePCWithdrawal(database, 42, target, box); err == nil {
			t.Fatalf("PC withdrawal accepted box=%d", box)
		}
	}
}

func TestConcurrentAcquisitionPlacesEveryPokemonWithoutOverfillingParty(t *testing.T) {
	database := partyDatabase(t)
	for slot := 2; slot < 5; slot++ {
		seedPCPokemon(t, database, 42, slot, BoxParty, slot, 7)
	}
	pokemon := *mustLoadParty(t, database, 42)[0]
	pokemon.RowID = 0
	var successes, inParty atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := pokemon
			party, _, _, err := SavePreparedPokemonToPartyOrPC(database, 42, &copy)
			if err != nil {
				t.Error(err)
				return
			}
			successes.Add(1)
			if party {
				inParty.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 4 || inParty.Load() != 1 || len(mustLoadParty(t, database, 42)) != 6 {
		t.Fatalf("acquisitions=%d party additions=%d", successes.Load(), inParty.Load())
	}
	box, err := LoadBox(database, 42, 0)
	if err != nil || len(box) != 3 {
		t.Fatalf("PC acquisitions: %d, %v", len(box), err)
	}
}
