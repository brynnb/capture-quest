package world

import (
	"log"
	"sync"

	"capturequest/internal/pokebattle"
)

// MapScript represents a single script entry for a map at a given script index.
type MapScript struct {
	ID             int
	MapID          int
	ScriptIndex    int
	ScriptLabel    string
	ScriptConstant string
	RawASM         *string // nullable — some entries have no ASM
}

// MapScriptManager loads map scripts from the database and provides
// per-map script lookup. In the MMO model, the "current script index"
// is per-player (derived from event flags), not global.
type MapScriptManager struct {
	db pokebattle.DBTX
	mu sync.RWMutex
	// scripts indexed by mapID -> scriptIndex -> MapScript
	byMap map[int]map[int]*MapScript
	// all scripts for a map, ordered by index
	ordered map[int][]*MapScript
}

// NewMapScriptManager creates a new MapScriptManager.
func NewMapScriptManager(db pokebattle.DBTX) *MapScriptManager {
	return &MapScriptManager{
		db:      db,
		byMap:   make(map[int]map[int]*MapScript),
		ordered: make(map[int][]*MapScript),
	}
}

// Load reads all map scripts from the database into memory.
func (m *MapScriptManager) Load() {
	rows, err := m.db.Query(
		`SELECT id, map_id, script_index, script_label, script_constant, raw_asm
		 FROM phaser_map_scripts WHERE map_id IS NOT NULL
		 ORDER BY map_id, script_index`)
	if err != nil {
		log.Printf("[MapScripts] Failed to load: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	byMap := make(map[int]map[int]*MapScript)
	ordered := make(map[int][]*MapScript)
	for rows.Next() {
		var s MapScript
		if err := rows.Scan(&s.ID, &s.MapID, &s.ScriptIndex, &s.ScriptLabel, &s.ScriptConstant, &s.RawASM); err != nil {
			log.Printf("[MapScripts] Error scanning row: %v", err)
			continue
		}
		if byMap[s.MapID] == nil {
			byMap[s.MapID] = make(map[int]*MapScript)
		}
		byMap[s.MapID][s.ScriptIndex] = &s
		ordered[s.MapID] = append(ordered[s.MapID], &s)
		count++
	}

	m.mu.Lock()
	m.byMap = byMap
	m.ordered = ordered
	m.mu.Unlock()

	log.Printf("[MapScripts] Loaded %d scripts across %d maps", count, len(byMap))
}

// GetScript returns the script at a given map + script index, or nil.
func (m *MapScriptManager) GetScript(mapID, scriptIndex int) *MapScript {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if scripts, ok := m.byMap[mapID]; ok {
		return scripts[scriptIndex]
	}
	return nil
}

// GetScriptsForMap returns all scripts for a map, ordered by script index.
func (m *MapScriptManager) GetScriptsForMap(mapID int) []*MapScript {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ordered[mapID]
}

// HasScripts returns true if the given map has any scripts.
func (m *MapScriptManager) HasScripts(mapID int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.ordered[mapID]) > 0
}

// ScriptCount returns the number of scripts for a map.
func (m *MapScriptManager) ScriptCount(mapID int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.ordered[mapID])
}
