package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/logutil"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"context"
	"errors"
	"fmt"
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
	SessionID       int        `json:"sessionId"`
	CharacterID     int        `json:"characterId"`
	CurrentX        int        `json:"currentX"`
	CurrentY        int        `json:"currentY"`
	MapID           int        `json:"mapId"`
	PreviousMapID   int        `json:"previousMapId,omitempty"`
	Direction       string     `json:"direction"`
	Path            []PathNode `json:"path"` // Remaining path to destination
	IsSurfing       bool       `json:"isSurfing,omitempty"`
	WantsBicycle    bool       `json:"wantsBicycle,omitempty"`
	BicycleRevision int64      `json:"bicycleRevision"`
	ForcedBicycle   bool       `json:"forcedBicycle,omitempty"`
	LastMoveTime    time.Time  `json:"lastMoveTime"`
	LastSaveTime    time.Time  `json:"lastSaveTime"` // Last time we persisted to DB
	pendingStep     *issuedPlayerStep
	positionDirty   bool
	lastSaveAttempt time.Time
	MoveSpeed       time.Duration `json:"moveSpeed"` // Time per tile, including runtime movement effects
}

type playerMovementStep struct {
	state             *PlayerMovementState
	isPathDestination bool
	movementSeq       int
}

type playerMovementSnapshot struct {
	SessionID    int
	CharacterID  int
	CurrentX     int
	CurrentY     int
	MapID        int
	Direction    string
	MoveSpeed    time.Duration
	MovementSeq  int
	PathFinished bool
	Bicycle      bool
	Surfing      bool
}

type BicycleToggleState struct {
	Revision     int64 `json:"revision"`
	WantsRiding  bool  `json:"wantsRiding"`
	ActiveRiding bool  `json:"activeRiding"`
	ForcedRiding bool  `json:"forcedRiding"`
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
	worker       periodicWorker
}

// NewPlayerMovementManager creates a new player movement manager
func NewPlayerMovementManager(wh *WorldHandler, actorManager *PhaserActorManager) *PlayerMovementManager {
	return &PlayerMovementManager{
		wh:           wh,
		actorManager: actorManager,
		players:      make(map[int]*PlayerMovementState),
	}
}

// Start begins the movement tick loop
func (m *PlayerMovementManager) Start() {
	m.worker.start(50*time.Millisecond, nil, m.processTick)
}

// Stop waits for movement persistence and step effects already in flight.
func (m *PlayerMovementManager) Stop() { m.worker.stop() }

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
		SessionID:     ses.SessionID,
		CharacterID:   charID,
		CurrentX:      x,
		CurrentY:      y,
		MapID:         mapID,
		PreviousMapID: -1,
		Direction:     direction,
		Path:          nil,
		LastMoveTime:  time.Now(),
		LastSaveTime:  time.Now(),
		MoveSpeed:     defaultPlayerMoveSpeed,
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
func (m *PlayerMovementManager) FlushPlayerPosition(ctx context.Context, charID int) error {
	m.mu.Lock()
	state := m.players[charID]
	if state == nil || !state.positionDirty {
		m.mu.Unlock()
		return nil
	}
	snapshot := *state
	state.lastSaveAttempt = time.Now()
	m.mu.Unlock()
	if err := commitPlayerPosition(ctx, m.wh.database, int64(charID), snapshot.MapID, snapshot.CurrentX, snapshot.CurrentY); err != nil {
		log.Printf("[PlayerMovement] Save position for %d: %v", charID, err)
		return err
	}
	m.mu.Lock()
	if current := m.players[charID]; current == state && current.CurrentX == snapshot.CurrentX && current.CurrentY == snapshot.CurrentY && current.MapID == snapshot.MapID {
		current.LastSaveTime = time.Now()
		current.positionDirty = false
	}
	m.mu.Unlock()
	return nil
}
func (m *PlayerMovementManager) markPositionCommitted(charID, x, y, mapID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state := m.players[charID]; state != nil && state.CurrentX == x && state.CurrentY == y && state.MapID == mapID {
		state.LastSaveTime = time.Now()
		state.positionDirty = false
	}
}

