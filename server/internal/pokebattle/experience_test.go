package pokebattle

import "testing"

func TestExpForLevel_KnownValues(t *testing.T) {
	// Known Gen 1 values from Bulbapedia
	tests := []struct {
		rate  GrowthRate
		level int
		want  int
	}{
		// Medium Fast (n^3): level 5 = 125, level 10 = 1000, level 100 = 1000000
		{GrowthMediumFast, 1, 0},
		{GrowthMediumFast, 5, 125},
		{GrowthMediumFast, 10, 1000},
		{GrowthMediumFast, 100, 1000000},

		// Fast (4/5 * n^3): level 10 = 800, level 100 = 800000
		{GrowthFast, 10, 800},
		{GrowthFast, 100, 800000},

		// Slow (5/4 * n^3): level 10 = 1250, level 100 = 1250000
		{GrowthSlow, 10, 1250},
		{GrowthSlow, 100, 1250000},

		// Medium Slow (6/5*n^3 - 15*n^2 + 100*n - 140):
		// level 10 = 1200 - 1500 + 1000 - 140 = 560
		{GrowthMediumSlow, 10, 560},
		// level 100 = 1200000 - 150000 + 10000 - 140 = 1059860
		{GrowthMediumSlow, 100, 1059860},
	}

	for _, tt := range tests {
		got := ExpForLevel(tt.rate, tt.level)
		if got != tt.want {
			t.Errorf("ExpForLevel(%s, %d) = %d, want %d", tt.rate, tt.level, got, tt.want)
		}
	}
}

func TestLevelForExp(t *testing.T) {
	// A Pokémon with 1000 XP in Medium Fast should be level 10
	level := LevelForExp(GrowthMediumFast, 1000)
	if level != 10 {
		t.Errorf("LevelForExp(MediumFast, 1000) = %d, want 10", level)
	}

	// 999 XP should be level 9
	level = LevelForExp(GrowthMediumFast, 999)
	if level != 9 {
		t.Errorf("LevelForExp(MediumFast, 999) = %d, want 9", level)
	}

	// 0 XP should be level 1
	level = LevelForExp(GrowthMediumFast, 0)
	if level != 1 {
		t.Errorf("LevelForExp(MediumFast, 0) = %d, want 1", level)
	}

	// Max XP should be level 100
	level = LevelForExp(GrowthMediumFast, 9999999)
	if level != 100 {
		t.Errorf("LevelForExp(MediumFast, 9999999) = %d, want 100", level)
	}
}

func TestCalculateBattleExp(t *testing.T) {
	// Charmander (base_exp=65) defeating a level 5 wild Weedle (base_exp=39)
	// (1.0 * 39 * 5) / 7 = 27
	exp := CalculateBattleExp(39, 5, false)
	if exp != 27 {
		t.Errorf("CalculateBattleExp(39, 5, false) = %d, want 27", exp)
	}

	// Same but trainer battle: (1.5 * 39 * 5) / 7 = 41
	exp = CalculateBattleExp(39, 5, true)
	if exp != 41 {
		t.Errorf("CalculateBattleExp(39, 5, true) = %d, want 41", exp)
	}
}

func TestGrowthRateFromString(t *testing.T) {
	tests := []struct {
		s    string
		want GrowthRate
	}{
		{"MEDIUM_FAST", GrowthMediumFast},
		{"MEDIUM_SLOW", GrowthMediumSlow},
		{"FAST", GrowthFast},
		{"SLOW", GrowthSlow},
		{"UNKNOWN", GrowthMediumFast}, // default
	}
	for _, tt := range tests {
		got := GrowthRateFromString(tt.s)
		if got != tt.want {
			t.Errorf("GrowthRateFromString(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}
