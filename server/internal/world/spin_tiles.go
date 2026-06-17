package world

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"capturequest/internal/pokebattle"
)

// SpinMovement represents one segment of a spin tile's forced movement.
type SpinMovement struct {
	Direction string `json:"direction"`
	Count     int    `json:"count"`
}

// SpinTile represents a floor tile that forces the player to slide.
type SpinTile struct {
	MapName   string
	X         int
	Y         int
	Movements []SpinMovement
}

// SpinTileManager loads spin/arrow tiles from the database and provides
// fast lookups by map name + tile position.
type SpinTileManager struct {
	db pokebattle.DBTX
	mu sync.RWMutex
	// byMap indexes spin tiles: mapName -> "x,y" -> SpinTile
	byMap map[string]map[string]*SpinTile
}

// NewSpinTileManager creates a new SpinTileManager.
func NewSpinTileManager(db pokebattle.DBTX) *SpinTileManager {
	return &SpinTileManager{
		db:    db,
		byMap: make(map[string]map[string]*SpinTile),
	}
}

// Load reads all spin tiles from the database into memory.
func (m *SpinTileManager) Load() {
	rows, err := m.db.Query(
		`SELECT map_name, x, y, movements FROM phaser_spin_tiles`)
	if err != nil {
		log.Printf("[SpinTiles] Failed to load: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	byMap := make(map[string]map[string]*SpinTile)
	for rows.Next() {
		var st SpinTile
		var movementsJSON string
		if err := rows.Scan(&st.MapName, &st.X, &st.Y, &movementsJSON); err != nil {
			log.Printf("[SpinTiles] Error scanning row: %v", err)
			continue
		}
		if err := json.Unmarshal([]byte(movementsJSON), &st.Movements); err != nil {
			log.Printf("[SpinTiles] Error parsing movements for %s (%d,%d): %v",
				st.MapName, st.X, st.Y, err)
			continue
		}
		if byMap[st.MapName] == nil {
			byMap[st.MapName] = make(map[string]*SpinTile)
		}
		key := fmt.Sprintf("%d,%d", st.X, st.Y)
		byMap[st.MapName][key] = &st
		count++
	}

	m.mu.Lock()
	m.byMap = byMap
	m.mu.Unlock()

	log.Printf("[SpinTiles] Loaded %d spin tiles across %d maps", count, len(byMap))
}

// CheckTile returns the spin tile at a given map+tile position, or nil if none.
func (m *SpinTileManager) CheckTile(mapName string, x, y int) *SpinTile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tiles, ok := m.byMap[mapName]
	if !ok {
		return nil
	}
	key := fmt.Sprintf("%d,%d", x, y)
	return tiles[key]
}

// ExpandMovements converts the compact SpinMovement list into a flat list of
// direction strings (e.g., ["LEFT","LEFT","UP","UP","UP"]) for the client.
func ExpandMovements(movements []SpinMovement) []string {
	var result []string
	for _, m := range movements {
		for i := 0; i < m.Count; i++ {
			result = append(result, strings.ToUpper(strings.TrimSpace(m.Direction)))
		}
	}
	return result
}
