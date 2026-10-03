package server

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"capturequest/internal/session"
	_ "modernc.org/sqlite"
)

type lifecycleTestWorld struct {
	shutdown func()
	failure  error
}

func (*lifecycleTestWorld) HandlePacket(*session.Session, []byte)   {}
func (*lifecycleTestWorld) RemoveSession(int)                       {}
func (w *lifecycleTestWorld) ShutdownContext(context.Context) error { w.shutdown(); return w.failure }

func TestServerShutdownDrainsHTTPAndWorldBeforeClosingDatabase(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered, release, stopping := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	s := &Server{sessionManager: session.NewSessionManager(), database: database}
	worldCalls := 0
	s.worldHandler = &lifecycleTestWorld{shutdown: func() {
		worldCalls++
		if err := database.Ping(); err != nil {
			t.Errorf("database closed before world drain: %v", err)
		}
	}}
	t.Cleanup(func() { once.Do(func() { close(release) }); s.StopServer() })
	handlerResult := make(chan error, 1)
	s.serveHTTP(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		handlerResult <- database.Ping()
		w.WriteHeader(http.StatusNoContent)
	}))
	s.httpServer.RegisterOnShutdown(func() { close(stopping) })
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := http.Get("http://" + listener.Addr().String())
		if err != nil {
			t.Error(err)
			return
		}
		resp.Body.Close()
	}()
	<-entered
	stopped := make(chan struct{})
	go func() { s.StopServer(); close(stopped) }()
	<-stopping
	select {
	case <-stopped:
		t.Fatal("shutdown returned during HTTP request")
	default:
	}
	if err := database.Ping(); err != nil {
		t.Fatal("database closed while HTTP handler active")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := s.StopServerContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked handler shutdown=%v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatal("timeout closed storage under active handler")
	}
	once.Do(func() { close(release) })
	if err := <-handlerResult; err != nil {
		t.Fatalf("active HTTP query failed: %v", err)
	}
	<-clientDone
	<-stopped
	s.StopServer()
	if worldCalls != 1 {
		t.Fatalf("world shutdown calls=%d", worldCalls)
	}
	if err := database.Ping(); err == nil {
		t.Fatal("database not closed after drain")
	}
	if err := s.StartServer(); err == nil {
		t.Fatal("server restarted after shutdown")
	}
}

func TestShutdownReturnsWorldFailureAfterClosingJoinedStorage(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	failure := errors.New("final persistence failed")
	s := &Server{database: database, worldHandler: &lifecycleTestWorld{shutdown: func() {}, failure: failure}}
	for i := 0; i < 2; i++ {
		if err := s.StopServerContext(context.Background()); !errors.Is(err, failure) {
			t.Fatalf("shutdown failure=%v", err)
		}
	}
	if err := database.Ping(); err == nil {
		t.Fatal("joined storage remained open")
	}
}
