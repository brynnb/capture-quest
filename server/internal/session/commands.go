package session

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

const MaxPendingCommands = 32

var ErrCommandQueueFull = errors.New("session command queue is full")
var ErrSessionClosed = errors.New("session is closed")

// commandGate bounds both executing and waiting callers. No per-packet worker
// goroutine is created. Reliable-stream and datagram readers share this owner.
// Callbacks must not recursively enter the gate.
type commandGate struct {
	once    sync.Once
	token   chan struct{}
	pending atomic.Int32
}

func (g *commandGate) init() {
	g.once.Do(func() { g.token = make(chan struct{}, 1); g.token <- struct{}{} })
}
func (s *Session) ExecuteCommand(ctx context.Context, command func()) error {
	if s.IsClosed() {
		return ErrSessionClosed
	}
	g := &s.commands
	g.init()
	if g.pending.Add(1) > MaxPendingCommands {
		g.pending.Add(-1)
		return ErrCommandQueueFull
	}
	defer g.pending.Add(-1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.token:
	}
	defer func() { g.token <- struct{}{} }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.IsClosed() {
		return ErrSessionClosed
	}
	command()
	return nil
}

// DrainCommands waits for the running callback before cleanup. Close the session
// first: queued callbacks then fail their closed check without touching state.
// This is a lifecycle barrier, not a command; it must run outside a callback.
func (s *Session) DrainCommands(cleanup func()) {
	g := &s.commands
	g.init()
	<-g.token
	defer func() { g.token <- struct{}{} }()
	cleanup()
}
