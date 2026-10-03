package world

import (
	"encoding/json"
	"fmt"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	model "capturequest/internal/db/models"
	"capturequest/internal/itemuse"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestItemDispatchPublishesOnlyCommittedEffects(t *testing.T) {
	database := testdb.Postgres(t)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: database}
	t.Cleanup(func() { removeBattle(1); db.GlobalWorldDB = previous })
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(1,'one');
	INSERT INTO character_wallet VALUES(1,1000);
 INSERT INTO phaser_pokemon(id,name,type_1,hp,atk,def,spd,spc,catch_rate,base_exp) VALUES(25,'PIKACHU','ELECTRIC',35,55,30,90,50,190,82);
	INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,cur_hp,max_hp) VALUES(1,0,0,25,50,1,95);
	INSERT INTO cq_items(id,name,short_name,is_usable,heal_amount) VALUES(1,'Potion','POTION',true,20);
	ALTER TABLE character_pokemon ADD CONSTRAINT fail_heal CHECK(cur_hp=1);`)
	instance, err := cqitems.NewStore(database).AddItemToInventory(1, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	messenger := &recordingMessenger{}
	ses := &session.Session{Authenticated: true, Client: &testSessionClient{char: &model.CharacterData{ID: 1}}, Messenger: messenger}
	registry := NewWorldOpCodeRegistry()
	registry.WH = &WorldHandler{database: database, Items: itemuse.New(database)}
	request := clientPacket(opcodes.CQItemUseRequest, fmt.Sprintf(`{"requestId":"use","command":{"characterId":1,"revision":0},"pokemonRowId":1,"instanceId":%d,"partySlot":0,"moveSlot":-1}`, instance))
	registry.HandleWorldPacket(ses, request)
	if len(messenger.streams) != 1 || messenger.streams[0].opcode != opcodes.CQItemUseResponse {
		t.Fatalf("failure messages=%+v", messenger.streams)
	}
	var failure struct {
		Success bool
		Error   string
	}
	if err := json.Unmarshal(messenger.streams[0].payload, &failure); err != nil {
		t.Fatal(err)
	}
	if failure.Success || failure.Error != "Could not use this item. Please try again." {
		t.Fatalf("failed transaction response=%+v", failure)
	}
	testdb.Exec(t, database, `ALTER TABLE character_pokemon DROP CONSTRAINT fail_heal`)
	// A field-item request cannot overwrite the active battle's authoritative state.
	setBattle(1, &pokebattle.BattleState{Phase: pokebattle.PhaseActionSelect})
	messenger.streams = nil
	registry.HandleWorldPacket(ses, request)
	if len(messenger.streams) != 1 {
		t.Fatalf("battle guard messages=%+v", messenger.streams)
	}
	if err := json.Unmarshal(messenger.streams[0].payload, &failure); err != nil {
		t.Fatal(err)
	}
	if failure.Success {
		t.Fatal("allowed field item during active battle")
	}
	removeBattle(1)
	messenger.streams = nil
	registry.HandleWorldPacket(ses, request)
	if len(messenger.streams) != 1 || messenger.streams[0].opcode != opcodes.CQItemUseResponse {
		t.Fatalf("committed messages=%+v", messenger.streams)
	}
	var success CQPartyItemUseResponse
	if err := json.Unmarshal(messenger.streams[0].payload, &success); err != nil {
		t.Fatal(err)
	}
	if !success.Success || success.RequestID != "use" || success.Outcome.NewQuantity != 0 || success.Inventory.CommandRevision != 1 || len(success.Inventory.Items) != 0 || len(success.Party) != 1 || success.Party[0].CurHP != 21 || success.Party[0].RowID != 1 {
		t.Fatalf("committed projection=%+v", success)
	}
}
