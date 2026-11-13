package economy

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

func shopDatabase(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES (1,'one'),(2,'two');
		INSERT INTO character_wallet VALUES (1, 1000),(2,1000);
		INSERT INTO cq_items(id,name,short_name,price,stack_size) VALUES (1,'Potion','POTION',100,99),(2,'Ball','BALL',200,99);
		INSERT INTO cq_merchants(id,name,map_id) VALUES (1,'Shop',38);
		INSERT INTO cq_merchant_items(merchant_id,item_id,price_override,quantity) VALUES (1,1,10,100);`)
	return database, New(database)
}

func assertWalletAndItems(t *testing.T, database *sql.DB, charID int, money, quantity int) {
	t.Helper()
	var gotMoney, gotQuantity int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=$1`, charID).Scan(&gotMoney); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COALESCE(SUM(ii.quantity),0) FROM cq_item_instances ii JOIN cq_character_inventory ci ON ii.id=ci.item_instance_id WHERE ci.character_id=$1`, charID).Scan(&gotQuantity); err != nil {
		t.Fatal(err)
	}
	if gotMoney != money || gotQuantity != quantity {
		t.Fatalf("character %d: money/items = %d/%d, want %d/%d", charID, gotMoney, gotQuantity, money, quantity)
	}
}

func TestPurchaseUsesOfferAndPreservesOverflow(t *testing.T) {
	database, service := shopDatabase(t)
	if _, err := cqitems.NewStore(database).AddItemToInventory(1, 1, 95); err != nil {
		t.Fatal(err)
	}
	result, err := service.Buy(context.Background(), 1, 38, 1, 1, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Money != 900 || result.Quantity != 10 {
		t.Fatalf("purchase = %+v", result)
	}
	if result.Inventory.Money != result.Money || len(result.Inventory.Items) != 2 {
		t.Fatalf("purchase omitted complete split-stack snapshot: %+v", result.Inventory)
	}
	var snapshotQuantity int
	for _, item := range result.Inventory.Items {
		snapshotQuantity += int(item.Instance.Quantity)
	}
	if snapshotQuantity != 105 {
		t.Fatalf("snapshot quantity=%d", snapshotQuantity)
	}
	assertWalletAndItems(t, database, 1, 900, 105)
	var stacks, maxQuantity, stock int
	if err := database.QueryRow(`SELECT COUNT(*), MAX(quantity) FROM cq_item_instances`).Scan(&stacks, &maxQuantity); err != nil {
		t.Fatal(err)
	}
	if stacks != 2 || maxQuantity != 99 {
		t.Fatalf("stacks/max = %d/%d", stacks, maxQuantity)
	}
	if err := database.QueryRow(`SELECT quantity FROM cq_merchant_items`).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 90 {
		t.Fatalf("stock = %d", stock)
	}
}

func TestPurchaseRejectsInvalidOfferQuantityAndInsufficientBalance(t *testing.T) {
	database, service := shopDatabase(t)
	for _, tc := range []struct {
		mapID, merchantID, itemID int32
		quantity                  uint16
	}{
		{39, 1, 1, 1}, {38, 2, 1, 1}, {38, 1, 2, 1}, {38, 1, 1, 0}, {38, 1, 1, 100},
	} {
		if _, err := service.Buy(context.Background(), 1, tc.mapID, tc.merchantID, tc.itemID, tc.quantity, 0); err == nil {
			t.Fatalf("accepted invalid purchase %+v", tc)
		}
	}
	testdb.Exec(t, database, `UPDATE character_wallet SET pokedollars=5 WHERE character_id=1`)
	if _, err := service.Buy(context.Background(), 1, 38, 1, 1, 1, 0); err == nil {
		t.Fatal("accepted insufficient balance")
	}
	assertWalletAndItems(t, database, 1, 5, 0)
}

func TestPurchaseRollsBackPaymentAndGrantOnDatabaseFailure(t *testing.T) {
	database, service := shopDatabase(t)
	// Fail after payment and instance creation, at inventory link insertion.
	testdb.Exec(t, database, `ALTER TABLE cq_character_inventory ADD CONSTRAINT reject_grant CHECK(character_id <> 1)`)
	if result, err := service.Buy(context.Background(), 1, 38, 1, 1, 1, 0); err == nil || result.InstanceID != 0 {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	assertWalletAndItems(t, database, 1, 1000, 0)
	var instances, stock int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_item_instances`).Scan(&instances); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT quantity FROM cq_merchant_items`).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if instances != 0 || stock != 100 {
		t.Fatalf("orphan instances=%d stock=%d", instances, stock)
	}
}

func TestConcurrentPurchasesCannotOverspendOrOversell(t *testing.T) {
	database, service := shopDatabase(t)
	testdb.Exec(t, database, `UPDATE character_wallet SET pokedollars=10; UPDATE cq_merchant_items SET quantity=1`)
	var workers sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func(charID int32) {
			defer workers.Done()
			_, err := service.Buy(context.Background(), charID, 38, 1, 1, 1, 0)
			results <- err
		}(int32(i%2 + 1))
	}
	workers.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful purchases=%d", succeeded)
	}
	var money, items int
	if err := database.QueryRow(`SELECT SUM(pokedollars) FROM character_wallet`).Scan(&money); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT SUM(quantity) FROM cq_item_instances`).Scan(&items); err != nil {
		t.Fatal(err)
	}
	if money != 10 || items != 1 {
		t.Fatalf("money/items=%d/%d", money, items)
	}
}

