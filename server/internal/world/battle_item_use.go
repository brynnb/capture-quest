package world

import (
	"fmt"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

type battleItemUsePayload struct {
	ItemID     int32
	InstanceID int32
	TargetSlot int
	MoveSlot   int
}

func findBattleInventoryItem(charID int32, req battleItemUsePayload) (*cqitems.CQInventoryItem, error) {
	if req.InstanceID > 0 {
		invItem, err := cqitems.FindInventoryItemByInstanceID(charID, req.InstanceID)
		if err != nil {
			return nil, err
		}
		if req.ItemID > 0 && invItem.Item.ID != req.ItemID {
			return nil, fmt.Errorf("item instance does not match requested item")
		}
		return invItem, nil
	}
	if req.ItemID <= 0 {
		return nil, fmt.Errorf("no item selected")
	}
	return cqitems.FindInventoryItemByItemID(charID, req.ItemID)
}

func useBattleInventoryItem(ses *session.Session, wh *WorldHandler, charID int64, battle *pokebattle.BattleState, invItem *cqitems.CQInventoryItem, req battleItemUsePayload, responseOpcode opcodes.OpCode) bool {
	item := invItem.Item
	if !item.IsUsable {
		sendBattleNoTurnMessage(ses, battle, "That item can't be used like that.", responseOpcode)
		return false
	}

	switch {
	case itemShortName(item) == "POKE_DOLL" || item.BonusFlee > 0:
		usePokeDollInBattle(ses, wh, charID, battle, invItem, responseOpcode)
		return false
	case itemShortName(item) == "POKE_FLUTE":
		usePokeFluteInBattle(ses, charID, battle, invItem, responseOpcode)
		return false
	case isMedicineItem(item):
		useMedicineInBattle(ses, charID, battle, invItem, req, responseOpcode)
		return false
	case battleItemHasEffect(item):
		useBoostItemInBattle(ses, charID, battle, invItem, responseOpcode)
		return false
	default:
		sendBattleNoTurnMessage(ses, battle, "That item can't be used here", responseOpcode)
		return false
	}
}

func useMedicineInBattle(ses *session.Session, charID int64, battle *pokebattle.BattleState, invItem *cqitems.CQInventoryItem, req battleItemUsePayload, responseOpcode opcodes.OpCode) {
	targetIdx := req.TargetSlot
	if targetIdx < 0 {
		targetIdx = battle.PlayerActive
	}
	if targetIdx < 0 || targetIdx >= len(battle.PlayerParty) || battle.PlayerParty[targetIdx] == nil {
		sendBattleNoTurnMessage(ses, battle, "Invalid target Pokémon", responseOpcode)
		return
	}

	targetPoke := battle.PlayerParty[targetIdx]
	msg, applyErr := pokebattle.ApplyItemEffect(targetPoke, medicineEffectFromItem(invItem.Item), req.MoveSlot)
	if applyErr != nil {
		sendBattleNoTurnMessage(ses, battle, applyErr.Error(), responseOpcode)
		return
	}

	cqitems.DecrementItemQuantity(int32(charID), invItem.Instance.ID)
	events := []pokebattle.BattleEvent{itemEffectEvent(msg, targetPoke)}
	if !battle.IsOver() {
		events = append(events, battle.ExecuteEnemyTurn()...)
	}
	sendBattleItemSuccess(ses, charID, battle, events, responseOpcode)
}

func useBoostItemInBattle(ses *session.Session, charID int64, battle *pokebattle.BattleState, invItem *cqitems.CQInventoryItem, responseOpcode opcodes.OpCode) {
	player := battle.GetPlayerPokemon()
	msg, applyErr := applyBattleBoostItem(invItem.Item, player)
	if applyErr != nil {
		sendBattleNoTurnMessage(ses, battle, applyErr.Error(), responseOpcode)
		return
	}

	cqitems.DecrementItemQuantity(int32(charID), invItem.Instance.ID)
	events := []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: msg}}
	if !battle.IsOver() {
		events = append(events, battle.ExecuteEnemyTurn()...)
	}
	sendBattleItemSuccess(ses, charID, battle, events, responseOpcode)
}

func usePokeFluteInBattle(ses *session.Session, charID int64, battle *pokebattle.BattleState, invItem *cqitems.CQInventoryItem, responseOpcode opcodes.OpCode) {
	msg, applyErr := applyPokeFluteBattle(battle)
	if applyErr != nil {
		sendBattleNoTurnMessage(ses, battle, applyErr.Error(), responseOpcode)
		return
	}

	events := []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: msg}}
	if !battle.IsOver() {
		events = append(events, battle.ExecuteEnemyTurn()...)
	}
	sendBattleItemSuccess(ses, charID, battle, events, responseOpcode)
}

func usePokeDollInBattle(ses *session.Session, wh *WorldHandler, charID int64, battle *pokebattle.BattleState, invItem *cqitems.CQInventoryItem, responseOpcode opcodes.OpCode) {
	if battle.BattleType == pokebattle.BattleTrainer {
		sendBattleNoTurnMessage(ses, battle, "Can't escape from a trainer battle!", responseOpcode)
		return
	}

	cqitems.DecrementItemQuantity(int32(charID), invItem.Instance.ID)
	battle.Phase = pokebattle.PhaseBattleEnd
	events := []pokebattle.BattleEvent{{Type: pokebattle.EventRunSuccess, Message: "Got away safely!"}}
	sendBattleItemSuccess(ses, charID, battle, events, responseOpcode)
	ses.SendStreamJSON(map[string]interface{}{"playerWon": false}, opcodes.PokeBattleEndNotify)
	sendPartyUpdate(ses)
	if battle.Trainer != nil && battle.Trainer.TrainerObjectID > 0 && wh != nil && wh.TrainerEncounter != nil {
		wh.TrainerEncounter.ClearSpottedByTrainer(charID, battle.Trainer.TrainerObjectID)
	}
}

func sendBattleItemSuccess(ses *session.Session, charID int64, battle *pokebattle.BattleState, events []pokebattle.BattleEvent, responseOpcode opcodes.OpCode) {
	myDB := db.GlobalWorldDB.DB
	if err := pokebattle.SavePokemonAfterBattle(myDB, charID, battle.PlayerParty); err != nil {
		// Keep the battle response flowing; the caller logs the durable battle action.
	}

	resp := map[string]interface{}{
		"success":       true,
		"playerPokemon": pokemonToDTO(battle.GetPlayerPokemon()),
		"enemyPokemon":  pokemonToDTO(battle.GetEnemyPokemon()),
		"phase":         phaseToString(battle.Phase),
		"turnNumber":    battle.TurnNumber,
		"events":        events,
	}
	attachBattlePartyMetadata(resp, battle)
	ses.SendStreamJSON(resp, responseOpcode)
	sendPartyUpdate(ses)
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
	if battle != nil && !battle.IsOver() {
		battle.Phase = pokebattle.PhaseActionSelect
	}
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
