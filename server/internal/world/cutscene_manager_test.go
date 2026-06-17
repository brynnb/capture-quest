package world

import "testing"

func TestSortCutscenesBySpecificityPrefersCaughtGatedReward(t *testing.T) {
	flag := "EVENT_GOT_HM05"
	requiredCaught := 10
	trigger := "TEXT_ROUTE2GATE_OAKS_AIDE"

	cutscenes := []*CutsceneScript{
		{
			ScriptLabel:      "Route2GateOaksAideHM05Blocked",
			MapName:          "ROUTE_2_GATE",
			TriggerType:      "npc_click",
			TriggerLabel:     &trigger,
			RequiresFlagAbst: &flag,
		},
		{
			ScriptLabel:      "Route2GateOaksAideHM05Reward",
			MapName:          "ROUTE_2_GATE",
			TriggerType:      "npc_click",
			TriggerLabel:     &trigger,
			RequiresFlagAbst: &flag,
			RequiresCaught:   &requiredCaught,
		},
	}

	sortCutscenesBySpecificity(cutscenes)

	if cutscenes[0].ScriptLabel != "Route2GateOaksAideHM05Reward" {
		t.Fatalf("first cutscene = %s, want reward branch", cutscenes[0].ScriptLabel)
	}
}

func TestSortCutscenesBySpecificityPrefersMoneyGatedBranch(t *testing.T) {
	flag := "EVENT_BOUGHT_MAGIKARP"
	requiredMoney := 500
	trigger := "TEXT_MTMOONPOKECENTER_MAGIKARP_SALESMAN"

	cutscenes := []*CutsceneScript{
		{
			ScriptLabel:      "MtMoonPokecenterMagikarpSalesmanGeneric",
			MapName:          "MT_MOON_POKECENTER",
			TriggerType:      "npc_click",
			TriggerLabel:     &trigger,
			RequiresFlagAbst: &flag,
		},
		{
			ScriptLabel:      "MtMoonPokecenterMagikarpSalesmanPurchase",
			MapName:          "MT_MOON_POKECENTER",
			TriggerType:      "npc_click",
			TriggerLabel:     &trigger,
			RequiresFlagAbst: &flag,
			RequiresMoney:    &requiredMoney,
		},
	}

	sortCutscenesBySpecificity(cutscenes)

	if cutscenes[0].ScriptLabel != "MtMoonPokecenterMagikarpSalesmanPurchase" {
		t.Fatalf("first cutscene = %s, want money-gated branch", cutscenes[0].ScriptLabel)
	}
}

func TestSortCutscenesBySpecificityPrefersCoinGatedBranch(t *testing.T) {
	flag := "EVENT_GOT_20_COINS_2"
	coinCaseItemID := 69
	fullCoinCase := 9990
	trigger := "TEXT_GAMECORNER_CLERK2"

	cutscenes := []*CutsceneScript{
		{
			ScriptLabel:      "GameCornerClerk2NoCoinCase",
			MapName:          "GAME_CORNER",
			TriggerType:      "npc_click",
			TriggerLabel:     &trigger,
			RequiresFlagAbst: &flag,
		},
		{
			ScriptLabel:      "GameCornerClerk2CoinCaseFull",
			MapName:          "GAME_CORNER",
			TriggerType:      "npc_click",
			TriggerLabel:     &trigger,
			RequiresFlagAbst: &flag,
			RequiresItemID:   &coinCaseItemID,
			RequiresCoins:    &fullCoinCase,
		},
	}

	sortCutscenesBySpecificity(cutscenes)

	if cutscenes[0].ScriptLabel != "GameCornerClerk2CoinCaseFull" {
		t.Fatalf("first cutscene = %s, want coin-gated full branch", cutscenes[0].ScriptLabel)
	}
}

func TestCheckEligibleRequiresPlayerFacing(t *testing.T) {
	requiredFacing := "UP"
	cs := &CutsceneScript{
		ScriptLabel:          "Route11Gate2FLeftBinocularsSnorlax",
		MapName:              "ROUTE_11_GATE_2F",
		TriggerType:          "npc_click",
		RequiresPlayerFacing: &requiredFacing,
	}
	manager := NewCutsceneManager(nil)
	flags := NewEventFlagManager(nil)

	if !manager.CheckEligible(cs, 1, flags, "UP") {
		t.Fatalf("expected script to be eligible when facing UP")
	}
	if manager.CheckEligible(cs, 1, flags, "DOWN") {
		t.Fatalf("expected script to be ineligible when facing DOWN")
	}
	if manager.CheckEligible(cs, 1, flags) {
		t.Fatalf("expected script to be ineligible without facing context")
	}
}

func TestCheckEligibleRequiresAllFlagArrayConditions(t *testing.T) {
	cs := &CutsceneScript{
		ScriptLabel:       "ViridianCityGamblerGymClosed",
		MapName:           "VIRIDIAN_CITY",
		TriggerType:       "npc_click",
		RequiresFlags:     []string{"EVENT_GOT_POKEDEX"},
		RequiresFlagsAbst: []string{"EVENT_BEAT_VIRIDIAN_GYM_GIOVANNI", "EVENT_GOT_EARTHBADGE"},
	}
	manager := NewCutsceneManager(nil)
	flags := NewEventFlagManager(nil)
	flags.flags[1] = map[string]bool{"EVENT_GOT_POKEDEX": true}

	if !manager.CheckEligible(cs, 1, flags) {
		t.Fatalf("expected script to be eligible with required flags present and absent flags missing")
	}

	flags.flags[1]["EVENT_GOT_EARTHBADGE"] = true
	if manager.CheckEligible(cs, 1, flags) {
		t.Fatalf("expected script to be ineligible when an absent-array flag is set")
	}

	delete(flags.flags[1], "EVENT_GOT_EARTHBADGE")
	delete(flags.flags[1], "EVENT_GOT_POKEDEX")
	if manager.CheckEligible(cs, 1, flags) {
		t.Fatalf("expected script to be ineligible when a required-array flag is missing")
	}
}
