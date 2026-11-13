package cqitems

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestOwnedInstanceReaderRequiresAgreementAndCancelsBlockedReads(t *testing.T) {
	database, store := inventoryDatabase(t)
	id, err := store.AddItemToInventory(1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.FindInventoryItemByInstanceIDContext(context.Background(), 1, id)
	if err != nil || item == nil || item.Instance.ID != id {
		t.Fatalf("owned item=%+v %v", item, err)
	}
	for _, mutation := range []string{
		`UPDATE cq_item_instances SET owner_id=2`,
		`UPDATE cq_item_instances SET owner_id=1,owner_type=1`,
	} {
		testdb.Exec(t, database, mutation)
		item, err := store.FindInventoryItemByInstanceIDContext(context.Background(), 1, id)
		if err != sql.ErrNoRows || item != nil {
			t.Fatalf("foreign instance=%+v %v", item, err)
		}
		if _, err := store.FindInventoryItemByInstanceID(1, id); err != sql.ErrNoRows {
			t.Fatalf("legacy API bypassed ownership: %v", err)
		}
	}
	testdb.Exec(t, database, `UPDATE cq_item_instances SET owner_type=0`)
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`LOCK TABLE cq_item_instances IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if item, err := store.FindInventoryItemByInstanceIDContext(ctx, 1, id); err == nil || item != nil {
		t.Fatalf("cancelled instance=%+v %v", item, err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("owned reader ignored caller deadline")
	}
}

func TestInventorySnapshotZeroWalletAndReadFailures(t *testing.T) {
	database, store := inventoryDatabase(t)
	testdb.Exec(t, database, `CREATE FUNCTION reject_snapshot_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'snapshot wrote character'; END $$;
 CREATE TRIGGER reject_snapshot_update BEFORE UPDATE ON character_data FOR EACH ROW EXECUTE FUNCTION reject_snapshot_update();`)
	snapshot, err := store.GetCharacterSnapshot(context.Background(), 1)
	if err != nil || snapshot.Items == nil || len(snapshot.Items) != 0 || snapshot.Money != 0 {
		t.Fatalf("empty owned snapshot=%+v %v", snapshot, err)
	}
	testdb.Exec(t, database, `DROP TABLE character_wallet`)
	snapshot, err = store.GetCharacterSnapshot(context.Background(), 1)
	if err == nil || snapshot.Items != nil {
		t.Fatalf("query failure became success: %+v %v", snapshot, err)
	}
}

func TestInventorySnapshotCannotReadUncommittedBagAndWallet(t *testing.T) {
	database, store := inventoryDatabase(t)
	testdb.Exec(t, database, `INSERT INTO character_wallet VALUES(1,100)`)
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE character_data SET id=id WHERE id=1; UPDATE character_wallet SET pokedollars=50 WHERE character_id=1`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewStore(tx).AddItemToInventory(1, 1, 2); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	snapshot, err := store.GetCharacterSnapshot(ctx, 1)
	if err == nil || snapshot.Items != nil {
		t.Fatalf("read crossed uncommitted ownership: %+v %v", snapshot, err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.GetCharacterSnapshot(context.Background(), 1)
	if err != nil || snapshot.Money != 50 || len(snapshot.Items) != 1 || snapshot.Items[0].Instance.Quantity != 2 {
		t.Fatalf("committed bag/balance=%+v %v", snapshot, err)
	}
}

func inventoryDatabase(t *testing.T) (*sql.DB, *Store) {
	t.Helper()
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(1,'one');
		INSERT INTO cq_items(id,name,short_name,stackable,stack_size) VALUES(1,'Potion','POTION',true,99),(2,'Key','KEY',false,1)`)
	return database, NewStore(database)
}

func TestConcurrentInventoryConsumptionAndGrants(t *testing.T) {
	database, store := inventoryDatabase(t)
	id, err := store.AddItemToInventory(1, 1, 4)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	errors := make(chan error, 8)
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); _, err := store.DecrementItemQuantity(1, id); errors <- err }()
	}
	workers.Wait()
	for i := 0; i < 4; i++ {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.DecrementItemQuantity(1, id); err == nil {
		t.Fatal("consumed deleted instance")
	}
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() { defer workers.Done(); _, err := store.AddItemToInventory(1, 1, 60); errors <- err }()
	}
	workers.Wait()
	for i := 0; i < 4; i++ {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	var count, total, maxQuantity int
	if err := database.QueryRow(`SELECT COUNT(*),SUM(quantity),MAX(quantity) FROM cq_item_instances`).Scan(&count, &total, &maxQuantity); err != nil {
		t.Fatal(err)
	}
	if count != 3 || total != 240 || maxQuantity != 99 {
		t.Fatalf("stacks/total/max = %d/%d/%d", count, total, maxQuantity)
	}
}

func TestNonstackableGrantAndNestedGrantRollback(t *testing.T) {
	database, store := inventoryDatabase(t)
	if _, err := store.AddItemToInventory(1, 2, 3); err != nil {
		t.Fatal(err)
	}
	var count, maxQuantity int
	if err := database.QueryRow(`SELECT COUNT(*),MAX(quantity) FROM cq_item_instances`).Scan(&count, &maxQuantity); err != nil {
		t.Fatal(err)
	}
	if count != 3 || maxQuantity != 1 {
		t.Fatalf("nonstackable grant = %d stacks of max %d", count, maxQuantity)
	}
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := NewStore(tx).AddItemToInventory(1, 1, 7); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_item_instances WHERE item_id=1`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("nested grant escaped rollback")
	}
}

func TestInventoryRemoveRollsBackLinkOnDeleteFailure(t *testing.T) {
	database, store := inventoryDatabase(t)
	id, err := store.AddItemToInventory(1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `CREATE TABLE held_item(id integer REFERENCES cq_item_instances(id))`)
	testdb.Exec(t, database, `INSERT INTO held_item VALUES($1)`, id)
	if err := store.RemoveItemFromInventory(1, id); err == nil {
		t.Fatal("expected referenced-item deletion failure")
	}
	if _, err := store.FindInventoryItemByInstanceID(1, id); err != nil {
		t.Fatalf("inventory link was not rolled back: %v", err)
	}
}