func (m *PlayerMovementManager) snapshotForState(state *PlayerMovementState, movementSeq int) playerMovementSnapshot {
	return playerMovementSnapshot{
		SessionID:    state.SessionID,
		CharacterID:  state.CharacterID,
		CurrentX:     state.CurrentX,
		CurrentY:     state.CurrentY,
		MapID:        state.MapID,
		Direction:    state.Direction,
		MoveSpeed:    state.MoveSpeed,
		MovementSeq:  movementSeq,
		PathFinished: len(state.Path) == 0,
		Bicycle:      m.isBicycleActive(state),
		Surfing:      state.IsSurfing,
	}
}

// Peer actor reads join immutable presence to one matching movement snapshot.
// Character ID alone is insufficient after a session replaces its registration.
func (m *PlayerMovementManager) snapshotForPresence(p session.Presence, mapID, x, y int) (playerMovementSnapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := m.players[int(p.CharacterID)]
	if state == nil || state.SessionID != p.SessionID || state.MapID != mapID || state.CurrentX != x || state.CurrentY != y {
		return playerMovementSnapshot{}, false
	}
	return m.snapshotForState(state, 0), true
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

// Bicycle preference belongs to this registered movement session. Desired state
// plus its revision prevents duplicate packets from toggling the preference back.
func (m *PlayerMovementManager) bicycleStateForSession(sesID, charID int) (BicycleToggleState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state := m.players[charID]
	if state == nil || state.SessionID != sesID {
		return BicycleToggleState{}, false
	}
	return BicycleToggleState{Revision: state.BicycleRevision, WantsRiding: state.WantsBicycle, ActiveRiding: m.isBicycleActive(state), ForcedRiding: state.ForcedBicycle}, true
}
func (m *PlayerMovementManager) setBicycleForSession(sesID, charID int, wants bool, revision int64) (BicycleToggleState, bool) {
	m.mu.Lock()
	state := m.players[charID]
	if state == nil || state.SessionID != sesID || state.BicycleRevision != revision {
		m.mu.Unlock()
		return BicycleToggleState{}, false
	}
	if state.ForcedBicycle && m.isBicycleActive(state) {
		wants = state.WantsBicycle
	}
	state.WantsBicycle = wants
	state.BicycleRevision++
	m.applyBicycleMapRules(state)
	result := BicycleToggleState{Revision: state.BicycleRevision, WantsRiding: state.WantsBicycle, ActiveRiding: m.isBicycleActive(state), ForcedRiding: state.ForcedBicycle}
	snapshot := m.snapshotForState(state, 0)
	m.mu.Unlock()
	m.broadcastSnapshot(snapshot, false)
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

// retirePosition removes the live writer and retains only a dirty final pose.
// Recovery runs behind character admission, so it cannot overwrite a new owner.
func (m *PlayerMovementManager) retirePosition(charID int) *playerMovementSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.players[charID]
	delete(m.players, charID)
	if state == nil || !state.positionDirty {
		return nil
	}
	snapshot := m.snapshotForState(state, 0)
	return &snapshot
}

// StopMovement clears any queued server-driven path for a player.
func (m *PlayerMovementManager) StopMovement(charID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}
	state.pendingStep = nil
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
		previousMapID := state.MapID
		if m.wh != nil && m.wh.phaserWarps != nil {
			if warp := m.wh.phaserWarps.warpAt(state.MapID, state.CurrentX, state.CurrentY); warp != nil {
				previousMapID = warp.SourceMapID
			}
		}
		state.PreviousMapID = previousMapID
		state.MapID = mapID
		if m.wh != nil && m.wh.sessionManager != nil {
			if ses, found := m.wh.sessionManager.GetSession(state.SessionID); found {
				ses.PreviousMapID = previousMapID
			}
		}
		state.pendingStep = nil
		state.Path = nil // Clear any pending path on old map
		m.applyBicycleMapRules(state)
	}
}

