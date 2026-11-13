package cqitems

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestCommandReceiptHeldConcurrentDuplicateExecutesOnce(t *testing.T) {
	database, store := inventoryDatabase(t)
	// Four commands, the real lock holder and its independent lock observer.
	database.SetMaxOpenConns(6)
	holder, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	if err := db.LockCharacter(holder, 1); err != nil {
		t.Fatal(err)
	}
	var holderPID int
	if err := holder.QueryRow(`SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	var rolls atomic.Int32
	apply := func(tx db.DBTX) ([]byte, error) {
		roll := rolls.Add(1)
		if _, err := NewStore(tx).AddItemToInventory(1, 1, 1); err != nil {
			return nil, err
		}
		return []byte(fmt.Sprintf(`{"kind":"slot","version":1,"roll":%d}`, roll)), nil
	}
	type outcome struct {
		data   []byte
		replay bool
		err    error
	}
	results := make(chan outcome, 4)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			data, replay, err := store.ExecuteCommandWithReceipt(context.Background(), 1, 0, "slot", "spin-once", []byte(`{"bet":3,"machineX":6}`), apply)
			results <- outcome{data, replay, err}
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var waiting int
		if err := database.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND $1=ANY(pg_blocking_pids(pid))`, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("receipt command did not wait on retained character lock")
		}
		time.Sleep(time.Millisecond)
	}
	if err := holder.Commit(); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	close(results)
	first := 0
	for result := range results {
		if result.err != nil || string(result.data) != `{"kind":"slot","version":1,"roll":1}` {
			t.Fatalf("duplicate changed outcome %+v", result)
		}
		if !result.replay {
			first++
		}
	}
	if first != 1 || rolls.Load() != 1 {
		t.Fatalf("new commits=%d random/effect invocations=%d", first, rolls.Load())
	}
	snapshot, err := store.GetCharacterSnapshot(context.Background(), 1)
	if err != nil || snapshot.CommandRevision != 1 || len(snapshot.Items) != 1 || snapshot.Items[0].Instance.Quantity != 1 {
		t.Fatalf("duplicated award snapshot=%+v %v", snapshot, err)
	}
	var rows int
	if err := database.QueryRow(`SELECT count(*) FROM character_shop_state WHERE character_id=1`).Scan(&rows); err != nil || rows != 1 {
		t.Fatalf("receipt rows=%d %v", rows, err)
	}
}

func TestCommandReceiptRejectsChangedPayloadFamilyAndCorruption(t *testing.T) {
	database, store := inventoryDatabase(t)
	calls := 0
	input := []byte(`{"bet":3,"machineX":6}`)
	apply := func(tx db.DBTX) ([]byte, error) {
		calls++
		if _, err := NewStore(tx).AddItemToInventory(1, 1, 1); err != nil {
			return nil, err
		}
		return []byte(`{"kind":"slot","version":1,"roll":7}`), nil
	}
	original, replay, err := store.ExecuteCommandWithReceipt(context.Background(), 1, 0, "slot", "same-id", input, apply)
	if err != nil || replay {
		t.Fatalf("initial receipt replay=%t %v", replay, err)
	}
	for _, test := range []struct {
		name, kind string
		revision   int64
		input      []byte
	}{
		{"payload", "slot", 0, []byte(`{"bet":2,"machineX":6}`)},
		{"family", "prize", 0, input},
		{"reuse-current-revision", "slot", 1, input},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, replayed, err := store.ExecuteCommandWithReceipt(context.Background(), 1, test.revision, test.kind, "same-id", test.input, apply)
			if err == nil || replayed || data != nil || calls != 1 {
				t.Fatalf("mismatched identity accepted data=%s replay=%t calls=%d %v", data, replayed, calls, err)
			}
		})
	}
	var encoded, hash string
	if err := database.QueryRow(`SELECT result_json,input_hash FROM character_shop_state WHERE character_id=1`).Scan(&encoded, &hash); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ name, field, value string }{
		{"version", "version", `2`},
		{"kind", "kind", `"prize"`},
		{"character", "characterId", `2`},
		{"revision", "revision", `2`},
		{"request", "requestId", `"different"`},
		{"fingerprint", "inputHash", `"broken"`},
		{"outcome-fingerprint", "outcomeHash", `"broken"`},
		{"changed-valid-outcome", "result", `{"kind":"slot","version":1,"roll":999}`},
		{"null-outcome", "result", `null`},
		{"unknown-envelope-field", "unexpected", `true`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var receipt map[string]json.RawMessage
			if err := json.Unmarshal([]byte(encoded), &receipt); err != nil {
				t.Fatal(err)
			}
			receipt[test.field] = json.RawMessage(test.value)
			corrupt, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			testdb.Exec(t, database, `UPDATE character_shop_state SET result_json=$1 WHERE character_id=1`, string(corrupt))
			data, replayed, err := store.ExecuteCommandWithReceipt(context.Background(), 1, 0, "slot", "same-id", input, apply)
			if err == nil || replayed || data != nil || calls != 1 {
				t.Fatalf("corrupt receipt accepted data=%s replay=%t calls=%d %v", data, replayed, calls, err)
			}
			testdb.Exec(t, database, `UPDATE character_shop_state SET result_json=$1 WHERE character_id=1`, encoded)
		})
	}
	for _, column := range []string{"input_hash", "result_json"} {
		t.Run("row-"+column, func(t *testing.T) {
			testdb.Exec(t, database, `UPDATE character_shop_state SET `+column+`='broken' WHERE character_id=1`)
			data, replayed, err := store.ExecuteCommandWithReceipt(context.Background(), 1, 0, "slot", "same-id", input, apply)
			if err == nil || replayed || data != nil || calls != 1 {
				t.Fatal("malformed durable row was accepted")
			}
			testdb.Exec(t, database, `UPDATE character_shop_state SET result_json=$1,input_hash=$2 WHERE character_id=1`, encoded, hash)
		})
	}
	data, replayed, err := store.ExecuteCommandWithReceipt(context.Background(), 1, 0, "slot", "same-id", input, apply)
	if err != nil || !replayed || string(data) != string(original) || calls != 1 {
		t.Fatalf("valid replay not restored data=%s replay=%t calls=%d %v", data, replayed, calls, err)
	}
}

