package world

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestMapLoadPositionSafariAndFlagCommitTogether(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.sessionManager = session.NewSessionManager()
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.Safari = NewSafariZoneManager(database)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 220, 14, 24
	ses.MapID = 220
	wh.PlayerMovement.RegisterPlayer(ses, 42, 14, 24, 220, "UP")
	testdb.Exec(t, database, `UPDATE character_data SET map_id=220,x=14,y=24 WHERE id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(192,'SEAFOAM_ISLANDS_1F',20,20,0);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id) VALUES(192,3,4,1);
 CREATE FUNCTION reject_load_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late load failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_load_commit AFTER INSERT ON character_event_flags
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.flag_name='EVENT_IN_SEAFOAM_ISLANDS') EXECUTE FUNCTION reject_load_commit();`)
	if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	if err := wh.EventFlags.LoadFlags(42); err != nil {
		t.Fatal(err)
	}
	// Preload collision residency through the existing actor boundary before disabling
	// globals to verify that arrival persistence uses the injected database.
	if err := wh.ActorManager.ensureWalkableMapLoaded(192); err != nil {
		t.Fatal(err)
	}
	db.GlobalWorldDB = nil
	request := `{"mapId":192,"destX":3,"destY":4,"requestId":"atomic"}`
	battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, request)
	var failure protocol.PhaserMapRequestError
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &failure) != nil || failure.Success || failure.Error == "" || failure.RequestID != "atomic" {
		t.Fatal("failed arrival published success")
	}
	var mapID, x, y int
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 220 || x != 14 || y != 24 {
		t.Fatal("late flag failure committed position")
	}
	safari, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || safari == nil || !safari.Active {
		t.Fatal("late flag failure ended Safari")
	}
	if wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") || char.MapID != 220 || ses.MapID != 220 {
		t.Fatal("uncommitted cache/live publication")
	}
	mx, my, mm, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || mx != 14 || my != 24 || mm != 220 {
		t.Fatal("failed arrival changed movement")
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_load_commit ON character_event_flags`)
	battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, request)
	var success protocol.PhaserMapLoadResponse
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &success); err != nil || !success.Success || success.MapID != 192 || success.X != 3 || success.Y != 4 {
		t.Fatalf("arrival retry: %+v %v", success, err)
	}
	if char.MapID != 192 || char.X != 3 || char.Y != 4 || !wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
		t.Fatal("committed state was not published")
	}
	safari, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || safari != nil {
		t.Fatal("committed arrival did not end Safari")
	}
}

func TestMapLoadDaisyOverridesUseDurableFlagsAndRollbackTogether(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	if err := wh.EventFlags.LoadFlags(42); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(0,'PALLET_TOWN',20,18,1);
 INSERT INTO phaser_objects(id,name,map_id) VALUES(11,'BluesHouse_NPC_1',37),(12,'BluesHouse_NPC_2',37);
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_TOWN_MAP'),(42,'EVENT_ENTERED_BLUES_HOUSE');
 CREATE FUNCTION reject_daisy_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late override failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_daisy_commit AFTER INSERT ON character_object_visibility_overrides
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_daisy_commit();`)
	db.GlobalWorldDB = nil
	if _, err := ApplyMapLoadScriptEffectsForMapName(42, "PALLET_TOWN", wh.EventFlags); err == nil {
		t.Fatal("late failure accepted")
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_object_visibility_overrides WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial visibility escaped")
	}
	on, err := queryEventFlag(database, 42, "EVENT_DAISY_WALKING")
	if err != nil || on || wh.EventFlags.CheckFlag(42, "EVENT_DAISY_WALKING") {
		t.Fatal("partial flag escaped")
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_daisy_commit ON character_object_visibility_overrides`)
	effect, err := ApplyMapLoadScriptEffectsForMapName(42, "PALLET_TOWN", wh.EventFlags)
	if err != nil || !effect.Changed() || !wh.EventFlags.CheckFlag(42, "EVENT_DAISY_WALKING") {
		t.Fatalf("durable eligibility retry: %+v %v", effect, err)
	}
	var hidden, shown bool
	if err := database.QueryRow(`SELECT visible FROM character_object_visibility_overrides WHERE character_id=42 AND object_id=11`).Scan(&hidden); err != nil || hidden {
		t.Fatal("old Daisy was not hidden")
	}
	if err := database.QueryRow(`SELECT visible FROM character_object_visibility_overrides WHERE character_id=42 AND object_id=12`).Scan(&shown); err != nil || !shown {
		t.Fatal("walking Daisy was not shown")
	}
}

func TestMapLoadCancellationRollsBackPositionAndRetries(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(192,'SEAFOAM_ISLANDS_1F',20,20,0);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id) VALUES(192,3,4,1);`)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`LOCK TABLE character_event_flags IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	_, err = commitMapLoad(ctx, database, 42, 192, 3, 4, true, true, true, 192, "SEAFOAM_ISLANDS_1F")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline: %v", err)
	}
	var mapID int
	if err := database.QueryRow(`SELECT map_id FROM character_data WHERE id=42`).Scan(&mapID); err != nil || mapID == 192 {
		t.Fatal("cancelled effect persisted position")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := commitMapLoad(context.Background(), database, 42, 192, 3, 4, true, true, true, 192, "SEAFOAM_ISLANDS_1F"); err != nil {
		t.Fatal(err)
	}
}

func TestMapLoadBoulderResetRollsBackWithFlagsAndPreservesOtherPositions(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(43,'other');
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(194,'VICTORY_ROAD_2F',20,20,0);
 INSERT INTO phaser_objects(id,name,map_id,sprite_name) VALUES(11,'boulder',108,'SPRITE_BOULDER'),(12,'other',108,'SPRITE_NPC');
 INSERT INTO character_object_positions(character_id,object_id,map_id,x,y) VALUES(42,11,108,1,1),(42,12,108,2,2),(43,11,108,3,3);
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH');
 CREATE FUNCTION reject_boulder_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late boulder failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_boulder_commit AFTER DELETE ON character_object_positions
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_boulder_commit();`)
	if err := wh.EventFlags.LoadFlags(42); err != nil {
		t.Fatal(err)
	}
	db.GlobalWorldDB = nil
	if _, err := ApplyMapLoadScriptEffectsForMapName(42, "VICTORY_ROAD_2F", wh.EventFlags); err == nil {
		t.Fatal("late delete failure accepted")
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_object_positions`).Scan(&count); err != nil || count != 3 {
		t.Fatal("partial boulder deletion escaped")
	}
	on, err := queryEventFlag(database, 42, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH")
	if err != nil || !on || !wh.EventFlags.CheckFlag(42, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH") {
		t.Fatal("partial flag reset escaped")
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_boulder_commit ON character_object_positions`)
	if _, err := ApplyMapLoadScriptEffectsForMapName(42, "VICTORY_ROAD_2F", wh.EventFlags); err != nil {
		t.Fatal(err)
	}
	if wh.EventFlags.CheckFlag(42, "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH") {
		t.Fatal("committed reset absent from cache")
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_object_positions WHERE (character_id=42 AND object_id=12) OR (character_id=43 AND object_id=11)`).Scan(&count); err != nil || count != 2 {
		t.Fatal("reset removed unrelated positions")
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_object_positions WHERE character_id=42 AND object_id=11`).Scan(&count); err != nil || count != 0 {
		t.Fatal("committed boulder reset absent")
	}
}
