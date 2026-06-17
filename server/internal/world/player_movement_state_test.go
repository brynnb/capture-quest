package world

import (
	"testing"

	"capturequest/internal/session"
)

func TestRegisterPlayerSeedsSessionPosition(t *testing.T) {
	manager := &PlayerMovementManager{
		players: make(map[int]*PlayerMovementState),
	}
	ses := &session.Session{SessionID: 7, MapID: -1}

	manager.RegisterPlayer(ses, 48, 9, 2, UnifiedOverworldMapID, "DOWN")

	if got, want := int(ses.X), 9; got != want {
		t.Fatalf("session X = %d, want %d", got, want)
	}
	if got, want := int(ses.Y), 2; got != want {
		t.Fatalf("session Y = %d, want %d", got, want)
	}
	if got, want := ses.MapID, UnifiedOverworldMapID; got != want {
		t.Fatalf("session MapID = %d, want %d", got, want)
	}
}

func TestBicycleSpeedIsActiveOnlyOnOverworld(t *testing.T) {
	manager := &PlayerMovementManager{
		actorManager: &PhaserActorManager{overworldMapIds: map[int]bool{1: true}},
		players: map[int]*PlayerMovementState{
			1: {
				CharacterID:  1,
				MapID:        1,
				WantsBicycle: true,
				MoveSpeed:    defaultPlayerMoveSpeed,
			},
		},
	}
	manager.updateMovementSpeed(manager.players[1])
	if got := manager.players[1].MoveSpeed; got != bicyclePlayerMoveSpeed {
		t.Fatalf("overworld bicycle speed = %s, want %s", got, bicyclePlayerMoveSpeed)
	}

	manager.UpdateMapID(1, 40)
	if got := manager.players[1].MoveSpeed; got != defaultPlayerMoveSpeed {
		t.Fatalf("interior bicycle speed = %s, want %s", got, defaultPlayerMoveSpeed)
	}

	manager.UpdateMapID(1, UnifiedOverworldMapID)
	if got := manager.players[1].MoveSpeed; got != bicyclePlayerMoveSpeed {
		t.Fatalf("returned overworld bicycle speed = %s, want %s", got, bicyclePlayerMoveSpeed)
	}
}

func TestBicycleSpeedIsClearlyFasterThanWalking(t *testing.T) {
	if bicyclePlayerMoveSpeed > defaultPlayerMoveSpeed/2 {
		t.Fatalf("bicycle speed = %s, want at least twice as fast as walking speed %s",
			bicyclePlayerMoveSpeed, defaultPlayerMoveSpeed)
	}
}

func TestCyclingRoadEntryForcesBicycleUntilInterior(t *testing.T) {
	manager := &PlayerMovementManager{
		actorManager: &PhaserActorManager{overworldMapIds: map[int]bool{
			route16MapID: true,
			route18MapID: true,
		}},
		players: map[int]*PlayerMovementState{
			1: {
				CharacterID: 1,
				MapID:       UnifiedOverworldMapID,
				CurrentX:    77,
				CurrentY:    -108,
				MoveSpeed:   defaultPlayerMoveSpeed,
			},
		},
	}

	manager.applyBicycleMapRules(manager.players[1])
	if state := manager.players[1]; !state.ForcedBicycle || state.MoveSpeed != bicyclePlayerMoveSpeed {
		t.Fatalf("cycling road entry state = forced:%t speed:%s, want forced bike speed %s",
			state.ForcedBicycle, state.MoveSpeed, bicyclePlayerMoveSpeed)
	}

	result, ok := manager.ToggleBicycle(1)
	if !ok || !result.ForcedRiding || !result.ActiveRiding {
		t.Fatalf("forced bicycle toggle = (%+v,%t), want forced active riding", result, ok)
	}
	if state := manager.players[1]; !state.ForcedBicycle || state.MoveSpeed != bicyclePlayerMoveSpeed {
		t.Fatalf("forced toggle changed state = forced:%t speed:%s", state.ForcedBicycle, state.MoveSpeed)
	}

	manager.UpdateMapID(1, 186)
	if state := manager.players[1]; state.ForcedBicycle || state.MoveSpeed != defaultPlayerMoveSpeed {
		t.Fatalf("interior gate state = forced:%t speed:%s, want walking speed %s",
			state.ForcedBicycle, state.MoveSpeed, defaultPlayerMoveSpeed)
	}
}
