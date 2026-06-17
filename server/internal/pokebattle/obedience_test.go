package pokebattle

import "testing"

func TestShouldCheckObedienceOnlyForOutsiderOverCap(t *testing.T) {
	player := makeTestPokemon("MEWTWO", 70, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, testMoves())
	enemy := makeTestPokemon("PIDGEY", 5, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	battle := NewWildBattle([]*Pokemon{player}, enemy)
	battle.PlayerTrainerID = 1
	battle.PlayerObedienceLevel = 30

	player.OriginalTrainerID = 1
	if battle.shouldCheckObedience(player) {
		t.Fatal("owned Pokémon should always obey")
	}

	player.OriginalTrainerID = 2
	if !battle.shouldCheckObedience(player) {
		t.Fatal("outsider over the badge cap should check obedience")
	}

	player.Level = 30
	if battle.shouldCheckObedience(player) {
		t.Fatal("outsider at the badge cap should obey")
	}
}

func TestOriginalTrainerIDForSavePreservesOutsider(t *testing.T) {
	pokemon := &Pokemon{OriginalTrainerID: 22}
	if got := originalTrainerIDForSave(11, pokemon); got != 22 {
		t.Fatalf("originalTrainerIDForSave outsider = %d, want 22", got)
	}

	pokemon.OriginalTrainerID = 0
	if got := originalTrainerIDForSave(11, pokemon); got != 11 {
		t.Fatalf("originalTrainerIDForSave new owned = %d, want 11", got)
	}
}

func TestObedienceOutcomeForFinalRoll(t *testing.T) {
	const level = 70
	const badgeLevel = 30

	if got := obedienceOutcomeForFinalRoll(level, badgeLevel, 39); got != obedienceOutcomeSleep {
		t.Fatalf("roll 39 outcome = %v, want sleep", got)
	}
	if got := obedienceOutcomeForFinalRoll(level, badgeLevel, 40); got != obedienceOutcomeSelfHit {
		t.Fatalf("roll 40 outcome = %v, want self-hit", got)
	}
	if got := obedienceOutcomeForFinalRoll(level, badgeLevel, 79); got != obedienceOutcomeSelfHit {
		t.Fatalf("roll 79 outcome = %v, want self-hit", got)
	}
	if got := obedienceOutcomeForFinalRoll(level, badgeLevel, 80); got != obedienceOutcomeNoAction {
		t.Fatalf("roll 80 outcome = %v, want no action", got)
	}
}

func TestDisobedientRandomMoveSpendsCommandedPP(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 1, Name: "Tackle", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 35, MaxPP: 35},
		{ID: 2, Name: "Ember", Type: TypeFire, Power: 40, Accuracy: 100, PP: 0, MaxPP: 25},
		{}, {},
	}
	player := makeTestPokemon("CHARIZARD", 70, TypeFire, TypeFlying, 78, 84, 78, 85, 100, moves)
	enemy := makeTestPokemon("PIDGEY", 5, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	battle := NewWildBattle([]*Pokemon{player}, enemy)

	if !battle.applyDisobedienceOutcome(player, enemy, 0, obedienceOutcomeRandomMove) {
		t.Fatal("expected disobedience to consume the turn")
	}
	if player.Moves[0].PP != 34 {
		t.Fatalf("commanded move PP = %d, want 34", player.Moves[0].PP)
	}
	if player.Moves[1].PP != 0 {
		t.Fatalf("random move PP = %d, want unchanged 0", player.Moves[1].PP)
	}

	foundRandomMove := false
	for _, event := range battle.Events {
		if event.Type == EventMoveUsed && event.MoveName == "Ember" {
			foundRandomMove = true
			break
		}
	}
	if !foundRandomMove {
		t.Fatalf("events = %#v, want Ember move_used", battle.Events)
	}
}
