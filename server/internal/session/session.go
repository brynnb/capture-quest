package session

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	entity "capturequest/internal/zone/interface"
)

type ClientMessenger interface {
	SendDatagram(sessionID int, data []byte) error
	SendStream(sessionID int, data []byte) error
}

// Session holds the context for a client session.
type Session struct {
	presence      atomic.Pointer[Presence]
	commands      commandGate
	GameCorner    GameCornerState
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
	// Private

	sendMu        sync.Mutex
	closed        bool
	closedMu      sync.RWMutex
	controlStream io.ReadWriteCloser // protected by closedMu; attached once
	lastHeartbeat atomic.Int64
	chatMu        sync.Mutex
	lastChatSent  time.Time

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

// PersistPlaytime writes the cumulative character total, then advances the
// whole-second boundary. The exclusively owned character was loaded after the
// previous owner drained. A cumulative write is safe to repeat even when a
// commit succeeded but its acknowledgement was lost; an additive write is not.
func (s *Session) PersistPlaytime(
	now time.Time,
	persist func(characterID int32, totalSeconds uint32) error,
) (uint32, error) {
	s.playtimeMu.Lock()
	defer s.playtimeMu.Unlock()
	seconds := elapsedWholeSeconds(s.playtimeStartedAt, now)
	if seconds == 0 || s.playtimeCharacter == 0 {
		return 0, nil
	}
	if err := persist(s.playtimeCharacter, s.playtimePersisted+seconds); err != nil {
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

// FinishPlaytime retires the tracker and returns an immutable final cumulative
// save. A disconnected owner's retry must not accrue time while waiting to recover.
func (s *Session) FinishPlaytime(now time.Time) (characterID int32, totalSeconds uint32) {
	s.playtimeMu.Lock()
	defer s.playtimeMu.Unlock()
	seconds := elapsedWholeSeconds(s.playtimeStartedAt, now)
	if seconds > 0 {
		characterID = s.playtimeCharacter
	}
	s.playtimePersisted += seconds
	totalSeconds = s.playtimePersisted
	s.playtimeStartedAt = time.Time{}
	s.playtimeCharacter = 0
	return
}

func elapsedWholeSeconds(start, now time.Time) uint32 {
	if start.IsZero() || !now.After(start) {
		return 0
	}
	return uint32(now.Sub(start) / time.Second)
}

// SessionManager manages active sessions.
type SessionManager struct {
	sealed   bool
	sessions map[int]*Session // sessionID -> Session
	mu       sync.RWMutex
	nextID   int
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

// CreateNextSession uses one ID sequence across all transports. Session IDs
// identify connections only; reconnecting always requires fresh authentication.
func (sm *SessionManager) CreateNextSession(messenger ClientMessenger, ip string, stream io.ReadWriteCloser) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.sealed {
		return nil
	}
	sm.nextID++
	return sm.createSession(messenger, sm.nextID, ip, stream)
}

// CreateSession initializes a new session with the given sessionID and accountID.
func (sm *SessionManager) CreateSession(messenger ClientMessenger, sessionID int, ip string, stream io.ReadWriteCloser) *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.sealed {
		return nil
	}
	if sessionID > sm.nextID {
		sm.nextID = sessionID
	}
	return sm.createSession(messenger, sessionID, ip, stream)
}

// Seal prevents new admissions before a shutdown snapshot is drained.
func (sm *SessionManager) Seal() {
	sm.mu.Lock()
	sm.sealed = true
	sm.mu.Unlock()
}

func (sm *SessionManager) createSession(messenger ClientMessenger, sessionID int, ip string, stream io.ReadWriteCloser) *Session {
	session := &Session{
		SessionID:     sessionID,
		Authenticated: false,
		MapID:         -1,
		PreviousMapID: -1,
		InstanceID:    0,
		controlStream: stream,
		IP:            ip,
		Messenger:     messenger,
	}
	session.RecordHeartbeat(time.Now())
	sm.sessions[sessionID] = session
	return session
}

func (s *Session) Close() {
	s.closedMu.Lock()
	if s.closed {
		s.closedMu.Unlock()
		return
	}
	s.closed = true
	s.GameCorner.Clear()
	stream := s.controlStream
	s.closedMu.Unlock()
	s.commands.init()
	s.commands.stop()

	// Messengers can serve several sessions, so close only this connection.
	if closer, ok := s.Messenger.(interface{ CloseSession(int) error }); ok {
		_ = closer.CloseSession(s.SessionID)
	}
	if stream != nil {
		_ = stream.Close()
	}
}

func (s *Session) IsClosed() bool {
	s.closedMu.RLock()
	defer s.closedMu.RUnlock()
	return s.closed
}

func (s *Session) AttachControlStream(stream io.ReadWriteCloser) bool {
	s.closedMu.Lock()
	defer s.closedMu.Unlock()
	if s.closed || s.controlStream != nil {
		return false
	}
	s.controlStream = stream
	return true
}

func (s *Session) WriteControlStream(data []byte) error {
	s.sendMu.Lock()
	defer s.sendMu.Unlock()
	s.closedMu.RLock()
	stream, closed := s.controlStream, s.closed
	s.closedMu.RUnlock()
	if closed || stream == nil {
		return fmt.Errorf("session %d has no open control stream", s.SessionID)
	}
	if deadlineStream, ok := stream.(interface{ SetWriteDeadline(time.Time) error }); ok {
		if err := deadlineStream.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return err
		}
	}
	n, err := stream.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func (s *Session) RecordHeartbeat(now time.Time) {
	s.lastHeartbeat.Store(now.UnixNano())
}

func (s *Session) LastHeartbeat() time.Time {
	n := s.lastHeartbeat.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
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

// AllowChatMessage stores throttle state on its connection owner; replacing a
// session cannot inherit a reused numeric session ID's timestamp.
func (s *Session) AllowChatMessage(now time.Time, interval time.Duration) bool {
	if s.IsClosed() {
		return false
	}
	s.chatMu.Lock()
	defer s.chatMu.Unlock()
	// Retain time.Now's monotonic clock, as the original throttle did. Unix
	// timestamps could block chat after a wall-clock adjustment.
	if !s.lastChatSent.IsZero() && now.Sub(s.lastChatSent) < interval {
		return false
	}
	s.lastChatSent = now
	return true
}
