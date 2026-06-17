package pokebattle

import (
	"testing"
)

// Tests verify that enemy party, battle metadata, trainer meta, and pending move learn
// all round-trip correctly through JSON. Player party is NOT stored in JSON — it's
// reloaded from character_pokemon on restore.

func TestBattleStateRoundTrip_Wild(t *testing.T) {
	wild := &Pokemon{
		ID: 16, Name: "PIDGEY", Level: 3, IsWild: true,
		Type1: TypeNormal, Type2: TypeFlying,
		BaseStats: BaseStats{HP: 40, Attack: 45, Defense: 40, Special: 35, Speed: 56},
		IVs:       IVs{Attack: 7, Defense: 3, Special: 5, Speed: 9},
		EVs:       EVs{},
		MaxHP:     19, CurHP: 12, Attack: 12, Defense: 11, Special: 10, Speed: 14,
		Moves: [4]MoveSlot{
			{ID: 33, Name: "TACKLE", Type: TypeNormal, Power: 35, Accuracy: 95, PP: 30, MaxPP: 35},
			{ID: 28, Name: "SAND-ATTACK", Type: TypeNormal, Power: 0, Accuracy: 100, PP: 15, MaxPP: 15},
		},
		Status: StatusBadPoison, BadPoisonTurns: 3, CatchRate: 255, BaseSpeed: 56,
		BaseExp: 55, GrowthRt: GrowthMediumSlow, Exp: 0,
	}

	battle := &BattleState{
		Phase:           PhaseActionSelect,
		BattleType:      BattleWild,
		TurnNumber:      3,
		PlayerParty:     []*Pokemon{{ID: 31, Name: "NIDOQUEEN"}}, // present but NOT serialized
		PlayerActive:    0,
		EnemyParty:      []*Pokemon{wild},
		EnemyActive:     0,
		RunAttempts:     1,
		PlayerCaught:    false,
		AllowedActions:  []string{"item"},
		GuaranteedCatch: true,
	}

	data, err := MarshalBattleState(battle)
	if err != nil {
		t.Fatalf("MarshalBattleState: %v", err)
	}

	restored, err := UnmarshalBattleState(data)
	if err != nil {
		t.Fatalf("UnmarshalBattleState: %v", err)
	}

	// Verify battle-level fields
	if restored.Phase != battle.Phase {
		t.Errorf("Phase: got %d, want %d", restored.Phase, battle.Phase)
	}
	if restored.BattleType != battle.BattleType {
		t.Errorf("BattleType: got %d, want %d", restored.BattleType, battle.BattleType)
	}
	if restored.TurnNumber != 3 {
		t.Errorf("TurnNumber: got %d, want 3", restored.TurnNumber)
	}
	if restored.RunAttempts != 1 {
		t.Errorf("RunAttempts: got %d, want 1", restored.RunAttempts)
	}
	if restored.PlayerActive != 0 {
		t.Errorf("PlayerActive: got %d, want 0", restored.PlayerActive)
	}
	if len(restored.AllowedActions) != 1 || restored.AllowedActions[0] != "item" {
		t.Errorf("AllowedActions: got %#v, want [item]", restored.AllowedActions)
	}
	if !restored.GuaranteedCatch {
		t.Error("GuaranteedCatch should round-trip true")
	}

	// Player party should NOT be in JSON — caller reloads from DB
	if len(restored.PlayerParty) != 0 {
		t.Errorf("PlayerParty should be empty from JSON, got %d", len(restored.PlayerParty))
	}

	// Verify enemy pokemon (fully serialized)
	if len(restored.EnemyParty) != 1 {
		t.Fatalf("EnemyParty len: got %d, want 1", len(restored.EnemyParty))
	}
	re := restored.EnemyParty[0]
	if re.ID != 16 || re.Name != "PIDGEY" || re.CurHP != 12 {
		t.Errorf("Enemy: got %d/%s/HP%d, want 16/PIDGEY/12", re.ID, re.Name, re.CurHP)
	}
	if re.CatchRate != 255 {
		t.Errorf("Enemy catch rate: got %d, want 255", re.CatchRate)
	}
	if re.Status != StatusBadPoison || re.BadPoisonTurns != 3 {
		t.Errorf("Enemy status: got %s/%d, want TOX/3", re.Status.String(), re.BadPoisonTurns)
	}
	if !re.IsWild {
		t.Error("Enemy should be wild")
	}
	if re.BaseStats.HP != 40 || re.BaseStats.Speed != 56 {
		t.Errorf("Enemy base stats: HP=%d Speed=%d, want 40/56", re.BaseStats.HP, re.BaseStats.Speed)
	}
	if re.Moves[0].PP != 30 || re.Moves[0].Name != "TACKLE" {
		t.Errorf("Enemy move0: %s PP=%d, want TACKLE/30", re.Moves[0].Name, re.Moves[0].PP)
	}
}

