package session

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestPeriodicCommandSkipsBusyAndClosedSessions(t *testing.T) {
	s := &Session{}
	if err := s.ExecuteCommand(context.Background(), func() {
		if err := s.TryExecuteCommand(func() { t.Error("busy callback executed") }); !errors.Is(err, ErrCommandBusy) {
			t.Fatalf("busy=%v", err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	called := false
	if err := s.TryExecuteCommand(func() { called = true }); err != nil || !called {
		t.Fatalf("idle callback: %v", err)
	}
	s.Close()
	if err := s.TryExecuteCommand(func() { t.Error("closed callback executed") }); !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("closed=%v", err)
	}
}

func TestCommandsSerializeAndBoundPendingCallers(t *testing.T) {
	s := &Session{}
	entered, release := make(chan struct{}), make(chan struct{})
	var wg sync.WaitGroup
	var value int
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.ExecuteCommand(context.Background(), func() { close(entered); <-release; value++ }); err != nil {
			t.Error(err)
		}
	}()
	<-entered
	for i := 1; i < MaxPendingCommands; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.ExecuteCommand(context.Background(), func() { value++ }); err != nil {
				t.Error(err)
			}
		}()
	}
	deadline := time.Now().Add(time.Second)
	for s.commands.pending.Load() != MaxPendingCommands {
		if time.Now().After(deadline) {
			t.Fatal("callers did not enqueue")
		}
		runtime.Gosched()
	}
	if err := s.ExecuteCommand(context.Background(), func() { t.Error("overflow executed") }); !errors.Is(err, ErrCommandQueueFull) {
		t.Fatalf("overflow=%v", err)
	}
	close(release)
	wg.Wait()
	if value != MaxPendingCommands {
		t.Fatalf("lost serialized updates: %d", value)
	}
}

func TestCloseDrainsRunningCommandAndRejectsQueuedWork(t *testing.T) {
	s := &Session{}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_ = s.ExecuteCommand(context.Background(), func() { close(entered); <-release })
	}()
	<-entered
	queued := make(chan error, 1)
	go func() {
		queued <- s.ExecuteCommand(context.Background(), func() { t.Error("closed queued command ran") })
	}()
	s.Close()
	drained := make(chan struct{})
	go func() { s.DrainCommands(func() { close(drained) }) }()
	select {
	case <-drained:
		t.Fatal("cleanup raced running command")
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	<-done
	<-drained
	if err := <-queued; !errors.Is(err, ErrSessionClosed) {
		t.Fatalf("queued=%v", err)
	}
	if err := s.ExecuteCommand(context.Background(), func() { t.Error("new closed command ran") }); !errors.Is(err, ErrSessionClosed) {
		t.Fatal(err)
	}
}

func TestCancelledCommandDoesNotExecuteAndReleasesCapacity(t *testing.T) {
	s := &Session{}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		_ = s.ExecuteCommand(context.Background(), func() { close(entered); <-release })
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.ExecuteCommand(ctx, func() { t.Error("cancelled work ran") }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	<-done
	if s.commands.pending.Load() != 0 {
		t.Fatal("capacity leaked")
	}
	if err := s.ExecuteCommand(context.Background(), func() {}); err != nil {
		t.Fatal(err)
	}
}

func TestRunningCommandObservesDeadlineAndReleasesOwner(t *testing.T) {
	s := &Session{}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := s.ExecuteCommand(ctx, func() {
		commandCtx := s.CommandContext()
		expected, _ := ctx.Deadline()
		if deadline, ok := commandCtx.Deadline(); !ok || !deadline.Equal(expected) {
			t.Error("running command lost admission deadline")
		}
		<-commandCtx.Done()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline result=%v", err)
	}
	if s.CommandContext().Err() != nil {
		t.Fatal("completed request context leaked")
	}
	if err := s.ExecuteCommand(context.Background(), func() {}); err != nil {
		t.Fatal(err)
	}
}

func TestCloseCancelsPeriodicOwnerBeforeCleanup(t *testing.T) {
	s := &Session{}
	entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.TryExecuteCommand(func() {
			ctx := s.CommandContext()
			close(entered)
			<-ctx.Done()
			close(cancelled)
			<-release
		})
	}()
	<-entered
	queued := make(chan error, 1)
	go func() {
		queued <- s.ExecuteCommand(context.Background(), func() { t.Error("closed queued callback ran") })
	}()
	deadline := time.Now().Add(time.Second)
	for s.commands.pending.Load() != 2 {
		if time.Now().After(deadline) {
			t.Fatal("queued callback did not enter gate")
		}
		runtime.Gosched()
	}
	s.Close()
	select {
	case err := <-queued:
		if !errors.Is(err, ErrSessionClosed) {
			t.Fatalf("closed queued result=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("closed queued callback kept waiting for owner")
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("close did not cancel running owner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := s.DrainCommandsContext(ctx, func() { t.Error("cleanup overlapped callback") }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("drain=%v", err)
	}
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("closed callback=%v", err)
	}
	cleaned := false
	s.DrainCommands(func() { cleaned = true })
	if !cleaned {
		t.Fatal("cleanup did not drain")
	}
}

func TestChatThrottleBelongsToSessionAndRetiresWithIt(t *testing.T) {
	first, replacement := &Session{SessionID: 7}, &Session{SessionID: 7}
	now := time.Unix(1, 0)
	interval := 500 * time.Millisecond
	if !first.AllowChatMessage(now, interval) || first.AllowChatMessage(now.Add(interval/2), interval) {
		t.Fatal("chat throttle did not bound first session")
	}
	if !replacement.AllowChatMessage(now, interval) {
		t.Fatal("replacement inherited reused session ID's limit")
	}
	if !first.AllowChatMessage(now.Add(interval), interval) {
		t.Fatal("throttle did not expire")
	}
	first.Close()
	if first.AllowChatMessage(now.Add(time.Second), interval) {
		t.Fatal("closed session admitted chat")
	}
}
