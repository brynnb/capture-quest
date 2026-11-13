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

type moveIntentRequestFixture struct {
	RequestID        string `json:"requestId"`
	MapID            int    `json:"mapId"`
	FromX            int    `json:"fromX"`
	FromY            int    `json:"fromY"`
	Direction        string `json:"direction"`
	CommandToken     string `json:"commandToken"`
	MovementStreamID string `json:"movementStreamId"`
	Sequence         uint64 `json:"sequence"`
}

type moveIntentReplyFixture struct {
	Success          bool   `json:"success"`
	Committed        bool   `json:"committed"`
	StopPath         bool   `json:"stopPath"`
	Replayed         bool   `json:"replayed"`
	RequestID        string `json:"requestId"`
	StepToken        string `json:"stepToken"`
	MapID            int    `json:"mapId"`
	X                int    `json:"x"`
	Y                int    `json:"y"`
	Direction        string `json:"direction"`
	Error            string `json:"error"`
	MovementStreamID string `json:"movementStreamId"`
	MovementSequence uint64 `json:"movementSequence"`
}

type moveIntentFixture struct {
	t        *testing.T
	wh       *WorldHandler
	ses      *session.Session
	messages *recordingMessenger
	stream   string
	token    uint64
}

func newMoveIntentFixture(t *testing.T) *moveIntentFixture {
	t.Helper()
	wh, ses, messages := setupPlayerStepBuffer(t)
	f := &moveIntentFixture{t: t, wh: wh, ses: ses, messages: messages}
	f.reset()
	return f
}

func (f *moveIntentFixture) read(reset bool) moveIntentReplyFixture {
	f.t.Helper()
	payload := fmt.Sprintf(`{"requestId":"intent-position","resetMovementStream":%t}`, reset)
	f.messages.streams = nil
	battleDispatch(f.t, f.wh, f.ses, opcodes.OwnedPlayerPositionRequest, payload)
	for _, message := range f.messages.streams {
		if message.opcode != opcodes.OwnedPlayerPositionResponse {
			continue
		}
		var result moveIntentReplyFixture
		if err := json.Unmarshal(message.payload, &result); err != nil || !result.Success {
			f.t.Fatalf("owned intent state=%s err=%v", message.payload, err)
		}
		return result
	}
	f.t.Fatal("missing owned movement stream")
	return moveIntentReplyFixture{}
}

func (f *moveIntentFixture) reset() moveIntentReplyFixture {
	f.t.Helper()
	old := f.stream
	result := f.read(true)
	if !validMovementToken(result.MovementStreamID) || result.MovementStreamID == old || result.MovementSequence != 0 {
		f.t.Fatalf("reset did not establish a fresh stream at sequence zero: %+v", result)
	}
	f.stream = result.MovementStreamID
	return result
}

func (f *moveIntentFixture) command(sequence uint64, x, y int, direction string) moveIntentRequestFixture {
	f.token++
	return moveIntentRequestFixture{
		RequestID: fmt.Sprintf("intent-%d", f.token),
		MapID:     f.wh.PlayerMovement.players[42].MapID, FromX: x, FromY: y, Direction: direction,
		CommandToken: fmt.Sprintf("%032x", f.token), MovementStreamID: f.stream, Sequence: sequence,
	}
}

func (f *moveIntentFixture) dispatch(ses *session.Session, command moveIntentRequestFixture) moveIntentReplyFixture {
	f.t.Helper()
	payload, err := json.Marshal(command)
	if err != nil {
		f.t.Fatal(err)
	}
	f.messages.streams = nil
	battleDispatch(f.t, f.wh, ses, opcodes.PlayerStepRequest, string(payload))
	for _, message := range f.messages.streams {
		if message.opcode != opcodes.PlayerStepResponse {
			continue
		}
		var result moveIntentReplyFixture
		if err := json.Unmarshal(message.payload, &result); err != nil {
			f.t.Fatal(err)
		}
		if result.RequestID == command.RequestID {
			return result
		}
	}
	f.t.Fatal("missing one-phase intent reply")
	return moveIntentReplyFixture{}
}

func (f *moveIntentFixture) accept(command moveIntentRequestFixture) moveIntentReplyFixture {
	f.t.Helper()
	result := f.dispatch(f.ses, command)
	if !result.Success || !result.Committed || result.StepToken != command.CommandToken {
		f.t.Fatalf("intent not committed: command=%+v reply=%+v", command, result)
	}
	return result
}