// projectCommittedTeleport replaces the owner's position after a committed
// warp/cutscene. Unlike an ordinary step, it always retires the current path.
// It must never manufacture a deferred position write between publication calls.
func (m *PlayerMovementManager) projectCommittedTeleport(charID int, x, y, mapID int, direction string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}

	if state.MapID != mapID && m.wh != nil && m.wh.CutTiles != nil {
		m.wh.CutTiles.ClearMap(int64(charID), state.MapID)
	}

	if state.MapID != mapID {
		state.PreviousMapID = state.MapID
		if m.wh != nil && m.wh.sessionManager != nil {
			if ses, found := m.wh.sessionManager.GetSession(state.SessionID); found {
				ses.PreviousMapID = state.MapID
			}
		}
	}
	state.pendingStep = nil
	state.positionDirty = false
	state.LastSaveTime = time.Now()
	state.CurrentX = x
	state.CurrentY = y
	state.MapID = mapID
	state.Direction = direction
	state.Path = nil // Clear any pending path
	state.IsSurfing = false
	m.applyBicycleMapRules(state)
}

// projectCommittedPosition applies the committed result without database I/O.
func (m *PlayerMovementManager) projectCommittedPosition(charID int, x, y, mapID int, direction string, surfing bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}

	if normalizedDirection := normalizeWarpDirection(direction); normalizedDirection != "" {
		state.Direction = normalizedDirection
	}

	// This is a committed result, not a deferred save obligation.
	state.positionDirty = false
	state.LastSaveTime = time.Now()
	state.IsSurfing = surfing
	if state.CurrentX == x && state.CurrentY == y && state.MapID == mapID {
		m.applyBicycleMapRules(state)
		return
	}

	if state.MapID != mapID && m.wh != nil && m.wh.CutTiles != nil {
		m.wh.CutTiles.ClearMap(int64(charID), state.MapID)
	}
	if state.MapID != mapID {
		previousMapID := state.MapID
		if m.wh != nil && m.wh.phaserWarps != nil {
			if warp := m.wh.phaserWarps.warpAt(state.MapID, state.CurrentX, state.CurrentY); warp != nil {
				previousMapID = warp.SourceMapID
			}
		}
		state.PreviousMapID = previousMapID
		if m.wh != nil && m.wh.sessionManager != nil {
			if ses, found := m.wh.sessionManager.GetSession(state.SessionID); found {
				ses.PreviousMapID = previousMapID
			}
		}
	}

	state.pendingStep = nil
	state.CurrentX = x
	state.CurrentY = y
	state.MapID = mapID
	state.Path = nil
	m.applyBicycleMapRules(state)
}

// ownedPlayerPosition reads the movement registration first and falls back to the
// selected character before registration. Call from the owning session command gate.
func (wh *WorldHandler) ownedPlayerPosition(ses *session.Session) (x, y, mapID int) {
	char := ses.Client.CharData()
	x, y, mapID = int(char.X), int(char.Y), int(char.MapID)
	if wh.PlayerMovement != nil {
		if mx, my, mm, ok := wh.PlayerMovement.GetPosition(int(char.ID)); ok {
			x, y, mapID = mx, my, mm
		}
	}
	return x, y, mapID
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
	if m.wh == nil || m.wh.sessionManager == nil {
		return
	}
	type candidate struct {
		characterID, sessionID int
		state                  *PlayerMovementState
	}
	m.mu.RLock()
	candidates := make([]candidate, 0, len(m.players))
	for id, state := range m.players {
		if len(state.Path) != 0 || state.positionDirty {
			candidates = append(candidates, candidate{id, state.SessionID, state})
		}
	}
	m.mu.RUnlock()
	for _, c := range candidates {
		ses, ok := m.wh.sessionManager.GetSession(c.sessionID)
		if !ok {
			continue
		}
		// Never wait for a session gate while holding the movement lock. Packet
		// handlers and disconnect cleanup take those locks in the reverse order.
		_ = ses.TryExecuteCommand(func() {
			if !ses.HasValidClient() || !m.wh.characterOwners.owns(int64(c.characterID), ses) {
				return
			}
			m.processCharacterTick(ses.CommandContext(), c.characterID, c.state)
		})
	}
}

