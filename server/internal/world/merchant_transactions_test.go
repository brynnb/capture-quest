package world

import (
	"encoding/json"
	"fmt"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	model "capturequest/internal/db/models"
	"capturequest/internal/economy"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestMerchantOpenDispatchUsesSourceReachEligibilityAndCorrelation(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Economy = economy.New(database)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.Cutscenes = NewCutsceneManager(database)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'ROOM',10,10),(2,'REMOTE',10,10);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text,sprite_name) VALUES(10,1,1,0,'npc','CLERK','CLERK','SPRITE_CLERK'),(11,2,1,0,'npc','REMOTE','REMOTE','SPRITE_CLERK');
 INSERT INTO cq_merchants(id,name,map_id) VALUES(1,'Shop',1),(2,'Remote',2);
 INSERT INTO cq_merchant_items(merchant_id,item_id) VALUES(1,1),(2,1)`)
	ses.Client.CharData().MapID = 1
	ses.MapID = 1
	db.GlobalWorldDB = nil
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	remoteID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 11)
	request := func(id int, want bool) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.CQMerchantOpenRequest, fmt.Sprintf(`{"requestId":"open","characterId":42,"actorId":%d}`, id))
		var response struct {
			Success     bool
			RequestID   string
			CharacterID int64
			Money       int64
			Items       []cqitems.CQMerchantItem
			Error       string
		}
		if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || response.Success != want || response.RequestID != "open" {
			t.Fatalf("response=%+v", messages.streams)
		}
		if want && (response.CharacterID != 42 || response.Money != 100 || len(response.Items) != 1) {
			t.Fatalf("menu=%+v", response)
		}
		if !want && (response.Error == "" || response.Items != nil) {
			t.Fatalf("failure=%+v", response)
		}
	}
	request(actorID, true)
	request(10, false)
	request(remoteID, false)
	ses.Client.CharData().X = 8
	request(actorID, false)
	ses.Client.CharData().X = 0
	testdb.Exec(t, database, `INSERT INTO phaser_event_object_visibility(map_id,map_name,object_name,visible) VALUES(1,'ROOM','CLERK',false)`)
	request(actorID, false)
	testdb.Exec(t, database, `DELETE FROM phaser_event_object_visibility`)
	flag := "SCRIPT_REQUIRED"
	wh.Cutscenes.byLabel["CLERK"] = &CutsceneScript{ScriptLabel: "CLERK", MapName: "ROOM", TriggerType: "npc_click", RequiresFlag: &flag, Actions: json.RawMessage(`[]`)}
	wh.EventFlags.flags[42] = map[string]bool{flag: true} // Stale cache must not determine eligibility.
	request(actorID, true)
	testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'SCRIPT_REQUIRED')`)
	wh.EventFlags.flags[42] = map[string]bool{}
	request(actorID, false)
	delete(wh.Cutscenes.byLabel, "CLERK")
	for _, payload := range []string{`{"requestId":"open","characterId":99,"actorId":1}`, `{"requestId":"open","characterId":42,"mapId":1}`, `{"requestId":"open","characterId":42,"actorId":1,"mapId":1}`} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.CQMerchantOpenRequest, payload)
		var result InventoryCommandError
		if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &result) != nil || result.Success || result.RequestID != "open" {
			t.Fatalf("invalid request=%s reply=%+v", payload, messages.streams)
		}
	}
	testdb.Exec(t, database, `DROP TABLE character_wallet`)
	request(actorID, false)
	requiredMoney := 1
	wh.Cutscenes.byLabel["CLERK"] = &CutsceneScript{ScriptLabel: "CLERK", MapName: "ROOM", TriggerType: "npc_click", RequiresMoney: &requiredMoney}
	request(actorID, false)
	var failedEligibility InventoryCommandError
	if err := json.Unmarshal(messages.streams[0].payload, &failedEligibility); err != nil || failedEligibility.Error != "Shop eligibility unavailable" {
		t.Fatalf("eligibility failure bypassed script: %+v %v", failedEligibility, err)
	}

}

