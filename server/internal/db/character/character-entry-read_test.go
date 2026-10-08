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

func TestOptionsEntryReadDefaultsOnlyUnsetDataAndHonorsPoolDeadline(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name,options) VALUES(9,'options-reader',NULL)`)
	opts, err := LoadOptionsFrom(context.Background(), database, 9)
	if err != nil || opts.RivalName != DefaultRivalName {
		t.Fatalf("unset defaults: %+v %v", opts, err)
	}
	for _, value := range []string{`[]`, `null`, `{"showNetworkStats":"bad"}`} {
		testdb.Exec(t, database, `UPDATE character_data SET options=$1 WHERE id=9`, value)
		if _, err := LoadOptionsFrom(context.Background(), database, 9); err == nil {
			t.Fatalf("malformed options defaulted: %s", value)
		}
	}
	if _, err := LoadOptionsFrom(context.Background(), database, 99); err == nil {
		t.Fatal("missing character defaulted")
	}
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := LoadOptionsFrom(ctx, database, 9); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("options escaped cancellation: %v", err)
	}
}
