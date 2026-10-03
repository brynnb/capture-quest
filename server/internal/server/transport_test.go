package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"capturequest/internal/api"
	"capturequest/internal/session"
	"github.com/gorilla/websocket"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

type transportTestWorld struct {
	manager *session.SessionManager
	packets chan []byte
	removed chan int
}

func (w *transportTestWorld) HandlePacket(_ *session.Session, data []byte) { w.packets <- data }
func (w *transportTestWorld) RemoveSession(id int) {
	if _, ok := w.manager.RemoveSession(id); ok {
		w.removed <- id
	}
}
func (w *transportTestWorld) ShutdownContext(context.Context) error {
	w.manager.Seal()
	w.manager.ForEachSession(func(s *session.Session) { s.Close() })
	return nil
}

func testWSServer(t *testing.T) (*Server, *transportTestWorld, string) {
	t.Helper()
	mgr := session.NewSessionManager()
	world := &transportTestWorld{mgr, make(chan []byte, 4), make(chan int, 4)}
	srv := &Server{sessionManager: mgr, worldHandler: world}
	server := httptest.NewServer(srv.makeWSHandler())
	t.Cleanup(func() {
		mgr.ForEachSession(func(s *session.Session) { mgr.RemoveSession(s.SessionID) })
		server.Close()
	})
	return srv, world, "ws" + strings.TrimPrefix(server.URL, "http")
}

func openTestWS(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatal(err)
	} // initial noop
	return conn
}

func TestWebSocketAdmissionAfterSessionSealClosesConnection(t *testing.T) {
	srv, _, url := testWSServer(t)
	srv.sessionManager.Seal()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("sealed server admitted connection")
	} else if e, ok := err.(net.Error); ok && e.Timeout() {
		t.Fatal("rejected connection was left open")
	}
	srv.sessionManager.ForEachSession(func(*session.Session) { t.Error("rejected session registered") })
}

