package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

// --- Request/Response types ---

type PokeBattleStartRequest struct {
	MapID int `json:"mapId"` // For wild encounters: which map to pull encounter data from
}

// PokemonDTO is the client-facing representation of a Pokémon in battle and party.
type PokemonDTO struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Level          int       `json:"level"`
	Type1          string    `json:"type1"`
	Type2          string    `json:"type2"`
	CurHP          int       `json:"curHp"`
	MaxHP          int       `json:"maxHp"`
	Attack         int       `json:"attack"`
	Defense        int       `json:"defense"`
	Speed          int       `json:"speed"`
	Special        int       `json:"special"`
	Exp            int       `json:"exp"`
	ExpToNextLevel int       `json:"expToNextLevel"`
	Status         string    `json:"status"`
	IsWild         bool      `json:"isWild"`
	BoxSlot        int       `json:"boxSlot"`
	CrySFX         string    `json:"crySfx,omitempty"`
	CryPitch       int       `json:"cryPitch,omitempty"`
	CryLength      int       `json:"cryLength,omitempty"`
	Moves          []MoveDTO `json:"moves"`
}

type MoveDTO struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Power        int    `json:"power"`
	Accuracy     int    `json:"accuracy"`
	PP           int    `json:"pp"`
	MaxPP        int    `json:"maxPp"`
	MoveSFX      string `json:"moveSfx,omitempty"`
	MoveSFXPitch int    `json:"moveSfxPitch,omitempty"`
	MoveSFXTempo int    `json:"moveSfxTempo,omitempty"`
}

type PokeBattleActionRequest struct {
	Action     string `json:"action"`     // "fight", "run", "switch", "item"
	MoveSlot   int    `json:"moveSlot"`   // 0-3 for fight, party index for switch
	ItemID     int32  `json:"itemId"`     // Item template ID (for "item" action)
	InstanceID int32  `json:"instanceId"` // Concrete inventory instance (optional)
	TargetSlot int    `json:"targetSlot"` // Party slot index for medicine items (-1 = active Pokémon)
}

type PokeBattleSwitchRequest struct {
	PartyIndex int    `json:"partyIndex"`
	Action     string `json:"action"` // "switch" (default) or "run" (wild only)
}

type CQBattleItemUseRequest struct {
	ItemID     int32 `json:"itemId"`
	InstanceID int32 `json:"instanceId"`
	TargetSlot int   `json:"targetSlot"`
	MoveSlot   int   `json:"moveSlot"`
}

// --- Helpers ---

func pokemonToDTO(p *pokebattle.Pokemon) PokemonDTO {
	// Calculate exp to next level
	expToNext := 0
	if p.Level < 100 {
		nextLevelExp := pokebattle.ExpForLevel(p.GrowthRt, p.Level+1)
		expToNext = nextLevelExp - p.Exp
		if expToNext < 0 {
			expToNext = 0
		}
	}

	dto := PokemonDTO{
		ID:             p.ID,
		Name:           p.Name,
		Level:          p.Level,
		Type1:          p.Type1.String(),
		Type2:          p.Type2.String(),
		CurHP:          p.CurHP,
		MaxHP:          p.MaxHP,
		Attack:         p.Attack,
		Defense:        p.Defense,
		Speed:          p.Speed,
		Special:        p.Special,
		Exp:            p.Exp,
		ExpToNextLevel: expToNext,
		Status:         p.Status.String(),
		IsWild:         p.IsWild,
		BoxSlot:        p.BoxSlot,
		CrySFX:         p.CrySFX,
		CryPitch:       p.CryPitch,
		CryLength:      p.CryLength,
	}
	for _, m := range p.Moves {
		if m.ID > 0 {
			dto.Moves = append(dto.Moves, MoveDTO{
				ID:           m.ID,
				Name:         m.Name,
				Type:         m.Type.String(),
				Power:        m.Power,
				Accuracy:     m.Accuracy,
				PP:           m.PP,
				MaxPP:        m.MaxPP,
				MoveSFX:      m.BattleSFX,
				MoveSFXPitch: m.SFXPitch,
				MoveSFXTempo: m.SFXTempo,
			})
		}
	}
	if dto.Moves == nil {
		dto.Moves = []MoveDTO{}
	}
	return dto
}

