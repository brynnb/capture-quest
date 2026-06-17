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
	SessionID     int           `json:"sessionId"`
	CharacterID   int           `json:"characterId"`
	CurrentX      int           `json:"currentX"`
	CurrentY      int           `json:"currentY"`
	MapID         int           `json:"mapId"`
	Direction     string        `json:"direction"`
	Path          []PathNode    `json:"path"` // Remaining path to destination
	IsSurfing     bool          `json:"isSurfing,omitempty"`
	WantsBicycle  bool          `json:"wantsBicycle,omitempty"`
	ForcedBicycle bool          `json:"forcedBicycle,omitempty"`
	LastMoveTime  time.Time     `json:"lastMoveTime"`
	LastSaveTime  time.Time     `json:"lastSaveTime"` // Last time we persisted to DB
	MoveSpeed     time.Duration `json:"moveSpeed"`    // Time per tile, including runtime movement effects
}

type playerMovementStep struct {
	state             *PlayerMovementState
	isPathDestination bool
	movementSeq       int
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
	log.Println("[PlayerMovement] Started player state manager")
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

// StopMovement clears any queued server-driven path for a player.
func (m *PlayerMovementManager) StopMovement(charID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}
	state.Path = nil
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
	state.IsSurfing = false
	m.applyBicycleMapRules(state)
}

// UpdateReportedPosition syncs the latest client-reported position.
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
	state.IsSurfing = isSurfableWaterTile(m.wh, mapID, x, y)
	m.applyBicycleMapRules(state)
}

// GetPosition returns the latest server-visible position reported for a player.
func (m *PlayerMovementManager) GetPosition(charID int) (x, y, mapID int, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.players[charID]
	if !exists {
		return 0, 0, 0, false
	}
	return state.CurrentX, state.CurrentY, state.MapID, true
}

// GetDirection returns the latest server-visible facing direction reported for a player.
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
		updates = append(updates, playerMovementStep{
			state:             state,
			isPathDestination: isFinished,
			movementSeq:       nextTile.ClientSeq,
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

				// Update the server-visible position for this forced warp tile.
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

	// Broadcast to nearby players.
	m.actorManager.broadcastActorUpdate(playerActor, ses.SessionID)

	// Forced server-side movement also updates the origin client.
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
