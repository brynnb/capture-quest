package world

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/db/pokedex"
	"capturequest/internal/itemuse"
	"capturequest/internal/pokebattle"
)

type battleRuleError string

func (e battleRuleError) Error() string { return string(e) }

type battleTurnResult struct {
	Script           *cutsceneMutation
	Events           []pokebattle.BattleEvent
	SentToPC         bool
	PCBox            int
	Fled             bool
	Lost             bool
	NoBlackoutOnLoss bool
	LossMessage      string
	Flags            []string
	Money            int64
	WalletChanged    bool
	Blackout         *BlackoutResult
}

// applyBattleTurn executes gameplay against a private battle and the supplied
// transaction. It cannot publish messages or change world/session caches.
func applyBattleTurn(tx db.DBTX, charID int64, battle *pokebattle.BattleState, req PokeBattleActionRequest) (battleTurnResult, error) {
	result := battleTurnResult{PCBox: -1}
	if battle.IsOver() {
		return result, battleRuleError("Not in battle")
	}
	if battle.Phase != pokebattle.PhaseActionSelect && battle.Phase != pokebattle.PhaseMoveSelect {
		return result, battleRuleError("Choose a Pokémon before taking another turn")
	}
	previousEnemy := battle.EnemyActive
	var events []pokebattle.BattleEvent
	var err error
	switch req.Action {
	case "fight", "run", "switch":
		action := pokebattle.ActionFight
		if req.Action == "run" {
			action = pokebattle.ActionRun
		}
		if req.Action == "switch" {
			action = pokebattle.ActionSwitch
			if err := validateBattleSwitch(battle, req.MoveSlot); err != nil {
				return result, battleRuleError(err.Error())
			}
		}
		if !battle.IsActionAllowed(action) {
			return result, battleRuleError("Use an item.")
		}
		events = battle.SubmitAction(pokebattle.TurnAction{Action: action, MoveSlot: req.MoveSlot})
	case "item":
		if !battle.IsActionAllowed(pokebattle.ActionItem) {
			return result, battleRuleError("Use an item.")
		}
		events, err = applyBattleInventoryItem(tx, charID, battle, req)
		if err != nil {
			return result, err
		}
	default:
		return result, battleRuleError("Invalid battle action")
	}
	result.Events = events
	if battle.Trainer != nil && battle.EnemyActive != previousEnemy {
		if err := markPokemonSeen(tx, charID, battle.GetEnemyPokemon().ID); err != nil {
			return result, err
		}
	}
	return settleBattleTurn(tx, charID, battle, result)
}