func buildBattleStateResponse(b *pokebattle.BattleState) map[string]interface{} {
	player := b.GetPlayerPokemon()
	enemy := b.GetEnemyPokemon()

	resp := map[string]interface{}{
		"success":       true,
		"phase":         phaseToString(b.Phase),
		"turnNumber":    b.TurnNumber,
		"playerPokemon": pokemonToDTO(player),
		"enemyPokemon":  pokemonToDTO(enemy),
	}
	attachBattlePartyMetadata(resp, b)
	return resp
}

func battlePartyDTOs(b *pokebattle.BattleState) []PokemonDTO {
	partyDTOs := make([]PokemonDTO, len(b.PlayerParty))
	for i, p := range b.PlayerParty {
		if p != nil {
			partyDTOs[i] = pokemonToDTO(p)
		}
	}
	return partyDTOs
}

func attachBattlePartyMetadata(resp map[string]interface{}, b *pokebattle.BattleState) {
	resp["playerParty"] = battlePartyDTOs(b)
	resp["playerActive"] = b.PlayerActive
	resp["battleType"] = battleTypeToString(b.BattleType)
	resp["allowedActions"] = b.AllowedActions
	resp["guaranteedCatch"] = b.GuaranteedCatch
}

func cutsceneActionsContainType(rawActions json.RawMessage, actionType string) bool {
	if len(rawActions) == 0 || string(rawActions) == "null" {
		return false
	}
	actions, err := DecodeCutsceneActions(rawActions)
	if err != nil {
		return false
	}
	return cutsceneActionListContainsType(actions, actionType)
}

func cutsceneActionListContainsType(actions []CutsceneAction, actionType string) bool {
	for _, action := range actions {
		if action.Type == actionType {
			return true
		}
		if len(action.Actions) > 0 && cutsceneActionListContainsType(action.Actions, actionType) {
			return true
		}
	}
	return false
}

func battleHasScriptedPartyHeal(battle *pokebattle.BattleState, wonOrCaught bool) bool {
	if battle == nil {
		return false
	}
	if battle.Trainer != nil {
		if wonOrCaught {
			return cutsceneActionsContainType(battle.Trainer.PostWinActions, "healParty")
		}
		return cutsceneActionsContainType(battle.Trainer.PostLoseActions, "healParty")
	}
	if wonOrCaught && battle.BattleType == pokebattle.BattleWild {
		return cutsceneActionsContainType(battle.WildPostWinActions, "healParty")
	}
	return false
}

func phaseToString(p pokebattle.BattlePhase) string {
	switch p {
	case pokebattle.PhaseActionSelect:
		return "action_select"
	case pokebattle.PhaseMoveSelect:
		return "move_select"
	case pokebattle.PhaseExecuteTurn:
		return "execute_turn"
	case pokebattle.PhaseFaintSwitch:
		return "faint_switch"
	case pokebattle.PhaseBattleEnd:
		return "battle_end"
	default:
		return "unknown"
	}
}

func battleTypeToString(bt pokebattle.BattleType) string {
	switch bt {
	case pokebattle.BattleWild:
		return "wild"
	case pokebattle.BattleTrainer:
		return "trainer"
	default:
		return "unknown"
	}
}

// --- Handlers ---

