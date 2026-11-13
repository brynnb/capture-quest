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
	"capturequest/internal/testdb"
)

func TestScriptInteractionAuthorizesServerPositionAndVisibility(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.Cutscenes = NewCutsceneManager(database)
	cs := &CutsceneScript{ScriptLabel: "GREETING", MapName: "ROOM", TriggerType: "npc_click", Actions: json.RawMessage(`[]`)}
	wh.Cutscenes.byLabel[cs.ScriptLabel] = cs
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'ROOM',10,10,0),(2,'OTHER',10,10,0),(3,'ROUTE',10,10,1);
		INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text) VALUES(10,1,1,0,'npc','GREETING','GREETING');
		INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,talk_over_tile) VALUES(1,1,0,1,true)`)
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	ses.Client.CharData().MapID = 1
	db.GlobalWorldDB = nil // All authorization loaders use the captured dependency.
	request := func(want bool) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.ScriptedEventInteractRequest, fmt.Sprintf(`{"actorId":%d}`, actorID))
		if len(messages.streams) == 0 || messages.streams[0].opcode != opcodes.ScriptedEventInteractResponse {
			t.Fatal("missing interaction response")
		}
		var response ScriptedEventInteractResponse
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil {
			t.Fatal(err)
		}
		if response.Started != want || response.Success != want {
			_, _, targetErr := wh.scriptInteractionTarget(ses, 10)
			t.Fatalf("response=%+v want started=%v target error=%v", response, want, targetErr)
		}
		if want && (len(messages.streams) != 2 || messages.streams[1].opcode != opcodes.CutsceneStartNotify) {
			t.Fatal("authorized interaction did not issue cutscene")
		}
		if !want && len(messages.streams) != 1 {
			t.Fatal("rejected interaction issued cutscene")
		}
	}
	request(true)
	ses.Client.CharData().MapID = 2
	request(false)
	ses.Client.CharData().MapID = 1
	ses.Client.CharData().X, ses.Client.CharData().Y = 8, 8
	request(false)
	ses.Client.CharData().X, ses.Client.CharData().Y = 0, 0
	testdb.Exec(t, database, `UPDATE phaser_objects SET x=2 WHERE id=10`)
	request(true) // Two tiles away across an actual counter.
	testdb.Exec(t, database, `UPDATE phaser_tiles SET talk_over_tile=false`)
	request(false)
	testdb.Exec(t, database, `INSERT INTO phaser_tile_images(id,image_path,talk_over_tile) VALUES(20,'counter',true);
		INSERT INTO phaser_tile_properties(tile_image_id,name) VALUES(20,'counter');
		INSERT INTO phaser_event_tile_overrides(map_id,map_name,x,y,tile_image_id,collision_type) VALUES(1,'ROOM',1,0,20,0)`)
	request(true) // Eligibility-aware event art can introduce a counter.
	testdb.Exec(t, database, `UPDATE phaser_tile_images SET talk_over_tile=false WHERE id=20;
		UPDATE phaser_tiles SET talk_over_tile=true`)
	request(false) // Event art can also remove the base counter permission.
	testdb.Exec(t, database, `DELETE FROM phaser_event_tile_overrides`)
	testdb.Exec(t, database, `UPDATE phaser_tiles SET talk_over_tile=true, is_tile_erased=1`)
	request(false)
	testdb.Exec(t, database, `UPDATE phaser_tiles SET is_tile_erased=0;
		UPDATE phaser_objects SET x=1,y=1 WHERE id=10`)
	request(false) // A diagonal is never an interaction target.
	// Runtime NPC movement replaces the static spawn; cloned values stay owned.
	x, y := 1, 0
	wh.ActorManager.walkingActors[actorID] = &PhaserActor{ID: actorID, X: &x, Y: &y}
	request(true)
	x = 8
	request(false)
	// Per-character position takes precedence over shared runtime movement.
	testdb.Exec(t, database, `INSERT INTO character_object_positions(character_id,object_id,map_id,x,y) VALUES(42,10,1,1,0)`)
	request(true)
	testdb.Exec(t, database, `INSERT INTO phaser_event_object_visibility(map_id,map_name,object_name,visible) VALUES(1,'ROOM','GREETING',false)`)
	request(false)
	testdb.Exec(t, database, `INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,true,'test')`)
	request(true)
	testdb.Exec(t, database, `UPDATE character_object_visibility_overrides SET visible=false`)
	request(false)
	testdb.Exec(t, database, `DELETE FROM character_object_visibility_overrides;
		DELETE FROM phaser_event_object_visibility;
		DELETE FROM character_object_positions;
		UPDATE phaser_objects SET map_id=3,x=101,y=200 WHERE id=10`)
	delete(wh.ActorManager.walkingActors, actorID)
	wh.Cutscenes.byLabel["GREETING"].MapName = "ROUTE"
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.players[42] = &PlayerMovementState{CurrentX: 100, CurrentY: 200, MapID: UnifiedOverworldMapID}
	request(true) // Unified player and native-map NPC use the same global space.
	testdb.Exec(t, database, `ALTER TABLE character_object_visibility_overrides RENAME TO unavailable_visibility`)
	request(false) // SQL failure must not silently expose an actor.
}

func TestScriptInteractionPublishesStartedOnlyAfterDurableIssuanceCommit(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.Cutscenes = NewCutsceneManager(database)
	cs := &CutsceneScript{ScriptLabel: "GREETING", MapName: "ROOM", TriggerType: "npc_click", Actions: json.RawMessage(`[]`)}
	wh.Cutscenes.byLabel[cs.ScriptLabel] = cs
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'ROOM',10,10,0);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text) VALUES(10,1,1,0,'npc','GREETING','GREETING');
 CREATE FUNCTION reject_issuance_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'issuance commit rejected'; END $$;
 CREATE CONSTRAINT TRIGGER reject_issuance_commit AFTER INSERT ON character_cutscene_plans DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_issuance_commit();`)
	ses.Client.CharData().MapID = 1
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	request := fmt.Sprintf(`{"actorId":%d}`, actorID)
	battleDispatch(t, wh, ses, opcodes.ScriptedEventInteractRequest, request)
	var response ScriptedEventInteractResponse
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.ScriptedEventInteractResponse || json.Unmarshal(messages.streams[0].payload, &response) != nil || response.Success || response.Started {
		t.Fatalf("announced failed issuance: %+v messages=%v", response, messages.streams)
	}
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_cutscene_plans WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed issuance plans=%d err=%v", count, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_issuance_commit ON character_cutscene_plans`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.ScriptedEventInteractRequest, request)
	if len(messages.streams) != 2 || json.Unmarshal(messages.streams[0].payload, &response) != nil || !response.Success || !response.Started || messages.streams[1].opcode != opcodes.CutsceneStartNotify {
		t.Fatalf("retry did not issue: %+v", response)
	}
	if err := database.QueryRow(`SELECT count(*) FROM character_cutscene_plans WHERE character_id=42 AND resolution='pending'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("retry plans=%d err=%v", count, err)
	}
}

func TestActorVisibilityOverridesApplyWithoutSourceRules(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,false,'test')`)
	name := "PLAIN_ACTOR"
	actors := []PhaserActor{{ID: 100, DbID: 10, MapID: 41, Name: &name}, {ID: 101, DbID: 11, MapID: 41}}
	for _, mapID := range []int{41, UnifiedOverworldMapID} {
		visible, err := applyEventObjectVisibilityContext(context.Background(), database, 42, mapID, wh.EventFlags, append([]PhaserActor(nil), actors...))
		if err != nil || len(visible) != 1 || visible[0].DbID != 11 {
			t.Fatalf("map=%d visible=%+v err=%v", mapID, visible, err)
		}
	}
	testdb.Exec(t, database, `ALTER TABLE character_object_visibility_overrides RENAME TO unavailable_overrides`)
	if _, err := applyEventObjectVisibilityContext(context.Background(), database, 42, 41, wh.EventFlags, actors); err == nil {
		t.Fatal("missing override table exposed actors")
	}
}

