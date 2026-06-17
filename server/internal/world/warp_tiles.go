package world

import (
	"fmt"
	"log"
	"sync"

	"capturequest/internal/pokebattle"
)

// WarpTile represents a teleporter pad that auto-triggers when stepped on.
// When a player steps on this tile, they are teleported to the destination.
type WarpTile struct {
	SourceMapID int
	X           int
	Y           int
	DestMapID   int
	DestX       int
	DestY       int
}

// WarpTileManager loads warp pad tiles from the database and provides
// fast lookups by map ID + tile position.
type WarpTileManager struct {
	db pokebattle.DBTX
	mu sync.RWMutex
	// byMap indexes warp tiles: mapID -> "x,y" -> WarpTile
	byMap map[int]map[string]*WarpTile
}

// NewWarpTileManager creates a new WarpTileManager.
func NewWarpTileManager(db pokebattle.DBTX) *WarpTileManager {
	return &WarpTileManager{
		db:    db,
		byMap: make(map[int]map[string]*WarpTile),
	}
}

// Load reads all warp tiles from the database into memory.
func (m *WarpTileManager) Load() {
	rows, err := m.db.Query(
		`SELECT source_map_id, x, y, destination_map_id, destination_x, destination_y FROM phaser_warp_tiles`)
	if err != nil {
		log.Printf("[WarpTiles] Failed to load: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	byMap := make(map[int]map[string]*WarpTile)
	for rows.Next() {
		var wt WarpTile
		if err := rows.Scan(&wt.SourceMapID, &wt.X, &wt.Y, &wt.DestMapID, &wt.DestX, &wt.DestY); err != nil {
			log.Printf("[WarpTiles] Error scanning row: %v", err)
			continue
		}
		if byMap[wt.SourceMapID] == nil {
			byMap[wt.SourceMapID] = make(map[string]*WarpTile)
		}
		key := fmt.Sprintf("%d,%d", wt.X, wt.Y)
		byMap[wt.SourceMapID][key] = &wt
		count++
	}

	m.mu.Lock()
	m.byMap = byMap
	m.mu.Unlock()

	log.Printf("[WarpTiles] Loaded %d warp tiles across %d maps", count, len(byMap))
}

// CheckTile returns the warp tile at a given map ID + tile position, or nil if none.
func (m *WarpTileManager) CheckTile(mapID, x, y int) *WarpTile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tiles, ok := m.byMap[mapID]
	if !ok {
		return nil
	}
	key := fmt.Sprintf("%d,%d", x, y)
	return tiles[key]
}
