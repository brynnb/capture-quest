package pokebattle

import "testing"

func TestHPiv(t *testing.T) {
	// IVs: Atk=15, Def=15, Spd=15, Spc=15 → HP IV = 1111 = 15
	ivs := IVs{Attack: 15, Defense: 15, Speed: 15, Special: 15}
	if ivs.HPiv() != 15 {
		t.Errorf("All-15 IVs: expected HP IV 15, got %d", ivs.HPiv())
	}

	// IVs: Atk=0, Def=0, Spd=0, Spc=0 → HP IV = 0000 = 0
	ivs = IVs{Attack: 0, Defense: 0, Speed: 0, Special: 0}
	if ivs.HPiv() != 0 {
		t.Errorf("All-0 IVs: expected HP IV 0, got %d", ivs.HPiv())
	}

	// IVs: Atk=5(0101), Def=10(1010), Spd=3(0011), Spc=12(1100)
	// HP IV = bit3(Atk&1=1), bit2(Def&1=0), bit1(Spd&1=1), bit0(Spc&1=0) = 1010 = 10
	ivs = IVs{Attack: 5, Defense: 10, Speed: 3, Special: 12}
	if ivs.HPiv() != 10 {
		t.Errorf("Mixed IVs: expected HP IV 10, got %d", ivs.HPiv())
	}
}

func TestCalculateHP(t *testing.T) {
	// Pikachu at level 81 with perfect IVs and 0 EVs
	// Base HP = 35, IV = 15, EV = 0, Level = 81
	// ((35 + 15) * 2 + 0) * 81 / 100 + 81 + 10
	// = (100) * 81 / 100 + 91
	// = 81 + 91 = 172
	hp := CalculateHP(35, 15, 0, 81)
	if hp != 172 {
		t.Errorf("Pikachu L81 perfect IV 0 EV: expected HP 172, got %d", hp)
	}

	// Level 100, base 255 (Chansey), perfect IVs, max EVs (65535)
	// EV bonus = floor(sqrt(65535)) / 4 = floor(255.998) / 4 = 255 / 4 = 63
	// ((255 + 15) * 2 + 63) * 100 / 100 + 100 + 10
	// = (540 + 63) + 110
	// = 603 + 110 = 713
	hp = CalculateHP(255, 15, 65535, 100)
	if hp != 713 {
		t.Errorf("Chansey L100 max: expected HP 713, got %d", hp)
	}
}

func TestCalculateStat(t *testing.T) {
	// Pikachu at level 81 with perfect IVs and 0 EVs
	// Base Speed = 90, IV = 15, EV = 0, Level = 81
	// ((90 + 15) * 2 + 0) * 81 / 100 + 5
	// = 210 * 81 / 100 + 5
	// = 170 + 5 = 175
	spd := CalculateStat(90, 15, 0, 81)
	if spd != 175 {
		t.Errorf("Pikachu L81 Speed: expected 175, got %d", spd)
	}

	// Level 5, base 49 (Bulbasaur Attack), IV 8, EV 0
	// ((49 + 8) * 2 + 0) * 5 / 100 + 5
	// = 114 * 5 / 100 + 5
	// = 5 + 5 = 10
	atk := CalculateStat(49, 8, 0, 5)
	if atk != 10 {
		t.Errorf("Bulbasaur L5 Attack: expected 10, got %d", atk)
	}
}

func TestCalculateAllStats(t *testing.T) {
	// Mewtwo at level 100, perfect IVs, 0 EVs
	base := BaseStats{HP: 106, Attack: 110, Defense: 90, Special: 154, Speed: 130}
	ivs := IVs{Attack: 15, Defense: 15, Speed: 15, Special: 15}
	evs := EVs{} // all 0

	hp, atk, def, spc, spd := CalculateAllStats(base, ivs, evs, 100)

	// HP = ((106+15)*2)*100/100 + 100 + 10 = 242 + 110 = 352
	if hp != 352 {
		t.Errorf("Mewtwo L100 HP: expected 352, got %d", hp)
	}
	// Atk = ((110+15)*2)*100/100 + 5 = 250 + 5 = 255
	if atk != 255 {
		t.Errorf("Mewtwo L100 Atk: expected 255, got %d", atk)
	}
	// Def = ((90+15)*2)*100/100 + 5 = 210 + 5 = 215
	if def != 215 {
		t.Errorf("Mewtwo L100 Def: expected 215, got %d", def)
	}
	// Spc = ((154+15)*2)*100/100 + 5 = 338 + 5 = 343
	if spc != 343 {
		t.Errorf("Mewtwo L100 Spc: expected 343, got %d", spc)
	}
	// Spd = ((130+15)*2)*100/100 + 5 = 290 + 5 = 295
	if spd != 295 {
		t.Errorf("Mewtwo L100 Spd: expected 295, got %d", spd)
	}
}

func TestGetMovesForLevel(t *testing.T) {
	// Bulbasaur: default moves are Tackle (33) and Growl (45)
	// Learnset: L7 Leech Seed (73), L13 Vine Whip (22), L20 Poison Powder (77), L27 Razor Leaf (75)
	defaults := [4]int{33, 45, 0, 0}
	learnset := [][2]int{
		{7, 73},  // Leech Seed
		{13, 22}, // Vine Whip
		{20, 77}, // Poison Powder
		{27, 75}, // Razor Leaf
	}

	// Level 5: only default moves
	moves := GetMovesForLevel(5, defaults, learnset)
	if moves != [4]int{33, 45, 0, 0} {
		t.Errorf("L5: expected [33 45 0 0], got %v", moves)
	}

	// Level 13: Tackle, Growl, Leech Seed, Vine Whip
	moves = GetMovesForLevel(13, defaults, learnset)
	if moves != [4]int{33, 45, 73, 22} {
		t.Errorf("L13: expected [33 45 73 22], got %v", moves)
	}

	// Level 27: all 4 slots full, oldest pushed out
	// After L7: [Tackle, Growl, Leech Seed]
	// After L13: [Tackle, Growl, Leech Seed, Vine Whip]
	// After L20: [Growl, Leech Seed, Vine Whip, Poison Powder] (Tackle pushed out)
	// After L27: [Leech Seed, Vine Whip, Poison Powder, Razor Leaf] (Growl pushed out)
	moves = GetMovesForLevel(27, defaults, learnset)
	if moves != [4]int{73, 22, 77, 75} {
		t.Errorf("L27: expected [73 22 77 75], got %v", moves)
	}
}
