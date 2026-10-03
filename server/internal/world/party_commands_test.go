package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"sync"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func partyCommandWorld(t *testing.T) (*sql.DB, *WorldHandler, *session.Session, *recordingMessenger, []int64) {
	t.Helper()
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,cur_hp,max_hp,nickname)
 VALUES(42,1,1,129,5,10,18,'second'),(42,2,2,25,5,10,18,'third')`)
	party, err := pokebattle.LoadParty(database, 42)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, len(party))
	for i, p := range party {
		ids[i] = p.RowID
	}
	// The transport must use the injected pool, including its domain callback.
	db.GlobalWorldDB = nil
	return database, wh, ses, messages, ids
}

func reorderPayload(t *testing.T, revision int64, ids []int64) string {
	t.Helper()
	payload, err := json.Marshal(PokemonPartyReorderRequest{RequestID: "reorder-test", Command: &InventoryCommandIdentity{CharacterID: 42, Revision: &revision}, PokemonIDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func assertPartyOrder(t *testing.T, database *sql.DB, ids []int64, revision int64) {
	t.Helper()
	party, err := pokebattle.LoadParty(database, 42)
	if err != nil {
		t.Fatal(err)
	}
	actual := make([]int64, len(party))
	for i, p := range party {
		actual[i] = p.RowID
	}
	if !reflect.DeepEqual(actual, ids) {
		t.Fatalf("party order=%v want=%v", actual, ids)
	}
	var rev int64
	if err := database.QueryRow(`SELECT COALESCE((SELECT revision FROM character_shop_state WHERE character_id=42),0)`).Scan(&rev); err != nil || rev != revision {
		t.Fatalf("revision=%d want=%d error=%v", rev, revision, err)
	}
}

func TestPartyReorderDispatcherCommitsOnceWithStableIdentity(t *testing.T) {
	database, wh, ses, messages, ids := partyCommandWorld(t)
	ordered := []int64{ids[1], ids[2], ids[0]}
	request := reorderPayload(t, 0, ordered)
	battleDispatch(t, wh, ses, opcodes.PokemonPartyReorderRequest, request)
	if len(messages.streams) != 1 {
		t.Fatalf("split publication: %+v", messages.streams)
	}
	var result PokemonPartyReorderResponse
	if err := json.Unmarshal(messages.streams[0].payload, &result); err != nil || !result.Success || result.RequestID != "reorder-test" || result.Inventory.CommandRevision != 1 || result.Inventory.Money != 100 || len(result.Party) != 3 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for i, p := range result.Party {
		if p.RowID != ordered[i] {
			t.Fatalf("wrong committed party: %+v", result.Party)
		}
	}
	for _, duplicate := range []string{request, `{"order":[1,2,0]}`} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.PokemonPartyReorderRequest, duplicate)
		var result InventoryCommandError
		if len(messages.streams) != 1 {
			t.Fatal("missing rejection")
		}
		if err := json.Unmarshal(messages.streams[0].payload, &result); err != nil || result.Success || result.Error == "" {
			t.Fatalf("duplicate accepted: %+v %v", result, err)
		}
		assertPartyOrder(t, database, ordered, 1)
	}
	// A subsequent inventory-family command shares this same admission revision.
	if _, err := cqitems.NewStore(database).ExecuteCommand(context.Background(), 42, 1, func(db.DBTX) error { return nil }); err != nil {
		t.Fatal(err)
	}
	battleDispatch(t, wh, ses, opcodes.PokemonPartyReorderRequest, reorderPayload(t, 1, ids))
	assertPartyOrder(t, database, ordered, 2)
	battleDispatch(t, wh, ses, opcodes.PokemonPartyReorderRequest, reorderPayload(t, 2, ids))
	assertPartyOrder(t, database, ids, 3)
}

func TestPartyReorderRejectsInvalidMembershipAndRollsBackAllStages(t *testing.T) {
	for _, stage := range []string{"duplicate IDs", "missing member", "foreign member", "character", "domain write", "projection", "commit"} {
		t.Run(stage, func(t *testing.T) {
			database, wh, ses, messages, ids := partyCommandWorld(t)
			ordered := []int64{ids[1], ids[2], ids[0]}
			switch stage {
			case "duplicate IDs":
				ordered[2] = ordered[0]
			case "missing member":
				ordered = ordered[:2]
			case "foreign member":
				testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(43,'other'); INSERT INTO character_pokemon(id,character_id,party_slot,box_slot,pokemon_id,level,cur_hp,max_hp) VALUES(999,43,0,0,25,5,10,18)`)
				ordered[2] = 999
			case "domain write":
				testdb.Exec(t, database, `CREATE FUNCTION reject_party_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.party_slot=1 THEN RAISE EXCEPTION 'reject party write'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_party_write BEFORE UPDATE ON character_pokemon FOR EACH ROW EXECUTE FUNCTION reject_party_write()`)
			case "projection":
				testdb.Exec(t, database, `ALTER TABLE character_wallet RENAME TO unavailable_wallet`)
			case "commit":
				testdb.Exec(t, database, `CREATE FUNCTION reject_party_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject party commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_party_commit AFTER UPDATE ON character_pokemon DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_party_commit()`)
			}
			request := reorderPayload(t, 0, ordered)
			if stage == "character" {
				request = `{"requestId":"reorder-test","command":{"characterId":43,"revision":0},"pokemonIds":[1,2,3]}`
			}
			battleDispatch(t, wh, ses, opcodes.PokemonPartyReorderRequest, request)
			var result InventoryCommandError
			if len(messages.streams) != 1 {
				t.Fatal("unexpected failure publication")
			}
			if err := json.Unmarshal(messages.streams[0].payload, &result); err != nil || result.Success || result.RequestID != "reorder-test" || result.Error == "" {
				t.Fatalf("failure=%+v %v", result, err)
			}
			assertPartyOrder(t, database, ids, 0)
		})
	}
}