func TestWebSocketFramesAndOversizeCleanup(t *testing.T) {
	for _, outer := range []bool{false, true} {
		t.Run(map[bool]string{false: "inner length", true: "outer message"}[outer], func(t *testing.T) {
			srv, world, url := testWSServer(t)
			conn := openTestWS(t, url)
			valid := []byte{4, 0, 0, 0, 11, 0, '{', '}'}
			// Partial application frames across WebSocket messages are supported.
			conn.WriteMessage(websocket.BinaryMessage, valid[:3])
			conn.WriteMessage(websocket.BinaryMessage, valid[3:])
			select {
			case packet := <-world.packets:
				if string(packet) != string(valid[4:]) {
					t.Fatalf("packet = %v", packet)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("valid frame not dispatched")
			}
			data := make([]byte, 4)
			binary.LittleEndian.PutUint32(data, ^uint32(0))
			if outer {
				data = make([]byte, api.MaxClientPacketSize+5)
			}
			conn.WriteMessage(websocket.BinaryMessage, data)
			select {
			case id := <-world.removed:
				if _, ok := srv.sessionManager.GetSession(id); ok {
					t.Fatal("session retained")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("oversized frame did not close session")
			}
			if _, _, err := conn.ReadMessage(); err == nil {
				t.Fatal("transport remained open")
			}
		})
	}
}

func TestWebSocketRemovalClosesOnlyOwnedConnection(t *testing.T) {
	srv, world, url := testWSServer(t)
	first := openTestWS(t, url)
	second := openTestWS(t, url)
	srv.sessionManager.RemoveSession(1)
	if _, _, err := first.ReadMessage(); err == nil {
		t.Fatal("removed connection stayed open")
	}
	if err := second.WriteMessage(websocket.BinaryMessage, []byte{4, 0, 0, 0, 11, 0, '{', '}'}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-world.packets:
	case <-time.After(2 * time.Second):
		t.Fatal("other connection was closed")
	}
}

func TestTransportsRejectSessionIDTakeoverBeforeUpgrade(t *testing.T) {
	srv := &Server{}
	for _, handler := range []http.HandlerFunc{srv.makeWSHandler(), srv.makeCaptureQuestHandler()} {
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequest(http.MethodGet, "/?sid=1", nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", response.Code)
		}
	}
}

func TestWebTransportSingleControlStreamAndCleanup(t *testing.T) {
	// Reuse Go's test certificate without starting application certificate
	// rotation or loading any local/production secrets.
	certificateServer := httptest.NewTLSServer(http.NotFoundHandler())
	tlsConfig := certificateServer.TLS.Clone()
	pool := x509.NewCertPool()
	pool.AddCert(certificateServer.Certificate())
	certificateServer.Close()
	tlsConfig.NextProtos = []string{"h3"}
	udp, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	mgr := session.NewSessionManager()
	world := &transportTestWorld{mgr, make(chan []byte, 4), make(chan int, 4)}
	srv := &Server{sessionManager: mgr, worldHandler: world, sessions: make(map[int]*webtransport.Session)}
	srv.wtServer = &webtransport.Server{H3: http3.Server{TLSConfig: tlsConfig, QUICConfig: &quic.Config{EnableDatagrams: true}, Handler: srv.makeCaptureQuestHandler()}}
	serveDone := make(chan error, 1)
	go func() { serveDone <- srv.wtServer.Serve(udp) }()
	t.Cleanup(func() {
		srv.wtServer.Close()
		select {
		case <-serveDone:
		case <-time.After(2 * time.Second):
			t.Error("WebTransport listener did not stop")
		}
	})
	dialer := &webtransport.Dialer{TLSClientConfig: &tls.Config{RootCAs: pool}}
	defer dialer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, conn, err := dialer.Dial(ctx, "https://"+udp.LocalAddr().String()+"/cq", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseWithError(0, "test done")
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stream.Write([]byte{4, 0, 0, 0, 11, 0, '{', '}'})
	select {
	case <-world.packets:
	case <-ctx.Done():
		t.Fatal("control packet not dispatched")
	}
	extra, err := conn.OpenStreamSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	extra.Write([]byte{4, 0, 0, 0, 11, 0, '{', '}'})
	select {
	case <-world.removed:
	case <-ctx.Done():
		t.Fatal("extra stream did not close connection")
	}
	select {
	case <-conn.Context().Done():
	case <-ctx.Done():
		t.Fatal("transport not closed")
	}
	select {
	case <-world.packets:
		t.Fatal("extra stream dispatched gameplay")
	default:
	}
}

type shortDeadlineConn struct {
	net.Conn
	deadlineSet chan time.Duration
}

func (c shortDeadlineConn) SetReadDeadline(deadline time.Time) error {
	if deadline.IsZero() {
		return c.Conn.SetReadDeadline(deadline)
	}
	c.deadlineSet <- time.Until(deadline)
	return c.Conn.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
}

func TestPartialFrameTimeoutClosesSession(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	mgr := session.NewSessionManager()
	world := &transportTestWorld{mgr, make(chan []byte, 1), make(chan int, 1)}
	srv := &Server{sessionManager: mgr, worldHandler: world}
	conn := shortDeadlineConn{serverConn, make(chan time.Duration, 1)}
	ses := mgr.CreateNextSession(nil, "test", conn)
	go srv.handleControlStream(ses, conn)
	if _, err := clientConn.Write([]byte{10, 0, 0, 0, 11}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-world.removed:
	case <-time.After(2 * time.Second):
		t.Fatal("partial frame did not time out")
	}
	if deadline := <-conn.deadlineSet; deadline <= 0 || deadline > clientFrameTimeout {
		t.Fatalf("invalid deadline %s", deadline)
	}
	if !ses.IsClosed() {
		t.Fatal("timed-out session still open")
	}
}

func TestShutdownClosesWebSocketBeforeBlockedHTTPDrain(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	mgr := session.NewSessionManager()
	world := &transportTestWorld{mgr, make(chan []byte, 4), make(chan int, 4)}
	srv := &Server{sessionManager: mgr, worldHandler: world}
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }); srv.StopServer() })
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", srv.makeWSHandler())
	mux.HandleFunc("/blocked", func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})
	srv.serveHTTP(listener, mux)
	conn := openTestWS(t, "ws://"+listener.Addr().String()+"/ws")
	clientDone := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String() + "/blocked")
		if resp != nil {
			resp.Body.Close()
		}
		clientDone <- err
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := srv.StopServerContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked shutdown=%v", err)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("shutdown kept websocket open")
	} else if e, ok := err.(net.Error); ok && e.Timeout() {
		t.Fatal("websocket only hit read timeout")
	}
	select {
	case <-world.removed:
	case <-time.After(time.Second):
		t.Fatal("websocket reader did not retire session")
	}
	once.Do(func() { close(release) })
	if err := <-clientDone; err != nil {
		t.Fatal(err)
	}
	if err := srv.StopServerContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}
