package world

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/content"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestMapScriptIssuanceUsesOwnedNativeLocation(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	db.GlobalWorldDB = nil // Location queries must use the world's injected database.
	wh.Content = content.New(database)
	wh.Cutscenes = NewCutsceneManager(database)
	for _, name := range []string{"ROOM", "OTHER", "ROUTE"} {
		cs := &CutsceneScript{ScriptLabel: name + "_SCRIPT", MapName: name, TriggerType: "map_script", Actions: json.RawMessage(`[]`)}
		wh.Cutscenes.byMap[name] = []*CutsceneScript{cs}
	}
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES
		(1,'ROOM',10,10,0),(2,'OTHER',10,10,0),(3,'ROUTE',10,10,1),(4,'NEIGHBOR',10,10,1);
		INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,original_source_map_id,is_original_tile_location,is_tile_erased)
		VALUES(100,200,1,4,3,1,1)`)
	ses.Client.CharData().MapID = 1
	request := func(name string, wantStart bool) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.PhaserMapScriptsRequest, `{"mapName":"`+name+`"}`)
		if len(messages.streams) == 0 || messages.streams[0].opcode != opcodes.PhaserMapScriptsResponse {
			t.Fatal("metadata response missing")
		}
		starts := 0
		for _, message := range messages.streams {
			if message.opcode == opcodes.CutsceneStartNotify {
				starts++
			}
		}
		if (starts == 1) != wantStart || starts > 1 {
			t.Fatalf("map=%s starts=%d want=%v", name, starts, wantStart)
		}
	}
	request("OTHER", false)
	request("ROOM", true)
	ses.Client.CharData().MapID = 3
	ses.Client.CharData().X, ses.Client.CharData().Y = 100, 200
	request("ROUTE", true) // Native overworld IDs also carry global coordinates.
	ses.Client.CharData().MapID = 1
	// Movement state overrides the character snapshot left from a previous map.
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.players[42] = &PlayerMovementState{CharacterID: 42, CurrentX: 100, CurrentY: 200, MapID: UnifiedOverworldMapID}
	request("ROOM", false)
	request("ROUTE", true) // Erased/edited art retains original native map identity.
	messages.streams = nil
	sendEligibleMapScriptAfterBattleClose(ses, 42, wh)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.CutsceneStartNotify {
		t.Fatal("post-battle overworld script not issued")
	}
	// A user tile cannot manufacture native script identity.
	testdb.Exec(t, database, `INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id) VALUES(101,200,1,3)`)
	wh.PlayerMovement.players[42].CurrentX = 101
	request("ROUTE", false)
	// Conflicting provenance fails closed rather than selecting the first row.
	wh.PlayerMovement.players[42].CurrentX = 100
	testdb.Exec(t, database, `DROP INDEX phaser_tiles_coord_unique_idx;
		INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,is_original_tile_location) VALUES(100,200,1,4,1)`)
	request("ROUTE", false)
	testdb.Exec(t, database, `ALTER TABLE phaser_tiles RENAME TO unavailable_tiles`)
	request("ROUTE", false)
}

func TestScriptLocationQueryTimeoutAndRetry(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	ses.Client.CharData().MapID = UnifiedOverworldMapID
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'ROUTE',10,10,1);
		INSERT INTO phaser_tiles(x,y,tile_image_id,source_map_id,is_original_tile_location) VALUES(0,0,1,1,1)`)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`LOCK TABLE phaser_tiles IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := wh.nativeScriptMap(ses); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("locked location query=%v", err)
	}
	if time.Since(started) > 7*time.Second {
		t.Fatal("location query exceeded deadline and driver allowance")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if name, err := wh.nativeScriptMap(ses); err != nil || name != "ROUTE" {
		t.Fatalf("retry=%s,%v", name, err)
	}
}
