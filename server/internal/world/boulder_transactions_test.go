package world

import (
	"context"
	"testing"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestBoulderPushAndInitialRouteCommitOrRollbackTogether(t *testing.T) {
	wh, _, _ := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,9,8,1,1); INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'Boulder','npc','SPRITE_BOULDER',8,8); UPDATE character_pokemon SET move1_id=70 WHERE character_id=42; INSERT INTO phaser_moves(id,constant_name,name,short_name,pp,type,power,accuracy) VALUES(70,'STRENGTH','STRENGTH','STRENGTH',15,'NORMAL',80,100); INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_RAINBOWBADGE'); INSERT INTO phaser_boulder_targets(target_family,map_name,source_label,x,y,flag,drops_through_hole) VALUES('victory_road','ROOM','TestSwitch',9,8,'EVENT_TEST_BOULDER_SWITCH',false); CREATE FUNCTION reject_boulder_cursor() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject cursor'; END $$; CREATE CONSTRAINT TRIGGER reject_boulder_cursor AFTER INSERT ON character_movement_routes DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_boulder_cursor();`)
	flags := &EventFlagManager{flags: map[int64]map[string]bool{42: {"EVENT_GOT_RAINBOWBADGE": true}}}
	db.GlobalWorldDB = nil
	if _, err := pushBoulder(context.Background(), wh.database, 42, 50, 7, 8, "RIGHT", true, flags); err == nil {
		t.Fatal("late failure committed push")
	}
	var changed int
	if err := wh.database.QueryRow(`SELECT (SELECT COUNT(*) FROM character_object_positions)+(SELECT COUNT(*) FROM character_field_move_state)+(SELECT COUNT(*) FROM character_movement_routes)+(SELECT COUNT(*) FROM character_event_flags WHERE flag_name='EVENT_TEST_BOULDER_SWITCH')`).Scan(&changed); err != nil || changed != 0 {
		t.Fatalf("partial boulder mutation: %d %v", changed, err)
	}
	if flags.CheckFlag(42, "EVENT_TEST_BOULDER_SWITCH") {
		t.Fatal("failed push published flags")
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_boulder_cursor ON character_movement_routes`)
	result, err := pushBoulder(context.Background(), wh.database, 42, 50, 7, 8, "RIGHT", true, flags)
	if err != nil || !result.Success {
		t.Fatalf("valid push failed: %+v %v", result, err)
	}
	route, err := readMovementRoute(context.Background(), wh.database, 42)
	if err != nil || route == nil || route.X != 7 || len(route.Path) != 1 || route.Path[0].X != 8 {
		t.Fatalf("initial handoff missing: %+v %v", route, err)
	}
	if !flags.CheckFlag(42, "EVENT_TEST_BOULDER_SWITCH") {
		t.Fatal("committed flag not published")
	}
	if _, err := pushBoulder(context.Background(), wh.database, 42, 50, 7, 8, "RIGHT", true, flags); err == nil {
		t.Fatal("duplicate source push ignored pending route")
	}
	var x int
	if err := wh.database.QueryRow(`SELECT x FROM character_object_positions WHERE character_id=42 AND object_id=777`).Scan(&x); err != nil || x != 9 {
		t.Fatal("duplicate moved boulder twice")
	}
}
