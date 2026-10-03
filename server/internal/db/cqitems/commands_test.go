package cqitems

import (
	"context"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

// Both consumers use this boundary; failed effects, projections and commits
// must roll back the revision as well as gameplay writes and publish nothing.
func TestCommandExecutorRollsBackEveryFailureStage(t *testing.T) {
	for _, stage := range []string{"domain", "projection", "commit", "cancellation"} {
		t.Run(stage, func(t *testing.T) {
			database, store := inventoryDatabase(t)
			if stage == "commit" {
				testdb.Exec(t, database, `CREATE FUNCTION reject_command_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject commit'; END $$;
    CREATE CONSTRAINT TRIGGER reject_command_commit AFTER INSERT ON character_shop_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_command_commit()`)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			snapshot, err := store.ExecuteCommand(ctx, 1, 0, func(tx db.DBTX) error {
				if _, err := NewStore(tx).AddItemToInventory(1, 1, 1); err != nil {
					return err
				}
				switch stage {
				case "domain":
					return fmt.Errorf("effect failed")
				case "projection":
					_, err := tx.Exec(`ALTER TABLE character_wallet RENAME TO unavailable_wallet`)
					return err
				case "cancellation":
					cancel()
				}
				return nil
			})
			if err == nil || snapshot.Items != nil || snapshot.CommandRevision != 0 {
				t.Fatalf("published failed result=%+v %v", snapshot, err)
			}
			// Wait for database/sql's asynchronous cancellation rollback to finish.
			restored, err := store.GetCharacterSnapshot(context.Background(), 1)
			if err != nil || len(restored.Items) != 0 || restored.CommandRevision != 0 {
				t.Fatalf("rollback=%+v %v", restored, err)
			}
		})
	}
}

func TestCommandExecutorCancelsWhileWaitingForCharacter(t *testing.T) {
	database, store := inventoryDatabase(t)
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := db.LockCharacter(tx, 1); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	called := false
	snapshot, err := store.ExecuteCommand(ctx, 1, 0, func(db.DBTX) error { called = true; return nil })
	if err == nil || called || snapshot.Items != nil || time.Since(started) > time.Second {
		t.Fatalf("cancellation: called=%v snapshot=%+v error=%v", called, snapshot, err)
	}
}
