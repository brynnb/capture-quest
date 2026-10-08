package world

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	model "capturequest/internal/db/models"
	"capturequest/internal/protocol"
	"capturequest/internal/testdb"
)

func TestPrivatePokedexReadsRejectZeroCharacterIdentity(t *testing.T) {
	_, wh, ses, messages := battleTestWorld(t)
	ses.Client = &testSessionClient{char: &model.CharacterData{}}
	HandlePokedexStatusRequest(ses, nil, wh)
	HandleTrainerCardRequest(ses, nil, wh)
	if len(messages.streams) != 2 {
		t.Fatal("invalid character identity omitted failure")
	}
	assertPokedexWireFailure(t, messages.streams[0], opcodes.PokedexStatusResponse, 0)
	assertPokedexWireFailure(t, messages.streams[1], opcodes.TrainerCardResponse, 0)
}

func TestPokedexReadRepliesEchoRequestAndSelectedCharacter(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	for _, opcode := range []opcodes.OpCode{opcodes.PokedexListRequest, opcodes.PokedexStatusRequest, opcodes.TrainerCardRequest} {
		battleDispatch(t, wh, ses, opcode, `{"requestId":"pokedex:owned"}`)
		var reply struct {
			RequestID   string `json:"requestId"`
			CharacterID int64  `json:"characterId"`
			Success     bool   `json:"success"`
		}
		if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &reply); err != nil || !reply.Success || reply.RequestID != "pokedex:owned" || reply.CharacterID != 42 {
			t.Fatalf("read identity=%+v error=%v", reply, err)
		}
	}
	testdb.Exec(t, database, `DROP TABLE character_wallet`)
	battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{"requestId":"pokedex:failed"}`)
	var failure protocol.PokedexReadError
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure); err != nil || failure.Success || failure.RequestID != "pokedex:failed" || failure.CharacterID != 42 || failure.Error == "" {
		t.Fatalf("failed identity=%+v error=%v", failure, err)
	}
}

func TestTrainerCardKeepsOneSnapshotAcrossConcurrentPublication(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	writer, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.Exec(`LOCK TABLE character_event_flags IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{}`) }()
	deadline := time.Now().Add(time.Second)
	for {
		var waiting bool
		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='character_event_flags'::regclass AND NOT granted)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			writer.Rollback()
			<-done
			t.Fatal("card did not reach its owned flag read")
		}
		runtime.Gosched()
	}
	if _, err := writer.Exec(`UPDATE character_wallet SET pokedollars=200 WHERE character_id=42;
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_BOULDERBADGE');
 INSERT INTO character_pokedex(character_id,pokemon_id,seen,caught) VALUES(42,129,1,1)`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	<-done
	var card protocol.TrainerCardResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &card) != nil || !card.Success || card.Money != 100 || card.BadgeCount != 0 || card.PokedexCaught != 1 {
		t.Fatalf("mixed card publication=%+v", card)
	}
	battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{}`)
	if len(messages.streams) != 2 || json.Unmarshal(messages.streams[1].payload, &card) != nil || !card.Success || card.Money != 200 || card.BadgeCount != 1 || card.PokedexCaught != 2 {
		t.Fatalf("fresh card missed durable publication=%+v", card)
	}
}

func TestPokedexRepairCommitFailureRejectsResponseAndRollsBack(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `CREATE FUNCTION reject_pokedex_repair() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject repair'; END $$;
 CREATE CONSTRAINT TRIGGER reject_pokedex_repair AFTER INSERT ON character_pokedex DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_pokedex_repair();`)
	battleDispatch(t, wh, ses, opcodes.PokedexStatusRequest, `{}`)
	if len(messages.streams) != 1 {
		t.Fatal("repair failure omitted response")
	}
	assertPokedexWireFailure(t, messages.streams[0], opcodes.PokedexStatusResponse, 42)
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_pokedex WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("repair partially committed: count=%d error=%v", count, err)
	}
}

