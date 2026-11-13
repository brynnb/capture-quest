package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func characterAccountReply(t *testing.T, messages *entryRecordingMessenger, opcode opcodes.OpCode, success bool, requestID string) []byte {
	t.Helper()
	replies := messages.snapshot()
	if len(replies) != 1 || replies[0].opcode != opcode {
		t.Fatalf("account command published extra or wrong streams: %+v", replies)
	}
	var header struct {
		Success   bool   `json:"success"`
		RequestID string `json:"requestId"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(replies[0].payload, &header); err != nil || header.Success != success || header.RequestID != requestID {
		t.Fatalf("account response=%s error=%v", replies[0].payload, err)
	}
	if !success {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(replies[0].payload, &fields); err != nil || header.Error == "" || fields["characters"] != nil {
			t.Fatalf("failure published a roster or no error: %s", replies[0].payload)
		}
	}
	return replies[0].payload
}

func TestCharacterListUsesInjectedCurrentRosterAndExplicitFailure(t *testing.T) {
	database, wh, ses, messages := characterEntryFixture(t)
	database.SetMaxOpenConns(1)
	testdb.Exec(t, database, `UPDATE character_data SET last_login=10 WHERE id=42;
	INSERT INTO character_data(id,account_id,name,last_login,map_id) VALUES(43,7,'Other',20,0),(44,8,'Foreign',30,1),(45,7,'Deleted',40,1);
	UPDATE character_data SET deleted_at=CURRENT_TIMESTAMP WHERE id=45`)
	battleDispatch(t, wh, ses, opcodes.CharacterListRequest, `{"requestId":"roster"}`)
	var reply CharacterListResponse
	if err := json.Unmarshal(characterAccountReply(t, messages, opcodes.CharacterListResponse, true, "roster"), &reply); err != nil {
		t.Fatal(err)
	}
	if reply.AccountID != 7 || len(reply.Characters) != 2 || reply.Characters[0].ID != 43 || reply.Characters[0].MapID != 0 || reply.Characters[1].ID != 42 {
		t.Fatalf("roster=%+v", reply)
	}
	// The same injected reader also supplies the existing publication.
	messages.streams = nil
	sendCharInfo(ses, wh, ses.AccountID)
	characterAccountReply(t, messages, opcodes.SendCharInfo, true, "")

	testdb.Exec(t, database, `UPDATE character_data SET deleted_at=CURRENT_TIMESTAMP WHERE account_id=7`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.CharacterListRequest, `{"requestId":"empty"}`)
	var empty CharacterListResponse
	if err := json.Unmarshal(characterAccountReply(t, messages, opcodes.CharacterListResponse, true, "empty"), &empty); err != nil || empty.Characters == nil || len(empty.Characters) != 0 {
		t.Fatalf("empty roster=%+v error=%v", empty, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_data RENAME TO unavailable_characters`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.CharacterListRequest, `{"requestId":"failed"}`)
	characterAccountReply(t, messages, opcodes.CharacterListResponse, false, "failed")
}

func TestCharacterDeleteStableIdentityAndCommittedRoster(t *testing.T) {
	database, wh, ses, messages := characterEntryFixture(t)
	database.SetMaxOpenConns(1)
	testdb.Exec(t, database, `INSERT INTO character_data(id,account_id,name,last_login) VALUES(43,7,'Other',1); UPDATE character_data SET name='Recreated' WHERE id=42`)
	seedCharacterCreationCatalog(t, database)
	battleDispatch(t, wh, ses, opcodes.DeleteCharacter, `{"requestId":"delete","characterId":42}`)
	var reply CharacterDeleteResponse
	if err := json.Unmarshal(characterAccountReply(t, messages, opcodes.CharacterDeleteResponse, true, "delete"), &reply); err != nil {
		t.Fatal(err)
	}
	if reply.AccountID != 7 || reply.CharacterID != 42 || len(reply.Characters) != 1 || reply.Characters[0].ID != 43 {
		t.Fatalf("committed roster=%+v", reply)
	}
	var deletedName, deletedAt string
	if err := database.QueryRow(`SELECT name,CAST(deleted_at AS text) FROM character_data WHERE id=42`).Scan(&deletedName, &deletedAt); err != nil {
		t.Fatal(err)
	}
	if deletedName == "Recreated" || deletedAt == "" || wh.characterOwners.owns(42, ses) {
		t.Fatal("deletion did not commit or retained its temporary owner")
	}

	newID, err := CharacterCreate(context.Background(), database, 7, validCharacterCreateRequest("Recreated"))
	if err != nil || newID == 42 {
		t.Fatalf("recreated name identity=%d error=%v", newID, err)
	}
	for _, requestID := range []string{"delete", "explicit-repeat"} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.DeleteCharacter, fmt.Sprintf(`{"requestId":%q,"characterId":42}`, requestID))
		var repeated CharacterDeleteResponse
		if err := json.Unmarshal(characterAccountReply(t, messages, opcodes.CharacterDeleteResponse, true, requestID), &repeated); err != nil {
			t.Fatal(err)
		}
		if len(repeated.Characters) != 2 {
			t.Fatalf("duplicate omitted current recreated character: %+v", repeated)
		}
		var same bool
		if err := database.QueryRow(`SELECT name=$1 AND CAST(deleted_at AS text)=$2 FROM character_data WHERE id=42`, deletedName, deletedAt).Scan(&same); err != nil || !same {
			t.Fatalf("duplicate rewrote tombstone: %v", err)
		}
		if err := database.QueryRow(`SELECT name='Recreated' AND account_id=7 AND deleted_at IS NULL FROM character_data WHERE id=$1`, newID).Scan(&same); err != nil || !same {
			t.Fatalf("stale ID touched recreated character: %v", err)
		}
	}
}

