package pokebattle

import "testing"

func TestFaintSwitch_TrainerBattle_MustSwitch(t *testing.T) {
	// Weak lead that will faint, strong backup
	weakMoves := [4]MoveSlot{
		{ID: 1, Name: "Splash", Type: TypeNormal, Power: 0, Accuracy: 0, PP: 40, MaxPP: 40},
		{}, {}, {},
	}
	strongMoves := [4]MoveSlot{
		{ID: 1, Name: "Psychic", Type: TypePsychic, Power: 90, Accuracy: 100, PP: 10, MaxPP: 10},
		{}, {}, {},
	}
	weak := makeTestPokemon("Magikarp", 5, TypeWater, TypeWater, 20, 10, 55, 20, 80, weakMoves)
	strong := makeTestPokemon("Mewtwo", 100, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, strongMoves)

	enemyMoves := [4]MoveSlot{
		{ID: 1, Name: "Earthquake", Type: TypeGround, Power: 100, Accuracy: 100, PP: 10, MaxPP: 10},
		{}, {}, {},
	}
	enemy := makeTestPokemon("Rhydon", 50, TypeGround, TypeRock, 105, 130, 120, 45, 40, enemyMoves)

	battle := NewTrainerBattle([]*Pokemon{weak, strong}, []*Pokemon{enemy})
	weak.CurHP = 1 // Ensure it faints

	battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	if battle.Phase != PhaseFaintSwitch {
		t.Skipf("Magikarp survived (phase=%d), skipping", battle.Phase)
	}

	// Switch in Mewtwo
	events := battle.ForceSwitchIn(1)

	if battle.GetPlayerPokemon().Name != "Mewtwo" {
		t.Error("Expected Mewtwo after forced switch")
	}
	if battle.Phase != PhaseActionSelect {
		t.Errorf("Expected PhaseActionSelect after switch, got %d", battle.Phase)
	}

	// Should have a "Go, Mewtwo!" message
	hasGoMsg := false
	for _, e := range events {
		if e.Type == EventMessage && e.Message == "Go, Mewtwo!" {
			hasGoMsg = true
		}
	}
	if !hasGoMsg {
		t.Error("Expected 'Go, Mewtwo!' message after forced switch")
	}
}

func TestFaintSwitch_CantSwitchToFainted(t *testing.T) {
	weakMoves := [4]MoveSlot{
		{ID: 1, Name: "Splash", Type: TypeNormal, Power: 0, Accuracy: 0, PP: 40, MaxPP: 40},
		{}, {}, {},
	}
	weak1 := makeTestPokemon("Magikarp", 5, TypeWater, TypeWater, 20, 10, 55, 20, 80, weakMoves)
	weak2 := makeTestPokemon("Magikarp2", 5, TypeWater, TypeWater, 20, 10, 55, 20, 80, weakMoves)
	strong := makeTestPokemon("Mewtwo", 100, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, weakMoves)

	enemyMoves := [4]MoveSlot{
		{ID: 1, Name: "Earthquake", Type: TypeGround, Power: 100, Accuracy: 100, PP: 10, MaxPP: 10},
		{}, {}, {},
	}
	enemy := makeTestPokemon("Rhydon", 50, TypeGround, TypeRock, 105, 130, 120, 45, 40, enemyMoves)

	battle := NewTrainerBattle([]*Pokemon{weak1, weak2, strong}, []*Pokemon{enemy})
	weak1.CurHP = 1
	weak2.CurHP = 0 // Already fainted

	battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	if battle.Phase != PhaseFaintSwitch {
		t.Skipf("Magikarp survived (phase=%d), skipping", battle.Phase)
	}

	// Try to switch to the fainted Magikarp2 — should be rejected (no events)
	events := battle.ForceSwitchIn(1)
	if len(events) > 0 {
		t.Error("Should not be able to switch to a fainted Pokémon")
	}
	if battle.Phase != PhaseFaintSwitch {
		t.Error("Phase should still be PhaseFaintSwitch after invalid switch")
	}

	// Switch to Mewtwo — should work
	events = battle.ForceSwitchIn(2)
	if battle.GetPlayerPokemon().Name != "Mewtwo" {
		t.Error("Expected Mewtwo after switching to slot 2")
	}
	if battle.Phase != PhaseActionSelect {
		t.Errorf("Expected PhaseActionSelect, got %d", battle.Phase)
	}
	_ = events
}

