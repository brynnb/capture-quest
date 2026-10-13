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
	testdb.Exec(t, database, `UPDATE character_data SET map_id=1,x=0,y=0 WHERE id=42`)
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	db.GlobalWorldDB = nil // The dispatcher and grant must use owned dependencies.
	request := fmt.Sprintf(`{"actorId":%d,"requestId":"pickup-test","command":{"characterId":42,"revision":0}}`, actorID)
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
	var revision int
	if err := database.QueryRow(`SELECT COALESCE((SELECT revision FROM character_shop_state WHERE character_id=42),0)`).Scan(&revision); err != nil || revision != 0 {
		t.Fatalf("failed collection changed revision: %d %v", revision, err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("grant escaped rollback: %d %v", count, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_collected_items DROP CONSTRAINT reject_collection`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.ItemPickupRequest, request)
	if len(messages.streams) != 2 || messages.streams[1].opcode != opcodes.ResourcesChangedNotify {
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
	if err := database.QueryRow(`SELECT revision FROM character_shop_state WHERE character_id=42`).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("duplicate changed revision: %d %v", revision, err)
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
	testdb.Exec(t, database, `UPDATE character_data SET map_id=1,x=0,y=0 WHERE id=42`)
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	deny := func() {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.ItemPickupRequest, fmt.Sprintf(`{"actorId":%d,"requestId":"pickup-test","command":{"characterId":42,"revision":0}}`, actorID))
		var response struct{ Success bool }
		if len(messages.streams) != 1 {
			t.Fatalf("denial publication: %+v", messages.streams)
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success {
			t.Fatalf("denial: %+v %v", response, err)
		}
	}
	ses.Client.CharData().X = 8
	testdb.Exec(t, database, `UPDATE character_data SET x=8 WHERE id=42`)
	deny()
	ses.Client.CharData().X = 0
	testdb.Exec(t, database, `UPDATE character_data SET x=0 WHERE id=42`)
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

func TestItemPickupUsesLockedDurableSourceVisibilityAndBattle(t *testing.T) {
	for _, kind := range []string{"durable-source-remote", "cached-source-remote", "durable-hidden", "cached-hidden", "durable-battle", "single-connection"} {
		t.Run(kind, func(t *testing.T) {
			database, wh, ses, messages := battleTestWorld(t)
			wh.ActorRegistry = NewActorRegistry()
			wh.ActorManager = NewPhaserActorManager(wh)
			testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'ROOM',10,10,0);
    INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,item_id) VALUES(10,1,1,0,'item','POTION',1);
    UPDATE character_data SET map_id=1,x=0,y=0 WHERE id=42`)
			ses.Client.CharData().MapID = 1
			if kind == "durable-source-remote" {
				testdb.Exec(t, database, `UPDATE character_data SET x=8 WHERE id=42`)
			}
			if kind == "cached-source-remote" {
				ses.Client.CharData().X = 8
			}
			if kind == "durable-hidden" || kind == "cached-hidden" {
				testdb.Exec(t, database, `INSERT INTO phaser_event_object_visibility(map_id,map_name,object_name,visible,requires_flag) VALUES(1,'ROOM','POTION',false,'PICKUP_HIDDEN')`)
				wh.EventFlags.flags[42] = map[string]bool{"PICKUP_HIDDEN": kind == "cached-hidden"}
				if kind == "durable-hidden" {
					testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'PICKUP_HIDDEN')`)
				}
			}
			if kind == "durable-battle" {
				battleTestStart(t, database, false, nil)
			}
			if kind == "single-connection" {
				database.SetMaxOpenConns(1)
			}
			actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
			battleDispatch(t, wh, ses, opcodes.ItemPickupRequest, fmt.Sprintf(`{"actorId":%d,"requestId":"pickup-test","command":{"characterId":42,"revision":0}}`, actorID))
			allowed := kind == "cached-source-remote" || kind == "cached-hidden" || kind == "single-connection"
			var reply struct{ Success bool }
			if len(messages.streams) == 0 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success != allowed {
				t.Fatalf("kind=%s reply=%+v streams=%+v", kind, reply, messages.streams)
			}
			var grants, markers int
			if err := database.QueryRow(`SELECT count(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&grants); err != nil {
				t.Fatal(err)
			}
			if err := database.QueryRow(`SELECT count(*) FROM character_collected_items WHERE character_id=42`).Scan(&markers); err != nil {
				t.Fatal(err)
			}
			want := 0
			if allowed {
				want = 1
			}
			if grants != want || markers != want {
				t.Fatalf("grant=%d marker=%d wanted=%d", grants, markers, want)
			}
		})
	}
}

func TestItemPickupRejectsRetiredAndForeignCommandEnvelopes(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'ROOM',10,10,0); INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,item_id) VALUES(10,1,1,0,'item','POTION',1);UPDATE character_data SET map_id=1,x=0,y=0 WHERE id=42`)
	actor := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	for _, payload := range []string{fmt.Sprintf(`{"actorId":%d}`, actor), fmt.Sprintf(`{"actorId":%d,"requestId":"foreign","command":{"characterId":43,"revision":0}}`, actor), fmt.Sprintf(`{"actorId":%d,"requestId":"missing-revision","command":{"characterId":42}}`, actor), fmt.Sprintf(`{"actorId":%d,"requestId":"unknown","command":{"characterId":42,"revision":0},"legacy":true}`, actor)} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.ItemPickupRequest, payload)
		var reply struct{ Success bool }
		if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success {
			t.Fatal("retired/foreign envelope accepted")
		}
	}
	var markers, grants, states int
	if err := database.QueryRow(`SELECT (SELECT count(*) FROM character_collected_items),(SELECT count(*) FROM cq_character_inventory),(SELECT count(*) FROM character_shop_state)`).Scan(&markers, &grants, &states); err != nil || markers != 0 || grants != 0 || states != 0 {
		t.Fatalf("invalid envelope mutated state: %d %d %d %v", markers, grants, states, err)
	}
}
