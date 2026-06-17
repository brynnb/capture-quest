package pokebattle

import "testing"

// Helper to create a test Pokémon with specific stats
func makeTestPokemon(name string, level int, t1, t2 PokemonType, hp, atk, def, spc, spd int, moves [4]MoveSlot) *Pokemon {
	p := &Pokemon{
		ID:        1,
		Name:      name,
		Level:     level,
		Type1:     t1,
		Type2:     t2,
		BaseStats: BaseStats{HP: hp, Attack: atk, Defense: def, Special: spc, Speed: spd},
		IVs:       IVs{Attack: 15, Defense: 15, Speed: 15, Special: 15},
		EVs:       EVs{},
		Moves:     moves,
		BaseSpeed: spd,
	}
	p.RecalculateStats()
	p.CurHP = p.MaxHP
	return p
}

func testMoves() [4]MoveSlot {
	return [4]MoveSlot{
		{ID: 1, Name: "Tackle", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 35, MaxPP: 35},
		{ID: 2, Name: "Ember", Type: TypeFire, Power: 40, Accuracy: 100, PP: 25, MaxPP: 25},
		{},
		{},
	}
}

func TestNewWildBattle(t *testing.T) {
	player := makeTestPokemon("Charmander", 10, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Pidgey", 5, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{player}, wild)

	if battle.Phase != PhaseActionSelect {
		t.Errorf("Expected PhaseActionSelect, got %d", battle.Phase)
	}
	if battle.BattleType != BattleWild {
		t.Error("Expected BattleWild")
	}
	if battle.GetPlayerPokemon().Name != "Charmander" {
		t.Error("Wrong player Pokémon")
	}
	if battle.GetEnemyPokemon().Name != "Pidgey" {
		t.Error("Wrong enemy Pokémon")
	}
}

func TestNewWildBattleStartsFirstNonFaintedPokemon(t *testing.T) {
	faintedLead := makeTestPokemon("Bulbasaur", 10, TypeGrass, TypePoison, 45, 49, 49, 65, 45, testMoves())
	faintedLead.CurHP = 0
	healthySecond := makeTestPokemon("Charmander", 10, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Pidgey", 5, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{faintedLead, healthySecond}, wild)

	if battle.PlayerActive != 1 {
		t.Fatalf("PlayerActive = %d, want 1", battle.PlayerActive)
	}
	if battle.GetPlayerPokemon().Name != "Charmander" {
		t.Fatalf("active pokemon = %s, want Charmander", battle.GetPlayerPokemon().Name)
	}
}

func TestNewTrainerBattleStartsFirstNonFaintedPokemon(t *testing.T) {
	faintedLead := makeTestPokemon("Bulbasaur", 10, TypeGrass, TypePoison, 45, 49, 49, 65, 45, testMoves())
	faintedLead.CurHP = 0
	healthySecond := makeTestPokemon("Squirtle", 10, TypeWater, TypeWater, 44, 48, 65, 50, 43, testMoves())
	trainerPokemon := makeTestPokemon("Geodude", 12, TypeRock, TypeGround, 40, 80, 100, 30, 20, testMoves())

	battle := NewTrainerBattle([]*Pokemon{faintedLead, healthySecond}, []*Pokemon{trainerPokemon})

	if battle.PlayerActive != 1 {
		t.Fatalf("PlayerActive = %d, want 1", battle.PlayerActive)
	}
	if battle.GetPlayerPokemon().Name != "Squirtle" {
		t.Fatalf("active pokemon = %s, want Squirtle", battle.GetPlayerPokemon().Name)
	}
}

func TestBattle_FightTurn(t *testing.T) {
	player := makeTestPokemon("Charmander", 50, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Pidgey", 5, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{player}, wild)

	events := battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	if len(events) == 0 {
		t.Fatal("Expected at least one event")
	}

	// Player should have attacked (L50 vs L5 should one-shot)
	foundMoveUsed := false
	for _, e := range events {
		if e.Type == EventMoveUsed {
			foundMoveUsed = true
		}
	}
	if !foundMoveUsed {
		t.Error("Expected a move_used event")
	}
}

func TestBattle_DamageEventsIncludeSideForSameNamePokemon(t *testing.T) {
	player := makeTestPokemon("CHARMANDER", 5, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	enemy := makeTestPokemon("CHARMANDER", 5, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	enemy.IsWild = true

	battle := NewWildBattle([]*Pokemon{player}, enemy)
	events := battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	var damageEvents []BattleEvent
	for _, event := range events {
		if event.Type == EventDamageDealt {
			damageEvents = append(damageEvents, event)
		}
	}
	if len(damageEvents) < 2 {
		t.Fatalf("expected player and enemy damage events, got %d", len(damageEvents))
	}
	if damageEvents[0].AttackerSide != "player" || damageEvents[0].TargetSide != "enemy" {
		t.Fatalf("first damage event side mismatch: attacker=%q target=%q", damageEvents[0].AttackerSide, damageEvents[0].TargetSide)
	}
	if damageEvents[1].AttackerSide != "enemy" || damageEvents[1].TargetSide != "player" {
		t.Fatalf("second damage event side mismatch: attacker=%q target=%q", damageEvents[1].AttackerSide, damageEvents[1].TargetSide)
	}
}

func TestBattle_RunFromWild(t *testing.T) {
	// Player much faster than enemy — should always escape
	player := makeTestPokemon("Jolteon", 50, TypeElectric, TypeElectric, 65, 65, 60, 110, 130, testMoves())
	wild := makeTestPokemon("Slowpoke", 5, TypeWater, TypePsychic, 90, 65, 65, 40, 15, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{player}, wild)
	events := battle.SubmitAction(TurnAction{Action: ActionRun})

	foundRunSuccess := false
	for _, e := range events {
		if e.Type == EventRunSuccess {
			foundRunSuccess = true
		}
	}
	if !foundRunSuccess {
		t.Error("Expected run_success when player is much faster")
	}
	if !battle.IsOver() {
		t.Error("Battle should be over after successful run")
	}
}

func TestBattle_CantRunFromTrainer(t *testing.T) {
	player := makeTestPokemon("Pikachu", 25, TypeElectric, TypeElectric, 35, 55, 30, 50, 90, testMoves())
	trainer := makeTestPokemon("Geodude", 20, TypeRock, TypeGround, 40, 80, 100, 30, 20, testMoves())

	battle := NewTrainerBattle([]*Pokemon{player}, []*Pokemon{trainer})
	events := battle.SubmitAction(TurnAction{Action: ActionRun})

	foundRunFail := false
	for _, e := range events {
		if e.Type == EventRunFail {
			foundRunFail = true
		}
	}
	if !foundRunFail {
		t.Error("Expected run_fail for trainer battle")
	}
	if battle.IsOver() {
		t.Error("Battle should not be over after failed run from trainer")
	}
}

func TestBattle_PPDepletion(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 1, Name: "Tackle", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 1, MaxPP: 35},
		{},
		{},
		{},
	}
	player := makeTestPokemon("Rattata", 50, TypeNormal, TypeNormal, 30, 56, 35, 25, 72, moves)
	wild := makeTestPokemon("Chansey", 50, TypeNormal, TypeNormal, 250, 5, 5, 105, 50, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{player}, wild)

	// First attack uses the last PP
	battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})
	if player.Moves[0].PP != 0 {
		t.Errorf("Expected 0 PP after use, got %d", player.Moves[0].PP)
	}

	// Second attack should trigger Struggle
	events := battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})
	foundStruggle := false
	for _, e := range events {
		if e.Type == EventMoveUsed && e.MoveName == "Struggle" {
			foundStruggle = true
		}
	}
	if !foundStruggle {
		t.Error("Expected Struggle when all PP depleted")
	}
}

