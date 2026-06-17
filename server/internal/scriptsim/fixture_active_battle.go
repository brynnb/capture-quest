package scriptsim

import (
	"fmt"

	"capturequest/internal/world"
)

func seedFixtureActiveBattle(charID int64, fixture FixtureActiveBattle) error {
	switch fixture.Type {
	case "wild":
		if fixture.PokemonID <= 0 || fixture.Level <= 0 {
			return fmt.Errorf("activeBattle wild fixture requires pokemonId and level")
		}
		_, _, err := world.StartScriptedWildBattle(charID, world.ScriptedWildBattleSpec{
			PokemonID:       fixture.PokemonID,
			Level:           fixture.Level,
			WinFlag:         fixture.WinFlag,
			PostWinMapName:  fixture.PostWinMapName,
			PostWinActions:  fixture.PostWinActions,
			AllowedActions:  fixture.AllowedActions,
			GuaranteedCatch: fixture.GuaranteedCatch,
		})
		if err != nil {
			return fmt.Errorf("seed wild active battle: %w", err)
		}
		return nil
	case "trainer":
		if fixture.TrainerClass == "" || fixture.PartyIndex <= 0 {
			return fmt.Errorf("activeBattle trainer fixture requires trainerClass and partyIndex")
		}
		_, _, err := world.StartScriptedTrainerBattle(charID, world.ScriptedTrainerBattleSpec{
			TrainerClass:     fixture.TrainerClass,
			PartyIndex:       fixture.PartyIndex,
			TrainerName:      fixture.TrainerName,
			WinFlag:          fixture.WinFlag,
			LoseFlag:         fixture.LoseFlag,
			LossMessage:      fixture.LossMessage,
			NoBlackoutOnLoss: fixture.NoBlackoutOnLoss,
			PostWinMapName:   fixture.PostWinMapName,
			PostWinActions:   fixture.PostWinActions,
			PostLoseMapName:  fixture.PostLoseMapName,
			PostLoseActions:  fixture.PostLoseActions,
		})
		if err != nil {
			return fmt.Errorf("seed trainer active battle: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported activeBattle fixture type %q", fixture.Type)
	}
}
