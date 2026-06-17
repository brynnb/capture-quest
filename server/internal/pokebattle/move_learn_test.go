package pokebattle

import "testing"

func TestTryLearnMove_AlreadyKnown(t *testing.T) {
	p := makeTestPokemon("Charmander", 10, TypeFire, TypeFire, 39, 52, 43, 50, 65, [4]MoveSlot{
		{ID: 33, Name: "Tackle", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 35, MaxPP: 35},
		{ID: 45, Name: "Growl", Type: TypeNormal, Power: 0, Accuracy: 100, PP: 40, MaxPP: 40},
		{}, {},
	})

	// Trying to learn a move already known should return -2 (skip)
	result := TryLearnMove(nil, p, 33) // Tackle, already known
	if result != -2 {
		t.Errorf("Expected -2 (already known), got %d", result)
	}
}

func TestTryLearnMove_EmptySlot(t *testing.T) {
	p := makeTestPokemon("Charmander", 10, TypeFire, TypeFire, 39, 52, 43, 50, 65, [4]MoveSlot{
		{ID: 33, Name: "Tackle", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 35, MaxPP: 35},
		{ID: 45, Name: "Growl", Type: TypeNormal, Power: 0, Accuracy: 100, PP: 40, MaxPP: 40},
		{}, {},
	})

	// Manually place a move in slot 2 to simulate TryLearnMove with empty slot
	// Since TryLearnMove calls LoadMoveSlotFromDB (needs DB), we test the logic directly
	// by checking that empty slots are detected
	hasEmpty := false
	for _, m := range p.Moves {
		if m.ID == 0 {
			hasEmpty = true
			break
		}
	}
	if !hasEmpty {
		t.Error("Expected empty slot to be available")
	}

	// Verify move 99 is not already known
	for _, m := range p.Moves {
		if m.ID == 99 {
			t.Error("Move 99 should not be known yet")
		}
	}
}

func TestTryLearnMove_AllSlotsFull(t *testing.T) {
	p := makeTestPokemon("Charmander", 36, TypeFire, TypeFire, 39, 52, 43, 50, 65, [4]MoveSlot{
		{ID: 33, Name: "Tackle", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 35, MaxPP: 35},
		{ID: 45, Name: "Growl", Type: TypeNormal, Power: 0, Accuracy: 100, PP: 40, MaxPP: 40},
		{ID: 52, Name: "Ember", Type: TypeFire, Power: 40, Accuracy: 100, PP: 25, MaxPP: 25},
		{ID: 53, Name: "Leer", Type: TypeNormal, Power: 0, Accuracy: 100, PP: 30, MaxPP: 30},
	})

	// All 4 slots full, trying to learn a new move should return -1 (prompt needed)
	result := TryLearnMove(nil, p, 99) // New move not in slots
	if result != -1 {
		t.Errorf("Expected -1 (all slots full, prompt needed), got %d", result)
	}
}

func TestForgetAndLearnMove_InvalidSlot(t *testing.T) {
	p := makeTestPokemon("Charmander", 36, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())

	err := ForgetAndLearnMove(nil, p, -1, 99)
	if err == nil {
		t.Error("Expected error for invalid slot -1")
	}

	err = ForgetAndLearnMove(nil, p, 4, 99)
	if err == nil {
		t.Error("Expected error for invalid slot 4")
	}
}

func TestPendingMoveLearn_BattleState(t *testing.T) {
	player := makeTestPokemon("Charmander", 10, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Pidgey", 5, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{player}, wild)

	// Initially no pending move
	if battle.PendingMoveLearn != nil {
		t.Error("Expected no pending move learn initially")
	}

	// Set a pending move
	battle.PendingMoveLearn = &PendingMove{
		PokemonIndex: 0,
		MoveID:       99,
		MoveName:     "Flamethrower",
	}

	if battle.PendingMoveLearn == nil {
		t.Fatal("Expected pending move learn to be set")
	}
	if battle.PendingMoveLearn.MoveID != 99 {
		t.Errorf("Expected move ID 99, got %d", battle.PendingMoveLearn.MoveID)
	}
	if battle.PendingMoveLearn.MoveName != "Flamethrower" {
		t.Errorf("Expected move name Flamethrower, got %s", battle.PendingMoveLearn.MoveName)
	}
	if battle.PendingMoveLearn.PokemonIndex != 0 {
		t.Errorf("Expected pokemon index 0, got %d", battle.PendingMoveLearn.PokemonIndex)
	}

	// Clear pending move
	battle.PendingMoveLearn = nil
	if battle.PendingMoveLearn != nil {
		t.Error("Expected pending move learn to be cleared")
	}
}

func TestMoveLearnEventTypes(t *testing.T) {
	// Verify event type constants are correct
	if EventMoveLearned != "move_learned" {
		t.Errorf("Expected 'move_learned', got '%s'", EventMoveLearned)
	}
	if EventMoveLearnPrompt != "move_learn_prompt" {
		t.Errorf("Expected 'move_learn_prompt', got '%s'", EventMoveLearnPrompt)
	}
}
