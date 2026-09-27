package world

import (
	"context"
	"testing"
	"time"

	"capturequest/internal/session"
)

func TestMovementTickUsesSessionOwnerAndRejectsStaleRegistration(t *testing.T) {
	_, wh, fixture, _ := battleTestWorld(t)
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
	_ = ses.ExecuteCommand(context.Background(), func() { m.processCharacterTick(42, state) })
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
