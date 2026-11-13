package world

import (
	"context"
	"testing"

	"capturequest/internal/session"
)

func TestBroadcastUsesPublishedPresenceWhileRecipientCommandRuns(t *testing.T) {
	sm := session.NewSessionManager()
	messages := &recordingMessenger{}
	s := sm.CreateNextSession(messages, "", nil)
	_ = s.ExecuteCommand(context.Background(), func() { s.Authenticated = true; s.MapID = 40 })
	m := NewPhaserActorManager(&WorldHandler{sessionManager: sm})
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_ = s.ExecuteCommand(context.Background(), func() { s.MapID = 63; close(entered); <-release })
	}()
	<-entered
	m.broadcastActorDespawn(7, 40)
	m.broadcastActorDespawn(7, 63)
	if len(messages.streams) != 1 {
		t.Fatalf("unpublished map change affected routing: %d", len(messages.streams))
	}
	close(release)
	<-done
	m.broadcastActorDespawn(7, 40)
	m.broadcastActorDespawn(7, 63)
	if len(messages.streams) != 2 {
		t.Fatalf("published map change missing: %d", len(messages.streams))
	}
	s.Close()
	m.broadcastActorDespawn(7, 63)
	if len(messages.streams) != 2 {
		t.Fatal("closed recipient received broadcast")
	}
}