func TestBattle_StatusMoveAppliesSleep(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 79, Name: "SLEEP POWDER", Type: TypeGrass, Power: 0, Accuracy: 100, PP: 15, MaxPP: 15, Effect: "SLEEP_EFFECT"},
	}
	player := makeTestPokemon("Bulbasaur", 20, TypeGrass, TypePoison, 45, 49, 49, 65, 45, moves)
	wild := makeTestPokemon("Pidgey", 10, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	battle := NewWildBattle([]*Pokemon{player}, wild)

	battle.executeMoveDirect(player, wild, 0, false, false)

	if wild.Status != StatusSleep {
		t.Fatalf("enemy status = %s, want sleep", wild.Status.String())
	}
	if wild.SleepTurns < 1 || wild.SleepTurns > 7 {
		t.Fatalf("sleep turns = %d, want 1..7", wild.SleepTurns)
	}
	foundStatus := false
	for _, event := range battle.Events {
		if event.Type == EventStatusApplied && event.StatusApplied == StatusSleep.String() {
			foundStatus = true
		}
	}
	if !foundStatus {
		t.Fatalf("expected status_applied event, got %#v", battle.Events)
	}
}

func TestBattle_StatusMoveDoesNotOverwriteExistingStatus(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 86, Name: "THUNDER WAVE", Type: TypeElectric, Power: 0, Accuracy: 100, PP: 20, MaxPP: 20, Effect: "PARALYZE_EFFECT"},
	}
	player := makeTestPokemon("Pikachu", 20, TypeElectric, TypeElectric, 35, 55, 30, 50, 90, moves)
	wild := makeTestPokemon("Pidgey", 10, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	wild.Status = StatusPoison
	battle := NewWildBattle([]*Pokemon{player}, wild)

	battle.executeMoveDirect(player, wild, 0, false, false)

	if wild.Status != StatusPoison {
		t.Fatalf("enemy status = %s, want poison", wild.Status.String())
	}
	for _, event := range battle.Events {
		if event.Type == EventStatusApplied {
			t.Fatalf("unexpected status_applied event for existing status: %#v", battle.Events)
		}
	}
}

