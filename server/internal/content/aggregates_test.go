package content

import (
	"context"
	"errors"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestAggregateReadSnapshotAndFailureBoundaries(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO phaser_map_scripts(map_name,script_index,script_label,script_constant,raw_asm) VALUES('TEST',0,'OLD_SCRIPT','CONST','old');
  INSERT INTO phaser_event_flags(map_name,flag_name,operation) VALUES('TEST','OLD_FLAG','set')`)
	_, err := readSnapshot(context.Background(), database, func(ctx context.Context, snapshot db.ContextDBTX) (int, error) {
		var name string
		if err := snapshot.QueryRowContext(ctx, `SELECT script_label FROM phaser_map_scripts WHERE map_name='TEST'`).Scan(&name); err != nil {
			return 0, err
		}
		if name != "OLD_SCRIPT" {
			t.Fatalf("initial script=%s", name)
		}
		// A separate publisher commits after the first read established the snapshot.
		testdb.Exec(t, database, `UPDATE phaser_map_scripts SET script_label='NEW_SCRIPT'; UPDATE phaser_event_flags SET flag_name='NEW_FLAG'`)
		if err := snapshot.QueryRowContext(ctx, `SELECT flag_name FROM phaser_event_flags WHERE map_name='TEST'`).Scan(&name); err != nil {
			return 0, err
		}
		if name != "OLD_FLAG" {
			t.Fatalf("mixed publication flag=%s", name)
		}
		return 1, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Even a completed projection must be discarded if commit loses its context.
	commitCtx, cancelCommit := context.WithCancel(context.Background())
	defer cancelCommit()
	loaded := false
	value, commitErr := readSnapshot(commitCtx, database, func(ctx context.Context, snapshot db.ContextDBTX) (int, error) {
		var count int
		if err := snapshot.QueryRowContext(ctx, `SELECT COUNT(*) FROM phaser_map_scripts`).Scan(&count); err != nil {
			return 0, err
		}
		loaded = true
		cancelCommit()
		return count, nil
	})
	if !loaded || commitErr == nil || value != 0 {
		t.Fatalf("cancelled commit leaked projection=%d loaded=%t error=%v", value, loaded, commitErr)
	}
	service := New(database)
	result, err := service.MapScripts(context.Background(), "TEST")
	if err != nil || !result.Success || result.Scripts[0].ScriptLabel != "NEW_SCRIPT" || result.EventFlags[0].FlagName != "NEW_FLAG" {
		t.Fatalf("next snapshot=%+v error=%v", result, err)
	}
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`LOCK TABLE phaser_npc_movement_data IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	result, err = service.MapScripts(ctx, "TEST")
	if !errors.Is(err, context.DeadlineExceeded) || result.Success || result.Scripts != nil {
		t.Fatalf("late cancellation result=%+v error=%v", result, err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("aggregate ignored cancellation")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MapScripts(context.Background(), "TEST"); err != nil {
		t.Fatalf("retry=%v", err)
	}
	testdb.Exec(t, database, `ALTER TABLE phaser_npc_movement_data RENAME TO missing_npc_movements`)
	result, err = service.MapScripts(context.Background(), "TEST")
	if err == nil || result.Success || result.Scripts != nil {
		t.Fatalf("late failure escaped result=%+v error=%v", result, err)
	}
}

func TestAggregateBudgetInterruptsLateBackgroundQuery(t *testing.T) {
	database := testdb.Postgres(t)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`LOCK TABLE phaser_npc_movement_data IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	result, err := New(database).MapScripts(context.Background(), "TEST")
	if !errors.Is(err, context.DeadlineExceeded) || result.Success || result.Scripts != nil {
		t.Fatalf("bounded aggregate=%+v error=%v", result, err)
	}
	if elapsed := time.Since(started); elapsed > 7*time.Second {
		t.Fatalf("late query ignored operation budget: %v", elapsed)
	}
}
