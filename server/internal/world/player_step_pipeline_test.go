package world

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func pipelineReply(t *testing.T, wh *WorldHandler, ses *session.Session, messages *recordingMessenger, token, extra string) protocol.PlayerStepCompleteResponse {
	t.Helper()
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"pipeline"%s}`, token, extra))
	for _, message := range messages.streams {
		var result protocol.PlayerStepCompleteResponse
		if json.Unmarshal(message.payload, &result) == nil && result.RequestID == "pipeline" {
			if !result.Success {
				t.Fatalf("completion rejected: %s", message.payload)
			}
			return result
		}
	}
	t.Fatal("missing pipeline completion")
	return protocol.PlayerStepCompleteResponse{}
}

func setupStepPipeline(t *testing.T) (*WorldHandler, *session.Session, *recordingMessenger, protocol.PlayerStepResponse) {
	t.Helper()
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,9,8,1,1),(50,10,8,1,1)`)
	wh.ActorManager.InvalidateCollisionMap(50)
	return wh, ses, messages, issueStep(t, wh, ses, messages)
}

func TestPlayerStepPipelineCommitsOnlyCurrentAndKeepsServerPacing(t *testing.T) {
	wh, ses, messages, step := setupStepPipeline(t)
	rowID := seedStepDaycare(t, wh)
	wh.PlayerMovement.players[42].pendingStep.completeAfter = time.Now().Add(30 * time.Millisecond)
	notBefore := wh.PlayerMovement.players[42].pendingStep.completeAfter
	result := pipelineReply(t, wh, ses, messages, step.StepToken, `,"nextDirection":"RIGHT"`)
	if time.Now().Before(notBefore) {
		t.Fatal("early completion bypassed server pacing")
	}
	if result.X != 8 || result.NextStep == nil || result.NextStep.X != 9 || result.NextStep.StepToken == step.StepToken || result.StopPath {
		t.Fatalf("prepared result=%+v", result)
	}
	assertStepPosition(t, wh, ses, 8)
	assertStepDaycare(t, wh, rowID, 126)
	pending := wh.PlayerMovement.players[42].pendingStep
	if pending == nil || pending.token != result.NextStep.StepToken || pending.parentToken != step.StepToken || pending.sourceX != 8 {
		t.Fatalf("pending continuation=%+v", pending)
	}
	final := pipelineReply(t, wh, ses, messages, result.NextStep.StepToken, "")
	if final.X != 9 || final.NextStep != nil {
		t.Fatalf("final=%+v", final)
	}
	assertStepPosition(t, wh, ses, 9)
	assertStepDaycare(t, wh, rowID, 127)
}

func TestPlayerStepPipelineBlockedContinuationDoesNotRejectCommit(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	step := issueStep(t, wh, ses, messages)
	result := pipelineReply(t, wh, ses, messages, step.StepToken, `,"nextDirection":"RIGHT"`)
	if result.X != 8 || result.NextStep != nil {
		t.Fatalf("blocked continuation=%+v", result)
	}
	assertStepPosition(t, wh, ses, 8)
	if wh.PlayerMovement.players[42].pendingStep != nil {
		t.Fatal("blocked continuation installed a token")
	}
}

func TestPlayerStepPipelineCancellationIsExactAndDoesNotMove(t *testing.T) {
	wh, ses, messages, step := setupStepPipeline(t)
	rowID := seedStepDaycare(t, wh)
	result := pipelineReply(t, wh, ses, messages, step.StepToken, `,"nextDirection":"RIGHT"`)
	prepared := result.NextStep.StepToken
	for range 2 {
		cancelled := pipelineReply(t, wh, ses, messages, prepared, `,"cancel":true`)
		if !cancelled.Cancelled || cancelled.X != 8 || cancelled.NextStep != nil {
			t.Fatalf("cancel=%+v", cancelled)
		}
		assertStepPosition(t, wh, ses, 8)
		assertStepDaycare(t, wh, rowID, 126)
	}
	if wh.PlayerMovement.players[42].pendingStep != nil {
		t.Fatal("cancel did not release movement")
	}
	// Facing and field actions must not remain blocked for the token's lifetime.
	battleDispatch(t, wh, ses, opcodes.PlayerFacingRequest, `{"mapId":50,"fromX":8,"fromY":8,"direction":"LEFT","requestId":"facing"}`)
	var facing protocol.PlayerFacingResponse
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &facing); err != nil || !facing.Success {
		t.Fatalf("facing after cancellation=%+v %v", facing, err)
	}
	// A late cancel of that old token must leave a newer authorization intact.
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"mapId":50,"fromX":8,"fromY":8,"direction":"RIGHT","requestId":"new"}`)
	current := wh.PlayerMovement.players[42].pendingStep
	pipelineReply(t, wh, ses, messages, prepared, `,"cancel":true`)
	if wh.PlayerMovement.players[42].pendingStep != current {
		t.Fatal("old cancellation consumed a replacement")
	}
	pipelineReply(t, wh, ses, messages, current.token, "")
	assertStepPosition(t, wh, ses, 9)
}

func TestPlayerStepPipelineReceiptReplayRetiresOnlyItsUnobservedChild(t *testing.T) {
	wh, ses, messages, step := setupStepPipeline(t)
	rowID := seedStepDaycare(t, wh)
	result := pipelineReply(t, wh, ses, messages, step.StepToken, `,"nextDirection":"RIGHT"`)
	if result.NextStep == nil {
		t.Fatal("missing prepared continuation")
	}
	replay := pipelineReply(t, wh, ses, messages, step.StepToken, `,"nextDirection":"LEFT"`)
	if !replay.Replayed || replay.NextStep != nil || wh.PlayerMovement.players[42].pendingStep != nil {
		t.Fatalf("replay=%+v", replay)
	}
	assertStepPosition(t, wh, ses, 8)
	assertStepDaycare(t, wh, rowID, 126)
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"mapId":50,"fromX":8,"fromY":8,"direction":"RIGHT","requestId":"new"}`)
	current := wh.PlayerMovement.players[42].pendingStep
	pipelineReply(t, wh, ses, messages, step.StepToken, `,"nextDirection":"RIGHT"`)
	if wh.PlayerMovement.players[42].pendingStep != current {
		t.Fatal("historical replay consumed a newer authorization")
	}
}

func TestPlayerStepPipelineRechecksDynamicCollision(t *testing.T) {
	wh, ses, messages, step := setupStepPipeline(t)
	result := pipelineReply(t, wh, ses, messages, step.StepToken, `,"nextDirection":"RIGHT"`)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'BLOCKER','npc','SPRITE_RED',9,8)`)
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"blocked"}`, result.NextStep.StepToken))
	var failure protocol.PlayerStepError
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure); err != nil || failure.Success || failure.Error == "" {
		t.Fatalf("dynamic blocker=%+v %v", failure, err)
	}
	assertStepPosition(t, wh, ses, 8)
}

func TestPlayerStepPipelineNeverContinuesAcrossPresentationBoundaries(t *testing.T) {
	for name, result := range map[string]movementStepResult{
		"stop":        {StopPath: true},
		"teleport":    {Teleport: true},
		"forced path": {ForcedPath: []PathNode{{X: 1, Y: 1}}},
		"trainer":     {Trainer: &pendingEncounter{}},
		"cutscene":    {Cutscene: &durableCutscene{}},
	} {
		if allowsPlayerStepContinuation(result) {
			t.Fatalf("prepared a continuation across %s", name)
		}
	}
	if !allowsPlayerStepContinuation(movementStepResult{}) {
		t.Fatal("ordinary walking cannot continue")
	}
}
