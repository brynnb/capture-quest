package world

import (
	"context"
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/testdb"
)

func TestNormalWarpBoundaryRejectsForgedActivationAndRollsBackLateFailure(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "DOWN")
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 50, 7, 8
	wh.Safari = NewSafariZoneManager(database)
	if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ROOM',20,20,0),(60,'OTHER',20,20,0),(192,'SEAFOAM_ISLANDS_1F',20,20,0);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id) VALUES(192,3,4,1);
 INSERT INTO phaser_warps(id,source_map_id,x,y,destination_map_id,destination_x,destination_y,warp_type) VALUES
 (1,50,7,9,192,3,4,'door'),(2,60,7,9,192,3,4,'door'),(3,50,30,30,192,3,4,'door'),(4,50,7,9,192,3,4,'inactive'),(5,50,7,9,192,3,4,'elevator');
 CREATE FUNCTION reject_warp_effect_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late warp failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_warp_effect_commit AFTER INSERT ON character_event_flags
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.flag_name='EVENT_IN_SEAFOAM_ISLANDS') EXECUTE FUNCTION reject_warp_effect_commit();`)
	db.GlobalWorldDB = nil
	for _, payload := range []string{
		`{"warpId":2,"direction":"DOWN","inputSource":"click","requestId":"remote"}`,
		`{"warpId":3,"direction":"DOWN","inputSource":"click","requestId":"distant"}`,
		`{"warpId":4,"direction":"DOWN","inputSource":"click","requestId":"inactive"}`,
		`{"warpId":5,"direction":"DOWN","inputSource":"click","requestId":"elevator"}`,
		`{"warpId":1,"direction":"DOWN","inputSource":"click","requestId":"forged","x":100,"mapId":9999}`,
		`{"warpId":1,"direction":"DOWN","inputSource":"click","requestId":"late"}`,
	} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.PhaserWarpActivateRequest, payload)
		var failure protocol.PhaserMapRequestError
		if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PhaserWarpActivateResponse || json.Unmarshal(messages.streams[0].payload, &failure) != nil || failure.Success || failure.Error == "" || failure.RequestID == "" {
			t.Fatalf("invalid activation accepted: %s", payload)
		}
		var mapID, x, y int
		if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 50 || x != 7 || y != 8 {
			t.Fatal("failed activation changed durable position")
		}
		visit, err := wh.Safari.GetSession(context.Background(), 42)
		if err != nil || visit == nil || !visit.Active || char.MapID != 50 || char.X != 7 || char.Y != 8 || wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
			t.Fatal("failed activation changed live/effect/Safari state")
		}
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_warp_effect_commit ON character_event_flags`)
	battleDispatch(t, wh, ses, opcodes.PhaserWarpActivateRequest, `{"warpId":1,"direction":"DOWN","inputSource":"keyboard","requestId":"retry"}`)
	var result protocol.PhaserWarpActivateResponse
	if json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &result) != nil || !result.Success || result.MapID != 192 || result.PlayerMapID != 192 || result.X != 3 || result.Y != 4 || result.RequestID != "retry" || !wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
		t.Fatalf("warp retry: %+v", result)
	}
	if char.MapID != 192 || char.X != 3 || char.Y != 4 {
		t.Fatal("committed warp was not published")
	}
	visit, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || visit != nil {
		t.Fatal("committed warp did not end Safari")
	}
}

func TestNormalWarpCommitsBuildingExitStepAndPerPlayerLastMap(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 50, 7, 8
	ses.PreviousMapID = 31
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ROOM',20,20,0),(31,'ROUTE_20',50,10,1);
 INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,is_original_tile_location) VALUES(200,300,1,31,1),(200,301,1,31,1);
 INSERT INTO phaser_warps(id,source_map_id,source_warp_index,x,y,destination_map_id,destination_x,destination_y,destination_kind,destination_warp_id,warp_type) VALUES
 (1,50,0,7,9,NULL,NULL,NULL,'last-map',1,'door'),(2,31,1,200,300,50,7,9,'map',0,'door');
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_IN_SEAFOAM_ISLANDS');`)
	db.GlobalWorldDB = nil
	battleDispatch(t, wh, ses, opcodes.PhaserWarpActivateRequest, `{"warpId":1,"direction":"DOWN","inputSource":"click","requestId":"exit"}`)
	var result protocol.PhaserWarpActivateResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &result) != nil || !result.Success || result.MapID != 31 || result.PlayerMapID != 9999 || result.X != 200 || result.Y != 301 || !result.AnimateExitStep || result.AnimationStartY == nil || *result.AnimationStartY != 300 || wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
		t.Fatalf("exit result: %+v", result)
	}
	var mapID, x, y int
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 9999 || x != 200 || y != 301 {
		t.Fatal("animation step was not committed")
	}
}

func TestNormalWarpChecksKeyboardDirectionAndSafariVisit(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 156, 7, 9
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(156,'SAFARI_ZONE_GATE',20,20,0),(220,'SAFARI_ZONE_CENTER',20,20,0);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id) VALUES(220,3,4,1);
 INSERT INTO phaser_warps(id,source_map_id,x,y,destination_map_id,destination_x,destination_y,warp_type,warp_direction) VALUES(1,156,7,9,220,3,4,'carpet','DOWN');`)
	db.GlobalWorldDB = nil
	for _, direction := range []string{"UP", "DOWN"} {
		payload, _ := json.Marshal(protocol.PhaserWarpActivateRequest{WarpID: 1, Direction: direction, InputSource: "keyboard", RequestID: direction})
		battleDispatch(t, wh, ses, opcodes.PhaserWarpActivateRequest, string(payload))
		var failure protocol.PhaserMapRequestError
		if json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure) != nil || failure.Success || failure.Error == "" || char.MapID != 156 {
			t.Fatal("direction or Safari guard bypassed")
		}
	}
	wh.Safari = NewSafariZoneManager(database)
	if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	battle := battleTestStart(t, database, false, nil)
	battleDispatch(t, wh, ses, opcodes.PhaserWarpActivateRequest, `{"warpId":1,"direction":"DOWN","inputSource":"keyboard","requestId":"battle"}`)
	var failure protocol.PhaserMapRequestError
	if json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure) != nil || failure.Success || char.MapID != 156 {
		t.Fatal("battle allowed normal warp")
	}
	forgetBattle(42, battle)
	battleDispatch(t, wh, ses, opcodes.PhaserWarpActivateRequest, `{"warpId":1,"direction":"DOWN","inputSource":"keyboard","requestId":"visit"}`)
	var success protocol.PhaserWarpActivateResponse
	if json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &success) != nil || !success.Success || char.MapID != 220 {
		t.Fatal("eligible Safari warp rejected")
	}
}