func applyBattleInventoryItem(tx db.DBTX, charID int64, battle *pokebattle.BattleState, req PokeBattleActionRequest) ([]pokebattle.BattleEvent, error) {
	store := cqitems.NewStore(tx)
	var owned *cqitems.CQInventoryItem
	var err error
	if req.InstanceID > 0 {
		owned, err = store.FindInventoryItemByInstanceID(int32(charID), req.InstanceID)
	} else if req.ItemID > 0 {
		owned, err = store.FindInventoryItemByItemID(int32(charID), req.ItemID)
	} else {
		return nil, battleRuleError("No item selected")
	}
	if err == sql.ErrNoRows {
		return nil, battleRuleError("You don't have that item")
	}
	if err != nil {
		return nil, err
	}
	if req.ItemID > 0 && owned.Item.ID != req.ItemID {
		return nil, battleRuleError("Item instance does not match the requested item")
	}
	var owns bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM cq_item_instances WHERE id=$1 AND owner_id=$2 AND owner_type=0 AND quantity>0)`, owned.Instance.ID, charID).Scan(&owns); err != nil {
		return nil, err
	}
	if !owns {
		return nil, battleRuleError("You don't have that item")
	}
	item := owned.Item
	if battle.GuaranteedCatch && item.BallModifier <= 0 {
		return nil, battleRuleError("Use a POKé BALL.")
	}
	if item.BallModifier > 0 {
		if battle.BattleType == pokebattle.BattleTrainer {
			return nil, battleRuleError("You can't catch another trainer's Pokémon!")
		}
		if _, err := store.DecrementItemQuantity(int32(charID), owned.Instance.ID); err != nil {
			return nil, err
		}
		return battle.SubmitAction(pokebattle.TurnAction{Action: pokebattle.ActionItem, BallModifier: item.BallModifier, ItemID: item.ID}), nil
	}
	if !item.IsUsable {
		return nil, battleRuleError("That item can't be used here")
	}
	consume := true
	var events []pokebattle.BattleEvent
	switch {
	case itemuse.ShortName(item) == "POKE_DOLL" || item.BonusFlee > 0:
		if battle.BattleType == pokebattle.BattleTrainer {
			return nil, battleRuleError("Can't escape from a trainer battle!")
		}
		battle.Phase = pokebattle.PhaseBattleEnd
		events = []pokebattle.BattleEvent{{Type: pokebattle.EventRunSuccess, Message: "Got away safely!"}}
	case itemuse.ShortName(item) == "POKE_FLUTE":
		consume = false
		message, err := itemuse.ApplyBattleFlute(battle)
		if err != nil {
			return nil, battleRuleError(err.Error())
		}
		events = []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: message}}
	case itemuse.IsMedicine(item):
		slot := req.TargetSlot
		if slot < 0 {
			slot = battle.PlayerActive
		}
		if slot < 0 || slot >= len(battle.PlayerParty) || battle.PlayerParty[slot] == nil {
			return nil, battleRuleError("Invalid target Pokémon")
		}
		target := battle.PlayerParty[slot]
		message, err := pokebattle.ApplyItemEffect(target, itemuse.MedicineEffect(item), req.MoveSlot)
		if err != nil {
			return nil, battleRuleError(err.Error())
		}
		events = []pokebattle.BattleEvent{itemEffectEvent(message, target)}
	case itemuse.HasBattleEffect(item):
		message, err := itemuse.ApplyBattleBoost(item, battle.GetPlayerPokemon())
		if err != nil {
			return nil, battleRuleError(err.Error())
		}
		events = []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: message}}
	default:
		return nil, battleRuleError("That item can't be used here")
	}
	if consume {
		if _, err := store.DecrementItemQuantity(int32(charID), owned.Instance.ID); err != nil {
			return nil, err
		}
	}
	if !battle.IsOver() {
		events = append(events, battle.ExecuteEnemyTurn()...)
	}
	return events, nil
}

func settleBattleTurn(tx db.DBTX, charID int64, battle *pokebattle.BattleState, result battleTurnResult) (battleTurnResult, error) {
	if !battle.IsOver() {
		return result, nil
	}
	if battle.PlayerWon() {
		var err error
		result.Events, err = awardBattleExperience(tx, charID, battle, result.Events)
		if err != nil {
			return result, err
		}
		if trainer := battle.Trainer; trainer != nil {
			if trainer.TrainerObjectID > 0 {
				if _, err := tx.Exec(`INSERT INTO character_defeated_trainers(character_id,trainer_object_id) VALUES($1,$2) ON CONFLICT(character_id,trainer_object_id) DO NOTHING`, charID, trainer.TrainerObjectID); err != nil {
					return result, err
				}
			}
			result.Flags = append(result.Flags, trainer.WinFlag)
			postEvents := []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: getTrainerDefeatText(trainer.ClassName)}}
			if trainer.PrizeMoney > 0 {
				if err := tx.QueryRow(`INSERT INTO character_wallet(character_id,pokedollars) VALUES($1,$2) ON CONFLICT(character_id) DO UPDATE SET pokedollars=character_wallet.pokedollars+EXCLUDED.pokedollars RETURNING pokedollars`, charID, trainer.PrizeMoney).Scan(&result.Money); err != nil {
					return result, err
				}
				result.WalletChanged = true
				postEvents = append(postEvents, pokebattle.BattleEvent{Type: pokebattle.EventMessage, Message: fmt.Sprintf("You got ¥%d for winning!", trainer.PrizeMoney)})
			}
			if battle.PendingMoveLearn != nil {
				battle.PostMoveLearnEvents = postEvents
			} else {
				result.Events = append(result.Events, postEvents...)
			}
		}
	}
	if battle.PlayerCaught {
		battle.Capture = &pokebattle.CapturePlacement{}
		caught := battle.GetEnemyPokemon()
		caught.IsWild = false
		if err := pokedex.MarkCaught(tx, charID, caught.ID); err != nil {
			return result, err
		}
		if len(battle.PlayerParty) < 6 {
			battle.PlayerParty = append(battle.PlayerParty, caught)
		} else {
			box, _, err := pokebattle.SavePokemonToPC(tx, charID, caught)
			if err != nil {
				return result, err
			}
			result.SentToPC = true
			result.PCBox = box
			battle.Capture.SentToPC = true
			battle.Capture.PCBox = box
		}
	}
	if battle.BattleType == pokebattle.BattleWild && (battle.PlayerWon() || battle.PlayerCaught) {
		result.Flags = append(result.Flags, battle.WildWinFlag)
	}
	result.Fled = battleEndedByRunSuccess(result.Events)
	result.Lost = !battle.PlayerWon() && !battle.PlayerCaught && !result.Fled
	if result.Lost && battle.Trainer != nil {
		result.NoBlackoutOnLoss = battle.Trainer.NoBlackoutOnLoss
		result.LossMessage = battle.Trainer.LossMessage
		result.Flags = append(result.Flags, battle.Trainer.LoseFlag)
		if result.LossMessage != "" {
			for i := range result.Events {
				if result.Events[i].Type == pokebattle.EventBattleLose {
					result.Events[i].Message = result.LossMessage
				}
			}
		}
	}
	for _, flag := range result.Flags {
		if flag == "" {
			continue
		}
		if err := writeEventFlag(tx, charID, flag, true); err != nil {
			return result, err
		}
	}
	if result.Lost {
		HealPokemonParty(battle.PlayerParty)
	}
	var actions json.RawMessage
	var mapName string
	if trainer := battle.Trainer; trainer != nil {
		if battle.PlayerWon() {
			actions = trainer.PostWinActions
			mapName = trainer.PostWinMapName
		}
		if result.Lost {
			actions = trainer.PostLoseActions
			mapName = trainer.PostLoseMapName
		}
	} else if battle.PlayerWon() || battle.PlayerCaught {
		actions = battle.WildPostWinActions
		mapName = battle.WildPostWinMapName
	}
	if len(actions) > 0 && string(actions) != "null" {
		mutation := &cutsceneMutation{database: tx, characterID: charID, party: &battle.PlayerParty, boundBattle: true}
		_, _, err := applyCutsceneActionList(CutsceneActionContext{mutation: mutation}, mapName, actions, charID)
		if err != nil {
			return result, fmt.Errorf("post-battle script: %w", err)
		}
		result.Script = mutation
	}
	if result.Lost && !result.NoBlackoutOnLoss {
		blackout, err := applyBlackoutInTransaction(tx, charID)
		if err != nil {
			return result, err
		}
		result.Blackout = &blackout
		result.WalletChanged = true
		result.Money = int64(blackout.NewMoney)
	}
	return result, nil
}

func awardBattleExperience(tx db.DBTX, charID int64, battle *pokebattle.BattleState, events []pokebattle.BattleEvent) ([]pokebattle.BattleEvent, error) {
	player := battle.GetPlayerPokemon()
	if player == nil {
		return nil, fmt.Errorf("battle winner has no active pokemon")
	}
	experience := 0
	for _, enemy := range battle.EnemyParty {
		experience += pokebattle.CalculateBattleExp(enemy.BaseExp, enemy.Level, battle.Trainer != nil)
		pokebattle.AddEVsFromDefeated(player, enemy)
	}
	if experience <= 0 {
		return events, nil
	}
	oldLevel := player.Level
	player.Exp += experience
	newLevel := pokebattle.LevelForExp(player.GrowthRt, player.Exp)
	if newLevel > 100 {
		newLevel = 100
	}
	events = append(events, pokebattle.BattleEvent{Type: pokebattle.EventExpGained, Message: fmt.Sprintf("%s gained %d Exp. Points!", player.Name, experience), ExpGained: experience})
	if newLevel <= oldLevel {
		return events, nil
	}
	oldHP := player.MaxHP
	player.Level = newLevel
	player.RecalculateStats()
	player.CurHP += player.MaxHP - oldHP
	events = append(events, pokebattle.BattleEvent{Type: pokebattle.EventMessage, Message: fmt.Sprintf("%s grew to level %d!", player.Name, newLevel)})
	evolvedID, evolvedName := pokebattle.CheckEvolution(tx, player)
	if player.EvolveLevel > 0 && player.Level >= player.EvolveLevel && player.EvolvePokemonName != "" && evolvedID == 0 {
		return nil, fmt.Errorf("could not resolve evolution %q", player.EvolvePokemonName)
	}
	if evolvedID > 0 {
		oldName := player.Name
		if err := pokebattle.EvolvePokemon(tx, player, evolvedID); err != nil {
			return nil, err
		}
		if err := pokedex.MarkCaught(tx, charID, evolvedID); err != nil {
			return nil, err
		}
		events = append(events, pokebattle.BattleEvent{Type: pokebattle.EventEvolution, Message: fmt.Sprintf("What? %s is evolving!\n%s evolved into %s!", oldName, oldName, evolvedName), EvolvedSpeciesID: evolvedID, EvolvedName: evolvedName})
	}
	moves, err := pokebattle.GetMovesLearnedInRange(tx, player.ID, oldLevel, newLevel)
	if err != nil {
		return nil, err
	}
	for _, learned := range moves {
		known := false
		empty := -1
		for i, m := range player.Moves {
			if m.ID == learned.MoveID {
				known = true
			}
			if m.ID == 0 && empty < 0 {
				empty = i
			}
		}
		if known {
			continue
		}
		if empty >= 0 {
			move, err := pokebattle.LoadMoveSlotFromDB(tx, learned.MoveID)
			if err != nil {
				return nil, err
			}
			player.Moves[empty] = move
			events = append(events, pokebattle.BattleEvent{Type: pokebattle.EventMoveLearned, Message: fmt.Sprintf("%s learned %s!", player.Name, learned.MoveName), NewMoveID: learned.MoveID, NewMoveName: learned.MoveName, LearnedSlot: empty})
		} else {
			battle.PendingMoveLearn = &pokebattle.PendingMove{PokemonIndex: battle.PlayerActive, MoveID: learned.MoveID, MoveName: learned.MoveName}
			events = append(events, pokebattle.BattleEvent{Type: pokebattle.EventMoveLearnPrompt, Message: fmt.Sprintf("%s wants to learn %s, but already knows 4 moves!", player.Name, learned.MoveName), NewMoveID: learned.MoveID, NewMoveName: learned.MoveName})
			break
		}
	}
	return events, nil
}