func TestMerchantDispatchPublishesOnlyCommittedResults(t *testing.T) {
	database := testdb.Postgres(t)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: database}
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(38,'SHOP',10,10);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text,sprite_name) VALUES(10,38,1,0,'npc','CLERK','CLERK','SPRITE_CLERK');
 INSERT INTO character_data(id,name) VALUES(1,'one');
		INSERT INTO character_wallet VALUES(1,100);
		INSERT INTO cq_items(id,name,short_name,price) VALUES(1,'Potion','POTION',10);
		INSERT INTO cq_merchants(id,name,map_id) VALUES(1,'Shop',38);
		INSERT INTO cq_merchant_items(merchant_id,item_id) VALUES(1,1);
		ALTER TABLE cq_character_inventory ADD CONSTRAINT fail_grant CHECK(character_id<>1);`)
	messenger := &recordingMessenger{}
	ses := &session.Session{Authenticated: true, MapID: 38, Client: &testSessionClient{char: &model.CharacterData{ID: 1, MapID: 38}}, Messenger: messenger}
	registry := NewWorldOpCodeRegistry()
	registry.WH = &WorldHandler{database: database, Economy: economy.New(database), ActorRegistry: NewActorRegistry(), Cutscenes: NewCutsceneManager(database)}
	registry.WH.ActorManager = NewPhaserActorManager(registry.WH)
	actorID := registry.WH.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	request := clientPacket(opcodes.CQMerchantBuyRequest, fmt.Sprintf(`{"actorId":%d,"requestId":"buy","command":{"characterId":1,"revision":0},"merchantId":1,"itemId":1,"quantity":1}`, actorID))
	registry.HandleWorldPacket(ses, request)
	if len(messenger.streams) != 1 || messenger.streams[0].opcode != opcodes.CQMerchantBuyResponse {
		t.Fatalf("failure messages=%+v", messenger.streams)
	}
	var failed struct{ Success bool }
	if err := json.Unmarshal(messenger.streams[0].payload, &failed); err != nil {
		t.Fatal(err)
	}
	if failed.Success {
		t.Fatal("published purchase success after failed grant")
	}
	testdb.Exec(t, database, `ALTER TABLE cq_character_inventory DROP CONSTRAINT fail_grant`)
	messenger.streams = nil
	registry.HandleWorldPacket(ses, request)
	if len(messenger.streams) != 1 || messenger.streams[0].opcode != opcodes.CQMerchantBuyResponse {
		t.Fatalf("success messages=%+v", messenger.streams)
	}
	var bought CQMerchantBuyResponse
	if err := json.Unmarshal(messenger.streams[0].payload, &bought); err != nil {
		t.Fatal(err)
	}
	if !bought.Success || bought.Money != 90 {
		t.Fatalf("purchase response=%+v", bought)
	}
	if bought.Inventory.Money != 90 || len(bought.Inventory.Items) != 1 || bought.Inventory.Items[0].Instance.Quantity != 1 {
		t.Fatalf("mutation reply omitted committed bag: %+v", bought)
	}
	if bought.RequestID != "buy" || bought.Inventory.CommandRevision != 1 {
		t.Fatalf("purchase lost command identity=%+v", bought)
	}
	for _, invalid := range []string{
		`{"merchantId":1,"itemId":1,"quantity":1}`,
		`{"requestId":"missing-revision","command":{"characterId":1},"merchantId":1,"itemId":1,"quantity":1}`,
		`{"requestId":"wrong-owner","command":{"characterId":2,"revision":1},"merchantId":1,"itemId":1,"quantity":1}`,
		fmt.Sprintf(`{"actorId":%d,"requestId":"duplicate","command":{"characterId":1,"revision":0},"merchantId":1,"itemId":1,"quantity":1}`, actorID),
	} {
		messenger.streams = nil
		registry.HandleWorldPacket(ses, clientPacket(opcodes.CQMerchantBuyRequest, invalid))
		var rejection InventoryCommandError
		if len(messenger.streams) != 1 || json.Unmarshal(messenger.streams[0].payload, &rejection) != nil || rejection.Success || rejection.Error == "" {
			t.Fatalf("invalid command=%s publication=%+v", invalid, messenger.streams)
		}
	}
	var money, quantity int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=1`).Scan(&money); err != nil || money != 90 {
		t.Fatalf("duplicate changed money=%d %v", money, err)
	}
	if err := database.QueryRow(`SELECT SUM(quantity) FROM cq_item_instances`).Scan(&quantity); err != nil || quantity != 1 {
		t.Fatalf("duplicate granted quantity=%d %v", quantity, err)
	}

}

