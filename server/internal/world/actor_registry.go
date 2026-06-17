package world

import (
	"fmt"
	"sync"
)

type ActorType string

const (
	ActorTypePlayer ActorType = "player"
	ActorTypeNPC    ActorType = "npc"
	ActorTypeObject ActorType = "object"
)

// ActorRegistry manages the mapping between persistent entity IDs (from DB)
// and runtime Phaser IDs to ensure a unified and collision-free ID space.
type ActorRegistry struct {
	nextID   int
	mappings map[string]int // "type:id" -> phaserID
	mu       sync.RWMutex
}

func NewActorRegistry() *ActorRegistry {
	return &ActorRegistry{
		nextID:   1, // Start sequential IDs from 1
		mappings: make(map[string]int),
	}
}

// GetPhaserID returns the runtime ID for an entity, creating it if it doesn't exist.
func (r *ActorRegistry) GetPhaserID(actorType ActorType, originalID int) int {
	key := fmt.Sprintf("%s:%d", actorType, originalID)

	r.mu.RLock()
	id, ok := r.mappings[key]
	r.mu.RUnlock()

	if ok {
		return id
	}

	// Double check pattern for thread safety
	r.mu.Lock()
	defer r.mu.Unlock()

	if id, ok = r.mappings[key]; ok {
		return id
	}

	id = r.nextID
	r.mappings[key] = id
	r.nextID++

	return id
}

// GetOriginalID reverse-maps a runtime Phaser ID back to the original DB ID.
// Returns 0 if not found.
func (r *ActorRegistry) GetOriginalID(actorType ActorType, phaserID int) int {
	prefix := fmt.Sprintf("%s:", actorType)
	r.mu.RLock()
	defer r.mu.RUnlock()
	for key, id := range r.mappings {
		if id == phaserID && len(key) > len(prefix) && key[:len(prefix)] == prefix {
			var origID int
			fmt.Sscanf(key[len(prefix):], "%d", &origID)
			return origID
		}
	}
	return 0
}

// AllocateTemporaryID returns a new unique runtime ID not tied to any DB entity.
// Used for cutscene-spawned actors like Oak during the Pallet Town escort.
func (r *ActorRegistry) AllocateTemporaryID() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextID
	r.nextID++
	return id
}

// Reset clears the registry (useful for server restarts/reloads)
func (r *ActorRegistry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID = 1
	r.mappings = make(map[string]int)
}
