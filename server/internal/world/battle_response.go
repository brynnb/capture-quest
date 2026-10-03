package world

import (
	"errors"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

func sendBattleCommitError(ses *session.Session, charID int64, current *pokebattle.BattleState, err error, opcode opcodes.OpCode) {
	var rule battleRuleError
	if errors.As(err, &rule) {
		if current != nil {
			sendBattleNoTurnMessage(ses, current, rule.Error(), opcode)
		} else {
			sendBattleItemError(ses, opcode, rule.Error())
		}
		return
	}
	log.Printf("[PokeBattle] Commit failed for character %d: %v", charID, err)
	sendBattleItemError(ses, opcode, "Could not save this battle action. Please reconnect to reload its state.")
}

func publishBattleTurn(ses *session.Session, wh *WorldHandler, charID int64, battle *pokebattle.BattleState, result battleTurnResult, opcode opcodes.OpCode) {
	if len(result.Flags) > 0 && wh.EventFlags != nil {
		if err := wh.EventFlags.LoadFlags(charID); err != nil {
			log.Printf("[PokeBattle] Refresh committed flags for character %d: %v", charID, err)
		}
	}
	if result.Script != nil {
		result.Script.publish(CutsceneActionContext{Session: ses, WorldHandler: wh, EventFlags: wh.EventFlags})
	}
	if result.WalletChanged {
		ses.SendStreamJSON(map[string]interface{}{"characterId": charID, "pokedollars": result.Money}, opcodes.CharacterWallet)
	}
	response := map[string]interface{}{
		"success": true, "phase": phaseToString(battle.Phase), "turnNumber": battle.TurnNumber,
		"events": result.Events, "playerPokemon": pokemonToDTO(battle.GetPlayerPokemon()), "enemyPokemon": pokemonToDTO(battle.GetEnemyPokemon()),
	}
	attachBattlePartyMetadata(response, battle)
	ses.SendStreamJSON(response, opcode)
	if battle.IsOver() {
		if battle.Trainer != nil && battle.Trainer.TrainerObjectID > 0 && wh.TrainerEncounter != nil {
			wh.TrainerEncounter.ClearSpottedByTrainer(charID, battle.Trainer.TrainerObjectID)
		}
		end := map[string]interface{}{"playerWon": battle.PlayerWon() || battle.PlayerCaught}
		if result.SentToPC {
			end["sentToPC"] = true
			end["pcBox"] = result.PCBox + 1
		}
		if result.Lost {
			end["blackout"] = !result.NoBlackoutOnLoss
			end["lossMessage"] = result.LossMessage
		}
		if b := result.Blackout; b != nil {
			refreshSafariFlags(wh, charID)
			publishCommittedPlayerPosition(ses, wh, b.MapID, b.X, b.Y, "DOWN")
			end["money"] = b.NewMoney
			end["moneyLost"] = b.MoneyLost
			end["blackoutMapId"] = b.MapID
			end["blackoutX"] = b.X
			end["blackoutY"] = b.Y
		}
		ses.SendStreamJSON(end, opcodes.PokeBattleEndNotify)
	}
	sendPokemonPartySnapshot(ses, battle.PlayerParty)
}

func itemEffectEvent(message string, target *pokebattle.Pokemon) pokebattle.BattleEvent {
	event := pokebattle.BattleEvent{Type: pokebattle.EventMessage, Message: message}
	if target != nil {
		event.TargetName = target.Name
		event.TargetHP = target.CurHP
		event.TargetMaxHP = target.MaxHP
	}
	return event
}

func sendBattleNoTurnMessage(ses *session.Session, battle *pokebattle.BattleState, message string, responseOpcode opcodes.OpCode) {
	resp := map[string]interface{}{
		"success":       true,
		"playerPokemon": pokemonToDTO(battle.GetPlayerPokemon()),
		"enemyPokemon":  pokemonToDTO(battle.GetEnemyPokemon()),
		"phase":         phaseToString(battle.Phase),
		"turnNumber":    battle.TurnNumber,
		"events": []pokebattle.BattleEvent{{
			Type:    pokebattle.EventMessage,
			Message: message,
		}},
	}
	attachBattlePartyMetadata(resp, battle)
	ses.SendStreamJSON(resp, responseOpcode)
}

func sendBattleItemError(ses *session.Session, responseOpcode opcodes.OpCode, err string) {
	ses.SendStreamJSON(map[string]interface{}{
		"success": false,
		"error":   err,
	}, responseOpcode)
}
