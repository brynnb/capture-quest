package world

import (
	"capturequest/internal/api/opcodes"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/logutil"
	"capturequest/internal/session"
	"log"
	"strings"
	"sync"
	"time"
)

// defaultPlayerMoveSpeed is the base movement speed for players (time per tile).
// This can be overridden per-player by runtime movement effects.
const defaultPlayerMoveSpeed = 200 * time.Millisecond
const bicyclePlayerMoveSpeed = 100 * time.Millisecond

// PlayerMovementState tracks a player's current movement
type PlayerMovementState struct {
	SessionID               int           `json:"sessionId"`
	CharacterID             int           `json:"characterId"`
	CurrentX                int           `json:"currentX"`
	CurrentY                int           `json:"currentY"`
	MapID                   int           `json:"mapId"`
	Direction               string        `json:"direction"`
	Path                    []PathNode    `json:"path"` // Remaining path to destination
	PendingWarpActivationID *int          `json:"pendingWarpActivationId,omitempty"`
	IsSurfing               bool          `json:"isSurfing,omitempty"`
	WantsBicycle            bool          `json:"wantsBicycle,omitempty"`
	ForcedBicycle           bool          `json:"forcedBicycle,omitempty"`
	LastMoveTime            time.Time     `json:"lastMoveTime"`
	LastSaveTime            time.Time     `json:"lastSaveTime"` // Last time we persisted to DB
	MoveSpeed               time.Duration `json:"moveSpeed"`    // Time per tile, including runtime movement effects
}

type playerMovementStep struct {
	state                   *PlayerMovementState
	isPathDestination       bool
	pendingWarpActivationID *int
	movementSeq             int
}

type playerMovementSnapshot struct {
	SessionID   int
	CharacterID int
	CurrentX    int
	CurrentY    int
	MapID       int
	Direction   string
	MoveSpeed   time.Duration
	MovementSeq int
	Bicycle     bool
	Surfing     bool
}

type BicycleToggleState struct {
	WantsRiding  bool `json:"wantsRiding"`
	ActiveRiding bool `json:"activeRiding"`
	ForcedRiding bool `json:"forcedRiding"`
}

// PathNode represents a single tile in a path
type PathNode struct {
	X         int `json:"x"`
	Y         int `json:"y"`
	ClientSeq int `json:"clientSeq,omitempty"`
}

// ClientMoveStep identifies a client-predicted path step so late server
// confirmations can be matched to the exact local prediction they acknowledge.
type ClientMoveStep struct {
	X   int `json:"x"`
	Y   int `json:"y"`
	Seq int `json:"seq"`
}

// PlayerMoveRequestResult describes how a player movement request was handled.
type PlayerMoveRequestResult int

const (
	PlayerMoveRequestStarted PlayerMoveRequestResult = iota
	PlayerMoveRequestNoop
	PlayerMoveRequestBlocked
	PlayerMoveRequestNoPath
)

type PlayerMoveRequestOptions struct {
	ActivateWarpID *int
	ExpectedStart  *PathNode
	ClientPath     []ClientMoveStep
	Direction      string
}

// PlayerMovementManager tracks server-visible player movement for persistence,
// multiplayer broadcasts, and map-trigger checks.
type PlayerMovementManager struct {
	wh           *WorldHandler
	actorManager *PhaserActorManager
	players      map[int]*PlayerMovementState // CharacterID -> state
	mu           sync.RWMutex
	ticker       *time.Ticker
	stopChan     chan struct{}
}

// NewPlayerMovementManager creates a new player movement manager
func NewPlayerMovementManager(wh *WorldHandler, actorManager *PhaserActorManager) *PlayerMovementManager {
	return &PlayerMovementManager{
		wh:           wh,
		actorManager: actorManager,
		players:      make(map[int]*PlayerMovementState),
		stopChan:     make(chan struct{}),
	}
}

// Start begins the movement tick loop
func (m *PlayerMovementManager) Start() {
	m.ticker = time.NewTicker(50 * time.Millisecond) // Check every 50ms for smooth movement
	go func() {
		for {
			select {
			case <-m.ticker.C:
				m.processTick()
			case <-m.stopChan:
				return
			}
		}
	}()
	log.Println("[PlayerMovement] Started server-side movement manager")
}

// Stop stops the movement tick loop
func (m *PlayerMovementManager) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	close(m.stopChan)
}

// RegisterPlayer adds or updates a player's movement state
func (m *PlayerMovementManager) RegisterPlayer(ses *session.Session, charID int, x, y, mapID int, direction string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.wh != nil && m.wh.CutTiles != nil {
		m.wh.CutTiles.ClearCharacter(int64(charID))
	}

	if normalized := normalizeWarpDirection(direction); normalized != "" {
		direction = normalized
	} else {
		direction = "DOWN"
	}

	state := &PlayerMovementState{
		SessionID:    ses.SessionID,
		CharacterID:  charID,
		CurrentX:     x,
		CurrentY:     y,
		MapID:        mapID,
		Direction:    direction,
		Path:         nil,
		LastMoveTime: time.Now(),
		LastSaveTime: time.Now(),
		MoveSpeed:    defaultPlayerMoveSpeed,
	}
	m.applyBicycleMapRules(state)
	m.players[charID] = state
	ses.X = float32(x)
	ses.Y = float32(y)
	ses.MapID = mapID
}

func (m *PlayerMovementManager) isBicycleActive(state *PlayerMovementState) bool {
	return state != nil &&
		(state.WantsBicycle || state.ForcedBicycle) &&
		m.actorManager != nil &&
		m.actorManager.IsOverworld(state.MapID)
}

func (m *PlayerMovementManager) updateMovementSpeed(state *PlayerMovementState) {
	if m.isBicycleActive(state) {
		state.MoveSpeed = bicyclePlayerMoveSpeed
		return
	}
	state.MoveSpeed = defaultPlayerMoveSpeed
}

