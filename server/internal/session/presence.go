package session

// Presence is the immutable projection used by other players and world timers.
// It contains values only, never a pointer into the mutable character model.
type Presence struct {
	SessionID              int
	Authenticated          bool
	MapID                  int
	X, Y                   float32
	CharacterID            uint32
	Name                   string
	Gender                 uint8
	CharacterMapID         uint32
	CharacterX, CharacterY float64
}

// PublishPresence must run inside the session gate (or before exposing a new
// session). Intermediate mutations remain private until publication.
func (s *Session) PublishPresence() Presence {
	p := Presence{SessionID: s.SessionID, Authenticated: s.Authenticated, MapID: s.MapID, X: s.X, Y: s.Y}
	if s.HasValidClient() {
		c := s.Client.CharData()
		p.CharacterID, p.Name, p.Gender = c.ID, c.Name, c.Gender
		p.CharacterMapID, p.CharacterX, p.CharacterY = c.MapID, c.X, c.Y
	}
	s.GameCorner.ObserveMap(int(p.CharacterMapID))
	s.presence.Store(&p)
	return p
}

// Presence can be read without entering another session's command gate. Closed
// sessions disappear immediately, even while their cleanup is still draining.
func (s *Session) Presence() Presence {
	if s.IsClosed() {
		return Presence{}
	}
	if p := s.presence.Load(); p != nil {
		return *p
	}
	return Presence{}
}
