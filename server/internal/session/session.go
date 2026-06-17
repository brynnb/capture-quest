package session

import (
	"fmt"
	"io"
	"sync"
	"time"

	entity "capturequest/internal/zone/interface"
)

type ClientMessenger interface {
	SendDatagram(sessionID int, data []byte) error
	SendStream(sessionID int, data []byte) error
}

// Session holds the context for a client session.
type Session struct {
	SessionID     int
	Authenticated bool
	AccountID     int64
	MapID         int     // Current map the session is in
	X             float32 // Current X coordinate
	Y             float32 // Current Y coordinate
	InstanceID    int     // Current instance ID the session is in
	IP            string  // Client IP address
	CharacterName string
	Client        entity.Client
	Messenger     ClientMessenger // For sending replies
	ControlStream io.ReadWriteCloser
	LastHeartbeat time.Time
	// Private

	sendMu   sync.Mutex
	closed   bool
	closedMu sync.RWMutex
}

// HasValidClient returns true if the session has a valid client with character data.
// Use this to guard handlers that require a logged-in character.
func (s *Session) HasValidClient() bool {
	return s.Client != nil && s.Client.CharData() != nil
}

// SessionManager manages active sessions.
type SessionManager struct {
	sessions map[int]*Session // sessionID -> Session
	mu       sync.RWMutex
}

// globalSessionManager holds the singleton SessionManager.
var globalSessionManager *SessionManager

func GetActiveSessionCount() int {
	sm := GetSessionManager()
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

// InitSessionManager initializes the global SessionManager.
func InitSessionManager(sm *SessionManager) {
	globalSessionManager = sm
}

// GetSessionManager returns the global SessionManager.
func GetSessionManager() *SessionManager {
	if globalSessionManager == nil {
		panic("SessionManager not initialized")
	}
	return globalSessionManager
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[int]*Session),
	}
}

func (sm *SessionManager) GetValidSession(sessionID int, ip string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	if session.IP != ip {
		return nil, fmt.Errorf("IP mismatch")
	}
	return session, nil
}

// CreateSession initializes a new session with the given sessionID and accountID.
func (sm *SessionManager) CreateSession(messenger ClientMessenger, sessionID int, ip string, stream io.ReadWriteCloser) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session := &Session{
		SessionID:     sessionID,
		Authenticated: false,
		MapID:         -1,
		InstanceID:    0,
		ControlStream: stream,
		IP:            ip,
		Messenger:     messenger,
	}
	sm.sessions[sessionID] = session
	return session
}

func (s *Session) Close() {
	s.closedMu.Lock()
	s.closed = true
	s.closedMu.Unlock()

	if closer, ok := s.Messenger.(io.Closer); ok {
		_ = closer.Close()
	}
}

// GetSession retrieves a session by sessionID.
func (sm *SessionManager) GetSession(sessionID int) (*Session, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, ok := sm.sessions[sessionID]
	return session, ok
}

// RemoveSession deletes a session by sessionID.
func (sm *SessionManager) RemoveSession(sessionID int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sess, ok := sm.sessions[sessionID]; ok {
		sess.Close() // free up the pools
		delete(sm.sessions, sessionID)
	}
}

// UpdateMap updates the mapID for a session.
func (sm *SessionManager) UpdateMap(sessionID int, mapID int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[sessionID]; ok {
		session.MapID = mapID
	}
}

// ForEachSession iterates over all active sessions and calls the provided function for each.
func (sm *SessionManager) ForEachSession(fn func(*Session)) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, session := range sm.sessions {
		fn(session)
	}
}
