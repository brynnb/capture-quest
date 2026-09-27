package world

import (
	"capturequest/internal/api/opcodes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
)

func TestCutsceneCompletionRollsBackRewardsAndPublishesAfterCommit(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	flag := "DONE"
	script := &CutsceneScript{RequiresFlagAbst: &flag, SetsFlags: []string{flag}, Actions: json.RawMessage(`[{"type":"giveItem","itemId":1,"quantity":2},{"type":"healParty"},{"type":"giveCoins","coins":10}]`)}
	ctx := CutsceneActionContext{Database: database, Session: ses, EventFlags: wh.EventFlags}
	testdb.Exec(t, database, `ALTER TABLE character_event_flags ADD CONSTRAINT reject_done CHECK(flag_name<>'DONE')`)
	if _, _, err := ApplyCutsceneScript(ctx, script, 42); err == nil {
		t.Fatal("accepted completion failure")
	}
	if len(messages.streams) != 0 || wh.EventFlags.CheckFlag(42, flag) {
		t.Fatal("published failed reward")
	}
	var count, hp int
	if err := database.QueryRow(`SELECT count(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("inventory=%d %v", count, err)
	}
	if err := database.QueryRow(`SELECT cur_hp FROM character_pokemon WHERE character_id=42`).Scan(&hp); err != nil || hp != 1 {
		t.Fatalf("party=%d %v", hp, err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM character_coins WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("coins=%d %v", count, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_event_flags DROP CONSTRAINT reject_done`)
	if _, completed, err := ApplyCutsceneScript(ctx, script, 42); err != nil || !completed {
		t.Fatalf("retry completed=%t err=%v", completed, err)
	}
	if !wh.EventFlags.CheckFlag(42, flag) || len(messages.streams) == 0 {
		t.Fatal("committed state not published")
	}
	messages.streams = nil
	if _, completed, err := ApplyCutsceneScript(ctx, script, 42); err != nil || completed {
		t.Fatalf("repeat completed=%t err=%v", completed, err)
	}
	if len(messages.streams) != 0 {
		t.Fatal("repeated completed reward published")
	}
	var quantity int
	if err := database.QueryRow(`SELECT sum(quantity) FROM cq_item_instances WHERE owner_id=42`).Scan(&quantity); err != nil || quantity != 2 {
		t.Fatalf("quantity=%d err=%v", quantity, err)
	}
}

func TestConcurrentCutsceneCompletionAwardsOnce(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	flag := "ONCE"
	script := &CutsceneScript{RequiresFlagAbst: &flag, SetsFlags: []string{flag}, Actions: json.RawMessage(`[{"type":"giveItem","itemId":1}]`)}
	var completed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, done, err := ApplyCutsceneScript(CutsceneActionContext{Database: database}, script, 42)
			if err != nil {
				t.Errorf("completion: %v", err)
			}
			if done {
				completed.Add(1)
			}
		}()
	}
	wg.Wait()
	if completed.Load() != 1 {
		t.Fatalf("completed=%d", completed.Load())
	}
}

