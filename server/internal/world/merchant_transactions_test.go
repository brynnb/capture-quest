package world

import (
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
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
	var bought struct {
		Success bool
		Money   int
	}
	if err := json.Unmarshal(messenger.streams[0].payload, &bought); err != nil {
		t.Fatal(err)
	}
	if !bought.Success || bought.Money != 90 {
		t.Fatalf("purchase response=%+v", bought)
	}
}
