package world

import (
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	model "capturequest/internal/db/models"
	"capturequest/internal/economy"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestMerchantOpenDispatchUsesInjectedOwnedMapAndRejectsReadFailures(t *testing.T) {
	database := testdb.Postgres(t)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(1,'one');
		INSERT INTO character_wallet VALUES(1,100);
		INSERT INTO cq_items(id,name,short_name,price) VALUES(1,'Potion','POTION',10);
		INSERT INTO cq_merchants(id,name,map_id) VALUES(1,'Shop',38),(2,'Remote',39);
		INSERT INTO cq_merchant_items(merchant_id,item_id) VALUES(1,1),(2,1);`)
	messages := &recordingMessenger{}
	ses := &session.Session{Authenticated: true, MapID: 38, Client: &testSessionClient{char: &model.CharacterData{ID: 1}}, Messenger: messages}
	registry := NewWorldOpCodeRegistry()
	registry.WH = &WorldHandler{Economy: economy.New(database)}
	check := func(payload string, wantSuccess bool) {
		t.Helper()
		messages.streams = nil
		registry.HandleWorldPacket(ses, clientPacket(opcodes.CQMerchantOpenRequest, payload))
		var result struct {
			Success bool
			Money   int64
			Items   []cqitems.CQMerchantItem
			Error   string
		}
		if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.CQMerchantOpenResponse || json.Unmarshal(messages.streams[0].payload, &result) != nil || result.Success != wantSuccess {
			t.Fatalf("request=%s response=%+v", payload, messages.streams)
		}
		if wantSuccess && (result.Money != 100 || len(result.Items) != 1) {
			t.Fatalf("incomplete menu=%+v", result)
		}
		if !wantSuccess && (result.Error == "" || result.Items != nil) {
			t.Fatalf("partial failure=%+v", result)
		}
	}
	check(`{"mapId":38}`, true)
	check(`{"merchantId":1}`, true)
	for _, payload := range []string{`{"mapId":39}`, `{"merchantId":2}`, `{"merchantId":1,"mapId":39}`, `{}`, `{"mapId":38,"unexpected":1}`, `{"mapId":-1}`} {
		check(payload, false)
	}
	testdb.Exec(t, database, `DROP TABLE character_wallet`)
	check(`{"mapId":38}`, false)
}

func TestMerchantDispatchPublishesOnlyCommittedResults(t *testing.T) {
	database := testdb.Postgres(t)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: database}
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(1,'one');
		INSERT INTO character_wallet VALUES(1,100);
		INSERT INTO cq_items(id,name,short_name,price) VALUES(1,'Potion','POTION',10);
		INSERT INTO cq_merchants(id,name,map_id) VALUES(1,'Shop',38);
		INSERT INTO cq_merchant_items(merchant_id,item_id) VALUES(1,1);
		ALTER TABLE cq_character_inventory ADD CONSTRAINT fail_grant CHECK(character_id<>1);`)
	messenger := &recordingMessenger{}
	ses := &session.Session{Authenticated: true, MapID: 38, Client: &testSessionClient{char: &model.CharacterData{ID: 1}}, Messenger: messenger}
	registry := NewWorldOpCodeRegistry()
	registry.WH = &WorldHandler{Economy: economy.New(database)}
	request := clientPacket(opcodes.CQMerchantBuyRequest, `{"requestId":"buy","shop":{"characterId":1,"revision":0},"merchantId":1,"itemId":1,"quantity":1}`)
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
	if bought.RequestID != "buy" || bought.Inventory.ShopRevision != 1 {
		t.Fatalf("purchase lost command identity=%+v", bought)
	}
	for _, invalid := range []string{
		`{"merchantId":1,"itemId":1,"quantity":1}`,
		`{"requestId":"missing-revision","shop":{"characterId":1},"merchantId":1,"itemId":1,"quantity":1}`,
		`{"requestId":"wrong-owner","shop":{"characterId":2,"revision":1},"merchantId":1,"itemId":1,"quantity":1}`,
		`{"requestId":"duplicate","shop":{"characterId":1,"revision":0},"merchantId":1,"itemId":1,"quantity":1}`,
	} {
		messenger.streams = nil
		registry.HandleWorldPacket(ses, clientPacket(opcodes.CQMerchantBuyRequest, invalid))
		var rejection ShopCommandError
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