func (f *moveIntentFixture) reject(command moveIntentRequestFixture) moveIntentReplyFixture {
	f.t.Helper()
	result := f.dispatch(f.ses, command)
	if result.Success || result.Committed || result.Error == "" {
		f.t.Fatalf("invalid intent accepted: command=%+v reply=%+v", command, result)
	}
	return result
}

func TestPlayerMoveIntentCommitsOnceAndRejectsReplayAfterClosedLoop(t *testing.T) {
	f := newMoveIntentFixture(t)
	rowID := seedStepDaycare(t, f.wh)
	first := f.command(1, 7, 8, "RIGHT")
	reply := f.accept(first)
	if reply.X != 8 || reply.Y != 8 || reply.Replayed {
		t.Fatalf("first committed point=%+v", reply)
	}
	assertStepPosition(t, f.wh, f.ses, 8)
	assertStepDaycare(t, f.wh, rowID, 126)
	replay := f.accept(first)
	if !replay.Replayed {
		t.Fatal("identical latest command did not return its receipt")
	}
	assertStepDaycare(t, f.wh, rowID, 126)

	f.accept(f.command(2, 8, 8, "LEFT"))
	assertStepPosition(t, f.wh, f.ses, 7)
	assertStepDaycare(t, f.wh, rowID, 127)
	// Its source is valid again after the loop; sequence ownership must still
	// prevent this older command from being mistaken for a new movement.
	f.reject(first)
	owned := f.read(false)
	if owned.MovementStreamID != f.stream || owned.MovementSequence != 2 {
		t.Fatalf("obsolete replay retired the live owner: %+v", owned)
	}
	f.accept(f.command(3, 7, 8, "RIGHT"))
	assertStepPosition(t, f.wh, f.ses, 8)
	assertStepDaycare(t, f.wh, rowID, 128)
}

func TestPlayerMoveIntentRejectsOutOfOrderAndAlteredCommandsWithoutLosingOwner(t *testing.T) {
	f := newMoveIntentFixture(t)
	rowID := seedStepDaycare(t, f.wh)
	f.reject(f.command(0, 7, 8, "RIGHT"))
	f.reject(f.command(2, 7, 8, "RIGHT"))
	current := f.command(1, 7, 8, "RIGHT")
	f.accept(current)

	altered := current
	altered.Direction = "LEFT"
	f.reject(altered)
	altered = current
	altered.FromX = 6
	f.reject(altered)
	altered = current
	altered.CommandToken = fmt.Sprintf("%032x", 999)
	f.reject(altered)
	owned := f.read(false)
	if owned.MovementStreamID != f.stream || owned.MovementSequence != 1 {
		t.Fatalf("malformed replay changed ownership: %+v", owned)
	}
	assertStepDaycare(t, f.wh, rowID, 126)
	f.accept(f.command(2, 8, 8, "RIGHT"))
	assertStepPosition(t, f.wh, f.ses, 9)
	assertStepDaycare(t, f.wh, rowID, 127)
}

func TestPlayerMoveIntentResetAndReplacementFenceLateCommands(t *testing.T) {
	for _, replacement := range []string{"reset", "new session"} {
		t.Run(replacement, func(t *testing.T) {
			f := newMoveIntentFixture(t)
			rowID := seedStepDaycare(t, f.wh)
			first := f.command(1, 7, 8, "RIGHT")
			f.accept(first)
			late := f.command(2, 8, 8, "LEFT")
			oldSession := f.ses
			if replacement == "new session" {
				next := f.wh.sessionManager.CreateNextSession(f.messages, "", nil)
				next.Client, next.Authenticated = f.ses.Client, true
				t.Cleanup(next.Close)
				f.wh.PlayerMovement.RegisterPlayer(next, 42, 8, 8, 50, "RIGHT")
				f.wh.PlayerMovement.players[42].MoveSpeed = time.Millisecond
				f.ses = next
			}
			f.reset()
			currentStream := f.stream
			// Even the newest durable receipt of the old stream must not be
			// accepted as a command belonging to its replacement.
			for _, old := range []moveIntentRequestFixture{first, late} {
				result := f.dispatch(oldSession, old)
				if result.Success || result.Error == "" {
					t.Fatalf("late old owner accepted: %+v", result)
				}
			}
			owned := f.read(false)
			if owned.MovementStreamID != currentStream || owned.MovementSequence != 0 {
				t.Fatalf("old command retired the replacement: %+v", owned)
			}
			assertStepPosition(t, f.wh, f.ses, 8)
			assertStepDaycare(t, f.wh, rowID, 126)
			f.accept(f.command(1, 8, 8, "LEFT"))
			assertStepPosition(t, f.wh, f.ses, 7)
			assertStepDaycare(t, f.wh, rowID, 127)
		})
	}
}

