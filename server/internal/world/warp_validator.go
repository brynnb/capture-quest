package world

import (
	"fmt"
	"log"
	"sync"

	"capturequest/internal/pokebattle"
)

// WarpValidatorEntry represents a valid warp: source tile → destination map.
type WarpValidatorEntry struct {
	SourceMapID int
	X           int
	Y           int
	DestMapID   int
}

// WarpValidator loads all warps into memory and provides fast validation
// of whether a player position update constitutes a legal warp.
type WarpValidator struct {
	db           pokebattle.DBTX
	actorManager *PhaserActorManager
	mu           sync.RWMutex
	// validWarps: "srcMapID:x,y" -> set of valid destination map IDs
	validWarps map[string]map[int]bool
}

// NewWarpValidator creates a new WarpValidator.
func NewWarpValidator(db pokebattle.DBTX) *WarpValidator {
	return &WarpValidator{
		db:         db,
		validWarps: make(map[string]map[int]bool),
	}
}

// SetActorManager sets the actor manager reference (called after construction).
func (v *WarpValidator) SetActorManager(am *PhaserActorManager) {
	v.actorManager = am
}

// Load reads all warps from phaser_warps into memory for fast lookup.
func (v *WarpValidator) Load() {
	rows, err := v.db.Query(
		`SELECT source_map_id, x, y, destination_map_id FROM phaser_warps WHERE destination_map_id IS NOT NULL`)
	if err != nil {
		log.Printf("[WarpValidator] Failed to load warps: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	validWarps := make(map[string]map[int]bool)
	for rows.Next() {
		var srcMap, x, y, destMap int
		if err := rows.Scan(&srcMap, &x, &y, &destMap); err != nil {
			log.Printf("[WarpValidator] Error scanning row: %v", err)
			continue
		}
		key := fmt.Sprintf("%d:%d,%d", srcMap, x, y)
		if validWarps[key] == nil {
			validWarps[key] = make(map[int]bool)
		}
		validWarps[key][destMap] = true

		// Also index under the unified overworld map ID (9999) so that
		// lookups work when the player's map ID is the unified overworld ID.
		if v.actorManager != nil && v.actorManager.IsOverworld(srcMap) {
			owKey := fmt.Sprintf("%d:%d,%d", UnifiedOverworldMapID, x, y)
			if validWarps[owKey] == nil {
				validWarps[owKey] = make(map[int]bool)
			}
			validWarps[owKey][destMap] = true
		}
		count++
	}

	v.mu.Lock()
	v.validWarps = validWarps
	v.mu.Unlock()

	log.Printf("[WarpValidator] Loaded %d warp entries", count)
}

// IsValidWarp checks if moving from (srcMap, srcX, srcY) to destMap is a legal warp.
// Allows a 1-tile tolerance around the warp position to account for movement timing.
func (v *WarpValidator) IsValidWarp(srcMapID, srcX, srcY, destMapID int) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Check exact position and adjacent tiles (1-tile tolerance)
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			key := fmt.Sprintf("%d:%d,%d", srcMapID, srcX+dx, srcY+dy)
			if dests, ok := v.validWarps[key]; ok {
				if dests[destMapID] {
					return true
				}
			}
		}
	}
	return false
}

// IsSameMap returns true if the position update is on the same map (not a warp).
func (v *WarpValidator) IsSameMap(oldMapID, newMapID int) bool {
	return oldMapID == newMapID
}
