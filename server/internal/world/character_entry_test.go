package world

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	model "capturequest/internal/db/models"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

type entryRecordingMessenger struct {
	mu         sync.Mutex
	streams    []recordedStreamMessage
	failOpcode opcodes.OpCode
	afterSend  func(opcodes.OpCode)
}

func (m *entryRecordingMessenger) SendDatagram(int, []byte) error { return nil }
func (m *entryRecordingMessenger) SendStream(sessionID int, data []byte) error {
	opcode := opcodes.OpCode(binary.LittleEndian.Uint16(data[4:6]))
	if m.failOpcode != 0 && m.failOpcode == opcode {
		return errors.New("entry transport failure")
	}
	m.mu.Lock()
	m.streams = append(m.streams, recordedStreamMessage{sessionID: sessionID, opcode: opcode, payload: append([]byte(nil), data[6:]...)})
	m.mu.Unlock()
	if m.afterSend != nil {
		m.afterSend(opcode)
	}
	return nil
}
func (m *entryRecordingMessenger) snapshot() []recordedStreamMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]recordedStreamMessage(nil), m.streams...)
}

func characterEntryFixture(t *testing.T) (*sql.DB, *WorldHandler, *session.Session, *entryRecordingMessenger) {
	t.Helper()
	database, wh, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `UPDATE character_data SET account_id=7,map_id=50,x=7,y=8 WHERE id=42;
        UPDATE character_wallet SET pokedollars=345 WHERE character_id=42;
        INSERT INTO character_bind(id,slot,map_id,x,y) VALUES(42,0,50,7,8);
        INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'ENTRY_READY');
        INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ENTRY_ROOM',20,20,0);
        INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,7,8,1,1);`)
	wh.sessionManager = session.NewSessionManager()
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	wh.WildEncounter = NewWildEncounterManager(wh, database)
	wh.Safari = NewSafariZoneManager(database)
	messages := &entryRecordingMessenger{}
	ses := &session.Session{SessionID: 17, AccountID: 7, Authenticated: true, Messenger: messages}
	// Every entry read, including the formerly global wallet/bind streams, must
	// use the selected world's injected database.
	db.GlobalWorldDB = nil
	t.Cleanup(func() { drainCharacterEntry(t, wh, ses) })
	return database, wh, ses, messages
}

func drainCharacterEntry(t *testing.T, wh *WorldHandler, ses *session.Session) {
	t.Helper()
	ses.Close()
	var err error
	ses.DrainCommands(func() { err = wh.cleanupCharacterSession(context.Background(), ses) })
	if err != nil {
		t.Errorf("entry cleanup: %v", err)
	}
}

func entryAcknowledgements(t *testing.T, messages []recordedStreamMessage) []int32 {
	t.Helper()
	var values []int32
	for _, message := range messages {
		if message.opcode != opcodes.PostEnterWorld {
			continue
		}
		var response SimpleSuccessResponse
		if err := json.Unmarshal(message.payload, &response); err != nil {
			t.Fatal(err)
		}
		values = append(values, response.Value)
	}
	return values
}

func assertEntryRejected(t *testing.T, messages []recordedStreamMessage) {
	t.Helper()
	values := entryAcknowledgements(t, messages)
	if len(values) != 1 || values[0] != 0 {
		t.Fatalf("entry acknowledgements=%v", values)
	}
	for _, message := range messages {
		switch message.opcode {
		case opcodes.CharacterData, opcodes.CharacterWallet, opcodes.CharacterBind, opcodes.ResourcesChangedNotify, opcodes.PokeBattleStartResponse:
			t.Fatalf("failed entry published opcode %d: %s", message.opcode, message.payload)
		}
	}
}

