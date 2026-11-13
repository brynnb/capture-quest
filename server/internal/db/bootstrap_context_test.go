package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestCancelledDatabaseOpenPreservesExistingHandle(t *testing.T) {
	previous := GlobalWorldDB
	t.Cleanup(func() { GlobalWorldDB = previous })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := InitWorldDB(ctx, "pgx", "postgres://unused:unused@127.0.0.1:1/unused?sslmode=disable"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled initialization: %v", err)
	}
	if GlobalWorldDB != previous {
		t.Fatal("failed initialization published a handle")
	}
}

func TestWorldTileSchemaDeadlineAndRetry(t *testing.T) {
	database := testdb.Postgres(t)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`LOCK TABLE phaser_tiles IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := EnsureWorldTileMutationSchema(ctx, database); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("locked schema upgrade: %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("schema upgrade did not observe deadline")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := EnsureWorldTileMutationSchema(context.Background(), database); err != nil {
		t.Fatalf("retry: %v", err)
	}
}