func TestPlayerMoveIntentRejectedNextInputRetiresItsSuffix(t *testing.T) {
	for _, cause := range []string{"source changed", "dynamic collision", "commit rollback"} {
		t.Run(cause, func(t *testing.T) {
			f := newMoveIntentFixture(t)
			rowID := seedStepDaycare(t, f.wh)
			f.accept(f.command(1, 7, 8, "RIGHT"))
			rejected := f.command(2, 8, 8, "RIGHT")
			switch cause {
			case "source changed":
				rejected.FromX = 7
			case "dynamic collision":
				testdb.Exec(t, f.wh.database, `INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'BLOCKER','npc','SPRITE_RED',9,8)`)
			case "commit rollback":
				testdb.Exec(t, f.wh.database, `CREATE FUNCTION reject_move_intent() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'intent rollback'; END $$;
 CREATE CONSTRAINT TRIGGER reject_move_intent AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.x=9) EXECUTE FUNCTION reject_move_intent();`)
			}
			f.reject(rejected)
			switch cause {
			case "dynamic collision":
				testdb.Exec(t, f.wh.database, `DELETE FROM phaser_objects WHERE id=777`)
			case "commit rollback":
				testdb.Exec(t, f.wh.database, `DROP TRIGGER reject_move_intent ON character_data`)
			}
			// The tile is valid again, but already queued commands from this
			// failed prediction chain still cannot move or rerun counters.
			f.reject(f.command(3, 8, 8, "RIGHT"))
			f.reject(rejected)
			assertStepPosition(t, f.wh, f.ses, 8)
			assertStepDaycare(t, f.wh, rowID, 126)
			f.reset()
			f.accept(f.command(1, 8, 8, "RIGHT"))
			assertStepPosition(t, f.wh, f.ses, 9)
			assertStepDaycare(t, f.wh, rowID, 127)
		})
	}
}