// HandlePokeBattleStart initiates a wild Pokémon battle.
// For now, only wild encounters are supported. Trainer battles will come later.
func HandlePokeBattleStart(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PokeBattleStartRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[PokeBattle] Invalid start request: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "invalid request",
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)

	// Check if already in battle
	if existing := getBattle(charID); existing != nil && !existing.IsOver() {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "already in battle",
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	myDB := db.GlobalWorldDB.DB

	// Select a wild encounter for this map
	pokemonID, level, err := pokebattle.SelectWildEncounter(myDB, req.MapID, "grass")
	if err != nil {
		log.Printf("[PokeBattle] No wild encounters for map %d: %v", req.MapID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "no wild pokemon here",
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	// Build the wild Pokémon
	wildPokemon, err := pokebattle.BuildWildPokemon(myDB, pokemonID, level)
	if err != nil {
		log.Printf("[PokeBattle] Failed to build wild pokemon %d: %v", pokemonID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "failed to create wild pokemon",
		}, opcodes.PokeBattleStartResponse)
		return false
	}

	// Load player's party from DB. New characters intentionally have no party
	// until Oak's starter script grants one.
	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil || len(playerParty) == 0 {
		log.Printf("[PokeBattle] No party for char %d (err: %v), triggering blackout", charID, err)
		ses.SendStreamJSON(buildBlackoutEndResponse(charID), opcodes.PokeBattleEndNotify)
		return false
	}

	// Check if any party Pokémon can battle (has HP > 0)
	hasAlive := false
	for _, p := range playerParty {
		if p.CurHP > 0 {
			hasAlive = true
			break
		}
	}
	if !hasAlive {
		log.Printf("[PokeBattle] All pokemon fainted for char %d, triggering blackout", charID)
		ses.SendStreamJSON(buildBlackoutEndResponse(charID), opcodes.PokeBattleEndNotify)
		return false
	}

	// Create battle
	battle := pokebattle.NewWildBattle(playerParty, wildPokemon)
	configureBattleObedience(battle, charID, wh.EventFlags)
	battle, err = startBattle(wh.database, charID, battle)
	if err != nil {
		log.Printf("[PokeBattle] Start failed for character %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Could not start battle. Please reconnect."}, opcodes.PokeBattleStartResponse)
		return false
	}

	log.Printf("[PokeBattle] %s started wild battle: L%d %s vs L%d %s",
		ses.Client.CharData().Name, playerParty[0].Level, playerParty[0].Name,
		wildPokemon.Level, wildPokemon.Name)

	resp := buildBattleStateResponse(battle)
	ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
	return false
}

// HandlePokeBattleAction processes a player's turn action (fight, run, switch).
func HandlePokeBattleAction(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PokeBattleActionRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	current := getBattle(charID)
	if current == nil || current.IsOver() {
		sendBattleItemError(ses, opcodes.PokeBattleActionResponse, "Not in battle")
		return false
	}
	var result battleTurnResult
	committed, err := pokebattle.CommitBattle(context.Background(), wh.database, charID, current, func(tx db.DBTX, next *pokebattle.BattleState) (err error) {
		result, err = applyBattleTurn(tx, charID, next, req)
		return err
	})
	if err != nil {
		sendBattleCommitError(ses, charID, current, err, opcodes.PokeBattleActionResponse)
		return false
	}
	setBattle(charID, committed)
	publishBattleTurn(ses, wh, charID, committed, result, opcodes.PokeBattleActionResponse)
	if req.Action == "item" || committed.IsOver() {
		sendCQInventorySnapshot(ses, int32(charID))
	}
	return false
}

// HandleCQBattleItemUse supports the item-specific battle opcode by routing it
// through the same battle action flow the current battle UI uses.
func HandleCQBattleItemUse(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req CQBattleItemUseRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[PokeBattle] Invalid CQ battle item request: %v", err)
		return false
	}
	actionPayload, err := json.Marshal(PokeBattleActionRequest{
		Action:     "item",
		MoveSlot:   req.MoveSlot,
		ItemID:     req.ItemID,
		InstanceID: req.InstanceID,
		TargetSlot: req.TargetSlot,
	})
	if err != nil {
		return false
	}
	return HandlePokeBattleAction(ses, actionPayload, wh)
}

