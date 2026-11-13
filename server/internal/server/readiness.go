package server

import (
	"context"
	"net/http"
	"time"
)

// Errors reports the first unexpected listener failure to the process owner.
// The buffered channel never blocks a serving goroutine during shutdown.
func (s *Server) Errors() <-chan error {
	s.failureOnce.Do(func() { s.failures = make(chan error, 1) })
	return s.failures
}

func (s *Server) reportServeFailure(err error) {
	if err == nil || s.draining.Load() {
		return
	}
	s.Errors()
	if s.failed.CompareAndSwap(false, true) {
		s.ready.Store(false)
		s.failures <- err
	}
}

func (s *Server) handleReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.ready.Load() || s.failed.Load() || s.draining.Load() || s.database == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if err := s.database.PingContext(ctx); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready\n"))
}
