package cqitems

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestCommandExecutorRejectsParentTransactions(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		t.Run(fmt.Sprintf("wrapped=%v", wrapped), func(t *testing.T) {
			database, store := inventoryDatabase(t)
			tx, err := database.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			check := func(parent db.DBTX) error {
				called := false
				snapshot, err := NewStore(parent).ExecuteCommand(context.Background(), 1, 0, func(inner db.DBTX) error {
					called = true
					_, err := NewStore(inner).AddItemToInventory(1, 1, 1)
					return err
				})
				if err == nil || called || !reflect.DeepEqual(snapshot, CQInventorySnapshot{}) {
					t.Fatalf("parent accepted: called=%v snapshot=%+v error=%v", called, snapshot, err)
				}
				// Rejection must leave the caller's transaction usable and untouched.
				var rows int
				if err := parent.QueryRow(`SELECT count(*) FROM character_shop_state`).Scan(&rows); err != nil || rows != 0 {
					t.Fatalf("parent revision rows=%d error=%v", rows, err)
				}
				return nil
			}
			if wrapped {
				err = db.Transaction(context.Background(), tx, check)
			} else {
				err = check(tx)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			snapshot, err := store.GetCharacterSnapshot(context.Background(), 1)
			if err != nil || len(snapshot.Items) != 0 || snapshot.CommandRevision != 0 {
				t.Fatalf("rejected command changed durable state: %+v %v", snapshot, err)
			}
		})
	}
}

func TestCommandExecutorSuccessIsCommittedOnReturn(t *testing.T) {
	database, store := inventoryDatabase(t)
	ctx := context.Background()
	// Reserve an independent connection before the command starts.
	reader, err := database.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	snapshot, err := store.ExecuteCommand(ctx, 1, 0, func(tx db.DBTX) error {
		_, err := NewStore(tx).AddItemToInventory(1, 1, 2)
		return err
	})
	if err != nil || snapshot.CommandRevision != 1 || len(snapshot.Items) != 1 {
		t.Fatalf("command=%+v %v", snapshot, err)
	}
	var revision int64
	var quantity int
	err = reader.QueryRowContext(ctx, `SELECT revision, quantity FROM character_shop_state
		JOIN cq_item_instances ON owner_id=character_id WHERE character_id=1 AND owner_type=0`).Scan(&revision, &quantity)
	if err != nil || revision != snapshot.CommandRevision || quantity != 2 {
		t.Fatalf("uncommitted success: revision=%d quantity=%d error=%v", revision, quantity, err)
	}
}

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
