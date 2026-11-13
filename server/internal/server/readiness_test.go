package server

import (
	"database/sql"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReadinessRequiresStartedHealthyDatabaseAndNoDrain(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	s := &Server{database: database}
	check := func(want int) {
		t.Helper()
		response := httptest.NewRecorder()
		s.handleReadiness(response, httptest.NewRequest(http.MethodGet, "/api/ready", nil))
		if response.Code != want {
			t.Fatalf("readiness=%d want %d", response.Code, want)
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("readiness can be cached")
		}
	}
	check(http.StatusServiceUnavailable)
	s.ready.Store(true)
	check(http.StatusOK)
	s.draining.Store(true)
	check(http.StatusServiceUnavailable)
	s.draining.Store(false)
	database.Close()
	check(http.StatusServiceUnavailable)
}

func TestUnexpectedHTTPListenerFailureReachesProcessOwner(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	s.ready.Store(true)
	s.serveHTTP(listener, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(s.StopServer)
	listener.Close()
	select {
	case err := <-s.Errors():
		if err == nil {
			t.Fatal("missing serve failure")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener failure was not reported")
	}
	if !s.failed.Load() || s.ready.Load() {
		t.Fatal("listener failure did not clear readiness")
	}
	s.reportServeFailure(net.ErrClosed)
	select {
	case <-s.Errors():
		t.Fatal("duplicate failure notification")
	default:
	}
}

func TestNormalHTTPShutdownDoesNotReportListenerFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{}
	s.serveHTTP(listener, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	s.StopServer()
	select {
	case err := <-s.Errors():
		t.Fatalf("normal shutdown reported failure: %v", err)
	default:
	}
}
