package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/testdb"
)

func TestEscapeRopeRejectsSourceChangedBeforeCharacterLock(t *testing.T) {
	for _, change := range []string{`UPDATE character_data SET map_id=60,x=3,y=4 WHERE id=42`, `UPDATE character_data SET x=8 WHERE id=42`} {
		t.Run(change, func(t *testing.T) {
			database, _, _, _ := battleTestWorld(t)
			testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_usable) VALUES(29,'Escape Rope','ESCAPE_ROPE',true); INSERT INTO phaser_maps(id,name,width,height) VALUES(50,'CAVE',20,20),(60,'EXIT',20,20); INSERT INTO phaser_warps(id,source_map_id,x,y,destination_map_id,destination_x,destination_y) VALUES(1,50,1,1,60,3,4); UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42`)
			instance, err := cqitems.NewStore(database).AddItemToInventory(42, 29, 2)
			if err != nil {
				t.Fatal(err)
			}
			lock, err := database.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback()
			if err := db.LockCharacter(lock, 42); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				_, err := useEscapeRope(ctx, database, 42, instance, 50, 7, 8, 0, func(id int) int { return id })
				finished <- err
			}()
			if _, err := lock.Exec(change); err != nil {
				t.Fatal(err)
			}
			var expectedMap, expectedX, expectedY int
			if err := lock.QueryRow(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=42`).Scan(&expectedMap, &expectedX, &expectedY); err != nil {
				t.Fatal(err)
			}
			if err := lock.Commit(); err != nil {
				t.Fatal(err)
			}
			var rejection *itemuse.Rejection
			if err := <-finished; !errors.As(err, &rejection) {
				t.Fatalf("source change accepted: %v", err)
			}
			owned, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance)
			if err != nil || owned.Instance.Quantity != 2 {
				t.Fatalf("rope consumed: %+v %v", owned, err)
			}
			var savedMap, savedX, savedY int
			if err := database.QueryRow(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=42`).Scan(&savedMap, &savedX, &savedY); err != nil || savedMap != expectedMap || savedX != expectedX || savedY != expectedY {
				t.Fatal("rejected rope overwrote the competing position")
			}
		})
	}
}

func TestBicycleDispatchRequiresOwnedInstanceAndReportsReadFailures(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Items = itemuse.New(database)
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
	// Correlated party commands cannot enter the legacy field-effect branch.
	assertRejected(fmt.Sprintf(`{"instanceId":%d,"requestId":"wrong-family","command":{"characterId":42,"revision":0}}`, instance), "That item can't be used outside of battle")
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
	if wh.PlayerMovement.players[42].WantsBicycle {
		t.Fatal("retired bicycle packet changed preference")
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
	request := fmt.Sprintf(`{"requestId":"rope","command":{"characterId":42,"revision":0},"instanceId":%d,"mapId":50,"x":7,"y":8}`, instance)
	battleDispatch(t, wh, ses, opcodes.EscapeRopeUseRequest, request)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.EscapeRopeUseResponse {
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
	battleDispatch(t, wh, ses, opcodes.EscapeRopeUseRequest, request)
	assertPosition(60, 3, 4)
	if _, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance); err == nil {
		t.Fatal("successful escape did not consume rope")
	}
	if len(messages.streams) != 2 || messages.streams[0].opcode != opcodes.EscapeRopeUseResponse || messages.streams[1].opcode != opcodes.ResourcesChangedNotify {
		t.Fatalf("success messages: %+v", messages.streams)
	}
	var success EscapeRopeUseResponse
	if err := json.Unmarshal(messages.streams[0].payload, &success); err != nil || !success.Success || success.CharacterID != 42 {
		t.Fatalf("success response %+v: %v", success, err)
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.EscapeRopeUseRequest, request)
	assertPosition(60, 3, 4)
	if len(messages.streams) != 1 {
		t.Fatalf("stale instance replay published a teleport: %+v", messages.streams)
	}
}

func TestEscapeRopeOldCommandCannotConsumeAgainAfterReturningToSource(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_usable) VALUES(29,'Escape Rope','ESCAPE_ROPE',true); INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'CAVE',20,20,0),(60,'EXIT',20,20,0); INSERT INTO phaser_warps(id,source_map_id,x,y,destination_map_id,destination_x,destination_y) VALUES(1,50,1,1,60,3,4); UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42`)
	instance, err := cqitems.NewStore(database).AddItemToInventory(42, 29, 2)
	if err != nil {
		t.Fatal(err)
	}
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	request := fmt.Sprintf(`{"requestId":"rope","command":{"characterId":42,"revision":0},"instanceId":%d,"mapId":50,"x":7,"y":8}`, instance)
	battleDispatch(t, wh, ses, opcodes.EscapeRopeUseRequest, request)
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42`)
	// Fresh registration at the same source must not erase durable admission.
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.EscapeRopeUseRequest, request)
	var response struct{ Success bool }
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || response.Success {
		t.Fatal("old command replay accepted")
	}
	snapshot, err := cqitems.NewStore(database).GetCharacterSnapshot(context.Background(), 42)
	if err != nil || snapshot.CommandRevision != 1 || len(snapshot.Items) != 1 || snapshot.Items[0].Instance.Quantity != 1 {
		t.Fatalf("replay changed bag/revision: %+v %v", snapshot, err)
	}
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || mapID != 50 || x != 7 || y != 8 {
		t.Fatal("old command moved fresh owner")
	}
}

func TestEscapeRopeRetiresExpiredStepButRejectsLiveAuthorization(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO cq_items(id,name,short_name,is_usable) VALUES(29,'Escape Rope','ESCAPE_ROPE',true); INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(60,'EXIT',20,20,0); INSERT INTO phaser_warps(id,source_map_id,x,y,destination_map_id,destination_x,destination_y) VALUES(1,50,1,1,60,3,4)`)
	instance, err := cqitems.NewStore(wh.database).AddItemToInventory(42, 29, 2)
	if err != nil {
		t.Fatal(err)
	}
	step := issueStep(t, wh, ses, messages)
	request := fmt.Sprintf(`{"requestId":"rope","command":{"characterId":42,"revision":0},"instanceId":%d,"mapId":50,"x":7,"y":8}`, instance)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.EscapeRopeUseRequest, request)
	var response EscapeRopeUseResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || response.Success {
		t.Fatal("live step ownership ignored")
	}
	snapshot, err := cqitems.NewStore(wh.database).GetCharacterSnapshot(context.Background(), 42)
	if err != nil || snapshot.CommandRevision != 0 || snapshot.Items[0].Instance.Quantity != 2 {
		t.Fatalf("live rejection changed bag: %+v %v", snapshot, err)
	}
	wh.PlayerMovement.players[42].pendingStep.issuedAt = time.Now().Add(-playerStepLifetime)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.EscapeRopeUseRequest, request)
	if len(messages.streams) != 2 || json.Unmarshal(messages.streams[0].payload, &response) != nil || !response.Success {
		t.Fatalf("expired step blocked Escape Rope: %+v", messages.streams)
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"requestId":"late","stepToken":%q}`, step.StepToken))
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
	snapshot, err = cqitems.NewStore(wh.database).GetCharacterSnapshot(context.Background(), 42)
	if !ok || mapID != 60 || x != 3 || y != 4 || err != nil || snapshot.CommandRevision != 1 || snapshot.Items[0].Instance.Quantity != 1 {
		t.Fatal("expired completion rewound escape or consumed twice")
	}
}
