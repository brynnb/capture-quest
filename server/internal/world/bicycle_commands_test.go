package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
	"encoding/json"
	"fmt"
	"testing"
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