func TestPlayerMoveIntentEffectBoundariesCommitOnceAndRetireFutureInputs(t *testing.T) {
	for _, kind := range []string{"trainer", "cutscene", "spin"} {
		t.Run(kind, func(t *testing.T) {
			f := newMoveIntentFixture(t)
			rowID := seedStepDaycare(t, f.wh)
			wantExperience := 126
			switch kind {
			case "trainer":
				f.wh.EventFlags = NewEventFlagManager(f.wh.database)
				f.wh.TrainerEncounter = NewTrainerEncounterManager(f.wh)
				f.wh.TrainerEncounter.byMap[50] = []*trainerSightData{{ObjectID: 77, MapID: 50, X: 8, Y: 7, Direction: "DOWN", SightRange: 1, RuntimeActorID: 707, Name: "TEST"}}
				wantExperience = 125
			case "cutscene":
				required := "INTENT_STEP_ALLOWED"
				testdb.Exec(t, f.wh.database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,$1)`, required)
				f.wh.EventFlags = NewEventFlagManager(f.wh.database)
				f.wh.CoordTriggers = NewCoordinateTriggerManager(f.wh.database)
				f.wh.CoordTriggers.byMap[50] = map[string][]CoordinateTrigger{tileKey(8, 8): {{MapID: 50, MapName: "ROOM", Label: "INTENT_STEP", X: 8, Y: 8}}}
				f.wh.Cutscenes = NewCutsceneManager(f.wh.database)
				f.wh.Cutscenes.byLabel["INTENT_STEP"] = &CutsceneScript{ScriptLabel: "INTENT_STEP", MapName: "ROOM", TriggerType: "coord", RequiresFlag: &required, Actions: json.RawMessage(`[]`)}
			case "spin":
				f.wh.Cutscenes = NewCutsceneManager(f.wh.database)
				f.wh.Cutscenes.mapIDToName[50] = "ROOM"
				f.wh.SpinTiles = NewSpinTileManager(f.wh.database)
				f.wh.SpinTiles.byMap["ROOM"] = map[string]*SpinTile{"8,8": {MapName: "ROOM", X: 8, Y: 8, Movements: []SpinMovement{{Direction: "RIGHT", Count: 2}}}}
			}
			first := f.command(1, 7, 8, "RIGHT")
			result := f.accept(first)
			if !result.StopPath {
				t.Fatalf("effect boundary did not stop predicted input: %+v", result)
			}
			// Retirement also fences an exact duplicate of the boundary; the
			// durable receipt does not reauthorize its ended input stream.
			f.reject(first)
			f.reject(f.command(2, 8, 8, "RIGHT"))
			f.reject(f.command(3, 9, 8, "RIGHT"))
			assertStepPosition(t, f.wh, f.ses, 8)
			assertStepDaycare(t, f.wh, rowID, wantExperience)
			if kind == "spin" {
				path := f.wh.PlayerMovement.players[42].Path
				if len(path) != 2 || path[0].X != 9 || path[1].X != 10 {
					t.Fatalf("rejected future input changed the forced route: %+v", path)
				}
			} else {
				table := "character_trainer_encounters"
				if kind == "cutscene" {
					table = "character_cutscene_plans"
				}
				var plans int
				if err := f.wh.database.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE character_id=42").Scan(&plans); err != nil || plans != 1 {
					t.Fatalf("effect plans=%d err=%v", plans, err)
				}
			}
		})
	}
}

func TestPlayerMoveIntentLostReplyReadRecoversWithoutReplayingMovement(t *testing.T) {
	f := newMoveIntentFixture(t)
	rowID := seedStepDaycare(t, f.wh)
	first := f.command(1, 7, 8, "RIGHT")
	f.accept(first) // Treat this publication as lost to the browser.
	owned := f.read(false)
	if owned.X != 8 || owned.Y != 8 || owned.MovementStreamID != f.stream || owned.MovementSequence != 1 {
		t.Fatalf("owned recovery did not expose the committed sequence: %+v", owned)
	}
	assertStepDaycare(t, f.wh, rowID, 126)
	f.reset()
	f.reject(first)
	f.accept(f.command(1, 8, 8, "RIGHT"))
	assertStepPosition(t, f.wh, f.ses, 9)
	assertStepDaycare(t, f.wh, rowID, 127)
}

type moveIntentCompletion struct {
	step     *issuedPlayerStep
	replayed bool
	err      error
}

// Observe the real scheduled deadline while the command is pacing. The original
// issued step remains immutable after completion; the returned completion copy
// instead records the actual commit start. This proves exact timing without a
// fragile upper bound on the host scheduler or PostgreSQL execution time.
func beginPacedMoveIntent(t *testing.T, f *moveIntentFixture, command moveIntentRequestFixture) (*issuedPlayerStep, <-chan moveIntentCompletion) {
	t.Helper()
	payload, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	var request protocol.PlayerStepRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	done := make(chan moveIntentCompletion, 1)
	go func() {
		step, replayed, err := f.wh.PlayerMovement.commitPlayerMoveIntent(f.ses, request)
		done <- moveIntentCompletion{step: step, replayed: replayed, err: err}
	}()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case result := <-done:
			t.Fatalf("command finished before its pacing deadline could be observed: %+v", result)
		case <-timer.C:
			t.Fatal("command never installed its paced step")
		case <-ticker.C:
			f.wh.PlayerMovement.mu.RLock()
			step := f.wh.PlayerMovement.players[42].pendingStep
			f.wh.PlayerMovement.mu.RUnlock()
			if step != nil {
				return step, done
			}
		}
	}
}

func awaitMoveIntent(t *testing.T, done <-chan moveIntentCompletion) moveIntentCompletion {
	t.Helper()
	select {
	case result := <-done:
		return result
	case <-time.After(3 * time.Second):
		t.Fatal("paced movement did not finish")
		return moveIntentCompletion{}
	}
}

func TestPlayerMoveIntentPacesEverySegmentIncludingLedgesAndReset(t *testing.T) {
	for _, fixture := range []struct {
		name       string
		directions []string
		durations  []time.Duration
		reset      bool
	}{
		{"ordinary", []string{"RIGHT", "RIGHT", "RIGHT"}, []time.Duration{200 * time.Millisecond, 200 * time.Millisecond, 200 * time.Millisecond}, false},
		{"initial ledge", []string{"DOWN", "RIGHT"}, []time.Duration{400 * time.Millisecond, 200 * time.Millisecond}, false},
		{"subsequent ledge", []string{"RIGHT", "DOWN"}, []time.Duration{200 * time.Millisecond, 400 * time.Millisecond}, false},
		{"reset retains clock", []string{"RIGHT", "RIGHT"}, []time.Duration{200 * time.Millisecond, 200 * time.Millisecond}, true},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			f := newMoveIntentFixture(t)
			testdb.Exec(t, f.wh.database, `UPDATE phaser_maps SET is_overworld=1 WHERE id=50;
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,8,9,1,0),(50,7,10,1,1),(50,8,10,1,1);
 UPDATE phaser_tiles SET map_id=NULL WHERE map_id=50;
 UPDATE phaser_tiles SET raw_foot_tile_id=44 WHERE map_id IS NULL AND x IN(7,8) AND y=8;
 UPDATE phaser_tiles SET raw_foot_tile_id=55 WHERE map_id IS NULL AND x IN(7,8) AND y=9;`)
			f.wh.ActorManager.overworldMapIds[50] = true
			f.wh.ActorManager.InvalidateCollisionMap(50)
			if err := f.wh.ActorManager.ensureWalkableMapLoaded(UnifiedOverworldMapID); err != nil {
				t.Fatal(err)
			}
			f.wh.PlayerMovement.RegisterPlayer(f.ses, 42, 7, 8, UnifiedOverworldMapID, "UP")
			f.wh.PlayerMovement.players[42].MoveSpeed = 200 * time.Millisecond
			f.reset()
			x, y, sequence := 7, 8, uint64(1)
			var previous time.Time
			for i, direction := range fixture.directions {
				if i > 0 && fixture.reset {
					f.reset()
					if got := f.wh.PlayerMovement.players[42].movementCommitStartedAt; !got.Equal(previous) {
						t.Fatalf("reset erased the previous paced commit: got %v want %v", got, previous)
					}
					sequence = 1
				}
				command := f.command(sequence, x, y, direction)
				scheduled, done := beginPacedMoveIntent(t, f, command)
				result := awaitMoveIntent(t, done)
				if result.err != nil || result.step == nil || result.replayed {
					t.Fatalf("paced command failed: %+v", result)
				}
				expected := scheduled.issuedAt.Add(fixture.durations[i])
				if i > 0 && !fixture.reset {
					expected = previous.Add(fixture.durations[i])
				} else if deadline := previous.Add(fixture.durations[i]); deadline.After(expected) {
					expected = deadline
				}
				if !scheduled.completeAfter.Equal(expected) {
					t.Fatalf("segment %d deadline=%v, want exactly %v (no admission delay or speedup)", i, scheduled.completeAfter, expected)
				}
				if result.step.completeAfter.Before(expected) {
					t.Fatalf("segment %d committed at %v before paced deadline %v", i, result.step.completeAfter, expected)
				}
				previous = result.step.completeAfter
				state := f.wh.PlayerMovement.players[42]
				if !state.movementCommitStartedAt.Equal(previous) || state.movementSequence != sequence || state.pendingStep != nil {
					t.Fatalf("completed command left incorrect pacing/sequence state: %+v", state)
				}
				x, y = result.step.x, result.step.y
				sequence++
			}
			var storedX, storedY, storedMap int
			if err := f.wh.database.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=42`).Scan(&storedX, &storedY, &storedMap); err != nil || storedX != x || storedY != y || storedMap != UnifiedOverworldMapID {
				t.Fatalf("paced durable endpoint=%d,%d map=%d err=%v", storedX, storedY, storedMap, err)
			}
		})
	}
}

