package world

import (
	"context"
	"testing"

	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

func TestSilphCardKeyDurableEligibilityAndCommitFailure(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	text := silphCardKeyDoors[0].TextConstant
	flag := silphCardKeyDoors[0].Flag
	outcome, err := handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err != nil || outcome.Opened || outcome.HasCardKey {
		t.Fatalf("missing key: %+v %v", outcome, err)
	}
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(48,'CARD KEY','CARD_KEY',true)`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 48, 1); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `
 CREATE FUNCTION reject_door_commit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'reject door commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_door_commit AFTER INSERT
 ON character_event_flags DEFERRABLE INITIALLY DEFERRED
 FOR EACH ROW EXECUTE FUNCTION reject_door_commit();`)
	outcome, err = handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err == nil || outcome != nil {
		t.Fatalf("commit failure publication: %+v %v", outcome, err)
	}
	on, err := queryEventFlag(database, 42, flag)
	if err != nil || on || wh.EventFlags.CheckFlag(42, flag) {
		t.Fatalf("failure flag: %v %v", on, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_door_commit ON character_event_flags`)
	outcome, err = handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err != nil || !outcome.Opened || !outcome.Changed || !wh.EventFlags.CheckFlag(42, flag) {
		t.Fatalf("retry: %+v %v", outcome, err)
	}
	wh.EventFlags.UnloadFlags(42)
	outcome, err = handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err != nil || !outcome.AlreadyOpen || outcome.Changed {
		t.Fatalf("stale cache duplicate: %+v %v", outcome, err)
	}
	// A previously opened door still follows the existing no-key dialogue rule.
	testdb.Exec(t, database, `DELETE FROM cq_character_inventory WHERE character_id=42`)
	outcome, err = handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err != nil || outcome.HasCardKey || outcome.Opened {
		t.Fatalf("removed key: %+v %v", outcome, err)
	}
}

func TestSilphSourceDoorLowerFloorDoesNotAuthorizeCrossing(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	manager := NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(207,'SILPH_CO_2F',20,20);
 INSERT INTO phaser_tile_images(id,image_path,raw_foot_tile_id) VALUES(1,'floor.png',1),(2,'door.png',24);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(207,4,3,1,1),(207,4,4,2,0),(207,4,5,1,1),(207,4,6,1,1);
 INSERT INTO phaser_event_tile_overrides(map_id,map_name,x,y,tile_image_id,collision_type,requires_flag_absent) VALUES(207,'SILPH_CO_2F',4,4,2,0,'EVENT_SILPH_CO_2_UNLOCKED_DOOR1'),(207,'SILPH_CO_2F',4,5,1,1,'EVENT_SILPH_CO_2_UNLOCKED_DOOR1');
 INSERT INTO phaser_event_tile_overrides(map_id,map_name,x,y,tile_image_id,collision_type,requires_flag) VALUES(207,'SILPH_CO_2F',4,4,1,1,'EVENT_SILPH_CO_2_UNLOCKED_DOOR1')`)
	path, err := manager.FindPathForCharacter(context.Background(), database, 42, 207, 4, 6, 4, 5, wh.EventFlags)
	if err != nil || len(path) != 1 {
		t.Fatalf("lower source floor blocked: %v %v", path, err)
	}
	path, err = manager.FindPathForCharacter(context.Background(), database, 42, 207, 4, 6, 4, 3, wh.EventFlags)
	if err != nil || len(path) != 0 {
		t.Fatalf("closed upper door allowed crossing: %v %v", path, err)
	}
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(48,'CARD KEY','CARD_KEY',true)`)
	if _, err = cqitems.NewStore(database).AddItemToInventory(42, 48, 1); err != nil {
		t.Fatal(err)
	}
	outcome, err := handleSilphCardKeyDoor(context.Background(), database, 42, silphCardKeyDoors[0].TextConstant, wh.EventFlags)
	if err != nil || !outcome.Opened {
		t.Fatalf("key did not open door: %+v %v", outcome, err)
	}
	path, err = manager.FindPathForCharacter(context.Background(), database, 42, 207, 4, 6, 4, 3, wh.EventFlags)
	if err != nil || len(path) != 3 {
		t.Fatalf("opened door still blocked: %v %v", path, err)
	}
}
