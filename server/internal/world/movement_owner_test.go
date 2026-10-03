package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/protocol"
	"context"
	"encoding/json"
	"testing"
	"time"

	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestMovementTickUsesSessionOwnerAndRejectsStaleRegistration(t *testing.T) {
	database, wh, fixture, _ := battleTestWorld(t)
	wh.sessionManager = session.NewSessionManager()
	ses := wh.sessionManager.CreateNextSession(&recordingMessenger{}, "", nil)
	ses.Client = fixture.Client
	ses.Authenticated = true
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	m := NewPlayerMovementManager(wh, wh.ActorManager)
	wh.PlayerMovement = m
	if err := wh.characterOwners.acquire(context.Background(), 42, ses, nil); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'ROOM',30,30,0); INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(1,10,10,1,1),(1,11,10,1,1); UPDATE character_data SET map_id=1,x=10,y=10 WHERE id=42;`)
	m.RegisterPlayer(ses, 42, 10, 10, 1, "RIGHT")
	state := m.players[42]
	state.Path = []PathNode{{X: 11, Y: 10}}
	state.LastMoveTime = time.Time{}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_ = ses.ExecuteCommand(context.Background(), func() { close(entered); <-release })
	}()
	<-entered
	m.processTick()
	if state.CurrentX != 10 || len(state.Path) != 1 {
		t.Fatal("timer mutated busy session")
	}
	close(release)
	<-done
	m.processTick()
	if state.CurrentX != 11 || len(state.Path) != 0 || ses.X != 11 {
		t.Fatal("idle owner did not advance and publish position")
	}

	m.RegisterPlayer(ses, 42, 20, 20, 1, "RIGHT")
	replacement := m.players[42]
	replacement.Path = []PathNode{{X: 21, Y: 20}}
	replacement.LastMoveTime = time.Time{}
	_ = ses.ExecuteCommand(context.Background(), func() { m.processCharacterTick(ses.CommandContext(), 42, state) })
	if replacement.CurrentX != 20 || len(replacement.Path) != 1 {
		t.Fatal("stale timer candidate advanced replacement registration")
	}
	wh.characterOwners.release(42, ses)
	m.processTick()
	if replacement.CurrentX != 20 {
		t.Fatal("unowned session advanced")
	}
	ses.Close()
}

func TestServerMovementCommitFailureRetainsSourcePathAndDoesNotPublish(t *testing.T) {
	wh, fixture, messages := setupIssuedStep(t)
	ses := wh.sessionManager.CreateNextSession(messages, "", nil)
	ses.Client = fixture.Client
	ses.Authenticated = true
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	m := wh.PlayerMovement
	m.RegisterPlayer(ses, 42, 7, 8, 50, "RIGHT")
	state := m.players[42]
	state.Path = []PathNode{{X: 8, Y: 8}}
	state.LastMoveTime = time.Time{}
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_forced_step() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late forced step failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_forced_step AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.x=8) EXECUTE FUNCTION reject_forced_step();`)
	_ = ses.ExecuteCommand(context.Background(), func() { m.processCharacterTick(ses.CommandContext(), 42, state) })
	assertStepPosition(t, wh, ses, 7)
	if len(state.Path) != 1 || len(messages.streams) != 0 {
		t.Fatal("failed forced step retired path or published success")
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_forced_step ON character_data`)
	state.LastMoveTime = time.Time{}
	_ = ses.ExecuteCommand(context.Background(), func() { m.processCharacterTick(ses.CommandContext(), 42, state) })
	assertStepPosition(t, wh, ses, 8)
	if len(state.Path) != 0 || len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.ServerPlayerMovementNotify {
		t.Fatalf("projection=%+v", messages.streams)
	}
	var position protocol.ServerPlayerMovementNotify
	if err := json.Unmarshal(messages.streams[0].payload, &position); err != nil || position.X != 8 || position.Y != 8 || !position.PathFinished || position.ActorID == 0 {
		t.Fatalf("position=%+v %v", position, err)
	}
	// Bicycle metadata is an ordinary actor refresh, not a forced position that
	// could retire the origin client's issued animation.
	messages.streams = nil
	m.ToggleBicycle(42)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PhaserActorPositionUpdate {
		t.Fatalf("bicycle refresh became forced movement: %+v", messages.streams)
	}
	ses.Close()
}
