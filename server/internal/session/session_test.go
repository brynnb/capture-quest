package session

import (
	"errors"
	"fmt"
	"net"
	"sync"
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

func TestSessionCloseUnblocksWriterAndRejectsReplacementStream(t *testing.T) {
	serverConn, peer := net.Pipe()
	defer peer.Close()
	ses := NewSessionManager().CreateNextSession(testMessenger{}, "test", serverConn)
	if ses.LastHeartbeat().IsZero() {
		t.Fatal("new session has no idle-expiry timestamp")
	}
	if ses.AttachControlStream(peer) {
		t.Fatal("replaced existing stream")
	}
	done := make(chan error, 1)
	go func() { done <- ses.WriteControlStream([]byte("blocked write")) }()
	ses.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("write succeeded without reader")
		}
	case <-time.After(time.Second):
		t.Fatal("closing session did not unblock writer")
	}
	if ses.AttachControlStream(peer) {
		t.Fatal("attached stream to closed session")
	}
}

func TestSessionIDsAreUniqueAcrossConcurrentConnections(t *testing.T) {
	manager := NewSessionManager()
	ids := make(chan int, 100)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 25; j++ {
				ids <- manager.CreateNextSession(testMessenger{}, "test", nil).SessionID
			}
		}()
	}
	workers.Wait()
	close(ids)
	seen := make(map[int]bool)
	for id := range ids {
		if id <= 0 || seen[id] {
			t.Fatalf("invalid or duplicate ID %d", id)
		}
		seen[id] = true
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

func TestSessionPlaytimeTracksAndClaimsWholeSeconds(t *testing.T) {
	var session Session
	started := time.Unix(1_000, 0)
	session.StartPlaytime(started, 12, 9)

	if got := session.CurrentPlaytime(started.Add(2500 * time.Millisecond)); got != 14 {
		t.Fatalf("CurrentPlaytime = %d, want 14", got)
	}
	got, err := session.PersistPlaytime(started.Add(2500*time.Millisecond), func(characterID int32, seconds uint32) error {
		if characterID != 9 || seconds != 2 {
			t.Fatalf("persist = (%d, %d), want (9, 2)", characterID, seconds)
		}
		return nil
	})
	if err != nil || got != 2 {
		t.Fatalf("PersistPlaytime = (%d, %v), want (2, nil)", got, err)
	}
	if got := session.CurrentPlaytime(started.Add(3900 * time.Millisecond)); got != 15 {
		t.Fatalf("CurrentPlaytime after claim = %d, want 15", got)
	}
}

func TestSessionPlaytimeRetainsFailedPersistenceInterval(t *testing.T) {
	var session Session
	started := time.Unix(2_000, 0)
	session.StartPlaytime(started, 20, 9)

	wantErr := fmt.Errorf("database unavailable")
	if _, err := session.PersistPlaytime(started.Add(5*time.Second), func(int32, uint32) error {
		return wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("PersistPlaytime error = %v, want %v", err, wantErr)
	}

	if got := session.CurrentPlaytime(started.Add(6 * time.Second)); got != 26 {
		t.Fatalf("CurrentPlaytime after rollback = %d, want 26", got)
	}
}

func TestSessionPlaytimeStopsAtCharacterSelect(t *testing.T) {
	var session Session
	started := time.Unix(3_000, 0)
	session.StartPlaytime(started, 7, 9)
	if got, err := session.PersistPlaytime(started.Add(4*time.Second), func(int32, uint32) error { return nil }); err != nil || got != 4 {
		t.Fatalf("PersistPlaytime = (%d, %v), want (4, nil)", got, err)
	}
	session.StopPlaytime()

	if got := session.CurrentPlaytime(started.Add(time.Hour)); got != 11 {
		t.Fatalf("CurrentPlaytime while stopped = %d, want 11", got)
	}
}