// Called only within the owning session's command gate. The pointer check rejects
// a movement registration replaced after the timer collected its candidates.
func (m *PlayerMovementManager) processCharacterTick(ctx context.Context, characterID int, expected *PlayerMovementState) {
	if ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	state := m.players[characterID]
	if state == nil || state != expected {
		m.mu.Unlock()
		return
	}
	now := time.Now()
	if len(state.Path) == 0 || now.Sub(state.LastMoveTime) < state.MoveSpeed {
		retry := state.positionDirty && now.Sub(state.lastSaveAttempt) >= 5*time.Second
		m.mu.Unlock()
		if retry {
			_ = m.FlushPlayerPosition(ctx, characterID)
		}
		return
	}
	planned := *state
	planned.Path = append([]PathNode(nil), state.Path...)
	m.mu.Unlock()

	// Dynamic reads and persistence cannot hold the shared player lock. The owner
	// gate prevents another gameplay command from replacing this source/path.
	if m.actorManager != nil {
		blockers, err := m.actorManager.npcBlockingPositionsContext(ctx, m.wh.database, int64(characterID), planned.MapID, m.wh.EventFlags)
		if err != nil {
			logutil.Debugf("[PlayerMovement] Read forced-step blockers for %d: %v", characterID, err)
			return
		}
		target := planned.Path[0]
		for _, blocker := range blockers {
			if blocker.X == target.X && blocker.Y == target.Y {
				err := db.Transaction(ctx, m.wh.database, func(tx db.DBTX) error {
					if err := db.LockCharacter(tx, int64(characterID)); err != nil {
						return err
					}
					route, err := loadMovementRouteIn(tx, int64(characterID))
					if err != nil {
						return err
					}
					if route != nil && (route.MapID != state.MapID || route.X != state.CurrentX || route.Y != state.CurrentY) {
						return fmt.Errorf("blocked route source changed")
					}
					return saveMovementRouteIn(tx, int64(characterID), 0, 0, 0, nil, false)
				})
				if err != nil {
					logutil.Debugf("[PlayerMovement] Retire blocked route for %d: %v", characterID, err)
					m.mu.Lock()
					if m.players[characterID] == state {
						state.LastMoveTime = now
					}
					m.mu.Unlock()
					return
				}
				m.mu.Lock()
				if current := m.players[characterID]; current == state {
					current.Path = nil
				}
				snapshot := m.snapshotForState(state, 0)
				m.mu.Unlock()
				m.broadcastSnapshot(snapshot, true)
				return // A stopped path is not a completed step or a durable step effect.
			}
		}
	}
	update, moved, err := m.planCharacterStep(ctx, &planned, now)
	if err != nil {
		logutil.Debugf("[PlayerMovement] Read forced-step collision for %d: %v", characterID, err)
		return
	}
	if !moved {
		return
	}
	ses, ok := m.wh.sessionManager.GetSession(state.SessionID)
	if !ok || !ses.HasValidClient() {
		return
	}
	effects, err := commitMovementStep(ctx, m.wh, int64(characterID), movementStepCandidate{SourceMap: state.MapID, SourceX: state.CurrentX, SourceY: state.CurrentY, MapID: planned.MapID, X: planned.CurrentX, Y: planned.CurrentY, Direction: planned.Direction, Forced: true, Surfing: planned.IsSurfing, RemainingPath: planned.Path, PathDestination: update.isPathDestination})
	if err != nil {
		logutil.Debugf("[PlayerMovement] Commit forced step for %d: %v", characterID, err)
		// A confirmed domain owner ends this path. Database/commit failures
		// retain it for the existing retry policy; neither can publish a target.
		if errors.Is(err, errBattleOwnership) {
			m.mu.Lock()
			current := m.players[characterID]
			if current != state {
				m.mu.Unlock()
				return
			}
			state.Path = nil
			snapshot := m.snapshotForState(state, 0)
			m.mu.Unlock()
			m.broadcastSnapshot(snapshot, true)
			return
		}
		// Retain source/path and retry at the existing movement cadence, without
		// publishing a position or executing effects after a failed commit.
		m.mu.Lock()
		if current := m.players[characterID]; current == state {
			current.LastMoveTime = now
		}
		m.mu.Unlock()
		return
	}
	if effects.Teleport {
		// Publish from the previous owned map so departure visibility and map
		// provenance survive a recovery or automatic warp in this step.
		publishCommittedPlayerPosition(ses, m.wh, effects.MapID, effects.X, effects.Y, effects.Direction)
	}
	m.mu.Lock()
	if m.players[characterID] != state {
		m.mu.Unlock()
		return
	}
	state.CurrentX, state.CurrentY, state.MapID, state.Direction = effects.X, effects.Y, effects.MapID, effects.Direction
	state.Path, state.LastMoveTime = planned.Path, planned.LastMoveTime
	if effects.StopPath {
		state.Path = nil
	}
	if effects.ForcedPath != nil {
		state.Path = effects.ForcedPath
	}
	if !effects.Teleport {
		state.IsSurfing, state.ForcedBicycle, state.MoveSpeed = effects.Surfing, planned.ForcedBicycle, planned.MoveSpeed
	}
	state.positionDirty = false
	state.LastSaveTime, state.lastSaveAttempt = now, now
	update.state = state
	m.mu.Unlock()
	if !effects.Teleport {
		m.broadcastPosition(state, update.movementSeq)
	}
	publishMovementStepEffects(ses, m.wh, int64(characterID), effects)
}