func TestCommandReceiptSupersededByAnyInventoryCommand(t *testing.T) {
	_, store := inventoryDatabase(t)
	effect := func(tx db.DBTX) error { _, err := NewStore(tx).AddItemToInventory(1, 1, 1); return err }
	apply := func(tx db.DBTX) ([]byte, error) {
		if err := effect(tx); err != nil {
			return nil, err
		}
		return []byte(`{"kind":"prize","version":1,"prizeId":7}`), nil
	}
	if _, replay, err := store.ExecuteCommandWithReceipt(context.Background(), 1, 0, "prize", "old", []byte(`{"prizeId":7}`), apply); err != nil || replay {
		t.Fatal("initial prize receipt failed", err)
	}
	if _, err := store.ExecuteCommand(context.Background(), 1, 1, effect); err != nil {
		t.Fatal(err)
	}
	called := false
	data, replay, err := store.ExecuteCommandWithReceipt(context.Background(), 1, 0, "prize", "old", []byte(`{"prizeId":7}`), func(db.DBTX) ([]byte, error) { called = true; return nil, nil })
	if err == nil || replay || data != nil || called {
		t.Fatal("superseded receipt replayed or granted again")
	}
	snapshot, err := store.GetCharacterSnapshot(context.Background(), 1)
	if err != nil || snapshot.CommandRevision != 2 || len(snapshot.Items) != 1 || snapshot.Items[0].Instance.Quantity != 2 {
		t.Fatalf("superseded result mutated inventory: %+v %v", snapshot, err)
	}
}

func TestCommandReceiptRollsBackEffectsReceiptAndRevision(t *testing.T) {
	for _, stage := range []string{"domain", "projection", "outcome", "commit", "cancellation"} {
		t.Run(stage, func(t *testing.T) {
			database, store := inventoryDatabase(t)
			if stage == "commit" {
				testdb.Exec(t, database, `CREATE FUNCTION reject_receipt_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late receipt commit'; END $$;
CREATE CONSTRAINT TRIGGER reject_receipt_commit AFTER UPDATE ON character_shop_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.request_id IS NOT NULL) EXECUTE FUNCTION reject_receipt_commit()`)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			result, replay, err := store.ExecuteCommandWithReceipt(ctx, 1, 0, "slot", "failed", []byte(`{"bet":1}`), func(tx db.DBTX) ([]byte, error) {
				if _, err := NewStore(tx).AddItemToInventory(1, 1, 1); err != nil {
					return nil, err
				}
				switch stage {
				case "domain":
					return nil, fmt.Errorf("effect failed")
				case "projection":
					if _, err := tx.Exec(`ALTER TABLE character_wallet RENAME TO unavailable_wallet`); err != nil {
						return nil, err
					}
				case "outcome":
					return []byte(`null`), nil
				case "cancellation":
					cancel()
				}
				return []byte(`{"kind":"slot","version":1,"roll":3}`), nil
			})
			if err == nil || result != nil || replay {
				t.Fatalf("failed stage published receipt result=%s replay=%t %v", result, replay, err)
			}
			// This read also waits for database/sql's cancellation rollback to finish.
			snapshot, err := store.GetCharacterSnapshot(context.Background(), 1)
			if err != nil || snapshot.CommandRevision != 0 || len(snapshot.Items) != 0 {
				t.Fatalf("failed command escaped rollback %+v %v", snapshot, err)
			}
			var receipts int
			if err := database.QueryRow(`SELECT count(*) FROM character_shop_state`).Scan(&receipts); err != nil || receipts != 0 {
				t.Fatalf("failed durable receipt count=%d %v", receipts, err)
			}
		})
	}
}

func TestCommandReceiptSingleConnectionAndParentBoundary(t *testing.T) {
	database, store := inventoryDatabase(t)
	database.SetMaxOpenConns(1)
	apply := func(tx db.DBTX) ([]byte, error) {
		if _, err := NewStore(tx).AddItemToInventory(1, 1, 1); err != nil {
			return nil, err
		}
		return []byte(`{"kind":"slot","version":1,"roll":1}`), nil
	}
	if _, _, err := store.ExecuteCommandWithReceipt(context.Background(), 1, 0, "slot", "single", nil, apply); err != nil {
		t.Fatal(err)
	}
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	called := false
	result, replay, err := NewStore(tx).ExecuteCommandWithReceipt(context.Background(), 1, 1, "slot", "parent", nil, func(db.DBTX) ([]byte, error) { called = true; return nil, nil })
	if err == nil || called || result != nil || replay {
		t.Fatal("receipt executor joined uncommitted parent")
	}
	var revision int64
	if err := tx.QueryRow(`SELECT revision FROM character_shop_state WHERE character_id=1`).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("parent changed or unusable revision=%d err=%v", revision, err)
	}
}
