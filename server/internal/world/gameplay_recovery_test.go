package world

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
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

func TestCapturePlacementRecoversAfterLostReplyWithoutAnotherCatch(t *testing.T) {
	for _, partySize := range []int{1, 6} {
		t.Run(fmt.Sprintf("party_%d", partySize), func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			if partySize == 6 {
				testdb.Exec(t, wh.database, `INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,exp,cur_hp,max_hp) SELECT 42,s,s,25,50,125000,1,95 FROM generate_series(1,5) s; INSERT INTO character_pc_state(character_id,current_box) VALUES(42,3)`)
			}
			var originalIDs int64
			if err := wh.database.QueryRow(`SELECT sum(id) FROM character_pokemon WHERE character_id=42`).Scan(&originalIDs); err != nil {
				t.Fatal(err)
			}
			current := battleTestStart(t, wh.database, false, func(b *pokebattle.BattleState) { b.GuaranteedCatch = true })
			instance, err := cqitems.NewStore(wh.database).AddItemToInventory(42, 2, 1)
			if err != nil {
				t.Fatal(err)
			}
			request := fmt.Sprintf(`{"requestId":"capture","action":"item","instanceId":%d,"itemId":2,"battle":{"battleId":%q,"revision":%d}}`, instance, current.BattleID, current.Revision)
			// Fail after party/PC insertion and item consumption, at battle save.
			testdb.Exec(t, wh.database, `ALTER TABLE character_battle_state ADD CONSTRAINT reject_capture_placement CHECK(battle_json::json->'capture' IS NULL)`)
			battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
			if getBattle(42) != current || current.Capture != nil {
				t.Fatal("failed capture published placement")
			}
			var count, quantity int
			if err := wh.database.QueryRow(`SELECT count(*) FROM character_pokemon WHERE character_id=42 AND pokemon_id=129`).Scan(&count); err != nil || count != 0 {
				t.Fatal("failed capture inserted pokemon", err)
			}
			if err := wh.database.QueryRow(`SELECT quantity FROM cq_item_instances WHERE id=$1`, instance).Scan(&quantity); err != nil || quantity != 1 {
				t.Fatal("failed capture spent ball", err)
			}
			testdb.Exec(t, wh.database, `ALTER TABLE character_battle_state DROP CONSTRAINT reject_capture_placement`)
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
			finished := getBattle(42)
			if finished == nil || !finished.PlayerCaught || finished.Capture == nil || finished.Capture.SentToPC != (partySize == 6) {
				t.Fatal("capture omitted committed placement")
			}
			forgetBattle(42, finished)
			messages.streams = nil // Model lost delivery; recover from durable state.
			battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"caught","current":true}`)
			snapshot := recoveryReply(t, messages).Battle
			if snapshot == nil || !snapshot.Caught || !snapshot.NeedsDismissal || snapshot.Capture == nil || snapshot.Capture.SentToPC != (partySize == 6) || snapshot.Revision != current.Revision+1 {
				t.Fatalf("capture recovery=%+v", snapshot)
			}
			if partySize == 6 && (snapshot.Capture.PCBox != 4 || len(snapshot.PlayerParty) != 6) {
				t.Fatal("recovery guessed PC box or changed full party")
			}
			if partySize == 1 && (len(snapshot.PlayerParty) != 2 || snapshot.PlayerParty[1].ID != 129) {
				t.Fatal("recovery omitted caught party member")
			}
			var caughtID int64
			var storedBox int
			if err := wh.database.QueryRow(`SELECT id,box FROM character_pokemon WHERE character_id=42 AND pokemon_id=129`).Scan(&caughtID, &storedBox); err != nil {
				t.Fatal(err)
			}
			if (partySize == 6 && storedBox != 3) || (partySize == 1 && storedBox != pokebattle.BoxParty) {
				t.Fatalf("capture placement differs from stored row: box=%d", storedBox)
			}
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.PokeBattleActionRequest, request)
			var rejected BattleCommandError
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &rejected) != nil || rejected.Success {
				t.Fatal("duplicate capture accepted")
			}
			var allIDs int64
			if err := wh.database.QueryRow(`SELECT count(*),sum(id) FROM character_pokemon WHERE character_id=42`).Scan(&count, &allIDs); err != nil || count != partySize+1 || allIDs != originalIDs+caughtID {
				t.Fatal("duplicate changed pokemon identities", err)
			}
			if err := wh.database.QueryRow(`SELECT count(*) FROM cq_item_instances WHERE id=$1`, instance).Scan(&count); err != nil || count != 0 {
				t.Fatal("ball consumption did not remain settled", err)
			}
		})
	}
}

func TestMoveChoiceRecoveryAfterLostReplyDoesNotRepeatSettlement(t *testing.T) {
	for _, slot := range []int{0, -1} {
		t.Run(fmt.Sprintf("slot_%d", slot), func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			current := battleTestStart(t, wh.database, false, nil)
			pending, err := pokebattle.CommitBattle(context.Background(), wh.database, 42, current, func(_ db.DBTX, b *pokebattle.BattleState) error {
				b.Phase = pokebattle.PhaseBattleEnd
				b.PendingMoveLearn = &pokebattle.PendingMove{PokemonIndex: 0, MoveID: 150, MoveName: "SPLASH"}
				b.PostMoveLearnEvents = []pokebattle.BattleEvent{{Type: pokebattle.EventMessage, Message: "Settled reward"}}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			var rowID, exp, money int
			if err := wh.database.QueryRow(`SELECT p.id,p.exp,w.pokedollars FROM character_pokemon p JOIN character_wallet w ON w.character_id=p.character_id WHERE p.character_id=42`).Scan(&rowID, &exp, &money); err != nil {
				t.Fatal(err)
			}
			// Retire the cache as owner replacement does. The read must recover the
			// exact pending choice, without making a finished battle dismissible.
			forgetBattle(42, getBattle(42))
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"pending","current":true}`)
			before := recoveryReply(t, messages).Battle
			if before == nil || before.NeedsDismissal || before.Phase != "move_learn_prompt" || before.PendingMove == nil || before.PendingMove.MoveID != 150 || before.PendingMove.PokemonIndex != 0 || before.Revision != pending.Revision {
				t.Fatalf("pending recovery=%+v", before)
			}
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.PokeBattleCloseRequest, `{}`)
			var rejected BattleCommandError
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &rejected) != nil || rejected.Success {
				t.Fatal("dismissed unresolved learning choice")
			}
			// Keep the original identity rather than letting battleDispatch bind a
			// later duplicate to the new revision.
			choice := fmt.Sprintf(`{"requestId":"choice","forgetSlot":%d,"battle":{"battleId":%q,"revision":%d}}`, slot, pending.BattleID, pending.Revision)
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.PokeMoveLearnRequest, choice)
			var committed BattleCommandResponse
			if len(messages.streams) != 2 || json.Unmarshal(messages.streams[0].payload, &committed) != nil || !committed.Success || committed.Learning == nil || committed.Learning.Skipped != (slot == -1) {
				t.Fatalf("learning reply=%+v", committed)
			}
			// Discard delivery and cache. Recovery must come from durable rows,
			// not from the successful reply or a previous owner's battle pointer.
			forgetBattle(42, getBattle(42))
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"settled","current":true}`)
			after := recoveryReply(t, messages).Battle
			if after == nil || !after.NeedsDismissal || after.PendingMove != nil || after.Phase != "battle_end" || after.BattleID != pending.BattleID || after.Revision != pending.Revision+1 {
				t.Fatalf("settled recovery=%+v", after)
			}
			wantMove := 0
			if slot == 0 {
				wantMove = 150
				if len(after.PlayerParty) != 1 || len(after.PlayerParty[0].Moves) == 0 || after.PlayerParty[0].Moves[0].ID != wantMove {
					t.Fatal("recovery omitted committed learned move")
				}
			}
			saved, err := pokebattle.ResumeBattle(context.Background(), wh.database, 42)
			if err != nil || saved == nil || saved.PlayerParty[0].Moves[0].ID != wantMove || saved.PendingMoveLearn != nil || len(saved.PostMoveLearnEvents) != 0 {
				t.Fatalf("settled save=%+v err=%v", saved, err)
			}
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.PokeMoveLearnRequest, choice)
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &rejected) != nil || rejected.Success {
				t.Fatal("accepted delayed duplicate choice")
			}
			var gotID, gotExp, gotMoney, gotMove int
			if err := wh.database.QueryRow(`SELECT p.id,p.exp,w.pokedollars,p.move1_id FROM character_pokemon p JOIN character_wallet w ON w.character_id=p.character_id WHERE p.character_id=42`).Scan(&gotID, &gotExp, &gotMoney, &gotMove); err != nil || gotID != rowID || gotExp != exp || gotMoney != money || gotMove != wantMove {
				t.Fatalf("settlement changed: id=%d exp=%d money=%d move=%d err=%v", gotID, gotExp, gotMoney, gotMove, err)
			}
			saved, err = pokebattle.ResumeBattle(context.Background(), wh.database, 42)
			if err != nil || saved == nil || saved.Revision != after.Revision {
				t.Fatal("duplicate advanced durable revision", err)
			}
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.PokeBattleCloseRequest, `{}`)
			var closed BattleCommandResponse
			if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PokeBattleCloseResponse || json.Unmarshal(messages.streams[0].payload, &closed) != nil || !closed.Success || closed.Battle != nil {
				t.Fatal("settled battle did not dismiss")
			}
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"closed","current":true}`)
			if recoveryReply(t, messages).Battle != nil {
				t.Fatal("dismissed learning battle revived")
			}
		})
	}
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

