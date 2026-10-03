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
	wh.PlayerMovement.RegisterPlayer(ses, 42, 3, 4, 192, "UP")
	// Owned movement has arrived; durable and published snapshots still await commit.
	ses.MapID, ses.X, ses.Y = 220, 14, 24
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
	request := `{"mapId":192,"requestId":"atomic"}`
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
	if !ok || mx != 3 || my != 4 || mm != 192 {
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
	_, err = commitMapLoad(ctx, database, 42, mapLoadArrival{MapID: 192, X: 3, Y: 4, ValidateCatalog: true, ApplyEffects: true})
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
	if _, err := commitMapLoad(context.Background(), database, 42, mapLoadArrival{MapID: 192, X: 3, Y: 4, ValidateCatalog: true, ApplyEffects: true}); err != nil {
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

func TestMapLoadUsesOriginalNativeProvenanceInsteadOfRectangle(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		x, y, source, original int
		native, reset          bool
	}{
		{"route outside old rectangle", 200, 300, 31, 31, true, true},
		{"edited neighbor inside old rectangle", 25, 110, 31, 32, true, false},
		{"user tile cannot manufacture route", 25, 110, 31, 31, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			database, wh, ses, messages := battleTestWorld(t)
			testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(31,'ROUTE_20',50,10,1),(32,'NEIGHBOR',10,10,1);
    INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_IN_SEAFOAM_ISLANDS');`)
			native := 0
			if tc.native {
				native = 1
			}
			if _, err := database.Exec(`INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,original_source_map_id,is_original_tile_location) VALUES($1,$2,1,$3,$4,$5)`, tc.x, tc.y, tc.source, tc.original, native); err != nil {
				t.Fatal(err)
			}
			db.GlobalWorldDB = nil
			ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = UnifiedOverworldMapID, float64(tc.x), float64(tc.y)
			payload, _ := json.Marshal(protocol.PhaserMapLoadRequest{MapID: UnifiedOverworldMapID, RequestID: "native"})
			battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, string(payload))
			var response protocol.PhaserMapLoadResponse
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || !response.Success || response.X != tc.x || response.Y != tc.y {
				t.Fatalf("arrival: %+v", response)
			}
			on, err := queryEventFlag(database, 42, "EVENT_IN_SEAFOAM_ISLANDS")
			if err != nil || on == tc.reset || wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") != on {
				t.Fatalf("route reset=%v flag=%v error=%v", tc.reset, on, err)
			}
		})
	}
}

func TestMapLoadNativePalletEffectsRunOutsideLegacyRouteSelection(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(0,'PALLET_TOWN',20,18,1);
 INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,is_original_tile_location) VALUES(200,300,1,0,1);
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_POKEBALLS_FROM_OAK');`)
	db.GlobalWorldDB = nil
	ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = UnifiedOverworldMapID, 200, 300
	battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, `{"mapId":9999,"requestId":"pallet"}`)
	var response protocol.PhaserMapLoadResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || !response.Success || !wh.EventFlags.CheckFlag(42, "EVENT_PALLET_AFTER_GETTING_POKEBALLS_2") {
		t.Fatalf("Pallet arrival: %+v", response)
	}
}

func TestMapLoadBrokenNativeProvenanceRollsBackArrival(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(fmt.Sprintf("ambiguous=%v", ambiguous), func(t *testing.T) {
			database, wh, ses, messages := battleTestWorld(t)
			testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(31,'ROUTE_20',50,10,1),(32,'NEIGHBOR',10,10,1);
    INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,is_original_tile_location) VALUES(200,300,1,31,1);
    INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_IN_SEAFOAM_ISLANDS');`)
			if ambiguous {
				testdb.Exec(t, database, `DROP INDEX phaser_tiles_coord_unique_idx; INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,is_original_tile_location) VALUES(200,300,1,32,1)`)
			} else {
				testdb.Exec(t, database, `UPDATE phaser_tiles SET source_map_id=NULL`)
			}
			var beforeMap, beforeX, beforeY int
			if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&beforeMap, &beforeX, &beforeY); err != nil {
				t.Fatal(err)
			}
			if err := wh.EventFlags.LoadFlags(42); err != nil {
				t.Fatal(err)
			}
			db.GlobalWorldDB = nil
			ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = UnifiedOverworldMapID, 200, 300
			battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, `{"mapId":9999,"requestId":"broken"}`)
			var response protocol.PhaserMapRequestError
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || response.Success || response.Error == "" {
				t.Fatalf("broken arrival: %+v", response)
			}
			var mapID, x, y int
			if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != beforeMap || x != beforeX || y != beforeY {
				t.Fatal("broken provenance committed destination")
			}
			if ses.MapID == 9999 || !wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
				t.Fatal("broken provenance published state")
			}
			if ambiguous {
				testdb.Exec(t, database, `DELETE FROM phaser_tiles WHERE source_map_id=32`)
			} else {
				testdb.Exec(t, database, `UPDATE phaser_tiles SET source_map_id=31`)
			}
			battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, `{"mapId":9999,"requestId":"retry"}`)
			var success protocol.PhaserMapLoadResponse
			if json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &success) != nil || !success.Success || wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
				t.Fatal("corrected provenance retry failed")
			}
		})
	}
}

