package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/logutil"
	"capturequest/internal/session"
	"container/heap"
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// ActorPathState tracks an actor that is being A*-pathed to a destination.
type ActorPathState struct {
	Actor        *PhaserActor
	Path         []PathNode
	LastMoveTime time.Time
	MoveSpeed    time.Duration
	OnComplete   func() // optional callback when path finishes
}

const (
	collisionBlocked = 0
	collisionLand    = 1
	collisionWater   = 2
)

type pathfindOptions struct {
	AllowWater bool
}

type tilePosition struct {
	X int
	Y int
}

type PhaserActorManager struct {
	wh                *WorldHandler
	walkingActors     map[int]*PhaserActor
	actorPaths        map[int]*ActorPathState // actorID -> pathed movement state
	collisionMap      map[int]map[string]int  // mapID -> "x,y" -> collisionType
	rawFootTileMap    map[int]map[string]int  // mapID -> "x,y" -> original 8x8 feet tile ID
	overworldMapIds   map[int]bool            // Set of map IDs that are part of the overworld
	collisionRevision uint64                  // Invalidates pending base-cache publications.
	mu                sync.RWMutex
	worker            periodicWorker
	nextActionTimes   map[int]time.Time
}

// NewPhaserActorManager creates and initializes the actor manager
func NewPhaserActorManager(wh *WorldHandler) *PhaserActorManager {
	mgr := &PhaserActorManager{
		wh:              wh,
		walkingActors:   make(map[int]*PhaserActor),
		actorPaths:      make(map[int]*ActorPathState),
		collisionMap:    make(map[int]map[string]int),
		rawFootTileMap:  make(map[int]map[string]int),
		overworldMapIds: make(map[int]bool),
		nextActionTimes: make(map[int]time.Time),
	}
	return mgr
}