func (m *PlayerMovementManager) isOverworldMovementMap(mapID int) bool {
	return mapID == UnifiedOverworldMapID ||
		(m.actorManager != nil && m.actorManager.IsOverworld(mapID))
}

func (m *PlayerMovementManager) applyBicycleMapRules(state *PlayerMovementState) {
	if state == nil {
		return
	}
	if !m.isOverworldMovementMap(state.MapID) {
		state.ForcedBicycle = false
		m.updateMovementSpeed(state)
		return
	}
	if isForcedBicycleEntryTile(state.MapID, state.CurrentX, state.CurrentY) {
		state.ForcedBicycle = true
	}
	m.updateMovementSpeed(state)
}

// FlushPlayerPosition immediately saves a player's current position to the database
// Useful when a player disconnects or warp/teleport happens
func (m *PlayerMovementManager) FlushPlayerPosition(charID int) {
	m.mu.RLock()
	state, ok := m.players[charID]
	m.mu.RUnlock()

	if ok {
		m.savePosition(state)
	}
}

func (m *PlayerMovementManager) SendAuthoritativePosition(charID int) {
	m.mu.RLock()
	state, ok := m.players[charID]
	if !ok {
		m.mu.RUnlock()
		return
	}
	snapshot := m.snapshotForState(state, 0)
	m.mu.RUnlock()

	m.sendPositionToSession(snapshot)
}

func (m *PlayerMovementManager) snapshotForState(state *PlayerMovementState, movementSeq int) playerMovementSnapshot {
	return playerMovementSnapshot{
		SessionID:   state.SessionID,
		CharacterID: state.CharacterID,
		CurrentX:    state.CurrentX,
		CurrentY:    state.CurrentY,
		MapID:       state.MapID,
		Direction:   state.Direction,
		MoveSpeed:   state.MoveSpeed,
		MovementSeq: movementSeq,
		Bicycle:     m.isBicycleActive(state),
		Surfing:     state.IsSurfing,
	}
}

// GetMoveSpeed returns a player's current movement speed in milliseconds.
// Returns the default if the player is not registered.
func (m *PlayerMovementManager) GetMoveSpeed(charID int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if state, ok := m.players[charID]; ok {
		return int(state.MoveSpeed.Milliseconds())
	}
	return int(defaultPlayerMoveSpeed.Milliseconds())
}

// ToggleBicycle switches a player's bicycle preference. Riding is active only
// on overworld maps; interiors temporarily use walking speed and walking sprite.
func (m *PlayerMovementManager) ToggleBicycle(charID int) (BicycleToggleState, bool) {
	m.mu.Lock()
	state, ok := m.players[charID]
	if !ok {
		m.mu.Unlock()
		return BicycleToggleState{}, false
	}
	if state.ForcedBicycle && m.isBicycleActive(state) {
		result := BicycleToggleState{
			WantsRiding:  true,
			ActiveRiding: true,
			ForcedRiding: true,
		}
		snapshot := m.snapshotForState(state, 0)
		m.mu.Unlock()

		m.broadcastSnapshot(snapshot)
		return result, true
	}
	state.WantsBicycle = !state.WantsBicycle
	m.applyBicycleMapRules(state)
	result := BicycleToggleState{
		WantsRiding:  state.WantsBicycle,
		ActiveRiding: m.isBicycleActive(state),
		ForcedRiding: state.ForcedBicycle,
	}
	snapshot := m.snapshotForState(state, 0)
	m.mu.Unlock()

	m.broadcastSnapshot(snapshot)
	return result, true
}

func (m *PlayerMovementManager) IsBicycleActive(charID int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.players[charID]
	return ok && m.isBicycleActive(state)
}

func (m *PlayerMovementManager) IsSurfing(charID int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.players[charID]
	return ok && state.IsSurfing
}

// UnregisterPlayer removes a player from movement tracking
func (m *PlayerMovementManager) UnregisterPlayer(charID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.players, charID)
}

// RequestMove starts a player moving to a destination.
func (m *PlayerMovementManager) RequestMove(charID int, destX, destY int) PlayerMoveRequestResult {
	return m.RequestMoveWithOptions(charID, destX, destY, PlayerMoveRequestOptions{})
}

