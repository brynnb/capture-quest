package world

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"

	"capturequest/internal/pokebattle"
)

type phaserMapWarp struct {
	ID            int
	SourceMapID   int
	X             int
	Y             int
	DestMapID     int
	DestX         int
	DestY         int
	WarpType      string
	WarpDirection string
}

type phaserWarpManager struct {
	db           pokebattle.DBTX
	actorManager *PhaserActorManager
	mu           sync.RWMutex
	byID         map[int]*phaserMapWarp
	byMap        map[int]map[string][]*phaserMapWarp
}

func newPhaserWarpManager(db pokebattle.DBTX) *phaserWarpManager {
	return &phaserWarpManager{
		db:    db,
		byID:  make(map[int]*phaserMapWarp),
		byMap: make(map[int]map[string][]*phaserMapWarp),
	}
}

func (m *phaserWarpManager) setActorManager(am *PhaserActorManager) {
	m.actorManager = am
}

func (m *phaserWarpManager) load() {
	rows, err := m.db.Query(`
		SELECT id, source_map_id, x, y, destination_map_id, destination_x, destination_y, warp_type, warp_direction
		FROM phaser_warps
		WHERE destination_map_id IS NOT NULL
		  AND destination_x IS NOT NULL
		  AND destination_y IS NOT NULL`)
	if err != nil {
		log.Printf("[PhaserWarps] Failed to load: %v", err)
		return
	}
	defer rows.Close()

	byID := make(map[int]*phaserMapWarp)
	byMap := make(map[int]map[string][]*phaserMapWarp)
	count := 0
	for rows.Next() {
		var warp phaserMapWarp
		var warpType string
		var warpDirection sql.NullString
		if err := rows.Scan(
			&warp.ID,
			&warp.SourceMapID,
			&warp.X,
			&warp.Y,
			&warp.DestMapID,
			&warp.DestX,
			&warp.DestY,
			&warpType,
			&warpDirection,
		); err != nil {
			log.Printf("[PhaserWarps] Error scanning row: %v", err)
			continue
		}
		warp.WarpType = normalizeWarpType(warpType)
		if warpDirection.Valid {
			warp.WarpDirection = normalizeWarpDirection(warpDirection.String)
		}

		w := &warp
		byID[w.ID] = w
		addPhaserWarpIndex(byMap, w.SourceMapID, w)
		if m.actorManager != nil && m.actorManager.IsOverworld(w.SourceMapID) {
			addPhaserWarpIndex(byMap, UnifiedOverworldMapID, w)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		log.Printf("[PhaserWarps] Error reading rows: %v", err)
	}

	m.mu.Lock()
	m.byID = byID
	m.byMap = byMap
	m.mu.Unlock()

	log.Printf("[PhaserWarps] Loaded %d normal map warps across %d map indexes", count, len(byMap))
}

func addPhaserWarpIndex(index map[int]map[string][]*phaserMapWarp, mapID int, warp *phaserMapWarp) {
	if index[mapID] == nil {
		index[mapID] = make(map[string][]*phaserMapWarp)
	}
	key := fmt.Sprintf("%d,%d", warp.X, warp.Y)
	index[mapID][key] = append(index[mapID][key], warp)
}

func (m *phaserWarpManager) warpAt(mapID, x, y int) *phaserMapWarp {
	m.mu.RLock()
	defer m.mu.RUnlock()

	warps := m.byMap[mapID][fmt.Sprintf("%d,%d", x, y)]
	if len(warps) == 0 {
		return nil
	}
	return warps[0]
}

func (m *phaserWarpManager) warpByID(id int) *phaserMapWarp {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byID[id]
}

func (m *phaserWarpManager) directionalWarpForFacingAttempt(mapID, x, y int, direction string, actorManager *PhaserActorManager) *phaserMapWarp {
	if warp := m.warpAt(mapID, x, y); warp != nil && warp.canActivateByDirection(mapID, x, y, direction, actorManager) {
		return warp
	}
	dx, dy, ok := warpDirectionDelta(direction)
	if !ok {
		return nil
	}
	if warp := m.warpAt(mapID, x+dx, y+dy); warp != nil && warp.canActivateByDirection(mapID, x, y, direction, actorManager) {
		return warp
	}
	return nil
}

func (w *phaserMapWarp) isDoor() bool {
	return normalizeWarpType(w.WarpType) == "door"
}

func (w *phaserMapWarp) isCarpet() bool {
	return normalizeWarpType(w.WarpType) == "carpet"
}

func (w *phaserMapWarp) canActivateByClick(playerMapID, playerX, playerY int, actorManager *PhaserActorManager) bool {
	if !w.matchesPlayerMap(playerMapID, actorManager) {
		return false
	}
	distance := abs(w.X-playerX) + abs(w.Y-playerY)
	if w.isCarpet() {
		if distance == 0 {
			return true
		}
		return distance == 1 && w.blockedWarpHasWalkableEntry(playerMapID, playerX, playerY, actorManager)
	}
	return distance <= 1
}

func (w *phaserMapWarp) canActivateForRequestedDestination(playerMapID, playerX, playerY, destX, destY int, actorManager *PhaserActorManager) bool {
	if !w.matchesPlayerMap(playerMapID, actorManager) {
		return false
	}
	if destX == playerX && destY == playerY {
		return true
	}
	return destX == w.X && destY == w.Y
}

func (w *phaserMapWarp) canActivateByDirection(playerMapID, playerX, playerY int, direction string, actorManager *PhaserActorManager) bool {
	if !w.matchesPlayerMap(playerMapID, actorManager) {
		return false
	}
	if w.X == playerX && w.Y == playerY {
		return w.isCarpet() && normalizeWarpDirection(w.WarpDirection) == normalizeWarpDirection(direction)
	}
	dx, dy, ok := warpDirectionDelta(direction)
	return ok &&
		w.X == playerX+dx &&
		w.Y == playerY+dy &&
		w.blockedWarpHasWalkableEntry(playerMapID, playerX, playerY, actorManager)
}

func (w *phaserMapWarp) canActivateOnPathDestination(playerMapID int, actorManager *PhaserActorManager) bool {
	if !w.matchesPlayerMap(playerMapID, actorManager) {
		return false
	}
	if w.isDoor() {
		return true
	}
	return w.isCarpet() && actorManager != nil && actorManager.IsOverworld(w.SourceMapID)
}

func (w *phaserMapWarp) matchesPlayerMap(playerMapID int, actorManager *PhaserActorManager) bool {
	if w.SourceMapID == playerMapID {
		return true
	}
	return playerMapID == UnifiedOverworldMapID && actorManager != nil && actorManager.IsOverworld(w.SourceMapID)
}

func (w *phaserMapWarp) blockedWarpHasWalkableEntry(playerMapID, entryX, entryY int, actorManager *PhaserActorManager) bool {
	if actorManager == nil {
		return false
	}

	collisionMapID := w.SourceMapID
	if playerMapID == UnifiedOverworldMapID && actorManager.IsOverworld(w.SourceMapID) {
		collisionMapID = UnifiedOverworldMapID
	}
	warpCollision, warpExists := actorManager.CollisionTypeAt(collisionMapID, w.X, w.Y)
	if warpExists && warpCollision == collisionLand {
		return false
	}

	entryCollision, entryExists := actorManager.CollisionTypeAt(collisionMapID, entryX, entryY)
	if !entryExists || entryCollision != collisionLand {
		return false
	}
	return true
}

func (w *phaserMapWarp) activationFacingDirection(playerMapID, playerX, playerY int, fallback string, actorManager *PhaserActorManager) string {
	normalizedFallback := normalizeWarpDirection(fallback)
	if playerX == w.X && playerY == w.Y && normalizedFallback != "" && w.sourceIsOverworld(playerMapID, actorManager) {
		return normalizedFallback
	}
	return w.facingDirectionFrom(playerX, playerY, fallback)
}

func (w *phaserMapWarp) sourceIsOverworld(playerMapID int, actorManager *PhaserActorManager) bool {
	if w.SourceMapID == UnifiedOverworldMapID || playerMapID == UnifiedOverworldMapID {
		return true
	}
	return actorManager != nil && actorManager.IsOverworld(w.SourceMapID)
}

func (w *phaserMapWarp) facingDirectionFrom(playerX, playerY int, fallback string) string {
	switch {
	case w.X > playerX:
		return "RIGHT"
	case w.X < playerX:
		return "LEFT"
	case w.Y > playerY:
		return "DOWN"
	case w.Y < playerY:
		return "UP"
	}
	if w.isCarpet() && normalizeWarpDirection(w.WarpDirection) != "" {
		return normalizeWarpDirection(w.WarpDirection)
	}
	if normalizeWarpDirection(fallback) != "" {
		return normalizeWarpDirection(fallback)
	}
	return "DOWN"
}

func normalizeWarpType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "carpet", "directional":
		return "carpet"
	default:
		return "door"
	}
}

func normalizeWarpDirection(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "UP", "DOWN", "LEFT", "RIGHT":
		return strings.ToUpper(strings.TrimSpace(value))
	default:
		return ""
	}
}

func warpDirectionDelta(direction string) (int, int, bool) {
	switch normalizeWarpDirection(direction) {
	case "UP":
		return 0, -1, true
	case "DOWN":
		return 0, 1, true
	case "LEFT":
		return -1, 0, true
	case "RIGHT":
		return 1, 0, true
	default:
		return 0, 0, false
	}
}
