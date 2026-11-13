package world

import (
	"sync"
	"time"
)

// periodicWorker owns one timer and joins its running callback before stop
// returns. It cannot restart after stop, including stop before first start.
// Callbacks must not call stop on their own worker.
type periodicWorker struct {
	mu      sync.Mutex
	stopCh  chan struct{}
	done    chan struct{}
	stopped bool
}

func (w *periodicWorker) start(interval time.Duration, initialize, tick func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stopped || w.done != nil {
		return
	}
	if initialize != nil {
		initialize()
	}
	w.stopCh, w.done = make(chan struct{}), make(chan struct{})
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-w.stopCh:
				return
			case <-ticker.C:
				// If a tick and stop became ready together, prefer stopping.
				select {
				case <-w.stopCh:
					return
				default:
				}
				tick()
			}
		}
	}()
}

func (w *periodicWorker) stop() {
	w.mu.Lock()
	if !w.stopped {
		w.stopped = true
		if w.stopCh != nil {
			close(w.stopCh)
		}
	}
	done := w.done
	w.mu.Unlock()
	if done != nil {
		<-done
	}
}
