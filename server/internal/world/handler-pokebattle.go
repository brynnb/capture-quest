package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

// --- Per-session battle state ---

var (
	activeBattles   = make(map[int64]*pokebattle.BattleState) // charID → battle
	activeBattlesMu sync.RWMutex
)

func getBattle(charID int64) *pokebattle.BattleState {
	activeBattlesMu.RLock()
	defer activeBattlesMu.RUnlock()
	return activeBattles[charID]
}

func setBattle(charID int64, b *pokebattle.BattleState) {
	activeBattlesMu.Lock()
	defer activeBattlesMu.Unlock()
	activeBattles[charID] = b
}

func removeBattle(charID int64) {
	activeBattlesMu.Lock()
	defer activeBattlesMu.Unlock()
	delete(activeBattles, charID)
	// Also delete any persisted battle from DB
	myDB := db.GlobalWorldDB.DB
	if err := pokebattle.DeleteBattleState(myDB, charID); err != nil {
		log.Printf("[PokeBattle] Failed to delete saved battle for char %d: %v", charID, err)
	}
}

// saveBattleOnDisconnect persists the in-memory battle to DB (if any) and removes it from memory.
// Called when a player disconnects so the battle can be restored on reconnect.
func saveBattleOnDisconnect(charID int64) {
	activeBattlesMu.Lock()
	battle, exists := activeBattles[charID]
	if exists {
		delete(activeBattles, charID)
	}
	activeBattlesMu.Unlock()

	if !exists || battle == nil {
		log.Printf("[PokeBattle] saveBattleOnDisconnect: no active battle for char %d (exists=%v, nil=%v)", charID, exists, battle == nil)
		return
	}

	log.Printf("[PokeBattle] saveBattleOnDisconnect: saving battle for char %d (phase=%d, pendingMove=%v)", charID, battle.Phase, battle.PendingMoveLearn != nil)

	myDB := db.GlobalWorldDB.DB

	// Save the player's party state first (HP/PP/status may have changed mid-battle)
	if err := pokebattle.SavePokemonAfterBattle(myDB, charID, battle.PlayerParty); err != nil {
		log.Printf("[PokeBattle] Failed to save party on disconnect for char %d: %v", charID, err)
	}

	// Persist the battle state
	if err := pokebattle.SaveBattleState(myDB, charID, battle); err != nil {
		log.Printf("[PokeBattle] Failed to save battle on disconnect for char %d: %v", charID, err)
	} else {
		log.Printf("[PokeBattle] Saved battle state for char %d on disconnect", charID)
	}
}

