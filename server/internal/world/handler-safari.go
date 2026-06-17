package world

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"sync"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

const (
	SafariZoneEntryFee  = 500
	SafariZoneMaxBalls  = 30
	SafariZoneMaxSteps  = 500
	SafariZoneGateMapID = 156 // SAFARI_ZONE_GATE

	SafariZoneCenterMapID   = 220
	SafariZoneDefaultEntryX = 14
	SafariZoneDefaultEntryY = 25
	SafariZoneGateReturnX   = 3
	SafariZoneGateReturnY   = 4

	EventInSafariZone   = "EVENT_IN_SAFARI_ZONE"
	EventSafariGameOver = "EVENT_SAFARI_GAME_OVER"
)

// Safari Zone map IDs (the 4 zones where encounters happen)
var safariZoneMapIDs = map[int]bool{
	217: true, // SAFARI_ZONE_EAST
	218: true, // SAFARI_ZONE_NORTH
	219: true, // SAFARI_ZONE_WEST
	220: true, // SAFARI_ZONE_CENTER
}

// SafariSession tracks a player's current Safari Zone visit.
type SafariSession struct {
	BallsLeft int
	StepsLeft int
	Active    bool
	Battle    *pokebattle.SafariBattleState // Non-nil if in a safari battle
}

type SafariEntryResult struct {
	Success       bool
	Message       string
	Money         int
	BallsLeft     int
	StepsLeft     int
	AlreadyActive bool
}

// SafariZoneManager manages active Safari Zone sessions per player.
type SafariZoneManager struct {
	mu       sync.RWMutex
	sessions map[int64]*SafariSession // charID -> session
}

// NewSafariZoneManager creates a new SafariZoneManager.
func NewSafariZoneManager() *SafariZoneManager {
	return &SafariZoneManager{
		sessions: make(map[int64]*SafariSession),
	}
}

// GetSession returns the safari session for a character, or nil.
func (m *SafariZoneManager) GetSession(charID int64) *SafariSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[charID]
}

// StartSession begins a new Safari Zone visit for a character.
func (m *SafariZoneManager) StartSession(charID int64) *SafariSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := &SafariSession{
		BallsLeft: SafariZoneMaxBalls,
		StepsLeft: SafariZoneMaxSteps,
		Active:    true,
	}
	m.sessions[charID] = s
	return s
}

func (m *SafariZoneManager) SetSession(charID int64, session SafariSession) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[charID] = &SafariSession{
		BallsLeft: session.BallsLeft,
		StepsLeft: session.StepsLeft,
		Active:    session.Active,
		Battle:    session.Battle,
	}
}

// EndSession removes a character's safari session.
func (m *SafariZoneManager) EndSession(charID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, charID)
}

// DecrementStep decreases the step counter. Returns true if steps ran out.
func (m *SafariZoneManager) DecrementStep(charID int64) (stepsLeft, ballsLeft int, expired bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[charID]
	if s == nil || !s.Active {
		return 0, 0, false
	}
	s.StepsLeft--
	if s.StepsLeft <= 0 {
		s.Active = false
		return 0, s.BallsLeft, true
	}
	return s.StepsLeft, s.BallsLeft, false
}

// IsInSafariZone checks if a map ID is one of the safari zone maps.
func IsInSafariZone(mapID int) bool {
	return safariZoneMapIDs[mapID]
}

func endSafariSessionIfLeavingMap(charID int64, sourceMapID, destMapID int, wh *WorldHandler) bool {
	if wh == nil || wh.Safari == nil {
		return false
	}
	if !IsInSafariZone(sourceMapID) || IsInSafariZone(destMapID) {
		return false
	}
	if session := wh.Safari.GetSession(charID); session == nil {
		return false
	}
	if destMapID == SafariZoneGateMapID {
		log.Printf("[Safari] Player %d entered Safari Zone gate from map %d; preserving session for gate exit script", charID, sourceMapID)
		return false
	}

	wh.Safari.EndSession(charID)
	if wh.EventFlags != nil {
		if err := wh.EventFlags.ResetFlag(charID, EventInSafariZone); err != nil {
			log.Printf("[Safari] Failed to reset %s for player %d: %v", EventInSafariZone, charID, err)
		}
	}
	log.Printf("[Safari] Player %d left Safari Zone map %d for map %d; ended session", charID, sourceMapID, destMapID)
	return true
}

