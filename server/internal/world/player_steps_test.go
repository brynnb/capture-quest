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
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &response); err != nil || response.Success || response.Error == "" {
		t.Fatal("duplicate acknowledged")
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
				wh.PlayerMovement.UpdatePosition(42, 7, 8, 50, "UP")
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
