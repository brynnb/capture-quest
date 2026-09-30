package session

import "sync"

// GameCornerState keeps the server's draw stable for one character map visit.
// Reopening a modal cannot reroll it. The world domain supplies the draw rule.
type GameCornerState struct {
	mu          sync.Mutex
	characterID int64
	selected    bool
	luckyIndex  int
	mapID       int
}

func (s *GameCornerState) LuckyIndex(characterID int64, mapID int, draw func() (int, error)) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.selected || s.characterID != characterID {
		index, err := draw()
		if err != nil {
			return 0, err
		}
		s.characterID, s.luckyIndex, s.selected, s.mapID = characterID, index, true, mapID
	}
	return s.luckyIndex, nil
}
func (s *GameCornerState) Clear() {
	s.mu.Lock()
	s.selected = false
	s.characterID = 0
	s.luckyIndex = 0
	s.mu.Unlock()
}

func (s *GameCornerState) ObserveMap(mapID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selected && s.mapID != mapID {
		s.selected = false
	}
}
