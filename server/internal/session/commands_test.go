package session

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

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
