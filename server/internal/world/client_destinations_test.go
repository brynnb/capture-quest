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

func TestClientDestinationCatalogValidationRollsBackPositionAndSafari(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	testdb.Exec(t, database, `UPDATE character_data SET map_id=220,x=14,y=24 WHERE id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(60,'EXIT',20,20,0);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,is_tile_erased) VALUES(60,3,4,1,1),(NULL,-10,20,1,0);`)
	if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	for _, dest := range [][3]int{{60, 3, 4}, {60, 4, 4}, {999, 3, 4}, {UnifiedOverworldMapID, 10, 20}, {0, -10, 20}} {
		if err := commitClientPlayerPosition(context.Background(), database, 42, dest[0], dest[1], dest[2]); err == nil {
			t.Fatalf("accepted invalid catalog destination %v", dest)
		}
	}
	var mapID, x, y int
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 220 || x != 14 || y != 24 {
		t.Fatalf("failed destination changed position %d,%d,%d: %v", mapID, x, y, err)
	}
	safari, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || safari == nil || !safari.Active {
		t.Fatalf("rejected destination changed Safari: %+v %v", safari, err)
	}
	if err := commitClientPlayerPosition(context.Background(), database, 42, UnifiedOverworldMapID, -10, 20); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != UnifiedOverworldMapID || x != -10 || y != 20 {
		t.Fatal("valid negative overworld destination was not saved")
	}
	safari, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || safari != nil {
		t.Fatalf("committed destination did not end Safari: %+v %v", safari, err)
	}
}

func TestMapMetadataAndPartialDestinationCannotClaimPlayerPresence(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 50, 7, 8
	ses.MapID = 50
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42; INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(60,'VIEW',20,20,0);`)
	for _, payload := range []string{`{"mapId":60}`, `{"mapId":60,"destX":3}`, `{"mapId":60,"destY":4}`, `{"mapId":60,"destX":3,"destY":4}`} {
		battleDispatch(t, wh, ses, opcodes.PhaserMapInfoRequest, payload)
		if ses.MapID != 50 || char.MapID != 50 || char.X != 7 || char.Y != 8 {
			t.Fatal("map query claimed player location")
		}
	}
	if len(messages.streams) != 4 {
		t.Fatalf("map responses=%d", len(messages.streams))
	}
	for _, message := range messages.streams {
		if message.opcode != opcodes.PhaserMapInfoResponse {
			t.Fatal("query broadcast player state")
		}
	}
	battleDispatch(t, wh, ses, opcodes.PhaserPlayerPositionUpdate, `{"mapId":60,"x":3,"y":4,"direction":"DOWN"}`)
	if len(messages.streams) != 5 || messages.streams[4].opcode != opcodes.ChatMessageBroadcast {
		t.Fatal("invalid position report published state instead of a failure")
	}
	if ses.MapID != 50 || char.MapID != 50 || char.X != 7 || char.Y != 8 {
		t.Fatal("invalid position report changed live state")
	}
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || mapID != 50 || x != 7 || y != 8 {
		t.Fatal("map query changed movement registration")
	}
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 50 || x != 7 || y != 8 {
		t.Fatal("map query changed saved position")
	}
}

func TestOverworldMapListCannotClaimPresenceOrUseGlobalDatabase(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'PALLET_TOWN',20,18,1)`)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 50, 7, 8
	ses.MapID = 50
	db.GlobalWorldDB = nil
	battleDispatch(t, wh, ses, opcodes.PhaserOverworldMapsRequest, `{}`)
	if ses.MapID != 50 || char.MapID != 50 || char.X != 7 || char.Y != 8 {
		t.Fatal("map list changed presence")
	}
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PhaserOverworldMapsResponse {
		t.Fatal("unexpected list response/publication")
	}
	var maps []protocol.PhaserMapInfo
	if err := json.Unmarshal(messages.streams[0].payload, &maps); err != nil || len(maps) != 1 || maps[0].Name != "PALLET_TOWN" {
		t.Fatalf("list wire contract: %+v %v", maps, err)
	}
	var listFields []map[string]json.RawMessage
	if err := json.Unmarshal(messages.streams[0].payload, &listFields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tilesetId", "tileMinX", "tileMinY", "tileMaxX", "tileMaxY"} {
		if _, present := listFields[0][key]; present {
			t.Fatalf("list emitted absent optional field %s", key)
		}
	}
	testdb.Exec(t, database, `DELETE FROM phaser_maps`)
	battleDispatch(t, wh, ses, opcodes.PhaserOverworldMapsRequest, `{}`)
	if string(messages.streams[1].payload) != "[]" {
		t.Fatalf("empty list wire contract: %s", messages.streams[1].payload)
	}
	testdb.Exec(t, database, `DROP TABLE phaser_maps CASCADE`)
	battleDispatch(t, wh, ses, opcodes.PhaserOverworldMapsRequest, `{}`)
	var failure protocol.ErrorResponse
	if err := json.Unmarshal(messages.streams[2].payload, &failure); err != nil || failure.Success || failure.Error == "" {
		t.Fatal("query failure accepted as partial list")
	}
	if ses.MapID != 50 {
		t.Fatal("failed metadata query changed presence")
	}
}