func TestRemoteScriptInteractionCannotUnlockCardKeyDoor(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.Cutscenes = NewCutsceneManager(database)
	door := silphCardKeyDoors[0]
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'SILPH_CO_2F',10,10),(2,'OTHER',10,10);
		INSERT INTO phaser_objects(id,map_id,x,y,object_type,text) VALUES(10,1,1,0,'object','TEXT_SILPH_CARD_KEY_DOOR_2F_1');
		INSERT INTO cq_items(id,name,short_name) VALUES(48,'Card Key','CARD_KEY');
		INSERT INTO cq_item_instances(id,item_id,quantity) VALUES(100,48,1);
		INSERT INTO cq_character_inventory(character_id,item_instance_id) VALUES(42,100)`)
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	request := func(mapID uint32, x float64, want bool) {
		t.Helper()
		ses.Client.CharData().MapID, ses.Client.CharData().X = mapID, x
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.ScriptedEventInteractRequest, fmt.Sprintf(`{"actorId":%d}`, actorID))
		var flag bool
		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM character_event_flags WHERE character_id=42 AND flag_name=$1)`, door.Flag).Scan(&flag); err != nil || flag != want {
			t.Fatalf("door flag=%v want=%v err=%v", flag, want, err)
		}
	}
	request(2, 0, false)
	request(1, 8, false)
	request(1, 0, true)
}

func TestScriptInteractionAuthorizationTimeoutAndRetry(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	ses.Client.CharData().MapID = 1
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'ROOM',10,10);
		INSERT INTO phaser_objects(id,map_id,x,y,object_type) VALUES(10,1,1,0,'npc')`)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`LOCK TABLE character_object_visibility_overrides IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, _, err := wh.scriptInteractionTarget(ses, 10); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("authorization timeout=%v", err)
	}
	if time.Since(started) > 7*time.Second {
		t.Fatal("authorization exceeded deadline and driver allowance")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := wh.scriptInteractionTarget(ses, 10); err != nil {
		t.Fatalf("authorization retry=%v", err)
	}
}

func TestRemoteScriptInteractionCannotMutateTrashPuzzle(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.Cutscenes = NewCutsceneManager(database)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(1,'VERMILION_GYM',10,10),(2,'OTHER',10,10);
		INSERT INTO phaser_objects(id,map_id,x,y,object_type,text) VALUES(10,1,1,0,'object','TEXT_VERMILIONGYM_TRASH_CAN_0')`)
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 10)
	ses.Client.CharData().MapID = 2
	request := func() {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.ScriptedEventInteractRequest, fmt.Sprintf(`{"actorId":%d}`, actorID))
	}
	request()
	ses.Client.CharData().MapID = 1
	ses.Client.CharData().X = 8
	request()
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_vermilion_gym_trash_state`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("remote puzzle mutation count=%d err=%v", count, err)
	}
	ses.Client.CharData().X = 0
	request()
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_vermilion_gym_trash_state`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("valid puzzle mutation count=%d err=%v", count, err)
	}
}
