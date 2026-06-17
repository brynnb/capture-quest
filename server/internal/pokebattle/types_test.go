package pokebattle

import "testing"

func TestTypeEffectiveness_SuperEffective(t *testing.T) {
	cases := []struct {
		atk, def PokemonType
	}{
		{TypeFire, TypeGrass},
		{TypeWater, TypeFire},
		{TypeElectric, TypeWater},
		{TypeGrass, TypeWater},
		{TypeIce, TypeDragon},
		{TypeFighting, TypeNormal},
		{TypeGround, TypeElectric},
		{TypeFlying, TypeBug},
		{TypePsychic, TypeFighting},
		{TypeBug, TypePsychic},
		{TypeRock, TypeFire},
		{TypeGhost, TypeGhost},
		{TypeDragon, TypeDragon},
		{TypePoison, TypeBug}, // Gen 1 specific
		{TypeBug, TypePoison}, // Gen 1 specific
	}
	for _, c := range cases {
		eff := GetTypeEffectiveness(c.atk, c.def)
		if eff != 20 {
			t.Errorf("%s vs %s: expected 20 (super effective), got %d", c.atk, c.def, eff)
		}
	}
}

func TestTypeEffectiveness_NotVeryEffective(t *testing.T) {
	cases := []struct {
		atk, def PokemonType
	}{
		{TypeFire, TypeWater},
		{TypeFire, TypeRock},
		{TypeWater, TypeGrass},
		{TypeElectric, TypeGrass},
		{TypeGrass, TypeFire},
		{TypeGrass, TypePoison},
		{TypeIce, TypeWater},
		{TypeFighting, TypePoison},
		{TypeFighting, TypeFlying},
		{TypeFighting, TypePsychic},
		{TypePoison, TypeGround},
		{TypeBug, TypeFire},
		{TypeBug, TypeGhost},
		{TypeRock, TypeFighting},
		{TypeNormal, TypeRock},
	}
	for _, c := range cases {
		eff := GetTypeEffectiveness(c.atk, c.def)
		if eff != 5 {
			t.Errorf("%s vs %s: expected 5 (not very effective), got %d", c.atk, c.def, eff)
		}
	}
}

func TestTypeEffectiveness_Immune(t *testing.T) {
	cases := []struct {
		atk, def PokemonType
	}{
		{TypeNormal, TypeGhost},
		{TypeElectric, TypeGround},
		{TypeFighting, TypeGhost},
		{TypeGround, TypeFlying},
		{TypeGhost, TypeNormal},
		{TypeGhost, TypePsychic}, // Gen 1 bug
	}
	for _, c := range cases {
		eff := GetTypeEffectiveness(c.atk, c.def)
		if eff != 0 {
			t.Errorf("%s vs %s: expected 0 (immune), got %d", c.atk, c.def, eff)
		}
	}
}

func TestTypeEffectiveness_Neutral(t *testing.T) {
	cases := []struct {
		atk, def PokemonType
	}{
		{TypeNormal, TypeNormal},
		{TypeFire, TypeNormal},
		{TypeWater, TypeNormal},
		{TypeIce, TypeFire}, // Gen 1: Ice vs Fire is neutral (changed to NVE in Gen 2)
	}
	for _, c := range cases {
		eff := GetTypeEffectiveness(c.atk, c.def)
		if eff != 10 {
			t.Errorf("%s vs %s: expected 10 (neutral), got %d", c.atk, c.def, eff)
		}
	}
}

func TestGetMoveEffectiveness_SingleType(t *testing.T) {
	// Fire vs Grass (single type) = 2.0x = 200
	eff := GetMoveEffectiveness(TypeFire, TypeGrass, TypeGrass)
	if eff != 200 {
		t.Errorf("Fire vs Grass: expected 200, got %d", eff)
	}

	// Normal vs Ghost (single type) = 0x = 0
	eff = GetMoveEffectiveness(TypeNormal, TypeGhost, TypeGhost)
	if eff != 0 {
		t.Errorf("Normal vs Ghost: expected 0, got %d", eff)
	}
}

func TestGetMoveEffectiveness_DualType(t *testing.T) {
	// Fire vs Grass/Ice = 2.0 * 2.0 = 4.0x = 400
	eff := GetMoveEffectiveness(TypeFire, TypeGrass, TypeIce)
	if eff != 400 {
		t.Errorf("Fire vs Grass/Ice: expected 400, got %d", eff)
	}

	// Electric vs Water/Flying = 2.0 * 2.0 = 4.0x = 400
	eff = GetMoveEffectiveness(TypeElectric, TypeWater, TypeFlying)
	if eff != 400 {
		t.Errorf("Electric vs Water/Flying: expected 400, got %d", eff)
	}

	// Grass vs Water/Poison = 2.0 * 0.5 = 1.0x = 100
	eff = GetMoveEffectiveness(TypeGrass, TypeWater, TypePoison)
	if eff != 100 {
		t.Errorf("Grass vs Water/Poison: expected 100, got %d", eff)
	}

	// Ground vs Flying/Normal = 0 * 1.0 = 0x = 0
	eff = GetMoveEffectiveness(TypeGround, TypeFlying, TypeNormal)
	if eff != 0 {
		t.Errorf("Ground vs Flying/Normal: expected 0, got %d", eff)
	}

	// Fire vs Water/Rock = 0.5 * 0.5 = 0.25x = 25
	eff = GetMoveEffectiveness(TypeFire, TypeWater, TypeRock)
	if eff != 25 {
		t.Errorf("Fire vs Water/Rock: expected 25, got %d", eff)
	}
}

func TestTypeFromString(t *testing.T) {
	typ, ok := TypeFromString("FIRE")
	if !ok || typ != TypeFire {
		t.Errorf("TypeFromString(FIRE): expected TypeFire, got %v (ok=%v)", typ, ok)
	}

	_, ok = TypeFromString("FAIRY")
	if ok {
		t.Error("TypeFromString(FAIRY): expected false, got true")
	}
}
