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
	PreviousMapID int     // Per-player source map for dynamic LAST_MAP exits
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

	playtimeMu        sync.Mutex
	playtimeStartedAt time.Time
	playtimePersisted uint32
	playtimeCharacter int32
}

// HasValidClient returns true if the session has a valid client with character data.
// Use this to guard handlers that require a logged-in character.
func (s *Session) HasValidClient() bool {
	return s.Client != nil && s.Client.CharData() != nil
}

// StartPlaytime begins tracking active play for the selected character.
func (s *Session) StartPlaytime(now time.Time, persistedSeconds uint32, characterID int32) {
	s.playtimeMu.Lock()
	s.playtimeStartedAt = now
	s.playtimePersisted = persistedSeconds
	s.playtimeCharacter = characterID
	s.playtimeMu.Unlock()
}

// CurrentPlaytime includes both persisted playtime and the active interval.
func (s *Session) CurrentPlaytime(now time.Time) uint32 {
	s.playtimeMu.Lock()
	defer s.playtimeMu.Unlock()
	return s.playtimePersisted + elapsedWholeSeconds(s.playtimeStartedAt, now)
}

// PersistPlaytime writes and then advances the whole-second persistence
// boundary while holding the per-session playtime lock. Quit and periodic
// flushes therefore cannot claim the same interval or race a rollback.
func (s *Session) PersistPlaytime(
	now time.Time,
	persist func(characterID int32, seconds uint32) error,
) (uint32, error) {
	s.playtimeMu.Lock()
	defer s.playtimeMu.Unlock()
	seconds := elapsedWholeSeconds(s.playtimeStartedAt, now)
	if seconds == 0 || s.playtimeCharacter == 0 {
		return 0, nil
	}
	if err := persist(s.playtimeCharacter, seconds); err != nil {
		return 0, err
	}
	s.playtimeStartedAt = s.playtimeStartedAt.Add(time.Duration(seconds) * time.Second)
	s.playtimePersisted += seconds
	return seconds, nil
}

// StopPlaytime pauses accumulation while the session is at character select.
func (s *Session) StopPlaytime() {
	s.playtimeMu.Lock()
	s.playtimeStartedAt = time.Time{}
	s.playtimeCharacter = 0
	s.playtimeMu.Unlock()
}

func elapsedWholeSeconds(start, now time.Time) uint32 {
	if start.IsZero() || !now.After(start) {
		return 0
	}
	return uint32(now.Sub(start) / time.Second)
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
		PreviousMapID: -1,
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

// RemoveSession atomically claims and deletes a session. It returns false when
// another disconnect path already removed the same session.
func (sm *SessionManager) RemoveSession(sessionID int) (*Session, bool) {
	sm.mu.Lock()
	sess, ok := sm.sessions[sessionID]
	if ok {
		delete(sm.sessions, sessionID)
	}
	sm.mu.Unlock()

	if !ok {
		return nil, false
	}
	// Closing transport resources can block, so it must happen after releasing
	// the manager lock. Otherwise one slow client can prevent every login.
	sess.Close()
	return sess, true
}

// UpdateMap updates the mapID for a session.
func (sm *SessionManager) UpdateMap(sessionID int, mapID int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, ok := sm.sessions[sessionID]; ok {
		session.MapID = mapID
	}
}

// ForEachSession iterates over a snapshot of active sessions. Callbacks often
// perform network writes and must never run while holding the manager lock.
func (sm *SessionManager) ForEachSession(fn func(*Session)) {
	sm.mu.RLock()
	snapshot := make([]*Session, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		snapshot = append(snapshot, session)
	}
	sm.mu.RUnlock()

	for _, session := range snapshot {
		fn(session)
	}
}