// RequestMoveWithOptions starts a player moving to a destination with optional
// interaction intent, such as clicking a warp mat/door.
func (m *PlayerMovementManager) RequestMoveWithOptions(charID int, destX, destY int, opts PlayerMoveRequestOptions) PlayerMoveRequestResult {
	// Block movement while in battle
	if existing := getBattle(int64(charID)); existing != nil && !existing.IsOver() {
		return PlayerMoveRequestBlocked
	}
	if m.wh != nil && m.wh.TrainerEncounter != nil && m.wh.TrainerEncounter.HasPendingEncounter(int64(charID)) {
		return PlayerMoveRequestBlocked
	}

	m.mu.RLock()
	state, ok := m.players[charID]
	if !ok {
		m.mu.RUnlock()
		logutil.Debugf("[PlayerMovement] Player %d not registered", charID)
		return PlayerMoveRequestBlocked
	}
	startX := state.CurrentX
	startY := state.CurrentY
	mapID := state.MapID
	direction := state.Direction
	if normalizedDirection := normalizeWarpDirection(opts.Direction); normalizedDirection != "" {
		direction = normalizedDirection
	}
	existingPath := copyPathNodes(state.Path)
	planningStartX := startX
	planningStartY := startY
	preservedPrefix := []PathNode(nil)
	if opts.ExpectedStart != nil {
		if opts.ExpectedStart.X == startX && opts.ExpectedStart.Y == startY {
			planningStartX = startX
			planningStartY = startY
		} else if prefix, ok := pathPrefixThrough(existingPath, opts.ExpectedStart.X, opts.ExpectedStart.Y); ok {
			preservedPrefix = prefix
			planningStartX = opts.ExpectedStart.X
			planningStartY = opts.ExpectedStart.Y
		} else {
			m.mu.RUnlock()
			logutil.Debugf("[PlayerMovement] Rejected predicted move for player %d: expected start (%d,%d) is not current (%d,%d) or queued path %v",
				charID, opts.ExpectedStart.X, opts.ExpectedStart.Y, startX, startY, existingPath)
			return PlayerMoveRequestBlocked
		}
	}
	m.mu.RUnlock()

	if planningStartX == destX && planningStartY == destY {
		if opts.ActivateWarpID != nil {
			if len(preservedPrefix) > 0 {
				m.mu.Lock()
				defer m.mu.Unlock()
				state, ok = m.players[charID]
				if !ok || state.MapID != mapID || state.CurrentX != startX || state.CurrentY != startY || !samePathPrefix(state.Path, preservedPrefix) {
					return PlayerMoveRequestBlocked
				}
				state.Path = preservedPrefix
				state.PendingWarpActivationID = cloneIntPtr(opts.ActivateWarpID)
				return PlayerMoveRequestStarted
			}
			if m.tryActivateRequestedWarp(charID, *opts.ActivateWarpID, mapID, planningStartX, planningStartY, direction) {
				return PlayerMoveRequestStarted
			}
			return PlayerMoveRequestBlocked
		}
		if len(preservedPrefix) > 0 {
			m.mu.Lock()
			defer m.mu.Unlock()
			state, ok = m.players[charID]
			if !ok || state.MapID != mapID || state.CurrentX != startX || state.CurrentY != startY || !samePathPrefix(state.Path, preservedPrefix) {
				return PlayerMoveRequestBlocked
			}
			state.Path = preservedPrefix
			state.PendingWarpActivationID = nil
			return PlayerMoveRequestStarted
		}
		return PlayerMoveRequestNoop
	}

	// Calculate path using A*
	path := m.findPath(charID, mapID, planningStartX, planningStartY, destX, destY)
	if len(path) == 0 {
		if opts.ActivateWarpID != nil &&
			m.canActivateRequestedWarpForDestination(*opts.ActivateWarpID, mapID, planningStartX, planningStartY, destX, destY) &&
			m.tryActivateRequestedWarp(charID, *opts.ActivateWarpID, mapID, planningStartX, planningStartY, direction) {
			return PlayerMoveRequestStarted
		}
		if result, attempted := m.tryPushBoulderFromMoveRequest(charID, mapID, planningStartX, planningStartY, destX, destY); attempted {
			if result.Success {
				m.queueStepAfterBoulderPush(charID, planningStartX, planningStartY, mapID, result)
				logutil.Debugf("[PlayerMovement] Player %d pushed boulder %s from (%d,%d) to (%d,%d)",
					charID, result.ObjectName, result.FromX, result.FromY, result.ToX, result.ToY)
				return PlayerMoveRequestStarted
			} else {
				logutil.Debugf("[PlayerMovement] Boulder push attempt for player %d failed: %s", charID, result.Message)
			}
			return PlayerMoveRequestBlocked
		}
		startCollision, startExists := 0, false
		destCollision, destExists := 0, false
		if m.actorManager != nil {
			startCollision, startExists = m.actorManager.CollisionTypeAt(mapID, planningStartX, planningStartY)
			destCollision, destExists = m.actorManager.CollisionTypeAt(mapID, destX, destY)
		}
		logutil.Debugf("[PlayerMovement] No path found for player %d on map %d from (%d,%d exists=%t collision=%d) to (%d,%d exists=%t collision=%d)",
			charID, mapID, planningStartX, planningStartY, startExists, startCollision, destX, destY, destExists, destCollision)
		return PlayerMoveRequestNoPath
	}
	path = applyClientMoveSeqs(path, opts.ClientPath)
	finalPath := append(copyPathNodes(preservedPrefix), path...)

	m.mu.Lock()
	defer m.mu.Unlock()
	state, ok = m.players[charID]
	if !ok {
		return PlayerMoveRequestBlocked
	}
	if state.MapID != mapID || state.CurrentX != startX || state.CurrentY != startY {
		return PlayerMoveRequestBlocked
	}
	if len(preservedPrefix) > 0 && !samePathPrefix(state.Path, preservedPrefix) {
		return PlayerMoveRequestBlocked
	}
	state.Path = finalPath
	state.PendingWarpActivationID = cloneIntPtr(opts.ActivateWarpID)
	logutil.Debugf("[PlayerMovement] Player %d moving from (%d,%d) to (%d,%d), %d steps",
		charID, planningStartX, planningStartY, destX, destY, len(finalPath))
	return PlayerMoveRequestStarted
}

func copyPathNodes(path []PathNode) []PathNode {
	if len(path) == 0 {
		return nil
	}
	copied := make([]PathNode, len(path))
	copy(copied, path)
	return copied
}

func pathPrefixThrough(path []PathNode, x, y int) ([]PathNode, bool) {
	for i, step := range path {
		if step.X == x && step.Y == y {
			return copyPathNodes(path[:i+1]), true
		}
	}
	return nil, false
}

func samePathPrefix(path []PathNode, prefix []PathNode) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i, step := range prefix {
		if path[i].X != step.X || path[i].Y != step.Y {
			return false
		}
	}
	return true
}

func applyClientMoveSeqs(path []PathNode, clientPath []ClientMoveStep) []PathNode {
	if len(path) == 0 || len(path) != len(clientPath) {
		return path
	}
	for i, clientStep := range clientPath {
		if path[i].X != clientStep.X || path[i].Y != clientStep.Y || clientStep.Seq <= 0 {
			return path
		}
	}
	withSeqs := copyPathNodes(path)
	for i, clientStep := range clientPath {
		withSeqs[i].ClientSeq = clientStep.Seq
	}
	return withSeqs
}

