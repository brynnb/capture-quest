package pokebattle

import "testing"

func TestCalculateDamage_Basic(t *testing.T) {
	// Level 50 Charizard (Fire/Flying) using Flamethrower (Fire, power 95)
	// vs Level 50 Venusaur (Grass/Poison)
	// Atk stat = 150, Def stat = 130
	// STAB: yes (Fire move, Fire attacker)
	// Type effectiveness: Fire vs Grass = 2.0x, Fire vs Poison = 1.0x → 2.0x total
	// Random = 255 (max), no crit
	result := CalculateDamage(DamageInput{
		Level:         50,
		AttackStat:    150,
		MoveType:      TypeFire,
		MovePower:     95,
		AttackerType1: TypeFire,
		AttackerType2: TypeFlying,
		DefenseStat:   130,
		DefenderType1: TypeGrass,
		DefenderType2: TypePoison,
		IsCritical:    false,
		RandomValue:   255,
	})

	if !result.IsSTAB {
		t.Error("Expected STAB to be true")
	}
	if result.Effectiveness != 200 {
		t.Errorf("Expected effectiveness 200, got %d", result.Effectiveness)
	}
	if result.Damage <= 0 {
		t.Errorf("Expected positive damage, got %d", result.Damage)
	}
}

func TestCalculateDamage_Immune(t *testing.T) {
	// Normal move vs Ghost type = immune
	result := CalculateDamage(DamageInput{
		Level:         50,
		AttackStat:    100,
		MoveType:      TypeNormal,
		MovePower:     40,
		AttackerType1: TypeNormal,
		AttackerType2: TypeNormal,
		DefenseStat:   100,
		DefenderType1: TypeGhost,
		DefenderType2: TypeGhost,
		IsCritical:    false,
		RandomValue:   255,
	})

	if result.Damage != 0 {
		t.Errorf("Expected 0 damage (immune), got %d", result.Damage)
	}
	if result.Effectiveness != 0 {
		t.Errorf("Expected effectiveness 0, got %d", result.Effectiveness)
	}
}

func TestCalculateDamage_StatusMove(t *testing.T) {
	// Status move (power 0) should do 0 damage
	result := CalculateDamage(DamageInput{
		Level:         50,
		AttackStat:    100,
		MoveType:      TypeNormal,
		MovePower:     0,
		AttackerType1: TypeNormal,
		AttackerType2: TypeNormal,
		DefenseStat:   100,
		DefenderType1: TypeNormal,
		DefenderType2: TypeNormal,
	})

	if result.Damage != 0 {
		t.Errorf("Expected 0 damage for status move, got %d", result.Damage)
	}
}

func TestCalculateDamage_CriticalHit(t *testing.T) {
	// Same setup, compare crit vs non-crit
	base := DamageInput{
		Level:         50,
		AttackStat:    100,
		MoveType:      TypeNormal,
		MovePower:     80,
		AttackerType1: TypeNormal,
		AttackerType2: TypeNormal,
		DefenseStat:   100,
		DefenderType1: TypeNormal,
		DefenderType2: TypeNormal,
		RandomValue:   255,
	}

	base.IsCritical = false
	normalResult := CalculateDamage(base)

	base.IsCritical = true
	critResult := CalculateDamage(base)

	if critResult.Damage <= normalResult.Damage {
		t.Errorf("Critical hit (%d) should deal more damage than normal (%d)", critResult.Damage, normalResult.Damage)
	}
}

func TestCalculateDamage_RandomRange(t *testing.T) {
	base := DamageInput{
		Level:         50,
		AttackStat:    100,
		MoveType:      TypeWater,
		MovePower:     90,
		AttackerType1: TypeWater,
		AttackerType2: TypeWater,
		DefenseStat:   80,
		DefenderType1: TypeFire,
		DefenderType2: TypeFire,
		IsCritical:    false,
	}

	// Min random (217)
	base.RandomValue = 217
	minResult := CalculateDamage(base)

	// Max random (255)
	base.RandomValue = 255
	maxResult := CalculateDamage(base)

	if minResult.Damage > maxResult.Damage {
		t.Errorf("Min random (%d) should not exceed max random (%d)", minResult.Damage, maxResult.Damage)
	}
	if minResult.Damage == maxResult.Damage && maxResult.Damage > 1 {
		t.Errorf("Min and max random should differ for non-trivial damage (both %d)", minResult.Damage)
	}
}

func TestCalculateDamage_4xEffective(t *testing.T) {
	// Ice vs Grass/Flying (e.g. Exeggutor... wait, Grass/Flying doesn't exist in Gen 1)
	// Use: Rock vs Fire/Flying (Charizard) = 2.0 * 2.0 = 4.0x
	result := CalculateDamage(DamageInput{
		Level:         50,
		AttackStat:    100,
		MoveType:      TypeRock,
		MovePower:     75,
		AttackerType1: TypeRock,
		AttackerType2: TypeGround,
		DefenseStat:   100,
		DefenderType1: TypeFire,
		DefenderType2: TypeFlying,
		IsCritical:    false,
		RandomValue:   255,
	})

	if result.Effectiveness != 400 {
		t.Errorf("Expected effectiveness 400 (4x), got %d", result.Effectiveness)
	}
}

func TestCriticalHitChance(t *testing.T) {
	// Pikachu base speed = 90
	// Normal move: 90/512 ≈ 0.1758
	chance := CriticalHitChance(90, false)
	if chance < 0.17 || chance > 0.18 {
		t.Errorf("Pikachu normal crit chance: expected ~0.1758, got %f", chance)
	}

	// High-crit move: 90*8/512 = 720/512 → capped at 255/256 ≈ 0.996
	chance = CriticalHitChance(90, true)
	if chance < 0.99 {
		t.Errorf("Pikachu high-crit chance: expected ~0.996, got %f", chance)
	}

	// Slowpoke base speed = 15
	// Normal: 15/512 ≈ 0.0293
	chance = CriticalHitChance(15, false)
	if chance < 0.02 || chance > 0.04 {
		t.Errorf("Slowpoke normal crit chance: expected ~0.029, got %f", chance)
	}

	// High-crit: 15*8 = 120 → 120/256 ≈ 0.469
	chance = CriticalHitChance(15, true)
	if chance < 0.46 || chance > 0.48 {
		t.Errorf("Slowpoke high-crit chance: expected ~0.469, got %f", chance)
	}
}

func TestIsCriticalHit(t *testing.T) {
	// Base speed 100, normal move: threshold = 100/2 = 50
	// randVal 49 → crit, randVal 50 → no crit
	if !IsCriticalHit(100, false, 49) {
		t.Error("Expected crit with randVal 49, threshold 50")
	}
	if IsCriticalHit(100, false, 50) {
		t.Error("Expected no crit with randVal 50, threshold 50")
	}

	// High-crit move, base speed 40: threshold = 40*8 = 320 → capped at 255
	// randVal 254 → crit, randVal 255 → no crit (0-indexed, 255 is not < 255)
	if !IsCriticalHit(40, true, 254) {
		t.Error("Expected crit with high-crit move, randVal 254")
	}
}
