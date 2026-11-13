package world

import (
	"bytes"
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
	for _, payload := range []string{`{"mapId":60,"requestId":"query"}`, `{"mapId":60,"destX":3,"requestId":"query"}`, `{"mapId":60,"destY":4,"requestId":"query"}`, `{"mapId":60,"destX":3,"destY":4,"requestId":"query"}`} {
		battleDispatch(t, wh, ses, opcodes.PhaserMapInfoRequest, payload)
		if ses.MapID != 50 || char.MapID != 50 || char.X != 7 || char.Y != 8 {
			t.Fatal("map query claimed player location")
		}
	}
	if len(messages.streams) != 4 {
		t.Fatalf("map responses=%d", len(messages.streams))
	}
	for index, message := range messages.streams {
		if message.opcode != opcodes.PhaserMapInfoResponse {
			t.Fatal("query broadcast player state")
		}
		var response struct {
			Success   bool
			RequestID string
		}
		if err := json.Unmarshal(message.payload, &response); err != nil || response.RequestID != "query" || response.Success != (index == 0) {
			t.Fatalf("legacy destination must reject with correlation: %+v %v", response, err)
		}
	}
	battleDispatch(t, wh, ses, opcodes.PhaserPlayerPositionUpdate, `{"mapId":60,"x":3,"y":4,"direction":"DOWN"}`)
	if len(messages.streams) != 4 {
		t.Fatal("retired position report reached gameplay or published state")
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
	for _, mutation := range []string{`DELETE FROM phaser_maps`, `DROP TABLE phaser_maps CASCADE`} {
		testdb.Exec(t, database, mutation)
		battleDispatch(t, wh, ses, opcodes.PhaserOverworldMapsRequest, `{}`)
	}
	for _, message := range messages.streams {
		var failure protocol.ErrorResponse
		if err := json.Unmarshal(message.payload, &failure); err != nil || failure.Success || failure.Error == "" {
			t.Fatal("reserved list request accepted")
		}
	}

	if ses.MapID != 50 {
		t.Fatal("failed metadata query changed presence")
	}
}

func TestCurrentMapMetadataDoesNotRecoverOrRunMapLoadEffects(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 192, 0, 0
	ses.MapID, ses.X, ses.Y = 192, 0, 0
	testdb.Exec(t, database, `UPDATE character_data SET map_id=192,x=0,y=0 WHERE id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(192,'SEAFOAM_ISLANDS_1F',20,20,0);`)
	db.GlobalWorldDB = nil
	battleDispatch(t, wh, ses, opcodes.PhaserMapInfoRequest, `{"mapId":192,"requestId":"pure"}`)
	if char.MapID != 192 || char.X != 0 || char.Y != 0 || ses.X != 0 || ses.Y != 0 {
		t.Fatal("metadata recovered or changed player state")
	}
	if wh.EventFlags.CheckFlag(42, "EVENT_IN_SEAFOAM_ISLANDS") {
		t.Fatal("metadata executed load effects")
	}
	var response protocol.PhaserMapInfoResponse
	if len(messages.streams) != 1 {
		t.Fatal("metadata emitted gameplay messages")
	}
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || !response.Success || response.RequestID != "pure" || response.ID != 192 {
		t.Fatalf("read response: %+v %v", response, err)
	}
	var mapID, x, y int
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 192 || x != 0 || y != 0 {
		t.Fatal("metadata wrote durable position")
	}
}

func TestMapLoadRejectsRemoteSavedLoadAndInvalidDestinations(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	char := ses.Client.CharData()
	char.MapID, char.X, char.Y = 50, 7, 8
	ses.MapID = 50
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(60,'REMOTE',20,20,0),(50,'CURRENT',20,20,0);`)
	payloads := []string{`{"mapId":60,"requestId":"remote"}`, `{"mapId":60,"destX":3,"requestId":"partial"}`, `{"mapId":60,"destX":3,"destY":4,"requestId":"missing"}`, `{"mapId":50,"destX":3,"destY":4,"requestId":"forged"}`, `{"mapId":50,"destX":null,"destY":null,"requestId":"null"}`, `{"mapId":50,"requestId":"trailing"} {}`, `{"mapId":50,"requestId":"junk"} garbage`}
	for _, payload := range payloads {
		if json.Valid([]byte(payload)) {
			battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, payload)
		} else {
			// The frame gate rejects invalid JSON before dispatch; exercise the
			// handler defense directly for trailing objects and garbage.
			HandlePhaserMapLoadRequest(ses, []byte(payload), wh)
		}
		var request protocol.PhaserMapLoadRequest
		var response protocol.PhaserMapRequestError
		if err := json.NewDecoder(bytes.NewReader([]byte(payload))).Decode(&request); err != nil {
			t.Fatal(err)
		}
		last := messages.streams[len(messages.streams)-1]
		if last.opcode != opcodes.PhaserMapLoadResponse {
			t.Fatal("load published wrong response")
		}
		if err := json.Unmarshal(last.payload, &response); err != nil || response.Success || response.RequestID != request.RequestID || response.Error == "" {
			t.Fatalf("load rejection=%+v %v", response, err)
		}
	}
	if len(messages.streams) != len(payloads) || char.MapID != 50 || char.X != 7 || char.Y != 8 || ses.MapID != 50 {
		t.Fatal("invalid load published or changed player state")
	}
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || mapID != 50 || x != 7 || y != 8 {
		t.Fatal("invalid load changed movement registration")
	}
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil || mapID != 50 || x != 7 || y != 8 {
		t.Fatal("invalid load changed stored position")
	}
}
