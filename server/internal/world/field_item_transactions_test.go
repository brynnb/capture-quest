package world

import (
	"encoding/json"
	"fmt"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

func TestBicycleDispatchRequiresOwnedInstanceAndReportsReadFailures(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_usable) VALUES(100,'Bicycle','BICYCLE',true)`)
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, UnifiedOverworldMapID, "UP")
	db.GlobalWorldDB = nil
	request := fmt.Sprintf(`{"instanceId":%d}`, instance)
	assertRejected := func(payload, expected string) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.CQItemUseRequest, payload)
		if wh.PlayerMovement.players[42].WantsBicycle || len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.CQItemUseResponse {
			t.Fatalf("rejection changed bicycle or published unexpected messages: %+v", messages.streams)
		}
		var response struct {
			Success bool
			Error   string
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error != expected {
			t.Fatalf("response=%+v error=%v", response, err)
		}
	}
	assertRejected(fmt.Sprintf(`{"instanceId":%d,"unknown":true}`, instance), "Invalid item use request")
	for _, mutation := range []string{
		`UPDATE cq_item_instances SET owner_id=43`,
		`UPDATE cq_item_instances SET owner_id=42,owner_type=1`,
		`UPDATE cq_item_instances SET owner_type=0,quantity=0`,
	} {
		testdb.Exec(t, database, mutation)
		assertRejected(request, "Item not found in inventory")
	}
	testdb.Exec(t, database, `UPDATE cq_item_instances SET quantity=1; ALTER TABLE cq_items RENAME TO unavailable_items`)
	assertRejected(request, "Could not read this item. Please try again.")
	testdb.Exec(t, database, `ALTER TABLE unavailable_items RENAME TO cq_items`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.CQItemUseRequest, request)
	if !wh.PlayerMovement.players[42].WantsBicycle {
		t.Fatal("owned bicycle did not toggle")
	}
	owned, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance)
	if err != nil || owned.Instance.Quantity != 1 {
		t.Fatalf("reusable bicycle was consumed: %+v %v", owned, err)
	}
}

func TestEscapeRopeCommitFailureDoesNotConsumeMoveOrPublish(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_usable) VALUES(29,'Escape Rope','ESCAPE_ROPE',true);
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'CAVE',20,20,0),(60,'EXIT',20,20,0);
 INSERT INTO phaser_warps(id,source_map_id,x,y,destination_map_id,destination_x,destination_y) VALUES(1,50,1,1,60,3,4);
 UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;
 CREATE FUNCTION reject_escape_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject escape commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_escape_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.map_id=60) EXECUTE FUNCTION reject_escape_commit();`)
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 29, 1)
	if err != nil {
		t.Fatal(err)
	}
	ses.Client.CharData().MapID = 50
	ses.Client.CharData().X = 7
	ses.Client.CharData().Y = 8
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	db.GlobalWorldDB = nil
	request := fmt.Sprintf(`{"instanceId":%d}`, instance)
	battleDispatch(t, wh, ses, opcodes.CQItemUseRequest, request)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.CQItemUseResponse {
		t.Fatalf("failed use published unexpected messages: %+v", messages.streams)
	}
	var response struct {
		Success bool
		Error   string
	}
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error == "" {
		t.Fatalf("failure response %+v: %v", response, err)
	}
	assertPosition := func(mapID, x, y int) {
		t.Helper()
		var savedMap int
		var savedX, savedY float64
		if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&savedMap, &savedX, &savedY); err != nil || savedMap != mapID || savedX != float64(x) || savedY != float64(y) {
			t.Fatalf("saved position (%d,%v,%v), want (%d,%d,%d): %v", savedMap, savedX, savedY, mapID, x, y, err)
		}
		mx, my, mm, ok := wh.PlayerMovement.GetPosition(42)
		char := ses.Client.CharData()
		if !ok || mx != x || my != y || mm != mapID || char.MapID != uint32(mapID) || char.X != float64(x) || char.Y != float64(y) {
			t.Fatalf("movement (%d,%d,%d) char %+v, want (%d,%d,%d)", mm, mx, my, char, mapID, x, y)
		}
	}
	assertPosition(50, 7, 8)
	found, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance)
	if err != nil || found.Instance.Quantity != 1 {
		t.Fatalf("failed escape consumed item: %+v %v", found, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_escape_commit ON character_data`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.CQItemUseRequest, request)
	assertPosition(60, 3, 4)
	if _, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance); err == nil {
		t.Fatal("successful escape did not consume rope")
	}
	if len(messages.streams) != 2 || messages.streams[0].opcode != opcodes.WarpTileTeleportNotify || messages.streams[1].opcode != opcodes.CQItemUseResponse {
		t.Fatalf("success messages: %+v", messages.streams)
	}
	var success struct {
		Success bool
		NewQty  uint16
	}
	if err := json.Unmarshal(messages.streams[1].payload, &success); err != nil || !success.Success || success.NewQty != 0 {
		t.Fatalf("success response %+v: %v", success, err)
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.CQItemUseRequest, request)
	assertPosition(60, 3, 4)
	if len(messages.streams) != 1 {
		t.Fatalf("stale instance replay published a teleport: %+v", messages.streams)
	}
}