// StopMovement clears any queued path for a player, halting server-side movement.
func (m *PlayerMovementManager) StopMovement(charID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}
	state.Path = nil
	state.PendingWarpActivationID = nil
}

// UpdateMapID updates just the map ID for a registered player (used when client reports a different map)
func (m *PlayerMovementManager) UpdateMapID(charID int, mapID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}
	if state.MapID != mapID {
		log.Printf("[PlayerMovement] Updating player %d map from %d to %d", charID, state.MapID, mapID)
		if m.wh != nil && m.wh.CutTiles != nil {
			m.wh.CutTiles.ClearMap(int64(charID), state.MapID)
		}
		state.MapID = mapID
		state.Path = nil // Clear any pending path on old map
		state.PendingWarpActivationID = nil
		m.applyBicycleMapRules(state)
	}
}

// UpdatePosition directly sets a player's position (for warps, spawns, etc.)
func (m *PlayerMovementManager) UpdatePosition(charID int, x, y, mapID int, direction string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}

	if state.MapID != mapID && m.wh != nil && m.wh.CutTiles != nil {
		m.wh.CutTiles.ClearMap(int64(charID), state.MapID)
	}

	state.CurrentX = x
	state.CurrentY = y
	state.MapID = mapID
	state.Direction = direction
	state.Path = nil // Clear any pending path
	state.PendingWarpActivationID = nil
	state.IsSurfing = false
	m.applyBicycleMapRules(state)
}

// UpdateReportedPosition syncs a client-reported position. If the report only
// confirms the server's current tile, preserve any queued path and pending
// interaction intent so multi-step click paths can still finish.
func (m *PlayerMovementManager) UpdateReportedPosition(charID int, x, y, mapID int, direction string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}

	if normalizedDirection := normalizeWarpDirection(direction); normalizedDirection != "" {
		state.Direction = normalizedDirection
	}

	if state.CurrentX == x && state.CurrentY == y && state.MapID == mapID {
		return
	}

	if state.MapID != mapID && m.wh != nil && m.wh.CutTiles != nil {
		m.wh.CutTiles.ClearMap(int64(charID), state.MapID)
	}

	state.CurrentX = x
	state.CurrentY = y
	state.MapID = mapID
	state.Path = nil
	state.PendingWarpActivationID = nil
	state.IsSurfing = false
	m.applyBicycleMapRules(state)
}

// GetPosition returns the current server-side position for a player
func (m *PlayerMovementManager) GetPosition(charID int) (x, y, mapID int, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.players[charID]
	if !exists {
		return 0, 0, 0, false
	}
	return state.CurrentX, state.CurrentY, state.MapID, true
}

// GetDirection returns the current server-side facing direction for a player.
func (m *PlayerMovementManager) GetDirection(charID int) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.players[charID]
	if !exists {
		return "", false
	}
	return state.Direction, true
}

