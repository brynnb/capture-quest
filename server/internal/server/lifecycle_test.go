package server

import (
	"database/sql"
	"net"
	"net/http"
	"sync"
	"testing"

	"capturequest/internal/session"
	_ "modernc.org/sqlite"
)

type lifecycleTestWorld struct{ shutdown func() }

func (*lifecycleTestWorld) HandlePacket(*session.Session, []byte) {}
func (*lifecycleTestWorld) RemoveSession(int)                     {}
func (w *lifecycleTestWorld) Shutdown()                           { w.shutdown() }

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
