package world

import (
	"fmt"
	"log"
	"sync"

	"capturequest/internal/pokebattle"
)

// EventFlagManager manages per-character event flags with an in-memory cache
// backed by the character_event_flags table. Flags are loaded on login and
// persisted on set/reset.
type EventFlagManager struct {
	db    pokebattle.DBTX
	mu    sync.RWMutex
	flags map[int64]map[string]bool // characterID -> set of active flag names
}

// NewEventFlagManager creates a new EventFlagManager.
func NewEventFlagManager(db pokebattle.DBTX) *EventFlagManager {
	return &EventFlagManager{
		db:    db,
		flags: make(map[int64]map[string]bool),
	}
}

// LoadFlags loads all event flags for a character from the database into the cache.
// Should be called when a character enters the world.
func (m *EventFlagManager) LoadFlags(charID int64) error {
	rows, err := m.db.Query(
		`SELECT flag_name FROM character_event_flags WHERE character_id = $1`, charID)
	if err != nil {
		return fmt.Errorf("load event flags for char %d: %w", charID, err)
	}
	defer rows.Close()

	flagSet := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		flagSet[name] = true
	}

	m.mu.Lock()
	m.flags[charID] = flagSet
	m.mu.Unlock()

	log.Printf("[EventFlags] Loaded %d flags for char %d", len(flagSet), charID)
	return nil
}

// UnloadFlags removes a character's flags from the cache.
// Should be called when a character leaves the world.
func (m *EventFlagManager) UnloadFlags(charID int64) {
	m.mu.Lock()
	delete(m.flags, charID)
	m.mu.Unlock()
}

// CheckFlag returns true if the given flag is set for the character.
func (m *EventFlagManager) CheckFlag(charID int64, flagName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if fs, ok := m.flags[charID]; ok {
		return fs[flagName]
	}
	return false
}

// SetFlag sets a flag for the character (persists to DB).
func (m *EventFlagManager) SetFlag(charID int64, flagName string) error {
	m.mu.Lock()
	if m.flags[charID] == nil {
		m.flags[charID] = make(map[string]bool)
	}
	if m.flags[charID][flagName] {
		m.mu.Unlock()
		return nil // Already set
	}
	m.flags[charID][flagName] = true
	m.mu.Unlock()

	_, err := m.db.Exec(
		`INSERT INTO character_event_flags (character_id, flag_name)
		VALUES ($1, $2)
		ON CONFLICT (character_id, flag_name) DO NOTHING`,
		charID, flagName)
	if err != nil {
		return fmt.Errorf("set event flag %s for char %d: %w", flagName, charID, err)
	}
	log.Printf("[EventFlags] Set %s for char %d", flagName, charID)
	return nil
}

// ResetFlag clears a flag for the character (removes from DB).
func (m *EventFlagManager) ResetFlag(charID int64, flagName string) error {
	m.mu.Lock()
	if fs, ok := m.flags[charID]; ok {
		delete(fs, flagName)
	}
	m.mu.Unlock()

	_, err := m.db.Exec(
		`DELETE FROM character_event_flags WHERE character_id = $1 AND flag_name = $2`,
		charID, flagName)
	if err != nil {
		return fmt.Errorf("reset event flag %s for char %d: %w", flagName, charID, err)
	}
	log.Printf("[EventFlags] Reset %s for char %d", flagName, charID)
	return nil
}

// ToggleFlag flips a flag for the character and returns true when the flag is set after toggling.
func (m *EventFlagManager) ToggleFlag(charID int64, flagName string) (bool, error) {
	if m.CheckFlag(charID, flagName) {
		if err := m.ResetFlag(charID, flagName); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := m.SetFlag(charID, flagName); err != nil {
		return false, err
	}
	return true, nil
}

// GetAllFlags returns a copy of all set flags for a character.
func (m *EventFlagManager) GetAllFlags(charID int64) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fs, ok := m.flags[charID]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(fs))
	for name := range fs {
		result = append(result, name)
	}
	return result
}

// SetFlagBatch sets multiple flags at once (e.g., after defeating a trainer).
func (m *EventFlagManager) SetFlagBatch(charID int64, flagNames []string) error {
	for _, name := range flagNames {
		if err := m.SetFlag(charID, name); err != nil {
			return err
		}
	}
	return nil
}
