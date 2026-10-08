package world

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func setupIssuedStep(t *testing.T) (*WorldHandler, *session.Session, *recordingMessenger) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.sessionManager = session.NewSessionManager()
	wh.EventFlags = nil
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	wh.PlayerMovement.players[42].MoveSpeed = time.Millisecond
	ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 50, 7, 8
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ROOM',20,20,0);
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,7,8,1,1),(50,8,8,1,1),(50,7,9,1,0);`)
	if err := wh.ActorManager.ensureWalkableMapLoaded(50); err != nil {
		t.Fatal(err)
	}
	return wh, ses, messages
}

func issueStep(t *testing.T, wh *WorldHandler, ses *session.Session, messages *recordingMessenger) protocol.PlayerStepResponse {
	t.Helper()
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT","requestId":"issue"}`)
	var response protocol.PlayerStepResponse
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &response); err != nil || !response.Success || response.StepToken == "" || response.X != 8 || response.Y != 8 {
		t.Fatalf("issuance=%+v %v", response, err)
	}
	return response
}

func assertStepPosition(t *testing.T, wh *WorldHandler, ses *session.Session, wantX int) {
	t.Helper()
	var x, y, mapID int
	if err := wh.database.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=42`).Scan(&x, &y, &mapID); err != nil || x != wantX || y != 8 || mapID != 50 {
		t.Fatalf("durable position=%d %d %d %v", x, y, mapID, err)
	}
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || x != wantX || y != 8 || mapID != 50 || ses.Client.CharData().X != float64(wantX) {
		t.Fatal("owned position changed unexpectedly")
	}
}

func TestIssuedPlayerStepCommitRollbackRetryAndDuplicate(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	step := issueStep(t, wh, ses, messages)
	assertStepPosition(t, wh, ses, 7) // Acceptance cannot publish or persist movement.
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_step_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late step failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_step_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.x=8) EXECUTE FUNCTION reject_step_commit();`)
	payload := fmt.Sprintf(`{"stepToken":%q,"requestId":"complete"}`, step.StepToken)
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, payload)
	var response protocol.PlayerStepError
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &response); err != nil || response.Success || response.Error == "" || response.X != 7 {
		t.Fatalf("failure=%+v %v", response, err)
	}
	assertStepPosition(t, wh, ses, 7)
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_step_commit ON character_data`)
	step = issueStep(t, wh, ses, messages)
	payload = fmt.Sprintf(`{"stepToken":%q,"requestId":"retry"}`, step.StepToken)
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, payload)
	var success protocol.PlayerStepCompleteResponse
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &success); err != nil || !success.Success || success.X != 8 {
		t.Fatalf("success=%+v %v", success, err)
	}
	assertStepPosition(t, wh, ses, 8)
	// Duplicate acknowledgement cannot execute durable step effects again.
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, payload)
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &success); err != nil || !success.Success || !success.Replayed || success.X != 8 {
		t.Fatalf("duplicate receipt=%+v %v", success, err)
	}
	assertStepPosition(t, wh, ses, 8)
}

func TestIssuedPlayerStepRejectsForgedSourceDirectionAndTarget(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	for _, payload := range []string{
		`{"mapId":60,"fromX":7,"fromY":8,"direction":"RIGHT","requestId":"remote"}`,
		`{"mapId":50,"fromX":6,"fromY":8,"direction":"RIGHT","requestId":"stale"}`,
		`{"mapId":50,"fromX":7,"fromY":8,"direction":"DOWN","requestId":"wall"}`,
		`{"mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT","x":99,"requestId":"forged"}`,
		`{"mapId":50,"fromX":7,"fromY":8,"direction":"INVALID","requestId":"direction"}`,
		`{"mapId":50,"direction":"RIGHT","requestId":"missing"}`,
	} {
		battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, payload)
		var failure protocol.PlayerStepError
		if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure); err != nil || failure.Success || failure.Error == "" {
			t.Fatalf("forged response=%+v %v", failure, err)
		}
		assertStepPosition(t, wh, ses, 7)
	}
}

func TestIssuedPlayerStepRejectsTeleportReplacementAndDynamicBlocker(t *testing.T) {
	for _, kind := range []string{"same-position teleport", "replaced connection", "NPC arrives", "expired"} {
		t.Run(kind, func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			step := issueStep(t, wh, ses, messages)
			switch kind {
			case "same-position teleport":
				stageTestPlayerPosition(wh.PlayerMovement, 42, 7, 8, 50, "UP")
			case "replaced connection":
				wh.PlayerMovement.RegisterPlayer(&session.Session{SessionID: ses.SessionID + 1}, 42, 7, 8, 50, "UP")
			case "NPC arrives":
				testdb.Exec(t, wh.database, `INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'BLOCKER','npc','SPRITE_RED',8,8)`)
			case "expired":
				wh.PlayerMovement.players[42].pendingStep.issuedAt = time.Now().Add(-11 * time.Second)
			}
			battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"stale"}`, step.StepToken))
			var failure protocol.PlayerStepError
			if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure); err != nil || failure.Success || failure.Error == "" {
				t.Fatalf("stale=%+v %v", failure, err)
			}
			assertStepPosition(t, wh, ses, 7)
		})
	}
}

