package itemuse

import (
	"fmt"
	"strings"

	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
)

const (
	cqItemTypeMisc           = 0
	cqItemTypePokeBall       = 1
	cqItemTypeMedicine       = 2
	cqItemTypeBattleItem     = 3
	cqItemTypeFieldItem      = 4
	cqItemTypeTM             = 5
	cqItemTypeHM             = 6
	cqItemTypeEvolutionStone = 9

	vitaminEVIncrease = 2560
	vitaminEVCap      = 25600
)

func ShortName(item cqitems.CQItem) string {
	return strings.ToUpper(item.ShortName)
}

func itemStatusCure(item cqitems.CQItem) string {
	if item.StatusCure == nil {
		return ""
	}
	return *item.StatusCure
}

func IsMedicine(item cqitems.CQItem) bool {
	return item.HealAmount > 0 || itemStatusCure(item) != "" || item.RevivePercent > 0 || item.PPRestore > 0
}

func isPPRestoreAll(item cqitems.CQItem) bool {
	switch ShortName(item) {
	case "ELIXIR", "MAX_ELIXIR":
		return true
	default:
		return false
	}
}

func MedicineEffect(item cqitems.CQItem) pokebattle.ItemEffect {
	eff := pokebattle.ItemEffectFromData(
		int(item.HealAmount),
		itemStatusCure(item),
		int(item.PPRestore),
		int(item.RevivePercent),
	)
	eff.PPRestoreAll = isPPRestoreAll(item)
	return eff
}

func isRareCandy(item cqitems.CQItem) bool {
	return ShortName(item) == "RARE_CANDY" || item.ID == 40
}

func isTMHM(item cqitems.CQItem) bool {
	return item.ItemType == cqItemTypeTM || item.ItemType == cqItemTypeHM
}

func isEvolutionStone(item cqitems.CQItem) bool {
	return item.ItemType == cqItemTypeEvolutionStone || strings.HasSuffix(ShortName(item), "_STONE")
}

func isPPUp(item cqitems.CQItem) bool {
	short := ShortName(item)
	return short == "PP_UP" || short == "PP_UP_2"
}

func vitaminTarget(item cqitems.CQItem) (statName string, current func(*pokebattle.Pokemon) *int, ok bool) {
	switch ShortName(item) {
	case "HP_UP":
		return "HP", func(p *pokebattle.Pokemon) *int { return &p.EVs.HP }, true
	case "PROTEIN":
		return "ATTACK", func(p *pokebattle.Pokemon) *int { return &p.EVs.Attack }, true
	case "IRON":
		return "DEFENSE", func(p *pokebattle.Pokemon) *int { return &p.EVs.Defense }, true
	case "CARBOS":
		return "SPEED", func(p *pokebattle.Pokemon) *int { return &p.EVs.Speed }, true
	case "CALCIUM":
		return "SPECIAL", func(p *pokebattle.Pokemon) *int { return &p.EVs.Special }, true
	default:
		return "", nil, false
	}
}

func itemUsableOnPartyOutsideBattle(item cqitems.CQItem) bool {
	if IsMedicine(item) || isRareCandy(item) || isTMHM(item) || isEvolutionStone(item) || isPPUp(item) {
		return true
	}
	_, _, isVitamin := vitaminTarget(item)
	return isVitamin
}

func applyVitamin(item cqitems.CQItem, p *pokebattle.Pokemon) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}
	statName, getEV, ok := vitaminTarget(item)
	if !ok {
		return "", fmt.Errorf("not a vitamin")
	}
	ev := getEV(p)
	if *ev >= vitaminEVCap {
		return "", fmt.Errorf("it won't have any effect")
	}

	oldMaxHP := p.MaxHP
	*ev += vitaminEVIncrease
	if *ev > vitaminEVCap {
		*ev = vitaminEVCap
	}
	p.RecalculateStats()
	if p.MaxHP > oldMaxHP {
		p.CurHP += p.MaxHP - oldMaxHP
	}
	if p.CurHP > p.MaxHP {
		p.CurHP = p.MaxHP
	}
	return fmt.Sprintf("%s's %s rose!", p.Name, statName), nil
}

func applyPPUp(p *pokebattle.Pokemon, moveSlot int) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}
	if moveSlot < 0 || moveSlot > 3 || p.Moves[moveSlot].ID == 0 {
		return "", fmt.Errorf("invalid move slot")
	}
	move := &p.Moves[moveSlot]
	if move.PPUps >= 3 {
		return "", fmt.Errorf("%s's PP won't go any higher", move.Name)
	}

	basePP := move.BasePP
	if basePP <= 0 {
		basePP = move.MaxPP * 5 / (5 + move.PPUps)
		if basePP <= 0 {
			basePP = move.MaxPP
		}
	}
	oldMaxPP := move.MaxPP
	move.PPUps++
	move.BasePP = basePP
	move.MaxPP = pokebattle.MaxPPWithUps(basePP, move.PPUps)
	move.PP += move.MaxPP - oldMaxPP
	if move.PP > move.MaxPP {
		move.PP = move.MaxPP
	}
	return fmt.Sprintf("%s's PP rose!", move.Name), nil
}

func applyStoneEvolution(myDB pokebattle.DBTX, item cqitems.CQItem, p *pokebattle.Pokemon) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}

	evolvedID, ok := stoneEvolutionTarget(ShortName(item), p.ID)
	if !ok {
		return "", fmt.Errorf("it won't have any effect")
	}
	oldName := p.Name
	if err := pokebattle.EvolvePokemon(myDB, p, evolvedID); err != nil {
		return "", fmt.Errorf("failed to evolve %s: %w", oldName, err)
	}
	return fmt.Sprintf("What? %s is evolving!\n%s evolved into %s!", oldName, oldName, p.Name), nil
}

