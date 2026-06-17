package world

import "testing"

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
