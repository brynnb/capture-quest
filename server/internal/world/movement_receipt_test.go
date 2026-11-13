package world

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestMovementReceiptSurvivesLostPublicationAndFreshOwner(t *testing.T) {
	wh, old, messages := setupIssuedStep(t)
	rowID := seedStepDaycare(t, wh)
	issued := issueStep(t, wh, old, messages)
	// Stop exactly between durable commit and transport/cache publication.
	_, err := commitMovementStep(context.Background(), wh, 42, movementStepCandidate{StepToken: issued.StepToken, SourceMap: 50, SourceX: 7, SourceY: 8, MapID: 50, X: 8, Y: 8, Direction: "RIGHT"})
	if err != nil {
		t.Fatal(err)
	}
	old.Close()
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	next := wh.sessionManager.CreateNextSession(messages, "", nil)
	next.Client, next.Authenticated = old.Client, true
	wh.PlayerMovement.RegisterPlayer(next, 42, 8, 8, 50, "RIGHT")
	publishCommittedPlayerLocation(next, wh, 50, 8, 8)
	messages.streams = nil
	battleDispatch(t, wh, next, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"new-owner"}`, issued.StepToken))
	if len(messages.streams) != 1 {
		t.Fatalf("replay published %d messages", len(messages.streams))
	}
	var replay protocol.PlayerStepCompleteResponse
	if err := json.Unmarshal(messages.streams[0].payload, &replay); err != nil || !replay.Success || !replay.Replayed || replay.RequestID != "new-owner" || replay.X != 8 {
		t.Fatalf("replay=%+v %v", replay, err)
	}
	assertStepDaycare(t, wh, rowID, 126)
	assertStepPosition(t, wh, next, 8)
	// A subsequent teleport must not turn a historical receipt into a position write.
	if _, err := setServerTeleportedPlayerPosition(next, wh, 50, 7, 8, "LEFT"); err != nil {
		t.Fatal(err)
	}
	messages.streams = nil
	battleDispatch(t, wh, next, opcodes.OwnedPlayerPositionRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"read"}`, issued.StepToken))
	var owned protocol.OwnedPlayerPositionResponse
	if err := json.Unmarshal(messages.streams[0].payload, &owned); err != nil || !owned.Success || owned.X != 7 || owned.CommittedStep == nil || owned.CommittedStep.X != 8 {
		t.Fatalf("owned/current receipt=%+v %v", owned, err)
	}
	battleDispatch(t, wh, next, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"old-retry"}`, issued.StepToken))
	assertStepPosition(t, wh, next, 7)
	assertStepDaycare(t, wh, rowID, 126)
}

func TestMovementReceiptFailureRollsBackStepAndKeepsPreviousReceipt(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	rowID := seedStepDaycare(t, wh)
	previous := strings.Repeat("a", 32)
	testdb.Exec(t, wh.database, `INSERT INTO character_movement_receipts(character_id,step_token,result_json) VALUES(42,$1,$2)`, previous, fmt.Sprintf(`{"version":1,"result":{"stepToken":%q,"mapId":50,"x":7,"y":8,"direction":"UP"}}`, previous))
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_step_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late receipt failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_step_receipt AFTER UPDATE ON character_movement_receipts DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_step_receipt();`)
	step := issueStep(t, wh, ses, messages)
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"failed-receipt"}`, step.StepToken))
	assertStepPosition(t, wh, ses, 7)
	assertStepDaycare(t, wh, rowID, 125)
	assertNoStepSuccess(t, messages)
	receipt, err := loadMovementReceipt(context.Background(), wh.database, 42, step.StepToken)
	if err != nil || receipt != nil {
		t.Fatalf("rolled back receipt=%+v %v", receipt, err)
	}
	receipt, err = loadMovementReceipt(context.Background(), wh.database, 42, previous)
	if err != nil || receipt == nil || receipt.X != 7 {
		t.Fatalf("previous receipt=%+v %v", receipt, err)
	}
}

func TestMovementReceiptIsCharacterScopedAndRejectsMalformedRecords(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	step := issueStep(t, wh, ses, messages)
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"commit"}`, step.StepToken))
	testdb.Exec(t, wh.database, `INSERT INTO character_data(id,name) VALUES(43,'other')`)
	receipt, err := loadMovementReceipt(context.Background(), wh.database, 43, step.StepToken)
	if err != nil || receipt != nil {
		t.Fatalf("cross-character receipt=%+v %v", receipt, err)
	}
	for _, raw := range []string{
		`not-json`,
		fmt.Sprintf(`{"version":1,"result":{"stepToken":%q,"mapId":50,"direction":"RIGHT"}}`, step.StepToken),
		fmt.Sprintf(`{"version":1,"result":{"stepToken":%q,"mapId":50,"x":null,"y":8,"direction":"RIGHT"}}`, step.StepToken),
		fmt.Sprintf(`{"version":1,"result":{"stepToken":%q,"mapId":50,"x":8,"direction":"RIGHT"}}`, step.StepToken),
		fmt.Sprintf(`{"version":99,"result":{"stepToken":%q,"mapId":50,"direction":"RIGHT"}}`, step.StepToken),
		fmt.Sprintf(`{"version":1,"result":{"stepToken":%q,"mapId":50,"direction":"RIGHT"}}`, strings.Repeat("b", 32)),
		fmt.Sprintf(`{"version":1,"result":{"stepToken":%q,"mapId":50,"direction":"sideways"}}`, step.StepToken),
	} {
		testdb.Exec(t, wh.database, `UPDATE character_movement_receipts SET result_json=$1 WHERE character_id=42`, raw)
		if _, err := loadMovementReceipt(context.Background(), wh.database, 42, step.StepToken); err == nil {
			t.Fatalf("accepted malformed receipt %s", raw)
		}
	}
	if _, err := wh.database.Exec(`DELETE FROM character_data WHERE id=42`); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := wh.database.QueryRow(`SELECT COUNT(*) FROM character_movement_receipts`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("orphan receipts=%d %v", remaining, err)
	}
}

func TestWorldConstructionRequiresMovementReceiptSchema(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `DROP TABLE character_movement_receipts`)
	wh, err := NewWorldHandler(context.Background(), session.NewSessionManager())
	if wh != nil || err == nil || !strings.Contains(err.Error(), "preload PlayerMovement") {
		t.Fatalf("construction=%v %v", wh, err)
	}
}
