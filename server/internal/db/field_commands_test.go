package db_test

import (
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"testing"
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
