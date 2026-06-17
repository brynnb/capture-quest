package world

import (
	"encoding/json"
	"testing"
)

func TestPokemonTower7FPostWinActionsUsesSourceMovementTable(t *testing.T) {
	tests := []struct {
		name           string
		flag           string
		playerX        int
		playerY        int
		wantActor      string
		wantText       string
		wantMovements  []string
		wantActionSize int
	}{
		{
			name:           "rocket 1 right down exit",
			flag:           "EVENT_BEAT_POKEMONTOWER_7_TRAINER_0",
			playerX:        9,
			playerY:        12,
			wantActor:      "ROCKET_1",
			wantText:       "TEXT_POKEMONTOWER7F_ROCKET1",
			wantMovements:  []string{"RIGHT", "DOWN", "DOWN", "DOWN", "DOWN", "DOWN", "LEFT"},
			wantActionSize: 2,
		},
		{
			name:           "rocket 2 down left exit",
			flag:           "EVENT_BEAT_POKEMONTOWER_7_TRAINER_1",
			playerX:        11,
			playerY:        9,
			wantActor:      "ROCKET_2",
			wantText:       "TEXT_POKEMONTOWER7F_ROCKET2",
			wantMovements:  []string{"DOWN", "DOWN", "DOWN", "LEFT", "DOWN", "DOWN"},
			wantActionSize: 2,
		},
		{
			name:           "rocket 3 right down exit",
			flag:           "EVENT_BEAT_POKEMONTOWER_7_TRAINER_2",
			playerX:        9,
			playerY:        8,
			wantActor:      "ROCKET_3",
			wantText:       "TEXT_POKEMONTOWER7F_ROCKET3",
			wantMovements:  []string{"RIGHT", "DOWN", "DOWN", "DOWN", "DOWN", "DOWN", "DOWN"},
			wantActionSize: 2,
		},
		{
			name:           "unknown tile still hides defeated rocket",
			flag:           "EVENT_BEAT_POKEMONTOWER_7_TRAINER_0",
			playerX:        1,
			playerY:        1,
			wantText:       "TEXT_POKEMONTOWER7F_ROCKET1",
			wantActionSize: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapName, raw := pokemonTower7FPostWinActions(&trainerSightData{
				MapID:     148,
				EventFlag: tt.flag,
			}, tt.playerX, tt.playerY)
			if mapName != pokemonTower7FMapName {
				t.Fatalf("mapName = %q, want %q", mapName, pokemonTower7FMapName)
			}
			var actions []CutsceneAction
			if err := json.Unmarshal(raw, &actions); err != nil {
				t.Fatalf("unmarshal actions: %v", err)
			}
			if len(actions) != tt.wantActionSize {
				t.Fatalf("len(actions) = %d, want %d: %#v", len(actions), tt.wantActionSize, actions)
			}
			if tt.wantMovements != nil {
				move := actions[0]
				if move.Type != "move" || move.Actor != tt.wantActor {
					t.Fatalf("move action = %#v, want actor %s", move, tt.wantActor)
				}
				if !sameStrings(move.Movements, tt.wantMovements) {
					t.Fatalf("movements = %#v, want %#v", move.Movements, tt.wantMovements)
				}
			}
			hide := actions[len(actions)-1]
			if hide.Type != "hideObject" || hide.TextConstant != tt.wantText {
				t.Fatalf("hide action = %#v, want text %s", hide, tt.wantText)
			}
		})
	}
}

func TestPokemonTower7FPostWinActionsIgnoresOtherMapsAndFlags(t *testing.T) {
	if _, raw := pokemonTower7FPostWinActions(&trainerSightData{MapID: 1, EventFlag: "EVENT_BEAT_POKEMONTOWER_7_TRAINER_0"}, 9, 12); len(raw) != 0 {
		t.Fatalf("expected no actions for non Tower 7F map, got %s", raw)
	}
	if _, raw := pokemonTower7FPostWinActions(&trainerSightData{MapID: 148, EventFlag: "EVENT_BEAT_OTHER"}, 9, 12); len(raw) != 0 {
		t.Fatalf("expected no actions for unrelated flag, got %s", raw)
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
