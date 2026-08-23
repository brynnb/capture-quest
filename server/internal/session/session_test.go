package session

import (
	"testing"
	"time"
)

type testMessenger struct{}

func (testMessenger) SendDatagram(int, []byte) error { return nil }
func (testMessenger) SendStream(int, []byte) error   { return nil }

func TestForEachSessionCallbackCanRemoveSession(t *testing.T) {
	manager := NewSessionManager()
	manager.CreateSession(testMessenger{}, 1, "127.0.0.1", nil)

	done := make(chan struct{})
	go func() {
		manager.ForEachSession(func(session *Session) {
			manager.RemoveSession(session.SessionID)
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ForEachSession held the manager lock while invoking callback")
	}
	if _, ok := manager.GetSession(1); ok {
		t.Fatal("session was not removed")
	}
}

func TestRemoveSessionIsIdempotent(t *testing.T) {
	manager := NewSessionManager()
	original := manager.CreateSession(testMessenger{}, 7, "127.0.0.1", nil)

	removed, ok := manager.RemoveSession(7)
	if !ok || removed != original {
		t.Fatalf("first removal = (%p, %v), want (%p, true)", removed, ok, original)
	}
	if removed, ok := manager.RemoveSession(7); ok || removed != nil {
		t.Fatalf("second removal = (%p, %v), want (nil, false)", removed, ok)
	}
}