func TestIssuedPlayerStepCollisionUsesInjectedStorageAndFailsClosed(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	db.GlobalWorldDB = nil
	wh.ActorManager.InvalidateCollisionMap(50)
	issueStep(t, wh, ses, messages)
	wh.PlayerMovement.players[42].pendingStep = nil
	testdb.Exec(t, wh.database, `DROP TABLE phaser_objects CASCADE`)
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT","requestId":"query-error"}`)
	var failure protocol.PlayerStepError
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure); err != nil || failure.Success || failure.Error == "" {
		t.Fatal("collision failure accepted movement")
	}
	assertStepPosition(t, wh, ses, 7)
}

func TestIssuedPlayerStepWaitHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitPlayerStep(ctx, time.Now().Add(time.Second)); err != context.Canceled {
		t.Fatalf("wait=%v", err)
	}
}

func TestIssuedPlayerStepPersistenceCancellationRetainsOwnedPosition(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	step := issueStep(t, wh, ses, messages)
	lock, err := wh.database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`UPDATE character_data SET x=x WHERE id=42`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var completionErr error
	ses.ExecuteCommand(ctx, func() { _, completionErr = wh.PlayerMovement.completePlayerStep(ses, step.StepToken) })
	if completionErr == nil {
		t.Fatal("cancelled completion accepted")
	}
	lock.Rollback()
	assertStepPosition(t, wh, ses, 7)
}

func TestIssuedPlayerStepSharedLedgeAndWaterRules(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	wh.ActorManager.collisionMap[50]["7,9"] = collisionWater
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"mapId":50,"fromX":7,"fromY":8,"direction":"DOWN","requestId":"no-surf"}`)
	var failed protocol.PlayerStepError
	json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failed)
	if failed.Error == "" {
		t.Fatal("water permitted without surfing")
	}
	wh.PlayerMovement.players[42].IsSurfing = true
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"mapId":50,"fromX":7,"fromY":8,"direction":"DOWN","requestId":"surf"}`)
	var water protocol.PlayerStepResponse
	json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &water)
	if !water.Success || water.Y != 9 || water.LedgeJump {
		t.Fatalf("water=%+v", water)
	}
	wh.PlayerMovement.players[42].pendingStep = nil
	wh.PlayerMovement.players[42].IsSurfing = false
	wh.ActorManager.overworldMapIds[50] = true
	wh.ActorManager.collisionMap[50]["7,9"] = collisionBlocked
	wh.ActorManager.collisionMap[50]["7,10"] = collisionLand
	wh.ActorManager.rawFootTileMap[50] = map[string]int{"7,8": 0x2c, "7,9": 0x37}
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"mapId":50,"fromX":7,"fromY":8,"direction":"DOWN","requestId":"ledge"}`)
	var ledge protocol.PlayerStepResponse
	json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &ledge)
	if !ledge.Success || ledge.Y != 10 || !ledge.LedgeJump {
		t.Fatalf("ledge=%+v", ledge)
	}
	assertStepPosition(t, wh, ses, 7)
}

func TestIssuedPlayerStepReplacementRetiresOldToken(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	first := issueStep(t, wh, ses, messages)
	second := issueStep(t, wh, ses, messages)
	if first.StepToken == second.StepToken {
		t.Fatal("replacement reused token")
	}
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"old"}`, first.StepToken))
	assertStepPosition(t, wh, ses, 7)
	if wh.PlayerMovement.players[42].pendingStep == nil || wh.PlayerMovement.players[42].pendingStep.token != second.StepToken {
		t.Fatal("old acknowledgement discarded current step")
	}
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"current"}`, second.StepToken))
	assertStepPosition(t, wh, ses, 8)
}

func TestPlayerFacingUsesOwnedSourceWithoutPersistingOrRunningStep(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_facing_position() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'facing must not write position'; END $$;
 CREATE TRIGGER reject_facing_position BEFORE UPDATE ON character_data FOR EACH ROW EXECUTE FUNCTION reject_facing_position();`)
	battleDispatch(t, wh, ses, opcodes.PlayerFacingRequest, `{"mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT","requestId":"face"}`)
	var result protocol.PlayerFacingResponse
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &result); err != nil || !result.Success || result.Direction != "RIGHT" || result.X != 7 || result.Y != 8 {
		t.Fatalf("facing=%+v %v", result, err)
	}
	assertStepPosition(t, wh, ses, 7)
	if direction, _ := wh.PlayerMovement.GetDirection(42); direction != "RIGHT" {
		t.Fatal("owned facing not changed")
	}
	for _, payload := range []string{
		`{"mapId":50,"fromX":7,"fromY":8,"direction":"UP","x":99,"requestId":"target"}`,
		`{"mapId":51,"fromX":7,"fromY":8,"direction":"UP","requestId":"map"}`,
		`{"mapId":50,"fromX":8,"fromY":8,"direction":"UP","requestId":"source"}`,
		`{"mapId":50,"direction":"UP","requestId":"missing"}`,
		`{"mapId":50,"fromX":7,"fromY":8,"direction":"INVALID","requestId":"dir"}`,
		`{"mapId":50,"fromX":7,"fromY":8,"direction":"UP","requestId":"trailing"}{}`,
	} {
		battleDispatch(t, wh, ses, opcodes.PlayerFacingRequest, payload)
		var failed protocol.PlayerStepError
		if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failed); err != nil || failed.Success || failed.Error == "" {
			t.Fatalf("rejection=%+v %v", failed, err)
		}
		if direction, _ := wh.PlayerMovement.GetDirection(42); direction != "RIGHT" {
			t.Fatal("invalid facing changed state")
		}
		assertStepPosition(t, wh, ses, 7)
	}
}

