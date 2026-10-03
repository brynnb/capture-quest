package world

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/economy"
	"capturequest/internal/testdb"
)

func TestRepelDispatcherCommitFailureAndDurableExpiry(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_usable,stackable,stack_size) VALUES(30,'Repel','REPEL',true,true,99);
 CREATE FUNCTION reject_repel_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject repel commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_repel_commit AFTER INSERT ON character_repels DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_repel_commit();`)
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 30, 2)
	if err != nil {
		t.Fatal(err)
	}
	wh.WildEncounter = NewWildEncounterManager(wh, database)
	db.GlobalWorldDB = nil
	request := fmt.Sprintf(`{"requestId":"repel","command":{"characterId":42,"revision":0},"instanceId":%d}`, instance)
	for _, stage := range []string{"commit", "projection"} {
		if stage == "projection" {
			testdb.Exec(t, database, `DROP TRIGGER reject_repel_commit ON character_repels; ALTER TABLE character_wallet RENAME TO unavailable_wallet`)
		}
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.RepelUseRequest, request)
		if len(messages.streams) != 1 {
			t.Fatalf("failed activation messages %+v", messages.streams)
		}
		var response struct {
			Success   bool
			Error     string
			RequestID string
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.RequestID != "repel" || response.Error != "Could not use the repel. Please try again." {
			t.Fatalf("activation failure %+v: %v", response, err)
		}
		found, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance)
		if err != nil || found.Instance.Quantity != 2 {
			t.Fatalf("failed activation consumed inventory %+v: %v", found, err)
		}
		status, err := wh.WildEncounter.RepelStatus(context.Background(), 42)
		if err != nil || status.Active {
			t.Fatalf("failed activation published repel %+v: %v", status, err)
		}
		var revision int64
		if err := database.QueryRow(`SELECT COALESCE((SELECT revision FROM character_shop_state WHERE character_id=42),0)`).Scan(&revision); err != nil || revision != 0 {
			t.Fatalf("failed activation advanced revision: %d %v", revision, err)
		}
	}
	testdb.Exec(t, database, `ALTER TABLE unavailable_wallet RENAME TO character_wallet`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.RepelUseRequest, request)
	var success RepelUseResponse
	if len(messages.streams) != 1 {
		t.Fatalf("success messages %+v", messages.streams)
	}
	if err := json.Unmarshal(messages.streams[0].payload, &success); err != nil || !success.Success || success.StepsLeft != RepelSteps || success.RequestID != "repel" || success.InstanceID != instance || success.Inventory.CommandRevision != 1 || len(success.Inventory.Items) != 1 || success.Inventory.Items[0].Instance.Quantity != 1 {
		t.Fatalf("success %+v: %v", success, err)
	}
	// A new manager has no inherited pointer/cache; the committed effect survives.
	wh.WildEncounter = NewWildEncounterManager(wh, database)
	status, err := wh.WildEncounter.RepelStatus(context.Background(), 42)
	if err != nil || !status.Active || status.StepsLeft != RepelSteps {
		t.Fatalf("recreated manager status %+v: %v", status, err)
	}
	if err := wh.WildEncounter.SetRepelSteps(context.Background(), 42, 1); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `CREATE CONSTRAINT TRIGGER reject_repel_expiry AFTER DELETE ON character_repels DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_repel_commit()`)
	messages.streams = nil
	if _, err := wh.WildEncounter.tickRepel(42, ses); err == nil {
		t.Fatal("expected failed expiry commit")
	}
	status, err = wh.WildEncounter.RepelStatus(context.Background(), 42)
	if err != nil || status.StepsLeft != 1 || len(messages.streams) != 0 {
		t.Fatalf("failed expiry status %+v messages %+v: %v", status, messages.streams, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_repel_expiry ON character_repels`)
	if _, err := wh.WildEncounter.tickRepel(42, ses); err != nil {
		t.Fatal(err)
	}
	if _, err := wh.WildEncounter.tickRepel(42, ses); err != nil {
		t.Fatal(err)
	}
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.RepelWoreOffNotify {
		t.Fatalf("expiry notification %+v", messages.streams)
	}
}