// processTick handles one tick of movement for all players
func (m *PlayerMovementManager) processTick() {
	m.mu.Lock()
	now := time.Now()
	updates := make([]playerMovementStep, 0)

	for _, state := range m.players {
		if len(state.Path) == 0 {
			continue
		}

		// Check if enough time has passed for next move
		if now.Sub(state.LastMoveTime) < state.MoveSpeed {
			continue
		}

		// Pop next tile from path
		nextTile := state.Path[0]
		var efm *EventFlagManager
		if m.wh != nil {
			efm = m.wh.EventFlags
		}
		if m.actorManager != nil && m.actorManager.IsNPCBlockingTileForCharacter(
			int64(state.CharacterID),
			state.MapID,
			nextTile.X,
			nextTile.Y,
			efm,
		) {
			log.Printf("[PlayerMovement] Stopping player %d before occupied NPC tile (%d,%d) on map %d",
				state.CharacterID, nextTile.X, nextTile.Y, state.MapID)
			state.Path = nil
			state.PendingWarpActivationID = nil
			updates = append(updates, playerMovementStep{state: state})
			continue
		}
		state.Path = state.Path[1:]

		// Calculate direction
		if nextTile.X > state.CurrentX {
			state.Direction = "RIGHT"
		} else if nextTile.X < state.CurrentX {
			state.Direction = "LEFT"
		} else if nextTile.Y > state.CurrentY {
			state.Direction = "DOWN"
		} else if nextTile.Y < state.CurrentY {
			state.Direction = "UP"
		}

		// Update position
		state.CurrentX = nextTile.X
		state.CurrentY = nextTile.Y
		state.LastMoveTime = now
		m.applyBicycleMapRules(state)
		if state.IsSurfing && m.actorManager != nil {
			if collisionType, exists := m.actorManager.CollisionTypeAt(state.MapID, state.CurrentX, state.CurrentY); exists && collisionType != collisionWater {
				state.IsSurfing = false
			}
		}

		// Decide if we should save to DB this tick
		// 1. We just finished the path
		// 2. OR it's been more than 5 seconds since last save
		isFinished := len(state.Path) == 0
		var pendingWarpActivationID *int
		if isFinished {
			pendingWarpActivationID = cloneIntPtr(state.PendingWarpActivationID)
			state.PendingWarpActivationID = nil
		}
		updates = append(updates, playerMovementStep{
			state:                   state,
			isPathDestination:       isFinished,
			pendingWarpActivationID: pendingWarpActivationID,
			movementSeq:             nextTile.ClientSeq,
		})
		shouldSave := isFinished || now.Sub(state.LastSaveTime) >= 5*time.Second

		if shouldSave {
			m.savePosition(state)
			state.LastSaveTime = now
		}
	}
	m.mu.Unlock()

	// Broadcast position updates to clients (Every tick for smooth animation)
	for _, update := range updates {
		m.broadcastPosition(update.state, update.movementSeq)
	}

	// Check trainer sight ranges for each player that moved
	for _, update := range updates {
		state := update.state
		ses, ok := m.wh.sessionManager.GetSession(state.SessionID)
		if !ok || !ses.HasValidClient() {
			continue
		}
		charID := int64(state.CharacterID)
		if m.wh.TrainerEncounter.CheckPlayerPosition(charID, state.CurrentX, state.CurrentY, state.MapID, ses) {
			// Trainer spotted the player. The server keeps the player in place
			// while the client locally animates the trainer approach.
			continue
		}

		TickDayCareStep(charID)

		// Safari Zone step check (Phase 11.3) — must come before normal wild encounters
		if m.wh.Safari != nil && IsInSafariZone(state.MapID) {
			if CheckSafariStep(charID, state.CurrentX, state.CurrentY, state.MapID, ses, m.wh) {
				// Safari encounter triggered or steps expired — stop movement
				m.mu.Lock()
				if ps, ok := m.players[state.CharacterID]; ok {
					ps.Path = nil
				}
				m.mu.Unlock()
				continue
			}
			// In safari zone, skip normal wild encounters
			continue
		}

		// Coordinate-trigger cutscenes get first chance after explicit Safari/trainer
		// movement handling. Some source scripts, like Pokemon Tower 5F's purified
		// zone, suppress encounters before displaying their cutscene.
		if m.tryTriggerCoordinateCutscene(state, charID, ses) {
			continue
		}

		// Check for wild encounters on each step (Phase 5.1)
		if m.wh.WildEncounter != nil && !m.isWildEncounterSuppressed(state, charID) {
			if m.wh.WildEncounter.CheckPlayerStep(charID, state.CurrentX, state.CurrentY, state.MapID, ses) {
				// Wild encounter triggered — stop the player's path
				m.mu.Lock()
				if ps, ok := m.players[state.CharacterID]; ok {
					ps.Path = nil // Clear remaining path
				}
				m.mu.Unlock()
				continue
			}
		}

		// Check spin/arrow tiles — force the player to slide along a path
		if m.wh.SpinTiles != nil {
			mapName := ""
			if m.wh.Cutscenes != nil {
				mapName = m.wh.Cutscenes.MapNameForID(state.MapID)
			}
			if mapName != "" {
				if st := m.wh.SpinTiles.CheckTile(mapName, state.CurrentX, state.CurrentY); st != nil {
					logutil.Debugf("[PlayerMovement] Spin tile at (%d,%d) map %s, forcing movement",
						state.CurrentX, state.CurrentY, mapName)

					// Expand the compact movements into individual steps and prepend to path
					expanded := ExpandMovements(st.Movements)
					spinPath := make([]PathNode, 0, len(expanded))
					x, y := state.CurrentX, state.CurrentY
					for _, dir := range expanded {
						switch dir {
						case "UP":
							y--
						case "DOWN":
							y++
						case "LEFT":
							x--
						case "RIGHT":
							x++
						}
						spinPath = append(spinPath, PathNode{X: x, Y: y})
					}

					// Replace the current path with the spin path
					m.mu.Lock()
					if ps, ok := m.players[state.CharacterID]; ok {
						ps.Path = spinPath
					}
					m.mu.Unlock()
				}
			}
		}

		// Check Seafoam Islands currents — force the player along source movement paths.
		if m.wh.EventFlags != nil && m.wh.Cutscenes != nil {
			mapName := m.wh.Cutscenes.MapNameForID(state.MapID)
			if current, ok := SeafoamCurrentAt(charID, mapName, state.CurrentX, state.CurrentY, m.wh.EventFlags); ok {
				logutil.Debugf("[PlayerMovement] Seafoam current %s at (%d,%d), forcing movement",
					current.Label, state.CurrentX, state.CurrentY)
				currentPath := SeafoamCurrentPath(state.CurrentX, state.CurrentY, current.Movements)
				m.mu.Lock()
				if ps, ok := m.players[state.CharacterID]; ok {
					ps.Path = currentPath
				}
				m.mu.Unlock()
				continue
			}
		}

		if update.isPathDestination && m.tryActivateNormalMapWarp(state, update.pendingWarpActivationID) {
			continue
		}

		// Check warp pad tiles only when this step is the requested destination.
		// Keyboard moves are single-step paths, so deliberate step-on warps still
		// fire, while long click paths can cross exit tiles without hijacking.
		if update.isPathDestination && m.wh.WarpTiles != nil {
			if wt := m.wh.WarpTiles.CheckTile(state.MapID, state.CurrentX, state.CurrentY); wt != nil {
				logutil.Debugf("[PlayerMovement] Warp tile at (%d,%d) map %d -> map %d (%d,%d)",
					state.CurrentX, state.CurrentY, state.MapID,
					wt.DestMapID, wt.DestX, wt.DestY)

				ses, ok := m.wh.sessionManager.GetSession(state.SessionID)
				if ok && m.isSafariEntryWarpBlocked(int64(state.CharacterID), state.MapID, wt.DestMapID, ses) {
					state.Path = nil
					continue
				}

				endSafariSessionIfLeavingMap(int64(state.CharacterID), state.MapID, wt.DestMapID, m.wh)

				previousMapID := state.MapID

				// Update server-side position
				state.CurrentX = wt.DestX
				state.CurrentY = wt.DestY
				state.MapID = wt.DestMapID
				state.Path = nil
				m.applyBicycleMapRules(state)

				// Send teleport notification to client
				if ok && ses.HasValidClient() {
					ses.X = float32(wt.DestX)
					ses.Y = float32(wt.DestY)
					if m.wh.ActorManager.IsOverworld(wt.DestMapID) {
						ses.MapID = UnifiedOverworldMapID
					} else {
						ses.MapID = wt.DestMapID
					}

					if char := ses.Client.CharData(); char != nil {
						char.X = float64(wt.DestX)
						char.Y = float64(wt.DestY)
						char.MapID = uint32(ses.MapID) // Use normalized ID (9999 for overworld)
					}

					broadcastPlayerVisibleMapChange(ses, m.wh, previousMapID)

					ses.SendStreamJSON(map[string]interface{}{
						"mapId": wt.DestMapID,
						"x":     wt.DestX,
						"y":     wt.DestY,
					}, opcodes.WarpTileTeleportNotify)

					m.savePosition(state)
				}
			}
		}
	}
}

