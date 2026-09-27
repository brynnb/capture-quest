package world

import (
	"sync"
	"testing"
	"time"

	"capturequest/internal/session"
)

func TestPeriodicWorkerStopsOnceAndJoinsRunningCallback(t *testing.T) {
	var w periodicWorker
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }); w.stop() })
	initializations := 0
	w.start(time.Millisecond, func() { initializations++ }, func() { close(entered); <-release })
	w.start(time.Millisecond, func() { initializations++ }, func() { t.Error("duplicate worker ran") })
	<-entered
	if initializations != 1 {
		t.Fatal("initialized worker twice")
	}
	stopped := make(chan struct{})
	go func() { w.stop(); close(stopped) }()
	<-w.stopCh
	select {
	case <-stopped:
		t.Fatal("stop returned during callback")
	default:
	}
	secondStop := make(chan struct{})
	go func() { w.stop(); close(secondStop) }()
	once.Do(func() { close(release) })
	<-stopped
	<-secondStop
	w.start(time.Millisecond, func() { t.Error("restarted stopped worker") }, func() { t.Error("tick after stop") })
}

func TestPeriodicWorkerStopBeforeStart(t *testing.T) {
	var w periodicWorker
	w.stop()
	w.stop()
	w.start(time.Millisecond, func() { t.Error("initialized after stop") }, func() { t.Error("tick after stop") })
}

func TestWorldShutdownJoinsEveryPeriodicWorker(t *testing.T) {
	for _, kind := range []string{"timeout", "movement", "actors"} {
		t.Run(kind, func(t *testing.T) {
			wh := &WorldHandler{sessionManager: session.NewSessionManager(), PlayerMovement: &PlayerMovementManager{}, ActorManager: &PhaserActorManager{}}
			w := &wh.timeoutWorker
			if kind == "movement" {
				w = &wh.PlayerMovement.worker
			}
			if kind == "actors" {
				w = &wh.ActorManager.worker
			}
			entered, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { once.Do(func() { close(release) }); wh.Shutdown() })
			w.start(time.Millisecond, nil, func() { close(entered); <-release })
			<-entered
			go func() { wh.Shutdown(); close(stopped) }()
			<-w.stopCh
			select {
			case <-stopped:
				t.Fatal("shutdown did not join callback")
			default:
			}
			once.Do(func() { close(release) })
			<-stopped
		})
	}
}
