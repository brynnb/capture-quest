package server

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"capturequest/internal/api"

	"github.com/gorilla/websocket"
)

// wsUpgrader handles HTTP -> WebSocket upgrade with permissive origin check.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSConn wraps a gorilla/websocket.Conn to implement io.ReadWriteCloser
// using binary messages with the same length-prefixed framing as the
// WebTransport control stream.
type WSConn struct {
	conn *websocket.Conn
	mu   sync.Mutex // serialise writes
	buf  []byte     // leftover from partial reads
}

// Read implements io.Reader. It reads from WebSocket binary messages,
// buffering across calls so the length-prefixed frame reader in
// handleControlStream works unchanged.
func (w *WSConn) Read(p []byte) (int, error) {
	for len(w.buf) == 0 {
		mt, msg, err := w.conn.ReadMessage()
		if err != nil {
			return 0, err
		}
		if mt != websocket.BinaryMessage {
			continue // skip non-binary (e.g. ping/pong text)
		}
		w.buf = msg
	}
	n := copy(p, w.buf)
	w.buf = w.buf[n:]
	return n, nil
}

// Write implements io.Writer. Each call sends one binary WebSocket message.
func (w *WSConn) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return 0, err
	}
	err := w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *WSConn) SetReadDeadline(deadline time.Time) error {
	return w.conn.SetReadDeadline(deadline)
}

// Close implements io.Closer.
func (w *WSConn) Close() error {
	return w.conn.Close()
}

// wsMessenger implements session.ClientMessenger for WebSocket sessions.
// Since WebSocket is reliable (TCP), both SendDatagram and SendStream
// write to the same WebSocket connection using the control-stream framing.
type wsMessenger struct {
	sessions   map[int]*WSConn
	sessionsMu sync.Mutex
}

func newWSMessenger() *wsMessenger {
	return &wsMessenger{sessions: make(map[int]*WSConn)}
}

func (m *wsMessenger) add(id int, conn *WSConn) {
	m.sessionsMu.Lock()
	m.sessions[id] = conn
	m.sessionsMu.Unlock()
}

func (m *wsMessenger) CloseSession(id int) error {
	m.sessionsMu.Lock()
	conn := m.sessions[id]
	delete(m.sessions, id)
	m.sessionsMu.Unlock()
	if conn != nil {
		return conn.Close()
	}
	return nil
}

// SendDatagram sends a datagram-style message over WebSocket.
// Format: [opcode:uint16_LE][payload] (same as WebTransport datagram)
func (m *wsMessenger) SendDatagram(sessionID int, data []byte) error {
	m.sessionsMu.Lock()
	conn, ok := m.sessions[sessionID]
	m.sessionsMu.Unlock()
	if !ok {
		return fmt.Errorf("ws session %d not found", sessionID)
	}
	_, err := conn.Write(data)
	return err
}

// SendStream sends a stream-style message over WebSocket.
// Format: [length:uint32_LE][opcode:uint16_LE][payload] (same as control stream)
func (m *wsMessenger) SendStream(sessionID int, data []byte) error {
	m.sessionsMu.Lock()
	conn, ok := m.sessions[sessionID]
	m.sessionsMu.Unlock()
	if !ok {
		return fmt.Errorf("ws session %d not found", sessionID)
	}
	_, err := conn.Write(data)
	return err
}

// makeWSHandler returns an http.HandlerFunc that upgrades to WebSocket
// and creates sessions compatible with the existing world handler.
func (s *Server) makeWSHandler() http.HandlerFunc {
	messenger := newWSMessenger()

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sid") != "" && r.URL.Query().Get("sid") != "0" {
			http.Error(w, "Reconnect requires authentication on a new connection", http.StatusBadRequest)
			return
		}
		wsConn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Limit the outer WebSocket message as well as the inner frame, before
		// ReadMessage allocates the complete message. The browser sends one
		// framed command per message; fragmented messages remain supported.
		wsConn.SetReadLimit(api.MaxClientPacketSize + 4)
		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		wsc := &WSConn{conn: wsConn}
		sessObj := s.sessionManager.CreateNextSession(messenger, clientIP, wsc)
		messenger.add(sessObj.SessionID, wsc)
		log.Printf("[WS] New session %d", sessObj.SessionID)
		initialFrame := make([]byte, 6)
		binary.LittleEndian.PutUint32(initialFrame[0:4], 2)
		if _, err := wsc.Write(initialFrame); err != nil {
			s.handleSessionClose(sessObj.SessionID)
			return
		}
		go s.handleControlStream(sessObj, wsc)
	}
}

// registerWSHandler adds the /ws endpoint to the given mux.
func (s *Server) registerWSHandler(mux *http.ServeMux) {
	mux.Handle("/ws", corsMiddleware(http.HandlerFunc(s.makeWSHandler())))
}