func TestCharacterDeleteRejectsLegacyForeignAndFailedCommit(t *testing.T) {
	for _, mode := range []string{"legacy", "missing", "foreign", "deferred_commit", "roster_failure"} {
		t.Run(mode, func(t *testing.T) {
			database, wh, ses, messages := characterEntryFixture(t)
			body := `{"requestId":"delete","characterId":42}`
			var foreignOwner *session.Session
			switch mode {
			case "legacy":
				body = `{"requestId":"delete","value":"battle"}`
			case "missing":
				body = `{"requestId":"delete","characterId":999}`
			case "foreign":
				ses.AccountID = 8
				foreignOwner = &session.Session{}
				if err := wh.characterOwners.acquire(context.Background(), 42, foreignOwner, nil); err != nil {
					t.Fatal(err)
				}
				defer wh.characterOwners.release(42, foreignOwner)
			case "deferred_commit":
				testdb.Exec(t, database, `CREATE FUNCTION reject_account_deletion() RETURNS trigger LANGUAGE plpgsql AS $fn$ BEGIN RAISE EXCEPTION 'reject deletion commit'; END $fn$;
				CREATE CONSTRAINT TRIGGER reject_account_deletion AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.deleted_at IS NOT NULL) EXECUTE FUNCTION reject_account_deletion()`)
			case "roster_failure":
				// Deletion itself can execute; the retained roster read must roll it
				// back rather than publish a committed delete with partial state.
				testdb.Exec(t, database, `ALTER TABLE character_data RENAME COLUMN class TO unavailable_class`)
			}
			battleDispatch(t, wh, ses, opcodes.DeleteCharacter, body)
			characterAccountReply(t, messages, opcodes.CharacterDeleteResponse, false, "delete")
			var intact bool
			if err := database.QueryRow(`SELECT name='battle' AND deleted_at IS NULL FROM character_data WHERE id=42`).Scan(&intact); err != nil || !intact {
				t.Fatalf("failed deletion changed character: %v", err)
			}
			if foreignOwner != nil && (foreignOwner.IsClosed() || !wh.characterOwners.owns(42, foreignOwner)) {
				t.Fatal("foreign request disturbed a live character owner")
			}
			if wh.characterOwners.owns(42, ses) {
				t.Fatal("failed deletion retained temporary owner")
			}
		})
	}
}

func waitCharacterDeleteRowWait(t *testing.T, database *sql.DB, holderPID int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting bool
		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1=ANY(pg_blocking_pids(pid)))`, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("deletion never waited on the retained character row")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestCharacterDeleteRetainsRowCancellationAndAccountRevalidation(t *testing.T) {
	for _, mode := range []string{"cancel", "account_changed"} {
		t.Run(mode, func(t *testing.T) {
			database, wh, ses, messages := characterEntryFixture(t)
			database.SetMaxOpenConns(4)
			holder, err := database.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			var holderPID, id int
			if err := holder.QueryRow(`SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			if err := holder.QueryRow(`SELECT id FROM character_data WHERE id=42 FOR UPDATE`).Scan(&id); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() {
				done <- ses.ExecuteCommand(ctx, func() { HandleCharacterDelete(ses, []byte(`{"requestId":"held","characterId":42}`), wh) })
			}()
			finished := false
			defer func() {
				cancel()
				_ = holder.Rollback()
				if !finished {
					select {
					case <-done:
					case <-time.After(3 * time.Second):
						t.Error("deletion did not drain after cleanup")
					}
				}
			}()
			waitCharacterDeleteRowWait(t, database, holderPID)
			if len(messages.snapshot()) != 0 {
				t.Fatal("deletion published before acquiring row")
			}
			if mode == "cancel" {
				cancel()
			} else {
				if _, err := holder.Exec(`UPDATE character_data SET account_id=8 WHERE id=42`); err != nil {
					t.Fatal(err)
				}
				if err := holder.Commit(); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				finished = true
				if mode == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancelled command=%v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("blocked deletion did not settle")
			}
			_ = holder.Rollback()
			characterAccountReply(t, messages, opcodes.CharacterDeleteResponse, false, "held")
			var intact bool
			if err := database.QueryRow(`SELECT name='battle' AND deleted_at IS NULL FROM character_data WHERE id=42`).Scan(&intact); err != nil || !intact {
				t.Fatalf("blocked deletion changed character: %v", err)
			}
			if wh.characterOwners.owns(42, ses) {
				t.Fatal("blocked deletion retained owner")
			}
		})
	}
}

