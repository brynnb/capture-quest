package world

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"capturequest/internal/db"
)

// EventFlagManager manages per-character event flags with an in-memory cache
// backed by the character_event_flags table. Flags are loaded on login and
// persisted on set/reset.
type EventFlagManager struct {
	db    *sql.DB
	mu    sync.RWMutex
	flags map[int64]map[string]bool // characterID -> set of active flag names
}

// NewEventFlagManager creates a new EventFlagManager.
func NewEventFlagManager(db *sql.DB) *EventFlagManager {
	return &EventFlagManager{
		db:    db,
		flags: make(map[int64]map[string]bool),
	}
}

// LoadFlags loads all event flags for a character from the database into the cache.
// Should be called when a character enters the world.
func (m *EventFlagManager) LoadFlags(charID int64) error {
	return m.LoadFlagsContext(context.Background(), charID)
}
func (m *EventFlagManager) LoadFlagsContext(ctx context.Context, charID int64) error {
	// Serialize the read and cache replacement so a slow older read cannot replace
	// a newer snapshot. Writers refresh from committed storage, not a stale delta.
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	rows, err := m.db.QueryContext(ctx, `SELECT flag_name FROM character_event_flags WHERE character_id=$1`, charID)
	if err != nil {
		return fmt.Errorf("load event flags for character %d: %w", charID, err)
	}
	defer rows.Close()
	flags := make(map[string]bool)
	for rows.Next() {
		var flag string
		if err := rows.Scan(&flag); err != nil {
			return err
		}
		flags[flag] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	m.flags[charID] = flags
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

// SetFlag persists a flag before refreshing its cached view.
func (m *EventFlagManager) SetFlag(charID int64, flag string) error {
	return m.writeFlags(charID, func(tx db.DBTX) error { return writeEventFlag(tx, charID, flag, true) })
}
func (m *EventFlagManager) ResetFlag(charID int64, flag string) error {
	return m.writeFlags(charID, func(tx db.DBTX) error { return writeEventFlag(tx, charID, flag, false) })
}
func (m *EventFlagManager) ToggleFlag(charID int64, flag string) (bool, error) {
	var on bool
	err := m.writeFlags(charID, func(tx db.DBTX) error {
		previous, err := queryEventFlag(tx, charID, flag)
		if err != nil {
			return err
		}
		on = !previous
		return writeEventFlag(tx, charID, flag, on)
	})
	return on, err
}
func (m *EventFlagManager) writeFlags(charID int64, apply func(db.DBTX) error) error {
	err := db.Transaction(context.Background(), m.db, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		return apply(tx)
	})
	if err != nil {
		return err
	}
	return m.LoadFlags(charID)
}

func queryEventFlag(database db.DBTX, charID int64, flag string) (bool, error) {
	var on bool
	err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM character_event_flags WHERE character_id=$1 AND flag_name=$2)`, charID, flag).Scan(&on)
	return on, err
}
func writeEventFlag(database db.DBTX, charID int64, flag string, on bool) error {
	if err := db.RequireTransaction(database); err != nil {
		return err
	}
	if flag == "" {
		return fmt.Errorf("event flag is required")
	}
	var err error
	if on {
		_, err = database.Exec(`INSERT INTO character_event_flags(character_id,flag_name) VALUES($1,$2) ON CONFLICT(character_id,flag_name) DO NOTHING`, charID, flag)
	} else {
		_, err = database.Exec(`DELETE FROM character_event_flags WHERE character_id=$1 AND flag_name=$2`, charID, flag)
	}
	return err
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
func (m *EventFlagManager) SetFlagBatch(charID int64, flags []string) error {
	return m.writeFlags(charID, func(tx db.DBTX) error {
		for _, flag := range flags {
			if err := writeEventFlag(tx, charID, flag, true); err != nil {
				return err
			}
		}
		return nil
	})
}