// HandlePokeBattleSwitch handles forced switch-in after a faint, or running from a wild battle.
func HandlePokeBattleSwitch(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PokeBattleSwitchRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	current := getBattle(charID)
	if current == nil {
		sendBattleItemError(ses, opcodes.PokeBattleSwitchResponse, "Not in battle")
		return false
	}
	var result battleTurnResult
	committed, err := pokebattle.CommitBattle(context.Background(), wh.database, charID, current, func(tx db.DBTX, next *pokebattle.BattleState) (err error) {
		if next.Phase != pokebattle.PhaseFaintSwitch {
			return battleRuleError("Not in faint switch phase")
		}
		if !next.IsActionAllowed(pokebattle.ActionSwitch) {
			return battleRuleError("Use an item.")
		}
		if req.Action == "run" {
			if next.BattleType == pokebattle.BattleTrainer {
				return battleRuleError("Can't run from a trainer battle!")
			}
			result.Events = next.RunFromFaintSwitch()
		} else {
			if err := validateBattleSwitch(next, req.PartyIndex); err != nil {
				return battleRuleError(err.Error())
			}
			result.Events = next.ForceSwitchIn(req.PartyIndex)
		}
		result, err = settleBattleTurn(tx, charID, next, result)
		return err
	})
	if err != nil {
		sendBattleCommitError(ses, charID, current, err, opcodes.PokeBattleSwitchResponse)
		return false
	}
	setBattle(charID, committed)
	publishBattleTurn(ses, wh, charID, committed, result, opcodes.PokeBattleSwitchResponse)
	return false
}

func validateBattleSwitch(battle *pokebattle.BattleState, partyIndex int) error {
	if partyIndex < 0 || partyIndex >= len(battle.PlayerParty) || battle.PlayerParty[partyIndex] == nil {
		return fmt.Errorf("Invalid Pokémon")
	}
	if partyIndex == battle.PlayerActive {
		return fmt.Errorf("That Pokémon is already out")
	}
	if battle.PlayerParty[partyIndex].IsFainted() {
		return fmt.Errorf("That Pokémon has fainted")
	}
	return nil
}

// getTrainerDefeatText returns a defeat quote for a trainer class.
// In the real games each trainer has unique text; for now we use class-based defaults.
func getTrainerDefeatText(className string) string {
	switch className {
	case "BUG_CATCHER":
		return "No! My bugs!"
	case "YOUNGSTER":
		return "Wow, you're strong!"
	case "LASS":
		return "Oh no, I lost!"
	case "HIKER":
		return "You're tougher than rocks!"
	case "SUPER_NERD":
		return "My calculations were off..."
	case "POKEMANIAC":
		return "I can't believe it!"
	case "SAILOR":
		return "You sunk my battle plan!"
	case "BIKER":
		return "Tch... not bad."
	case "JR_TRAINER_M", "JR_TRAINER_F":
		return "I still have a lot to learn..."
	case "BEAUTY":
		return "Oh, how ugly of me to lose!"
	case "GENTLEMAN":
		return "A fine battle, indeed."
	case "SCIENTIST":
		return "My research was incomplete!"
	case "ROCKER":
		return "My Pokémon rocked out too hard!"
	case "JUGGLER":
		return "I dropped the ball..."
	case "TAMER":
		return "You've tamed me!"
	case "BIRD_KEEPER":
		return "My birds have been grounded!"
	case "BLACKBELT":
		return "Your technique is flawless!"
	case "PSYCHIC_TR":
		return "I didn't foresee this..."
	case "CHANNELER":
		return "The spirits have abandoned me..."
	case "ROCKET_GRUNT":
		return "This isn't over!"
	default:
		return "You're pretty good!"
	}
}

// sendPartyUpdate loads the player's party from DB and pushes it to the client.
func sendPartyUpdate(ses *session.Session) {
	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB
	party, err := pokebattle.LoadParty(myDB, charID)
	if err != nil {
		log.Printf("[Party] Failed to load party for update (char %d): %v", charID, err)
		return
	}
	sendPokemonPartySnapshot(ses, party)
}

// Publish the committed domain snapshot without another mutable database read.
func sendPokemonPartySnapshot(ses *session.Session, party []*pokebattle.Pokemon) {
	partyDTOs := make([]PokemonDTO, 0, len(party))
	for _, p := range party {
		partyDTOs = append(partyDTOs, pokemonToDTO(p))
	}
	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"party":   partyDTOs,
	}, opcodes.PokemonPartyResponse)
}

