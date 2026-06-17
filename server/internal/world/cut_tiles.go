package world

import "sync"

const (
	cuttableTreeRawFootTileID   = 0x3d
	cutTreeReplacementTileImage = 25
	cutTreeReplacementRawFoot   = 0x52
)

type CutTileManager struct {
	mu    sync.RWMutex
	tiles map[int64]map[int]map[string]bool
}

func NewCutTileManager() *CutTileManager {
	return &CutTileManager{
		tiles: make(map[int64]map[int]map[string]bool),
	}
}

func (m *CutTileManager) MarkCut(charID int64, mapID, x, y int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	byMap := m.tiles[charID]
	if byMap == nil {
		byMap = make(map[int]map[string]bool)
		m.tiles[charID] = byMap
	}
	cutTiles := byMap[mapID]
	if cutTiles == nil {
		cutTiles = make(map[string]bool)
		byMap[mapID] = cutTiles
	}
	cutTiles[tileKey(x, y)] = true
}

func (m *CutTileManager) ClearMap(charID int64, mapID int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if byMap := m.tiles[charID]; byMap != nil {
		delete(byMap, mapID)
		if len(byMap) == 0 {
			delete(m.tiles, charID)
		}
	}
}

func (m *CutTileManager) ClearCharacter(charID int64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tiles, charID)
}

func (m *CutTileManager) CollisionOverrides(charID int64, mapID int) map[string]int {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutTiles := m.tiles[charID][mapID]
	if len(cutTiles) == 0 {
		return nil
	}
	overrides := make(map[string]int, len(cutTiles))
	for key := range cutTiles {
		overrides[key] = collisionLand
	}
	return overrides
}

func (m *CutTileManager) RawFootOverrides(charID int64, mapID int) map[string]*int {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutTiles := m.tiles[charID][mapID]
	if len(cutTiles) == 0 {
		return nil
	}
	overrides := make(map[string]*int, len(cutTiles))
	for key := range cutTiles {
		replacement := cutTreeReplacementRawFoot
		overrides[key] = &replacement
	}
	return overrides
}