func TestBattleScriptFailureRollsBackVictory(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	current := battleTestStart(t, database, true, func(b *pokebattle.BattleState) {
		b.Trainer.PostWinActions = json.RawMessage(`[{"type":"giveItem","itemId":1},{"type":"setFlag","flag":"SCRIPT_DONE"},{"type":"healParty"}]`)
	})
	testdb.Exec(t, database, `ALTER TABLE character_event_flags ADD CONSTRAINT reject_script CHECK(flag_name<>'SCRIPT_DONE')`)
	var result battleTurnResult
	apply := func(tx db.DBTX, next *pokebattle.BattleState) (err error) {
		next.EnemyParty[0].CurHP = 0
		next.Phase = pokebattle.PhaseBattleEnd
		result, err = settleBattleTurn(tx, 42, next, battleTurnResult{})
		return err
	}
	if _, err := pokebattle.CommitBattle(context.Background(), database, 42, current, apply); err == nil {
		t.Fatal("accepted failed script")
	}
	var money, count int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 100 {
		t.Fatalf("money=%d %v", money, err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("inventory=%d %v", count, err)
	}
	saved, err := pokebattle.ResumeBattle(context.Background(), database, 42)
	if err != nil || saved.IsOver() {
		t.Fatalf("battle=%+v %v", saved, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_event_flags DROP CONSTRAINT reject_script`)
	next, err := pokebattle.CommitBattle(context.Background(), database, 42, current, apply)
	if err != nil {
		t.Fatal(err)
	}
	if result.Script == nil || next.PlayerParty[0].CurHP != next.PlayerParty[0].MaxHP {
		t.Fatal("script did not join party commit")
	}
}

func TestEventFlagFailureDoesNotPoisonCacheAndBatchIsAtomic(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `ALTER TABLE character_event_flags ADD CONSTRAINT reject_bad CHECK(flag_name<>'BAD')`)
	if err := wh.EventFlags.SetFlagBatch(42, []string{"GOOD", "BAD"}); err == nil {
		t.Fatal("accepted failed batch")
	}
	if wh.EventFlags.CheckFlag(42, "GOOD") || wh.EventFlags.CheckFlag(42, "BAD") {
		t.Fatal("failed flags cached")
	}
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_event_flags WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("partial flags=%d %v", count, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_event_flags DROP CONSTRAINT reject_bad`)
	if err := wh.EventFlags.SetFlag(42, "BAD"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := wh.EventFlags.ToggleFlag(42, "BAD"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if !wh.EventFlags.CheckFlag(42, "BAD") {
		t.Fatal("concurrent toggles lost")
	}
}

func TestCutsceneBattleStartRollsBackWithLaterAction(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	ctx := CutsceneActionContext{Database: database, Session: ses, EventFlags: wh.EventFlags}
	raw := json.RawMessage(`[{"type":"healParty"},{"type":"startWildBattle","pokemonId":129,"level":5},{"type":"setFlag","flag":"STARTED"}]`)
	testdb.Exec(t, database, `ALTER TABLE character_event_flags ADD CONSTRAINT reject_started CHECK(flag_name<>'STARTED')`)
	if _, _, err := ApplyCutsceneActionList(ctx, "TEST", raw, 42); err == nil {
		t.Fatal("accepted failed script after battle start")
	}
	if len(messages.streams) != 0 || getBattle(42) != nil {
		t.Fatal("published battle before script committed")
	}
	if saved, err := pokebattle.LoadBattleState(database, 42); err != nil || saved != nil {
		t.Fatalf("saved uncommitted battle=%+v %v", saved, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_event_flags DROP CONSTRAINT reject_started`)
	if _, _, err := ApplyCutsceneActionList(ctx, "TEST", raw, 42); err != nil {
		t.Fatal(err)
	}
	saved, err := pokebattle.ResumeBattle(context.Background(), database, 42)
	if err != nil || saved == nil || saved.PlayerParty[0].CurHP != saved.PlayerParty[0].MaxHP || getBattle(42) == nil {
		t.Fatalf("start=%+v %v", saved, err)
	}
}

func TestCutsceneCompletionRequiresIssuedSnapshotAndToken(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	script := &CutsceneScript{ScriptLabel: "Reward", Actions: json.RawMessage(`[{"type":"giveItem","itemId":1}]`)}
	// Knowing a real script label grants no authority.
	battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, `{"scriptLabel":"Reward"}`)
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatal("unissued reward granted")
	}
	SendCutsceneToPlayer(ses, script)
	var issued struct {
		CompletionToken string `json:"completionToken"`
	}
	if err := json.Unmarshal(messages.streams[0].payload, &issued); err != nil || issued.CompletionToken == "" {
		t.Fatalf("issued=%+v %v", issued, err)
	}
	// Completion executes the issued snapshot, not later edits to the catalog.
	script.Actions = json.RawMessage(`[{"type":"giveItem","itemId":2}]`)
	request := fmt.Sprintf(`{"scriptLabel":"Reward","completionToken":%q}`, issued.CompletionToken)
	battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, request)
	battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, request)
	var quantity int
	if err := database.QueryRow(`SELECT COALESCE(sum(quantity),0) FROM cq_item_instances WHERE owner_id=42 AND item_id=1`).Scan(&quantity); err != nil || quantity != 1 {
		t.Fatalf("quantity=%d %v", quantity, err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM cq_item_instances WHERE owner_id=42 AND item_id=2`).Scan(&count); err != nil || count != 0 {
		t.Fatal("executed replacement script")
	}
}

func TestIssuedCutsceneFailureKeepsTokenForRetry(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	script := &CutsceneScript{ScriptLabel: "Retry", Actions: json.RawMessage(`[{"type":"giveItem","itemId":1}]`), SetsFlags: []string{"RETRY_DONE"}}
	testdb.Exec(t, database, `ALTER TABLE character_event_flags ADD CONSTRAINT reject_retry CHECK(flag_name<>'RETRY_DONE')`)
	SendCutsceneToPlayer(ses, script)
	var issued struct {
		CompletionToken string `json:"completionToken"`
	}
	if err := json.Unmarshal(messages.streams[0].payload, &issued); err != nil {
		t.Fatal(err)
	}
	request := fmt.Sprintf(`{"scriptLabel":"Retry","completionToken":%q}`, issued.CompletionToken)
	battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, request)
	testdb.Exec(t, database, `ALTER TABLE character_event_flags DROP CONSTRAINT reject_retry`)
	battleDispatch(t, wh, ses, opcodes.CutsceneEndRequest, request)
	var quantity int
	if err := database.QueryRow(`SELECT sum(quantity) FROM cq_item_instances WHERE owner_id=42`).Scan(&quantity); err != nil || quantity != 1 {
		t.Fatalf("retry quantity=%d %v", quantity, err)
	}
}

func TestCutsceneCompletionRechecksDurableEligibility(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	tests := []struct {
		name      string
		configure func(*CutsceneScript)
		prepare   string
	}{
		{"required flag", func(s *CutsceneScript) { v := "MISSING"; s.RequiresFlag = &v }, ""},
		{"required flags", func(s *CutsceneScript) { s.RequiresFlags = []string{"MISSING"} }, ""},
		{"absent flags", func(s *CutsceneScript) { s.RequiresFlagsAbst = []string{"PRESENT"} }, `INSERT INTO character_event_flags VALUES(42,'PRESENT',CURRENT_TIMESTAMP) ON CONFLICT DO NOTHING`},
		{"missing item", func(s *CutsceneScript) { v := 2; s.RequiresItemID = &v }, ""},
		{"caught threshold", func(s *CutsceneScript) { v := 1; s.RequiresCaught = &v }, ""},
		{"money minimum", func(s *CutsceneScript) { v := 101; s.RequiresMoney = &v }, ""},
		{"money exclusive upper", func(s *CutsceneScript) { v := 100; s.RequiresMoneyBelow = &v }, ""},
		{"coin minimum", func(s *CutsceneScript) { v := 1; s.RequiresCoins = &v }, ""},
		{"coin exclusive upper", func(s *CutsceneScript) { v := 1; s.RequiresCoinsBelow = &v }, `INSERT INTO character_coins(character_id,coins) VALUES(42,1)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.prepare != "" {
				testdb.Exec(t, database, tt.prepare)
			}
			script := &CutsceneScript{Actions: json.RawMessage(`[{"type":"giveItem","itemId":1}]`)}
			tt.configure(script)
			if _, completed, err := ApplyCutsceneScript(CutsceneActionContext{Database: database}, script, 42); err != nil || completed {
				t.Fatalf("ineligible completed=%t error=%v", completed, err)
			}
		})
	}
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("ineligible grants=%d %v", count, err)
	}
}

func TestCutsceneEligibilityUsesOwnedInventoryAndExactThresholds(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'READY');
 INSERT INTO character_coins(character_id,coins) VALUES(42,10);
 INSERT INTO character_pokedex(character_id,pokemon_id,seen,caught) VALUES(42,25,1,1);`)
	itemID, money, coins, caught := 2, 100, 10, 1
	flag := "READY"
	script := &CutsceneScript{RequiresFlag: &flag, RequiresItemAbst: &itemID, RequiresMoney: &money, RequiresCoins: &coins, RequiresCaught: &caught, Actions: json.RawMessage(`[{"type":"giveItem","itemId":2}]`)}
	if _, done, err := ApplyCutsceneScript(CutsceneActionContext{Database: database}, script, 42); err != nil || !done {
		t.Fatalf("eligible done=%t %v", done, err)
	}
	if _, done, err := ApplyCutsceneScript(CutsceneActionContext{Database: database}, script, 42); err != nil || done {
		t.Fatalf("owned item did not reject duplicate: %t %v", done, err)
	}
	// A corrupt link to someone else's item must never satisfy ownership.
	testdb.Exec(t, database, `UPDATE cq_item_instances SET owner_id=43 WHERE owner_id=42`)
	script.RequiresItemAbst = nil
	script.RequiresItemID = &itemID
	if _, done, err := ApplyCutsceneScript(CutsceneActionContext{Database: database}, script, 42); err != nil || done {
		t.Fatalf("foreign item authorized reward: %t %v", done, err)
	}
}
