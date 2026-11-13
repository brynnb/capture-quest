package world

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func setupPlayerStepBuffer(t *testing.T) (*WorldHandler, *session.Session, *recordingMessenger) {
	t.Helper()
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `UPDATE phaser_maps SET width=40 WHERE id=50;
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type)
 SELECT 50,x,8,1,1 FROM generate_series(9,31) AS x;`)
	wh.ActorManager.InvalidateCollisionMap(50)
	return wh, ses, messages
}

func issuePlayerStepBuffer(t *testing.T, wh *WorldHandler, ses *session.Session, messages *recordingMessenger, directions ...string) protocol.PlayerStepResponse {
	t.Helper()
	if len(directions) == 0 {
		t.Fatal("buffer fixture needs a first direction")
	}
	x, y := 7, 8
	mapID := wh.PlayerMovement.players[42].MapID
	payload, err := json.Marshal(protocol.PlayerStepRequest{
		MapID: mapID, FromX: &x, FromY: &y, Direction: directions[0],
		LookAhead: directions[1:], RequestID: "buffer-issue",
	})
	if err != nil {
		t.Fatal(err)
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, string(payload))
	for _, message := range messages.streams {
		if message.opcode != opcodes.PlayerStepResponse {
			continue
		}
		var result protocol.PlayerStepResponse
		if err := json.Unmarshal(message.payload, &result); err != nil || !result.Success || !result.Buffered || result.StepToken == "" {
			t.Fatalf("buffer issuance=%s err=%v", message.payload, err)
		}
		return result
	}
	t.Fatal("missing buffer issuance")
	return protocol.PlayerStepResponse{}
}

func assertBufferedStepRejected(t *testing.T, wh *WorldHandler, ses *session.Session, messages *recordingMessenger, token string) {
	t.Helper()
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"requestId":"buffer-reject","stepToken":%q}`, token))
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PlayerStepCompleteResponse {
		t.Fatalf("rejected step published extra messages: %+v", messages.streams)
	}
	var failure protocol.PlayerStepError
	if err := json.Unmarshal(messages.streams[0].payload, &failure); err != nil || failure.Success || failure.Error == "" {
		t.Fatalf("buffer rejection=%+v err=%v", failure, err)
	}
	receipt, err := loadMovementReceipt(context.Background(), wh.database, 42, token)
	if err != nil || receipt != nil {
		t.Fatalf("rejected child retained a receipt: %+v err=%v", receipt, err)
	}
}

func TestPlayerStepBufferBoundsWindowWithoutCommittingUnusedGrants(t *testing.T) {
	wh, ses, messages := setupPlayerStepBuffer(t)
	rowID := seedStepDaycare(t, wh)
	directions := []string{"RIGHT", "RIGHT", "RIGHT", "RIGHT", "RIGHT", "RIGHT", "RIGHT", "RIGHT"}
	step := issuePlayerStepBuffer(t, wh, ses, messages, directions...)
	window := append([]protocol.PlayerStepResponse{step}, step.ContinuationSteps...)
	if len(window) != 8 {
		t.Fatalf("issued window length=%d, want 8", len(window))
	}
	seen := map[string]bool{}
	for i, grant := range window {
		if grant.X != 8+i || grant.Y != 8 || grant.MapID != 50 || !validMovementToken(grant.StepToken) || seen[grant.StepToken] {
			t.Fatalf("invalid grant %d: %+v", i, grant)
		}
		seen[grant.StepToken] = true
		receipt, err := loadMovementReceipt(context.Background(), wh.database, 42, grant.StepToken)
		if err != nil || receipt != nil {
			t.Fatalf("unused grant committed a receipt: %+v err=%v", receipt, err)
		}
	}
	assertStepPosition(t, wh, ses, 7)
	assertStepDaycare(t, wh, rowID, 125)

	// An oversized admission must leave the valid live window untouched.
	before := wh.PlayerMovement.players[42].pendingStep
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"requestId":"too-large","mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT","lookAhead":["RIGHT","RIGHT","RIGHT","RIGHT","RIGHT","RIGHT","RIGHT","RIGHT"]}`)
	var rejected protocol.PlayerStepError
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &rejected) != nil || rejected.Success || rejected.Error == "" {
		t.Fatalf("oversized window accepted: %+v", rejected)
	}
	if wh.PlayerMovement.players[42].pendingStep != before {
		t.Fatal("oversized request replaced the usable window")
	}

	// Completing one tile frees one slot; an oversized refill still cannot
	// exceed eight pending grants or commit any of them in advance.
	result := pipelineReply(t, wh, ses, messages, step.StepToken,
		fmt.Sprintf(`,"afterStepToken":%q,"lookAhead":["RIGHT","RIGHT","RIGHT","RIGHT","RIGHT","RIGHT","RIGHT"]`, window[7].StepToken))
	if len(result.ContinuationSteps) != 8 || result.ContinuationSteps[7].X != 16 {
		t.Fatalf("refilled window=%+v", result.ContinuationSteps)
	}
	pending := 0
	for next := wh.PlayerMovement.players[42].pendingStep; next != nil; next = next.next {
		pending++
	}
	if pending != 8 {
		t.Fatalf("server retained %d pending grants", pending)
	}
	assertStepPosition(t, wh, ses, 8)
	assertStepDaycare(t, wh, rowID, 126)
}