func TestLastBallSafariCaptureRecoveryRetainsPlacementAndExpiry(t *testing.T) {
	for _, size := range []int{1, 6} {
		t.Run(fmt.Sprintf("party_%d", size), func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			wh.Safari = NewSafariZoneManager(wh.database)
			if size == 6 {
				testdb.Exec(t, wh.database, `INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,exp,cur_hp,max_hp) SELECT 42,s,s,25,50,125000,1,95 FROM generate_series(1,5) s; INSERT INTO character_pc_state(character_id,current_box) VALUES(42,3)`)
			}
			var result safariActionResult
			for i := 0; i < 128; i++ {
				// Use the real capture roll; only reset independent test encounters.
				seedSafariBattle(t, wh.Safari, 1)
				var err error
				result, err = safariFixtureAction(t, wh.Safari, "ball")
				if err != nil {
					t.Fatal(err)
				}
				if result.Battle.Caught {
					break
				}
			}
			if result.Battle == nil || !result.Battle.Caught {
				t.Fatal("no capture reached recovery")
			}
			publishCommittedPlayerPosition(ses, wh, SafariZoneGateMapID, SafariZoneGateReturnX, SafariZoneGateReturnY, "DOWN")
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"expired","current":true}`)
			reply := recoveryReply(t, messages)
			s := reply.Safari
			if s == nil || s.Active || !s.IsOver || !s.Caught || s.BallsLeft != 0 || s.ExitMessage != SafariExpiryMessage || s.BattleID != result.Battle.BattleID || s.Revision != 2 || reply.Position.MapID != SafariZoneGateMapID {
				t.Fatalf("expired capture recovery=%+v position=%+v", s, reply.Position)
			}
			if s.SentToPC != (size == 6) || (size == 6 && (s.PCBox != 4 || len(s.PlayerParty) != 6)) || (size == 1 && (len(s.PlayerParty) != 2 || s.PlayerParty[1].ID != 129)) {
				t.Fatalf("placement/party=%+v", s)
			}
			if _, err := wh.Safari.act(context.Background(), 42, "close", BattleCommandIdentity{BattleID: s.BattleID, Revision: s.Revision}); err != nil {
				t.Fatal(err)
			}
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"closed","current":true}`)
			if recoveryReply(t, messages).Safari != nil {
				t.Fatal("dismissed encounter recovered again")
			}
		})
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

