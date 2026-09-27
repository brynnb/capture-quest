package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// IssuedEvents binds completion to the exact server-issued payload and character.
// Claims serialize duplicate completion attempts without holding a lock during
// database work. This is session authorization, not durable request deduplication.
type IssuedEvents struct {
	mu      sync.Mutex
	entries map[string]*issuedEvent
}
type issuedEvent struct {
	label       string
	characterID int64
	payload     []byte
	expires     time.Time
	claimed     bool
}

func (e *IssuedEvents) Issue(characterID int64, label string, payload []byte) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	if e.entries == nil {
		e.entries = make(map[string]*issuedEvent)
	}
	for token, event := range e.entries {
		if !event.claimed && now.After(event.expires) {
			delete(e.entries, token)
		}
	}
	if len(e.entries) >= 8 {
		return "", fmt.Errorf("too many pending events")
	}
	token := uuid.NewString()
	e.entries[token] = &issuedEvent{label: label, characterID: characterID, payload: append([]byte(nil), payload...), expires: now.Add(30 * time.Minute)}
	return token, nil
}
func (e *IssuedEvents) Claim(characterID int64, label, token string) ([]byte, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	event := e.entries[token]
	if event == nil || event.claimed || event.characterID != characterID || event.label != label || time.Now().After(event.expires) {
		return nil, false
	}
	event.claimed = true
	return append([]byte(nil), event.payload...), true
}
func (e *IssuedEvents) Finish(token string, committed bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if committed {
		delete(e.entries, token)
	} else if event := e.entries[token]; event != nil {
		event.claimed = false
	}
}
func (e *IssuedEvents) Clear() { e.mu.Lock(); e.entries = nil; e.mu.Unlock() }