func TestConcurrentRepelActivationConsumesOnceAcrossManagers(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_usable,stackable,stack_size) VALUES(30,'Repel','REPEL',true,true,99)`)
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 30, 4)
	if err != nil {
		t.Fatal(err)
	}
	db.GlobalWorldDB = nil
	results := make(chan error, 4)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			owned := &WorldHandler{database: database}
			owned.WildEncounter = NewWildEncounterManager(owned, database)
			_, err := UseRepelInventoryItem(context.Background(), owned, 42, instance, 0)
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else if !strings.Contains(err.Error(), "stale") {
			t.Fatal(err)
		}
	}
	found, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance)
	if err != nil || accepted != 1 || found.Instance.Quantity != 3 {
		t.Fatalf("accepted=%d inventory %+v: %v", accepted, found, err)
	}
	wh.WildEncounter = NewWildEncounterManager(wh, database)
	status, err := wh.WildEncounter.RepelStatus(context.Background(), 42)
	if err != nil || status.StepsLeft != RepelSteps {
		t.Fatalf("durable status %+v: %v", status, err)
	}
}

func TestRepelRevisionSurvivesExpiryAndIsSharedWithShop(t *testing.T) {
	for _, itemID := range []int32{RepelItemID, SuperRepelItemID, MaxRepelItemID} {
		t.Run(fmt.Sprint(itemID), func(t *testing.T) {
			database, wh, _, _ := battleTestWorld(t)
			testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,price,is_usable,stackable,stack_size) VALUES($1,'Repel','REPEL',100,true,true,99)`, itemID)
			testdb.Exec(t, database, `INSERT INTO cq_merchants(id,name,map_id) VALUES(1,'Shop',1)`)
			store := cqitems.NewStore(database)
			instance, err := store.AddItemToInventory(42, itemID, 3)
			if err != nil {
				t.Fatal(err)
			}
			wh.WildEncounter = NewWildEncounterManager(wh, database)
			ctx := context.Background()
			first, err := UseRepelInventoryItem(ctx, wh, 42, instance, 0)
			steps, _ := RepelStepsForItem(int(itemID))
			if err != nil || first.StepsLeft != steps || first.Inventory.CommandRevision != 1 || first.Inventory.Items[0].Instance.Quantity != 2 {
				t.Fatalf("first=%+v %v", first, err)
			}
			// A new intention while active fails without spending a revision.
			if _, err := UseRepelInventoryItem(ctx, wh, 42, instance, 1); err == nil || err.Error() != "A repel is already active!" {
				t.Fatalf("active rejection=%v", err)
			}
			if err := wh.WildEncounter.SetRepelSteps(ctx, 42, 1); err != nil {
				t.Fatal(err)
			}
			if wore, err := wh.WildEncounter.AdvanceRepelStep(ctx, 42); err != nil || !wore {
				t.Fatalf("expiry=%v %v", wore, err)
			}
			wh.WildEncounter = NewWildEncounterManager(wh, database)
			// Active-status checks alone cannot reject this delayed duplicate.
			if _, err := UseRepelInventoryItem(ctx, wh, 42, instance, 0); err == nil || !strings.Contains(err.Error(), "stale") {
				t.Fatalf("expired replay=%v", err)
			}
			second, err := UseRepelInventoryItem(ctx, wh, 42, instance, 1)
			if err != nil || second.Inventory.CommandRevision != 2 || second.Inventory.Items[0].Instance.Quantity != 1 {
				t.Fatalf("second=%+v %v", second, err)
			}
			shop := economy.New(database)
			if _, err := shop.Sell(ctx, 42, 1, instance, 1); err == nil || !strings.Contains(err.Error(), "stale") {
				t.Fatalf("cross-family replay=%v", err)
			}
			sale, err := shop.Sell(ctx, 42, 1, instance, 2)
			if err != nil || sale.Inventory.CommandRevision != 3 || len(sale.Inventory.Items) != 0 {
				t.Fatalf("sale=%+v %v", sale, err)
			}
		})
	}
}

func TestRepelDispatchRejectsLegacyIdentityAndForeignInstances(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.WildEncounter = NewWildEncounterManager(wh, database)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,stackable,stack_size) VALUES(30,'Repel','REPEL',true,99)`)
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 30, 2)
	if err != nil {
		t.Fatal(err)
	}
	requests := []struct {
		opcode  opcodes.OpCode
		payload string
	}{
		{opcodes.RepelUseRequest, `{"itemId":30}`},
		{opcodes.CQItemUseRequest, fmt.Sprintf(`{"instanceId":%d}`, instance)},
		{opcodes.RepelUseRequest, fmt.Sprintf(`{"requestId":"bad","instanceId":%d}`, instance)},
		{opcodes.RepelUseRequest, fmt.Sprintf(`{"requestId":"bad","command":{"characterId":43,"revision":0},"instanceId":%d}`, instance)},
		{opcodes.RepelUseRequest, fmt.Sprintf(`{"requestId":"bad","command":{"characterId":42},"instanceId":%d}`, instance)},
	}
	valid := fmt.Sprintf(`{"requestId":"owned","command":{"characterId":42,"revision":0},"instanceId":%d}`, instance)
	for _, req := range requests {
		messages.streams = nil
		battleDispatch(t, wh, ses, req.opcode, req.payload)
		var response InventoryCommandError
		if len(messages.streams) != 1 {
			t.Fatalf("missing rejection: %+v", messages.streams)
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error == "" {
			t.Fatalf("invalid request accepted: %+v %v", response, err)
		}
	}
	for _, mutation := range []string{`UPDATE cq_item_instances SET owner_id=43`, `UPDATE cq_item_instances SET owner_id=42,owner_type=1`} {
		testdb.Exec(t, database, mutation)
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.RepelUseRequest, valid)
		var response InventoryCommandError
		if len(messages.streams) != 1 {
			t.Fatalf("missing ownership rejection: %+v", messages.streams)
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error != "You don't have that item." {
			t.Fatalf("foreign instance=%+v %v", response, err)
		}
	}
	testdb.Exec(t, database, `UPDATE cq_item_instances SET owner_type=0`)
	battleTestStart(t, database, false, nil)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.RepelUseRequest, valid)
	var rejection InventoryCommandError
	if len(messages.streams) != 1 {
		t.Fatalf("missing battle rejection: %+v", messages.streams)
	}
	if err := json.Unmarshal(messages.streams[0].payload, &rejection); err != nil || rejection.Success || rejection.Error != "Use the battle item menu during a battle" {
		t.Fatalf("battle rejection=%+v %v", rejection, err)
	}
	snapshot, err := cqitems.NewStore(database).GetCharacterSnapshot(context.Background(), 42)
	status, statusErr := wh.WildEncounter.RepelStatus(context.Background(), 42)
	if err != nil || statusErr != nil || status.Active || snapshot.CommandRevision != 0 || snapshot.Items[0].Instance.Quantity != 2 {
		t.Fatalf("rejections changed state: %+v %+v %v %v", snapshot, status, err, statusErr)
	}
}