func (m *PlayerMovementManager) tryActivateNormalMapWarp(state *PlayerMovementState, activationWarpID *int) bool {
	if m.wh == nil || m.wh.phaserWarps == nil {
		return false
	}

	var warp *phaserMapWarp
	if activationWarpID != nil {
		warp = m.wh.phaserWarps.warpByID(*activationWarpID)
		if warp == nil || !warp.canActivateByClick(state.MapID, state.CurrentX, state.CurrentY, m.wh.ActorManager) {
			return false
		}
	} else {
		warp = m.wh.phaserWarps.warpAt(state.MapID, state.CurrentX, state.CurrentY)
		if warp == nil || !warp.canActivateOnPathDestination(state.MapID, m.wh.ActorManager) {
			return false
		}
	}
	if m.warpBlockedByNPC(state.CharacterID, state.MapID, warp) {
		return false
	}

	direction := warp.activationFacingDirection(state.MapID, state.CurrentX, state.CurrentY, state.Direction, m.wh.ActorManager)
	return m.activateWarpForState(state, warp, direction)
}

func (m *PlayerMovementManager) canActivateRequestedWarpForDestination(warpID, mapID, playerX, playerY, destX, destY int) bool {
	if m.wh == nil || m.wh.phaserWarps == nil {
		return false
	}
	warp := m.wh.phaserWarps.warpByID(warpID)
	return warp != nil && warp.canActivateForRequestedDestination(mapID, playerX, playerY, destX, destY, m.wh.ActorManager)
}

func (m *PlayerMovementManager) tryActivateRequestedWarp(charID, warpID, mapID, x, y int, fallbackDirection string) bool {
	if m.wh == nil || m.wh.phaserWarps == nil {
		return false
	}
	warp := m.wh.phaserWarps.warpByID(warpID)
	if warp == nil || !warp.canActivateByClick(mapID, x, y, m.wh.ActorManager) {
		return false
	}
	if m.warpBlockedByNPC(charID, mapID, warp) {
		return false
	}

	m.mu.RLock()
	state, ok := m.players[charID]
	m.mu.RUnlock()
	if !ok || state.MapID != mapID || state.CurrentX != x || state.CurrentY != y {
		return false
	}

	direction := warp.activationFacingDirection(mapID, x, y, fallbackDirection, m.wh.ActorManager)
	return m.activateWarpForState(state, warp, direction)
}

func (m *PlayerMovementManager) TryActivateDirectionalWarpFromFacingAttempt(charID int, mapID, x, y int, direction string) bool {
	if m.wh == nil || m.wh.phaserWarps == nil {
		return false
	}
	warp := m.wh.phaserWarps.directionalWarpForFacingAttempt(mapID, x, y, direction, m.wh.ActorManager)
	if warp == nil {
		return false
	}
	if m.warpBlockedByNPC(charID, mapID, warp) {
		return false
	}

	m.mu.RLock()
	state, ok := m.players[charID]
	m.mu.RUnlock()
	if !ok || state.MapID != mapID || state.CurrentX != x || state.CurrentY != y {
		return false
	}

	return m.activateWarpForState(state, warp, normalizeWarpDirection(direction))
}