func TestBattleStateRoundTrip_Trainer(t *testing.T) {
	enemy1 := &Pokemon{
		ID: 13, Name: "WEEDLE", Level: 9,
		Type1: TypeBug, Type2: TypePoison,
		MaxHP: 28, CurHP: 0, // fainted
	}
	enemy2 := &Pokemon{
		ID: 14, Name: "KAKUNA", Level: 9,
		Type1: TypeBug, Type2: TypePoison,
		MaxHP: 27, CurHP: 20,
		Moves: [4]MoveSlot{
			{ID: 106, Name: "HARDEN", Type: TypeNormal, Power: 0, Accuracy: 0, PP: 28, MaxPP: 30},
		},
	}

	battle := &BattleState{
		Phase:        PhaseActionSelect,
		BattleType:   BattleTrainer,
		TurnNumber:   5,
		PlayerParty:  []*Pokemon{{ID: 4, Name: "CHARMANDER"}},
		PlayerActive: 0,
		EnemyParty:   []*Pokemon{enemy1, enemy2},
		EnemyActive:  1,
		Trainer: &TrainerMeta{
			ClassName:       "BUG_CATCHER",
			Name:            "Bug Catcher Rick",
			PrizeMoney:      90,
			TrainerObjectID: 42,
		},
	}

	data, err := MarshalBattleState(battle)
	if err != nil {
		t.Fatalf("MarshalBattleState: %v", err)
	}

	restored, err := UnmarshalBattleState(data)
	if err != nil {
		t.Fatalf("UnmarshalBattleState: %v", err)
	}

	if restored.BattleType != BattleTrainer {
		t.Errorf("BattleType: got %d, want %d", restored.BattleType, BattleTrainer)
	}
	if restored.EnemyActive != 1 {
		t.Errorf("EnemyActive: got %d, want 1", restored.EnemyActive)
	}
	if len(restored.EnemyParty) != 2 {
		t.Fatalf("EnemyParty len: got %d, want 2", len(restored.EnemyParty))
	}
	if restored.EnemyParty[0].CurHP != 0 {
		t.Errorf("Enemy[0] should be fainted, got HP %d", restored.EnemyParty[0].CurHP)
	}
	if restored.EnemyParty[1].CurHP != 20 {
		t.Errorf("Enemy[1] HP: got %d, want 20", restored.EnemyParty[1].CurHP)
	}
	if restored.EnemyParty[1].Moves[0].PP != 28 {
		t.Errorf("Enemy[1] move PP: got %d, want 28", restored.EnemyParty[1].Moves[0].PP)
	}

	// Verify trainer meta
	if restored.Trainer == nil {
		t.Fatal("Trainer should not be nil")
	}
	if restored.Trainer.ClassName != "BUG_CATCHER" {
		t.Errorf("Trainer class: got %q, want BUG_CATCHER", restored.Trainer.ClassName)
	}
	if restored.Trainer.TrainerObjectID != 42 {
		t.Errorf("Trainer object ID: got %d, want 42", restored.Trainer.TrainerObjectID)
	}
	if restored.Trainer.PrizeMoney != 90 {
		t.Errorf("Trainer prize: got %d, want 90", restored.Trainer.PrizeMoney)
	}
}

func TestBattleStateRoundTrip_PendingMoveLearn(t *testing.T) {
	battle := &BattleState{
		Phase:        PhaseBattleEnd,
		BattleType:   BattleWild,
		PlayerParty:  []*Pokemon{{ID: 31, Name: "NIDOQUEEN"}},
		PlayerActive: 0,
		EnemyParty:   []*Pokemon{{ID: 16, Name: "PIDGEY", CurHP: 0, MaxHP: 19}},
		EnemyActive:  0,
		PendingMoveLearn: &PendingMove{
			PokemonIndex: 0,
			MoveID:       40,
			MoveName:     "POISON STING",
		},
	}

	data, err := MarshalBattleState(battle)
	if err != nil {
		t.Fatalf("MarshalBattleState: %v", err)
	}

	restored, err := UnmarshalBattleState(data)
	if err != nil {
		t.Fatalf("UnmarshalBattleState: %v", err)
	}

	if restored.PendingMoveLearn == nil {
		t.Fatal("PendingMoveLearn should not be nil")
	}
	if restored.PendingMoveLearn.MoveID != 40 {
		t.Errorf("Pending move ID: got %d, want 40", restored.PendingMoveLearn.MoveID)
	}
	if restored.PendingMoveLearn.MoveName != "POISON STING" {
		t.Errorf("Pending move name: got %q, want POISON STING", restored.PendingMoveLearn.MoveName)
	}
}