// restoreBattleOnLogin checks for a saved battle in the DB and restores it.
// The player party is reloaded from character_pokemon (the source of truth),
// NOT from the battle JSON, to avoid sync issues.
// Returns the battle state if one was restored, nil otherwise.
func restoreBattleOnLogin(charID int64) *pokebattle.BattleState {
	myDB := db.GlobalWorldDB.DB
	battle, err := pokebattle.LoadBattleState(myDB, charID)
	if err != nil {
		log.Printf("[PokeBattle] Failed to load saved battle for char %d: %v", charID, err)
		return nil
	}
	if battle == nil {
		return nil
	}

	// Reload the player party from character_pokemon (source of truth).
	// saveBattleOnDisconnect already saved the mid-battle HP/PP/status there.
	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil || len(playerParty) == 0 {
		log.Printf("[PokeBattle] Failed to reload player party for restored battle (char %d): %v", charID, err)
		// Can't restore without a party — clean up
		pokebattle.DeleteBattleState(myDB, charID)
		return nil
	}
	battle.PlayerParty = playerParty
	configureBattleObedience(battle, charID, nil)

	// Clamp PlayerActive to a battle-ready slot. The party is reloaded from the
	// database, so a saved active slot may now point at a fainted Pokémon.
	if battle.PlayerActive < 0 || battle.PlayerActive >= len(playerParty) || playerParty[battle.PlayerActive] == nil || playerParty[battle.PlayerActive].IsFainted() {
		battle.PlayerActive = pokebattle.FirstAlivePartyIndex(playerParty)
	}

	// Restore into the in-memory map
	setBattle(charID, battle)

	// Delete from DB now that it's in memory
	if err := pokebattle.DeleteBattleState(myDB, charID); err != nil {
		log.Printf("[PokeBattle] Failed to delete restored battle for char %d: %v", charID, err)
	}

	log.Printf("[PokeBattle] Restored battle state for char %d from DB (type=%d, phase=%d, enemy=%s L%d)",
		charID, battle.BattleType, battle.Phase,
		battle.EnemyParty[battle.EnemyActive].Name, battle.EnemyParty[battle.EnemyActive].Level)
	return battle
}

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
	setBattle(charID, battle)

	// Mark wild Pokémon as seen in Pokédex (Phase 10.2)
	MarkPokemonSeen(charID, pokemonID)

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
		log.Printf("[PokeBattle] Invalid action request: %v", err)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	battle := getBattle(charID)
	if battle == nil || battle.IsOver() {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "not in battle",
		}, opcodes.PokeBattleActionResponse)
		return false
	}

	var action pokebattle.TurnAction
	switch req.Action {
	case "fight":
		if !battle.IsActionAllowed(pokebattle.ActionFight) {
			sendBattleNoTurnMessage(ses, battle, "Use an item.", opcodes.PokeBattleActionResponse)
			return false
		}
		action = pokebattle.TurnAction{Action: pokebattle.ActionFight, MoveSlot: req.MoveSlot}
	case "run":
		if !battle.IsActionAllowed(pokebattle.ActionRun) {
			sendBattleNoTurnMessage(ses, battle, "Use an item.", opcodes.PokeBattleActionResponse)
			return false
		}
		action = pokebattle.TurnAction{Action: pokebattle.ActionRun}
	case "switch":
		if !battle.IsActionAllowed(pokebattle.ActionSwitch) {
			sendBattleNoTurnMessage(ses, battle, "Use an item.", opcodes.PokeBattleActionResponse)
			return false
		}
		if err := validateBattleSwitch(battle, req.MoveSlot); err != nil {
			sendBattleNoTurnMessage(ses, battle, err.Error(), opcodes.PokeBattleActionResponse)
			return false
		}
		action = pokebattle.TurnAction{Action: pokebattle.ActionSwitch, MoveSlot: req.MoveSlot}
	case "item":
		if !battle.IsActionAllowed(pokebattle.ActionItem) {
			sendBattleNoTurnMessage(ses, battle, "Use an item.", opcodes.PokeBattleActionResponse)
			return false
		}
		invItem, err := findBattleInventoryItem(int32(charID), battleItemUsePayload{
			ItemID:     req.ItemID,
			InstanceID: req.InstanceID,
			TargetSlot: req.TargetSlot,
			MoveSlot:   req.MoveSlot,
		})
		if err != nil || invItem == nil {
			sendBattleNoTurnMessage(ses, battle, "You don't have that item", opcodes.PokeBattleActionResponse)
			return false
		}

		item := invItem.Item
		if item.BallModifier > 0 {
			// Poké Ball — use catch logic
			cqitems.DecrementItemQuantity(int32(charID), invItem.Instance.ID)
			action = pokebattle.TurnAction{
				Action:       pokebattle.ActionItem,
				BallModifier: item.BallModifier,
				ItemID:       item.ID,
			}
		} else {
			if battle.GuaranteedCatch {
				sendBattleNoTurnMessage(ses, battle, "Use a POKé BALL.", opcodes.PokeBattleActionResponse)
				return false
			}
			return useBattleInventoryItem(ses, wh, charID, battle, invItem, battleItemUsePayload{
				ItemID:     item.ID,
				InstanceID: invItem.Instance.ID,
				TargetSlot: req.TargetSlot,
				MoveSlot:   req.MoveSlot,
			}, opcodes.PokeBattleActionResponse)
		}
	default:
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "invalid action: " + req.Action,
		}, opcodes.PokeBattleActionResponse)
		return false
	}

	events := battle.SubmitAction(action)

	// Award experience if the player won
	if battle.IsOver() && battle.PlayerWon() {
		isTrainer := battle.Trainer != nil
		playerPoke := battle.GetPlayerPokemon()

		// Sum XP from all defeated enemy Pokémon
		totalPokeExp := 0
		for _, enemy := range battle.EnemyParty {
			exp := pokebattle.CalculateBattleExp(enemy.BaseExp, enemy.Level, isTrainer)
			totalPokeExp += exp
		}

		// Award EVs from all defeated enemy Pokémon (Gen 1: gain base stats of defeated)
		if playerPoke != nil {
			for _, enemy := range battle.EnemyParty {
				pokebattle.AddEVsFromDefeated(playerPoke, enemy)
			}
		}

		// Award Pokémon XP
		if totalPokeExp > 0 && playerPoke != nil {
			oldLevel := playerPoke.Level
			playerPoke.Exp += totalPokeExp
			newLevel := pokebattle.LevelForExp(playerPoke.GrowthRt, playerPoke.Exp)
			if newLevel > 100 {
				newLevel = 100
			}

			expMsg := fmt.Sprintf("%s gained %d Exp. Points!", playerPoke.Name, totalPokeExp)
			events = append(events, pokebattle.BattleEvent{
				Type:      pokebattle.EventExpGained,
				Message:   expMsg,
				ExpGained: totalPokeExp,
			})

			if newLevel > oldLevel {
				oldMaxHP := playerPoke.MaxHP
				playerPoke.Level = newLevel
				playerPoke.RecalculateStats()
				// In Gen 1, current HP increases by the same amount as max HP on level-up
				playerPoke.CurHP += playerPoke.MaxHP - oldMaxHP
				events = append(events, pokebattle.BattleEvent{
					Type:    pokebattle.EventMessage,
					Message: fmt.Sprintf("%s grew to level %d!", playerPoke.Name, newLevel),
				})

				myDB := db.GlobalWorldDB.DB

				// Check for evolution at the new level
				evolvedID, evolvedName := pokebattle.CheckEvolution(myDB, playerPoke)
				if evolvedID > 0 {
					oldName := playerPoke.Name
					if err := pokebattle.EvolvePokemon(myDB, playerPoke, evolvedID); err != nil {
						log.Printf("[PokeBattle] Failed to evolve %s: %v", oldName, err)
					} else {
						events = append(events, pokebattle.BattleEvent{
							Type:             pokebattle.EventEvolution,
							Message:          fmt.Sprintf("What? %s is evolving!\n%s evolved into %s!", oldName, oldName, evolvedName),
							EvolvedSpeciesID: evolvedID,
							EvolvedName:      evolvedName,
						})
					}
				}

				// Check for new moves learned in the level range
				// Use the (possibly evolved) species ID for the learnset lookup
				newMoves, err := pokebattle.GetMovesLearnedInRange(myDB, playerPoke.ID, oldLevel, newLevel)
				if err != nil {
					log.Printf("[PokeBattle] Failed to check learnset for %s: %v", playerPoke.Name, err)
				}
				for _, lm := range newMoves {
					result := pokebattle.TryLearnMove(myDB, playerPoke, lm.MoveID)
					if result == -2 {
						// Already knows this move, skip
						continue
					} else if result >= 0 {
						// Auto-learned into empty slot
						events = append(events, pokebattle.BattleEvent{
							Type:        pokebattle.EventMoveLearned,
							Message:     fmt.Sprintf("%s learned %s!", playerPoke.Name, lm.MoveName),
							NewMoveID:   lm.MoveID,
							NewMoveName: lm.MoveName,
							LearnedSlot: result,
						})
					} else {
						// All 4 slots full — need player to choose
						// Store pending move on the battle so the client can prompt
						battle.PendingMoveLearn = &pokebattle.PendingMove{
							PokemonIndex: battle.PlayerActive,
							MoveID:       lm.MoveID,
							MoveName:     lm.MoveName,
						}
						events = append(events, pokebattle.BattleEvent{
							Type:        pokebattle.EventMoveLearnPrompt,
							Message:     fmt.Sprintf("%s wants to learn %s, but already knows 4 moves!", playerPoke.Name, lm.MoveName),
							NewMoveID:   lm.MoveID,
							NewMoveName: lm.MoveName,
						})
						break // Only one pending move at a time
					}
				}
			}
		}

		// --- Post-move-learn events: trainer dialogue and prize money ---
		// These are collected separately. If there's a pending move learn prompt,
		// they get stored on the battle and sent after the prompt is resolved.
		var postEvents []pokebattle.BattleEvent

		// Trainer-specific end messages: parting words, prize money
		if battle.Trainer != nil {
			if battle.Trainer.TrainerObjectID > 0 {
				wh.TrainerEncounter.MarkTrainerDefeated(charID, battle.Trainer.TrainerObjectID)
			}
			if battle.Trainer.WinFlag != "" && wh.EventFlags != nil {
				if err := wh.EventFlags.SetFlag(charID, battle.Trainer.WinFlag); err != nil {
					log.Printf("[PokeBattle] Failed to set trainer win flag %s for char %d: %v", battle.Trainer.WinFlag, charID, err)
				}
			}
			if err := applyScriptedTrainerPostWinActions(ses, battle.Trainer, charID, wh); err != nil {
				log.Printf("[PokeBattle] Failed to apply trainer post-win actions for char %d: %v", charID, err)
			}
			postEvents = append(postEvents, pokebattle.BattleEvent{
				Type:    pokebattle.EventMessage,
				Message: getTrainerDefeatText(battle.Trainer.ClassName),
			})
			if battle.Trainer.PrizeMoney > 0 {
				postEvents = append(postEvents, pokebattle.BattleEvent{
					Type:    pokebattle.EventMessage,
					Message: fmt.Sprintf("You got ¥%d for winning!", battle.Trainer.PrizeMoney),
				})
				charData := ses.Client.CharData()
				ctx := context.Background()
				prize := battle.Trainer.PrizeMoney
				if err := db_character.AddPokedollars(ctx, int32(charData.ID), prize); err != nil {
					log.Printf("[PokeBattle] Failed to add prize money %d for char %d: %v", prize, charData.ID, err)
				} else {
					wallet, _ := db_character.GetCharacterWallet(ctx, uint32(charData.ID))
					ses.SendStreamJSON(StructToMap(wallet), opcodes.CharacterWallet)
					log.Printf("[PokeBattle] %s earned ¥%d prize money from trainer battle", charData.Name, prize)
				}
			}
		}

		// If there's a pending move learn, store post-events for later.
		// Otherwise, append them to the main event list now.
		if battle.PendingMoveLearn != nil && len(postEvents) > 0 {
			battle.PostMoveLearnEvents = postEvents
		} else {
			events = append(events, postEvents...)
		}
	}

	// If the player caught a wild Pokémon, add it to their party or PC
	sentToPC := false
	pcBox := -1
	if battle.IsOver() && battle.PlayerCaught {
		caughtPoke := battle.GetEnemyPokemon()
		caughtPoke.IsWild = false
		// Mark as caught in Pokédex (Phase 10.2)
		MarkPokemonCaught(charID, caughtPoke.ID)
		if len(battle.PlayerParty) < 6 {
			battle.PlayerParty = append(battle.PlayerParty, caughtPoke)
			log.Printf("[PokeBattle] Char %d caught %s (L%d) — added to party slot %d",
				charID, caughtPoke.Name, caughtPoke.Level, len(battle.PlayerParty)-1)
		} else {
			// Party full — save to Bill's PC
			myDB := db.GlobalWorldDB.DB
			box, slot, pcErr := pokebattle.SavePokemonToPC(myDB, charID, caughtPoke)
			if pcErr != nil {
				log.Printf("[PokeBattle] Failed to save %s to PC for char %d: %v", caughtPoke.Name, charID, pcErr)
			} else {
				sentToPC = true
				pcBox = box
				log.Printf("[PokeBattle] Char %d party full, sent L%d %s to PC box %d slot %d",
					charID, caughtPoke.Level, caughtPoke.Name, box, slot)
			}
		}
	}

	if battle.IsOver() && battle.BattleType == pokebattle.BattleWild && (battle.PlayerWon() || battle.PlayerCaught) {
		if battle.WildWinFlag != "" && wh.EventFlags != nil {
			if err := wh.EventFlags.SetFlag(charID, battle.WildWinFlag); err != nil {
				log.Printf("[PokeBattle] Failed to set wild win flag %s for char %d: %v", battle.WildWinFlag, charID, err)
			}
		}
		if err := applyScriptedWildPostWinActions(ses, battle, charID, wh); err != nil {
			log.Printf("[PokeBattle] Failed to apply wild post-win actions for char %d: %v", charID, err)
		}
	}

	fled := battleEndedByRunSuccess(events)
	lost := battle.IsOver() && !battle.PlayerWon() && !battle.PlayerCaught && !fled
	noBlackoutOnLoss := false
	lossMessage := ""
	if lost && battle.Trainer != nil {
		noBlackoutOnLoss = battle.Trainer.NoBlackoutOnLoss
		lossMessage = battle.Trainer.LossMessage
		if battle.Trainer.LoseFlag != "" && wh.EventFlags != nil {
			if err := wh.EventFlags.SetFlag(charID, battle.Trainer.LoseFlag); err != nil {
				log.Printf("[PokeBattle] Failed to set trainer lose flag %s for char %d: %v", battle.Trainer.LoseFlag, charID, err)
			}
		}
		if err := applyScriptedTrainerPostLoseActions(ses, battle.Trainer, charID, wh); err != nil {
			log.Printf("[PokeBattle] Failed to apply trainer post-lose actions for char %d: %v", charID, err)
		}
		if lossMessage != "" {
			for i := range events {
				if events[i].Type == pokebattle.EventBattleLose {
					events[i].Message = lossMessage
				}
			}
		}
	}

	// Persist player's Pokémon party to DB after every battle end
	// (win, lose, or flee — HP/PP/status all need saving)
	if battle.IsOver() {
		scriptedHeal := battleHasScriptedPartyHeal(battle, battle.PlayerWon() || battle.PlayerCaught)
		// On loss, heal all party Pokémon to full before saving. Most losses then
		// blackout-warp; scripted tutorial losses can remain in place.
		if lost || scriptedHeal {
			HealPokemonParty(battle.PlayerParty)
			if lost {
				log.Printf("[PokeBattle] Loss: healed all party Pokémon for char %d (blackout=%t)", charID, !noBlackoutOnLoss)
			} else {
				log.Printf("[PokeBattle] Scripted post-battle heal: healed all party Pokémon for char %d", charID)
			}
		}

		myDB := db.GlobalWorldDB.DB
		if err := pokebattle.SavePokemonAfterBattle(myDB, charID, battle.PlayerParty); err != nil {
			log.Printf("[PokeBattle] Failed to persist party for char %d: %v", charID, err)
		}
	}

	// Build response with updated state
	player := battle.GetPlayerPokemon()
	enemy := battle.GetEnemyPokemon()

	resp := map[string]interface{}{
		"success":       true,
		"phase":         phaseToString(battle.Phase),
		"turnNumber":    battle.TurnNumber,
		"events":        events,
		"playerPokemon": pokemonToDTO(player),
		"enemyPokemon":  pokemonToDTO(enemy),
	}
	attachBattlePartyMetadata(resp, battle)

	ses.SendStreamJSON(resp, opcodes.PokeBattleActionResponse)

	// If battle ended, clean up and send end notification
	if battle.IsOver() {
		// Clear spottedBy so trainer can re-trigger if re-battles are enabled
		if battle.Trainer != nil && battle.Trainer.TrainerObjectID > 0 {
			wh.TrainerEncounter.ClearSpottedByTrainer(charID, battle.Trainer.TrainerObjectID)
		}
		var endResp map[string]interface{}
		if battle.PlayerWon() || battle.PlayerCaught {
			endResp = map[string]interface{}{"playerWon": true}
			if sentToPC {
				endResp["sentToPC"] = true
				endResp["pcBox"] = pcBox + 1 // 1-indexed for display
			}
		} else if fled {
			endResp = map[string]interface{}{"playerWon": false}
		} else if lost && noBlackoutOnLoss {
			endResp = map[string]interface{}{
				"playerWon":   false,
				"blackout":    false,
				"lossMessage": lossMessage,
			}
		} else {
			endResp = buildBlackoutEndResponse(charID)
			if lossMessage != "" {
				endResp["lossMessage"] = lossMessage
			}
		}
		ses.SendStreamJSON(endResp, opcodes.PokeBattleEndNotify)

		// Send updated party data so client UI reflects post-battle state
		sendPartyUpdate(ses)

		// Don't remove the battle here — the client will send PokeBattleCloseRequest
		// after the player dismisses the battle end screen. This ensures the battle
		// stays in memory if the player disconnects during event animation or move learn.
		log.Printf("[PokeBattle] Battle over for char %d — waiting for client close (pendingMove=%v)", charID, battle.PendingMoveLearn != nil)
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
		log.Printf("[PokeBattle] Invalid switch request: %v", err)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	battle := getBattle(charID)
	if battle == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "not in battle",
		}, opcodes.PokeBattleSwitchResponse)
		return false
	}

	if !battle.IsActionAllowed(pokebattle.ActionSwitch) {
		sendBattleNoTurnMessage(ses, battle, "Use an item.", opcodes.PokeBattleSwitchResponse)
		return false
	}

	if battle.Phase != pokebattle.PhaseFaintSwitch {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "not in faint switch phase",
		}, opcodes.PokeBattleSwitchResponse)
		return false
	}

	// Handle run attempt during faint switch (wild battles only)
	if req.Action == "run" {
		if battle.BattleType == pokebattle.BattleTrainer {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   "Can't run from a trainer battle!",
			}, opcodes.PokeBattleSwitchResponse)
			return false
		}

		// Attempt to flee using the same run formula
		events := battle.RunFromFaintSwitch()

		player := battle.GetPlayerPokemon()
		enemy := battle.GetEnemyPokemon()

		resp := map[string]interface{}{
			"success":       true,
			"phase":         phaseToString(battle.Phase),
			"turnNumber":    battle.TurnNumber,
			"events":        events,
			"playerPokemon": pokemonToDTO(player),
			"enemyPokemon":  pokemonToDTO(enemy),
		}
		attachBattlePartyMetadata(resp, battle)

		ses.SendStreamJSON(resp, opcodes.PokeBattleSwitchResponse)

		// If the run succeeded, end the battle
		if battle.IsOver() {
			myDB := db.GlobalWorldDB.DB
			if err := pokebattle.SavePokemonAfterBattle(myDB, charID, battle.PlayerParty); err != nil {
				log.Printf("[PokeBattle] Failed to persist party for char %d: %v", charID, err)
			}
			ses.SendStreamJSON(map[string]interface{}{"playerWon": false}, opcodes.PokeBattleEndNotify)
			sendPartyUpdate(ses)
			removeBattle(charID)
		}

		return false
	}

	// Default: switch in a new Pokémon
	events := battle.ForceSwitchIn(req.PartyIndex)

	player := battle.GetPlayerPokemon()
	enemy := battle.GetEnemyPokemon()

	resp := map[string]interface{}{
		"success":       true,
		"phase":         phaseToString(battle.Phase),
		"turnNumber":    battle.TurnNumber,
		"events":        events,
		"playerPokemon": pokemonToDTO(player),
		"enemyPokemon":  pokemonToDTO(enemy),
	}
	attachBattlePartyMetadata(resp, battle)

	ses.SendStreamJSON(resp, opcodes.PokeBattleSwitchResponse)
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
		ForgetSlot int `json:"forgetSlot"` // 0-3 to replace, -1 to skip
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[PokeBattle] Invalid PokeMoveLearnRequest: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "invalid request",
		}, opcodes.PokeMoveLearnResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	battle := getBattle(charID)
	if battle == nil || battle.PendingMoveLearn == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "no pending move to learn",
		}, opcodes.PokeMoveLearnResponse)
		return false
	}

	pending := battle.PendingMoveLearn
	poke := battle.PlayerParty[pending.PokemonIndex]
	myDB := db.GlobalWorldDB.DB

	if req.ForgetSlot == -1 {
		// Player chose not to learn the move
		battle.PendingMoveLearn = nil
		resp := map[string]interface{}{
			"success": true,
			"skipped": true,
			"message": fmt.Sprintf("%s did not learn %s.", poke.Name, pending.MoveName),
		}
		if len(battle.PostMoveLearnEvents) > 0 {
			resp["postEvents"] = battle.PostMoveLearnEvents
			battle.PostMoveLearnEvents = nil
		}
		ses.SendStreamJSON(resp, opcodes.PokeMoveLearnResponse)

		// Persist party (moves may have changed from earlier auto-learns)
		if err := pokebattle.SavePokemonAfterBattle(myDB, charID, battle.PlayerParty); err != nil {
			log.Printf("[PokeBattle] Failed to persist party after move skip for char %d: %v", charID, err)
		}
		sendPartyUpdate(ses)
		// Don't removeBattle here — client sends PokeBattleCloseRequest
		return false
	}

	if req.ForgetSlot < 0 || req.ForgetSlot >= 4 {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "invalid slot index",
		}, opcodes.PokeMoveLearnResponse)
		return false
	}

	forgottenMove := poke.Moves[req.ForgetSlot].Name
	if err := pokebattle.ForgetAndLearnMove(myDB, poke, req.ForgetSlot, pending.MoveID); err != nil {
		log.Printf("[PokeBattle] Failed to learn move %d for %s: %v", pending.MoveID, poke.Name, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "failed to learn move",
		}, opcodes.PokeMoveLearnResponse)
		return false
	}

	battle.PendingMoveLearn = nil

	// Build updated pokemon DTO
	pokeDTO := pokemonToDTO(poke)

	resp := map[string]interface{}{
		"success":        true,
		"skipped":        false,
		"message":        fmt.Sprintf("1, 2, and… Poof!\n%s forgot %s.\nAnd…\n%s learned %s!", poke.Name, forgottenMove, poke.Name, pending.MoveName),
		"updatedPokemon": pokeDTO,
		"forgetSlot":     req.ForgetSlot,
		"newMoveId":      pending.MoveID,
		"newMoveName":    pending.MoveName,
	}
	if len(battle.PostMoveLearnEvents) > 0 {
		resp["postEvents"] = battle.PostMoveLearnEvents
		battle.PostMoveLearnEvents = nil
	}
	ses.SendStreamJSON(resp, opcodes.PokeMoveLearnResponse)

	// Persist party
	if err := pokebattle.SavePokemonAfterBattle(myDB, charID, battle.PlayerParty); err != nil {
		log.Printf("[PokeBattle] Failed to persist party after move learn for char %d: %v", charID, err)
	}
	sendPartyUpdate(ses)
	// Don't removeBattle here — client sends PokeBattleCloseRequest

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
	removeBattle(charID)
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