func TestEnterWorldAcknowledgesOnlyRestoredInitialState(t *testing.T) {
	for _, withBattle := range []bool{false, true} {
		name := "ordinary"
		if withBattle {
			name = "restored_battle"
		}
		t.Run(name, func(t *testing.T) {
			database, wh, ses, messages := characterEntryFixture(t)
			if withBattle {
				battle := battleTestStart(t, database, false, nil)
				forgetBattle(42, battle)
			}
			messages.afterSend = func(opcode opcodes.OpCode) {
				if opcode != opcodes.PostEnterWorld {
					return
				}
				if !wh.EventFlags.CheckFlag(42, "ENTRY_READY") {
					t.Error("ack preceded event flag restoration")
				}
				if withBattle && getBattle(42) == nil {
					t.Error("ack preceded durable battle restoration")
				}
			}
			battleDispatch(t, wh, ses, opcodes.EnterWorld, `{"name":"battle"}`)
			if ses.IsClosed() || !ses.HasValidClient() || !wh.characterOwners.owns(42, ses) {
				t.Fatal("successful entry lost its owner")
			}
			got := messages.snapshot()
			if len(got) < 4 {
				t.Fatalf("entry streams=%+v", got)
			}
			want := []opcodes.OpCode{opcodes.CharacterData, opcodes.CharacterWallet, opcodes.CharacterBind, opcodes.PostEnterWorld}
			for i, opcode := range want {
				if got[i].opcode != opcode {
					t.Fatalf("stream %d=%d want %d", i, got[i].opcode, opcode)
				}
			}
			values := entryAcknowledgements(t, got)
			if len(values) != 1 || values[0] != 1 {
				t.Fatalf("acknowledgements=%v", values)
			}
			var char protocol.CharacterData
			var wallet model.CharacterWallet
			var bind model.CharacterBind
			if json.Unmarshal(got[0].payload, &char) != nil || char.CharacterData.ID != 42 || char.CharacterData.X != 7 {
				t.Fatalf("initial character=%s", got[0].payload)
			}
			if json.Unmarshal(got[1].payload, &wallet) != nil || wallet.Pokedollars != 345 {
				t.Fatalf("initial wallet=%s", got[1].payload)
			}
			if json.Unmarshal(got[2].payload, &bind) != nil || bind.ID != 42 || bind.MapID != 50 {
				t.Fatalf("initial bind=%s", got[2].payload)
			}
			if withBattle {
				found := false
				for _, message := range got {
					if message.opcode == opcodes.PokeBattleStartResponse {
						var response map[string]interface{}
						if json.Unmarshal(message.payload, &response) != nil || response["restored"] != true {
							t.Fatalf("restored battle=%s", message.payload)
						}
						found = true
					}
				}
				if !found {
					t.Fatal("restored battle publication missing")
				}
			}
			// A second entry packet cannot replace or close the working owner.
			messages.afterSend = nil
			oldClient := ses.Client
			HandleEnterWorld(ses, []byte(`{"name":"another"}`), wh)
			if ses.IsClosed() || ses.Client != oldClient || !wh.characterOwners.owns(42, ses) {
				t.Fatal("duplicate entry changed active owner")
			}
			values = entryAcknowledgements(t, messages.snapshot())
			if len(values) != 2 || values[1] != 0 {
				t.Fatalf("duplicate acknowledgements=%v", values)
			}
		})
	}
}

func TestEnterWorldMissingOptionalInitialRowsAreNotStorageFailure(t *testing.T) {
	database, wh, ses, messages := characterEntryFixture(t)
	testdb.Exec(t, database, `DELETE FROM character_wallet WHERE character_id=42; DELETE FROM character_bind WHERE id=42`)
	battleDispatch(t, wh, ses, opcodes.EnterWorld, `{"name":"battle"}`)
	got := messages.snapshot()
	values := entryAcknowledgements(t, got)
	if ses.IsClosed() || len(values) != 1 || values[0] != 1 {
		t.Fatalf("optional rows blocked entry: %v", values)
	}
	var wallet model.CharacterWallet
	if json.Unmarshal(got[1].payload, &wallet) != nil || wallet.CharacterID != 42 || wallet.Pokedollars != 0 {
		t.Fatalf("empty wallet=%s", got[1].payload)
	}
	if string(got[2].payload) != "null" {
		t.Fatalf("absent bind=%s", got[2].payload)
	}
}

func TestEnterWorldRejectsMalformedOrDeniedRequestBeforeAdmission(t *testing.T) {
	for _, payload := range []string{`broken`, `{}`, `null`, `{"name":"  "}`} {
		messages := &entryRecordingMessenger{}
		ses := &session.Session{Authenticated: true, Messenger: messages}
		HandleEnterWorld(ses, []byte(payload), &WorldHandler{})
		assertEntryRejected(t, messages.snapshot())
		if ses.IsClosed() || ses.HasValidClient() {
			t.Fatalf("malformed request admitted or closed session: %s", payload)
		}
	}
	for _, failure := range []string{"wrong_account", "missing_character", "lookup_failure"} {
		t.Run(failure, func(t *testing.T) {
			database, wh, ses, messages := characterEntryFixture(t)
			payload := `{"name":"battle"}`
			switch failure {
			case "wrong_account":
				ses.AccountID = 8
			case "missing_character":
				payload = `{"name":"missing"}`
			case "lookup_failure":
				testdb.Exec(t, database, `ALTER TABLE character_data RENAME TO unavailable_character_data`)
			}
			HandleEnterWorld(ses, []byte(payload), wh)
			assertEntryRejected(t, messages.snapshot())
			if ses.IsClosed() || ses.HasValidClient() || wh.characterOwners.owns(42, ses) {
				t.Fatal("denied request changed owner")
			}
		})
	}
}