// HandlePokemonPartyReorder reorders the player's party based on the client's new order.
func HandlePokemonPartyReorder(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		Order []int `json:"order"` // New order as array of current indices, e.g. [2,0,1] means old slot 2 → new slot 0
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Party] Invalid reorder request: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "invalid request",
		}, opcodes.PokemonPartyReorderResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB

	party, err := pokebattle.LoadParty(myDB, charID)
	if err != nil {
		log.Printf("[Party] Failed to load party for reorder (char %d): %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "failed to load party",
		}, opcodes.PokemonPartyReorderResponse)
		return false
	}

	// Validate the order array
	if len(req.Order) != len(party) {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "order length mismatch",
		}, opcodes.PokemonPartyReorderResponse)
		return false
	}

	// Check for valid indices and no duplicates
	seen := make(map[int]bool)
	for _, idx := range req.Order {
		if idx < 0 || idx >= len(party) || seen[idx] {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   "invalid order indices",
			}, opcodes.PokemonPartyReorderResponse)
			return false
		}
		seen[idx] = true
	}

	// Build reordered party
	newParty := make([]*pokebattle.Pokemon, len(party))
	for newSlot, oldIdx := range req.Order {
		newParty[newSlot] = party[oldIdx]
	}

	if err := pokebattle.SaveParty(myDB, charID, newParty); err != nil {
		log.Printf("[Party] Failed to save reordered party for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "failed to save party",
		}, opcodes.PokemonPartyReorderResponse)
		return false
	}

	// Send back updated party
	partyDTOs := make([]PokemonDTO, 0, len(newParty))
	for _, p := range newParty {
		partyDTOs = append(partyDTOs, pokemonToDTO(p))
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"party":   partyDTOs,
	}, opcodes.PokemonPartyReorderResponse)

	log.Printf("[Party] Reordered party for char %d: %v", charID, req.Order)
	return false
}

// HandlePokemonPartyRequest sends the player's current Pokémon party to the client.
func HandlePokemonPartyRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	charID := int64(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB

	party, err := pokebattle.LoadParty(myDB, charID)
	if err != nil {
		log.Printf("[Party] Failed to load party for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "failed to load party",
		}, opcodes.PokemonPartyResponse)
		return false
	}

	partyDTOs := make([]PokemonDTO, 0, len(party))
	for _, p := range party {
		partyDTOs = append(partyDTOs, pokemonToDTO(p))
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"party":   partyDTOs,
	}, opcodes.PokemonPartyResponse)
	return false
}

