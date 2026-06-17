package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/config"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
)

// TrainerEncounterNotifyPayload is sent to the client when a trainer spots the player.
// The client should show "!" and animate the trainer locally to ApproachToX/Y.
// The player position remains server-authoritative and does not get force-walked.
type TrainerEncounterNotifyPayload struct {
	TrainerActorID int    `json:"trainerActorId"` // Runtime actor ID (from ActorRegistry)
	TrainerX       int    `json:"trainerX"`       // Trainer's current position
	TrainerY       int    `json:"trainerY"`
	PlayerX        int    `json:"playerX"`
	PlayerY        int    `json:"playerY"`
	ApproachToX    int    `json:"approachToX"` // Client-only trainer destination adjacent to the player
	ApproachToY    int    `json:"approachToY"`
	WalkToX        int    `json:"walkToX"` // Legacy: current player tile so old clients do not force-walk
	WalkToY        int    `json:"walkToY"`
	TrainerClass   string `json:"trainerClass"`
	TrainerName    string `json:"trainerName"`
}

// TrainerEncounterReadyRequest is sent by the client when the local trainer
// approach animation finishes and the battle can start.
type TrainerEncounterReadyRequest struct {
	TrainerActorID int `json:"trainerActorId"`
}

// trainerSightData holds preloaded trainer info for sight range checks.
type trainerSightData struct {
	ObjectID             int    // DB object ID (before ActorRegistry remapping)
	MapID                int    // phaser_maps.id
	X                    int    // Global X position
	Y                    int    // Global Y position
	Direction            string // Facing direction: UP, DOWN, LEFT, RIGHT
	SightRange           int    // How many tiles the trainer can see
	EventFlag            string // Flag set when trainer is beaten (empty = no flag)
	TrainerClass         string
	PartyIndex           int
	Name                 string
	IsGymLeader          bool
	RuntimeActorID       int // After ActorRegistry remapping
	BattleTextLabel      string
	EndBattleTextLabel   string
	AfterBattleTextLabel string
}

// pendingEncounter tracks a trainer encounter that's waiting for the client-only
// trainer approach animation to finish.
type pendingEncounter struct {
	TrainerData *trainerSightData
	CharID      int64
	PlayerX     int
	PlayerY     int
}

// TrainerEncounterManager handles trainer sight range checks and encounter initiation.
type TrainerEncounterManager struct {
	wh       *WorldHandler
	trainers []trainerSightData          // All trainers with sight range > 0
	byMap    map[int][]*trainerSightData // mapID → trainers on that map

	// Track which trainers each player has already been spotted by (to avoid re-triggering)
	spottedBy   map[int64]map[int]bool // charID → set of trainer ObjectIDs already triggered
	spottedByMu sync.RWMutex

	// Track pending encounters (player is walking to trainer)
	pending   map[int64]*pendingEncounter // charID → pending encounter
	pendingMu sync.RWMutex
}

// NewTrainerEncounterManager creates and initializes the trainer encounter manager.
func NewTrainerEncounterManager(wh *WorldHandler) *TrainerEncounterManager {
	mgr := &TrainerEncounterManager{
		wh:        wh,
		byMap:     make(map[int][]*trainerSightData),
		spottedBy: make(map[int64]map[int]bool),
		pending:   make(map[int64]*pendingEncounter),
	}
	return mgr
}