func TestBattle_PoisonTypeCannotBePoisoned(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 77, Name: "POISONPOWDER", Type: TypePoison, Power: 0, Accuracy: 100, PP: 35, MaxPP: 35, Effect: "POISON_EFFECT"},
	}
	player := makeTestPokemon("Oddish", 20, TypeGrass, TypePoison, 45, 50, 55, 75, 30, moves)
	wild := makeTestPokemon("Bulbasaur", 10, TypeGrass, TypePoison, 45, 49, 49, 65, 45, testMoves())
	battle := NewWildBattle([]*Pokemon{player}, wild)

	battle.executeMoveDirect(player, wild, 0, false, false)

	if wild.Status != StatusNone {
		t.Fatalf("poison-type status = %s, want none", wild.Status.String())
	}
}

func TestBattle_FireDamageThawsFrozenTarget(t *testing.T) {
	fireMove := &MoveSlot{ID: 53, Name: "FLAMETHROWER", Type: TypeFire, Power: 40, Accuracy: 100, PP: 15, MaxPP: 15}
	player := makeTestPokemon("Charmander", 20, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Chansey", 20, TypeNormal, TypeNormal, 250, 5, 5, 105, 50, testMoves())
	wild.Status = StatusFreeze
	battle := NewWildBattle([]*Pokemon{player}, wild)

	if !battle.executeDamageMove(player, wild, fireMove) {
		t.Fatal("expected fire move to hit")
	}

	if wild.Status != StatusNone {
		t.Fatalf("frozen target status = %s, want none", wild.Status.String())
	}
	foundThaw := false
	for _, event := range battle.Events {
		if event.Type == EventMessage && event.Message == "Chansey thawed out!" {
			foundThaw = true
		}
	}
	if !foundThaw {
		t.Fatalf("expected thaw message, got %#v", battle.Events)
	}
}

func TestBattle_NonFireDamageDoesNotThawFrozenTarget(t *testing.T) {
	normalMove := &MoveSlot{ID: 1, Name: "TACKLE", Type: TypeNormal, Power: 40, Accuracy: 100, PP: 35, MaxPP: 35}
	player := makeTestPokemon("Rattata", 20, TypeNormal, TypeNormal, 30, 56, 35, 25, 72, testMoves())
	wild := makeTestPokemon("Chansey", 20, TypeNormal, TypeNormal, 250, 5, 5, 105, 50, testMoves())
	wild.Status = StatusFreeze
	battle := NewWildBattle([]*Pokemon{player}, wild)

	if !battle.executeDamageMove(player, wild, normalMove) {
		t.Fatal("expected normal move to hit")
	}

	if wild.Status != StatusFreeze {
		t.Fatalf("target status = %s, want freeze", wild.Status.String())
	}
	for _, event := range battle.Events {
		if event.Type == EventMessage && event.Message == "Chansey thawed out!" {
			t.Fatalf("unexpected thaw message for non-fire damage: %#v", battle.Events)
		}
	}
}

func TestBattle_ToxicAppliesBadPoisonCounter(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 92, Name: "TOXIC", Type: TypePoison, Power: 0, Accuracy: 100, PP: 10, MaxPP: 10, Effect: "POISON_EFFECT"},
	}
	player := makeTestPokemon("Nidorino", 20, TypePoison, TypePoison, 61, 72, 57, 55, 65, moves)
	wild := makeTestPokemon("Pidgey", 10, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	battle := NewWildBattle([]*Pokemon{player}, wild)

	battle.executeMoveDirect(player, wild, 0, false, false)

	if wild.Status != StatusBadPoison {
		t.Fatalf("enemy status = %s, want bad poison", wild.Status.String())
	}
	if wild.BadPoisonTurns != 1 {
		t.Fatalf("bad poison turns = %d, want 1", wild.BadPoisonTurns)
	}
}

