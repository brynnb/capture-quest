package world

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestItemPickupRollbackPublicationAndRetry(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'ROOM',10,10,0);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,item_id) VALUES(10,1,1,0,'item','POTION',1);
 ALTER TABLE character_collected_items ADD CONSTRAINT reject_collection CHECK(object_id<>10)`)
	ses.Client.CharData().MapID = 1
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	db.GlobalWorldDB = nil // The dispatcher and grant must use owned dependencies.
	request := fmt.Sprintf(`{"actorId":%d}`, actorID)
	battleDispatch(t, wh, ses, opcodes.ItemPickupRequest, request)
	if len(messages.streams) != 1 {
		t.Fatalf("failure publication: %+v", messages.streams)
	}
	var response struct {
		Success    bool
		InstanceID int32
	}
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success {
		t.Fatalf("failure response: %+v %v", response, err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("grant escaped rollback: %d %v", count, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_collected_items DROP CONSTRAINT reject_collection`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.ItemPickupRequest, request)
	if len(messages.streams) != 2 || messages.streams[1].opcode != opcodes.CQInventoryResponse {
		t.Fatalf("success publication: %+v", messages.streams)
	}
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || !response.Success || response.InstanceID == 0 {
		t.Fatalf("success response: %+v %v", response, err)
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.ItemPickupRequest, request)
	if len(messages.streams) != 1 {
		t.Fatalf("duplicate publication: %+v", messages.streams)
	}
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success {
		t.Fatalf("duplicate response: %+v %v", response, err)
	}
	if err := database.QueryRow(`SELECT SUM(ii.quantity) FROM cq_character_inventory ci JOIN cq_item_instances ii ON ii.id=ci.item_instance_id WHERE ci.character_id=42`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate grant: %d %v", count, err)
	}
}

func TestItemPickupConcurrentCollection(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_objects(id,map_id,object_type,item_id) VALUES(10,1,'item',1)`)
	results := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := collectItem(context.Background(), database, 42, 10); results <- err }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if err.Error() != "Already collected" {
			t.Fatal(err)
		}
	}
	var grants, markers int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&grants); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_collected_items WHERE character_id=42`).Scan(&markers); err != nil {
		t.Fatal(err)
	}
	if successes != 1 || grants != 1 || markers != 1 {
		t.Fatalf("successes=%d grants=%d markers=%d", successes, grants, markers)
	}
}

func TestItemPickupRejectsRemoteHiddenAndCounterTargets(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'ROOM',10,10,0);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,item_id) VALUES(10,1,1,0,'item','POTION',1)`)
	ses.Client.CharData().MapID = 1
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	deny := func() {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.ItemPickupRequest, fmt.Sprintf(`{"actorId":%d}`, actorID))
		var response struct{ Success bool }
		if len(messages.streams) != 1 {
			t.Fatalf("denial publication: %+v", messages.streams)
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success {
			t.Fatalf("denial: %+v %v", response, err)
		}
	}
	ses.Client.CharData().X = 8
	deny()
	ses.Client.CharData().X = 0
	testdb.Exec(t, database, `INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,false,'test')`)
	deny()
	testdb.Exec(t, database, `DELETE FROM character_object_visibility_overrides; UPDATE phaser_objects SET x=2 WHERE id=10;
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,talk_over_tile) VALUES(1,1,0,1,true)`)
	deny()
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_collected_items`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("denial mutated collection: %d %v", count, err)
	}
}