func (m *PlayerMovementManager) QueueDirectionalWarpActivationAtPredictedPosition(charID int, mapID, x, y int, direction string) bool {
	normalizedDirection := normalizeWarpDirection(direction)
	if normalizedDirection == "" || m.wh == nil || m.wh.phaserWarps == nil {
		return false
	}

	warp := m.wh.phaserWarps.directionalWarpForFacingAttempt(mapID, x, y, normalizedDirection, m.wh.ActorManager)
	if warp == nil {
		return false
	}
	if m.warpBlockedByNPC(charID, mapID, warp) {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok || state.MapID != mapID {
		return false
	}
	prefix, ok := pathPrefixThrough(state.Path, x, y)
	if !ok {
		return false
	}
	state.Path = prefix
	state.PendingWarpActivationID = cloneIntPtr(&warp.ID)
	state.Direction = normalizedDirection
	return true
}

func (m *PlayerMovementManager) warpBlockedByNPC(charID, playerMapID int, warp *phaserMapWarp) bool {
	if m == nil || m.actorManager == nil || warp == nil {
		return false
	}

	blockerMapID := warp.SourceMapID
	if playerMapID == UnifiedOverworldMapID && m.actorManager.IsOverworld(warp.SourceMapID) {
		blockerMapID = UnifiedOverworldMapID
	}

	var efm *EventFlagManager
	if m.wh != nil {
		efm = m.wh.EventFlags
	}
	return m.actorManager.IsNPCBlockingTileForCharacter(
		int64(charID),
		blockerMapID,
		warp.X,
		warp.Y,
		efm,
	)
}

func (m *PlayerMovementManager) activateWarpForState(state *PlayerMovementState, warp *phaserMapWarp, direction string) bool {
	if warp == nil {
		return false
	}
	normalizedMapID := warp.DestMapID
	if m.wh != nil && m.wh.ActorManager != nil && m.wh.ActorManager.IsOverworld(warp.DestMapID) {
		normalizedMapID = UnifiedOverworldMapID
	}
	if normalizeWarpDirection(direction) == "" {
		direction = "DOWN"
	}

	ses, hasSession := m.wh.sessionManager.GetSession(state.SessionID)
	if hasSession && m.isSafariEntryWarpBlocked(int64(state.CharacterID), state.MapID, warp.DestMapID, ses) {
		state.Path = nil
		state.PendingWarpActivationID = nil
		return false
	}

	logutil.Debugf("[PlayerMovement] Activating %s warp %d at (%d,%d) map %d -> map %d (%d,%d)",
		normalizeWarpType(warp.WarpType), warp.ID, warp.X, warp.Y, state.MapID,
		warp.DestMapID, warp.DestX, warp.DestY)

	previousMapID := state.MapID

	endSafariSessionIfLeavingMap(int64(state.CharacterID), state.MapID, warp.DestMapID, m.wh)

	state.CurrentX = warp.DestX
	state.CurrentY = warp.DestY
	state.MapID = normalizedMapID
	state.Direction = direction
	state.Path = nil
	state.PendingWarpActivationID = nil
	state.IsSurfing = false
	m.applyBicycleMapRules(state)

	if hasSession && ses.HasValidClient() {
		ses.X = float32(warp.DestX)
		ses.Y = float32(warp.DestY)
		ses.MapID = normalizedMapID

		if char := ses.Client.CharData(); char != nil {
			char.X = float64(warp.DestX)
			char.Y = float64(warp.DestY)
			char.MapID = uint32(normalizedMapID)
		}

		broadcastPlayerVisibleMapChange(ses, m.wh, previousMapID)

		ses.SendStreamJSON(map[string]interface{}{
			"mapId":     warp.DestMapID,
			"x":         warp.DestX,
			"y":         warp.DestY,
			"direction": direction,
		}, opcodes.WarpTileTeleportNotify)
	}

	m.savePosition(state)
	return true
}

func (m *PlayerMovementManager) isSafariEntryWarpBlocked(charID int64, sourceMapID, destMapID int, ses *session.Session) bool {
	if sourceMapID != SafariZoneGateMapID || !IsInSafariZone(destMapID) {
		return false
	}
	if m.wh != nil && m.wh.Safari != nil {
		if safari := m.wh.Safari.GetSession(charID); safari != nil && safari.Active {
			return false
		}
	}
	if ses != nil {
		SendSystemMessage(ses, "Please check in at the counter first.")
	}
	log.Printf("[Safari] Blocked unpaid Safari Zone entry warp for player %d from map %d to map %d", charID, sourceMapID, destMapID)
	return true
}

func (m *PlayerMovementManager) tryTriggerCoordinateCutscene(state *PlayerMovementState, charID int64, ses *session.Session) bool {
	if m.wh == nil || m.wh.CoordTriggers == nil || m.wh.Cutscenes == nil || m.wh.EventFlags == nil {
		return false
	}

	triggers := m.wh.CoordTriggers.CheckTileTriggers(state.MapID, state.CurrentX, state.CurrentY)
	if len(triggers) == 0 {
		return false
	}

	logutil.Debugf("[PlayerMovement] Coordinate triggers at (%d,%d) map %d: %v",
		state.CurrentX, state.CurrentY, state.MapID, triggerLabels(triggers))

	for _, trigger := range triggers {
		cs := m.wh.Cutscenes.FindEligibleCoordCutsceneForTrigger(trigger, charID, m.wh.EventFlags, state.Direction)
		if cs == nil {
			continue
		}

		SendCutsceneToPlayer(ses, cs, m.wh)
		m.mu.Lock()
		if ps, ok := m.players[state.CharacterID]; ok {
			ps.Path = nil
			ps.PendingWarpActivationID = nil
		}
		m.mu.Unlock()
		return true
	}

	return false
}

func (m *PlayerMovementManager) isWildEncounterSuppressed(state *PlayerMovementState, charID int64) bool {
	if m.wh == nil || m.wh.Cutscenes == nil || m.wh.EventFlags == nil || state == nil {
		return false
	}

	mapName := m.wh.Cutscenes.MapNameForID(state.MapID)
	if !strings.EqualFold(mapName, "POKEMON_TOWER_5F") {
		return false
	}

	const purifiedZoneFlag = "EVENT_IN_PURIFIED_ZONE"
	if isPokemonTower5FPurifiedZone(mapName, state.CurrentX, state.CurrentY) {
		return true
	}

	if m.wh.EventFlags.CheckFlag(charID, purifiedZoneFlag) {
		if err := m.wh.EventFlags.ResetFlag(charID, purifiedZoneFlag); err != nil {
			log.Printf("[PlayerMovement] Failed to reset %s for player %d outside Pokemon Tower 5F purified zone: %v",
				purifiedZoneFlag, charID, err)
		}
	}
	return false
}

func isPokemonTower5FPurifiedZone(mapName string, x, y int) bool {
	if !strings.EqualFold(mapName, "POKEMON_TOWER_5F") {
		return false
	}
	return (x == 10 || x == 11) && (y == 8 || y == 9)
}

func triggerLabels(triggers []CoordinateTrigger) []string {
	labels := make([]string, len(triggers))
	for i, trigger := range triggers {
		labels[i] = trigger.Label
	}
	return labels
}

func (m *PlayerMovementManager) MovePlayerTo(charID int, x, y, mapID int, direction string, isSurfing bool) bool {
	normalizedDirection := normalizeWarpDirection(direction)
	if normalizedDirection == "" {
		normalizedDirection = "DOWN"
	}

	m.mu.Lock()
	state, ok := m.players[charID]
	if !ok {
		m.mu.Unlock()
		return false
	}
	state.CurrentX = x
	state.CurrentY = y
	state.MapID = mapID
	state.Direction = normalizedDirection
	state.Path = nil
	state.PendingWarpActivationID = nil
	state.IsSurfing = isSurfing
	m.applyBicycleMapRules(state)
	m.mu.Unlock()

	m.syncSessionPosition(state)

	m.savePosition(state)
	m.broadcastPosition(state, 0)
	return true
}

func (m *PlayerMovementManager) syncSessionPosition(state *PlayerMovementState) {
	if m.wh == nil || m.wh.sessionManager == nil || state == nil {
		return
	}
	ses, ok := m.wh.sessionManager.GetSession(state.SessionID)
	if !ok || !ses.HasValidClient() {
		return
	}

	ses.X = float32(state.CurrentX)
	ses.Y = float32(state.CurrentY)
	ses.MapID = state.MapID
	if char := ses.Client.CharData(); char != nil {
		char.X = float64(state.CurrentX)
		char.Y = float64(state.CurrentY)
		char.MapID = uint32(state.MapID)
	}
}

// broadcastPosition sends position update to all relevant clients
func (m *PlayerMovementManager) broadcastPosition(state *PlayerMovementState, movementSeq int) {
	m.broadcastSnapshot(m.snapshotForState(state, movementSeq))
}

func (m *PlayerMovementManager) broadcastSnapshot(snapshot playerMovementSnapshot) {
	if m.wh == nil || m.actorManager == nil {
		return
	}
	ses, playerActor, ok := m.playerActorForSnapshot(snapshot)
	if !ok {
		return
	}

	// Broadcast to all players (excluding the originating session to avoid rubber-banding)
	m.actorManager.broadcastActorUpdate(playerActor, ses.SessionID)

	// Also send directly to the player themselves for prediction confirmation.
	ses.SendStreamJSON(StructToMap(*playerActor), opcodes.PhaserActorPositionUpdate)
}

func (m *PlayerMovementManager) sendPositionToSession(snapshot playerMovementSnapshot) {
	if m.wh == nil {
		return
	}
	ses, playerActor, ok := m.playerActorForSnapshot(snapshot)
	if !ok {
		return
	}
	ses.SendStreamJSON(StructToMap(*playerActor), opcodes.PhaserActorPositionUpdate)
}

func (m *PlayerMovementManager) playerActorForSnapshot(snapshot playerMovementSnapshot) (*session.Session, *PhaserActor, bool) {
	if m.wh == nil || m.wh.sessionManager == nil || m.wh.ActorRegistry == nil {
		return nil, nil, false
	}
	ses, ok := m.wh.sessionManager.GetSession(snapshot.SessionID)
	if !ok || !ses.HasValidClient() {
		return nil, nil, false
	}

	char := ses.Client.CharData()
	if char == nil {
		return nil, nil, false
	}

	ses.X = float32(snapshot.CurrentX)
	ses.Y = float32(snapshot.CurrentY)
	ses.MapID = snapshot.MapID
	char.X = float64(snapshot.CurrentX)
	char.Y = float64(snapshot.CurrentY)
	char.MapID = uint32(snapshot.MapID)

	spriteName := playerSpriteName(char.Gender, snapshot.Bicycle, snapshot.Surfing)

	x := snapshot.CurrentX
	y := snapshot.CurrentY
	both := "BOTH"
	var movementSeq *int
	if snapshot.MovementSeq > 0 {
		seq := snapshot.MovementSeq
		movementSeq = &seq
	}

	playerActor := PhaserActor{
		ID:              m.wh.ActorRegistry.GetPhaserID(ActorTypePlayer, snapshot.CharacterID),
		InternalID:      snapshot.CharacterID,
		X:               &x,
		Y:               &y,
		MapID:           snapshot.MapID,
		ObjectType:      "player",
		SpriteName:      &spriteName,
		Name:            &char.Name,
		ActionDirection: &snapshot.Direction,
		MovementType:    &both,
		MoveSpeed:       int(snapshot.MoveSpeed.Milliseconds()),
		MovementSeq:     movementSeq,
	}
	return ses, &playerActor, true
}

// savePosition persists position to database
func (m *PlayerMovementManager) savePosition(state *PlayerMovementState) {
	err := db_character.UpdateCharacterPosition(
		int32(state.CharacterID),
		uint32(state.MapID),
		float64(state.CurrentX),
		float64(state.CurrentY),
		0, // Z
		0, // Heading
	)
	if err != nil {
		log.Printf("[PlayerMovement] Failed to save position for player %d: %v", state.CharacterID, err)
	}
}

// findPath delegates to the shared A* implementation on PhaserActorManager.
func (m *PlayerMovementManager) findPath(charID int, mapID, startX, startY, endX, endY int) []PathNode {
	logutil.Debugf("[PlayerMovement] Finding path from (%d,%d) to (%d,%d) on map %d",
		startX, startY, endX, endY, mapID)
	if m.wh != nil && m.wh.EventFlags != nil {
		return m.actorManager.FindPathForCharacterWithOptions(
			int64(charID),
			mapID,
			startX,
			startY,
			endX,
			endY,
			m.wh.EventFlags,
			pathfindOptions{AllowWater: m.isPlayerSurfing(charID)},
		)
	}
	return m.actorManager.FindPath(mapID, startX, startY, endX, endY)
}

func (m *PlayerMovementManager) isPlayerSurfing(charID int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.players[charID]
	return ok && state.IsSurfing
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// A* node for pathfinding
type AStarNode struct {
	X      int        `json:"x"`
	Y      int        `json:"y"`
	G      int        `json:"g"`
	H      int        `json:"h"`
	F      int        `json:"f"`
	Parent *AStarNode `json:"parent,omitempty"`
	index  int        // For priority queue
}

// Priority queue implementation for A*
type PriorityQueue []*AStarNode

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].F < pq[j].F
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	node := x.(*AStarNode)
	node.index = n
	*pq = append(*pq, node)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	node := old[n-1]
	old[n-1] = nil
	node.index = -1
	*pq = old[0 : n-1]
	return node
}