func TryStartSafariZoneVisit(charID int64, safari *SafariZoneManager) SafariEntryResult {
	if existing := safari.GetSession(charID); existing != nil && existing.Active {
		return SafariEntryResult{
			Success:       true,
			Message:       "already in safari session",
			Money:         safariMoneyBalance(charID),
			BallsLeft:     existing.BallsLeft,
			StepsLeft:     existing.StepsLeft,
			AlreadyActive: true,
		}
	}

	money, ok := deductSafariEntryFee(charID)
	if !ok {
		return SafariEntryResult{
			Success: false,
			Message: "not enough money",
			Money:   money,
		}
	}

	s := safari.StartSession(charID)
	log.Printf("[Safari] Player %d entered Safari Zone (%d balls, %d steps)", charID, s.BallsLeft, s.StepsLeft)
	return SafariEntryResult{
		Success:   true,
		Money:     money,
		BallsLeft: s.BallsLeft,
		StepsLeft: s.StepsLeft,
	}
}

func deductSafariEntryFee(charID int64) (int, bool) {
	if _, err := db.GlobalWorldDB.DB.Exec(
		`INSERT INTO character_wallet (character_id, pokedollars)
		VALUES ($1, 0)
		ON CONFLICT (character_id) DO NOTHING`,
		charID); err != nil {
		log.Printf("[Safari] Failed to ensure wallet row for char %d: %v", charID, err)
		return safariMoneyBalance(charID), false
	}
	result, err := db.GlobalWorldDB.DB.Exec(
		`UPDATE character_wallet SET pokedollars = pokedollars - $1 WHERE character_id = $2 AND pokedollars >= $3`,
		SafariZoneEntryFee, charID, SafariZoneEntryFee)
	if err != nil {
		log.Printf("[Safari] Failed to deduct entry fee for char %d: %v", charID, err)
		return safariMoneyBalance(charID), false
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return safariMoneyBalance(charID), false
	}
	return safariMoneyBalance(charID), true
}