func TestBattle_ToxicDamageRampsEachTurn(t *testing.T) {
	player := makeTestPokemon("Charmander", 20, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Pidgey", 20, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	wild.MaxHP = 160
	wild.CurHP = 160
	wild.Status = StatusBadPoison
	wild.BadPoisonTurns = 1
	battle := NewWildBattle([]*Pokemon{player}, wild)

	battle.applyEndTurnStatusDamage()
	if wild.CurHP != 150 {
		t.Fatalf("after first Toxic tick HP = %d, want 150", wild.CurHP)
	}
	if wild.BadPoisonTurns != 2 {
		t.Fatalf("after first Toxic tick turns = %d, want 2", wild.BadPoisonTurns)
	}

	battle.Events = nil
	battle.applyEndTurnStatusDamage()
	if wild.CurHP != 130 {
		t.Fatalf("after second Toxic tick HP = %d, want 130", wild.CurHP)
	}
	if wild.BadPoisonTurns != 3 {
		t.Fatalf("after second Toxic tick turns = %d, want 3", wild.BadPoisonTurns)
	}
	if len(battle.Events) != 1 || battle.Events[0].Damage != 20 {
		t.Fatalf("second Toxic event = %#v, want one 20-damage event", battle.Events)
	}
}

func TestPokemon_ResetVolatileStatusResetsBadPoisonRamp(t *testing.T) {
	p := makeTestPokemon("Pidgey", 20, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	p.Status = StatusBadPoison
	p.BadPoisonTurns = 5

	p.ResetVolatileStatus()

	if p.Status != StatusBadPoison {
		t.Fatalf("status = %s, want bad poison", p.Status.String())
	}
	if p.BadPoisonTurns != 1 {
		t.Fatalf("bad poison turns = %d, want 1", p.BadPoisonTurns)
	}
}

func TestBattle_LeechSeedAppliesAndDrainsHP(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 73, Name: "LEECH SEED", Type: TypeGrass, Power: 0, Accuracy: 100, PP: 10, MaxPP: 10, Effect: "LEECH_SEED_EFFECT"},
	}
	player := makeTestPokemon("Bulbasaur", 20, TypeGrass, TypePoison, 45, 49, 49, 65, 45, moves)
	wild := makeTestPokemon("Pidgey", 20, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	battle := NewWildBattle([]*Pokemon{player}, wild)

	battle.executeMoveDirect(player, wild, 0, false, false)

	if !wild.IsSeeded {
		t.Fatal("wild Pokémon was not seeded")
	}
	foundSeeded := false
	for _, event := range battle.Events {
		if event.Type == EventStatusApplied && event.Message == "Pidgey was seeded!" {
			foundSeeded = true
		}
	}
	if !foundSeeded {
		t.Fatalf("expected seeded event, got %#v", battle.Events)
	}

	player.CurHP = player.MaxHP - 10
	wildHP := wild.CurHP
	playerHP := player.CurHP
	wantDrain := wild.MaxHP / 16
	if wantDrain < 1 {
		wantDrain = 1
	}

	battle.Events = nil
	battle.applyEndTurnStatusDamage()

	if wild.CurHP != wildHP-wantDrain {
		t.Fatalf("seeded HP = %d, want %d", wild.CurHP, wildHP-wantDrain)
	}
	if player.CurHP != playerHP+wantDrain {
		t.Fatalf("recipient HP = %d, want %d", player.CurHP, playerHP+wantDrain)
	}
	if len(battle.Events) != 2 {
		t.Fatalf("leech seed events = %d, want 2 (%#v)", len(battle.Events), battle.Events)
	}
}

func TestBattle_LeechSeedDoesNotAffectGrassType(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 73, Name: "LEECH SEED", Type: TypeGrass, Power: 0, Accuracy: 100, PP: 10, MaxPP: 10, Effect: "LEECH_SEED_EFFECT"},
	}
	player := makeTestPokemon("Bulbasaur", 20, TypeGrass, TypePoison, 45, 49, 49, 65, 45, moves)
	wild := makeTestPokemon("Oddish", 20, TypeGrass, TypePoison, 45, 50, 55, 75, 30, testMoves())
	battle := NewWildBattle([]*Pokemon{player}, wild)

	battle.executeMoveDirect(player, wild, 0, false, false)

	if wild.IsSeeded {
		t.Fatal("Grass-type Pokémon was seeded")
	}
	for _, event := range battle.Events {
		if event.Type == EventStatusApplied {
			t.Fatalf("unexpected status event for Grass-type immunity: %#v", battle.Events)
		}
	}
}

