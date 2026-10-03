package world

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/testdb"
)

func recoveryReply(t *testing.T, messages *recordingMessenger) GameplayStateResponse {
	t.Helper()
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.GameplayStateResponse {
		t.Fatalf("partial recovery publications: %+v", messages.streams)
	}
	var reply GameplayStateResponse
	if err := json.Unmarshal(messages.streams[0].payload, &reply); err != nil || !reply.Success {
		t.Fatalf("recovery %+v %v", reply, err)
	}
	return reply
}

func TestGameplayRecoveryRefreshesBattleFromStorageAndNeverWritesCharacter(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	current := battleTestStart(t, wh.database, true, nil)
	next, err := pokebattle.CommitBattle(context.Background(), wh.database, 42, current, func(_ db.DBTX, b *pokebattle.BattleState) error { b.Phase = pokebattle.PhaseFaintSwitch; return nil })
	if err != nil {
		t.Fatal(err)
	}
	// Cache still points to the older action-select battle: durable state must win.
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_recovery_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'recovery attempted character write'; END $$; CREATE CONSTRAINT TRIGGER reject_recovery_write AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_recovery_write();`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"recover","mapId":50}`)
	reply := recoveryReply(t, messages)
	if reply.Battle == nil || reply.Battle.Phase != "faint_switch" || reply.Battle.Revision != next.Revision || reply.Battle.BattleID != current.BattleID || reply.Battle.PlayerPokemon.CurHP != 1 || getBattle(42).Revision != next.Revision {
		t.Fatalf("battle recovery %+v", reply.Battle)
	}
	if reply.Safari != nil || reply.Trainer != nil || reply.Cutscene != nil || reply.Position.X != 7 {
		t.Fatal("mixed recovery snapshot")
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_recovery_write ON character_data`)
	pending, err := pokebattle.CommitBattle(context.Background(), wh.database, 42, next, func(_ db.DBTX, b *pokebattle.BattleState) error {
		b.Phase = pokebattle.PhaseBattleEnd
		b.PendingMoveLearn = &pokebattle.PendingMove{PokemonIndex: 0, MoveID: 150, MoveName: "Splash"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"choice","mapId":50}`)
	reply = recoveryReply(t, messages)
	if reply.Battle == nil || reply.Battle.Phase != "move_learn_prompt" || reply.Battle.PendingMove == nil || reply.Battle.PendingMove.MoveID != 150 || reply.Battle.Revision != pending.Revision {
		t.Fatalf("pending choice %+v", reply.Battle)
	}
	testdb.Exec(t, wh.database, `DELETE FROM character_battle_state WHERE character_id=42`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"none","mapId":50}`)
	reply = recoveryReply(t, messages)
	if reply.Battle != nil || getBattle(42) != nil {
		t.Fatal("recovery preserved stale battle")
	}
}

func TestGameplayRecoveryReadsSafariBattleCountersAndRejectsCorruptStore(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	wild, err := pokebattle.BuildWildPokemon(wh.database, 129, 5)
	if err != nil {
		t.Fatal(err)
	}
	visit := &SafariSession{Active: true, BallsLeft: 7, StepsLeft: 93, Battle: pokebattle.NewSafariBattle(wild, 7, 93)}
	if err := db.Transaction(context.Background(), wh.database, func(tx db.DBTX) error { return saveSafariSessionIn(tx, 42, visit) }); err != nil {
		t.Fatal(err)
	}
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"safari","mapId":50}`)
	reply := recoveryReply(t, messages)
	if reply.Safari == nil || reply.Safari.Pokemon == nil || reply.Safari.Pokemon.ID != 129 || reply.Safari.BallsLeft != 7 || reply.Safari.StepsLeft != 93 || reply.Battle != nil {
		t.Fatalf("safari recovery %+v", reply.Safari)
	}
	stale := &pokebattle.BattleState{}
	setBattle(42, stale)
	testdb.Exec(t, wh.database, `UPDATE character_safari_state SET state_json='{"version":999,"visit":{}}' WHERE character_id=42`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"bad","mapId":50}`)
	var denied protocol.PlayerStepError
	if len(messages.streams) != 1 {
		t.Fatal("published partial safari state")
	}
	if getBattle(42) != stale {
		t.Fatal("failed recovery changed battle ownership")
	}
	if err := json.Unmarshal(messages.streams[0].payload, &denied); err != nil || denied.Success || denied.RequestID != "bad" || denied.Error == "" {
		t.Fatalf("corrupt recovery %+v %v", denied, err)
	}
}

func TestGameplayRecoveryTrainerPlanThenLostBattleStart(t *testing.T) {
	wh, ses, messages, notify := pendingTrainerFixture(t)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"plan","mapId":50}`)
	reply := recoveryReply(t, messages)
	if reply.Trainer == nil || reply.Trainer.EncounterToken != notify.EncounterToken || reply.Trainer.TrainerActorID != notify.TrainerActorID || reply.Battle != nil {
		t.Fatalf("trainer recovery %+v", reply.Trainer)
	}
	trainerReady(t, wh, ses, notify)
	saved := getBattle(42)
	forgetBattle(42, saved) // Simulate cache loss; discard the already-committed notification.
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"battle","mapId":50}`)
	reply = recoveryReply(t, messages)
	if reply.Trainer != nil || reply.Battle == nil || reply.Battle.BattleID != saved.BattleID || reply.Battle.TrainerClass != "TEST" || reply.Battle.PlayerActive != 0 {
		t.Fatalf("committed trainer recovery %+v", reply)
	}
}

func TestGameplayRecoveryCutsceneSourceAndCancellationDeadline(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	script := &CutsceneScript{ScriptLabel: "Pending", MapName: "ROOM", Actions: json.RawMessage(`[{"type":"giveItem","itemId":1}]`)}
	SendCutsceneToPlayer(ses, script, wh)
	notify := issuedPlanNotify(t, messages)
	script.Actions = json.RawMessage(`[{"type":"giveItem","itemId":2}]`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"script","mapId":50}`)
	reply := recoveryReply(t, messages)
	if reply.Cutscene == nil || reply.Cutscene.CompletionToken != notify.CompletionToken || string(reply.Cutscene.Actions) != string(notify.Actions) {
		t.Fatalf("pending script recovery %+v", reply.Cutscene)
	}
	for _, payload := range []string{`{"requestId":"foreign","mapId":51}`, `{"requestId":"forged","mapId":50,"x":99}`} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, payload)
		var denied protocol.PlayerStepError
		if len(messages.streams) != 1 {
			t.Fatal("invalid recovery published plans")
		}
		if err := json.Unmarshal(messages.streams[0].payload, &denied); err != nil || denied.Success {
			t.Fatalf("invalid recovery accepted %+v %v", denied, err)
		}
	}
	// Invalid JSON is rejected by framing before the handler, with no response.
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"trailing","mapId":50} {}`)
	if len(messages.streams) != 0 {
		t.Fatal("malformed frame entered recovery handler")
	}
	lock, err := wh.database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if err := db.LockCharacter(lock, 42); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, _, err := readGameplayState(ctx, ses, wh, GameplayStateRequest{RequestID: "deadline", MapID: 50}); err == nil {
		t.Fatal("locked recovery ignored cancellation")
	}
	if time.Since(started) > time.Second {
		t.Fatal("recovery exceeded caller cancellation bound")
	}
}