// HandlePokeMoveLearn handles the player's response to a move learn prompt.
// The client sends forgetSlot (0-3 to forget a move, or -1 to skip learning).
func HandlePokeMoveLearn(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		ForgetSlot int `json:"forgetSlot"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		sendBattleItemError(ses, opcodes.PokeMoveLearnResponse, "Invalid request")
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	current := getBattle(charID)
	if current == nil || current.PendingMoveLearn == nil {
		sendBattleItemError(ses, opcodes.PokeMoveLearnResponse, "No pending move to learn")
		return false
	}
	var response map[string]interface{}
	committed, err := pokebattle.CommitBattle(context.Background(), wh.database, charID, current, func(tx db.DBTX, next *pokebattle.BattleState) error {
		pending := next.PendingMoveLearn
		if req.ForgetSlot < -1 || req.ForgetSlot >= 4 {
			return battleRuleError("Invalid slot index")
		}
		if pending.PokemonIndex < 0 || pending.PokemonIndex >= len(next.PlayerParty) {
			return fmt.Errorf("invalid pending move pokemon index")
		}
		pokemon := next.PlayerParty[pending.PokemonIndex]
		response = map[string]interface{}{"success": true, "skipped": req.ForgetSlot == -1}
		if req.ForgetSlot == -1 {
			response["message"] = fmt.Sprintf("%s did not learn %s.", pokemon.Name, pending.MoveName)
		} else {
			forgotten := pokemon.Moves[req.ForgetSlot].Name
			if err := pokebattle.ForgetAndLearnMove(tx, pokemon, req.ForgetSlot, pending.MoveID); err != nil {
				return err
			}
			response["message"] = fmt.Sprintf("1, 2, and… Poof!\n%s forgot %s.\nAnd…\n%s learned %s!", pokemon.Name, forgotten, pokemon.Name, pending.MoveName)
			response["updatedPokemon"] = pokemonToDTO(pokemon)
			response["forgetSlot"] = req.ForgetSlot
			response["newMoveId"] = pending.MoveID
			response["newMoveName"] = pending.MoveName
		}
		if len(next.PostMoveLearnEvents) > 0 {
			response["postEvents"] = next.PostMoveLearnEvents
		}
		next.PendingMoveLearn = nil
		next.PostMoveLearnEvents = nil
		return nil
	})
	if err != nil {
		sendBattleCommitError(ses, charID, nil, err, opcodes.PokeMoveLearnResponse)
		return false
	}
	setBattle(charID, committed)
	ses.SendStreamJSON(response, opcodes.PokeMoveLearnResponse)
	sendPokemonPartySnapshot(ses, committed.PlayerParty)
	return false
}

// HandlePokeBattleClose is called by the client when the player dismisses the battle screen.
// This is the only place where the battle is cleaned up from memory.
func HandlePokeBattleClose(ses *session.Session, _ []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	log.Printf("[PokeBattle] Client closed battle for char %d", charID)
	battle := getBattle(charID)
	shouldSendPostBattleScript := battleShouldSendPostBattleMapScript(battle)
	if battle == nil {
		return false
	}
	if err := pokebattle.CloseBattle(context.Background(), wh.database, charID, battle); err != nil {
		log.Printf("[PokeBattle] Close failed for character %d: %v", charID, err)
		return false
	}
	forgetBattle(charID, battle)
	if shouldSendPostBattleScript {
		sendEligibleMapScriptAfterBattleClose(ses, charID, wh)
	}
	return false
}

func battleShouldSendPostBattleMapScript(battle *pokebattle.BattleState) bool {
	if battle == nil || !battle.IsOver() || battle.Trainer == nil {
		return false
	}
	if battle.PlayerWon() {
		return battle.Trainer.WinFlag != ""
	}
	return battle.Trainer.NoBlackoutOnLoss && battle.Trainer.LoseFlag != ""
}

func sendEligibleMapScriptAfterBattleClose(ses *session.Session, charID int64, wh *WorldHandler) {
	if wh == nil || wh.Cutscenes == nil || wh.EventFlags == nil {
		return
	}
	mapID := ses.MapID
	if charData := ses.Client.CharData(); charData != nil {
		mapID = int(charData.MapID)
	}
	mapName := wh.Cutscenes.MapNameForID(mapID)
	if mapName == "" {
		return
	}
	playerFacing := ""
	if wh.PlayerMovement != nil {
		playerFacing, _ = wh.PlayerMovement.GetDirection(int(charID))
	}
	if cs := wh.Cutscenes.FindEligibleMapScriptCutscene(mapName, charID, wh.EventFlags, playerFacing); cs != nil {
		SendCutsceneToPlayer(ses, cs, wh)
	}
}

func battleEndedByRunSuccess(events []pokebattle.BattleEvent) bool {
	for _, event := range events {
		if event.Type == pokebattle.EventRunSuccess {
			return true
		}
	}
	return false
}

// buildBlackoutEndResponse creates a PokeBattleEndNotify payload for a loss/blackout,
// including the last visited Pokémon Center coordinates so the client knows where to warp.
func buildBlackoutEndResponse(charID int64) map[string]interface{} {
	resp := map[string]interface{}{
		"playerWon": false,
		"blackout":  true,
	}

	blackout, blackoutErr := ApplyBlackoutForCharacter(charID)
	if blackoutErr != nil {
		log.Printf("[PokeBattle] Failed to apply blackout state (char %d): %v", charID, blackoutErr)
		opts, err := db_character.LoadOptions(context.Background(), int32(charID))
		if err == nil && opts.LastPokeCenterMapID != 0 {
			resp["blackoutMapId"] = opts.LastPokeCenterMapID
			resp["blackoutX"] = opts.LastPokeCenterX
			resp["blackoutY"] = opts.LastPokeCenterY
			return resp
		}
		// Fall back to Viridian City Pokémon Center
		resp["blackoutMapId"] = 41
		resp["blackoutX"] = 3
		resp["blackoutY"] = 4
		return resp
	}

	resp["money"] = blackout.NewMoney
	resp["moneyLost"] = blackout.MoneyLost
	resp["blackoutMapId"] = blackout.MapID
	resp["blackoutX"] = blackout.X
	resp["blackoutY"] = blackout.Y
	return resp
}
