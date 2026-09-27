package world

import (
	"context"
	"errors"
	"sync"

	"capturequest/internal/session"
)

var errCharacterHandoff = errors.New("character connection handoff already in progress")

type characterOwner struct {
	session *session.Session
	handoff bool
}

// characterOwners arbitrates authenticated character entry. Never hold mu while
// draining a session: disconnect cleanup also consults this registry. Callers
// acquire from an empty session's command gate, preventing cross-session cycles.
type characterOwners struct {
	mu      sync.Mutex
	entries map[int64]*characterOwner
}

func (o *characterOwners) acquire(ctx context.Context, id int64, next *session.Session, cleanup func(*session.Session)) error {
	o.mu.Lock()
	if o.entries == nil {
		o.entries = make(map[int64]*characterOwner)
	}
	e := o.entries[id]
	if e == nil {
		e = &characterOwner{}
		o.entries[id] = e
	}
	if e.handoff {
		o.mu.Unlock()
		return errCharacterHandoff
	}
	if e.session == next {
		o.mu.Unlock()
		return errors.New("session already owns character")
	}
	e.handoff = true
	previous := e.session
	o.mu.Unlock()

	var err error
	if previous != nil {
		previous.Close()
		err = previous.DrainCommandsContext(ctx, func() { cleanup(previous) })
	}
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && next.IsClosed() {
		err = session.ErrSessionClosed
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	e.handoff = false
	if err == nil {
		e.session = next
	}
	if e.session == nil {
		delete(o.entries, id)
	}
	return err
}

func (o *characterOwners) owns(id int64, s *session.Session) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	e := o.entries[id]
	return e != nil && e.session == s
}

func (o *characterOwners) release(id int64, s *session.Session) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if e := o.entries[id]; e != nil && e.session == s {
		e.session = nil
		if !e.handoff {
			delete(o.entries, id)
		}
	}
}