func stoneEvolutionTarget(stone string, pokemonID int) (int, bool) {
	evolutions := map[string]map[int]int{
		"MOON_STONE": {
			30: 31, 33: 34, 35: 36, 39: 40,
		},
		"FIRE_STONE": {
			37: 38, 58: 59, 133: 136,
		},
		"THUNDER_STONE": {
			25: 26, 133: 135,
		},
		"WATER_STONE": {
			61: 62, 90: 91, 120: 121, 133: 134,
		},
		"LEAF_STONE": {
			44: 45, 70: 71, 102: 103,
		},
	}
	targets, ok := evolutions[stone]
	if !ok {
		return 0, false
	}
	evolvedID, ok := targets[pokemonID]
	return evolvedID, ok
}

func HasBattleEffect(item cqitems.CQItem) bool {
	return item.ItemType == cqItemTypeBattleItem ||
		item.IsGuardDrink ||
		item.BonusAttack != 0 ||
		item.BonusDefense != 0 ||
		item.BonusSpeed != 0 ||
		item.BonusSpecial != 0 ||
		item.BonusAccuracy != 0 ||
		item.BonusEvasion != 0 ||
		item.BonusCrit != 0 ||
		item.BonusFlee != 0 ||
		ShortName(item) == "POKE_FLUTE"
}

func ApplyBattleBoost(item cqitems.CQItem, p *pokebattle.Pokemon) (string, error) {
	if p == nil {
		return "", fmt.Errorf("no Pokémon selected")
	}
	if p.IsFainted() {
		return "", fmt.Errorf("%s has fainted", p.Name)
	}

	switch ShortName(item) {
	case "GUARD_SPEC":
		if p.GuardSpec {
			return "", fmt.Errorf("it won't have any effect")
		}
		p.GuardSpec = true
		return fmt.Sprintf("%s became guarded against stat drops!", p.Name), nil
	case "DIRE_HIT":
		if p.DireHit {
			return "", fmt.Errorf("it won't have any effect")
		}
		p.DireHit = true
		return fmt.Sprintf("%s is getting pumped!", p.Name), nil
	}

	boosts := battleStageBoosts(item)
	if len(boosts) == 0 {
		return "", fmt.Errorf("that item can't be used here")
	}

	var messages []string
	for _, boost := range boosts {
		msg, err := raiseBattleStage(p, boost.name, boost.delta)
		if err != nil {
			continue
		}
		messages = append(messages, msg)
	}
	if len(messages) == 0 {
		return "", fmt.Errorf("it won't have any effect")
	}
	return strings.Join(messages, "\n"), nil
}

type battleStageBoost struct {
	name  string
	delta int
}

func battleStageBoosts(item cqitems.CQItem) []battleStageBoost {
	short := ShortName(item)
	candidates := []battleStageBoost{
		{name: "ATTACK", delta: int(defaultBattleBoost(short, "X_ATTACK", item.BonusAttack))},
		{name: "DEFENSE", delta: int(defaultBattleBoost(short, "X_DEFEND", item.BonusDefense))},
		{name: "SPEED", delta: int(defaultBattleBoost(short, "X_SPEED", item.BonusSpeed))},
		{name: "SPECIAL", delta: int(defaultBattleBoost(short, "X_SPECIAL", item.BonusSpecial))},
		{name: "ACCURACY", delta: int(defaultBattleBoost(short, "X_ACCURACY", item.BonusAccuracy))},
		{name: "EVASION", delta: int(item.BonusEvasion)},
	}
	boosts := make([]battleStageBoost, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.delta != 0 {
			boosts = append(boosts, candidate)
		}
	}
	return boosts
}

func defaultBattleBoost(short string, matchingShort string, dbValue int32) int32 {
	if dbValue != 0 {
		return dbValue
	}
	if short == matchingShort {
		return 1
	}
	return 0
}

func raiseBattleStage(p *pokebattle.Pokemon, statName string, delta int) (string, error) {
	stage := battleStagePointer(p, statName)
	if stage == nil {
		return "", fmt.Errorf("unknown stat")
	}
	if *stage >= 6 && delta > 0 {
		return "", fmt.Errorf("it won't have any effect")
	}
	if *stage <= -6 && delta < 0 {
		return "", fmt.Errorf("it won't have any effect")
	}
	*stage += delta
	if *stage > 6 {
		*stage = 6
	}
	if *stage < -6 {
		*stage = -6
	}
	return fmt.Sprintf("%s's %s rose!", p.Name, statName), nil
}

func battleStagePointer(p *pokebattle.Pokemon, statName string) *int {
	switch statName {
	case "ATTACK":
		return &p.AtkStage
	case "DEFENSE":
		return &p.DefStage
	case "SPEED":
		return &p.SpdStage
	case "SPECIAL":
		return &p.SpcStage
	case "ACCURACY":
		return &p.AccStage
	case "EVASION":
		return &p.EvaStage
	default:
		return nil
	}
}

func ApplyBattleFlute(battle *pokebattle.BattleState) (string, error) {
	var woke []string
	for _, p := range []*pokebattle.Pokemon{battle.GetPlayerPokemon(), battle.GetEnemyPokemon()} {
		if p != nil && p.Status == pokebattle.StatusSleep {
			p.ClearMajorStatus()
			woke = append(woke, p.Name)
		}
	}
	if len(woke) == 0 {
		return "", fmt.Errorf("it won't have any effect")
	}
	return "The Poké Flute played!\n" + strings.Join(woke, " woke up!\n") + " woke up!", nil
}
