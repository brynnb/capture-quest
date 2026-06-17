package world

import (
	"testing"

	"capturequest/internal/db/cqitems"
)

func TestStoneEvolutionTargetGen1Stones(t *testing.T) {
	tests := []struct {
		stone     string
		pokemonID int
		wantID    int
	}{
		{"MOON_STONE", 30, 31},
		{"MOON_STONE", 33, 34},
		{"MOON_STONE", 35, 36},
		{"MOON_STONE", 39, 40},
		{"FIRE_STONE", 37, 38},
		{"FIRE_STONE", 58, 59},
		{"FIRE_STONE", 133, 136},
		{"THUNDER_STONE", 25, 26},
		{"THUNDER_STONE", 133, 135},
		{"WATER_STONE", 61, 62},
		{"WATER_STONE", 90, 91},
		{"WATER_STONE", 120, 121},
		{"WATER_STONE", 133, 134},
		{"LEAF_STONE", 44, 45},
		{"LEAF_STONE", 70, 71},
		{"LEAF_STONE", 102, 103},
	}

	for _, tt := range tests {
		gotID, ok := stoneEvolutionTarget(tt.stone, tt.pokemonID)
		if !ok || gotID != tt.wantID {
			t.Fatalf("%s on %d = (%d, %v), want (%d, true)", tt.stone, tt.pokemonID, gotID, ok, tt.wantID)
		}
	}
}

func TestStoneEvolutionTargetRejectsWrongStone(t *testing.T) {
	if gotID, ok := stoneEvolutionTarget("FIRE_STONE", 25); ok || gotID != 0 {
		t.Fatalf("FIRE_STONE on Pikachu = (%d, %v), want no evolution", gotID, ok)
	}
}

func TestIsEvolutionStoneUsesTypeOrName(t *testing.T) {
	if !isEvolutionStone(cqitems.CQItem{ItemType: cqItemTypeEvolutionStone, ShortName: "ODD_ROCK"}) {
		t.Fatal("expected evolution item type to count as evolution stone")
	}
	if !isEvolutionStone(cqitems.CQItem{ItemType: cqItemTypeMisc, ShortName: "WATER_STONE"}) {
		t.Fatal("expected _STONE item short name to count as evolution stone")
	}
	if isEvolutionStone(cqitems.CQItem{ItemType: cqItemTypeMisc, ShortName: "RARE_CANDY"}) {
		t.Fatal("did not expect non-stone item to count as evolution stone")
	}
}
