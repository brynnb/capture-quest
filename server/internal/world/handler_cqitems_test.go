package world

import (
	"testing"
)

func TestCoinCaseMessageReportsBalance(t *testing.T) {
	tests := []struct {
		name  string
		coins int
		want  string
	}{
		{name: "zero", coins: 0, want: "You have 0 coins."},
		{name: "one", coins: 1, want: "You have 1 coin."},
		{name: "many", coins: 70, want: "You have 70 coins."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coinCaseMessage(tt.coins); got != tt.want {
				t.Fatalf("coinCaseMessage(%d) = %q, want %q", tt.coins, got, tt.want)
			}
		})
	}
}
