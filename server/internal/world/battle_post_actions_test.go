package world

import (
	"encoding/json"
	"testing"

	"capturequest/internal/pokebattle"
)

func TestBattleHasScriptedPartyHeal(t *testing.T) {
	tests := []struct {
		name        string
		battle      *pokebattle.BattleState
		wonOrCaught bool
		want        bool
	}{
		{
			name: "trainer win heal",
			battle: &pokebattle.BattleState{
				Trainer: &pokebattle.TrainerMeta{
					PostWinActions: json.RawMessage(`[{"type":"healParty"}]`),
				},
			},
			wonOrCaught: true,
			want:        true,
		},
		{
			name: "trainer lose heal",
			battle: &pokebattle.BattleState{
				Trainer: &pokebattle.TrainerMeta{
					PostLoseActions: json.RawMessage(`[{"type":"healParty"}]`),
				},
			},
			want: true,
		},
		{
			name: "nested parallel heal",
			battle: &pokebattle.BattleState{
				Trainer: &pokebattle.TrainerMeta{
					PostWinActions: json.RawMessage(`[{"type":"parallel","actions":[{"type":"healParty"}]}]`),
				},
			},
			wonOrCaught: true,
			want:        true,
		},
		{
			name: "wild win heal",
			battle: &pokebattle.BattleState{
				BattleType:         pokebattle.BattleWild,
				WildPostWinActions: json.RawMessage(`[{"type":"healParty"}]`),
			},
			wonOrCaught: true,
			want:        true,
		},
		{
			name: "trainer win ignores lose-only heal",
			battle: &pokebattle.BattleState{
				Trainer: &pokebattle.TrainerMeta{
					PostLoseActions: json.RawMessage(`[{"type":"healParty"}]`),
				},
			},
			wonOrCaught: true,
			want:        false,
		},
		{
			name: "invalid actions are not treated as heal",
			battle: &pokebattle.BattleState{
				Trainer: &pokebattle.TrainerMeta{
					PostWinActions: json.RawMessage(`not json`),
				},
			},
			wonOrCaught: true,
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := battleHasScriptedPartyHeal(tt.battle, tt.wonOrCaught); got != tt.want {
				t.Fatalf("battleHasScriptedPartyHeal() = %t, want %t", got, tt.want)
			}
		})
	}
}
