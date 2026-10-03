package world

import (
	"errors"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

type BattleCommandResponse struct {
	Success   bool                                 `json:"success" tstype:"true"`
	RequestID string                               `json:"requestId"`
	Battle    *GameplayBattleState                 `json:"battle" tstype:"GameplayBattleState | null"`
	Position  protocol.OwnedPlayerPositionResponse `json:"position" tstype:"import(\"./protocol\").OwnedPlayerPositionResponse"`
	Events    []pokebattle.BattleEvent             `json:"events"`
	End       *BattleEndOutcome                    `json:"end,omitempty"`
	Learning  *BattleLearningOutcome               `json:"learning,omitempty"`
}

type BattleCommandError struct {
	Success   bool   `json:"success" tstype:"false"`
	RequestID string `json:"requestId"`
	Error     string `json:"error"`
}

type BattleEndOutcome struct {
	PlayerWon     bool   `json:"playerWon"`
	SentToPC      bool   `json:"sentToPC,omitempty"`
	PCBox         int    `json:"pcBox,omitempty"`
	Blackout      bool   `json:"blackout,omitempty"`
	LossMessage   string `json:"lossMessage,omitempty"`
	BlackoutMapID int    `json:"blackoutMapId"`
	BlackoutX     int    `json:"blackoutX"`
	BlackoutY     int    `json:"blackoutY"`
	Money         int    `json:"money,omitempty"`
	MoneyLost     int    `json:"moneyLost,omitempty"`
}

type BattleLearningOutcome struct {
	Skipped        bool                     `json:"skipped"`
	Message        string                   `json:"message"`
	UpdatedPokemon *PokemonDTO              `json:"updatedPokemon,omitempty"`
	ForgetSlot     int                      `json:"forgetSlot,omitempty"`
	NewMoveID      int                      `json:"newMoveId,omitempty"`
	NewMoveName    string                   `json:"newMoveName,omitempty"`
	PostEvents     []pokebattle.BattleEvent `json:"postEvents,omitempty"`
}

func validBattleRequestID(requestID string) bool { return requestID != "" && len(requestID) <= 64 }

func sendBattleCommitError(ses *session.Session, wh *WorldHandler, requestID string, charID int64, current *pokebattle.BattleState, err error, opcode opcodes.OpCode) {
	var rule battleRuleError
	if errors.As(err, &rule) {
		if current != nil {
			sendBattleNoTurnMessage(ses, wh, requestID, current, rule.Error(), opcode)
		} else {
			sendBattleCommandError(ses, requestID, opcode, rule.Error())
		}
		return
	}
	log.Printf("[PokeBattle] Commit failed for character %d: %v", charID, err)
	sendBattleCommandError(ses, requestID, opcode, "Could not save this battle action. Recover its current state.")
}

func publishBattleTurn(ses *session.Session, wh *WorldHandler, charID int64, battle *pokebattle.BattleState, result battleTurnResult, opcode opcodes.OpCode, requestID string) {
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
	response := BattleCommandResponse{Success: true, RequestID: requestID, Position: wh.ownedPlayerSnapshot(ses, requestID), Battle: gameplayBattleSnapshot(battle), Events: result.Events}
	if response.Events == nil {
		response.Events = []pokebattle.BattleEvent{}
	}
	if battle.IsOver() {
		if battle.Trainer != nil && battle.Trainer.TrainerObjectID > 0 && wh.TrainerEncounter != nil {
			wh.TrainerEncounter.ClearSpottedByTrainer(charID, battle.Trainer.TrainerObjectID)
		}
		end := &BattleEndOutcome{PlayerWon: battle.PlayerWon() || battle.PlayerCaught}
		if result.SentToPC {
			end.SentToPC = true
			end.PCBox = result.PCBox + 1
		}
		if result.Lost {
			end.Blackout = !result.NoBlackoutOnLoss
			end.LossMessage = result.LossMessage
		}
		if b := result.Blackout; b != nil {
			refreshSafariFlags(wh, charID)
			publishCommittedPlayerPosition(ses, wh, b.MapID, b.X, b.Y, "DOWN")
			end.Money = b.NewMoney
			end.MoneyLost = b.MoneyLost
			end.BlackoutMapID = b.MapID
			end.BlackoutX = b.X
			end.BlackoutY = b.Y
		}
		response.End = end
	}
	response.Position = wh.ownedPlayerSnapshot(ses, requestID)
	// Events and terminal outcome share one correlated publication. A late reply
	// has no separate uncorrelated end notification that can mutate a newer panel.
	ses.SendStreamJSON(response, opcode)
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

func sendBattleNoTurnMessage(ses *session.Session, wh *WorldHandler, requestID string, battle *pokebattle.BattleState, message string, responseOpcode opcodes.OpCode) {
	ses.SendStreamJSON(BattleCommandResponse{Success: true, RequestID: requestID, Position: wh.ownedPlayerSnapshot(ses, requestID), Battle: gameplayBattleSnapshot(battle), Events: []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: message}}}, responseOpcode)
}

func sendBattleCommandError(ses *session.Session, requestID string, responseOpcode opcodes.OpCode, message string) {
	ses.SendStreamJSON(BattleCommandError{RequestID: requestID, Error: message}, responseOpcode)
}