func TestEnterWorldRestoreFailureNeverAcknowledgesAndDrainsPartialOwner(t *testing.T) {
	for _, failure := range []struct {
		name, sql string
		installed bool
	}{
		{"options", `UPDATE character_data SET options='[]' WHERE id=42`, false},
		{"movement", `INSERT INTO character_movement_routes(character_id,map_id,x,y,path_json) VALUES(42,50,7,8,'broken')`, true},
		{"safari", `ALTER TABLE character_safari_state RENAME TO unavailable_safari_state`, true},
		{"flags", `ALTER TABLE character_event_flags RENAME TO unavailable_event_flags`, true},
		{"battle", `ALTER TABLE character_battle_state RENAME TO unavailable_battle_state`, true},
		{"wallet", `ALTER TABLE character_wallet RENAME TO unavailable_wallet`, true},
		{"bind", `ALTER TABLE character_bind RENAME TO unavailable_bind`, true},
	} {
		t.Run(failure.name, func(t *testing.T) {
			database, wh, ses, messages := characterEntryFixture(t)
			testdb.Exec(t, database, failure.sql)
			battleDispatch(t, wh, ses, opcodes.EnterWorld, `{"name":"battle"}`)
			assertEntryRejected(t, messages.snapshot())
			if !ses.IsClosed() {
				t.Fatal("failed partial entry left transport open")
			}
			if ses.HasValidClient() != failure.installed || wh.characterOwners.owns(42, ses) != failure.installed {
				t.Fatal("partial owner released before its drain or empty owner retained")
			}
			if ses.Presence().CharacterID != 0 {
				t.Fatal("failed entry published gameplay presence")
			}
			drainCharacterEntry(t, wh, ses)
			if ses.HasValidClient() || wh.characterOwners.owns(42, ses) {
				t.Fatal("drain retained failed entry owner")
			}
			if _, _, _, ok := wh.PlayerMovement.GetPosition(42); ok {
				t.Fatal("drain retained failed movement registration")
			}
			if getBattle(42) != nil || wh.EventFlags.CheckFlag(42, "ENTRY_READY") {
				t.Fatal("drain retained failed entry caches")
			}
		})
	}
}

func TestEnterWorldBlockedRestoreCancellationPublishesNoEarlySuccess(t *testing.T) {
	database, wh, ses, messages := characterEntryFixture(t)
	database.SetMaxOpenConns(4)
	holder, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	if _, err := holder.Exec(`LOCK TABLE character_event_flags IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	var holderPID int
	if err := holder.QueryRow(`SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- ses.ExecuteCommand(ctx, func() { HandleEnterWorld(ses, []byte(`{"name":"battle"}`), wh) })
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting bool
		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1=ANY(pg_blocking_pids(pid)))`, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("entry never reached the held restore query")
		}
		time.Sleep(time.Millisecond)
	}
	if got := messages.snapshot(); len(got) != 0 {
		t.Fatalf("entry published before held restore completed: %+v", got)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled entry=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled entry did not drain")
	}
	assertEntryRejected(t, messages.snapshot())
	if !ses.IsClosed() || !wh.characterOwners.owns(42, ses) {
		t.Fatal("cancelled partial owner not closed and retained for drain")
	}
	if err := holder.Rollback(); err != nil {
		t.Fatal(err)
	}
	drainCharacterEntry(t, wh, ses)
	if wh.characterOwners.owns(42, ses) {
		t.Fatal("cancelled owner survived cleanup")
	}
}

func TestEnterWorldInitialPublicationFailureOrRetirementCannotSucceed(t *testing.T) {
	for _, failure := range []string{"wallet_send", "ack_send", "retired_during_character_send"} {
		t.Run(failure, func(t *testing.T) {
			_, wh, ses, messages := characterEntryFixture(t)
			switch failure {
			case "wallet_send":
				messages.failOpcode = opcodes.CharacterWallet
			case "ack_send":
				messages.failOpcode = opcodes.PostEnterWorld
			case "retired_during_character_send":
				messages.afterSend = func(opcode opcodes.OpCode) {
					if opcode == opcodes.CharacterData {
						ses.Close()
					}
				}
			}
			battleDispatch(t, wh, ses, opcodes.EnterWorld, `{"name":"battle"}`)
			for _, value := range entryAcknowledgements(t, messages.snapshot()) {
				if value == 1 {
					t.Fatal("failed/retired publication acknowledged entry")
				}
			}
			if !ses.IsClosed() {
				t.Fatal("failed/retired entry transport left open")
			}
			drainCharacterEntry(t, wh, ses)
			if wh.characterOwners.owns(42, ses) {
				t.Fatal("failed/retired entry retained character owner")
			}
		})
	}
}
