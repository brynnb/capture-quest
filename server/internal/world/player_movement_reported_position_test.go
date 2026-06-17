package world

import "testing"

func TestUpdateReportedPositionPreservesQueuedPathForSameTile(t *testing.T) {
	manager := &PlayerMovementManager{
		players: map[int]*PlayerMovementState{
			1: {
				CharacterID: 1,
				CurrentX:    4,
				CurrentY:    10,
				MapID:       40,
				Direction:   "DOWN",
				Path:        []PathNode{{X: 4, Y: 11}},
			},
		},
	}

	manager.UpdateReportedPosition(1, 4, 10, 40, "UP")

	state := manager.players[1]
	if state.Direction != "UP" {
		t.Fatalf("direction = %q, want UP", state.Direction)
	}
	if len(state.Path) != 1 || state.Path[0].X != 4 || state.Path[0].Y != 11 {
		t.Fatalf("path = %#v, want preserved path to (4,11)", state.Path)
	}

	manager.UpdateReportedPosition(1, 4, 10, 40, "")
	if state.Direction != "UP" {
		t.Fatalf("empty reported direction changed direction to %q, want UP", state.Direction)
	}
}

func TestUpdateReportedPositionClearsQueuedPathForDifferentTile(t *testing.T) {
	manager := &PlayerMovementManager{
		players: map[int]*PlayerMovementState{
			1: {
				CharacterID: 1,
				CurrentX:    4,
				CurrentY:    10,
				MapID:       40,
				Direction:   "DOWN",
				Path:        []PathNode{{X: 4, Y: 11}},
			},
		},
	}

	manager.UpdateReportedPosition(1, 5, 10, 40, "RIGHT")

	state := manager.players[1]
	if state.CurrentX != 5 || state.CurrentY != 10 || state.Direction != "RIGHT" {
		t.Fatalf("state position/direction = (%d,%d,%s), want (5,10,RIGHT)", state.CurrentX, state.CurrentY, state.Direction)
	}
	if state.Path != nil {
		t.Fatalf("path = %#v, want cleared", state.Path)
	}
}
