package world

import (
	"context"
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/testdb"
)

func TestTerminalBattleRecoveryAndAtomicPostBattlePlan(t *testing.T) {
	for _, failCommit := range []bool{false, true} {
		name := "success"
		if failCommit {
			name = "late-plan-failure"
		}
		t.Run(name, func(t *testing.T) {
			wh, ses, messages := setupIssuedStep(t)
			wh.Cutscenes = NewCutsceneManager(wh.database)
			winFlag := "WON_TEST"
			wh.Cutscenes.byMap["ROOM"] = []*CutsceneScript{{ScriptLabel: "TrainerReward", MapName: "ROOM", TriggerType: "map_script", RequiresFlag: &winFlag, Actions: json.RawMessage(`[{"type":"dialogue","text":"Trainer reward."}]`)}}
			current := battleTestStart(t, wh.database, true, nil)
			finished, err := pokebattle.CommitBattle(context.Background(), wh.database, 42, current, func(tx db.DBTX, b *pokebattle.BattleState) error {
				b.EnemyParty[0].CurHP = 0
				b.Phase = pokebattle.PhaseBattleEnd
				if err := writeEventFlag(tx, 42, winFlag, true); err != nil {
					return err
				}
				_, err := tx.Exec(`UPDATE character_wallet SET pokedollars=pokedollars+250 WHERE character_id=42`)
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			forgetBattle(42, getBattle(42)) // Owner/cache loss before terminal delivery.
			testdb.Exec(t, wh.database, `CREATE FUNCTION reject_terminal_read_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'terminal read wrote character'; END $$; CREATE CONSTRAINT TRIGGER reject_terminal_read_write AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_terminal_read_write();`)
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"terminal","current":true}`)
			reply := recoveryReply(t, messages)
			if reply.Battle == nil || !reply.Battle.NeedsDismissal || reply.Battle.BattleID != finished.BattleID || reply.Battle.Revision != finished.Revision || reply.Cutscene != nil {
				t.Fatalf("terminal snapshot %+v", reply)
			}
			testdb.Exec(t, wh.database, `DROP TRIGGER reject_terminal_read_write ON character_data`)
			if failCommit {
				testdb.Exec(t, wh.database, `CREATE FUNCTION reject_terminal_plan() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'plan commit failed'; END $$; CREATE CONSTRAINT TRIGGER reject_terminal_plan AFTER INSERT ON character_cutscene_plans DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_terminal_plan();`)
			}
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.PokeBattleCloseRequest, `{"requestId":"dismiss"}`)
			saved, err := pokebattle.LoadBattleState(wh.database, 42)
			if err != nil {
				t.Fatal(err)
			}
			var count, money int
			if err := wh.database.QueryRow(`SELECT COUNT(*) FROM character_cutscene_plans WHERE character_id=42`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if err := wh.database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil {
				t.Fatal(err)
			}
			if money != 350 {
				t.Fatalf("dismissal repeated rewards: %d", money)
			}
			if failCommit {
				if saved == nil || saved.Revision != finished.Revision || count != 0 || getBattle(42) == nil {
					t.Fatalf("failed dismissal lost authority: saved=%+v plans=%d", saved, count)
				}
				var denied BattleCommandError
				if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &denied) != nil || denied.Success || denied.RequestID != "dismiss" {
					t.Fatalf("partial failure publication %+v", messages.streams)
				}
				return
			}
			if saved != nil || count != 1 || getBattle(42) != nil {
				t.Fatalf("dismissal did not settle: saved=%+v plans=%d", saved, count)
			}
			if len(messages.streams) != 2 || messages.streams[0].opcode != opcodes.PokeBattleCloseResponse || messages.streams[1].opcode != opcodes.CutsceneStartNotify {
				t.Fatalf("committed publication %+v", messages.streams)
			}
			var plan protocol.CutsceneStartNotify
			if err := json.Unmarshal(messages.streams[1].payload, &plan); err != nil {
				t.Fatal(err)
			}
			// Lost acknowledgement and plan notification recover durable plan only.
			messages.streams = nil
			battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"lost-close","current":true}`)
			recovered := recoveryReply(t, messages)
			if recovered.Battle != nil || recovered.Cutscene == nil || recovered.Cutscene.CompletionToken != plan.CompletionToken {
				t.Fatalf("lost-close recovery %+v", recovered)
			}
		})
	}
}