func TestPlayerStepBufferPreservesUsablePrefixBeforeBlockedTile(t *testing.T) {
	wh, ses, messages := setupPlayerStepBuffer(t)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'BLOCKER','npc','SPRITE_RED',10,8)`)
	step := issuePlayerStepBuffer(t, wh, ses, messages, "RIGHT", "RIGHT", "RIGHT", "RIGHT")
	if step.X != 8 || len(step.ContinuationSteps) != 1 || step.ContinuationSteps[0].X != 9 {
		t.Fatalf("blocked future tile did not preserve exactly the safe prefix: %+v", step)
	}
	assertStepPosition(t, wh, ses, 7)
}

func TestPlayerStepBufferPacesEveryAuthorizedSegmentIncludingLedges(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		directions []string
		durations  []time.Duration
	}{
		{"ordinary", []string{"RIGHT", "RIGHT"}, []time.Duration{200 * time.Millisecond, 200 * time.Millisecond}},
		{"initial ledge", []string{"DOWN", "RIGHT"}, []time.Duration{400 * time.Millisecond, 200 * time.Millisecond}},
		{"buffered ledge", []string{"RIGHT", "DOWN"}, []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			wh, ses, messages := setupPlayerStepBuffer(t)
			testdb.Exec(t, wh.database, `UPDATE phaser_maps SET is_overworld=1 WHERE id=50;
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,8,9,1,0),(50,7,10,1,1),(50,8,10,1,1);
 UPDATE phaser_tiles SET map_id=NULL WHERE map_id=50;
 UPDATE phaser_tiles SET raw_foot_tile_id=44 WHERE map_id IS NULL AND x IN(7,8) AND y=8;
 UPDATE phaser_tiles SET raw_foot_tile_id=55 WHERE map_id IS NULL AND x IN(7,8) AND y=9;`)
			// Ledges use overworld foot-tile rules. Both the movement registration
			// and source normalization must therefore use the unified visible map.
			wh.ActorManager.overworldMapIds[50] = true
			wh.ActorManager.InvalidateCollisionMap(50)
			if err := wh.ActorManager.ensureWalkableMapLoaded(UnifiedOverworldMapID); err != nil {
				t.Fatal(err)
			}
			wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, UnifiedOverworldMapID, "UP")
			wh.PlayerMovement.players[42].MoveSpeed = 200 * time.Millisecond
			step := issuePlayerStepBuffer(t, wh, ses, messages, fixture.directions...)
			if len(step.ContinuationSteps) != 1 {
				t.Fatalf("missing paced continuation: %+v", step)
			}
			head := wh.PlayerMovement.players[42].pendingStep
			child := head.next
			firstDeadline := head.completeAfter
			if got := head.completeAfter.Sub(head.issuedAt); got != fixture.durations[0] {
				t.Fatalf("initial deadline=%v, want %v", got, fixture.durations[0])
			}
			if got := child.completeAfter.Sub(head.completeAfter); got != fixture.durations[1] {
				t.Fatalf("child deadline interval=%v, want %v", got, fixture.durations[1])
			}
			pipelineReply(t, wh, ses, messages, step.StepToken, "")
			if time.Now().Before(firstDeadline) {
				t.Fatal("initial step committed before its travel duration")
			}
			pending := wh.PlayerMovement.players[42].pendingStep
			if pending != child || pending.completeAfter.Before(firstDeadline.Add(fixture.durations[1])) {
				t.Fatalf("completion accelerated the next segment: %+v", pending)
			}
			nextDeadline := pending.completeAfter
			result := pipelineReply(t, wh, ses, messages, child.token, "")
			if time.Now().Before(nextDeadline) {
				t.Fatal("buffered step committed before its travel duration")
			}
			if result.X != child.x || result.Y != child.y || wh.PlayerMovement.players[42].pendingStep != nil {
				t.Fatalf("paced route did not finish: %+v", result)
			}
			var x, y int
			if err := wh.database.QueryRow(`SELECT x,y FROM character_data WHERE id=42`).Scan(&x, &y); err != nil || x != child.x || y != child.y {
				t.Fatalf("paced durable endpoint=%d,%d err=%v", x, y, err)
			}
		})
	}
}

func TestPlayerStepBufferRevalidatesCollisionAndRetiresDependentGrants(t *testing.T) {
	wh, ses, messages := setupPlayerStepBuffer(t)
	rowID := seedStepDaycare(t, wh)
	step := issuePlayerStepBuffer(t, wh, ses, messages, "RIGHT", "RIGHT", "RIGHT")
	pipelineReply(t, wh, ses, messages, step.StepToken, "")
	testdb.Exec(t, wh.database, `INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'BLOCKER','npc','SPRITE_RED',9,8)`)

	assertBufferedStepRejected(t, wh, ses, messages, step.ContinuationSteps[0].StepToken)
	if wh.PlayerMovement.players[42].pendingStep != nil {
		t.Fatal("blocked head retained dependent authorizations")
	}
	assertBufferedStepRejected(t, wh, ses, messages, step.ContinuationSteps[1].StepToken)
	assertStepPosition(t, wh, ses, 8)
	assertStepDaycare(t, wh, rowID, 126)
}

func TestPlayerStepBufferParentReplayPreservesAlreadyObservedChild(t *testing.T) {
	wh, ses, messages := setupPlayerStepBuffer(t)
	rowID := seedStepDaycare(t, wh)
	step := issuePlayerStepBuffer(t, wh, ses, messages, "RIGHT", "RIGHT", "RIGHT")
	pipelineReply(t, wh, ses, messages, step.StepToken, "")
	child := wh.PlayerMovement.players[42].pendingStep

	replay := pipelineReply(t, wh, ses, messages, step.StepToken, "")
	if !replay.Replayed || child == nil || wh.PlayerMovement.players[42].pendingStep != child || child.token != step.ContinuationSteps[0].StepToken {
		t.Fatal("parent replay invalidated its already published buffered child")
	}
	assertStepPosition(t, wh, ses, 8)
	assertStepDaycare(t, wh, rowID, 126)
	pipelineReply(t, wh, ses, messages, child.token, "")
	assertStepPosition(t, wh, ses, 9)
	assertStepDaycare(t, wh, rowID, 127)
}

func TestPlayerStepBufferLaterReceiptRepairsLostRefillPublication(t *testing.T) {
	wh, ses, messages := setupPlayerStepBuffer(t)
	rowID := seedStepDaycare(t, wh)
	step := issuePlayerStepBuffer(t, wh, ses, messages, "RIGHT", "RIGHT", "RIGHT")
	lost := pipelineReply(t, wh, ses, messages, step.StepToken,
		fmt.Sprintf(`,"afterStepToken":%q,"lookAhead":["RIGHT","RIGHT"]`, step.ContinuationSteps[1].StepToken))
	if len(lost.ContinuationSteps) != 4 {
		t.Fatalf("refill did not produce the expected window: %+v", lost)
	}
	// The client still knows this child from the initial admission, even if
	// the reply which appended x=11 and x=12 never reached it.
	repaired := pipelineReply(t, wh, ses, messages, step.ContinuationSteps[0].StepToken, "")
	if len(repaired.ContinuationSteps) != 3 {
		t.Fatalf("later receipt omitted the pending window: %+v", repaired)
	}
	for i, grant := range repaired.ContinuationSteps {
		if grant.StepToken != lost.ContinuationSteps[i+1].StepToken || grant.X != 10+i {
			t.Fatalf("republication replaced or reordered grant %d: %+v", i, grant)
		}
	}
	assertStepPosition(t, wh, ses, 9)
	assertStepDaycare(t, wh, rowID, 127)
	cancelled := pipelineReply(t, wh, ses, messages, repaired.ContinuationSteps[0].StepToken, `,"cancel":true`)
	if !cancelled.Cancelled || wh.PlayerMovement.players[42].pendingStep != nil {
		t.Fatal("cancelling the first unused grant did not retire its entire suffix")
	}
	assertStepPosition(t, wh, ses, 9)
	assertStepDaycare(t, wh, rowID, 127)
}

func TestPlayerStepBufferEffectBoundaryRetiresSuffixWithoutExtraCommits(t *testing.T) {
	for _, kind := range []string{"trainer", "cutscene", "spin"} {
		t.Run(kind, func(t *testing.T) {
			wh, ses, messages := setupPlayerStepBuffer(t)
			rowID := seedStepDaycare(t, wh)
			wantExperience := 126
			switch kind {
			case "trainer":
				wh.EventFlags = NewEventFlagManager(wh.database)
				wh.TrainerEncounter = NewTrainerEncounterManager(wh)
				wh.TrainerEncounter.byMap[50] = []*trainerSightData{{ObjectID: 77, MapID: 50, X: 8, Y: 7, Direction: "DOWN", SightRange: 1, RuntimeActorID: 707, Name: "TEST"}}
				wantExperience = 125 // Trainer arrival precedes ordinary step counters.
			case "cutscene":
				required := "BUFFER_STEP_ALLOWED"
				testdb.Exec(t, wh.database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,$1)`, required)
				wh.EventFlags = NewEventFlagManager(wh.database)
				wh.CoordTriggers = NewCoordinateTriggerManager(wh.database)
				wh.CoordTriggers.byMap[50] = map[string][]CoordinateTrigger{tileKey(8, 8): {{MapID: 50, MapName: "ROOM", Label: "BUFFER_STEP", X: 8, Y: 8}}}
				wh.Cutscenes = NewCutsceneManager(wh.database)
				wh.Cutscenes.byLabel["BUFFER_STEP"] = &CutsceneScript{ScriptLabel: "BUFFER_STEP", MapName: "ROOM", TriggerType: "coord", RequiresFlag: &required, Actions: json.RawMessage(`[]`)}
			case "spin":
				wh.Cutscenes = NewCutsceneManager(wh.database)
				wh.Cutscenes.mapIDToName[50] = "ROOM"
				wh.SpinTiles = NewSpinTileManager(wh.database)
				wh.SpinTiles.byMap["ROOM"] = map[string]*SpinTile{"8,8": {MapName: "ROOM", X: 8, Y: 8, Movements: []SpinMovement{{Direction: "RIGHT", Count: 2}}}}
			}
			step := issuePlayerStepBuffer(t, wh, ses, messages, "RIGHT", "RIGHT", "RIGHT")
			if len(step.ContinuationSteps) != 2 {
				t.Fatalf("fixture has no already-issued suffix: %+v", step)
			}
			result := pipelineReply(t, wh, ses, messages, step.StepToken, "")
			if !result.StopPath || len(result.ContinuationSteps) != 0 || wh.PlayerMovement.players[42].pendingStep != nil {
				t.Fatalf("effect retained a user continuation: %+v", result)
			}
			assertStepPosition(t, wh, ses, 8)
			assertStepDaycare(t, wh, rowID, wantExperience)
			for _, child := range step.ContinuationSteps {
				assertBufferedStepRejected(t, wh, ses, messages, child.StepToken)
			}
			replay := pipelineReply(t, wh, ses, messages, step.StepToken, "")
			if !replay.Replayed || len(messages.streams) != 1 {
				t.Fatal("replayed boundary republished gameplay effects")
			}
			assertStepPosition(t, wh, ses, 8)
			assertStepDaycare(t, wh, rowID, wantExperience)
			if kind == "spin" {
				path := wh.PlayerMovement.players[42].Path
				if len(path) != 2 || path[0].X != 9 || path[1].X != 10 {
					t.Fatalf("user suffix replaced the committed forced route: %+v", path)
				}
			} else {
				table := "character_trainer_encounters"
				if kind == "cutscene" {
					table = "character_cutscene_plans"
				}
				var plans int
				if err := wh.database.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE character_id=42").Scan(&plans); err != nil || plans != 1 {
					t.Fatalf("boundary plans=%d err=%v", plans, err)
				}
			}
		})
	}
}

