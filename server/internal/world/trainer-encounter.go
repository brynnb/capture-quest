package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
)

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
	MapID       int
	Token       string
	Resolution  string
}

// TrainerEncounterManager handles trainer sight range checks and encounter initiation.
type TrainerEncounterManager struct {
	wh       *WorldHandler
	trainers []trainerSightData          // All trainers with sight range > 0
	byMap    map[int][]*trainerSightData // mapID → trainers on that map

	// Track which trainers each player has already been spotted by (to avoid re-triggering)
	spottedBy   map[int64]map[int]bool // charID → set of trainer ObjectIDs already triggered
	spottedByMu sync.RWMutex
}

// NewTrainerEncounterManager creates and initializes the trainer encounter manager.
func NewTrainerEncounterManager(wh *WorldHandler) *TrainerEncounterManager {
	mgr := &TrainerEncounterManager{
		wh:        wh,
		byMap:     make(map[int][]*trainerSightData),
		spottedBy: make(map[int64]map[int]bool),
	}
	return mgr
}

// Load queries the DB for all trainer NPCs that have a sight range and preloads them.
// Must be called after ActorManager.Load() so ActorRegistry is populated.
// This is startup-only; publish the complete immutable index before timers start.
func (m *TrainerEncounterManager) Load(ctx context.Context) error {
	if err := m.requireEncounterSchema(ctx); err != nil {
		return err
	}
	if m.wh == nil || m.wh.database == nil {
		return fmt.Errorf("trainer preload requires a database")
	}
	myDB := m.wh.database
	trainers := make([]trainerSightData, 0)
	byMap := make(map[int][]*trainerSightData)

	rows, err := myDB.QueryContext(ctx, `
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
		return fmt.Errorf("[TrainerEncounter] Failed to load trainer objects: %w", err)
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
			return fmt.Errorf("[TrainerEncounter] Error scanning trainer: %w", err)
		}

		if !globalX.Valid || !globalY.Valid {
			return fmt.Errorf("trainer object %d has no coordinates", t.ObjectID)
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

		trainers = append(trainers, t)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("read trainer rows: %w", err)
	}
	// Index by map ID
	for i := range trainers {
		t := &trainers[i]
		byMap[t.MapID] = append(byMap[t.MapID], t)
		// Also index under the unified overworld map ID if this map is overworld
		if m.wh.ActorManager.IsOverworld(t.MapID) {
			byMap[UnifiedOverworldMapID] = append(byMap[UnifiedOverworldMapID], t)
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	m.trainers, m.byMap = trainers, byMap
	log.Printf("[TrainerEncounter] Loaded %d trainers with sight range across %d maps", len(trainers), len(byMap))
	return nil
}

// CheckPlayerPosition is called on each player movement tick.
// It checks if the player's new position is in any trainer's line of sight.
// Returns true if an encounter was triggered (caller should stop player movement).
// Selection has no publication or cache mutation. The movement transaction
// stages it, and only its successful commit may reserve/publish the encounter.
func (m *TrainerEncounterManager) publishPositionEncounter(enc *pendingEncounter, ses *session.Session) {
	t, charID, playerX, playerY := enc.TrainerData, enc.CharID, enc.PlayerX, enc.PlayerY
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

	// Send notification to client
	payload := protocol.TrainerEncounterNotifyPayload{
		EncounterToken: enc.Token,
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
	ses.SendStreamJSON(payload, opcodes.TrainerEncounterNotify)

	// Stop any queued movement helper state. The player stays put while
	// the client animates the trainer locally.
	m.wh.PlayerMovement.StopMovement(int(charID))

}

func (m *TrainerEncounterManager) planPositionEncounter(ctx context.Context, q db.DBTX, charID int64, playerX, playerY, mapID int, flags *EventFlagManager) (*trainerSightData, error) {
	trainers := m.byMap[mapID]
	if len(trainers) == 0 {
		return nil, nil
	}
	if existing := getBattle(charID); existing != nil && !existing.IsOver() {
		return nil, nil
	}
	for _, t := range trainers {
		if !m.canAutoTriggerBySight(t) {
			continue
		}
		m.spottedByMu.RLock()
		spotted := m.spottedBy[charID] != nil && m.spottedBy[charID][t.ObjectID]
		m.spottedByMu.RUnlock()
		if spotted {
			continue
		}
		eligible, err := trainerEligibleIn(q, charID, t, flags)
		if err != nil {
			return nil, err
		}
		if !eligible {
			continue
		}
		if !m.isInSightLine(t, playerX, playerY) {
			continue
		}
		dx, dy, _ := trainerSightDirectionDelta(t.Direction)
		clear := true
		for x, y := t.X+dx, t.Y+dy; x != playerX || y != playerY; x, y = x+dx, y+dy {
			if m.isTrainerSightBlockedByTile(t.MapID, x, y) {
				clear = false
				break
			}
			blocked, err := m.isTrainerSightBlockedByObjectIn(ctx, q, charID, t, x, y, flags)
			if err != nil {
				return nil, err
			}
			if blocked {
				clear = false
				break
			}
		}
		if clear {
			return t, nil
		}
	}
	return nil, nil
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
	blocked, err := m.isTrainerSightBlockedByObjectIn(context.Background(), db.GlobalWorldDB.DB, charID, t, x, y, m.eventFlags())
	return err == nil && blocked
}

func (m *TrainerEncounterManager) isTrainerSightBlockedByObjectIn(ctx context.Context, q db.DBTX, charID int64, t *trainerSightData, x, y int, flags *EventFlagManager) (bool, error) {
	rules, err := eventObjectVisibilityForMapContext(ctx, q.(db.ContextDBTX), t.MapID)
	if err != nil {
		return false, err
	}
	overrides, err := objectVisibilityOverridesForCharacterContext(ctx, q.(db.ContextDBTX), charID)
	if err != nil {
		return false, err
	}
	rows, err := q.Query(`
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
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var objectID int
		var name, objectType string
		if err := rows.Scan(&objectID, &name, &objectType); err != nil {
			return false, err
		}
		if !trainerSightObjectTypeBlocks(objectType) {
			continue
		}
		visible, label := currentEventObjectVisibility(charID, flags, name, rules)
		visible, _ = applyObjectVisibilityOverride(objectID, visible, label, overrides)
		if visible {
			return true, nil
		}
	}
	return false, rows.Err()
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
	var req protocol.TrainerEncounterReadyRequest
	if decodePlayerMovement(payload, &req) != nil || !validMovementToken(req.EncounterToken) || wh.TrainerEncounter == nil {
		return false
	}
	result, err := wh.TrainerEncounter.resolveEncounter(ses.CommandContext(), int64(ses.Client.CharData().ID), req)
	if err != nil {
		log.Printf("[TrainerEncounter] Ready rejected: %v", err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Could not start battle. Please reconnect."}, opcodes.PokeBattleStartResponse)
		return false
	}
	if result.Battle != nil {
		setBattle(int64(ses.Client.CharData().ID), result.Battle)
		resp := buildBattleStateResponse(result.Battle)
		resp["trainerClass"], resp["trainerName"] = result.Battle.Trainer.ClassName, result.DisplayName
		if !result.Replayed {
			resp["events"] = []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: result.DisplayName + " wants to fight!"}, {Type: pokebattle.EventMessage, Message: result.DisplayName + " sent out " + result.Battle.GetEnemyPokemon().Name + "!"}}
		}
		ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
	} else if result.Blackout != nil {
		publishStandaloneBlackout(ses, wh, int64(ses.Client.CharData().ID), *result.Blackout, result.Party)
	}
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

// ClearPlayer retires presentation tracking; durable pending encounters survive disconnect.
func (m *TrainerEncounterManager) ClearPlayer(charID int64) {
	m.spottedByMu.Lock()
	delete(m.spottedBy, charID)
	m.spottedByMu.Unlock()

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
