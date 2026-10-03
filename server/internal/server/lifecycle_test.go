package server

import (
	"context"
	"database/sql"
	"errors"
	"io"
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
	requestCancelled := make(chan struct{})
	s.serveHTTP(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(requestCancelled)
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
	case <-requestCancelled:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel HTTP request")
	}
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

func TestHTTPForceCloseInterruptsBodyReadButJoinsHandlerBeforeStorageClose(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{database: database, gracePeriod: 20 * time.Millisecond}
	entered, release := make(chan struct{}), make(chan struct{})
	bodyResult, queryResult := make(chan error, 1), make(chan error, 1)
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }); s.StopServer() })
	s.serveHTTP(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		_, err := io.ReadAll(r.Body)
		bodyResult <- err
		<-release
		queryResult <- database.Ping()
	}))
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "POST /slow HTTP/1.1\r\nHost: localhost\r\nContent-Length: 100\r\n\r\nx"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("body handler not started")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- s.StopServerContext(context.Background()) }()
	select {
	case err := <-bodyResult:
		if err == nil {
			t.Fatal("partial body accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not interrupt body read")
	}
	select {
	case err := <-stopped:
		t.Fatalf("force close failed to join handler: %v", err)
	default:
	}
	if err := database.Ping(); err != nil {
		t.Fatal("storage closed under interrupted handler")
	}
	once.Do(func() { close(release) })
	if err := <-queryResult; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-stopped:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("forced drain result=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("released drain did not complete")
	}
	if err := database.Ping(); err == nil {
		t.Fatal("joined storage remained open")
	}
}
