package world

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

type ScriptedTrainerBattleSpec struct {
	TrainerClass     string
	PartyIndex       int
	TrainerName      string
	TrainerObjectID  int
	WinFlag          string
	LoseFlag         string
	LossMessage      string
	NoBlackoutOnLoss bool
	PostWinMapName   string
	PostWinActions   json.RawMessage
	PostLoseMapName  string
	PostLoseActions  json.RawMessage
}

type ScriptedWildBattleSpec struct {
	PokemonID       int
	Level           int
	WinFlag         string
	PostWinMapName  string
	PostWinActions  json.RawMessage
	AllowedActions  []string
	GuaranteedCatch bool
}

type ActiveBattleSummary struct {
	BattleType       string
	Phase            string
	TrainerClass     string
	TrainerName      string
	WinFlag          string
	LoseFlag         string
	LossMessage      string
	NoBlackoutOnLoss bool
	PostWinMapName   string
	PostWinActions   json.RawMessage
	PostLoseMapName  string
	PostLoseActions  json.RawMessage
	AllowedActions   []string
	GuaranteedCatch  bool
	EnemyParty       []BattlePokemonSummary
}

type BattlePokemonSummary struct {
	Slot      int
	SpeciesID int
	Name      string
	Level     int
}

func StartScriptedTrainerBattle(charID int64, spec ScriptedTrainerBattleSpec) (*pokebattle.BattleState, []pokebattle.BattleEvent, error) {
	var battle *pokebattle.BattleState
	var events []pokebattle.BattleEvent
	err := db.Transaction(context.Background(), db.GlobalWorldDB.DB, func(tx db.DBTX) (err error) {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		battle, events, err = prepareScriptedTrainerBattle(tx, charID, spec)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	setBattle(charID, battle)
	return battle, events, nil
}

func prepareScriptedTrainerBattle(myDB db.DBTX, charID int64, spec ScriptedTrainerBattleSpec) (*pokebattle.BattleState, []pokebattle.BattleEvent, error) {
	if spec.TrainerClass == "" {
		return nil, nil, fmt.Errorf("startTrainerBattle missing trainerClass")
	}
	if spec.PartyIndex <= 0 {
		return nil, nil, fmt.Errorf("startTrainerBattle missing partyIndex")
	}

	trainerParty, err := pokebattle.BuildTrainerParty(myDB, spec.TrainerClass, spec.PartyIndex)
	if err != nil {
		return nil, nil, err
	}
	if len(trainerParty) == 0 {
		return nil, nil, fmt.Errorf("trainer party is empty")
	}
	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil {
		return nil, nil, fmt.Errorf("load player party: %w", err)
	}
	if len(playerParty) == 0 {
		return nil, nil, fmt.Errorf("player party is empty")
	}
	if !partyHasAlivePokemon(playerParty) {
		return nil, nil, fmt.Errorf("player party has no battle-ready pokemon")
	}

	var baseMoney int
	var displayName string
	if err := myDB.QueryRow(`SELECT base_money,display_name FROM phaser_trainer_classes WHERE constant_name=$1`, spec.TrainerClass).Scan(&baseMoney, &displayName); err != nil {
		return nil, nil, err
	}
	prizeMoney := trainerPrizeMoneyFromBase(baseMoney, trainerParty)
	trainerName := spec.TrainerName
	if trainerName == "" {
		trainerName = displayName
	}
	trainerName, err = trainerNameForCharacterFromDB(myDB, charID, spec.TrainerClass, trainerName)
	if err != nil {
		return nil, nil, err
	}

	battle := pokebattle.NewTrainerBattle(playerParty, trainerParty)
	if err := configureBattleObedienceFromDB(myDB, battle, charID); err != nil {
		return nil, nil, err
	}
	battle.Trainer = &pokebattle.TrainerMeta{
		ClassName:        spec.TrainerClass,
		Name:             trainerName,
		PrizeMoney:       prizeMoney,
		TrainerObjectID:  spec.TrainerObjectID,
		WinFlag:          spec.WinFlag,
		LoseFlag:         spec.LoseFlag,
		LossMessage:      spec.LossMessage,
		NoBlackoutOnLoss: spec.NoBlackoutOnLoss,
		PostWinMapName:   spec.PostWinMapName,
		PostWinActions:   spec.PostWinActions,
		PostLoseMapName:  spec.PostLoseMapName,
		PostLoseActions:  spec.PostLoseActions,
	}
	battle, err = pokebattle.StartBattleInTransaction(myDB, charID, battle, func(tx db.DBTX, next *pokebattle.BattleState) error {
		return markPokemonSeen(tx, charID, next.GetEnemyPokemon().ID)
	})
	if err != nil {
		return nil, nil, err
	}

	events := []pokebattle.BattleEvent{
		{Type: pokebattle.EventMessage, Message: trainerName + " wants to fight!"},
		{Type: pokebattle.EventMessage, Message: trainerName + " sent out " + trainerParty[0].Name + "!"},
	}
	return battle, events, nil
}

func StartScriptedWildBattle(charID int64, spec ScriptedWildBattleSpec) (*pokebattle.BattleState, []pokebattle.BattleEvent, error) {
	var battle *pokebattle.BattleState
	var events []pokebattle.BattleEvent
	err := db.Transaction(context.Background(), db.GlobalWorldDB.DB, func(tx db.DBTX) (err error) {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		battle, events, err = prepareScriptedWildBattle(tx, charID, spec)
		return err
	})
	if err != nil {
		return nil, nil, err
	}
	setBattle(charID, battle)
	return battle, events, nil
}

func prepareScriptedWildBattle(myDB db.DBTX, charID int64, spec ScriptedWildBattleSpec) (*pokebattle.BattleState, []pokebattle.BattleEvent, error) {
	if spec.PokemonID <= 0 {
		return nil, nil, fmt.Errorf("startWildBattle missing pokemonId")
	}
	if spec.Level <= 0 {
		return nil, nil, fmt.Errorf("startWildBattle missing level")
	}

	wildPokemon, err := pokebattle.BuildWildPokemon(myDB, spec.PokemonID, spec.Level)
	if err != nil {
		return nil, nil, err
	}
	playerParty, err := pokebattle.LoadParty(myDB, charID)
	if err != nil {
		return nil, nil, fmt.Errorf("load player party: %w", err)
	}
	if len(playerParty) == 0 {
		return nil, nil, fmt.Errorf("player party is empty")
	}
	if !partyHasAlivePokemon(playerParty) {
		return nil, nil, fmt.Errorf("player party has no battle-ready pokemon")
	}

	battle := pokebattle.NewWildBattle(playerParty, wildPokemon)
	if err := configureBattleObedienceFromDB(myDB, battle, charID); err != nil {
		return nil, nil, err
	}
	battle.WildWinFlag = spec.WinFlag
	battle.WildPostWinMapName = spec.PostWinMapName
	battle.WildPostWinActions = spec.PostWinActions
	battle.AllowedActions = spec.AllowedActions
	battle.GuaranteedCatch = spec.GuaranteedCatch
	battle, err = pokebattle.StartBattleInTransaction(myDB, charID, battle, func(tx db.DBTX, next *pokebattle.BattleState) error {
		return markPokemonSeen(tx, charID, next.GetEnemyPokemon().ID)
	})
	if err != nil {
		return nil, nil, err
	}

	events := []pokebattle.BattleEvent{
		{Type: pokebattle.EventMessage, Message: "Wild " + wildPokemon.Name + " appeared!"},
	}
	return battle, events, nil
}

func ClearBattleForCharacter(charID int64) {
	removeBattle(charID)
}

func ActiveBattleSummaryForCharacter(charID int64) *ActiveBattleSummary {
	battle := getBattle(charID)
	if battle == nil || battle.IsOver() {
		return nil
	}

	summary := &ActiveBattleSummary{
		BattleType:      battleTypeToString(battle.BattleType),
		Phase:           phaseToString(battle.Phase),
		WinFlag:         battle.WildWinFlag,
		AllowedActions:  battle.AllowedActions,
		GuaranteedCatch: battle.GuaranteedCatch,
	}
	if battle.WildPostWinMapName != "" {
		summary.PostWinMapName = battle.WildPostWinMapName
	}
	if len(battle.WildPostWinActions) > 0 {
		summary.PostWinActions = battle.WildPostWinActions
	}
	if battle.Trainer != nil {
		summary.TrainerClass = battle.Trainer.ClassName
		summary.TrainerName = battle.Trainer.Name
		summary.WinFlag = battle.Trainer.WinFlag
		summary.LoseFlag = battle.Trainer.LoseFlag
		summary.LossMessage = battle.Trainer.LossMessage
		summary.NoBlackoutOnLoss = battle.Trainer.NoBlackoutOnLoss
		summary.PostWinMapName = battle.Trainer.PostWinMapName
		summary.PostWinActions = battle.Trainer.PostWinActions
		summary.PostLoseMapName = battle.Trainer.PostLoseMapName
		summary.PostLoseActions = battle.Trainer.PostLoseActions
	}
	for slot, pokemon := range battle.EnemyParty {
		if pokemon == nil {
			continue
		}
		summary.EnemyParty = append(summary.EnemyParty, BattlePokemonSummary{
			Slot:      slot,
			SpeciesID: pokemon.ID,
			Name:      pokemon.Name,
			Level:     pokemon.Level,
		})
	}
	return summary
}

func partyHasAlivePokemon(party []*pokebattle.Pokemon) bool {
	for _, pokemon := range party {
		if pokemon != nil && pokemon.CurHP > 0 {
			return true
		}
	}
	return false
}

func trainerPrizeMoney(trainerClass string, trainerParty []*pokebattle.Pokemon) int {
	var baseMoney int
	_ = db.GlobalWorldDB.DB.QueryRow(`SELECT base_money FROM phaser_trainer_classes WHERE constant_name = $1`, trainerClass).Scan(&baseMoney)
	return trainerPrizeMoneyFromBase(baseMoney, trainerParty)
}

func trainerPrizeMoneyFromBase(baseMoney int, trainerParty []*pokebattle.Pokemon) int {
	// Red/Blue pays the class rate times the level of the last party member,
	// rather than the highest level anywhere in the party.
	for i := len(trainerParty) - 1; i >= 0; i-- {
		if trainerParty[i] != nil {
			return baseMoney * trainerParty[i].Level
		}
	}
	return 0
}

func trainerDisplayName(trainerClass string) string {
	var displayName string
	_ = db.GlobalWorldDB.DB.QueryRow(`SELECT display_name FROM phaser_trainer_classes WHERE constant_name = $1`, trainerClass).Scan(&displayName)
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	return trainerClass
}
