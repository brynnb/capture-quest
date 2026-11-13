package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBicycleDesiredStateIsRevisionAndSessionOwned(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO cq_items(id,name,short_name,is_usable) VALUES(100,'Bicycle','BICYCLE',true)`)
	instance, err := cqitems.NewStore(wh.database).AddItemToInventory(42, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	request := fmt.Sprintf(`{"requestId":"ride","characterId":42,"instanceId":%d,"wantsRiding":true,"revision":0}`, instance)
	battleDispatch(t, wh, ses, opcodes.BicycleStateRequest, request)
	var result BicycleStateResponse
	for _, m := range messages.streams {
		if m.opcode == opcodes.BicycleStateResponse {
			if err := json.Unmarshal(m.payload, &result); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !result.Success || !result.Bicycle.WantsRiding || result.Bicycle.Revision != 1 {
		t.Fatalf("result=%+v", result)
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.BicycleStateRequest, request)
	var rejected struct{ Success bool }
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &rejected) != nil || rejected.Success {
		t.Fatal("duplicate desired state accepted")
	}
	current, ok := wh.PlayerMovement.bicycleStateForSession(ses.SessionID, 42)
	if !ok || !current.WantsRiding || current.Revision != 1 {
		t.Fatal("duplicate changed movement preference")
	}
	if _, ok := wh.PlayerMovement.setBicycleForSession(ses.SessionID+1, 42, false, 1); ok {
		t.Fatal("replacement owner accepted")
	}
	owned, err := cqitems.NewStore(wh.database).FindInventoryItemByInstanceID(42, instance)
	if err != nil || owned.Instance.Quantity != 1 {
		t.Fatal("bicycle consumed")
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.BicycleStateRequest, `{"requestId":"current","characterId":42}`)
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &result) != nil || !result.Success || result.Bicycle.Revision != 1 {
		t.Fatal("current-state recovery failed")
	}
}

func TestBicycleAdmissionRejectsUnownedItemsAndDurableBattleOwners(t *testing.T) {
	for _, scenario := range []string{"foreign item", "empty item", "wrong item", "uncached battle", "terminal battle", "safari battle", "read failure"} {
		t.Run(scenario, func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			testdb.Exec(t, wh.database, `INSERT INTO cq_items(id,name,short_name,is_usable) VALUES(100,'Bicycle','BICYCLE',true)`)
			instance, err := cqitems.NewStore(wh.database).AddItemToInventory(42, 100, 1)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "foreign item":
				testdb.Exec(t, wh.database, `UPDATE cq_item_instances SET owner_id=43`)
			case "empty item":
				testdb.Exec(t, wh.database, `UPDATE cq_item_instances SET quantity=0`)
			case "wrong item":
				testdb.Exec(t, wh.database, `UPDATE cq_items SET short_name='TOWN_MAP' WHERE id=100`)
			case "uncached battle", "terminal battle":
				current := battleTestStart(t, wh.database, false, nil)
				if scenario == "terminal battle" {
					_, err := pokebattle.CommitBattle(context.Background(), wh.database, 42, current, func(tx db.DBTX, b *pokebattle.BattleState) error {
						b.EnemyParty[0].CurHP = 0
						b.Phase = pokebattle.PhaseBattleEnd
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				forgetBattle(42, getBattle(42))
			case "safari battle":
				wh.Safari = NewSafariZoneManager(wh.database)
				seedSafariBattle(t, wh.Safari, 3)
			case "read failure":
				testdb.Exec(t, wh.database, `ALTER TABLE cq_items RENAME TO unavailable_items`)
			}
			battleDispatch(t, wh, ses, opcodes.BicycleStateRequest, fmt.Sprintf(`{"requestId":"deny","characterId":42,"instanceId":%d,"wantsRiding":true,"revision":0}`, instance))
			var reply struct{ Success bool }
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success {
				t.Fatalf("admission accepted: %+v", messages.streams)
			}
			state, ok := wh.PlayerMovement.bicycleStateForSession(ses.SessionID, 42)
			if !ok || state.WantsRiding || state.Revision != 0 {
				t.Fatalf("rejection changed state: %+v", state)
			}
			// A rejection never prevents recovery of the session preference.
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.BicycleStateRequest, `{"requestId":"recover","characterId":42}`)
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || !reply.Success {
				t.Fatal("recovery unavailable")
			}
		})
	}
}

func TestBicycleCancelledAdmissionCannotChangeMovementPreference(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO cq_items(id,name,short_name,is_usable) VALUES(100,'Bicycle','BICYCLE',true)`)
	instance, err := cqitems.NewStore(wh.database).AddItemToInventory(42, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	lock, err := wh.database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if err := db.LockCharacter(lock, 42); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = ses.ExecuteCommand(ctx, func() {
		HandleBicycleState(ses, []byte(fmt.Sprintf(`{"requestId":"cancelled","characterId":42,"instanceId":%d,"wantsRiding":true,"revision":0}`, instance)), wh)
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation not propagated: %v", err)
	}
	state, ok := wh.PlayerMovement.bicycleStateForSession(ses.SessionID, 42)
	if !ok || state.WantsRiding || state.Revision != 0 {
		t.Fatalf("cancelled admission changed state: %+v", state)
	}
	for _, message := range messages.streams {
		var reply struct{ Success bool }
		if message.opcode != opcodes.BicycleStateResponse || json.Unmarshal(message.payload, &reply) != nil || reply.Success {
			t.Fatal("cancelled admission published success")
		}
	}
}
