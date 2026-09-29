package scriptedevents

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestSyncCancellationAndLockedQueryRetry(t *testing.T) {
	database := testdb.Postgres(t)
	root := t.TempDir()
	scripts := filepath.Join(root, scriptsDirName)
	if err := os.Mkdir(scripts, 0700); err != nil {
		t.Fatal(err)
	}
	event := `{"scriptLabel":"ContextTest","mapName":"TEST","trigger":{"type":"npc_click","label":"TEST_CONTEXT"},"actions":[{"type":"dialogue","text":"hello"}]}`
	if err := os.WriteFile(filepath.Join(scripts, "test.json"), []byte(event), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Sync(ctx, database, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled sync: %v", err)
	}
	if err := SyncDefault(ctx, database); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled default sync: %v", err)
	}
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`LOCK TABLE phaser_cutscene_scripts IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	deadlineCtx, cancelDeadline := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelDeadline()
	started := time.Now()
	if _, err := Sync(deadlineCtx, database, root); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("locked sync: %v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("sync did not observe deadline")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(context.Background(), database, root); err != nil {
		t.Fatalf("retry: %v", err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM phaser_cutscene_scripts WHERE script_label = 'ContextTest'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("published scripts = %d", count)
	}
}
