package world

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/testdb"
)

func issuedPlanNotify(t *testing.T, messages *recordingMessenger) protocol.CutsceneStartNotify {
	t.Helper()
	var notify protocol.CutsceneStartNotify
	for _, message := range messages.streams {
		if message.opcode == opcodes.CutsceneStartNotify {
			if err := json.Unmarshal(message.payload, &notify); err != nil {
				t.Fatal(err)
			}
		}
	}
	if notify.CompletionToken == "" {
		t.Fatal("no issued cutscene")
	}
	return notify
}

func TestDurableCutsceneFreshOwnerResumesSnapshotAndReplaysCurrentPosition(t *testing.T) {
	wh, old, messages := setupIssuedStep(t)
	script := &CutsceneScript{ScriptLabel: "Durable", MapName: "ROOM", Actions: json.RawMessage(`[{"type":"movePlayer","movements":["RIGHT"]},{"type":"giveItem","itemId":1}]`)}
	SendCutsceneToPlayer(old, script, wh)
	notify := issuedPlanNotify(t, messages)
	old.Close()
	next := wh.sessionManager.CreateNextSession(messages, "", nil)
	next.Authenticated, next.Client = true, old.Client
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.PlayerMovement.RegisterPlayer(next, 42, 7, 8, 50, "UP")
	script.Actions = json.RawMessage(`[{"type":"giveItem","itemId":2}]`)
	messages.streams = nil
	battleDispatch(t, wh, next, opcodes.OwnedPlayerPositionRequest, `{"requestId":"resume"}`)
	resumed := issuedPlanNotify(t, messages)
	if resumed.CompletionToken != notify.CompletionToken || string(resumed.Actions) != string(notify.Actions) {
		t.Fatal("recovery replaced issued snapshot/token")
	}
	request := fmt.Sprintf(`{"requestId":"complete","scriptLabel":"Durable","completionToken":%q}`, notify.CompletionToken)
	battleDispatch(t, wh, next, opcodes.CutsceneEndRequest, request)
	assertStepPosition(t, wh, next, 8)
	if _, err := setServerTeleportedPlayerPosition(next, wh, 50, 7, 8, "LEFT"); err != nil {
		t.Fatal(err)
	}
	// Replay must not even fire the shared no-op character UPDATE lock.
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_receipt_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'receipt attempted character write'; END $$; CREATE CONSTRAINT TRIGGER reject_receipt_write AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_receipt_write();`)
	messages.streams = nil
	battleDispatch(t, wh, next, opcodes.CutsceneEndRequest, request)
	var result protocol.CutsceneEndResponse
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &result); err != nil || !result.Success || !result.Completed || !result.Replayed || result.X != 7 {
		t.Fatalf("replay=%+v err=%v", result, err)
	}
	var quantity, foreign int
	if err := wh.database.QueryRow(`SELECT COALESCE(SUM(quantity) FILTER(WHERE item_id=1),0),COALESCE(SUM(quantity) FILTER(WHERE item_id=2),0) FROM cq_item_instances WHERE owner_id=42`).Scan(&quantity, &foreign); err != nil || quantity != 1 || foreign != 0 {
		t.Fatalf("replayed rewards %d %d %v", quantity, foreign, err)
	}
}

func TestDurableCutsceneLateResolutionFailureRollsBackRewardsAndPosition(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	script := &CutsceneScript{ScriptLabel: "Late", Actions: json.RawMessage(`[{"type":"giveItem","itemId":1},{"type":"movePlayer","movements":["RIGHT"]}]`)}
	SendCutsceneToPlayer(ses, script, wh)
	notify := issuedPlanNotify(t, messages)
	request := fmt.Sprintf(`{"requestId":"end","scriptLabel":"Late","completionToken":%q}`, notify.CompletionToken)
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_plan_resolution() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late cutscene resolution'; END $$; CREATE CONSTRAINT TRIGGER reject_plan_resolution AFTER UPDATE ON character_cutscene_plans DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.resolution='resolved') EXECUTE FUNCTION reject_plan_resolution();`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, request)
	assertStepPosition(t, wh, ses, 7)
	var resolution string
	var count int
	if err := wh.database.QueryRow(`SELECT resolution FROM character_cutscene_plans WHERE completion_token=$1`, notify.CompletionToken).Scan(&resolution); err != nil || resolution != "pending" {
		t.Fatalf("failed resolution %s %v", resolution, err)
	}
	if err := wh.database.QueryRow(`SELECT count(*) FROM cq_item_instances WHERE owner_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed receipt published rewards")
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_plan_resolution ON character_cutscene_plans`)
	battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, request)
	assertStepPosition(t, wh, ses, 8)
}