func TestMerchantMutationsRecheckReachVisibilityAndCurrentScriptEligibility(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Economy = economy.New(database)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.Cutscenes = NewCutsceneManager(database)
	ses.MapID = 1
	ses.Client.CharData().MapID = 1
	testdb.Exec(t, database, `UPDATE cq_items SET price=10 WHERE id=1;
		INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'SHOP',10,10),(2,'OTHER',10,10);
		INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text,sprite_name) VALUES(10,1,1,0,'npc','CLERK','CLERK','SPRITE_CLERK'),(11,2,1,0,'npc','REMOTE','REMOTE','SPRITE_CLERK'),(12,1,1,0,'npc','NURSE','NURSE','SPRITE_NURSE');
		INSERT INTO cq_merchants(id,name,map_id) VALUES(1,'Shop',1),(2,'Other',2);
		INSERT INTO cq_merchant_items(merchant_id,item_id) VALUES(1,1)`)
	instanceID, err := cqitems.NewStore(database).AddItemToInventory(42, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	clerkID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	packet := func(op opcodes.OpCode, actorID int, revision int) string {
		if op == opcodes.CQMerchantBuyRequest {
			return fmt.Sprintf(`{"requestId":"command","actorId":%d,"command":{"characterId":42,"revision":%d},"merchantId":1,"itemId":1,"quantity":1}`, actorID, revision)
		}
		return fmt.Sprintf(`{"requestId":"command","actorId":%d,"command":{"characterId":42,"revision":%d},"instanceId":%d}`, actorID, revision, instanceID)
	}
	check := func(actorID int, revision int, want bool, ops ...opcodes.OpCode) {
		t.Helper()
		for _, op := range ops {
			messages.streams = nil
			battleDispatch(t, wh, ses, op, packet(op, actorID, revision))
			var result struct {
				Success   bool
				RequestID string
			}
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &result) != nil || result.Success != want || result.RequestID != "command" {
				t.Fatalf("opcode=%d response=%+v", op, messages.streams)
			}
		}
	}
	ops := []opcodes.OpCode{opcodes.CQMerchantBuyRequest, opcodes.CQMerchantSellRequest}
	// Even a successfully opened menu is not permission to mutate after moving.
	battleDispatch(t, wh, ses, opcodes.CQMerchantOpenRequest, fmt.Sprintf(`{"requestId":"open","characterId":42,"actorId":%d}`, clerkID))
	ses.Client.CharData().X = 8
	check(clerkID, 0, false, ops...)
	ses.Client.CharData().X = 0
	check(0, 0, false, ops...)
	check(10, 0, false, ops...)
	check(wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 11), 0, false, ops...)
	check(wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 12), 0, false, ops...)
	testdb.Exec(t, database, `INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,false,'test')`)
	check(clerkID, 0, false, ops...)
	testdb.Exec(t, database, `DELETE FROM character_object_visibility_overrides`)
	flag := "SCRIPT_REQUIRED"
	wh.Cutscenes.byLabel["CLERK"] = &CutsceneScript{ScriptLabel: "CLERK", MapName: "SHOP", TriggerType: "npc_click", RequiresFlag: &flag}
	testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'SCRIPT_REQUIRED')`)
	wh.EventFlags.flags[42] = map[string]bool{} // Stale session cache cannot skip the script.
	check(clerkID, 0, false, ops...)
	delete(wh.Cutscenes.byLabel, "CLERK")
	var money, quantity, revision int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 100 {
		t.Fatalf("rejection money=%d %v", money, err)
	}
	if err := database.QueryRow(`SELECT quantity FROM cq_item_instances WHERE id=$1`, instanceID).Scan(&quantity); err != nil || quantity != 2 {
		t.Fatalf("rejection quantity=%d %v", quantity, err)
	}
	if err := database.QueryRow(`SELECT COALESCE((SELECT revision FROM character_shop_state WHERE character_id=42),0)`).Scan(&revision); err != nil || revision != 0 {
		t.Fatalf("rejection advanced revision=%d %v", revision, err)
	}
	check(clerkID, 0, true, opcodes.CQMerchantBuyRequest)
	check(clerkID, 1, true, opcodes.CQMerchantSellRequest)
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 105 {
		t.Fatalf("sale money=%d %v", money, err)
	}
	battleTestStart(t, database, false, nil)
	check(clerkID, 2, false, ops...)
	var rejected InventoryCommandError
	if err := json.Unmarshal(messages.streams[0].payload, &rejected); err != nil || rejected.Error != "Shop unavailable during battle" {
		t.Fatalf("battle admitted shop: %+v %v", rejected, err)
	}
}

func TestInventoryDispatchFailsWholeReadOnWalletError(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	if _, err := cqitems.NewStore(wh.database).AddItemToInventory(42, 1, 2); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, wh.database, `DROP TABLE character_wallet`)
	battleDispatch(t, wh, ses, opcodes.CQInventoryRequest, `{}`)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.CQInventoryResponse {
		t.Fatalf("inventory publication=%+v", messages.streams)
	}
	var reply struct {
		Success bool
		Error   string
		Items   []cqitems.CQInventoryItem
	}
	if err := json.Unmarshal(messages.streams[0].payload, &reply); err != nil || reply.Success || reply.Error == "" || reply.Items != nil {
		t.Fatalf("wallet query failure cleared/published bag: %+v %v", reply, err)
	}
}
