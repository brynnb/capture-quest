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
	testdb.Exec(t, database, `DELETE FROM character_event_flags WHERE character_id=42`)
	wh.EventFlags.publishCommittedFlags(42, map[string]bool{"OPEN": true})
	projected = read()
	if !projected.Success || projected.Tiles[0].TileImageID != 1 {
		t.Fatalf("stale positive flag granted override: %+v", projected)
	}
	testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'OPEN'); DELETE FROM phaser_tile_properties WHERE tile_image_id=2`)
	projected = read()
	var failed protocol.PlayerStepError
	if err := json.Unmarshal(messages.streams[0].payload, &failed); err != nil || projected.Success || failed.Error == "" || !strings.Contains(failed.Error, "image=2") {
		t.Fatalf("missing image metadata became base-tile success: %+v %v", failed, err)
	}
}
