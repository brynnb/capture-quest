package world

import (
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

func TestSlotMachineBoundaryUsesSourceTargetAndOwnedLuck(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 INSERT INTO character_coins(character_id,coins) VALUES(42,100);
 INSERT INTO phaser_hidden_objects(id,map_constant,map_id,x,y,item_or_direction,routine) VALUES
 (1,'GAME_CORNER',135,18,15,'ANY_FACING','StartSlotMachine'),
 (2,'GAME_CORNER',135,18,14,'SLOTS_OUTTOLUNCH','StartSlotMachine'),
 (3,'GAME_CORNER',135,18,13,'ANY_FACING','OtherRoutine')`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	ses.MapID = 135
	ses.Client.CharData().MapID = 135
	ses.Client.CharData().X = 17
	ses.Client.CharData().Y = 15
	if _, err := ses.GameCorner.LuckyIndex(42, 135, func() (int, error) { return 1, nil }); err != nil {
		t.Fatal(err)
	}
	deny := func(payload string) {
		t.Helper()
		previous, err := gameCornerCoinBalance(database, 42)
		if err != nil {
			t.Fatal(err)
		}
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.GameCornerSlotPlayRequest, payload)
		var result struct {
			Success bool
			Error   string
		}
		if len(messages.streams) != 1 {
			t.Fatalf("messages %+v", messages.streams)
		}
		if err := json.Unmarshal(messages.streams[0].payload, &result); err != nil || result.Success || result.Error == "" {
			t.Fatalf("denial %+v %v", result, err)
		}
		coins, err := gameCornerCoinBalance(database, 42)
		if err != nil || coins != previous {
			t.Fatalf("denial spent coins %d %d %v", previous, coins, err)
		}
	}
	deny(`{"bet":1,"isLucky":true}`) // Old targetless packets cannot play.
	deny(`{"bet":1,"machineX":18,"machineY":14,"isLucky":true}`)
	deny(`{"bet":1,"machineX":17,"machineY":15,"isLucky":true}`)
	ses.Client.CharData().Y = 13
	deny(`{"bet":1,"machineX":18,"machineY":13}`)
	ses.Client.CharData().Y = 15
	ses.Client.CharData().X = 1
	deny(`{"bet":1,"machineX":18,"machineY":15}`)
	ses.Client.CharData().X = 17
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameCornerSlotPlayRequest, `{"bet":1,"machineX":18,"machineY":15,"isLucky":false}`)
	var result struct {
		Success bool
		IsLucky bool
	}
	if len(messages.streams) != 1 {
		t.Fatalf("messages %+v", messages.streams)
	}
	if err := json.Unmarshal(messages.streams[0].payload, &result); err != nil || !result.Success || !result.IsLucky {
		t.Fatalf("owned luck %+v %v", result, err)
	}
	// Presence publication, rather than a client modal action, ends the map visit.
	ses.Client.CharData().MapID = 137
	ses.PublishPresence()
	index, err := ses.GameCorner.LuckyIndex(42, 135, func() (int, error) { return 2, nil })
	if err != nil || index != 2 {
		t.Fatalf("map departure retained luck: %d %v", index, err)
	}
	ses.Client.CharData().MapID = 135
	// Moving away after opening the modal must revoke reach for later spins.
	ses.Client.CharData().X = 1
	deny(`{"bet":1,"machineX":18,"machineY":15}`)
}
