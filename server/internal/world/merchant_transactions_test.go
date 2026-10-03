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
	request := clientPacket(opcodes.CQMerchantBuyRequest, `{"merchantId":1,"itemId":1,"quantity":1}`)
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
	if len(messenger.streams) != 2 || messenger.streams[1].opcode != opcodes.CQInventoryResponse {
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
	var snapshot CQInventoryResponse
	if err := json.Unmarshal(messenger.streams[1].payload, &snapshot); err != nil || snapshot.Money != bought.Inventory.Money || len(snapshot.Items) != 1 || snapshot.Items[0].Instance.ID != bought.Inventory.Items[0].Instance.ID {
		t.Fatalf("compatibility stream differs from commit: %+v %v", snapshot, err)
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