func TestDurableCutsceneConcurrentCompletionAndBoundedPlans(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	script := &CutsceneScript{ScriptLabel: "Concurrent", Actions: json.RawMessage(`[{"type":"giveItem","itemId":1}]`)}
	SendCutsceneToPlayer(ses, script, wh)
	notify := issuedPlanNotify(t, messages)
	SendCutsceneToPlayer(ses, script, wh)
	if issuedPlanNotify(t, messages).CompletionToken != notify.CompletionToken {
		t.Fatal("duplicate pending issuance replaced token")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := runCutsceneMutation(context.Background(), CutsceneActionContext{Database: wh.database, issuedCompletion: &cutsceneCompletion{Token: notify.CompletionToken, Label: "Concurrent"}}, "", nil, 42, nil)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var quantity int
	if err := wh.database.QueryRow(`SELECT SUM(quantity) FROM cq_item_instances WHERE owner_id=42 AND item_id=1`).Scan(&quantity); err != nil || quantity != 1 {
		t.Fatalf("concurrent rewards %d %v", quantity, err)
	}
	// Terminal outcomes and pending authority have independent bounded retention.
	SendCutsceneToPlayer(ses, &CutsceneScript{ScriptLabel: "LateTerminal", Actions: json.RawMessage(`[]`)}, wh)
	late := issuedPlanNotify(t, messages)
	for i := 0; i < 10; i++ {
		token := ""
		err := db.Transaction(context.Background(), wh.database, func(tx db.DBTX) error {
			if err := db.LockCharacter(tx, 42); err != nil {
				return err
			}
			plan, err := issueCutsceneIn(tx, 42, issuedCutscene{Script: CutsceneScript{ScriptLabel: fmt.Sprintf("Terminal%d", i), Actions: json.RawMessage(`[]`)}, MapID: 50, X: 7, Y: 8})
			if err == nil {
				token = plan.Token
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = runCutsceneMutation(context.Background(), CutsceneActionContext{Database: wh.database, issuedCompletion: &cutsceneCompletion{Token: token, Label: fmt.Sprintf("Terminal%d", i)}}, "", nil, 42, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		completion := &cutsceneCompletion{Token: late.CompletionToken, Label: "LateTerminal"}
		_, completed, err := runCutsceneMutation(context.Background(), CutsceneActionContext{Database: wh.database, issuedCompletion: completion}, "", nil, 42, nil)
		if err != nil || !completed || completion.Replayed != (i == 1) {
			t.Fatalf("late result pruned on completion: completed=%t replay=%t %v", completed, completion.Replayed, err)
		}
	}
	for i := 0; i < 9; i++ {
		err := db.Transaction(context.Background(), wh.database, func(tx db.DBTX) error {
			if err := db.LockCharacter(tx, 42); err != nil {
				return err
			}
			_, err := issueCutsceneIn(tx, 42, issuedCutscene{Script: CutsceneScript{ScriptLabel: fmt.Sprintf("Pending%d", i), Actions: json.RawMessage(`[]`)}, MapID: 50, X: 7, Y: 8})
			return err
		})
		if (i < 8 && err != nil) || (i == 8 && err == nil) {
			t.Fatalf("pending admission %d: %v", i, err)
		}
	}
	var pending, total int
	if err := wh.database.QueryRow(`SELECT COUNT(*) FILTER(WHERE resolution='pending'),COUNT(*) FROM character_cutscene_plans WHERE character_id=42`).Scan(&pending, &total); err != nil || pending != 8 || total != 16 {
		t.Fatalf("retention pending=%d total=%d %v", pending, total, err)
	}
	if _, err := setServerTeleportedPlayerPosition(ses, wh, 50, 8, 8, "RIGHT"); err != nil {
		t.Fatal(err)
	}
	if err := wh.database.QueryRow(`SELECT COUNT(*) FROM character_cutscene_plans WHERE character_id=42 AND resolution='pending'`).Scan(&pending); err != nil || pending != 0 {
		t.Fatal("teleport left incompatible pending authority")
	}
	testdb.Exec(t, wh.database, `DROP TABLE character_cutscene_plans`)
	if err := requireCutsceneIssuanceSchema(context.Background(), wh.database); err == nil {
		t.Fatal("missing issuance schema accepted")
	}
}

func TestDurableCutsceneCancellationPreservesRewardsAndRetiresPendingInputGate(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	script := &CutsceneScript{ScriptLabel: "Declined", Actions: json.RawMessage(`[{"type":"giveItem","itemId":1}]`)}
	SendCutsceneToPlayer(ses, script, wh)
	notify := issuedPlanNotify(t, messages)
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"requestId":"blocked","mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT"}`)
	var denied protocol.PlayerStepError
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &denied); err != nil || denied.Success || denied.Error == "" {
		t.Fatalf("pending input accepted %+v %v", denied, err)
	}
	cancel := fmt.Sprintf(`{"requestId":"cancel","scriptLabel":"Declined","completionToken":%q,"cancel":true}`, notify.CompletionToken)
	for i := 0; i < 2; i++ {
		battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, cancel)
		var reply protocol.CutsceneEndResponse
		if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &reply); err != nil || !reply.Success || reply.Completed || reply.Replayed != (i == 1) {
			t.Fatalf("cancel %+v %v", reply, err)
		}
	}
	normal := fmt.Sprintf(`{"requestId":"forged","scriptLabel":"Declined","completionToken":%q}`, notify.CompletionToken)
	battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, normal)
	var count int
	if err := wh.database.QueryRow(`SELECT COUNT(*) FROM cq_item_instances WHERE owner_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatal("cancelled plan granted reward")
	}
	battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"requestId":"walking","mapId":50,"fromX":7,"fromY":8,"direction":"RIGHT"}`)
	var accepted protocol.PlayerStepResponse
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &accepted); err != nil || !accepted.Success {
		t.Fatalf("cancel did not release input %+v %v", accepted, err)
	}
}
