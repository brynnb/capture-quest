package server

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"

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
	err := w.conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
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

func (m *wsMessenger) remove(id int) {
	m.sessionsMu.Lock()
	delete(m.sessions, id)
	m.sessionsMu.Unlock()
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
	var nextID int
	var nextIDMu sync.Mutex

	return func(w http.ResponseWriter, r *http.Request) {
		wsConn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[WS] Upgrade error: %v", err)
			return
		}

		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		params := r.URL.Query()

		wsc := &WSConn{conn: wsConn}

		// Try reconnect
		var sessObj *session.Session
		if sidStr := params.Get("sid"); sidStr != "" && sidStr != "0" {
			if sid, e := strconv.Atoi(sidStr); e == nil {
				if existing, e2 := s.sessionManager.GetValidSession(sid, clientIP); e2 == nil {
					log.Printf("[WS] Reconnecting session %d from %s", sid, clientIP)
					sessObj = existing
					existing.Messenger = messenger
					existing.ControlStream = wsc
					messenger.add(sid, wsc)
					existing.SendJSON(map[string]interface{}{}, opcodes.Reconnect)
				}
			}
		}

		// New session
		if sessObj == nil {
			nextIDMu.Lock()
			nextID++
			sid := nextID + 100000 // Offset to avoid collision with WebTransport session IDs
			nextIDMu.Unlock()

			messenger.add(sid, wsc)
			sessObj = s.sessionManager.CreateSession(messenger, sid, clientIP, wsc)
			log.Printf("[WS] New session %d from %s", sid, clientIP)

			// Send initial noop frame (same as WebTransport)
			initialFrame := make([]byte, 6)
			binary.LittleEndian.PutUint32(initialFrame[0:4], 2)
			binary.LittleEndian.PutUint16(initialFrame[4:6], 0)
			if _, err := wsc.Write(initialFrame); err != nil {
				log.Printf("[WS] Failed to send initial frame: %v", err)
			}
		}

		sid := sessObj.SessionID

		// Read loop — reuse handleControlStream logic
		// The WSConn.Read() method returns data from WebSocket binary messages,
		// so the existing length-prefixed frame parser works unchanged.
		go func() {
			defer func() {
				messenger.remove(sid)
				s.handleSessionClose(sid)
			}()
			s.handleControlStream(sessObj, wsc, sid, clientIP)
		}()
	}
}

// registerWSHandler adds the /ws endpoint to the given mux.
func (s *Server) registerWSHandler(mux *http.ServeMux) {
	mux.Handle("/ws", corsMiddleware(http.HandlerFunc(s.makeWSHandler())))
}
