package scriptsim

import (
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/world"
)

func ResolveActiveBattle(charID int64, resolve ResolveBattle, efm *world.EventFlagManager) ([]ActionEffect, error) {
	if resolve.Result != "win" && resolve.Result != "lose" && resolve.Result != "catch" {
		return nil, fmt.Errorf("unsupported resolveBattle result %q", resolve.Result)
	}

	active := world.ActiveBattleSummaryForCharacter(charID)
	if active == nil {
		return nil, fmt.Errorf("resolveBattle requested but no battle is active")
	}
	if resolve.Result == "catch" && active.BattleType != "wild" {
		return nil, fmt.Errorf("resolveBattle catch is only supported for wild battles")
	}
	flag := active.WinFlag
	postActions := active.PostWinActions
	postMapName := active.PostWinMapName
	if resolve.Result == "lose" {
		flag = active.LoseFlag
		postActions = active.PostLoseActions
		postMapName = active.PostLoseMapName
	}
	if flag != "" {
		if err := efm.SetFlag(charID, flag); err != nil {
			return nil, err
		}
	}
	detail := fmt.Sprintf("win trainer=%s winFlag=%s", active.TrainerClass, flag)
	if resolve.Result == "lose" {
		detail = fmt.Sprintf("lose trainer=%s loseFlag=%s", active.TrainerClass, flag)
	}
	if active.BattleType == "wild" {
		target := ""
		if len(active.EnemyParty) > 0 {
			target = fmt.Sprintf("#%d %s", active.EnemyParty[0].SpeciesID, active.EnemyParty[0].Name)
		}
		detail = fmt.Sprintf("win type=%s target=%s winFlag=%s", active.BattleType, target, flag)
		if resolve.Result == "lose" {
			detail = fmt.Sprintf("lose type=%s target=%s loseFlag=%s", active.BattleType, target, flag)
		} else if resolve.Result == "catch" {
			detail = fmt.Sprintf("catch type=%s target=%s winFlag=%s", active.BattleType, target, flag)
		}
	}
	effects := []ActionEffect{{
		Type:    "resolveBattle",
		Detail:  detail,
		Changed: true,
	}}
	if resolve.Result == "catch" {
		catchEffects, err := resolveWildCatch(charID, active)
		if err != nil {
			return effects, err
		}
		effects = append(effects, catchEffects...)
	}
	if len(postActions) > 0 && string(postActions) != "null" {
		if postMapName == "" {
			postMapName = "UNIFIED_OVERWORLD"
		}
		postWinEffects, err := ExecuteActionList(charID, postMapName, postActions, efm)
		if err != nil {
			return effects, err
		}
		effects = append(effects, postWinEffects...)
	}
	if resolve.Result == "lose" && !active.NoBlackoutOnLoss {
		blackout, err := world.ApplyBlackoutForCharacter(charID)
		if err != nil {
			return effects, err
		}
		effects = append(effects, ActionEffect{
			Type:    "blackoutMoney",
			Detail:  fmt.Sprintf("lost=%d money=%d", blackout.MoneyLost, blackout.NewMoney),
			Changed: blackout.MoneyLost > 0,
		})
	}
	world.ClearBattleForCharacter(charID)

	return effects, nil
}

func resolveWildCatch(charID int64, active *world.ActiveBattleSummary) ([]ActionEffect, error) {
	if active == nil || len(active.EnemyParty) == 0 {
		return nil, fmt.Errorf("resolveBattle catch requires an active wild pokemon")
	}
	enemy := active.EnemyParty[0]
	addedToParty, box, slot, err := pokebattle.AddPokemonToPartyOrPC(db.GlobalWorldDB.DB, charID, enemy.SpeciesID, enemy.Level)
	if err != nil {
		return nil, fmt.Errorf("save caught pokemon #%d L%d: %w", enemy.SpeciesID, enemy.Level, err)
	}
	world.MarkPokemonCaught(charID, enemy.SpeciesID)
	detail := fmt.Sprintf("#%d %s L%d", enemy.SpeciesID, enemy.Name, enemy.Level)
	if addedToParty {
		detail += " party"
	} else {
		detail += fmt.Sprintf(" pc=%d/%d", box, slot)
	}
	return []ActionEffect{{
		Type:    "catchPokemon",
		Detail:  detail,
		Changed: true,
	}}, nil
}
