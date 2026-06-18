package world

import (
	"testing"

	"capturequest/internal/db/cqitems"
)

func TestTMHMNeedsMoveSlotResponseIncludesPendingContext(t *testing.T) {
	req := cqItemUseRequest{
		InstanceID: 42,
		PartySlot:  3,
		MoveSlot:   -1,
	}

	resp := tmhmNeedsMoveSlotResponse(req, "PIKACHU", "TM24", "THUNDERBOLT", 85)

	if resp["success"] != true {
		t.Fatalf("success = %#v, want true", resp["success"])
	}
	if resp["needsMoveSlot"] != true {
		t.Fatalf("needsMoveSlot = %#v, want true", resp["needsMoveSlot"])
	}
	if resp["instanceId"] != int32(42) {
		t.Fatalf("instanceId = %#v, want 42", resp["instanceId"])
	}
	if resp["partySlot"] != 3 {
		t.Fatalf("partySlot = %#v, want 3", resp["partySlot"])
	}
	if resp["moveId"] != 85 {
		t.Fatalf("moveId = %#v, want 85", resp["moveId"])
	}
	if resp["moveName"] != "THUNDERBOLT" {
		t.Fatalf("moveName = %#v, want THUNDERBOLT", resp["moveName"])
	}

	wantMessage := "PIKACHU wants to learn THUNDERBOLT, but already knows 4 moves. Choose a move to forget."
	if resp["message"] != wantMessage {
		t.Fatalf("message = %#v, want %q", resp["message"], wantMessage)
	}
}

func TestItemUsableOnPartyOutsideBattleRejectsBattleOnlyAndNonUsableItems(t *testing.T) {
	tests := []struct {
		name string
		item cqitems.CQItem
		want bool
	}{
		{
			name: "potion",
			item: cqitems.CQItem{
				ShortName:  "POTION",
				HealAmount: 20,
			},
			want: true,
		},
		{
			name: "x attack battle only",
			item: cqitems.CQItem{
				ShortName:   "X_ATTACK",
				BonusAttack: 1,
			},
			want: false,
		},
		{
			name: "nugget non usable",
			item: cqitems.CQItem{
				ShortName: "NUGGET",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := itemUsableOnPartyOutsideBattle(tt.item); got != tt.want {
				t.Fatalf("itemUsableOnPartyOutsideBattle(%s) = %v, want %v", tt.item.ShortName, got, tt.want)
			}
		})
	}
}

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