// planCharacterStep plans a detached candidate under the owner gate.
// It does not publish or persist the candidate.
func (m *PlayerMovementManager) planCharacterStep(ctx context.Context, state *PlayerMovementState, now time.Time) (playerMovementStep, bool, error) {
	if len(state.Path) == 0 {
		return playerMovementStep{}, false, nil
	}

	// Check if enough time has passed for next move
	if now.Sub(state.LastMoveTime) < state.MoveSpeed {
		return playerMovementStep{}, false, nil
	}

	// Pop next tile from path
	nextTile := state.Path[0]
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
	state.positionDirty = true
	m.applyBicycleMapRules(state)
	if state.IsSurfing {
		if m.actorManager == nil || m.wh == nil || m.wh.database == nil {
			return playerMovementStep{}, false, fmt.Errorf("forced collision service unavailable")
		}
		collision, _, err := m.actorManager.baseCollision(ctx, m.wh.database, state.MapID, true)
		if err != nil {
			return playerMovementStep{}, false, err
		}
		if collisionType, exists := collision[tileKey(state.CurrentX, state.CurrentY)]; exists && collisionType != collisionWater {
			state.IsSurfing = false
		}
	}

	update := playerMovementStep{
		state:             state,
		isPathDestination: len(state.Path) == 0,
		movementSeq:       nextTile.ClientSeq,
	}
	return update, true, nil
}

