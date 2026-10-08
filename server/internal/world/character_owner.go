package world

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"capturequest/internal/session"
)

var errCharacterHandoff = errors.New("character connection handoff already in progress")

type characterOwner struct {
	session *session.Session
	handoff bool
	// Failed final saves outlive the retired session. Admission or sealed shutdown
	// retries them behind the same barrier, before any replacement can load.
	cleanup func(context.Context) error
}

// characterOwners arbitrates authenticated character entry. Never hold mu while
// draining a session: disconnect cleanup also consults this registry. Callers
// acquire from an empty session's command gate, preventing cross-session cycles.
type characterOwners struct {
	mu      sync.Mutex
	entries map[int64]*characterOwner
	sealed  bool
}

func (o *characterOwners) acquire(ctx context.Context, id int64, next *session.Session, cleanup func(context.Context, *session.Session) error) error {
	o.mu.Lock()
	if o.sealed {
		o.mu.Unlock()
		return session.ErrSessionClosed
	}
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
	recovery := e.cleanup
	o.mu.Unlock()

	var err error
	if recovery != nil {
		err = recovery(ctx)
	}
	recovered := recovery != nil && err == nil
	if err == nil && previous != nil {
		previous.Close()
		var cleanupErr error
		err = previous.DrainCommandsContext(ctx, func() { cleanupErr = cleanup(ctx, previous) })
		if err == nil {
			err = cleanupErr
		}
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
	if recovered {
		e.cleanup = nil
	}
	if err == nil && o.sealed {
		err = session.ErrSessionClosed
	}
	if err == nil {
		e.session = next
	}
	if e.session == nil && e.cleanup == nil {
		delete(o.entries, id)
	}
	return err
}

func (o *characterOwners) seal() {
	o.mu.Lock()
	o.sealed = true
	o.mu.Unlock()
}

// recoverPending runs after the sealed session registry and its claimed cleanup
// work have drained. Storage stays open until this pass completes. Never hold
// the registry mutex across I/O; the handoff marker fences each recovery.
func (o *characterOwners) recoverPending(ctx context.Context) error {
	o.mu.Lock()
	if !o.sealed {
		o.mu.Unlock()
		return errors.New("character cleanup recovery requires sealed admissions")
	}
	ids := make([]int64, 0, len(o.entries))
	for id := range o.entries {
		ids = append(ids, id)
	}
	o.mu.Unlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var result error
	for _, id := range ids {
		o.mu.Lock()
		e := o.entries[id]
		if e == nil {
			o.mu.Unlock()
			continue
		}
		if e.handoff || e.session != nil {
			o.mu.Unlock()
			result = errors.Join(result, fmt.Errorf("character %d: owner drain unfinished", id))
			continue
		}
		if e.cleanup == nil {
			delete(o.entries, id)
			o.mu.Unlock()
			continue
		}
		e.handoff = true
		recovery := e.cleanup
		o.mu.Unlock()
		err := recovery(ctx)
		o.mu.Lock()
		e.handoff = false
		if err == nil {
			e.cleanup = nil
			delete(o.entries, id)
		}
		o.mu.Unlock()
		if err != nil {
			result = errors.Join(result, fmt.Errorf("character %d: %w", id, err))
		}
	}
	return result
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
		if !e.handoff && e.cleanup == nil {
			delete(o.entries, id)
		}
	}
}

func (o *characterOwners) retire(id int64, s *session.Session, recovery func(context.Context) error, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if e := o.entries[id]; e != nil && e.session == s {
		e.session = nil
		if err != nil {
			e.cleanup = recovery
		}
		if !e.handoff && e.cleanup == nil {
			delete(o.entries, id)
		}
	}
}