func TestCharacterDeleteDrainsLiveOwnerBeforeCommit(t *testing.T) {
	database, wh, old, _ := characterEntryFixture(t)
	battleDispatch(t, wh, old, opcodes.EnterWorld, `{"name":"battle"}`)
	if !old.HasValidClient() {
		t.Fatal("fixture did not enter world")
	}
	messages := &entryRecordingMessenger{}
	next := &session.Session{SessionID: 18, AccountID: 7, Authenticated: true, Messenger: messages}
	entered, finish, oldDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var release sync.Once
	go func() {
		defer close(oldDone)
		_ = old.ExecuteCommand(context.Background(), func() {
			close(entered)
			<-finish
			if _, err := database.Exec(`UPDATE character_data SET x=21 WHERE id=42`); err != nil {
				t.Error(err)
			}
		})
	}()
	<-entered
	done := make(chan struct{})
	go func() {
		defer close(done)
		battleDispatch(t, wh, next, opcodes.DeleteCharacter, `{"requestId":"drain","characterId":42}`)
	}()
	defer func() {
		release.Do(func() { close(finish) })
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("delete drain did not finish")
		}
		<-oldDone
	}()
	deadline := time.Now().Add(3 * time.Second)
	for !old.IsClosed() {
		if time.Now().After(deadline) {
			t.Fatal("deletion never retired previous transport")
		}
		time.Sleep(time.Millisecond)
	}
	if len(messages.snapshot()) != 0 {
		t.Fatal("deletion acknowledged before previous command drained")
	}
	var deleted bool
	if err := database.QueryRow(`SELECT deleted_at IS NOT NULL FROM character_data WHERE id=42`).Scan(&deleted); err != nil || deleted {
		t.Fatalf("deletion overtook live writer: %v", err)
	}
	release.Do(func() { close(finish) })
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("deletion did not complete after writer drained")
	}
	characterAccountReply(t, messages, opcodes.CharacterDeleteResponse, true, "drain")
	if old.HasValidClient() || wh.characterOwners.owns(42, old) || wh.characterOwners.owns(42, next) {
		t.Fatal("deletion left live or temporary owner")
	}
	var x int
	if err := database.QueryRow(`SELECT CAST(x AS INTEGER),deleted_at IS NOT NULL FROM character_data WHERE id=42`).Scan(&x, &deleted); err != nil || x != 21 || !deleted {
		t.Fatalf("drained position/delete=%d/%t error=%v", x, deleted, err)
	}
	messages.streams = nil
	battleDispatch(t, wh, next, opcodes.EnterWorld, `{"name":"battle"}`)
	assertEntryRejected(t, messages.snapshot())
	if next.HasValidClient() {
		t.Fatal("entry revived deleted identity")
	}
}

func TestCharacterRosterPoolWaitHonorsCancellation(t *testing.T) {
	database, wh, ses, messages := characterEntryFixture(t)
	database.SetMaxOpenConns(1)
	held, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	before := database.Stats().WaitCount
	done := make(chan error, 1)
	go func() {
		done <- ses.ExecuteCommand(ctx, func() { HandleCharacterList(ses, []byte(`{"requestId":"pool"}`), wh) })
	}()
	deadline := time.Now().Add(3 * time.Second)
	for database.Stats().WaitCount == before {
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatal("roster never waited for pool")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled roster=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("roster ignored cancellation")
	}
	characterAccountReply(t, messages, opcodes.CharacterListResponse, false, "pool")
}

func TestCharacterListDoesNotHideExistingCharactersBeyondCreationLimit(t *testing.T) {
	database, wh, ses, messages := characterEntryFixture(t)
	testdb.Exec(t, database, `UPDATE character_data SET last_login=0 WHERE id=42;
	INSERT INTO character_data(id,account_id,name,last_login) SELECT id,7,'Existing'||id,10 FROM generate_series(100,109) AS id`)
	battleDispatch(t, wh, ses, opcodes.CharacterListRequest, `{"requestId":"all-existing"}`)
	var reply CharacterListResponse
	if err := json.Unmarshal(characterAccountReply(t, messages, opcodes.CharacterListResponse, true, "all-existing"), &reply); err != nil {
		t.Fatal(err)
	}
	if len(reply.Characters) != 11 || reply.Characters[10].ID != 42 {
		t.Fatalf("current roster hid existing characters: %+v", reply)
	}
}