func (m *PlayerMovementManager) isSafariEntryWarpBlocked(ctx context.Context, charID int64, sourceMapID, destMapID int, ses *session.Session) bool {
	if sourceMapID != SafariZoneGateMapID || !IsInSafariZone(destMapID) {
		return false
	}
	if m.wh != nil && m.wh.Safari != nil {
		safari, err := m.wh.Safari.GetSession(ctx, charID)
		if err != nil {
			log.Printf("[Safari] Entry guard for %d: %v", charID, err)
			if ses != nil {
				SendSystemMessage(ses, "Safari state is unavailable. Please try again.")
			}
			return true
		}
		if safari != nil && safari.Active {
			return false
		}
	}
	if ses != nil {
		SendSystemMessage(ses, "Please check in at the counter first.")
	}
	log.Printf("[Safari] Blocked unpaid Safari Zone entry warp for player %d from map %d to map %d", charID, sourceMapID, destMapID)
	return true
}

func isPokemonTower5FPurifiedZone(mapName string, x, y int) bool {
	if !strings.EqualFold(mapName, "POKEMON_TOWER_5F") {
		return false
	}
	return (x == 10 || x == 11) && (y == 8 || y == 9)
}

// Surf entry shares position/Repel/encounter/blackout commit with movement,
// preserving its existing wild-only policy. The handler publishes effects.
func (m *PlayerMovementManager) SurfTo(ctx context.Context, ses *session.Session, x, y, mapID int, direction string) (movementStepResult, error) {
	charID := int(ses.Client.CharData().ID)
	m.mu.RLock()
	state := m.players[charID]
	if state == nil || state.SessionID != ses.SessionID {
		m.mu.RUnlock()
		return movementStepResult{}, fmt.Errorf("SURF movement owner absent")
	}
	sourceMap, sourceX, sourceY := state.MapID, state.CurrentX, state.CurrentY
	m.mu.RUnlock()
	result, err := commitMovementStep(ctx, m.wh, int64(charID), movementStepCandidate{SourceMap: sourceMap, SourceX: sourceX, SourceY: sourceY, MapID: mapID, X: x, Y: y, Direction: normalizeWarpDirection(direction), SurfEntry: true})
	if err != nil {
		return movementStepResult{}, err
	}
	m.projectCommittedPosition(charID, result.X, result.Y, result.MapID, result.Direction, result.Surfing)
	m.mu.Lock()
	if current := m.players[charID]; current == state {
		current.Path = nil
		current.pendingStep = nil
		m.applyBicycleMapRules(current)
	}
	m.mu.Unlock()
	publishCommittedPlayerLocation(ses, m.wh, result.MapID, result.X, result.Y)
	if !result.Teleport {
		m.broadcastPosition(state, 0)
	}
	return result, nil
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
	m.broadcastSnapshot(m.snapshotForState(state, movementSeq), true)
}

func (m *PlayerMovementManager) broadcastSnapshot(snapshot playerMovementSnapshot, serverControlled bool) {
	if m.wh == nil || m.actorManager == nil {
		return
	}
	ses, playerActor, ok := m.playerActorForSnapshot(snapshot)
	if !ok {
		return
	}

	// Broadcast to nearby players.
	m.actorManager.broadcastActorUpdate(playerActor, ses.SessionID)

	// Cosmetic refreshes must not retire a local issued animation. Only committed
	// server path points (or a stopped path) carry authoritative projection.
	if !serverControlled {
		ses.SendStreamJSON(StructToMap(*playerActor), opcodes.PhaserActorPositionUpdate)
		return
	}
	// Forced server-side movement also updates the origin client.
	ses.SendStreamJSON(protocol.ServerPlayerMovementNotify{SpriteName: *playerActor.SpriteName, ActorID: playerActor.ID, MapID: snapshot.MapID, X: snapshot.CurrentX, Y: snapshot.CurrentY, Direction: snapshot.Direction, MoveSpeed: int(snapshot.MoveSpeed.Milliseconds()), PathFinished: snapshot.PathFinished}, opcodes.ServerPlayerMovementNotify)
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
	name := char.Name

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
		Name:            &name,
		ActionDirection: &snapshot.Direction,
		MovementType:    &both,
		MoveSpeed:       int(snapshot.MoveSpeed.Milliseconds()),
		MovementSeq:     movementSeq,
	}
	return ses, &playerActor, true
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