func TestPlayerMoveIntentRechecksCollisionDuringPacingAndRetiresQueuedInput(t *testing.T) {
	f := newMoveIntentFixture(t)
	rowID := seedStepDaycare(t, f.wh)
	f.wh.PlayerMovement.players[42].MoveSpeed = 200 * time.Millisecond
	first := f.command(1, 7, 8, "RIGHT")
	_, done := beginPacedMoveIntent(t, f, first)
	// Admission saw an empty tile. A moving NPC occupies it before the commit.
	testdb.Exec(t, f.wh.database, `INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'BLOCKER','npc','SPRITE_RED',8,8)`)
	result := awaitMoveIntent(t, done)
	if result.err == nil || result.step != nil {
		t.Fatalf("collision introduced during pacing was accepted: %+v", result)
	}
	testdb.Exec(t, f.wh.database, `DELETE FROM phaser_objects WHERE id=777`)
	f.reject(first)
	f.reject(f.command(2, 7, 8, "RIGHT"))
	assertStepPosition(t, f.wh, f.ses, 7)
	assertStepDaycare(t, f.wh, rowID, 125)
	if state := f.wh.PlayerMovement.players[42]; state.movementStreamID != "" || state.pendingStep != nil {
		t.Fatal("rejected atomic command retained a usable input stream")
	}
	f.reset()
	f.accept(f.command(1, 7, 8, "RIGHT"))
	assertStepPosition(t, f.wh, f.ses, 8)
	assertStepDaycare(t, f.wh, rowID, 126)
}
