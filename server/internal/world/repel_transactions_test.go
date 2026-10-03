package world

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
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
	for _, request := range []struct {
		opcode  opcodes.OpCode
		payload string
	}{
		{opcodes.CQItemUseRequest, fmt.Sprintf(`{"instanceId":%d}`, instance)},
		{opcodes.RepelUseRequest, `{"itemId":30}`},
	} {
		messages.streams = nil
		battleDispatch(t, wh, ses, request.opcode, request.payload)
		if len(messages.streams) != 1 {
			t.Fatalf("failed activation messages %+v", messages.streams)
		}
		var response struct {
			Success bool
			Error   string
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error != "Could not use the repel. Please try again." {
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
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_repel_commit ON character_repels`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.RepelUseRequest, `{"itemId":30}`)
	var success struct {
		Success   bool
		StepsLeft int
	}
	if len(messages.streams) != 1 {
		t.Fatalf("success messages %+v", messages.streams)
	}
	if err := json.Unmarshal(messages.streams[0].payload, &success); err != nil || !success.Success || success.StepsLeft != RepelSteps {
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
			_, err := UseRepelInventoryItem(context.Background(), owned, 42, 30, nil)
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else if err.Error() != "A repel is already active!" {
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