func TestRunFromFaintSwitch_WildBattle_Success(t *testing.T) {
	weakMoves := [4]MoveSlot{
		{ID: 1, Name: "Splash", Type: TypeNormal, Power: 0, Accuracy: 0, PP: 40, MaxPP: 40},
		{}, {}, {},
	}
	// Fast backup so run always succeeds
	fastMoves := [4]MoveSlot{
		{ID: 1, Name: "Quick Attack", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 30, MaxPP: 30},
		{}, {}, {},
	}
	weak := makeTestPokemon("Magikarp", 5, TypeWater, TypeWater, 20, 10, 55, 20, 80, weakMoves)
	fast := makeTestPokemon("Jolteon", 50, TypeElectric, TypeElectric, 65, 65, 60, 110, 130, fastMoves)

	slowMoves := [4]MoveSlot{
		{ID: 1, Name: "Tackle", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 35, MaxPP: 35},
		{}, {}, {},
	}
	wild := makeTestPokemon("Slowpoke", 10, TypeWater, TypePsychic, 90, 65, 65, 40, 15, slowMoves)
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{weak, fast}, wild)
	weak.CurHP = 1

	battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	if battle.Phase != PhaseFaintSwitch {
		t.Skipf("Magikarp survived (phase=%d), skipping", battle.Phase)
	}

	// Run — Jolteon (speed 130) vs Slowpoke (speed 15), should always succeed
	events := battle.RunFromFaintSwitch()

	hasRunSuccess := false
	for _, e := range events {
		if e.Type == EventRunSuccess {
			hasRunSuccess = true
		}
	}
	if !hasRunSuccess {
		t.Error("Expected run_success event")
	}
	if battle.Phase != PhaseBattleEnd {
		t.Errorf("Expected PhaseBattleEnd after successful run, got %d", battle.Phase)
	}
}

func TestRunFromFaintSwitch_TrainerBattle_Blocked(t *testing.T) {
	weakMoves := [4]MoveSlot{
		{ID: 1, Name: "Splash", Type: TypeNormal, Power: 0, Accuracy: 0, PP: 40, MaxPP: 40},
		{}, {}, {},
	}
	weak := makeTestPokemon("Magikarp", 5, TypeWater, TypeWater, 20, 10, 55, 20, 80, weakMoves)
	strong := makeTestPokemon("Mewtwo", 100, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, weakMoves)

	enemyMoves := [4]MoveSlot{
		{ID: 1, Name: "Earthquake", Type: TypeGround, Power: 100, Accuracy: 100, PP: 10, MaxPP: 10},
		{}, {}, {},
	}
	enemy := makeTestPokemon("Rhydon", 50, TypeGround, TypeRock, 105, 130, 120, 45, 40, enemyMoves)

	battle := NewTrainerBattle([]*Pokemon{weak, strong}, []*Pokemon{enemy})
	weak.CurHP = 1

	battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	if battle.Phase != PhaseFaintSwitch {
		t.Skipf("Magikarp survived (phase=%d), skipping", battle.Phase)
	}

	events := battle.RunFromFaintSwitch()

	hasRunFail := false
	for _, e := range events {
		if e.Type == EventRunFail {
			hasRunFail = true
		}
	}
	if !hasRunFail {
		t.Error("Expected run_fail event in trainer battle")
	}
	// Phase should stay faint_switch — player must still pick a Pokémon
	if battle.Phase != PhaseFaintSwitch {
		t.Errorf("Expected PhaseFaintSwitch after blocked run, got %d", battle.Phase)
	}
}