func safariMoneyBalance(charID int64) int {
	var money sql.NullInt64
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT pokedollars FROM character_wallet WHERE character_id = $1`, charID).Scan(&money); err != nil {
		return 0
	}
	if !money.Valid {
		return 0
	}
	return int(money.Int64)
}

// HandleSafariZoneEnter handles the player entering the Safari Zone.
// If payload contains "statusOnly":true, it only returns existing session data
// (used on reconnect/warp) and never creates a new session.
// Otherwise it creates a new session (used by gate NPC interaction).
func HandleSafariZoneEnter(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		StatusOnly bool `json:"statusOnly"`
	}
	_ = json.Unmarshal(payload, &req)

	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	// Check if already in safari
	if existing := wh.Safari.GetSession(charID); existing != nil && existing.Active {
		ses.SendStreamJSON(map[string]interface{}{
			"success":   true,
			"ballsLeft": existing.BallsLeft,
			"stepsLeft": existing.StepsLeft,
		}, opcodes.SafariZoneEnterResponse)
		// Also send step update so the HUD shows on reconnect
		ses.SendStreamJSON(map[string]interface{}{
			"stepsLeft": existing.StepsLeft,
			"ballsLeft": existing.BallsLeft,
		}, opcodes.SafariZoneStepUpdate)
		// If there's an active battle, re-send it so the encounter UI resumes
		if existing.Battle != nil && !existing.Battle.IsOver() {
			wild := existing.Battle.WildPokemon
			ses.SendStreamJSON(map[string]interface{}{
				"pokemon": map[string]interface{}{
					"id":    wild.ID,
					"name":  wild.Name,
					"level": wild.Level,
					"hp":    wild.CurHP,
					"maxHp": wild.MaxHP,
				},
				"ballsLeft": existing.BallsLeft,
				"stepsLeft": existing.StepsLeft,
			}, opcodes.SafariBattleStartNotify)
		}
		return false
	}

	// Status-only check: no existing session, so just report no active safari
	if req.StatusOnly {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"message": "no active safari session",
		}, opcodes.SafariZoneEnterResponse)
		return false
	}

	result := TryStartSafariZoneVisit(charID, wh.Safari)
	if !result.Success {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"message": result.Message,
			"money":   result.Money,
		}, opcodes.SafariZoneEnterResponse)
		return false
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"ballsLeft": result.BallsLeft,
		"stepsLeft": result.StepsLeft,
		"money":     result.Money,
	}, opcodes.SafariZoneEnterResponse)

	// Also send a step update so the HUD shows immediately
	ses.SendStreamJSON(map[string]interface{}{
		"stepsLeft": result.StepsLeft,
		"ballsLeft": result.BallsLeft,
	}, opcodes.SafariZoneStepUpdate)

	return false
}

// HandleSafariBattleAction processes a safari battle action (ball, bait, rock, run).
func HandleSafariBattleAction(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		Action string `json:"action"` // "ball", "bait", "rock", "run"
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Safari] Invalid action request: %v", err)
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	safariSes := wh.Safari.GetSession(charID)
	if safariSes == nil || !safariSes.Active || safariSes.Battle == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "not in safari battle",
		}, opcodes.SafariBattleActionResponse)
		return false
	}

	battle := safariSes.Battle

	switch req.Action {
	case "ball":
		safariSes.BallsLeft = battle.BallsLeft // Sync before action
		battle.ThrowBall()
		safariSes.BallsLeft = battle.BallsLeft
	case "bait":
		battle.ThrowBait()
	case "rock":
		battle.ThrowRock()
	case "run":
		battle.Run()
	default:
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "invalid action",
		}, opcodes.SafariBattleActionResponse)
		return false
	}

	// Build response
	resp := map[string]interface{}{
		"success":   true,
		"events":    battle.Events,
		"ballsLeft": safariSes.BallsLeft,
		"stepsLeft": safariSes.StepsLeft,
		"isOver":    battle.IsOver(),
		"caught":    battle.Caught,
		"fled":      battle.Fled,
	}

	if battle.Caught {
		// Add the caught Pokémon to the player's party or PC
		myDB := db.GlobalWorldDB.DB
		caughtPoke := battle.WildPokemon
		caughtPoke.IsWild = false
		// Mark as caught in Pokédex (Phase 10.2)
		MarkPokemonCaught(charID, caughtPoke.ID)

		sentToPC := false
		pcBox := -1

		// Load current party to check if there's room
		party, err := pokebattle.LoadParty(myDB, charID)
		if err == nil && len(party) < 6 {
			// Add to party
			party = append(party, caughtPoke)
			if saveErr := pokebattle.SaveParty(myDB, charID, party); saveErr != nil {
				log.Printf("[Safari] Failed to save party for char %d: %v", charID, saveErr)
			}
		} else {
			// Party full — save to Bill's PC
			box, slot, pcErr := pokebattle.SavePokemonToPC(myDB, charID, caughtPoke)
			if pcErr != nil {
				log.Printf("[Safari] Failed to save %s to PC for char %d: %v", caughtPoke.Name, charID, pcErr)
			} else {
				sentToPC = true
				pcBox = box
				log.Printf("[Safari] Player %d party full, sent L%d %s to PC box %d slot %d",
					charID, caughtPoke.Level, caughtPoke.Name, box, slot)
			}
		}

		resp["caughtPokemon"] = map[string]interface{}{
			"id":    caughtPoke.ID,
			"name":  caughtPoke.Name,
			"level": caughtPoke.Level,
		}
		resp["sentToPC"] = sentToPC
		if sentToPC {
			resp["pcBox"] = pcBox + 1 // 1-indexed for display
		}
		log.Printf("[Safari] Player %d caught L%d %s", charID, caughtPoke.Level, caughtPoke.Name)
	}

	if battle.IsOver() {
		safariSes.Battle = nil // Clear battle reference

		// Check if out of balls — end safari visit and warp back to gate (Gen 1 behavior)
		if safariSes.BallsLeft <= 0 && !battle.Caught {
			safariSes.Active = false
			resp["safariOver"] = true
		}
	}

	ses.SendStreamJSON(resp, opcodes.SafariBattleActionResponse)

	// If safari visit is over (out of balls), send exit notification to warp player back
	if !safariSes.Active {
		ses.SendStreamJSON(map[string]interface{}{
			"stepsLeft": 0,
			"ballsLeft": 0,
			"message":   "PA: Ding-dong! Your SAFARI GAME is over!",
			"mapId":     SafariZoneGateMapID,
			"x":         SafariZoneGateReturnX,
			"y":         SafariZoneGateReturnY,
			"direction": "DOWN",
		}, opcodes.SafariZoneExitNotify)
	}

	return false
}

// CheckSafariStep is called from the movement tick when a player steps in a safari zone.
// It decrements the step counter, checks for encounters, and handles expiry.
func CheckSafariStep(charID int64, x, y, mapID int, ses *session.Session, wh *WorldHandler) bool {
	if !IsInSafariZone(mapID) {
		return false
	}

	safariSes := wh.Safari.GetSession(charID)
	if safariSes == nil || !safariSes.Active {
		return false
	}

	// Don't process steps while in a battle
	if safariSes.Battle != nil {
		return false
	}

	// Decrement step counter
	stepsLeft, ballsLeft, expired := wh.Safari.DecrementStep(charID)

	if expired {
		log.Printf("[Safari] Player %d ran out of steps", charID)
		ses.SendStreamJSON(map[string]interface{}{
			"stepsLeft": 0,
			"ballsLeft": ballsLeft,
			"message":   "PA: Ding-dong! Your SAFARI GAME is over!",
			"mapId":     SafariZoneGateMapID,
			"x":         SafariZoneGateReturnX,
			"y":         SafariZoneGateReturnY,
			"direction": "DOWN",
		}, opcodes.SafariZoneExitNotify)
		return true // Stop movement
	}

	// Send step update every step so the client HUD stays current
	ses.SendStreamJSON(map[string]interface{}{
		"stepsLeft": stepsLeft,
		"ballsLeft": ballsLeft,
	}, opcodes.SafariZoneStepUpdate)

	// Check for wild encounter (uses normal encounter system but starts safari battle)
	areaID := wh.WildEncounter.getEncounterAreaID(mapID, x, y)
	if areaID == 0 {
		return false
	}

	area, ok := wh.WildEncounter.areas[areaID]
	if !ok || area.EncounterRate == 0 || len(area.Slots) == 0 {
		return false
	}

	// Gen 1 encounter rate check
	roll := rand.Intn(256)
	if roll >= area.EncounterRate {
		return false
	}

	// Encounter triggered — start safari battle
	pokemonID, level := wh.WildEncounter.selectEncounterPokemon(area)
	myDB := db.GlobalWorldDB.DB
	wildPokemon, err := pokebattle.BuildWildPokemon(myDB, pokemonID, level)
	if err != nil {
		log.Printf("[Safari] Failed to build wild pokemon %d: %v", pokemonID, err)
		return false
	}

	battle := pokebattle.NewSafariBattle(wildPokemon, safariSes.BallsLeft, safariSes.StepsLeft)
	safariSes.Battle = battle

	// Mark safari Pokémon as seen in Pokédex (Phase 10.2)
	MarkPokemonSeen(charID, pokemonID)

	log.Printf("[Safari] Player %d encountered L%d %s (%d balls left)",
		charID, wildPokemon.Level, wildPokemon.Name, safariSes.BallsLeft)

	ses.SendStreamJSON(map[string]interface{}{
		"pokemon": map[string]interface{}{
			"id":        wildPokemon.ID,
			"name":      wildPokemon.Name,
			"level":     wildPokemon.Level,
			"hp":        wildPokemon.CurHP,
			"maxHp":     wildPokemon.MaxHP,
			"spriteId":  wildPokemon.ID,
			"catchRate": wildPokemon.CatchRate,
		},
		"ballsLeft": safariSes.BallsLeft,
		"stepsLeft": safariSes.StepsLeft,
	}, opcodes.SafariBattleStartNotify)

	return true // Stop movement
}
