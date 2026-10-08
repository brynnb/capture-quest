package world

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestSameTileCommittedTeleportRetiresPathWithoutStorageRead(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	m := NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement = m
	m.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	m.players[42].Path = []PathNode{{X: 8, Y: 8}}
	m.players[42].pendingStep = &issuedPlayerStep{}
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	done := make(chan struct{})
	go func() { m.projectCommittedTeleport(42, 7, 8, 50, "RIGHT"); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("committed teleport waited for storage")
	}
	state := m.players[42]
	if len(state.Path) != 0 || state.pendingStep != nil || state.Direction != "RIGHT" || database.Stats().WaitCount != before {
		t.Fatal("same-tile teleport retained movement intent or entered storage")
	}
}

func TestTeleportCommitFailurePreservesSafariPositionAndPublication(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	ses.Client.CharData().MapID = 220
	ses.Client.CharData().X = 14
	ses.Client.CharData().Y = 24
	wh.PlayerMovement.RegisterPlayer(ses, 42, 14, 24, 220, "UP")
	if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `UPDATE character_data SET map_id=220,x=14,y=24 WHERE id=42;
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_IN_SAFARI_ZONE');
 CREATE FUNCTION reject_position_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late position failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_position_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.map_id=60) EXECUTE FUNCTION reject_position_commit();`)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	if err := teleportPlayerTo(ses, wh, 60, 3, 4); err == nil {
		t.Fatal("accepted failed teleport")
	}
	if len(messages.streams) != 0 {
		t.Fatalf("failed teleport notified %+v", messages.streams)
	}
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || mapID != 220 || x != 14 || y != 24 || ses.MapID != 220 || ses.Client.CharData().MapID != 220 {
		t.Fatalf("failed live position %d %d %d", x, y, mapID)
	}
	s, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s == nil || !s.Active {
		t.Fatalf("failed teleport removed Safari=%+v %v", s, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_position_commit ON character_data`)
	if err := teleportPlayerTo(ses, wh, 60, 3, 4); err != nil {
		t.Fatal(err)
	}
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.WarpTileTeleportNotify {
		t.Fatalf("committed teleport=%+v", messages.streams)
	}
	s, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s != nil || wh.EventFlags.CheckFlag(42, EventInSafariZone) {
		t.Fatalf("committed Safari exit=%+v %v", s, err)
	}
}

func TestMapLoadRejectsLatePersistenceFailure(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 50, 7, 8
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(60,'EXIT',20,20,0);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id) VALUES(60,3,4,1);
 CREATE FUNCTION reject_arrival_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late arrival failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_arrival_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.map_id=60) EXECUTE FUNCTION reject_arrival_commit();`)
	db.GlobalWorldDB = nil
	wh.PlayerMovement.projectCommittedTeleport(42, 3, 4, 60, "UP")
	battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, `{"mapId":60,"requestId":"arrival"}`)
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || x != 3 || y != 4 || mapID != 60 || ses.MapID != 50 || ses.Client.CharData().MapID != 50 {
		t.Fatalf("failed arrival changed live position: %d %d %d", x, y, mapID)
	}
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PhaserMapLoadResponse {
		t.Fatalf("failed arrival published %+v", messages.streams)
	}
	var response struct {
		Success bool
		Error   string
	}
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error == "" {
		t.Fatalf("map response=%+v %v", response, err)
	}
	if err := database.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=42`).Scan(&x, &y, &mapID); err != nil || x != 7 || y != 8 || mapID != 50 {
		t.Fatalf("failed arrival changed durable position: %d %d %d %v", x, y, mapID, err)
	}
}

func TestMapDestinationCommitIsNotOverwrittenByInvalidSavedPositionRecovery(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	wh.sessionManager = session.NewSessionManager()
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.EventFlags = nil // This test isolates destination persistence from map-load scripts.
	ses.Client.CharData().X = 0
	ses.Client.CharData().Y = 0
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(60,'EXIT',20,20,0); INSERT INTO phaser_tiles(map_id,x,y,tile_image_id) VALUES(60,3,4,1)`)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 3, 4, 60, "UP")
	battleDispatch(t, wh, ses, opcodes.PhaserMapLoadRequest, `{"mapId":60,"requestId":"arrival"}`)
	var x, y, mapID int
	if err := database.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=42`).Scan(&x, &y, &mapID); err != nil || x != 3 || y != 4 || mapID != 60 {
		t.Fatalf("destination overwritten: %d %d %d %v", x, y, mapID, err)
	}
	if ses.Client.CharData().X != 3 || ses.Client.CharData().Y != 4 || ses.MapID != 60 {
		t.Fatal("live destination disagrees with committed position")
	}
}

func TestPositionTransactionCancellationWhileCharacterLocked(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`UPDATE character_data SET id=id WHERE id=42`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- commitPlayerPosition(ctx, wh.database, 42, 60, 3, 4) }()
	select {
	case err := <-done:
		if err == nil || ctx.Err() == nil {
			t.Fatalf("cancelled position result=%v context=%v", err, ctx.Err())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("position ignored cancellation")
	}
}

func TestDisconnectCancelsIssuedStepTransactionBeforeCleanup(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	step := issueStep(t, wh, ses, messages)
	database := wh.database
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`UPDATE character_data SET id=id WHERE id=42`); err != nil {
		t.Fatal(err)
	}
	registry := NewWorldOpCodeRegistry()
	registry.WH = wh
	done := make(chan struct{})
	go func() {
		defer close(done)
		registry.HandleWorldPacket(ses, clientPacket(opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"blocked"}`, step.StepToken)))
	}()
	deadline := time.Now().Add(time.Second)
	for database.Stats().InUse < 2 {
		if time.Now().After(deadline) {
			t.Fatal("issued step did not start its transaction")
		}
		runtime.Gosched()
	}
	ses.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disconnect did not cancel blocked position transaction")
	}
	cleaned := false
	ses.DrainCommands(func() { cleaned = true })
	if !cleaned {
		t.Fatal("cancelled command kept cleanup blocked")
	}
	var x, y, mapID int
	if err := database.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=42`).Scan(&x, &y, &mapID); err != nil || x != 7 || y != 8 || mapID != 50 {
		t.Fatalf("cancelled position changed durable state: %d %d %d %v", x, y, mapID, err)
	}
	if ses.Client.CharData().X != 7 || ses.Client.CharData().Y != 8 {
		t.Fatal("cancelled position changed live state")
	}
}