func TestPlayerStepBufferRefillAfterLateTailMaintainsCadence(t *testing.T) {
	wh, ses, messages := setupPlayerStepBuffer(t)
	rowID := seedStepDaycare(t, wh)
	speed := 200 * time.Millisecond
	wh.PlayerMovement.players[42].MoveSpeed = speed
	step := issuePlayerStepBuffer(t, wh, ses, messages, "RIGHT")
	// The final token's request was delayed in transit. Reusing its old
	// scheduled deadline must not make a newly appended tile instantly due.
	wh.PlayerMovement.players[42].pendingStep.completeAfter = time.Now().Add(-time.Second)
	beforeCompletion := time.Now()
	result := pipelineReply(t, wh, ses, messages, step.StepToken,
		fmt.Sprintf(`,"afterStepToken":%q,"lookAhead":["RIGHT"]`, step.StepToken))
	if len(result.ContinuationSteps) != 1 || result.ContinuationSteps[0].X != 9 {
		t.Fatalf("empty-window refill=%+v", result)
	}
	next := wh.PlayerMovement.players[42].pendingStep
	if next == nil || next.completeAfter.Before(beforeCompletion.Add(speed)) {
		t.Fatalf("late tail compressed the next tile's duration: %+v", next)
	}
	assertStepPosition(t, wh, ses, 8)
	assertStepDaycare(t, wh, rowID, 126)
	deadline := next.completeAfter
	pipelineReply(t, wh, ses, messages, next.token, "")
	if time.Now().Before(deadline) {
		t.Fatal("refilled tile committed before its actual cadence deadline")
	}
	assertStepPosition(t, wh, ses, 9)
	assertStepDaycare(t, wh, rowID, 127)
}