func TestPlayerFacingCannotReplaceIssuedOrServerMovement(t *testing.T) {
	for _, kind := range []string{"issued step", "server path", "replaced owner"} {
		t.Run(kind, func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			switch kind {
			case "issued step":
				issueStep(t, wh, ses, messages)
			case "server path":
				wh.PlayerMovement.players[42].Path = []PathNode{{X: 8, Y: 8}}
			case "replaced owner":
				wh.PlayerMovement.players[42].SessionID++
			}
			pending := wh.PlayerMovement.players[42].pendingStep
			battleDispatch(t, wh, ses, opcodes.PlayerFacingRequest, `{"mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT","requestId":"face"}`)
			var result protocol.PlayerStepError
			json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &result)
			if result.Success || result.Error == "" {
				t.Fatal("busy/stale facing accepted")
			}
			if wh.PlayerMovement.players[42].pendingStep != pending {
				t.Fatal("facing retired outstanding step")
			}
			if direction, _ := wh.PlayerMovement.GetDirection(42); direction != "UP" {
				t.Fatal("busy/stale facing changed direction")
			}
		})
	}
}

func TestPlayerFacingPreservesAdjacentStrengthBoulderAndQueuedStep(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	wh.EventFlags = NewEventFlagManager(wh.database)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES(70,'STRENGTH','STRENGTH','STRENGTH','NO_ADDITIONAL_EFFECT',80,'NORMAL',100,15);
 UPDATE character_pokemon SET move1_id=70 WHERE character_id=42;
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_RAINBOWBADGE');
 INSERT INTO phaser_objects(id,map_id,x,y,local_x,local_y,name,sprite_name) VALUES(7001,50,8,8,8,8,'FixtureBoulder','SPRITE_BOULDER');
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,9,8,1,1);`)
	if err := wh.EventFlags.LoadFlags(42); err != nil {
		t.Fatal(err)
	}
	battleDispatch(t, wh, ses, opcodes.PlayerFacingRequest, `{"mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT","requestId":"push"}`)
	var result protocol.PlayerFacingResponse
	json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &result)
	if !result.Success || !result.ServerMovementPending {
		t.Fatalf("facing rejected: %+v", result)
	}
	var x, y int
	if err := wh.database.QueryRow(`SELECT x,y FROM character_object_positions WHERE character_id=42 AND object_id=7001`).Scan(&x, &y); err != nil || x != 9 || y != 8 {
		t.Fatalf("boulder=%d,%d %v", x, y, err)
	}
	assertStepPosition(t, wh, ses, 7)
	path := wh.PlayerMovement.players[42].Path
	if len(path) != 1 || path[0].X != 8 || path[0].Y != 8 {
		t.Fatalf("queued step=%+v", path)
	}
}

func TestPlayerStepLifetimeRetiresAtOneExactBoundary(t *testing.T) {
	issuedAt := time.Now()
	pending := &issuedPlayerStep{issuedAt: issuedAt}
	state := &PlayerMovementState{pendingStep: pending}
	if state.activePlayerStep(issuedAt.Add(playerStepLifetime-time.Nanosecond)) != pending {
		t.Fatal("live authorization retired early")
	}
	if state.activePlayerStep(issuedAt.Add(playerStepLifetime)) != nil || state.pendingStep != nil {
		t.Fatal("authorization survived its deadline")
	}
}

func TestIssuedStepCannotCommitUnavailableMapOrErasedTileFromWarmCache(t *testing.T) {
	for _, change := range []string{`DELETE FROM phaser_maps WHERE id=50`, `UPDATE phaser_tiles SET is_tile_erased=1 WHERE map_id=50 AND x=8 AND y=8`} {
		t.Run(change, func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			step := issueStep(t, wh, ses, messages)
			testdb.Exec(t, wh.database, change)
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"requestId":"unavailable","stepToken":%q}`, step.StepToken))
			var reply protocol.PlayerStepError
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success || reply.Error == "" {
				t.Fatal("unavailable catalog committed")
			}
			assertStepPosition(t, wh, ses, 7)
			receipt, err := loadMovementReceipt(context.Background(), wh.database, 42, step.StepToken)
			if err != nil || receipt != nil {
				t.Fatalf("failed completion stored a receipt: %+v %v", receipt, err)
			}
		})
	}
}