func TestPokedexReadDoesNotRewriteAlreadyCompleteOwnedStatus(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO character_pokedex(character_id,pokemon_id,seen,caught,first_seen_at,first_caught_at) VALUES(42,25,1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
 CREATE FUNCTION reject_redundant_pokedex_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'redundant pokedex write'; END $$;
 CREATE CONSTRAINT TRIGGER reject_redundant_pokedex_write AFTER UPDATE ON character_pokedex DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_redundant_pokedex_write();`)
	battleDispatch(t, wh, ses, opcodes.PokedexStatusRequest, `{}`)
	var response protocol.PokedexStatusResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || !response.Success || len(response.Status) != 1 || !response.Status[0].Caught {
		t.Fatalf("complete status was unnecessarily rewritten: %+v", response)
	}
}

func TestPokedexStatusCancellationDoesNotWaitForPoolOrRepair(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = ses.ExecuteCommand(ctx, func() { HandlePokedexStatusRequest(ses, nil, wh) })
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		lease.Close()
		<-done
		t.Fatal("status read ignored caller pool deadline")
	}
	lease.Close()
	if len(messages.streams) != 1 {
		t.Fatal("cancelled read omitted terminal response")
	}
	assertPokedexWireFailure(t, messages.streams[0], opcodes.PokedexStatusResponse, 42)
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_pokedex WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cancelled read repaired pokedex: count=%d error=%v", count, err)
	}
}

func TestTrainerCardUsesOwnedEmptyWalletPolicyAndPreservesSQLFailure(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `DELETE FROM character_wallet WHERE character_id=42`)
	db.GlobalWorldDB = nil
	battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{}`)
	var card struct {
		Success bool `json:"success"`
		Money   int  `json:"money"`
	}
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &card) != nil || !card.Success || card.Money != 0 {
		t.Fatal("empty wallet did not return a zero-balance trainer card")
	}
	testdb.Exec(t, database, `DROP TABLE character_wallet`)
	battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{}`)
	if len(messages.streams) != 2 {
		t.Fatal("wallet SQL failure omitted terminal response")
	}
	assertPokedexWireFailure(t, messages.streams[1], opcodes.TrainerCardResponse, 42)
}

func TestPokedexAndTrainerCardWireContracts(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	ses.Client.CharData().Name = "Red"
	testdb.Exec(t, database, `UPDATE phaser_pokemon SET base_cry=18, cry_pitch=0, cry_length=255 WHERE id=25`)
	// These query handlers must use their world's injected database.
	db.GlobalWorldDB = nil
	battleDispatch(t, wh, ses, opcodes.PokedexListRequest, `{}`)
	if len(messages.streams) != 1 {
		t.Fatalf("list messages=%d", len(messages.streams))
	}
	var list struct {
		Success bool             `json:"success"`
		Species []map[string]any `json:"species"`
		Status  []map[string]any `json:"status"`
	}
	if err := json.Unmarshal(messages.streams[0].payload, &list); err != nil {
		t.Fatal(err)
	}
	if messages.streams[0].opcode != opcodes.PokedexListResponse || !list.Success || len(list.Species) != 2 || len(list.Status) != 1 {
		t.Fatalf("list=%+v", list)
	}
	pikachu := list.Species[0]
	if pikachu["crySfx"] != "SFX_CRY_12" || pikachu["cryPitch"] != float64(0) || pikachu["cryLength"] != float64(255) {
		t.Fatalf("cry metadata=%v", pikachu)
	}
	if _, exists := pikachu["crySFX"]; exists {
		t.Fatal("legacy acronym field leaked")
	}
	if value, exists := pikachu["type2"]; !exists || value != nil {
		t.Fatalf("nullable type2=%v exists=%t", value, exists)
	}
	if _, exists := list.Species[1]["crySfx"]; exists {
		t.Fatal("missing cry was not omitted")
	}
	if list.Status[0]["pokemonId"] != float64(25) || list.Status[0]["caught"] != true {
		t.Fatalf("status=%v", list.Status)
	}

	battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{}`)
	if len(messages.streams) != 2 || messages.streams[1].opcode != opcodes.TrainerCardResponse {
		t.Fatalf("card messages=%v", messages.streams)
	}
	var card map[string]any
	if err := json.Unmarshal(messages.streams[1].payload, &card); err != nil {
		t.Fatal(err)
	}
	if card["success"] != true || card["name"] != "Red" || card["money"] != float64(100) || card["pokedexCaught"] != float64(1) {
		t.Fatalf("card=%v", card)
	}
	if badges, ok := card["badges"].([]any); !ok || len(badges) != 0 {
		t.Fatalf("empty badges=%#v", card["badges"])
	}

	testdb.Exec(t, database, `DELETE FROM character_pokemon; DELETE FROM character_pokedex`)
	battleDispatch(t, wh, ses, opcodes.PokedexStatusRequest, `{}`)
	var status map[string]any
	if err := json.Unmarshal(messages.streams[2].payload, &status); err != nil {
		t.Fatal(err)
	}
	if entries, ok := status["status"].([]any); !ok || len(entries) != 0 {
		t.Fatalf("empty status=%#v", status["status"])
	}

	testdb.Exec(t, database, `ALTER TABLE phaser_pokemon RENAME TO unavailable_pokemon`)
	battleDispatch(t, wh, ses, opcodes.PokedexListRequest, `{}`)
	assertPokedexWireFailure(t, messages.streams[3], opcodes.PokedexListResponse, 42)
	// A scan error after a valid species must not publish a shortened success list.
	testdb.Exec(t, database, `ALTER TABLE unavailable_pokemon RENAME TO phaser_pokemon;
  ALTER TABLE phaser_pokemon ALTER COLUMN name DROP NOT NULL;
  UPDATE phaser_pokemon SET name=NULL WHERE id=129`)
	battleDispatch(t, wh, ses, opcodes.PokedexListRequest, `{}`)
	assertPokedexWireFailure(t, messages.streams[4], opcodes.PokedexListResponse, 42)
	testdb.Exec(t, database, `UPDATE phaser_pokemon SET name='MAGIKARP' WHERE id=129;
  ALTER TABLE character_wallet RENAME TO unavailable_wallet`)
	battleDispatch(t, wh, ses, opcodes.TrainerCardRequest, `{}`)
	assertPokedexWireFailure(t, messages.streams[5], opcodes.TrainerCardResponse, 42)
	// Reconciliation failure must not masquerade as an empty status snapshot.
	testdb.Exec(t, database, `DROP TABLE character_pokedex`)
	battleDispatch(t, wh, ses, opcodes.PokedexStatusRequest, `{}`)
	assertPokedexWireFailure(t, messages.streams[6], opcodes.PokedexStatusResponse, 42)
}

func assertQueryWireFailure(t *testing.T, message recordedStreamMessage, opcode opcodes.OpCode) {
	t.Helper()
	var failure map[string]any
	if err := json.Unmarshal(message.payload, &failure); err != nil {
		t.Fatal(err)
	}
	explanation, ok := failure["error"].(string)
	if message.opcode != opcode || failure["success"] != false || !ok || explanation == "" || len(failure) != 2 {
		t.Fatalf("query failure=%v opcode=%d", failure, message.opcode)
	}
}

func assertPokedexWireFailure(t *testing.T, message recordedStreamMessage, opcode opcodes.OpCode, characterID int64) {
	t.Helper()
	var failure map[string]any
	if err := json.Unmarshal(message.payload, &failure); err != nil {
		t.Fatal(err)
	}
	explanation, ok := failure["error"].(string)
	if message.opcode != opcode || failure["success"] != false || !ok || explanation == "" || failure["requestId"] != "" || failure["characterId"] != float64(characterID) || len(failure) != 4 {
		t.Fatalf("correlated query failure=%v opcode=%d", failure, message.opcode)
	}
}