func TestCurrentGameplayRecoveryReturnsOnlyOneCoherentPlanAndKeepsSourceValidation(t *testing.T) {
	wh, ses, messages := setupIssuedStep(t)
	script := &CutsceneScript{ScriptLabel: "PendingCurrent", MapName: "ROOM", Actions: json.RawMessage(`[{"type":"giveItem","itemId":1}]`)}
	SendCutsceneToPlayer(ses, script, wh)
	notify := issuedPlanNotify(t, messages)
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_current_recovery_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'current recovery attempted write'; END $$; CREATE CONSTRAINT TRIGGER reject_current_recovery_write AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_current_recovery_write();`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"current","current":true}`)
	reply := recoveryReply(t, messages)
	if reply.Position.MapID != 50 || reply.Position.X != 7 || reply.Cutscene == nil || reply.Cutscene.CompletionToken != notify.CompletionToken {
		t.Fatalf("current recovery %+v", reply)
	}
	for _, payload := range []string{`{"requestId":"conflict","current":true,"mapId":50}`, `{"requestId":"missing"}`, `{"requestId":"stale","mapId":51}`} {
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, payload)
		var denied protocol.PlayerStepError
		if len(messages.streams) != 1 {
			t.Fatal("invalid selector published independent plans")
		}
		if err := json.Unmarshal(messages.streams[0].payload, &denied); err != nil || denied.Success || denied.Error == "" {
			t.Fatalf("invalid selector %+v %v", denied, err)
		}
	}
	// Current selects the owned map; it cannot bypass saved/owned agreement.
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_current_recovery_write ON character_data; UPDATE character_data SET x=99 WHERE id=42`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"source","current":true}`)
	var denied protocol.PlayerStepError
	if len(messages.streams) != 1 {
		t.Fatal("source mismatch published independent plans")
	}
	if err := json.Unmarshal(messages.streams[0].payload, &denied); err != nil || denied.Success {
		t.Fatalf("source mismatch %+v %v", denied, err)
	}
}
