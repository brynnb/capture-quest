package db_test

import (
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFieldCommandReceiptReplayRevisionAndLateCommitRollback(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'field')`)
	revision := int64(0)
	calls := 0
	apply := func(tx db.DBTX) ([]byte, error) {
		calls++
		_, err := tx.Exec(`UPDATE character_data SET time_played=time_played+1 WHERE id=42`)
		return []byte(`{"hooked":false}`), err
	}
	first, replay, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "first", &revision, []byte("input"), apply)
	if err != nil || replay || !json.Valid(first) {
		t.Fatalf("first=%s replay=%v err=%v", first, replay, err)
	}
	again, replay, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "first", &revision, []byte("input"), apply)
	if err != nil || !replay || string(again) != string(first) || calls != 1 {
		t.Fatalf("replay=%s %v %v calls=%d", again, replay, err, calls)
	}
	if _, _, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "first", &revision, []byte("different"), apply); err == nil {
		t.Fatal("changed input replay accepted")
	}
	revision = 1
	if _, _, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "second", &revision, []byte("input"), apply); err != nil {
		t.Fatal(err)
	}
	revision = 0
	if _, _, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "first", &revision, []byte("input"), apply); err == nil {
		t.Fatal("replaced receipt replay reran")
	}
	testdb.Exec(t, database, `CREATE FUNCTION reject_field_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late receipt failure'; END $$; CREATE CONSTRAINT TRIGGER reject_field_receipt AFTER UPDATE ON character_field_command_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_field_receipt()`)
	revision = 2
	result, _, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "third", &revision, []byte("input"), apply)
	if err == nil || result != nil {
		t.Fatalf("uncommitted result=%s err=%v", result, err)
	}
	var played, stored, rows int
	if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id=42`).Scan(&played); err != nil || played != 2 {
		t.Fatalf("rolled back gameplay=%d err=%v", played, err)
	}
	if err := database.QueryRow(`SELECT revision FROM character_field_command_state WHERE character_id=42 AND domain='fishing'`).Scan(&stored); err != nil || stored != 2 {
		t.Fatalf("revision=%d err=%v", stored, err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM character_field_command_state WHERE character_id=42`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("receipt growth=%d err=%v", rows, err)
	}
}

func TestFieldCommandIdentityValidationAndStoredReceiptCorruption(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'identity')`)
	called := false
	apply := func(DBTX db.DBTX) ([]byte, error) { called = true; return []byte(`{"ok":true}`), nil }
	if _, _, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "tagged", nil, nil, apply); err == nil || called {
		t.Fatal("tagged legacy bypass invoked gameplay")
	}
	if _, _, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "", nil, nil, apply); err == nil || called {
		t.Fatal("untagged legacy bypass invoked gameplay")
	}

	revision := int64(0)
	_, _, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "first", &revision, []byte("input"), func(tx db.DBTX) ([]byte, error) { revision = 99; return []byte(`{"ok":true}`), nil })
	if err != nil {
		t.Fatalf("callback changed captured identity: %v", err)
	}
	revision = 0
	testdb.Exec(t, database, `UPDATE character_field_command_state SET result_json='broken' WHERE character_id=42`)
	result, replay, err := db.ExecuteFieldCommand(context.Background(), database, 42, "fishing", "first", &revision, []byte("input"), apply)
	if err == nil || replay || result != nil || called {
		t.Fatal("corrupt replay advertised or executed gameplay")
	}
}

func TestConcurrentFieldDuplicateWaitsAndReplaysOneCommit(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'concurrent')`)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var ownerPID int
	var calls atomic.Int32
	apply := func(tx db.DBTX) ([]byte, error) {
		calls.Add(1)
		if err := tx.QueryRow(`SELECT pg_backend_pid()`).Scan(&ownerPID); err != nil {
			return nil, err
		}
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		_, err := tx.Exec(`UPDATE character_data SET time_played=time_played+1 WHERE id=42`)
		return []byte(`{"ok":true}`), err
	}
	type outcome struct {
		replay bool
		err    error
	}
	results := make(chan outcome, 2)
	execute := func() {
		revision := int64(0)
		_, replay, err := db.ExecuteFieldCommand(ctx, database, 42, "fishing", "same", &revision, []byte("input"), apply)
		results <- outcome{replay, err}
	}
	go execute()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("first command did not reach apply")
	}
	go execute()
	for {
		var waiting bool
		if err := database.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, ownerPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if ctx.Err() != nil {
			t.Fatal("duplicate did not wait on owner")
		}
		runtime.Gosched()
	}
	unblock()
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || first.replay == second.replay || calls.Load() != 1 {
		t.Fatalf("first=%+v second=%+v calls=%d", first, second, calls.Load())
	}
	var played int
	if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id=42`).Scan(&played); err != nil || played != 1 {
		t.Fatalf("duplicate effect=%d %v", played, err)
	}
}