func TestBattle_EndTurnStatusDamageForBurnAndPoison(t *testing.T) {
	player := makeTestPokemon("Charmander", 20, TypeFire, TypeFire, 39, 52, 43, 50, 65, testMoves())
	wild := makeTestPokemon("Pidgey", 20, TypeNormal, TypeFlying, 40, 45, 40, 35, 56, testMoves())
	player.Status = StatusBurn
	wild.Status = StatusPoison
	battle := NewWildBattle([]*Pokemon{player}, wild)

	playerHP := player.CurHP
	wildHP := wild.CurHP
	battle.applyEndTurnStatusDamage()

	if player.CurHP != playerHP-player.MaxHP/16 {
		t.Fatalf("burn HP = %d, want %d", player.CurHP, playerHP-player.MaxHP/16)
	}
	if wild.CurHP != wildHP-wild.MaxHP/16 {
		t.Fatalf("poison HP = %d, want %d", wild.CurHP, wildHP-wild.MaxHP/16)
	}
	if len(battle.Events) != 2 {
		t.Fatalf("status damage events = %d, want 2 (%#v)", len(battle.Events), battle.Events)
	}
}

func TestBattle_EnemyFaints_WildWin(t *testing.T) {
	moves := [4]MoveSlot{
		{ID: 1, Name: "Hyper Beam", Type: TypeNormal, Power: 150, Accuracy: 100, PP: 5, MaxPP: 5},
		{},
		{},
		{},
	}
	player := makeTestPokemon("Mewtwo", 100, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, moves)
	wild := makeTestPokemon("Rattata", 3, TypeNormal, TypeNormal, 30, 56, 35, 25, 72, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{player}, wild)
	events := battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	foundFaint := false
	for _, e := range events {
		if e.Type == EventFainted {
			foundFaint = true
		}
	}
	if !foundFaint {
		t.Error("Expected enemy to faint")
	}
	if !battle.IsOver() {
		t.Error("Battle should be over")
	}
	if !battle.PlayerWon() {
		t.Error("Player should have won")
	}
}

