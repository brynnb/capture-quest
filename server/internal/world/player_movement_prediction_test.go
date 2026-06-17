package world

import (
	"testing"

	"capturequest/internal/session"
)

func TestRequestMoveAppendsFromPredictedQueuedStart(t *testing.T) {
	manager := newPredictionTestMovementManager()
	manager.players[1] = &PlayerMovementState{
		CharacterID: 1,
		CurrentX:    0,
		CurrentY:    0,
		MapID:       1,
		Direction:   "RIGHT",
		Path:        []PathNode{{X: 1, Y: 0, ClientSeq: 1}},
		MoveSpeed:   defaultPlayerMoveSpeed,
	}

	result := manager.RequestMoveWithOptions(1, 1, -1, PlayerMoveRequestOptions{
		ExpectedStart: &PathNode{X: 1, Y: 0},
		ClientPath:    []ClientMoveStep{{X: 1, Y: -1, Seq: 2}},
	})

	if result != PlayerMoveRequestStarted {
		t.Fatalf("RequestMoveWithOptions() = %v, want PlayerMoveRequestStarted", result)
	}
	got := manager.players[1].Path
	want := []PathNode{
		{X: 1, Y: 0, ClientSeq: 1},
		{X: 1, Y: -1, ClientSeq: 2},
	}
	if len(got) != len(want) {
		t.Fatalf("path len = %d (%#v), want %d (%#v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path[%d] = %#v, want %#v (full path %#v)", i, got[i], want[i], got)
		}
	}
}

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

func TestRequestMoveRejectsPredictedStartOutsideCurrentPath(t *testing.T) {
	manager := newPredictionTestMovementManager()
	manager.players[1] = &PlayerMovementState{
		CharacterID: 1,
		CurrentX:    0,
		CurrentY:    0,
		MapID:       1,
		Direction:   "RIGHT",
		Path:        []PathNode{{X: 1, Y: 0, ClientSeq: 1}},
		MoveSpeed:   defaultPlayerMoveSpeed,
	}

	result := manager.RequestMoveWithOptions(1, 2, -1, PlayerMoveRequestOptions{
		ExpectedStart: &PathNode{X: 2, Y: 0},
		ClientPath:    []ClientMoveStep{{X: 2, Y: -1, Seq: 2}},
	})

	if result != PlayerMoveRequestBlocked {
		t.Fatalf("RequestMoveWithOptions() = %v, want PlayerMoveRequestBlocked", result)
	}
	got := manager.players[1].Path
	if len(got) != 1 || got[0] != (PathNode{X: 1, Y: 0, ClientSeq: 1}) {
		t.Fatalf("path = %#v, want original queued path preserved", got)
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

func TestPredictedFacingAttemptQueuesWarpActivation(t *testing.T) {
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			1: {
				"0,0": collisionLand,
				"1,0": collisionLand,
				"2,0": collisionBlocked,
			},
		},
	}
	warp := &phaserMapWarp{
		ID:          42,
		SourceMapID: 1,
		X:           2,
		Y:           0,
		DestMapID:   63,
		DestX:       2,
		DestY:       7,
		WarpType:    "door",
	}
	warpManager := newPhaserWarpManager(nil)
	addPhaserWarpIndex(warpManager.byMap, warp.SourceMapID, warp)

	manager := &PlayerMovementManager{
		wh: &WorldHandler{
			ActorManager: actorManager,
			phaserWarps:  warpManager,
		},
		actorManager: actorManager,
		players: map[int]*PlayerMovementState{
			1: {
				CharacterID: 1,
				CurrentX:    0,
				CurrentY:    0,
				MapID:       1,
				Direction:   "RIGHT",
				Path:        []PathNode{{X: 1, Y: 0, ClientSeq: 7}},
				MoveSpeed:   bicyclePlayerMoveSpeed,
			},
		},
	}

	ok := manager.QueueDirectionalWarpActivationAtPredictedPosition(1, 1, 1, 0, "RIGHT")
	if !ok {
		t.Fatal("QueueDirectionalWarpActivationAtPredictedPosition returned false, want true")
	}
	state := manager.players[1]
	if state.PendingWarpActivationID == nil || *state.PendingWarpActivationID != 42 {
		t.Fatalf("pending warp id = %v, want 42", state.PendingWarpActivationID)
	}
	if len(state.Path) != 1 || state.Path[0] != (PathNode{X: 1, Y: 0, ClientSeq: 7}) {
		t.Fatalf("path = %#v, want preserved prefix through predicted tile", state.Path)
	}
}

func TestPredictedRequestedWarpQueuesActivation(t *testing.T) {
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			1: {
				"0,0": collisionLand,
				"1,0": collisionLand,
				"2,0": collisionBlocked,
			},
		},
	}
	warp := &phaserMapWarp{
		ID:          42,
		SourceMapID: 1,
		X:           2,
		Y:           0,
		DestMapID:   63,
		DestX:       2,
		DestY:       7,
		WarpType:    "door",
	}
	warpManager := newPhaserWarpManager(nil)
	warpManager.byID[warp.ID] = warp
	addPhaserWarpIndex(warpManager.byMap, warp.SourceMapID, warp)

	manager := &PlayerMovementManager{
		wh: &WorldHandler{
			ActorManager: actorManager,
			phaserWarps:  warpManager,
		},
		actorManager: actorManager,
		players: map[int]*PlayerMovementState{
			1: {
				CharacterID: 1,
				CurrentX:    0,
				CurrentY:    0,
				MapID:       1,
				Direction:   "RIGHT",
				Path:        []PathNode{{X: 1, Y: 0, ClientSeq: 7}},
				MoveSpeed:   bicyclePlayerMoveSpeed,
			},
		},
	}

	result := manager.RequestMoveWithOptions(1, 1, 0, PlayerMoveRequestOptions{
		ActivateWarpID: &warp.ID,
		ExpectedStart:  &PathNode{X: 1, Y: 0},
	})
	if result != PlayerMoveRequestStarted {
		t.Fatalf("RequestMoveWithOptions() = %v, want PlayerMoveRequestStarted", result)
	}
	state := manager.players[1]
	if state.PendingWarpActivationID == nil || *state.PendingWarpActivationID != 42 {
		t.Fatalf("pending warp id = %v, want 42", state.PendingWarpActivationID)
	}
	if len(state.Path) != 1 || state.Path[0] != (PathNode{X: 1, Y: 0, ClientSeq: 7}) {
		t.Fatalf("path = %#v, want preserved prefix through predicted tile", state.Path)
	}
}

func newPredictionTestMovementManager() *PlayerMovementManager {
	actorManager := NewPhaserActorManager(nil)
	actorManager.collisionMap[1] = map[string]int{
		"0,0":  collisionLand,
		"1,0":  collisionLand,
		"1,-1": collisionLand,
		"2,0":  collisionLand,
		"2,-1": collisionLand,
	}
	return &PlayerMovementManager{
		actorManager: actorManager,
		players:      map[int]*PlayerMovementState{},
	}
}
