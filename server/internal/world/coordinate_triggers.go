package world

import (
	"fmt"
	"log"
	"sync"

	"capturequest/internal/pokebattle"
)

// CoordinateTrigger represents a tile that fires a script when stepped on.
type CoordinateTrigger struct {
	ID      int
	MapID   int
	MapName string
	Label   string
	X       int
	Y       int
}

// CoordinateTriggerManager loads coordinate triggers from the database and
// provides fast lookups by map+tile position.
type CoordinateTriggerManager struct {
	db pokebattle.DBTX
	mu sync.RWMutex
	// triggers indexed by mapID -> "x,y" -> trigger rows
	byMap map[int]map[string][]CoordinateTrigger
}

// NewCoordinateTriggerManager creates a new CoordinateTriggerManager.
func NewCoordinateTriggerManager(db pokebattle.DBTX) *CoordinateTriggerManager {
	return &CoordinateTriggerManager{
		db:    db,
		byMap: make(map[int]map[string][]CoordinateTrigger),
	}
}

// Load reads all coordinate triggers from the database into memory.
func (m *CoordinateTriggerManager) Load() {
	rows, err := m.db.Query(
		`SELECT id, map_id, map_name, label, x, y FROM phaser_coordinate_triggers WHERE map_id IS NOT NULL`)
	if err != nil {
		log.Printf("[CoordTriggers] Failed to load: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	byMap := make(map[int]map[string][]CoordinateTrigger)
	for rows.Next() {
		var t CoordinateTrigger
		if err := rows.Scan(&t.ID, &t.MapID, &t.MapName, &t.Label, &t.X, &t.Y); err != nil {
			log.Printf("[CoordTriggers] Error scanning row: %v", err)
			continue
		}
		if byMap[t.MapID] == nil {
			byMap[t.MapID] = make(map[string][]CoordinateTrigger)
		}
		key := tileKey(t.X, t.Y)
		byMap[t.MapID][key] = append(byMap[t.MapID][key], t)
		count++
	}

	m.mu.Lock()
	m.byMap = byMap
	m.mu.Unlock()

	log.Printf("[CoordTriggers] Loaded %d coordinate triggers across %d maps", count, len(byMap))
}

// CheckTileTriggers returns coordinate trigger rows at a given map+tile position, or nil if none.
func (m *CoordinateTriggerManager) CheckTileTriggers(mapID, x, y int) []CoordinateTrigger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tiles, ok := m.byMap[mapID]
	if !ok {
		return nil
	}
	triggers := tiles[tileKey(x, y)]
	if len(triggers) == 0 {
		return nil
	}
	result := make([]CoordinateTrigger, len(triggers))
	copy(result, triggers)
	return result
}

// CheckTile returns the trigger labels at a given map+tile position, or nil if none.
func (m *CoordinateTriggerManager) CheckTile(mapID, x, y int) []string {
	triggers := m.CheckTileTriggers(mapID, x, y)
	if len(triggers) == 0 {
		return nil
	}
	labels := make([]string, len(triggers))
	for i, trigger := range triggers {
		labels[i] = trigger.Label
	}
	return labels
}

// tileKey returns a string key for a tile coordinate.
func tileKey(x, y int) string {
	return fmt.Sprintf("%d,%d", x, y)
}