func TestPartyReorderCancelsBlockedCharacterWithoutMutation(t *testing.T) {
	database, wh, ses, _, ids := partyCommandWorld(t)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if err := db.LockCharacter(lock, 42); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = ses.ExecuteCommand(ctx, func() {
		HandlePokemonPartyReorder(ses, []byte(reorderPayload(t, 0, []int64{ids[1], ids[2], ids[0]})), wh)
	})
	if err == nil {
		t.Fatal("blocked reorder did not observe cancellation")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertPartyOrder(t, database, ids, 0)
}

func TestPartyReorderConcurrentDuplicatesShareRevision(t *testing.T) {
	database, wh, ses, _, ids := partyCommandWorld(t)
	ordered := []int64{ids[1], ids[2], ids[0]}
	payload := []byte(reorderPayload(t, 0, ordered))
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			other := &session.Session{Client: ses.Client, Messenger: &recordingMessenger{}}
			HandlePokemonPartyReorder(other, payload, wh)
		}()
	}
	wg.Wait()
	assertPartyOrder(t, database, ordered, 1)
}

func TestPartyReorderRespectsDurableBattleOwnership(t *testing.T) {
	for _, state := range []string{"active", "terminal", "safari"} {
		t.Run(state, func(t *testing.T) {
			database, wh, ses, _, ids := partyCommandWorld(t)
			if state == "safari" {
				seedSafariBattle(t, NewSafariZoneManager(database), 7)
			} else {
				battle := battleTestStart(t, database, false, func(b *pokebattle.BattleState) {
					if state == "terminal" {
						b.Phase = pokebattle.PhaseBattleEnd
					}
				})
				forgetBattle(42, battle)
			}
			battleDispatch(t, wh, ses, opcodes.PokemonPartyReorderRequest, reorderPayload(t, 0, []int64{ids[1], ids[2], ids[0]}))
			assertPartyOrder(t, database, ids, 0)
		})
	}
}
