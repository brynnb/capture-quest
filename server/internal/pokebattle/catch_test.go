package pokebattle

import (
	"testing"
)

func TestCatch_MasterBall(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Mewtwo", 70, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, testMoves())
	wild.IsWild = true
	wild.CatchRate = 3 // Mewtwo has very low catch rate

	battle := NewWildBattle([]*Pokemon{player}, wild)

	events := battle.SubmitAction(TurnAction{Action: ActionItem, BallModifier: 255.0})

	// Master Ball always catches
	hasCatchSuccess := false
	for _, e := range events {
		if e.Type == EventCatchSuccess {
			hasCatchSuccess = true
		}
	}
	if !hasCatchSuccess {
		t.Error("Master Ball should always catch — no catch_success event found")
	}
	if !battle.PlayerCaught {
		t.Error("PlayerCaught should be true after Master Ball")
	}
	if battle.Phase != PhaseBattleEnd {
		t.Errorf("Expected PhaseBattleEnd, got %d", battle.Phase)
	}
}

func TestCatch_GuaranteedCatchPokeBall(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Mewtwo", 70, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, testMoves())
	wild.IsWild = true
	wild.CatchRate = 3

	battle := NewWildBattle([]*Pokemon{player}, wild)
	battle.GuaranteedCatch = true

	events := battle.SubmitAction(TurnAction{Action: ActionItem, BallModifier: 1.0})

	if !battle.PlayerCaught {
		t.Fatal("PlayerCaught should be true after guaranteed catch")
	}
	if battle.Phase != PhaseBattleEnd {
		t.Fatalf("phase = %d, want PhaseBattleEnd", battle.Phase)
	}
	if !eventsContainType(events, EventCatchSuccess) {
		t.Fatalf("events = %#v, want catch_success", events)
	}
}

func TestCatch_ItemsOnlyBattleRejectsFight(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Weedle", 5, TypeBug, TypePoison, 40, 35, 30, 20, 50, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{player}, wild)
	battle.AllowedActions = []string{"item"}

	events := battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	if battle.Phase != PhaseActionSelect {
		t.Fatalf("phase = %d, want action select", battle.Phase)
	}
	if battle.TurnNumber != 0 {
		t.Fatalf("turn number = %d, want 0", battle.TurnNumber)
	}
	if len(events) != 1 || events[0].Type != EventMessage {
		t.Fatalf("events = %#v, want one message", events)
	}
}

func TestCatch_TrainerBattle_Blocked(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	enemy := makeTestPokemon("Pidgey", 5, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())

	battle := NewTrainerBattle([]*Pokemon{player}, []*Pokemon{enemy})
	battle.Trainer = &TrainerMeta{
		ClassName: "BUG_CATCHER",
		Name:      "Bug Catcher Rick",
	}

	events := battle.SubmitAction(TurnAction{Action: ActionItem, BallModifier: 255.0})

	// Should be blocked with a message
	hasBlockMsg := false
	for _, e := range events {
		if e.Type == EventMessage && e.Message != "" {
			hasBlockMsg = true
		}
	}
	if !hasBlockMsg {
		t.Error("Expected block message when throwing ball at trainer Pokémon")
	}
	if battle.PlayerCaught {
		t.Error("PlayerCaught should be false for trainer battles")
	}
	if battle.Phase == PhaseBattleEnd {
		t.Error("Battle should not end from a blocked catch attempt")
	}
}

func eventsContainType(events []BattleEvent, eventType BattleEventType) bool {
	for _, event := range events {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func TestCatch_PokeBall_LowHP_HighCatchRate(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Magikarp", 5, TypeWater, TypeWater, 20, 10, 55, 15, 20, testMoves())
	wild.IsWild = true
	wild.CatchRate = 255 // Magikarp has max catch rate

	// Run many attempts — with catch rate 255 and 1 HP, should catch most of the time
	caught := 0
	attempts := 100
	for i := 0; i < attempts; i++ {
		// Reset battle state for each attempt
		b := NewWildBattle([]*Pokemon{player}, wild)
		b.GetEnemyPokemon().CurHP = 1
		events := b.SubmitAction(TurnAction{Action: ActionItem, BallModifier: 1.0})

		for _, e := range events {
			if e.Type == EventCatchSuccess {
				caught++
				break
			}
		}
	}

	// With catch rate 255 and 1 HP, should catch at least 70% of the time
	if caught < 70 {
		t.Errorf("Expected at least 70%% catch rate for Magikarp at 1 HP, got %d/%d", caught, attempts)
	}
}

func TestCatch_PokeBall_FullHP_LowCatchRate(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Mewtwo", 70, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, testMoves())
	wild.IsWild = true
	wild.CatchRate = 3 // Mewtwo: very hard to catch

	// Run many attempts — with catch rate 3 and full HP, should rarely catch
	caught := 0
	attempts := 100
	for i := 0; i < attempts; i++ {
		b := NewWildBattle([]*Pokemon{player}, wild)
		events := b.SubmitAction(TurnAction{Action: ActionItem, BallModifier: 1.0})

		for _, e := range events {
			if e.Type == EventCatchSuccess {
				caught++
				break
			}
		}
	}

	// Mewtwo at full HP with Poké Ball should be very hard — less than 20%
	if caught > 20 {
		t.Errorf("Expected low catch rate for Mewtwo at full HP, got %d/%d", caught, attempts)
	}
}

func TestCatch_FailedCatch_EnemyAttacks(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Mewtwo", 70, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, testMoves())
	wild.IsWild = true
	wild.CatchRate = 3

	// Run many attempts until we get a failed catch, then check enemy attacked
	for i := 0; i < 200; i++ {
		b := NewWildBattle([]*Pokemon{player}, wild)
		events := b.SubmitAction(TurnAction{Action: ActionItem, BallModifier: 1.0})

		hasCatchFail := false
		hasMoveUsed := false
		for _, e := range events {
			if e.Type == EventCatchFail {
				hasCatchFail = true
			}
			if e.Type == EventMoveUsed && e.AttackerName == "Mewtwo" {
				hasMoveUsed = true
			}
		}

		if hasCatchFail {
			if !hasMoveUsed {
				t.Error("Enemy should attack after a failed catch attempt")
			}
			return // Test passed
		}
	}
	t.Skip("Could not trigger a failed catch in 200 attempts (unlikely but possible)")
}

func TestCatch_ShakesInEvents(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Pidgey", 5, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	wild.IsWild = true
	wild.CatchRate = 255

	// Run attempts and verify catch_attempt events have shakes field
	for i := 0; i < 50; i++ {
		b := NewWildBattle([]*Pokemon{player}, wild)
		events := b.SubmitAction(TurnAction{Action: ActionItem, BallModifier: 1.0})

		for _, e := range events {
			if e.Type == EventCatchAttempt {
				if e.Shakes < 0 || e.Shakes > 3 {
					t.Errorf("Shakes should be 0-3, got %d", e.Shakes)
				}
				return // Found a catch attempt event with valid shakes
			}
		}
	}
	t.Error("No catch_attempt event found in 50 attempts")
}