// Load queries the DB for all trainer NPCs that have a sight range and preloads them.
// Must be called after ActorManager.Start() so ActorRegistry is populated.
func (m *TrainerEncounterManager) Load() {
	myDB := db.GlobalWorldDB.DB

	rows, err := myDB.Query(`
		SELECT
			po.id,
			po.map_id,
			COALESCE(po.x, po.local_x) as global_x,
			COALESCE(po.y, po.local_y) as global_y,
			po.action_direction,
			po.trainer_class,
			po.trainer_party_index,
			po.name,
			COALESCE(tc.is_gym_leader, 0) AS is_gym_leader,
			th.event_flag,
			th.sight_range,
			th.battle_text_label,
			th.end_battle_text_label,
			th.after_battle_text_label
		FROM phaser_objects po
		LEFT JOIN phaser_maps pm
			ON pm.id = po.map_id
		JOIN phaser_text_pointers tp
			ON tp.text_constant = po.text
			AND tp.is_trainer = 1
		JOIN phaser_trainer_headers th
			ON th.header_index = (
				SELECT COUNT(*) - 1
				FROM phaser_text_pointers tp_rank
				WHERE tp_rank.is_trainer = 1
				  AND tp_rank.pointer_index <= tp.pointer_index
				  AND LOWER(REPLACE(tp_rank.map_name, '_', '')) = LOWER(REPLACE(tp.map_name, '_', ''))
			)
			AND (
				th.map_id = po.map_id
				OR LOWER(REPLACE(th.map_name, '_', '')) = LOWER(REPLACE(pm.name, '_', ''))
				OR LOWER(REPLACE(th.map_name, '_', '')) = LOWER(REPLACE(tp.map_name, '_', ''))
			)
		LEFT JOIN phaser_trainer_classes tc
			ON tc.constant_name = po.trainer_class
		WHERE po.trainer_class IS NOT NULL
			AND po.trainer_class != ''
			AND th.sight_range IS NOT NULL
			AND th.sight_range > 0
	`)
	if err != nil {
		log.Printf("[TrainerEncounter] Failed to load trainer objects: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var t trainerSightData
		var globalX, globalY sql.NullInt64
		var isGymLeader sql.NullInt64
		var direction, trainerClass, name sql.NullString
		var eventFlag, battleTextLabel, endBattleTextLabel, afterBattleTextLabel sql.NullString

		if err := rows.Scan(&t.ObjectID, &t.MapID, &globalX, &globalY,
			&direction, &trainerClass, &t.PartyIndex, &name,
			&isGymLeader, &eventFlag, &t.SightRange, &battleTextLabel, &endBattleTextLabel, &afterBattleTextLabel); err != nil {
			log.Printf("[TrainerEncounter] Error scanning trainer: %v", err)
			continue
		}

		if !globalX.Valid || !globalY.Valid {
			continue
		}
		t.X = int(globalX.Int64)
		t.Y = int(globalY.Int64)

		if direction.Valid {
			t.Direction = direction.String
		} else {
			t.Direction = "DOWN"
		}
		if trainerClass.Valid {
			t.TrainerClass = trainerClass.String
		}
		if name.Valid {
			t.Name = name.String
		}
		t.IsGymLeader = isGymLeader.Valid && isGymLeader.Int64 != 0
		if eventFlag.Valid {
			t.EventFlag = eventFlag.String
		}
		if battleTextLabel.Valid {
			t.BattleTextLabel = battleTextLabel.String
		}
		if endBattleTextLabel.Valid {
			t.EndBattleTextLabel = endBattleTextLabel.String
		}
		if afterBattleTextLabel.Valid {
			t.AfterBattleTextLabel = afterBattleTextLabel.String
		}

		// Remap to runtime actor ID
		t.RuntimeActorID = m.wh.ActorRegistry.GetPhaserID(ActorTypeNPC, t.ObjectID)

		m.trainers = append(m.trainers, t)
	}

	// Index by map ID
	for i := range m.trainers {
		t := &m.trainers[i]
		m.byMap[t.MapID] = append(m.byMap[t.MapID], t)
		// Also index under the unified overworld map ID if this map is overworld
		if m.wh.ActorManager.IsOverworld(t.MapID) {
			m.byMap[UnifiedOverworldMapID] = append(m.byMap[UnifiedOverworldMapID], t)
		}
	}

	log.Printf("[TrainerEncounter] Loaded %d trainers with sight range across %d maps", len(m.trainers), len(m.byMap))
}

// CheckPlayerPosition is called on each player movement tick.
// It checks if the player's new position is in any trainer's line of sight.
// Returns true if an encounter was triggered (caller should stop player movement).
func (m *TrainerEncounterManager) CheckPlayerPosition(charID int64, playerX, playerY, mapID int, ses *session.Session) bool {
	trainers := m.byMap[mapID]
	if len(trainers) == 0 {
		return false
	}

	// Don't trigger if player is already in a battle
	if existing := getBattle(charID); existing != nil && !existing.IsOver() {
		return false
	}

	// Don't trigger if player already has a pending encounter
	m.pendingMu.RLock()
	_, hasPending := m.pending[charID]
	m.pendingMu.RUnlock()
	if hasPending {
		return false
	}

	for _, t := range trainers {
		if !m.canAutoTriggerBySight(t) {
			continue
		}
		if trainerBattleSuppressedByGymLeaderDefeat(charID, t, m.wh) {
			continue
		}

		// Skip if already triggered for this player
		m.spottedByMu.RLock()
		alreadySpotted := m.spottedBy[charID] != nil && m.spottedBy[charID][t.ObjectID]
		m.spottedByMu.RUnlock()
		if alreadySpotted {
			continue
		}

		// Skip if player has already defeated this trainer
		if m.IsTrainerDefeated(charID, t.ObjectID) {
			// Check if re-battles are enabled via player options
			opts, _ := db_character.LoadOptions(context.Background(), int32(charID))
			if opts == nil || !opts.AllowTrainerRebattles {
				continue
			}
		}

		if trainerSightDebugEnabled() {
			m.logTrainerSightDebug(charID, t)
		}

		if !m.hasClearSightLine(charID, t, playerX, playerY) {
			continue
		}

		// Player is in this trainer's line of sight!
		log.Printf("[TrainerEncounter] Trainer %s (obj %d) spotted player %d at (%d,%d)",
			t.Name, t.ObjectID, charID, playerX, playerY)

		// Mark as spotted so we don't re-trigger
		m.spottedByMu.Lock()
		if m.spottedBy[charID] == nil {
			m.spottedBy[charID] = make(map[int]bool)
		}
		m.spottedBy[charID][t.ObjectID] = true
		m.spottedByMu.Unlock()

		// Calculate where the client should locally walk the trainer to.
		approachToX, approachToY := m.approachTargetForPlayer(t, playerX, playerY)

		// Store pending encounter
		m.pendingMu.Lock()
		m.pending[charID] = &pendingEncounter{
			TrainerData: t,
			CharID:      charID,
			PlayerX:     playerX,
			PlayerY:     playerY,
		}
		m.pendingMu.Unlock()

		// Send notification to client
		payload := TrainerEncounterNotifyPayload{
			TrainerActorID: t.RuntimeActorID,
			TrainerX:       t.X,
			TrainerY:       t.Y,
			PlayerX:        playerX,
			PlayerY:        playerY,
			ApproachToX:    approachToX,
			ApproachToY:    approachToY,
			WalkToX:        playerX,
			WalkToY:        playerY,
			TrainerClass:   t.TrainerClass,
			TrainerName:    t.Name,
		}
		ses.SendStreamJSON(StructToMap(payload), opcodes.TrainerEncounterNotify)

		// Stop any existing server-side path. The player stays put while
		// the client animates the trainer locally.
		m.wh.PlayerMovement.StopMovement(int(charID))

		return true
	}

	return false
}

func (m *TrainerEncounterManager) canAutoTriggerBySight(t *trainerSightData) bool {
	return t != nil && !t.IsGymLeader
}

// isInSightLine checks if (playerX, playerY) is within the trainer's line of sight.
// The trainer looks in their facing direction for sight_range tiles.
func (m *TrainerEncounterManager) isInSightLine(t *trainerSightData, playerX, playerY int) bool {
	switch t.Direction {
	case "UP":
		if playerX != t.X {
			return false
		}
		return playerY < t.Y && playerY >= t.Y-t.SightRange
	case "DOWN":
		if playerX != t.X {
			return false
		}
		return playerY > t.Y && playerY <= t.Y+t.SightRange
	case "LEFT":
		if playerY != t.Y {
			return false
		}
		return playerX < t.X && playerX >= t.X-t.SightRange
	case "RIGHT":
		if playerY != t.Y {
			return false
		}
		return playerX > t.X && playerX <= t.X+t.SightRange
	default:
		return false
	}
}

func (m *TrainerEncounterManager) hasClearSightLine(charID int64, t *trainerSightData, playerX, playerY int) bool {
	if !m.isInSightLine(t, playerX, playerY) {
		return false
	}
	dx, dy, ok := trainerSightDirectionDelta(t.Direction)
	if !ok {
		return false
	}
	x, y := t.X+dx, t.Y+dy
	for x != playerX || y != playerY {
		if m.isTrainerSightBlockedAt(charID, t, x, y) {
			return false
		}
		x += dx
		y += dy
	}
	return true
}

func (m *TrainerEncounterManager) activeSightTiles(charID int64, t *trainerSightData) []PathNode {
	if t == nil || t.SightRange <= 0 {
		return nil
	}
	dx, dy, ok := trainerSightDirectionDelta(t.Direction)
	if !ok {
		return nil
	}
	tiles := make([]PathNode, 0, t.SightRange)
	for step := 1; step <= t.SightRange; step++ {
		x, y := t.X+dx*step, t.Y+dy*step
		if m.isTrainerSightBlockedAt(charID, t, x, y) {
			break
		}
		tiles = append(tiles, PathNode{X: x, Y: y})
	}
	return tiles
}

func trainerSightDirectionDelta(direction string) (int, int, bool) {
	switch direction {
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

func (m *TrainerEncounterManager) isTrainerSightBlockedAt(charID int64, t *trainerSightData, x, y int) bool {
	if t == nil {
		return true
	}
	if m.isTrainerSightBlockedByTile(t.MapID, x, y) {
		return true
	}
	return m.isTrainerSightBlockedByObject(charID, t, x, y)
}

func (m *TrainerEncounterManager) isTrainerSightBlockedByTile(mapID, x, y int) bool {
	if m == nil || m.wh == nil || m.wh.ActorManager == nil {
		return false
	}
	collisionMap := m.wh.ActorManager.collisionMapForMap(mapID)
	if collisionMap == nil {
		return false
	}
	collisionType, exists := collisionMap[tileKey(x, y)]
	return !exists || collisionType == 0
}

func (m *TrainerEncounterManager) isTrainerSightBlockedByObject(charID int64, t *trainerSightData, x, y int) bool {
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		return false
	}
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id, COALESCE(po.name, ''), COALESCE(po.object_type, '')
		FROM phaser_objects po
		LEFT JOIN character_object_positions cop
			ON cop.character_id = $1 AND cop.object_id = po.id
		LEFT JOIN character_collected_items cci
			ON cci.character_id = $2 AND cci.object_id = po.id
		WHERE po.map_id = $3
			AND po.id != $4
			AND COALESCE(cop.x, po.x, po.local_x) = $5
			AND COALESCE(cop.y, po.y, po.local_y) = $6
			AND cci.object_id IS NULL`,
		charID, charID, t.MapID, t.ObjectID, x, y)
	if err != nil {
		return false
	}
	defer rows.Close()

	rules, _ := eventObjectVisibilityForMap(t.MapID)
	overrides, _ := objectVisibilityOverridesForCharacter(charID)
	for rows.Next() {
		var objectID int
		var name, objectType string
		if err := rows.Scan(&objectID, &name, &objectType); err != nil {
			continue
		}
		if !trainerSightObjectTypeBlocks(objectType) {
			continue
		}
		visible, label := currentEventObjectVisibility(charID, m.eventFlags(), name, rules)
		visible, _ = applyObjectVisibilityOverride(objectID, visible, label, overrides)
		if visible {
			return true
		}
	}
	return false
}

func trainerSightObjectTypeBlocks(objectType string) bool {
	switch objectType {
	case "npc", "item", "pc", "sign":
		return true
	default:
		return false
	}
}

func (m *TrainerEncounterManager) eventFlags() *EventFlagManager {
	if m == nil || m.wh == nil {
		return nil
	}
	return m.wh.EventFlags
}

func trainerSightDebugEnabled() bool {
	if os.Getenv("CAPTUREQUEST_DEBUG_TRAINER_SIGHT") != "true" {
		return false
	}
	cfg, err := config.Get()
	return err == nil && cfg.Local
}

func (m *TrainerEncounterManager) logTrainerSightDebug(charID int64, t *trainerSightData) {
	tiles := m.activeSightTiles(charID, t)
	log.Printf("[TrainerSightDebug] trainer=%s object=%d map=%d pos=(%d,%d) facing=%s range=%d activeTiles=%v",
		t.Name, t.ObjectID, t.MapID, t.X, t.Y, t.Direction, t.SightRange, tiles)
}

// approachTargetForPlayer returns the tile where the trainer should stop during
// the local client approach: adjacent to the player, along the trainer's sight line.
func (m *TrainerEncounterManager) approachTargetForPlayer(t *trainerSightData, playerX, playerY int) (int, int) {
	x, y := playerX, playerY
	switch t.Direction {
	case "UP":
		y = playerY + 1
	case "DOWN":
		y = playerY - 1
	case "LEFT":
		x = playerX + 1
	case "RIGHT":
		x = playerX - 1
	}
	return x, y
}

// HandleTrainerEncounterReady is called when the client reports the local trainer
// approach animation has finished. This initiates the trainer battle.
func HandleTrainerEncounterReady(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req TrainerEncounterReadyRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[TrainerEncounter] Invalid TrainerEncounterReady: %v", err)
		return false
	}

	charID := int64(ses.Client.CharData().ID)

	// Get pending encounter
	wh.TrainerEncounter.pendingMu.Lock()
	enc, ok := wh.TrainerEncounter.pending[charID]
	if ok {
		delete(wh.TrainerEncounter.pending, charID)
	}
	wh.TrainerEncounter.pendingMu.Unlock()

	if !ok || enc == nil {
		log.Printf("[TrainerEncounter] No pending encounter for char %d", charID)
		return false
	}

	t := enc.TrainerData
	if req.TrainerActorID != t.RuntimeActorID {
		log.Printf("[TrainerEncounter] Pending encounter mismatch for char %d: got actor %d, expected %d",
			charID, req.TrainerActorID, t.RuntimeActorID)
		return false
	}

	// Check if already in battle (shouldn't happen, but safety check)
	if existing := getBattle(charID); existing != nil && !existing.IsOver() {
		log.Printf("[TrainerEncounter] Player %d already in battle, skipping trainer encounter", charID)
		return false
	}

	myDB := db.GlobalWorldDB.DB

	// Build the trainer's party
	trainerParty, err := pokebattle.BuildTrainerParty(myDB, t.TrainerClass, t.PartyIndex)
	if err != nil || len(trainerParty) == 0 {
		log.Printf("[TrainerEncounter] Failed to build trainer party for %s/%d: %v", t.TrainerClass, t.PartyIndex, err)
		return false
	}

	// Load player's party from DB. New characters intentionally have no party
	// until Oak's starter script grants one.
	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil || len(playerParty) == 0 {
		log.Printf("[TrainerEncounter] No party for char %d (err: %v), triggering blackout", charID, err)
		ses.SendStreamJSON(buildBlackoutEndResponse(charID), opcodes.PokeBattleEndNotify)
		return false
	}

	// Check if any party Pokémon can battle
	hasAlive := false
	for _, p := range playerParty {
		if p.CurHP > 0 {
			hasAlive = true
			break
		}
	}
	if !hasAlive {
		log.Printf("[TrainerEncounter] All pokemon fainted for char %d, triggering blackout", charID)
		ses.SendStreamJSON(buildBlackoutEndResponse(charID), opcodes.PokeBattleEndNotify)
		return false
	}

	// Calculate prize money: base_money * highest level in trainer's party
	highestLevel := 0
	for _, p := range trainerParty {
		if p.Level > highestLevel {
			highestLevel = p.Level
		}
	}
	var baseMoney int
	_ = myDB.QueryRow(`SELECT base_money FROM phaser_trainer_classes WHERE constant_name = $1`, t.TrainerClass).Scan(&baseMoney)
	prizeMoney := baseMoney * highestLevel

	// Mark all trainer Pokémon as seen in Pokédex (Phase 10.2)
	for _, tp := range trainerParty {
		MarkPokemonSeen(charID, tp.ID)
	}

	// Create trainer battle
	battle := pokebattle.NewTrainerBattle(playerParty, trainerParty)
	configureBattleObedience(battle, charID, wh.EventFlags)
	battle.Trainer = &pokebattle.TrainerMeta{
		ClassName:       t.TrainerClass,
		Name:            t.Name,
		PrizeMoney:      prizeMoney,
		TrainerObjectID: t.ObjectID,
		WinFlag:         t.EventFlag,
	}
	applyPokemonTower7FPostWinMetadata(battle.Trainer, t, enc.PlayerX, enc.PlayerY)
	setBattle(charID, battle)

	log.Printf("[TrainerEncounter] %s started trainer battle vs %s (class=%s, party=%d, %d pokemon)",
		ses.Client.CharData().Name, t.Name, t.TrainerClass, t.PartyIndex, len(trainerParty))

	// Look up display name for the trainer class
	var displayName string
	_ = myDB.QueryRow(`SELECT display_name FROM phaser_trainer_classes WHERE constant_name = $1`, t.TrainerClass).Scan(&displayName)
	if displayName == "" {
		displayName = t.TrainerClass
	}
	displayName = trainerNameForCharacter(charID, t.TrainerClass, displayName)

	// Build intro events
	introEvents := []pokebattle.BattleEvent{
		{Type: pokebattle.EventMessage, Message: displayName + " wants to fight!"},
		{Type: pokebattle.EventMessage, Message: displayName + " sent out " + trainerParty[0].Name + "!"},
	}

	resp := buildBattleStateResponse(battle)
	resp["trainerClass"] = t.TrainerClass
	resp["trainerName"] = displayName
	resp["events"] = introEvents
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)

	return false
}

// ClearSpottedByTrainer removes a single trainer from the spottedBy map for a player.
// Called after a battle ends so the trainer can re-trigger if re-battles are enabled.
func (m *TrainerEncounterManager) ClearSpottedByTrainer(charID int64, trainerObjectID int) {
	m.spottedByMu.Lock()
	if m.spottedBy[charID] != nil {
		delete(m.spottedBy[charID], trainerObjectID)
	}
	m.spottedByMu.Unlock()
}

// ClearPlayer removes all tracking data for a player (call on disconnect/zone change).
func (m *TrainerEncounterManager) ClearPlayer(charID int64) {
	m.spottedByMu.Lock()
	delete(m.spottedBy, charID)
	m.spottedByMu.Unlock()

	m.pendingMu.Lock()
	delete(m.pending, charID)
	m.pendingMu.Unlock()
}

func (m *TrainerEncounterManager) HasPendingEncounter(charID int64) bool {
	m.pendingMu.RLock()
	defer m.pendingMu.RUnlock()
	_, ok := m.pending[charID]
	return ok
}

// GetTrainersOnMap returns the number of trainers with sight range on a given map (for debugging).
func (m *TrainerEncounterManager) GetTrainersOnMap(mapID int) int {
	return len(m.byMap[mapID])
}

// formatTrainerKey creates a unique key for a trainer encounter.
func formatTrainerKey(mapID, objectID int) string {
	return fmt.Sprintf("%d:%d", mapID, objectID)
}

// IsTrainerDefeated checks if a character has already defeated a specific trainer.
func (m *TrainerEncounterManager) IsTrainerDefeated(charID int64, trainerObjectID int) bool {
	var count int
	err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT COUNT(*) FROM character_defeated_trainers WHERE character_id = $1 AND trainer_object_id = $2`,
		charID, trainerObjectID,
	).Scan(&count)
	if err != nil {
		log.Printf("[TrainerEncounter] Error checking defeated trainer: %v", err)
		return false
	}
	return count > 0
}

// MarkTrainerDefeated records that a character has defeated a trainer.
func (m *TrainerEncounterManager) MarkTrainerDefeated(charID int64, trainerObjectID int) {
	_, err := db.GlobalWorldDB.DB.Exec(
		`INSERT INTO character_defeated_trainers (character_id, trainer_object_id)
		VALUES ($1, $2)
		ON CONFLICT (character_id, trainer_object_id) DO NOTHING`,
		charID, trainerObjectID,
	)
	if err != nil {
		log.Printf("[TrainerEncounter] Error marking trainer defeated: %v", err)
	}
}

// GetTrainerByObjectID returns the trainer sight data for a given object ID, or nil.
func (m *TrainerEncounterManager) GetTrainerByObjectID(objectID int) *trainerSightData {
	for i := range m.trainers {
		if m.trainers[i].ObjectID == objectID {
			return &m.trainers[i]
		}
	}
	return nil
}
