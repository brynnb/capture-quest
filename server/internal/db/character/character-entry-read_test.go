package db_character

import (
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"errors"
	"testing"
	"time"
)

func TestCharacterEntryReadUsesInjectedDatabaseAndPoolCancellation(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(9,'owned-reader')`)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	character, err := GetCharacterByNameContext(context.Background(), database, "owned-reader")
	if err != nil || character.ID != 9 {
		t.Fatalf("injected character read: %+v %v", character, err)
	}
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := GetCharacterByNameContext(ctx, database, "owned-reader"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pool wait ignored entry cancellation: %v", err)
	}
	if database.Stats().WaitCount <= before.WaitCount {
		t.Fatal("regression did not exercise actual pool contention")
	}
}