func TestCurrentMapLoadCommitsOwnedMovementAndPreservesPath(t *testing.T) {
	for _, zeroSnapshot := range []bool{true, false} {
		t.Run(fmt.Sprintf("zeroSnapshot=%v", zeroSnapshot), func(t *testing.T) {
			database, wh, ses, messages := battleTestWorld(t)
			wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
			wh.PlayerMovement.RegisterPlayer(ses, 42, 200, 300, UnifiedOverworldMapID, "LEFT")
			state := wh.PlayerMovement.players[42]
			state.Path = []PathNode{{X: 201, Y: 300}, {X: 202, Y: 300}}
			state.IsSurfing = true
			state.PreviousMapID = 192
			state.positionDirty = true
			beforeSave := state.LastSaveTime
			char := ses.Client.CharData()
			char.MapID = 192
			char.X, char.Y = 2, 3
			if zeroSnapshot {
				char.X, char.Y = 0, 0
			}
			ses.MapID = 192
			ses.X, ses.Y = float32(char.X), float32(char.Y)
			testdb.Exec(t, database, `UPDATE character_data SET map_id=192,x=2,y=3 WHERE id=42;
    INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(31,'ROUTE_20',50,10,1),(192,'SEAFOAM_ISLANDS_1F',20,20,0);
    INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,is_original_tile_location) VALUES(200,300,1,31,1);
    INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_IN_SEAFOAM_ISLANDS');
    CREATE FUNCTION reject_current_load_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late current load failure'; END $$;
    CREATE CONSTRAINT TRIGGER reject_current_load_commit AFTER UPDATE ON character_data
    DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.map_id=9999) EXECUTE FUNCTION reject_current_load_commit();`)
			if err := wh.EventFlags.LoadFlags(42); err != nil {
				t.Fatal(err)
			}
			db.GlobalWorldDB = nil
			// A stale view is not permission to execute that map's effects.
			battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, `{"mapId":192,"requestId":"remote"}`)
			var failure protocol.PhaserMapRequestError
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &failure) != nil || failure.Success || failure.Error == "" {
				t.Fatal("stale view authorized")
			}
			messages.streams = nil
			request := `{"mapId":9999,"requestId":"owned"}`
			battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, request)
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &failure) != nil || failure.Success || failure.Error == "" || failure.RequestID != "owned" {
				t.Fatal("late failure published success")
			}
			var mapID, x, y int
			if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 192 || x != 2 || y != 3 {
				t.Fatal("current load partially committed position")
			}
			on, err := queryEventFlag(database, 42, "EVENT_IN_SEAFOAM_ISLANDS")
			if err != nil || !on || !wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
				t.Fatal("current load partially committed effect")
			}
			if char.MapID != 192 || ses.MapID != 192 || state.MapID != 9999 || state.CurrentX != 200 || state.CurrentY != 300 || !state.positionDirty || !state.LastSaveTime.Equal(beforeSave) {
				t.Fatal("failed load replaced live state")
			}
			testdb.Exec(t, database, `DROP TRIGGER reject_current_load_commit ON character_data`)
			battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, request)
			var response protocol.PhaserMapLoadResponse
			if json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &response) != nil || !response.Success || response.MapID != 9999 || response.X != 200 || response.Y != 300 {
				t.Fatalf("owned arrival: %+v", response)
			}
			if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 9999 || x != 200 || y != 300 {
				t.Fatal("owned arrival did not persist position")
			}
			if char.MapID != 9999 || char.X != 200 || char.Y != 300 || ses.MapID != 9999 || ses.X != 200 || ses.Y != 300 || state.positionDirty || wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
				t.Fatal("owned arrival projections disagree")
			}
			if len(state.Path) != 2 || state.Path[0] != (PathNode{X: 201, Y: 300}) || state.Direction != "LEFT" || !state.IsSurfing || state.PreviousMapID != 192 {
				t.Fatal("current load discarded movement intent")
			}
		})
	}
}

func TestCurrentMapLoadRecoversOwnedZeroInsteadOfValidStaleSnapshot(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 0, 0, UnifiedOverworldMapID, "LEFT")
	state := wh.PlayerMovement.players[42]
	state.Path = []PathNode{{X: 1, Y: 0}}
	char := ses.Client.CharData()
	char.MapID = 192
	char.X, char.Y = 2, 3
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(0,'PALLET_TOWN',20,18,1),(31,'ROUTE_20',50,10,1);
 INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,is_original_tile_location) VALUES(0,0,1,31,1),(9,4,1,0,1);
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_POKEBALLS_FROM_OAK'),(42,'EVENT_IN_SEAFOAM_ISLANDS');`)
	db.GlobalWorldDB = nil
	battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, `{"mapId":9999,"requestId":"recover"}`)
	var response protocol.PhaserMapLoadResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || !response.Success || response.MapID != RecoverySpawnMap || response.X != int(RecoverySpawnX) || response.Y != int(RecoverySpawnY) {
		t.Fatalf("owned recovery: %+v", response)
	}
	if !wh.EventFlags.CheckFlag(42, "EVENT_PALLET_AFTER_GETTING_POKEBALLS_2") || !wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
		t.Fatal("recovery ran the stale map's effects")
	}
	if len(state.Path) != 0 || state.Direction != RecoverySpawnDirection || state.CurrentX != int(RecoverySpawnX) || state.CurrentY != int(RecoverySpawnY) {
		t.Fatal("recovery did not publish its committed destination")
	}
}
