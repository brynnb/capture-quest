package world

import (
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTileQueryUsesInjectedPoolAndOwnerCancellation(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- ses.ExecuteCommand(ctx, func() { HandlePhaserTilesRequest(ses, []byte(`{"mapId":9999,"requestId":"tile:cancel"}`), wh) })
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("tile owner deadline: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		lease.Close()
		<-done
		t.Fatal("tile query escaped owner deadline")
	}
	if database.Stats().WaitCount == before {
		t.Fatal("tile read did not wait on injected pool")
	}
}

func TestTileProjectionUsesCommittedFlagsAndRejectsUnavailableMetadata(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(50,'GATE',2,2);
 INSERT INTO phaser_tile_images(id,image_path,raw_foot_tile_id,talk_over_tile) VALUES(1,'base.png',1,false),(2,'open.png',2,true);
 INSERT INTO phaser_tile_properties(tile_image_id,collision_type) VALUES(1,1),(2,0);
 INSERT INTO phaser_tiles(x,y,local_x,local_y,map_id,source_map_id,tile_image_id,collision_type,raw_foot_tile_id,is_native_game_data,coordinate_origin,content_origin) VALUES(1,1,1,1,50,50,1,1,1,true,'native','native');
 INSERT INTO phaser_event_tile_overrides(map_id,map_name,x,y,tile_image_id,collision_type,requires_flag) VALUES(50,'GATE',1,1,2,0,'OPEN');
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'OPEN');`)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	database.SetMaxOpenConns(1)
	wh.EventFlags.publishCommittedFlags(42, map[string]bool{}) // Deliberately stale cache.
	read := func() PhaserTilesResponse {
		messages.streams = nil
		HandlePhaserTilesRequest(ses, []byte(`{"mapId":50,"requestId":"tile:projection"}`), wh)
		if len(messages.streams) != 1 {
			t.Fatal("missing tile publication")
		}
		var response PhaserTilesResponse
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil {
			t.Fatal(err)
		}
		return response
	}
	projected := read()
	if !projected.Success || len(projected.Tiles) != 1 || projected.Tiles[0].TileImageID != 2 || projected.Tiles[0].CollisionType != 0 || projected.Tiles[0].RawFootTileID == nil || *projected.Tiles[0].RawFootTileID != 2 || !projected.Tiles[0].TalkOverTile {
		t.Fatalf("projection used stale cache/default properties: %+v", projected)
	}
	testdb.Exec(t, database, `INSERT INTO phaser_tile_images(id,image_path,raw_foot_tile_id,talk_over_tile) VALUES(3,'priority.png',3,false); INSERT INTO phaser_tile_properties(tile_image_id,collision_type) VALUES(3,0); INSERT INTO phaser_event_tile_overrides(map_id,map_name,x,y,tile_image_id,collision_type,requires_flag) VALUES(50,'GATE',1,1,3,0,'OPEN')`)
	projected = read()
	states, err := EventTileStatesForCharacter(context.Background(), database, 42, 50)
	if err != nil || len(states) != 1 || projected.Tiles[0].TileImageID != 3 || states[0].TileImageID != 3 || states[0].RawFootTileID == nil || *states[0].RawFootTileID != 3 {
		t.Fatalf("read/publisher override priority differs: %+v %+v %v", projected, states, err)
	}
	messages.streams = nil
	sendEventTileStatesForSession(ses, 42, "GATE", wh)
	var publication TileEditorBroadcastPayload
	if len(messages.streams) != 1 {
		t.Fatal("missing owned publication")
	}
	if err := json.Unmarshal(messages.streams[0].payload, &publication); err != nil || len(publication.Tiles) != 1 || publication.Tiles[0].TileImageID != 3 {
		t.Fatalf("publication used global/cache/first override: %+v %v", publication, err)
	}
	messages.streams = nil
	ses.MapID = 50
	sendEventTileStatesForSession(ses, 42, "MISSING_MAP", wh)
	if len(messages.streams) != 0 {
		t.Fatal("failed named-map lookup published through guessed session map")
	}
	testdb.Exec(t, database, `DELETE FROM character_event_flags WHERE character_id=42`)
	wh.EventFlags.publishCommittedFlags(42, map[string]bool{"OPEN": true})
	projected = read()
	if !projected.Success || projected.Tiles[0].TileImageID != 1 {
		t.Fatalf("stale positive flag granted override: %+v", projected)
	}
	testdb.Exec(t, database, `UPDATE phaser_tiles SET is_tile_erased=1 WHERE map_id=50 AND x=1 AND y=1`)
	states, err = EventTileStatesForCharacter(context.Background(), database, 42, 50)
	if err != nil || len(states) != 1 || !states[0].Erased {
		t.Fatalf("erased base did not resolve to removal: %+v %v", states, err)
	}
	messages.streams = nil
	sendEventTileStatesForSession(ses, 42, "GATE", wh)
	if len(messages.streams) != 1 {
		t.Fatal("erased base update was silently omitted")
	}
	publication = TileEditorBroadcastPayload{}
	if err := json.Unmarshal(messages.streams[0].payload, &publication); err != nil || len(publication.Tiles) != 1 || !publication.Tiles[0].Erased {
		t.Fatalf("missing explicit erase publication: %+v %v", publication, err)
	}
	testdb.Exec(t, database, `DELETE FROM phaser_tiles WHERE map_id=50 AND x=1 AND y=1`)
	if states, err := EventTileStatesForCharacter(context.Background(), database, 42, 50); err == nil || states != nil {
		t.Fatalf("missing base disguised as removal: %+v %v", states, err)
	}
	testdb.Exec(t, database, `INSERT INTO phaser_tiles(x,y,local_x,local_y,map_id,source_map_id,tile_image_id,collision_type,raw_foot_tile_id,is_native_game_data,coordinate_origin,content_origin) VALUES(1,1,1,1,50,50,1,1,1,true,'native','native'); INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'OPEN'); DELETE FROM phaser_tile_properties WHERE tile_image_id=3`)
	projected = read()
	if !projected.Success || len(projected.Tiles) != 1 || projected.Tiles[0].RawFootTileID == nil || *projected.Tiles[0].RawFootTileID != 3 {
		t.Fatalf("valid imported image required an editor palette row: %+v", projected)
	}
	states, err = EventTileStatesForCharacter(context.Background(), database, 42, 50)
	if err != nil || len(states) != 1 || states[0].TileImageID != 3 || states[0].RawFootTileID == nil || *states[0].RawFootTileID != 3 {
		t.Fatalf("publication required editor palette: %+v %v", states, err)
	}
	testdb.Exec(t, database, `DELETE FROM phaser_tile_images WHERE id=3`)
	projected = read()
	var failed protocol.PlayerStepError
	if err := json.Unmarshal(messages.streams[0].payload, &failed); err != nil || projected.Success || failed.Error == "" || !strings.Contains(failed.Error, "image=3") {
		t.Fatalf("missing image metadata became base-tile success: %+v %v", failed, err)
	}
}

func TestCharacterCollisionUsesCommittedRuleSelection(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(9999,'WORLD',2,2,1);
 INSERT INTO phaser_tile_images(id,image_path,raw_foot_tile_id,talk_over_tile) VALUES(2,'selected.png',2,false);
 INSERT INTO phaser_tile_properties(tile_image_id,collision_type) VALUES(2,0);
 INSERT INTO phaser_event_tile_overrides(map_id,map_name,x,y,tile_image_id,collision_type,requires_flag) VALUES(9999,'WORLD',1,1,999,2,'OPEN'),(9999,'WORLD',1,1,2,0,'OPEN');
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'OPEN');`)
	manager := &PhaserActorManager{collisionMap: map[int]map[string]int{9999: {tileKey(1, 1): 1}}, rawFootTileMap: map[int]map[string]int{9999: {tileKey(1, 1): 1}}}
	wh.EventFlags.publishCommittedFlags(42, map[string]bool{})
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	database.SetMaxOpenConns(1)
	collision, raw, err := manager.characterCollision(context.Background(), database, 42, 9999, 0, 0, wh.EventFlags)
	if err != nil || collision[tileKey(1, 1)] != 0 || raw[tileKey(1, 1)] != 2 {
		t.Fatalf("stale-negative cache or superseded metadata selected: collision=%v raw=%v err=%v", collision, raw, err)
	}
	testdb.Exec(t, database, `DELETE FROM character_event_flags WHERE character_id=42`)
	wh.EventFlags.publishCommittedFlags(42, map[string]bool{"OPEN": true})
	collision, raw, err = manager.characterCollision(context.Background(), database, 42, 9999, 0, 0, wh.EventFlags)
	if err != nil || collision[tileKey(1, 1)] != 1 || raw[tileKey(1, 1)] != 1 {
		t.Fatalf("stale-positive cache authorized rule: collision=%v raw=%v err=%v", collision, raw, err)
	}
	rollback := errors.New("reject outer movement transaction")
	err = db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		if _, err := tx.Exec(`INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'OPEN')`); err != nil {
			return err
		}
		collision, raw, err := manager.characterCollision(context.Background(), tx.(db.ReadDBTX), 42, 9999, 0, 0, wh.EventFlags)
		if err != nil || collision[tileKey(1, 1)] != 0 || raw[tileKey(1, 1)] != 2 {
			t.Fatalf("collision did not join caller's transaction: %v %v %v", collision, raw, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	collision, raw, err = manager.characterCollision(context.Background(), database, 42, 9999, 0, 0, wh.EventFlags)
	if err != nil || collision[tileKey(1, 1)] != 1 {
		t.Fatalf("rolled-back flags leaked into collision: %v %v", collision, err)
	}
	testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'OPEN'); DELETE FROM phaser_tile_images WHERE id=2`)
	collision, raw, err = manager.characterCollision(context.Background(), database, 42, 9999, 0, 0, wh.EventFlags)
	if err == nil || collision != nil || raw != nil {
		t.Fatalf("missing winning metadata returned partial success: %v %v %v", collision, raw, err)
	}
}

func TestCharacterCollisionHonorsPoolWaitCancellation(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	manager := &PhaserActorManager{}
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	collision, raw, err := manager.characterCollision(ctx, database, 42, 9999, 0, 0, wh.EventFlags)
	if !errors.Is(err, context.DeadlineExceeded) || collision != nil || raw != nil || database.Stats().WaitCount <= before {
		t.Fatalf("collision escaped owned pool wait: %v %v %v", collision, raw, err)
	}
}

func TestOwnedPathfindingSeparatesReadFailureFromNoRoute(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	manager := NewPhaserActorManager(nil)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(50,'ROOM',10,10); INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,7,8,1,1),(50,8,8,1,1)`)
	path, err := manager.FindPathForCharacter(context.Background(), database, 42, 50, 7, 8, 8, 8, nil)
	if err != nil || len(path) != 1 || path[0].X != 8 {
		t.Fatalf("owned route: %v %v", path, err)
	}
	testdb.Exec(t, database, `UPDATE phaser_tiles SET collision_type=0 WHERE map_id=50 AND x=8 AND y=8`)
	manager.InvalidateCollisionMap(50)
	path, err = manager.FindPathForCharacter(context.Background(), database, 42, 50, 7, 8, 8, 8, nil)
	if err != nil || len(path) != 0 {
		t.Fatalf("legitimate no route: %v %v", path, err)
	}
	testdb.Exec(t, database, `ALTER TABLE phaser_tiles RENAME TO unavailable_tiles`)
	manager.InvalidateCollisionMap(50)
	path, err = manager.FindPathForCharacter(context.Background(), database, 42, 50, 7, 8, 8, 8, nil)
	if err == nil || path != nil {
		t.Fatalf("source failure disguised as no route: %v %v", path, err)
	}
}

func TestOwnedPathfindingHonorsPoolWaitDeadline(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	before := database.Stats().WaitCount
	path, err := NewPhaserActorManager(nil).FindPathForCharacter(ctx, database, 42, 50, 7, 8, 8, 8, nil)
	if !errors.Is(err, context.DeadlineExceeded) || path != nil || database.Stats().WaitCount <= before {
		t.Fatalf("owned pathfinding escaped pool deadline: %v %v", path, err)
	}
}