// Start begins the actor simulation
func (m *PhaserActorManager) Load(ctx context.Context) error {
	staged := NewPhaserActorManager(m.wh)
	if err := staged.loadOverworldMapIds(ctx); err != nil {
		return err
	}
	if err := staged.loadWalkingActors(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	m.overworldMapIds, m.walkingActors = staged.overworldMapIds, staged.walkingActors
	m.collisionRevision++
	m.collisionMap, m.rawFootTileMap = staged.collisionMap, staged.rawFootTileMap
	m.nextActionTimes = staged.nextActionTimes
	m.mu.Unlock()
	log.Printf("[PhaserActorManager] Loaded simulation for %d actors", len(staged.walkingActors))
	return nil
}

func (m *PhaserActorManager) Start() {
	m.worker.start(250*time.Millisecond, nil, m.simulateMovement)
}

// Stop waits for the actor simulation and its current callback to finish.
func (m *PhaserActorManager) Stop() { m.worker.stop() }

// IsOverworld returns true if the map ID is part of the overworld
func (m *PhaserActorManager) IsOverworld(mapID int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isOverworldMapLocked(mapID)
}

func (m *PhaserActorManager) isOverworldMapLocked(mapID int) bool {
	return mapID == UnifiedOverworldMapID || m.overworldMapIds[mapID]
}

func (m *PhaserActorManager) shouldSendActorToSessionMap(actor *PhaserActor, sessionMapID int) bool {
	if actor == nil {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	isOverworldActor := m.isOverworldMapLocked(actor.MapID)
	playerOnOverworld := m.isOverworldMapLocked(sessionMapID)
	if sessionMapID == actor.MapID || (isOverworldActor && playerOnOverworld) {
		return true
	}

	// Keep overworld NPCs warm for players in interiors. NPC movement still
	// runs on the server, and map loads read static DB defaults unless the
	// client has a recent runtime position.
	return isOverworldActor && actor.ObjectType == "npc"
}

// Direct command callers own ses; cross-session broadcasts use Presence instead.
func (m *PhaserActorManager) actorForSession(actor *PhaserActor, ses *session.Session) (PhaserActor, bool) {
	if ses == nil || actor == nil || !m.shouldSendActorToSessionMap(actor, ses.MapID) {
		return PhaserActor{}, false
	}
	if !eventVisibilityAppliesToActor(actor) {
		return *actor, true
	}
	if !ses.HasValidClient() || m.wh == nil || m.wh.database == nil {
		return PhaserActor{}, false
	}
	result, visible, err := m.actorForCharacterContext(ses.CommandContext(), m.wh.database, *actor, int64(ses.Client.CharData().ID))
	if err != nil {
		return PhaserActor{}, false
	}
	return result, visible
}

func (m *PhaserActorManager) actorForPresence(actor *PhaserActor, p session.Presence) (PhaserActor, bool) {
	if actor == nil || !m.shouldSendActorToSessionMap(actor, p.MapID) {
		return PhaserActor{}, false
	}
	if !eventVisibilityAppliesToActor(actor) {
		return *actor, true
	}
	if p.CharacterID == 0 {
		return PhaserActor{}, false
	}
	return m.actorForCharacter(*actor, int64(p.CharacterID))
}

func eventVisibilityAppliesToActor(actor *PhaserActor) bool {
	if actor == nil {
		return false
	}
	return actor.ObjectType == "npc" || actor.ObjectType == "object" || actor.ObjectType == "item"
}

func (m *PhaserActorManager) actorForCharacter(actor PhaserActor, charID int64) (PhaserActor, bool) {
	if m == nil || m.wh == nil || m.wh.database == nil {
		return PhaserActor{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, visible, err := m.actorForCharacterContext(ctx, m.wh.database, actor, charID)
	if err != nil {
		logutil.Debugf("[Actors] Viewer %d projection for actor %d: %v", charID, actor.ID, err)
		return PhaserActor{}, false
	}
	return result, visible
}

func (m *PhaserActorManager) actorForCharacterContext(ctx context.Context, database db.ContextDBTX, actor PhaserActor, charID int64) (PhaserActor, bool, error) {
	if eventVisibilityAppliesToActor(&actor) {
		var flags *EventFlagManager
		if m.wh != nil {
			flags = m.wh.EventFlags
		}
		visible, err := applyEventObjectVisibilityContext(ctx, database, charID, actor.MapID, flags, []PhaserActor{actor})
		if err != nil {
			return PhaserActor{}, false, err
		}
		if len(visible) == 0 {
			return PhaserActor{}, false, nil
		}
		actor = visible[0]
	}
	actors, err := applyCharacterObjectPositionsContext(ctx, database, charID, []PhaserActor{actor})
	if err != nil {
		return PhaserActor{}, false, err
	}
	if len(actors) == 0 {
		return PhaserActor{}, false, nil
	}
	return actors[0], true, nil
}

func cloneIntPtr(v *int) *int {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

func cloneStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

func cloneBoolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	cloned := *v
	return &cloned
}

func (m *PhaserActorManager) applyRuntimeActorState(actor *PhaserActor) {
	if m == nil || actor == nil {
		return
	}

	m.mu.RLock()
	runtimeActor, ok := m.walkingActors[actor.ID]
	if !ok {
		m.mu.RUnlock()
		return
	}

	x := cloneIntPtr(runtimeActor.X)
	y := cloneIntPtr(runtimeActor.Y)
	actionType := cloneStringPtr(runtimeActor.ActionType)
	actionDirection := cloneStringPtr(runtimeActor.ActionDirection)
	frame := cloneIntPtr(runtimeActor.Frame)
	flipX := cloneBoolPtr(runtimeActor.FlipX)
	moveSpeed := runtimeActor.MoveSpeed
	movementType := cloneStringPtr(runtimeActor.MovementType)
	m.mu.RUnlock()

	if x != nil {
		actor.X = x
	}
	if y != nil {
		actor.Y = y
	}
	if actionType != nil {
		actor.ActionType = actionType
	}
	if actionDirection != nil {
		actor.ActionDirection = actionDirection
	}
	if frame != nil {
		actor.Frame = frame
	}
	if flipX != nil {
		actor.FlipX = flipX
	}
	if moveSpeed != 0 {
		actor.MoveSpeed = moveSpeed
	}
	if movementType != nil {
		actor.MovementType = movementType
	}
}

func (m *PhaserActorManager) loadOverworldMapIds(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rows, err := db.GlobalWorldDB.DB.QueryContext(ctx, "SELECT id FROM phaser_maps WHERE is_overworld = 1")
	if err != nil {
		return fmt.Errorf("[PhaserActorManager] Error loading overworld map IDs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan overworld map: %w", err)
		}
		m.overworldMapIds[id] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read overworld maps: %w", err)
	}
	return nil
}

func (m *PhaserActorManager) loadWalkingActors(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rows, err := db.GlobalWorldDB.DB.QueryContext(ctx, `
		SELECT po.id, po.map_id,
			COALESCE(po.x, po.local_x) as x,
			COALESCE(po.y, po.local_y) as y,
			po.object_type, po.sprite_name, po.name, po.action_type, po.action_direction, po.movement_type,
			po.text, po.trainer_class, po.trainer_party_index, po.item_id
		FROM phaser_objects po
		JOIN phaser_maps pm ON po.map_id = pm.id
		WHERE po.object_type = 'npc' AND po.sprite_name IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("[PhaserActorManager] Error loading walking actors: %w", err)
	}
	defer rows.Close()

	loadedMaps := make(map[int]bool)
	for rows.Next() {
		var n PhaserActor
		var x, y sql.NullInt64
		if err := rows.Scan(&n.ID, &n.MapID, &x, &y, &n.ObjectType, &n.SpriteName, &n.Name, &n.ActionType, &n.ActionDirection, &n.MovementType, &n.Text, &n.TrainerClass, &n.TrainerPartyIndex, &n.ItemID); err != nil {
			return fmt.Errorf("[PhaserActorManager] Error scanning actor: %w", err)
		}
		if !x.Valid || !y.Valid {
			return fmt.Errorf("actor %d on map %d has no coordinates", n.ID, n.MapID)
		}

		if x.Valid {
			val := int(x.Int64)
			n.X = &val
		}
		if y.Valid {
			val := int(y.Int64)
			n.Y = &val
		}

		n.DbID = n.ID

		// Remap the database ID to the unified runtime ID via the ActorRegistry.
		// This ensures movement broadcasts use the same IDs the client received
		// from HandlePhaserActorsRequest (which also remaps via the registry).
		if m.wh != nil && m.wh.ActorRegistry != nil {
			n.ID = m.wh.ActorRegistry.GetPhaserID(ActorTypeNPC, n.ID)
		}

		// Set default move speed for walking actors (300ms per tile)
		n.MoveSpeed = 300
		m.walkingActors[n.ID] = &n

		// Initialize next action time with a random offset to stagger them immediately
		m.nextActionTimes[n.ID] = time.Now().Add(time.Duration(rand.Intn(2000)) * time.Millisecond)

		loadedMaps[n.MapID] = true
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("read actors: %w", err)
	}
	// Exhaust the actor cursor before issuing collision queries, including when
	// the database pool permits only one active connection.
	for mapID := range loadedMaps {
		if err := m.ensureWalkableMapLoadedLocked(ctx, mapID); err != nil {
			return fmt.Errorf("actor map %d collision: %w", mapID, err)
		}
	}
	if len(loadedMaps) > 0 {
		log.Printf("[PhaserActorManager] Pre-loaded collision maps for %d maps", len(loadedMaps))
	}
	return nil
}

func (m *PhaserActorManager) ensureWalkableMapLoadedLocked(ctx context.Context, mapID int) error {
	return m.ensureWalkableMapLoadedLockedIn(ctx, db.GlobalWorldDB.DB, mapID)
}

func (m *PhaserActorManager) ensureWalkableMapLoadedLockedIn(ctx context.Context, database db.ContextDBTX, mapID int) error {
	if m.collisionMap == nil {
		m.collisionMap = make(map[int]map[string]int)
	}
	if m.rawFootTileMap == nil {
		m.rawFootTileMap = make(map[int]map[string]int)
	}
	if _, ok := m.collisionMap[mapID]; ok {
		return nil
	}

	collisions, feet, err := loadBaseCollision(ctx, database, mapID, m.overworldMapIds[mapID] || mapID == 0 || mapID == UnifiedOverworldMapID)
	if err != nil {
		return err
	}
	m.collisionMap[mapID], m.rawFootTileMap[mapID] = collisions, feet
	return nil
}

// Bootstrap calls the same row reader on a private staging manager. Runtime
// readers never hold the shared actor lock during database I/O.
func loadBaseCollision(ctx context.Context, database db.ContextDBTX, mapID int, overworld bool) (map[string]int, map[string]int, error) {
	if database == nil {
		return nil, nil, fmt.Errorf("collision database unavailable")
	}
	if pool, ok := database.(*sql.DB); ok && pool == nil {
		return nil, nil, fmt.Errorf("collision database unavailable")
	}
	collisions := make(map[string]int)
	feet := make(map[string]int)

	var rows *sql.Rows
	var err error

	// For overworld (map 9999 or any overworld map piece), load ALL overworld tiles
	// since they're stitched together with global coordinates
	if overworld {
		// Overworld tiles use global coordinates and have map_id IS NULL.
		rows, err = database.QueryContext(ctx, `
				SELECT x, y, collision_type, raw_foot_tile_id
				FROM phaser_tiles
				WHERE map_id IS NULL
				  AND is_tile_erased = 0`)
		if err != nil {
			return nil, nil, fmt.Errorf("[PhaserActorManager] Error loading overworld walkable maps: %w", err)
		}
	} else {
		// For interior maps, just load that specific map
		rows, err = database.QueryContext(ctx, `
				SELECT x, y, collision_type, raw_foot_tile_id
				FROM phaser_tiles
				WHERE map_id = $1
				  AND is_tile_erased = 0`, mapID)
		if err != nil {
			return nil, nil, fmt.Errorf("[PhaserActorManager] Error loading walkable map %d: %w", mapID, err)
		}
	}
	defer rows.Close()

	for rows.Next() {
		var x, y, collisionType int
		var rawFootTileID sql.NullInt64
		if err := rows.Scan(&x, &y, &collisionType, &rawFootTileID); err != nil {
			return nil, nil, fmt.Errorf("scan collision map %d: %w", mapID, err)
		}
		key := fmt.Sprintf("%d,%d", x, y)
		collisions[key] = collisionType
		if rawFootTileID.Valid {
			feet[key] = int(rawFootTileID.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read collision map %d: %w", mapID, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return collisions, feet, nil
}

func (m *PhaserActorManager) baseCollision(ctx context.Context, database db.ContextDBTX, mapID int, publish bool, snapshotRevision ...uint64) (map[string]int, map[string]int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	m.mu.RLock()
	collision, exists := m.collisionMap[mapID]
	raw := m.rawFootTileMap[mapID]
	revision := m.collisionRevision
	overworld := m.overworldMapIds[mapID] || mapID == 0 || mapID == UnifiedOverworldMapID
	m.mu.RUnlock()
	if len(snapshotRevision) > 0 && revision != snapshotRevision[0] {
		return nil, nil, fmt.Errorf("collision map %d snapshot retired", mapID)
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if exists {
		return collision, raw, nil
	}
	collision, raw, err := loadBaseCollision(ctx, database, mapID, overworld)
	if err != nil {
		return nil, nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if m.collisionRevision != revision {
		return nil, nil, fmt.Errorf("collision map %d invalidated during read", mapID)
	}
	if publish {
		if m.collisionMap == nil {
			m.collisionMap = make(map[int]map[string]int)
		}
		if m.rawFootTileMap == nil {
			m.rawFootTileMap = make(map[int]map[string]int)
		}
		m.collisionMap[mapID], m.rawFootTileMap[mapID] = collision, raw
	}
	return collision, raw, nil
}

func (m *PhaserActorManager) ensureWalkableMapLoaded(mapID int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var database db.ContextDBTX
	if m.wh != nil && m.wh.database != nil {
		database = m.wh.database
	} else if db.GlobalWorldDB != nil {
		database = db.GlobalWorldDB.DB
	}
	if database == nil {
		return fmt.Errorf("collision database unavailable")
	}
	_, _, err := m.baseCollision(ctx, database, mapID, true)
	return err
}

// Overworld map IDs all read the same stitched tile corpus. Invalidating one
// must retire every cached alias and any read that started before the edit.
func (m *PhaserActorManager) InvalidateCollisionMap(mapID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collisionRevision++
	if mapID == 0 || m.isOverworldMapLocked(mapID) {
		for id := range m.collisionMap {
			if id == 0 || m.isOverworldMapLocked(id) {
				delete(m.collisionMap, id)
				delete(m.rawFootTileMap, id)
			}
		}
		return
	}
	delete(m.collisionMap, mapID)
	delete(m.rawFootTileMap, mapID)
}

func (m *PhaserActorManager) simulateMovement() {
	// Skip all NPC simulation when no players are connected.
	// This eliminates CPU usage from pathfinding and wandering
	// when nobody is online to see it.
	if session.GetActiveSessionCount() == 0 {
		return
	}

	now := time.Now()

	// Process A*-pathed actors first (cutscene NPCs, scripted movement)
	m.processPathedMovement(now)

	if captureQuestTestModeEnabled() {
		return
	}

	m.mu.Lock()
	actorsToUpdate := make([]*PhaserActor, 0)

	// totalWalking := len(m.walkingActors)
	// movesAttempted := 0
	// movesFound := 0

	for id, actor := range m.walkingActors {
		// Only simulate actors that have global coordinates assigned
		if actor.X == nil || actor.Y == nil {
			continue
		}

		// Skip actors that are being A*-pathed (they're handled by processPathedMovement)
		if _, pathed := m.actorPaths[id]; pathed {
			continue
		}

		// Only process if it's time for this NPC's next move
		nextTime, exists := m.nextActionTimes[id]
		if exists && now.Before(nextTime) {
			continue
		}

		// movesAttempted++

		// Set next check time (2 seconds + 0-1 seconds jitter)
		m.nextActionTimes[id] = now.Add(2 * time.Second).Add(time.Duration(rand.Intn(1000)) * time.Millisecond)

		// Random chance to act (70%)
		if rand.Float32() < 0.7 {
			actionType := "STAY"
			if actor.ActionType != nil {
				actionType = *actor.ActionType
			}

			var newX, newY int
			var dir string

			if actionType == "WALK" {
				newX, newY, dir = m.calculateNextMove(actor)
			} else {
				// For STAY, characters only turn in place
				if actor.X != nil && actor.Y != nil {
					newX, newY = *actor.X, *actor.Y
				}
				// 50% chance to pick a random direction
				if rand.Float32() > 0.5 {
					directions := []string{"UP", "DOWN", "LEFT", "RIGHT"}
					dir = directions[rand.Intn(len(directions))]
				} else {
					if actor.ActionDirection != nil {
						dir = *actor.ActionDirection
					} else {
						dir = "DOWN"
					}
				}
			}

			// Update and broadcast if position OR direction changed
			currentDir := ""
			if actor.ActionDirection != nil {
				currentDir = *actor.ActionDirection
			}

			if (actor.X == nil || actor.Y == nil) || newX != *actor.X || newY != *actor.Y || dir != currentDir {
				updateX, updateY := newX, newY
				actor.X = &updateX
				actor.Y = &updateY
				actor.ActionDirection = &dir
				actorsToUpdate = append(actorsToUpdate, actor)
				// movesFound++
			}
		}
	}
	m.mu.Unlock()

	// if movesAttempted > 0 {
	// 	// Only log periodically or when actual moves occur to avoid spamming the console 4x a second
	// 	// if movesFound > 0 || rand.Float32() < 0.01 {
	// 	// 	log.Printf("[PhaserActorManager] Simulation: %d active, %d staggered checks, %d moves found", totalWalking, movesAttempted, movesFound)
	// 	// }
	// }

	// Broadcast updates
	for _, actor := range actorsToUpdate {
		m.broadcastActorUpdate(actor, 0)
	}
}

func (m *PhaserActorManager) calculateNextMove(actor *PhaserActor) (int, int, string) {
	// Determine allowed directions based on ActionDirection constraint
	var directions []string
	constraint := ""
	if actor.ActionDirection != nil {
		constraint = *actor.ActionDirection
	}

	switch constraint {
	case "UP_DOWN":
		directions = []string{"UP", "DOWN"}
	case "LEFT_RIGHT":
		directions = []string{"LEFT", "RIGHT"}
	case "ANY_DIR", "NONE", "DOWN", "UP", "LEFT", "RIGHT", "":
		// For ANY_DIR or specific facing directions, NPCs can usually wander anywhere if they are 'WALK' type
		// but we'll default to all 4 for wandering.
		directions = []string{"UP", "DOWN", "LEFT", "RIGHT"}
	default:
		// Default to all directions for wandering
		directions = []string{"UP", "DOWN", "LEFT", "RIGHT"}
	}

	rand.Shuffle(len(directions), func(i, j int) {
		directions[i], directions[j] = directions[j], directions[i]
	})

	actorX, actorY := 0, 0
	if actor.X != nil {
		actorX = *actor.X
	}
	if actor.Y != nil {
		actorY = *actor.Y
	}

	currentKey := fmt.Sprintf("%d,%d", actorX, actorY)
	currentType := m.collisionMap[actor.MapID][currentKey]

	// Determine allowed terrains based on MovementType
	moveType := "LAND"
	if actor.MovementType != nil {
		moveType = *actor.MovementType
	}

	for _, dir := range directions {
		nextX, nextY := actorX, actorY
		switch dir {
		case "UP":
			nextY--
		case "DOWN":
			nextY++
		case "LEFT":
			nextX--
		case "RIGHT":
			nextX++
		}

		key := fmt.Sprintf("%d,%d", nextX, nextY)
		nextType, tileExists := m.collisionMap[actor.MapID][key]

		if !tileExists {
			continue
		}
		if m.isRuntimeNPCBlockingTile(actor.MapID, nextX, nextY, actor.ID) ||
			m.isPlayerBlockingTile(actor.MapID, nextX, nextY) {
			continue
		}

		// Movement logic:
		// 0: Solid, 1: Land, 2: Water
		canMove := false
		if nextType != 0 {
			switch moveType {
			case "WATER":
				canMove = (nextType == 2)
			case "LAND":
				canMove = (nextType == 1)
			case "BOTH":
				canMove = true
			}
		}

		// Stuck recovery: if currently on a tile not allowed for our type,
		// allow moving to ANY adjacent tile that matches our type or ANY tile if we are really stuck
		isCurrentAllowed := false
		switch moveType {
		case "WATER":
			isCurrentAllowed = (currentType == 2)
		case "LAND":
			isCurrentAllowed = (currentType == 1)
		case "BOTH":
			isCurrentAllowed = (currentType != 0)
		}

		if canMove || (!isCurrentAllowed && nextType != 0) {
			return nextX, nextY, dir
		}
	}

	// If couldn't move, pick a new random direction to turn (50% chance to turn, 50% chance to stay same)
	if rand.Float32() > 0.5 {
		return actorX, actorY, directions[0]
	}

	currentDir := "DOWN"
	if actor.ActionDirection != nil {
		currentDir = *actor.ActionDirection
	}

	return actorX, actorY, currentDir
}

func (m *PhaserActorManager) broadcastActorSpawn(actor *PhaserActor, originSessionID int) {
	m.wh.sessionManager.ForEachSession(func(ses *session.Session) {
		presence := ses.Presence()
		if !presence.Authenticated {
			return
		}

		// Don't broadcast back to the originating session
		if ses.SessionID == originSessionID {
			return
		}

		if actorForSession, ok := m.actorForPresence(actor, presence); ok {
			ses.SendStreamJSON(actorForSession, opcodes.PhaserActorPositionUpdate)
		}
	})
}

func (m *PhaserActorManager) broadcastActorUpdate(actor *PhaserActor, originSessionID int) {
	m.wh.sessionManager.ForEachSession(func(ses *session.Session) {
		presence := ses.Presence()
		if !presence.Authenticated {
			return
		}

		// Don't broadcast back to the originating session
		if originSessionID != 0 && ses.SessionID == originSessionID {
			return
		}

		if actorForSession, ok := m.actorForPresence(actor, presence); ok {
			// Using SendStreamJSON for reliability as position updates are important
			ses.SendStreamJSON(StructToMap(&actorForSession), opcodes.PhaserActorPositionUpdate)
		}
	})
}

func (m *PhaserActorManager) SendObjectActorToSession(objectID int, ses *session.Session) error {
	if m == nil || ses == nil || !ses.Authenticated || !ses.HasValidClient() {
		return nil
	}

	actor, err := m.loadPhaserObjectActorContext(ses.CommandContext(), m.wh.database, objectID)
	if err != nil {
		return err
	}
	actor, ok := m.actorForSession(&actor, ses)
	if !ok {
		return nil
	}

	ses.SendStreamJSON(actor, opcodes.PhaserActorPositionUpdate)
	return nil
}

func (m *PhaserActorManager) loadPhaserObjectActorContext(ctx context.Context, database db.ContextDBTX, objectID int) (PhaserActor, error) {
	var actor PhaserActor
	var x, y sql.NullInt64
	if err := database.QueryRowContext(ctx, `
		SELECT id, map_id,
			COALESCE(x, local_x) AS x,
			COALESCE(y, local_y) AS y,
			COALESCE(object_type, 'object'), sprite_name, name, action_type, action_direction, COALESCE(movement_type, 'LAND'),
			text, trainer_class, trainer_party_index, item_id
		FROM phaser_objects
		WHERE id = $1`, objectID).Scan(
		&actor.ID,
		&actor.MapID,
		&x,
		&y,
		&actor.ObjectType,
		&actor.SpriteName,
		&actor.Name,
		&actor.ActionType,
		&actor.ActionDirection,
		&actor.MovementType,
		&actor.Text,
		&actor.TrainerClass,
		&actor.TrainerPartyIndex,
		&actor.ItemID,
	); err != nil {
		return PhaserActor{}, err
	}

	if x.Valid {
		v := int(x.Int64)
		actor.X = &v
	}
	if y.Valid {
		v := int(y.Int64)
		actor.Y = &v
	}

	actor.DbID = actor.ID
	if actor.ActionType != nil && *actor.ActionType == "WALK" {
		actor.MoveSpeed = 300
	}
	if m.wh != nil && m.wh.ActorRegistry != nil {
		actor.ID = m.wh.ActorRegistry.GetPhaserID(ActorTypeNPC, actor.ID)
	}
	m.applyRuntimeActorState(&actor)
	return actor, nil
}

// FindPath runs A* pathfinding on the collision map. Shared by player and actor movement.
func (m *PhaserActorManager) FindPath(mapID, startX, startY, endX, endY int) []PathNode {
	collisionMap := m.collisionMapForMap(mapID)
	if collisionMap == nil {
		return nil
	}
	return findPathOnCollisionMap(collisionMap, nil, startX, startY, endX, endY)
}

func (m *PhaserActorManager) FindPathForCharacter(charID int64, mapID, startX, startY, endX, endY int, efm *EventFlagManager) []PathNode {
	return m.FindPathForCharacterWithOptions(charID, mapID, startX, startY, endX, endY, efm, pathfindOptions{})
}

func (m *PhaserActorManager) FindPathForCharacterWithOptions(charID int64, mapID, startX, startY, endX, endY int, efm *EventFlagManager, opts pathfindOptions) []PathNode {
	var database db.ReadDBTX
	if m.wh != nil && m.wh.database != nil {
		database = m.wh.database
	} else if db.GlobalWorldDB != nil {
		database = db.GlobalWorldDB.DB
	}
	if database == nil {
		return nil
	}
	collisionMap, rawFootTileMap, err := m.characterCollision(context.Background(), database, charID, mapID, startX, startY, efm)
	if err != nil {
		log.Printf("[ActorManager] Character collision: %v", err)
		return nil
	}
	return findPathOnCollisionMapWithOptions(collisionMap, rawFootTileMap, startX, startY, endX, endY, opts)
}

// characterCollision is shared by pathfinding and issued player steps. A failed
// dynamic blocker read must never authorize movement through that blocker.
func (m *PhaserActorManager) characterCollision(ctx context.Context, database db.ReadDBTX, charID int64, mapID, startX, startY int, efm *EventFlagManager) (map[string]int, map[string]int, error) {
	if pool, ok := database.(*sql.DB); ok {
		// Capture before opening the snapshot: its first flag query can precede
		// an edit even when the subsequent cold base query follows invalidation.
		m.mu.RLock()
		revision := m.collisionRevision
		m.mu.RUnlock()
		type view struct{ collision, raw map[string]int }
		result, err := db.ReadSnapshot(ctx, pool, func(ctx context.Context, q db.ReadDBTX) (view, error) {
			collision, raw, err := m.characterCollisionIn(ctx, q, charID, mapID, startX, startY, efm, true, revision)
			return view{collision, raw}, err
		})
		if err == nil {
			m.mu.RLock()
			current := m.collisionRevision == revision
			m.mu.RUnlock()
			if !current {
				return nil, nil, fmt.Errorf("collision map %d snapshot retired", mapID)
			}
		}
		return result.collision, result.raw, err
	}
	return m.characterCollisionIn(ctx, database, charID, mapID, startX, startY, efm, false)
}

func (m *PhaserActorManager) characterCollisionIn(ctx context.Context, database db.ReadDBTX, charID int64, mapID, startX, startY int, efm *EventFlagManager, publishBase bool, snapshotRevision ...uint64) (map[string]int, map[string]int, error) {
	// Player collision and presentation must select rules from durable flags in
	// the same owned snapshot. A stale cache is not movement authorization.
	if efm != nil {
		flags, err := eventFlagSnapshotIn(database, charID)
		if err != nil {
			return nil, nil, err
		}
		efm = flags
	}
	// A cold gameplay-transaction read may include uncommitted rows; it must
	// remain local to that transaction rather than warming the shared cache.
	collisionMap, rawFootTileMap, err := m.baseCollision(ctx, database, mapID, publishBase, snapshotRevision...)
	if err != nil {
		return nil, nil, err
	}
	m.mu.RLock()
	overworld := m.isOverworldMapLocked(mapID)
	m.mu.RUnlock()
	if !overworld {
		rawFootTileMap = nil
	}
	var tileOverrides []eventTileOverride
	if efm != nil {
		tileOverrides, err = eventTileOverridesForMapContext(ctx, database, mapID)
		if err != nil {
			return nil, nil, err
		}
	}
	selected := eligibleEventTileOverrides(charID, efm, tileOverrides)
	overrides := make(map[string]int)
	for key, override := range selected {
		overrides[key] = override.CollisionType
	}
	if len(overrides) > 0 {
		withOverrides := copyCollisionMap(collisionMap, len(overrides))
		for key, collisionType := range overrides {
			withOverrides[key] = collisionType
		}
		collisionMap = withOverrides
	}
	if m.wh != nil && m.wh.CutTiles != nil {
		cutOverrides := m.wh.CutTiles.CollisionOverrides(charID, mapID)
		if len(cutOverrides) > 0 {
			withOverrides := copyCollisionMap(collisionMap, len(cutOverrides))
			for key, collisionType := range cutOverrides {
				withOverrides[key] = collisionType
			}
			collisionMap = withOverrides
		}
	}
	if rawFootTileMap != nil {
		var rawOverrides map[string]*int
		if efm != nil {
			rawOverrides = make(map[string]*int)
			for _, override := range selected {
				props, err := tileRuntimePropertiesForTileImageContext(ctx, database, override.TileImageID)
				if err != nil {
					return nil, nil, fmt.Errorf("collision event tile map=%d coordinate=(%d,%d) image=%d: %w", mapID, override.X, override.Y, override.TileImageID, err)
				}
				rawOverrides[tileKey(override.X, override.Y)] = props.RawFootTileID
			}
		}
		if len(rawOverrides) > 0 {
			withOverrides := copyRawFootTileMap(rawFootTileMap, 0)
			for key, rawFootTileID := range rawOverrides {
				if rawFootTileID == nil {
					delete(withOverrides, key)
					continue
				}
				withOverrides[key] = *rawFootTileID
			}
			rawFootTileMap = withOverrides
		}
		if m.wh != nil && m.wh.CutTiles != nil {
			cutRawOverrides := m.wh.CutTiles.RawFootOverrides(charID, mapID)
			if len(cutRawOverrides) > 0 {
				withOverrides := copyRawFootTileMap(rawFootTileMap, 0)
				for key, rawFootTileID := range cutRawOverrides {
					if rawFootTileID == nil {
						delete(withOverrides, key)
						continue
					}
					withOverrides[key] = *rawFootTileID
				}
				rawFootTileMap = withOverrides
			}
		}
	}
	var boulders []BoulderObjectState
	if efm != nil {
		boulders, err = boulderObjectsForCharacterContext(ctx, database, charID, mapID, efm)
	}
	if err == nil && len(boulders) > 0 {
		withBoulders := copyCollisionMap(collisionMap, len(boulders))
		for _, boulder := range boulders {
			if !boulder.Visible {
				continue
			}
			if boulder.X == startX && boulder.Y == startY {
				continue
			}
			withBoulders[tileKey(boulder.X, boulder.Y)] = 0
		}
		collisionMap = withBoulders
	} else if err != nil {
		return nil, nil, fmt.Errorf("boulder collision for char %d map %d: %w", charID, mapID, err)
	}
	blockers, err := m.npcBlockingPositionsContext(ctx, database, charID, mapID, efm)
	if err != nil {
		return nil, nil, err
	}
	if len(blockers) > 0 {
		collisionMap = collisionMapWithBlockedPositions(collisionMap, blockers, startX, startY)
	}
	return collisionMap, rawFootTileMap, nil
}

func collisionMapWithBlockedPositions(collisionMap map[string]int, blockers []tilePosition, startX, startY int) map[string]int {
	if len(blockers) == 0 {
		return collisionMap
	}
	withBlockers := copyCollisionMap(collisionMap, len(blockers))
	for _, blocker := range blockers {
		if blocker.X == startX && blocker.Y == startY {
			continue
		}
		withBlockers[tileKey(blocker.X, blocker.Y)] = collisionBlocked
	}
	return withBlockers
}

func copyCollisionMap(collisionMap map[string]int, extraCapacity int) map[string]int {
	copied := make(map[string]int, len(collisionMap)+extraCapacity)
	for key, collisionType := range collisionMap {
		copied[key] = collisionType
	}
	return copied
}

func copyRawFootTileMap(rawFootTileMap map[string]int, extraCapacity int) map[string]int {
	copied := make(map[string]int, len(rawFootTileMap)+extraCapacity)
	for key, rawFootTileID := range rawFootTileMap {
		copied[key] = rawFootTileID
	}
	return copied
}

func (m *PhaserActorManager) collisionMapForMap(mapID int) map[string]int {
	m.mu.RLock()
	collisionMap, exists := m.collisionMap[mapID]
	m.mu.RUnlock()

	if !exists {
		if err := m.ensureWalkableMapLoaded(mapID); err != nil {
			log.Printf("[PhaserActorManager] Collision load failed: %v", err)
			return nil
		}
		m.mu.RLock()
		collisionMap = m.collisionMap[mapID]
		m.mu.RUnlock()
	}

	if collisionMap == nil {
		return nil
	}
	return collisionMap
}

func (m *PhaserActorManager) rawFootTileMapForMap(mapID int) map[string]int {
	m.mu.RLock()
	rawFootTileMap, exists := m.rawFootTileMap[mapID]
	m.mu.RUnlock()

	if !exists {
		if err := m.ensureWalkableMapLoaded(mapID); err != nil {
			log.Printf("[PhaserActorManager] Collision load failed: %v", err)
			return nil
		}
		m.mu.RLock()
		rawFootTileMap = m.rawFootTileMap[mapID]
		m.mu.RUnlock()
	}

	return rawFootTileMap
}

func (m *PhaserActorManager) TileExists(mapID, x, y int) bool {
	collisionMap := m.collisionMapForMap(mapID)
	if collisionMap == nil {
		return false
	}
	_, exists := collisionMap[fmt.Sprintf("%d,%d", x, y)]
	return exists
}

func (m *PhaserActorManager) CollisionTypeAt(mapID, x, y int) (int, bool) {
	collisionMap := m.collisionMapForMap(mapID)
	if collisionMap == nil {
		return collisionBlocked, false
	}
	collisionType, exists := collisionMap[fmt.Sprintf("%d,%d", x, y)]
	return collisionType, exists
}

func (m *PhaserActorManager) IsNPCBlockingTileForCharacter(charID int64, mapID, x, y int, efm *EventFlagManager) bool {
	for _, blocker := range m.NPCBlockingPositionsForCharacter(charID, mapID, efm) {
		if blocker.X == x && blocker.Y == y {
			return true
		}
	}
	return false
}

func (m *PhaserActorManager) NPCBlockingPositionsForCharacter(charID int64, mapID int, efm *EventFlagManager) []tilePosition {
	if m == nil || db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		return nil
	}
	positions, err := m.npcBlockingPositionsContext(context.Background(), db.GlobalWorldDB.DB, charID, mapID, efm)
	if err != nil {
		log.Printf("[ActorManager] NPC collision: %v", err)
		return nil
	}
	return positions
}

func (m *PhaserActorManager) npcBlockingPositionsContext(ctx context.Context, database db.ContextDBTX, charID int64, mapID int, efm *EventFlagManager) ([]tilePosition, error) {
	actors, err := m.npcBlockingActorsContext(ctx, database, mapID)
	if err != nil {
		return nil, err
	}
	if len(actors) == 0 {
		return nil, nil
	}

	actors, err = applyEventObjectVisibilityContext(ctx, database, charID, mapID, efm, actors)
	if err != nil {
		return nil, err
	}
	positionsByID, err := characterObjectPositionsContext(ctx, database, charID)
	if err != nil {
		return nil, err
	}
	for i := range actors {
		if pos, ok := positionsByID[actors[i].DbID]; ok {
			x, y := pos.X, pos.Y
			actors[i].X, actors[i].Y = &x, &y
		}
	}

	positions := make([]tilePosition, 0, len(actors))
	for _, actor := range actors {
		if actor.X == nil || actor.Y == nil {
			continue
		}
		positions = append(positions, tilePosition{X: *actor.X, Y: *actor.Y})
	}
	return positions, nil
}

func (m *PhaserActorManager) npcBlockingActors(mapID int) ([]PhaserActor, error) {
	return m.npcBlockingActorsContext(context.Background(), db.GlobalWorldDB.DB, mapID)
}

func (m *PhaserActorManager) npcBlockingActorsContext(ctx context.Context, database db.ContextDBTX, mapID int) ([]PhaserActor, error) {
	query := `
		SELECT po.id, po.map_id,
			COALESCE(po.x, po.local_x) AS x,
			COALESCE(po.y, po.local_y) AS y,
			po.object_type, po.sprite_name, po.name, po.action_type, po.action_direction, po.movement_type,
			po.text, po.trainer_class, po.trainer_party_index, po.item_id
		FROM phaser_objects po
		WHERE po.object_type = 'npc'
		  AND po.sprite_name IS NOT NULL
		  AND po.map_id = $1`
	args := []interface{}{mapID}
	if mapID == UnifiedOverworldMapID {
		query = `
			SELECT po.id, po.map_id,
				COALESCE(po.x, po.local_x) AS x,
				COALESCE(po.y, po.local_y) AS y,
				po.object_type, po.sprite_name, po.name, po.action_type, po.action_direction, po.movement_type,
				po.text, po.trainer_class, po.trainer_party_index, po.item_id
			FROM phaser_objects po
			JOIN phaser_maps pm ON pm.id = po.map_id
			WHERE po.object_type = 'npc'
			  AND po.sprite_name IS NOT NULL
			  AND pm.is_overworld = 1`
		args = nil
	}

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	actors := []PhaserActor{}
	for rows.Next() {
		var actor PhaserActor
		var x, y sql.NullInt64
		if err := rows.Scan(
			&actor.ID,
			&actor.MapID,
			&x,
			&y,
			&actor.ObjectType,
			&actor.SpriteName,
			&actor.Name,
			&actor.ActionType,
			&actor.ActionDirection,
			&actor.MovementType,
			&actor.Text,
			&actor.TrainerClass,
			&actor.TrainerPartyIndex,
			&actor.ItemID,
		); err != nil {
			return nil, err
		}
		if x.Valid {
			v := int(x.Int64)
			actor.X = &v
		}
		if y.Valid {
			v := int(y.Int64)
			actor.Y = &v
		}
		actor.DbID = actor.ID
		if m.wh != nil && m.wh.ActorRegistry != nil {
			actor.ID = m.wh.ActorRegistry.GetPhaserID(ActorTypeNPC, actor.ID)
		}
		m.applyRuntimeActorState(&actor)
		actors = append(actors, actor)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return actors, nil
}

func (m *PhaserActorManager) isRuntimeNPCBlockingTile(mapID, x, y, ignoredActorID int) bool {
	if m == nil {
		return false
	}
	for id, actor := range m.walkingActors {
		if id == ignoredActorID || actor == nil || actor.X == nil || actor.Y == nil {
			continue
		}
		if !m.actorSharesMovementMapLocked(actor.MapID, mapID) {
			continue
		}
		if *actor.X == x && *actor.Y == y {
			return true
		}
	}
	return false
}

func (m *PhaserActorManager) isPlayerBlockingTile(mapID, x, y int) bool {
	if m == nil || m.wh == nil || m.wh.sessionManager == nil {
		return false
	}
	blocked := false
	m.wh.sessionManager.ForEachSession(func(ses *session.Session) {
		presence := ses.Presence()
		if blocked || presence.CharacterID == 0 {
			return
		}
		if !m.actorSharesMovementMapLocked(presence.MapID, mapID) {
			return
		}
		if int(presence.X) == x && int(presence.Y) == y {
			blocked = true
		}
	})
	return blocked
}

func (m *PhaserActorManager) actorSharesMovementMap(actorMapID, movementMapID int) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.actorSharesMovementMapLocked(actorMapID, movementMapID)
}

func (m *PhaserActorManager) actorSharesMovementMapLocked(actorMapID, movementMapID int) bool {
	if actorMapID == movementMapID {
		return true
	}
	return (movementMapID == UnifiedOverworldMapID && m.isOverworldMapLocked(actorMapID)) ||
		(actorMapID == UnifiedOverworldMapID && m.isOverworldMapLocked(movementMapID))
}

func findPathOnCollisionMap(collisionMap map[string]int, rawFootTileMap map[string]int, startX, startY, endX, endY int) []PathNode {
	return findPathOnCollisionMapWithOptions(collisionMap, rawFootTileMap, startX, startY, endX, endY, pathfindOptions{})
}

func findPathOnCollisionMapWithOptions(collisionMap map[string]int, rawFootTileMap map[string]int, startX, startY, endX, endY int, opts pathfindOptions) []PathNode {
	openSet := make(PriorityQueue, 0)
	heap.Init(&openSet)

	startNode := &AStarNode{
		X: startX, Y: startY,
		G: 0, H: abs(endX-startX) + abs(endY-startY),
	}
	startNode.F = startNode.G + startNode.H
	heap.Push(&openSet, startNode)
	closedSet := make(map[string]bool)
	gScore := make(map[string]int)
	startKey := fmt.Sprintf("%d,%d", startX, startY)
	gScore[startKey] = 0

	directions := []struct {
		name   string
		dx, dy int
	}{
		{name: "UP", dx: 0, dy: -1},
		{name: "DOWN", dx: 0, dy: 1},
		{name: "LEFT", dx: -1, dy: 0},
		{name: "RIGHT", dx: 1, dy: 0},
	}
	iterations := 0
	maxIterations := 10000

	for openSet.Len() > 0 && iterations < maxIterations {
		iterations++
		current := heap.Pop(&openSet).(*AStarNode)

		if current.X == endX && current.Y == endY {
			var path []PathNode
			node := current
			for node.Parent != nil {
				path = append([]PathNode{{X: node.X, Y: node.Y}}, path...)
				node = node.Parent
			}
			return path
		}

		key := fmt.Sprintf("%d,%d", current.X, current.Y)
		if closedSet[key] {
			continue
		}
		closedSet[key] = true

		for _, dir := range directions {
			nx, ny := current.X+dir.dx, current.Y+dir.dy
			nkey := fmt.Sprintf("%d,%d", nx, ny)
			next := PathNode{X: nx, Y: ny}
			if closedSet[nkey] {
				continue
			}
			collisionType, tileExists := collisionMap[nkey]
			if !tileExists || !isPathableCollision(collisionType, opts) {
				var ok bool
				next, ok = ledgeLandingNeighbor(collisionMap, rawFootTileMap, current.X, current.Y, dir.name, dir.dx, dir.dy)
				if !ok {
					continue
				}
				nkey = fmt.Sprintf("%d,%d", next.X, next.Y)
				if closedSet[nkey] {
					continue
				}
			}
			g := current.G + 1
			if prevG, seen := gScore[nkey]; seen && g >= prevG {
				continue
			}
			gScore[nkey] = g
			h := abs(endX-next.X) + abs(endY-next.Y)
			neighbor := &AStarNode{X: next.X, Y: next.Y, G: g, H: h, F: g + h, Parent: current}
			heap.Push(&openSet, neighbor)
		}
	}
	return nil
}

func isPathableCollision(collisionType int, opts pathfindOptions) bool {
	return collisionType == collisionLand || (opts.AllowWater && collisionType == collisionWater)
}

func ledgeLandingNeighbor(collisionMap map[string]int, rawFootTileMap map[string]int, x, y int, direction string, dx, dy int) (PathNode, bool) {
	if len(rawFootTileMap) == 0 {
		return PathNode{}, false
	}

	standingRawFootTileID, ok := rawFootTileMap[fmt.Sprintf("%d,%d", x, y)]
	if !ok {
		return PathNode{}, false
	}
	frontRawFootTileID, ok := rawFootTileMap[fmt.Sprintf("%d,%d", x+dx, y+dy)]
	if !ok {
		return PathNode{}, false
	}
	if !canJumpLedge(direction, standingRawFootTileID, frontRawFootTileID) {
		return PathNode{}, false
	}

	landing := PathNode{X: x + 2*dx, Y: y + 2*dy}
	collisionType, tileExists := collisionMap[fmt.Sprintf("%d,%d", landing.X, landing.Y)]
	if !tileExists || collisionType != collisionLand {
		return PathNode{}, false
	}
	return landing, true
}

// SpawnTemporaryActor creates a temporary actor, broadcasts it to all relevant
// clients, and registers it for pathed movement. Returns the runtime actor ID.
func (m *PhaserActorManager) SpawnTemporaryActor(name, spriteName, objectType string, x, y, mapID int, moveSpeed time.Duration) int {
	actorID := m.wh.ActorRegistry.AllocateTemporaryID()

	xVal, yVal := x, y
	dir := "DOWN"
	both := "BOTH"

	actor := &PhaserActor{
		ID:              actorID,
		X:               &xVal,
		Y:               &yVal,
		MapID:           mapID,
		ObjectType:      objectType,
		SpriteName:      &spriteName,
		Name:            &name,
		ActionDirection: &dir,
		MovementType:    &both,
		MoveSpeed:       int(moveSpeed.Milliseconds()),
	}

	m.mu.Lock()
	m.walkingActors[actorID] = actor
	m.mu.Unlock()

	m.broadcastActorSpawn(actor, 0)
	log.Printf("[ActorManager] Spawned temporary actor %q (id=%d) at (%d,%d) map %d", name, actorID, x, y, mapID)
	return actorID
}

// DespawnTemporaryActor removes a temporary actor and broadcasts the despawn.
func (m *PhaserActorManager) DespawnTemporaryActor(actorID int) {
	m.mu.Lock()
	actor, ok := m.walkingActors[actorID]
	mapID := 0
	if ok {
		mapID = actor.MapID
	}
	delete(m.walkingActors, actorID)
	delete(m.actorPaths, actorID)
	delete(m.nextActionTimes, actorID)
	m.mu.Unlock()

	if ok {
		m.broadcastActorDespawn(actorID, mapID)
		log.Printf("[ActorManager] Despawned temporary actor id=%d", actorID)
	}
}

// RequestActorMove paths an actor to a destination using A* pathfinding.
// The actor will be moved tile-by-tile in the simulation tick with proper
// broadcast updates so the client animates the walk.
// An optional onComplete callback fires when the actor reaches the destination.
func (m *PhaserActorManager) RequestActorMove(actorID, destX, destY int, onComplete func()) {
	m.mu.RLock()
	actor, ok := m.walkingActors[actorID]
	m.mu.RUnlock()
	if !ok || actor.X == nil || actor.Y == nil {
		log.Printf("[ActorManager] RequestActorMove: actor %d not found or has no position", actorID)
		if onComplete != nil {
			onComplete()
		}
		return
	}

	path := m.FindPath(actor.MapID, *actor.X, *actor.Y, destX, destY)
	if len(path) == 0 {
		log.Printf("[ActorManager] No path for actor %d from (%d,%d) to (%d,%d)", actorID, *actor.X, *actor.Y, destX, destY)
		if onComplete != nil {
			onComplete()
		}
		return
	}

	speed := time.Duration(actor.MoveSpeed) * time.Millisecond
	if speed == 0 {
		speed = 300 * time.Millisecond
	}

	m.mu.Lock()
	m.actorPaths[actorID] = &ActorPathState{
		Actor:        actor,
		Path:         path,
		LastMoveTime: time.Now(),
		MoveSpeed:    speed,
		OnComplete:   onComplete,
	}
	m.mu.Unlock()

	log.Printf("[ActorManager] Actor %d pathing from (%d,%d) to (%d,%d), %d steps",
		actorID, *actor.X, *actor.Y, destX, destY, len(path))
}

// processPathedMovement advances all A*-pathed actors by one tick.
// Called from simulateMovement on every tick.
func (m *PhaserActorManager) processPathedMovement(now time.Time) {
	m.mu.Lock()
	var completed []int
	var updates []*PhaserActor

	for id, ps := range m.actorPaths {
		if len(ps.Path) == 0 {
			completed = append(completed, id)
			continue
		}
		if now.Sub(ps.LastMoveTime) < ps.MoveSpeed {
			continue
		}

		next := ps.Path[0]
		ps.Path = ps.Path[1:]
		ps.LastMoveTime = now

		// Calculate direction
		dir := "DOWN"
		if ps.Actor.X != nil && ps.Actor.Y != nil {
			if next.X > *ps.Actor.X {
				dir = "RIGHT"
			} else if next.X < *ps.Actor.X {
				dir = "LEFT"
			} else if next.Y > *ps.Actor.Y {
				dir = "DOWN"
			} else if next.Y < *ps.Actor.Y {
				dir = "UP"
			}
		}

		newX, newY := next.X, next.Y
		ps.Actor.X = &newX
		ps.Actor.Y = &newY
		ps.Actor.ActionDirection = &dir
		updates = append(updates, ps.Actor)

		if len(ps.Path) == 0 {
			completed = append(completed, id)
		}
	}

	// Collect callbacks before releasing lock
	var callbacks []func()
	for _, id := range completed {
		if ps, ok := m.actorPaths[id]; ok && ps.OnComplete != nil {
			callbacks = append(callbacks, ps.OnComplete)
		}
		delete(m.actorPaths, id)
	}
	m.mu.Unlock()

	// Broadcast position updates
	for _, actor := range updates {
		m.broadcastActorUpdate(actor, 0)
	}

	// Fire completion callbacks outside the lock
	for _, cb := range callbacks {
		cb()
	}
}

func (m *PhaserActorManager) broadcastActorDespawn(actorID int, mapID int) {
	m.broadcastActorDespawnExcept(actorID, mapID, 0)
}

func (m *PhaserActorManager) broadcastActorDespawnExcept(actorID int, mapID int, excludedSessionID int) {
	m.mu.RLock()
	isOverworldMap := mapID == UnifiedOverworldMapID || m.overworldMapIds[mapID]
	m.mu.RUnlock()

	m.wh.sessionManager.ForEachSession(func(ses *session.Session) {
		presence := ses.Presence()
		if !presence.Authenticated {
			return
		}
		if excludedSessionID != 0 && ses.SessionID == excludedSessionID {
			return
		}

		m.mu.RLock()
		playerOnOverworld := presence.MapID == UnifiedOverworldMapID || m.overworldMapIds[presence.MapID]
		m.mu.RUnlock()

		shouldSend := presence.MapID == mapID || (isOverworldMap && playerOnOverworld)

		if shouldSend {
			ses.SendStreamJSON(map[string]interface{}{
				"id": actorID,
			}, opcodes.PhaserActorDespawn)
		}
	})
}