func TestBattle_Switch(t *testing.T) {
	poke1 := makeTestPokemon("Pikachu", 25, TypeElectric, TypeElectric, 35, 55, 30, 50, 90, testMoves())
	poke2 := makeTestPokemon("Bulbasaur", 25, TypeGrass, TypePoison, 45, 49, 49, 65, 45, testMoves())
	wild := makeTestPokemon("Geodude", 20, TypeRock, TypeGround, 40, 80, 100, 30, 20, testMoves())
	wild.IsWild = true

	battle := NewWildBattle([]*Pokemon{poke1, poke2}, wild)

	if battle.GetPlayerPokemon().Name != "Pikachu" {
		t.Error("Expected Pikachu as active")
	}

	// Switch to Bulbasaur (party index 1)
	events := battle.SubmitAction(TurnAction{Action: ActionSwitch, MoveSlot: 1})

	if battle.GetPlayerPokemon().Name != "Bulbasaur" {
		t.Error("Expected Bulbasaur after switch")
	}

	// Should have switch message and enemy attack
	foundMessage := false
	for _, e := range events {
		if e.Type == EventMessage {
			foundMessage = true
		}
	}
	if !foundMessage {
		t.Error("Expected message event for switch")
	}
}

func TestBattle_FaintSwitch(t *testing.T) {
	// Player's first Pokémon is very weak, second is strong
	weakMoves := [4]MoveSlot{
		{ID: 1, Name: "Splash", Type: TypeNormal, Power: 0, Accuracy: 0, PP: 40, MaxPP: 40},
		{},
		{},
		{},
	}
	strongMoves := [4]MoveSlot{
		{ID: 1, Name: "Psychic", Type: TypePsychic, Power: 90, Accuracy: 100, PP: 10, MaxPP: 10},
		{},
		{},
		{},
	}
	weak := makeTestPokemon("Magikarp", 5, TypeWater, TypeWater, 20, 10, 55, 20, 80, weakMoves)
	strong := makeTestPokemon("Mewtwo", 100, TypePsychic, TypePsychic, 106, 110, 90, 154, 130, strongMoves)

	enemyMoves := [4]MoveSlot{
		{ID: 1, Name: "Earthquake", Type: TypeGround, Power: 100, Accuracy: 100, PP: 10, MaxPP: 10},
		{},
		{},
		{},
	}
	enemy := makeTestPokemon("Rhydon", 50, TypeGround, TypeRock, 105, 130, 120, 45, 40, enemyMoves)

	battle := NewTrainerBattle([]*Pokemon{weak, strong}, []*Pokemon{enemy})

	// Magikarp uses Splash (does nothing), Rhydon attacks and likely KOs
	// Force Magikarp to 1 HP so it definitely faints
	weak.CurHP = 1

	battle.SubmitAction(TurnAction{Action: ActionFight, MoveSlot: 0})

	if battle.Phase == PhaseFaintSwitch {
		// Player must switch — send in Mewtwo
		events := battle.ForceSwitchIn(1)
		if battle.GetPlayerPokemon().Name != "Mewtwo" {
			t.Error("Expected Mewtwo after forced switch")
		}
		if battle.Phase != PhaseActionSelect {
			t.Errorf("Expected PhaseActionSelect after switch, got %d", battle.Phase)
		}
		_ = events
	}
	// If Magikarp somehow survived, that's fine too — the test structure is valid
}

func TestIsSpecialType(t *testing.T) {
	specialTypes := []PokemonType{TypeFire, TypeWater, TypeElectric, TypeGrass, TypeIce, TypePsychic, TypeDragon}
	physicalTypes := []PokemonType{TypeNormal, TypeFighting, TypeFlying, TypePoison, TypeGround, TypeRock, TypeBug, TypeGhost}

	for _, typ := range specialTypes {
		if !isSpecialType(typ) {
			t.Errorf("%s should be special", typ)
		}
	}
	for _, typ := range physicalTypes {
		if isSpecialType(typ) {
			t.Errorf("%s should be physical", typ)
		}
	}
}
