package world

import (
	"github.com/google/uuid"
	"sync"
)

const (
	cuttableTreeRawFootTileID = 0x3d
	cutTreeReplacementRawFoot = 0x52
)

type CutTileManager struct {
	mu     sync.RWMutex
	tiles  map[int64]map[int]map[string]TileEdit
	scopes map[int64]map[int]string
}

func NewCutTileManager() *CutTileManager {
	return &CutTileManager{
		tiles:  make(map[int64]map[int]map[string]TileEdit),
		scopes: make(map[int64]map[int]string),
	}
}

func (m *CutTileManager) MarkCut(charID int64, mapID int, scope string, tile TileEdit) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if scope == "" || m.scopes[charID][mapID] != scope {
		return false
	}

	byMap := m.tiles[charID]
	if byMap == nil {
		byMap = make(map[int]map[string]TileEdit)
		m.tiles[charID] = byMap
	}
	cutTiles := byMap[mapID]
	if cutTiles == nil {
		cutTiles = make(map[string]TileEdit)
		byMap[mapID] = cutTiles
	}
	cutTiles[tileKey(tile.X, tile.Y)] = copyCutTile(tile)
	return true
}

func (m *CutTileManager) ClearMap(charID int64, mapID int) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if scopes := m.scopes[charID]; scopes != nil {
		delete(scopes, mapID)
		if len(scopes) == 0 {
			delete(m.scopes, charID)
		}
	}

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
	delete(m.scopes, charID)
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
	for key, tile := range cutTiles {
		overrides[key] = tile.CollisionType
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
	for key, tile := range cutTiles {
		overrides[key] = copyCutTile(tile).RawFootTileID
	}
	return overrides
}

func copyCutTile(tile TileEdit) TileEdit {
	if tile.RawFootTileID != nil {
		raw := *tile.RawFootTileID
		tile.RawFootTileID = &raw
	}
	return tile
}

// Project the same transient record used by movement, after event overrides.
// Base identity/provenance remains intact; this is not an authored world edit.
func (m *CutTileManager) ApplyToTiles(charID int64, mapID int, tiles []PhaserTile) []PhaserTile {
	if m == nil {
		return tiles
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	cuts := m.tiles[charID][mapID]
	for i := range tiles {
		tile, ok := cuts[tileKey(tiles[i].X, tiles[i].Y)]
		if !ok {
			continue
		}
		tile = copyCutTile(tile)
		tiles[i].TileImageID = tile.TileImageID
		tiles[i].CollisionType = tile.CollisionType
		tiles[i].RawFootTileID = tile.RawFootTileID
		tiles[i].TalkOverTile = tile.TalkOverTile
		tiles[i].ContentOrigin = "cut"
	}
	return tiles
}

// Scope is an opaque visit identity, not a durable receipt. Clearing or replacing
// the owner removes it, so delayed commands cannot recreate expired transient art.
func (m *CutTileManager) Scope(charID int64, mapID int) string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.scopes[charID] == nil {
		m.scopes[charID] = make(map[int]string)
	}
	if m.scopes[charID][mapID] == "" {
		m.scopes[charID][mapID] = uuid.NewString()
	}
	return m.scopes[charID][mapID]
}

func (m *CutTileManager) MatchesScope(charID int64, mapID int, scope string) bool {
	if m == nil || scope == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scopes[charID][mapID] == scope
}
func (m *CutTileManager) HasCut(charID int64, mapID int, scope string, x, y int) bool {
	if m == nil || scope == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.scopes[charID][mapID] != scope {
		return false
	}
	_, ok := m.tiles[charID][mapID][tileKey(x, y)]
	return ok
}
