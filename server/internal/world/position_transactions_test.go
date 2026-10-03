package world

import (
	"context"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestTeleportCommitFailurePreservesSafariPositionAndPublication(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	ses.Client.CharData().MapID = 220
	ses.Client.CharData().X = 14
	ses.Client.CharData().Y = 24
	wh.PlayerMovement.RegisterPlayer(ses, 42, 14, 24, 220, "UP")
	if err := wh.Safari.SetSession(42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
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
	s, err := wh.Safari.GetSession(42)
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
	s, err = wh.Safari.GetSession(42)
	if err != nil || s != nil || wh.EventFlags.CheckFlag(42, EventInSafariZone) {
		t.Fatalf("committed Safari exit=%+v %v", s, err)
	}
}

func TestReportedPositionAndMapInfoRejectLatePersistenceFailure(t *testing.T) {
	for _, opcode := range []opcodes.OpCode{opcodes.PhaserPlayerPositionUpdate, opcodes.PhaserMapInfoRequest} {
		t.Run(string(rune(opcode)), func(t *testing.T) {
			database, wh, ses, messages := battleTestWorld(t)
			wh.ActorRegistry = NewActorRegistry()
			wh.ActorManager = NewPhaserActorManager(wh)
			wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
			ses.Client.CharData().MapID = 50
			ses.Client.CharData().X = 7
			ses.Client.CharData().Y = 8
			wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
			testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(60,'EXIT',20,20,0);
 CREATE FUNCTION reject_reported_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late reported failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_reported_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.map_id=60) EXECUTE FUNCTION reject_reported_commit();`)
			db.GlobalWorldDB = nil
			payload := `{"mapId":60,"x":3,"y":4,"direction":"DOWN"}`
			if opcode == opcodes.PhaserMapInfoRequest {
				payload = `{"mapId":60,"destX":3,"destY":4}`
			}
			battleDispatch(t, wh, ses, opcode, payload)
			x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
			if !ok || x != 7 || y != 8 || mapID != 50 || ses.MapID != 50 || ses.Client.CharData().MapID != 50 {
				t.Fatalf("reported failed position %d %d %d", x, y, mapID)
			}
			if len(messages.streams) != 1 {
				t.Fatalf("failed position published %+v", messages.streams)
			}
			if opcode == opcodes.PhaserMapInfoRequest {
				var response struct {
					Success bool
					Error   string
				}
				if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error == "" {
					t.Fatalf("map response=%+v %v", response, err)
				}
			}
		})
	}
}

func TestMovementSaveFailureKeepsDirtyStateForRetry(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	m := NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement = m
	m.RegisterPlayer(ses, 42, 1, 2, 50, "UP")
	m.UpdatePosition(42, 7, 8, 50, "UP")
	before := m.players[42].LastSaveTime
	testdb.Exec(t, database, `CREATE FUNCTION reject_flush_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late flush failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_flush_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_flush_commit();`)
	db.GlobalWorldDB = nil
	if err := m.FlushPlayerPosition(context.Background(), 42); err == nil {
		t.Fatal("flush ignored commit failure")
	}
	if !m.players[42].positionDirty || !m.players[42].LastSaveTime.Equal(before) {
		t.Fatal("failed flush marked saved")
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_flush_commit ON character_data`)
	if err := m.FlushPlayerPosition(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if m.players[42].positionDirty || !m.players[42].LastSaveTime.After(before) {
		t.Fatal("retry did not record commit")
	}
	var x, y int
	if err := database.QueryRow(`SELECT x,y FROM character_data WHERE id=42`).Scan(&x, &y); err != nil || x != 7 || y != 8 {
		t.Fatalf("retry saved %d %d %v", x, y, err)
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
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(60,'EXIT',20,20,0)`)
	battleDispatch(t, wh, ses, opcodes.PhaserMapInfoRequest, `{"mapId":60,"destX":3,"destY":4}`)
	var x, y, mapID int
	if err := database.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=42`).Scan(&x, &y, &mapID); err != nil || x != 3 || y != 4 || mapID != 60 {
		t.Fatalf("destination overwritten: %d %d %d %v", x, y, mapID, err)
	}
	if ses.Client.CharData().X != 3 || ses.Client.CharData().Y != 4 || ses.MapID != 60 {
		t.Fatal("live destination disagrees with committed position")
	}
}

func TestBlockedMovementFlushReleasesGlobalLockAndDoesNotMarkNewSnapshotSaved(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	m := NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement = m
	m.RegisterPlayer(ses, 42, 1, 2, 50, "UP")
	m.UpdatePosition(42, 7, 8, 50, "UP")
	before := m.players[42].LastSaveTime
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`UPDATE character_data SET id=id WHERE id=42`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- m.FlushPlayerPosition(context.Background(), 42) }()
	deadline := time.Now().Add(time.Second)
	for database.Stats().InUse < 2 {
		if time.Now().After(deadline) {
			t.Fatal("flush did not enter its transaction")
		}
		runtime.Gosched()
	}
	available := make(chan struct{})
	go func() { m.UpdatePosition(42, 9, 10, 50, "UP"); close(available) }()
	select {
	case <-available:
	case <-time.After(time.Second):
		t.Fatal("database wait held movement lock")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !m.players[42].positionDirty || !m.players[42].LastSaveTime.Equal(before) {
		t.Fatal("old flush marked newer snapshot committed")
	}
	if err := m.FlushPlayerPosition(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	var x, y int
	if err := database.QueryRow(`SELECT x,y FROM character_data WHERE id=42`).Scan(&x, &y); err != nil || x != 9 || y != 10 {
		t.Fatalf("new snapshot=%d %d %v", x, y, err)
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

func TestDisconnectCancelsReportedPositionTransactionBeforeCleanup(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	wh.ActorManager = NewPhaserActorManager(wh)
	ses.Client.CharData().MapID = 50
	ses.Client.CharData().X = 7
	ses.Client.CharData().Y = 8
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42`)
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
		registry.HandleWorldPacket(ses, clientPacket(opcodes.PhaserPlayerPositionUpdate, `{"mapId":60,"x":3,"y":4,"direction":"DOWN"}`))
	}()
	deadline := time.Now().Add(time.Second)
	for database.Stats().InUse < 2 {
		if time.Now().After(deadline) {
			t.Fatal("reported position did not start its transaction")
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