func TestSaleOwnershipDuplicateAndRollback(t *testing.T) {
	database, service := shopDatabase(t)
	store := cqitems.NewStore(database)
	id, err := store.AddItemToInventory(1, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Sell(context.Background(), 1, 39, id, 0); err == nil {
		t.Fatal("sold without a merchant on the owned map")
	}
	if _, err := service.Sell(context.Background(), 2, 38, id, 0); err == nil {
		t.Fatal("sold another character's stack")
	}
	if err := store.RemoveItemFromInventory(2, id); err == nil {
		t.Fatal("removed another character's stack")
	}
	if _, err := store.DecrementItemQuantity(2, id); err == nil {
		t.Fatal("consumed another character's stack")
	}
	testdb.Exec(t, database, `ALTER TABLE character_wallet ADD CONSTRAINT reject_credit CHECK(pokedollars<=1000)`)
	if _, err := service.Sell(context.Background(), 1, 38, id, 0); err == nil {
		t.Fatal("sale ignored failed credit")
	}
	assertWalletAndItems(t, database, 1, 1000, 3)
	testdb.Exec(t, database, `ALTER TABLE character_wallet DROP CONSTRAINT reject_credit`)
	result, err := service.Sell(context.Background(), 1, 38, id, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.SellPrice != 150 || result.InstanceID != id {
		t.Fatalf("sale=%+v", result)
	}
	if result.Inventory.Money != 1150 || result.Inventory.Items == nil || len(result.Inventory.Items) != 0 {
		t.Fatalf("sale did not return authoritative empty bag: %+v", result.Inventory)
	}
	if _, err := service.Sell(context.Background(), 1, 38, id, 0); err == nil {
		t.Fatal("sold same stack twice")
	}
	assertWalletAndItems(t, database, 1, 1150, 0)
}

func TestSaleSourcePolicyRejectsKeyItemsAndHMsButAcceptsUnstockedItems(t *testing.T) {
	database, service := shopDatabase(t)
	store := cqitems.NewStore(database)
	id, err := store.AddItemToInventory(1, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{
		`UPDATE cq_items SET is_key_item=true WHERE id=1`,
		`UPDATE cq_items SET is_key_item=false,item_type=6 WHERE id=1`,
		`UPDATE cq_items SET item_type=0,price=1 WHERE id=1`,
	} {
		testdb.Exec(t, database, change)
		if result, err := service.Sell(context.Background(), 1, 38, id, 0); err == nil || result.InstanceID != 0 {
			t.Fatalf("unsellable result=%+v error=%v", result, err)
		}
		assertWalletAndItems(t, database, 1, 1000, 2)
	}
	// Item 2 is not in this shop's offers; source sale policy uses the bag,
	// rather than restricting it to the merchant's purchase catalog.
	unstocked, err := store.AddItemToInventory(1, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Sell(context.Background(), 1, 38, unstocked, 0)
	if err != nil || result.SellPrice != 200 || result.Inventory.CommandRevision != 1 {
		t.Fatalf("unstocked sale=%+v %v", result, err)
	}
	assertWalletAndItems(t, database, 1, 1200, 2)
}

func TestPurchaseCancellationReleasesLocks(t *testing.T) {
	database, service := shopDatabase(t)
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE character_data SET id=id WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := service.Buy(ctx, 1, 38, 1, 1, 1, 0); err == nil {
		t.Fatal("purchase did not cancel waiting for lock")
	}
	tx.Rollback()
	assertWalletAndItems(t, database, 1, 1000, 0)
	if _, err := service.Buy(context.Background(), 1, 38, 1, 1, 1, 0); err != nil {
		t.Fatalf("purchase after cancellation: %v", err)
	}
}

func TestShopSnapshotFailureRollsBackMutation(t *testing.T) {
	for _, action := range []string{"buy", "sell"} {
		t.Run(action, func(t *testing.T) {
			database, service := shopDatabase(t)
			store := cqitems.NewStore(database)
			id, err := store.AddItemToInventory(1, 1, 3)
			if err != nil {
				t.Fatal(err)
			}
			other, err := store.AddItemToInventory(1, 2, 1)
			if err != nil {
				t.Fatal(err)
			}
			// Grant/removal succeeds, then the final bag read encounters invalid
			// persisted data in another stack. It must roll back the whole action.
			testdb.Exec(t, database, `UPDATE cq_item_instances SET charges=256 WHERE id=$1`, other)
			if action == "buy" {
				result, err := service.Buy(context.Background(), 1, 38, 1, 1, 1, 0)
				if err == nil || result.InstanceID != 0 || result.Inventory.Items != nil {
					t.Fatalf("partial purchase=%+v %v", result, err)
				}
			} else {
				result, err := service.Sell(context.Background(), 1, 38, id, 0)
				if err == nil || result.InstanceID != 0 || result.Inventory.Items != nil {
					t.Fatalf("partial sale=%+v %v", result, err)
				}
			}
			assertWalletAndItems(t, database, 1, 1000, 4)
			var stock int
			if err := database.QueryRow(`SELECT quantity FROM cq_merchant_items`).Scan(&stock); err != nil || stock != 100 {
				t.Fatalf("failed snapshot changed stock=%d %v", stock, err)
			}
		})
	}
}

func TestCommandRevisionRejectsConcurrentDuplicatesAndSurvivesNewService(t *testing.T) {
	database, service := shopDatabase(t)
	var workers sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := service.Buy(context.Background(), 1, 38, 1, 1, 2, 0)
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("duplicate successes=%d", succeeded)
	}
	assertWalletAndItems(t, database, 1, 980, 2)
	service = New(database) // The guard belongs to durable state, not this process.
	if _, err := service.Buy(context.Background(), 1, 38, 1, 1, 2, 0); err == nil {
		t.Fatal("reconnected duplicate purchased twice")
	}
	snapshot, err := cqitems.NewStore(database).GetCharacterSnapshot(context.Background(), 1)
	if err != nil || snapshot.CommandRevision != 1 {
		t.Fatalf("current revision=%+v %v", snapshot, err)
	}
	id := snapshot.Items[0].Instance.ID
	if _, err := service.Sell(context.Background(), 1, 38, id, 0); err == nil {
		t.Fatal("sale reused stale purchase revision")
	}
	sale, err := service.Sell(context.Background(), 1, 38, id, 1)
	if err != nil || sale.Inventory.CommandRevision != 2 {
		t.Fatalf("new sale=%+v %v", sale, err)
	}
	assertWalletAndItems(t, database, 1, 1080, 0)
	if _, err := service.Sell(context.Background(), 1, 38, id, 1); err == nil {
		t.Fatal("duplicate sale accepted")
	}
}

func TestCommandRevisionLateCommitFailureRollsBackGuardAndEffects(t *testing.T) {
	database, service := shopDatabase(t)
	testdb.Exec(t, database, `CREATE FUNCTION reject_shop_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late shop commit failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_shop_commit AFTER UPDATE ON character_shop_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_shop_commit();`)
	result, err := service.Buy(context.Background(), 1, 38, 1, 1, 1, 0)
	if err == nil || result.Inventory.Items != nil || result.InstanceID != 0 {
		t.Fatalf("failed commit published result=%+v %v", result, err)
	}
	assertWalletAndItems(t, database, 1, 1000, 0)
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_shop_state`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed commit advanced guard=%d %v", count, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_shop_commit ON character_shop_state`)
	result, err = service.Buy(context.Background(), 1, 38, 1, 1, 1, 0)
	if err != nil || result.Inventory.CommandRevision != 1 {
		t.Fatalf("retry after rollback=%+v %v", result, err)
	}
}

func TestShopSchemaValidationRejectsMissingCommandState(t *testing.T) {
	database, service := shopDatabase(t)
	if err := service.ValidateSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `DROP TABLE character_shop_state`)
	if err := service.ValidateSchema(context.Background()); err == nil {
		t.Fatal("missing shop schema passed readiness")
	}
}
