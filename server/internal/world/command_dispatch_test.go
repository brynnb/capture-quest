package world

import (
	"sync"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

func TestDispatcherSerializesPrerequisitesAndHandlerState(t *testing.T) {
	registry := NewWorldOpCodeRegistry()
	// Login is admitted without authentication and rejected after character entry.
	// A custom login handler here isolates the real dispatch boundary from storage.
	registry.handlers[opcodes.JWTLogin] = func(s *session.Session, _ []byte, _ *WorldHandler) bool {
		s.AccountID++
		return false
	}
	s := &session.Session{}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); registry.HandleWorldPacket(s, clientPacket(opcodes.JWTLogin, "{}")) }()
	}
	wg.Wait()
	if s.AccountID != 16 {
		t.Fatalf("lost commands: %d", s.AccountID)
	}
	s.Close()
	registry.HandleWorldPacket(s, clientPacket(opcodes.JWTLogin, "{}"))
	if s.AccountID != 16 {
		t.Fatal("closed session command executed")
	}
}
