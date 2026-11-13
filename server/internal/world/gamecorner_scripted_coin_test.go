package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	model "capturequest/internal/db/models"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func scriptedCoinFixture(t *testing.T) (*sql.DB, *WorldHandler, *session.Session, *recordingMessenger, *CutsceneScript) {
	t.Helper()
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
UPDATE character_wallet SET pokedollars=2000 WHERE character_id=42;
INSERT INTO character_coins(character_id,coins) VALUES(42,3);
INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(135,'GAME_CORNER',20,20,0);
UPDATE character_data SET map_id=135,x=5,y=8 WHERE id=42`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 135, 5, 8
	caseID, price, maximum := CoinCaseItemID, CoinPurchasePrice, MaxCoins
	// These are the canonical generated clerk's mutation and admission policies;
	// rendered/scenario tests cover its full native dialogue and choice sequence.
	script := &CutsceneScript{ScriptLabel: "GameCornerClerk1BuyCoins", MapName: "GAME_CORNER", RequiresItemID: &caseID, RequiresMoney: &price, RequiresCoinsBelow: &maximum, Actions: json.RawMessage(`[{"type":"takeMoney","money":1000},{"type":"giveCoins","coins":50}]`)}
	db.GlobalWorldDB = nil
	return database, wh, ses, messages, script
}

func issueScriptedCoins(t *testing.T, database *sql.DB, script *CutsceneScript) *cutsceneCompletion {
	t.Helper()
	var plan *durableCutscene
	err := db.Transaction(context.Background(), database, func(tx db.DBTX) (err error) {
		if err := db.LockCharacter(tx, 42); err != nil {
			return err
		}
		plan, err = issueCutsceneIn(tx, 42, issuedCutscene{Script: *script, MapID: 135, X: 5, Y: 8})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return &cutsceneCompletion{Token: plan.Token, Label: script.ScriptLabel}
}

func assertScriptedCoinBalances(t *testing.T, database *sql.DB, wantMoney, wantCoins int) {
	t.Helper()
	var money, coins int
	if err := database.QueryRow(`SELECT w.pokedollars,c.coins FROM character_wallet w JOIN character_coins c USING(character_id) WHERE character_id=42`).Scan(&money, &coins); err != nil || money != wantMoney || coins != wantCoins {
		t.Fatalf("money=%d coins=%d want=%d/%d err=%v", money, coins, wantMoney, wantCoins, err)
	}
}

func TestGameCornerRetiredCoinLanesNeverReadOrSpend(t *testing.T) {
	messages := &recordingMessenger{}
	ses := &session.Session{Authenticated: true, Client: &testSessionClient{char: &model.CharacterData{ID: 42}}, Messenger: messages}
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	for _, handler := range []func(*session.Session, []byte, *WorldHandler) bool{HandleGameCornerCoinBalance, HandleGameCornerBuyCoins} {
		for _, payload := range []string{`{}`, `{"requestId":"stale","command":{"characterId":42,"revision":0}}`, `broken`} {
			messages.streams = nil
			handler(ses, []byte(payload), nil)
			var reply struct {
				Success bool
				Error   string
				Coins   *int
				Money   *int
			}
			if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.GameCornerCoinBalanceResponse || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success || reply.Error == "" || reply.Coins != nil || reply.Money != nil {
				t.Fatalf("legacy coin lane returned a balance/success: %+v", messages.streams)
			}
		}
	}
	messages.streams = nil
	item := &cqitems.CQInventoryItem{Item: cqitems.CQItem{ID: CoinCaseItemID, ShortName: "COIN_CASE"}, Instance: cqitems.CQItemInstance{Quantity: 1}}
	if !tryHandleFieldItemUse(ses, nil, item, 42, CQItemUseRequest{}) {
		t.Fatal("legacy Coin Case was not rejected")
	}
	var reply struct {
		Success bool
		Error   string
	}
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success || reply.Error == "" {
		t.Fatalf("legacy Coin Case reply %+v", messages.streams)
	}
}

func TestGameCornerScriptedCoinsRollbackReplayAndCurrentRecovery(t *testing.T) {
	database, wh, ses, messages, script := scriptedCoinFixture(t)
	completion := issueScriptedCoins(t, database, script)
	apply := func(completion *cutsceneCompletion) (bool, error) {
		_, completed, err := runCutsceneMutation(context.Background(), CutsceneActionContext{Database: database, Session: ses, WorldHandler: wh, issuedCompletion: completion}, "", nil, 42, nil)
		return completed, err
	}
	testdb.Exec(t, database, `CREATE FUNCTION reject_coin_script_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late coin script commit';END $$;
CREATE CONSTRAINT TRIGGER reject_coin_script_commit AFTER UPDATE ON character_cutscene_plans DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.resolution='resolved') EXECUTE FUNCTION reject_coin_script_commit()`)
	if completed, err := apply(completion); err == nil || completed {
		t.Fatalf("failed commit accepted completed=%t err=%v", completed, err)
	}
	assertScriptedCoinBalances(t, database, 2000, 3)
	if len(messages.streams) != 0 {
		t.Fatal("failed coin script published resources/narrative")
	}
	var resolution string
	if err := database.QueryRow(`SELECT resolution FROM character_cutscene_plans WHERE completion_token=$1`, completion.Token).Scan(&resolution); err != nil || resolution != "pending" {
		t.Fatalf("failed receipt=%s %v", resolution, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_coin_script_commit ON character_cutscene_plans`)
	if completed, err := apply(completion); err != nil || !completed || completion.Replayed {
		t.Fatalf("purchase completed=%t replay=%t err=%v", completed, completion.Replayed, err)
	}
	assertScriptedCoinBalances(t, database, 1000, 53)
	resources := 0
	for _, message := range messages.streams {
		if message.opcode == opcodes.CharacterWallet {
			t.Fatal("script published a historical partial wallet")
		}
		if message.opcode == opcodes.ResourcesChangedNotify {
			resources++
		}
	}
	if resources != 1 {
		t.Fatalf("money/coins did not coalesce to one current-resource notice: %d", resources)
	}
	snapshot, _, err := readGameplayState(context.Background(), ses, wh, GameplayStateRequest{RequestID: "coins-current", MapID: 135})
	if err != nil || snapshot.Coins != 53 || snapshot.Wallet.Pokedollars != 1000 {
		t.Fatalf("coin/wallet snapshot %+v err=%v", snapshot, err)
	}
	// Later gameplay and source changes must survive replay after owner replacement.
	testdb.Exec(t, database, `UPDATE character_coins SET coins=51 WHERE character_id=42;UPDATE character_wallet SET pokedollars=700 WHERE character_id=42;UPDATE character_data SET x=9 WHERE id=42`)
	ses.Client.CharData().X = 9
	messages.streams = nil
	fresh := &cutsceneCompletion{Token: completion.Token, Label: completion.Label}
	if completed, err := apply(fresh); err != nil || !completed || !fresh.Replayed {
		t.Fatalf("replay completed=%t replay=%t err=%v", completed, fresh.Replayed, err)
	}
	assertScriptedCoinBalances(t, database, 700, 51)
	if len(messages.streams) != 0 {
		t.Fatal("historical coin receipt republished effects")
	}
	snapshot, _, err = readGameplayState(context.Background(), ses, wh, GameplayStateRequest{RequestID: "coins-after-replay", Current: true})
	if err != nil || snapshot.Coins != 51 || snapshot.Wallet.Pokedollars != 700 || snapshot.Position.X != 9 {
		t.Fatalf("replay recovery %+v err=%v", snapshot, err)
	}
	tampered := &cutsceneCompletion{Token: completion.Token, Label: "OtherClerk"}
	if _, err := apply(tampered); err == nil {
		t.Fatal("changed script identity replay accepted")
	}
	assertScriptedCoinBalances(t, database, 700, 51)
}

func TestGameCornerScriptedCoinsConcurrentCompletionChargesOnce(t *testing.T) {
	database, _, _, _, script := scriptedCoinFixture(t)
	original := issueScriptedCoins(t, database, script)
	type outcome struct {
		completed, replayed bool
		err                 error
	}
	results := make(chan outcome, 4)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			completion := &cutsceneCompletion{Token: original.Token, Label: original.Label}
			_, completed, err := runCutsceneMutation(context.Background(), CutsceneActionContext{Database: database, issuedCompletion: completion}, "", nil, 42, nil)
			results <- outcome{completed, completion.Replayed, err}
		}()
	}
	workers.Wait()
	close(results)
	committed := 0
	for result := range results {
		if result.err != nil || !result.completed {
			t.Fatalf("concurrent completion %+v", result)
		}
		if !result.replayed {
			committed++
		}
	}
	if committed != 1 {
		t.Fatalf("new charges=%d", committed)
	}
	assertScriptedCoinBalances(t, database, 1000, 53)
}

func TestGameCornerScriptedCoinsRecheckSourceEligibilityAndCancellation(t *testing.T) {
	for _, kind := range []string{"saved-source", "owned-source", "missing-case", "money", "full-case", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			database, wh, ses, _, script := scriptedCoinFixture(t)
			completion := issueScriptedCoins(t, database, script)
			wantMoney, wantCoins := 2000, 3
			switch kind {
			case "saved-source":
				testdb.Exec(t, database, `UPDATE character_data SET x=6 WHERE id=42`)
			case "owned-source":
				ses.Client.CharData().X = 6
			case "missing-case":
				testdb.Exec(t, database, `DELETE FROM cq_character_inventory WHERE character_id=42`)
			case "money":
				testdb.Exec(t, database, `UPDATE character_wallet SET pokedollars=999 WHERE character_id=42`)
				wantMoney = 999
			case "full-case":
				testdb.Exec(t, database, `UPDATE character_coins SET coins=9999 WHERE character_id=42`)
				wantCoins = 9999
			case "cancelled":
				completion.Cancel = true
			}
			_, completed, err := runCutsceneMutation(context.Background(), CutsceneActionContext{Database: database, Session: ses, WorldHandler: wh, issuedCompletion: completion}, "", nil, 42, nil)
			if completed {
				t.Fatalf("invalid %s purchase completed", kind)
			}
			if (kind == "saved-source" || kind == "owned-source") && err == nil {
				t.Fatal("changed source accepted")
			}
			assertScriptedCoinBalances(t, database, wantMoney, wantCoins)
			if kind == "cancelled" {
				completion.Cancel = false
				if _, _, err := runCutsceneMutation(context.Background(), CutsceneActionContext{Database: database, issuedCompletion: completion}, "", nil, 42, nil); err == nil {
					t.Fatal("cancelled token later charged")
				}
			}
		})
	}
}

func TestGameCornerCoinRecoveryReadsOneOwnedWalletAndFailsClosed(t *testing.T) {
	database, wh, ses, messages, _ := scriptedCoinFixture(t)
	holder, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	if err := db.LockCharacter(holder, 42); err != nil {
		t.Fatal(err)
	}
	var holderPID int
	if err := holder.QueryRow(`SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	if _, err := holder.Exec(`UPDATE character_wallet SET pokedollars=700 WHERE character_id=42`); err != nil {
		t.Fatal(err)
	}
	type result struct {
		snapshot GameplayStateResponse
		err      error
	}
	results := make(chan result, 1)
	go func() {
		snapshot, _, err := readGameplayState(context.Background(), ses, wh, GameplayStateRequest{RequestID: "owned-coins", Current: true})
		results <- result{snapshot, err}
	}()
	deadline := time.Now().Add(time.Second)
	for {
		var waiting bool
		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1=ANY(pg_blocking_pids(pid)))`, holderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("coin recovery did not wait on shared character owner")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := holder.Exec(`UPDATE character_coins SET coins=51 WHERE character_id=42`); err != nil {
		t.Fatal(err)
	}
	if err := holder.Commit(); err != nil {
		t.Fatal(err)
	}
	read := <-results
	if read.err != nil || read.snapshot.Wallet.Pokedollars != 700 || read.snapshot.Coins != 51 {
		t.Fatalf("mixed coin/wallet read %+v %v", read.snapshot, read.err)
	}
	testdb.Exec(t, database, `DELETE FROM character_coins WHERE character_id=42`)
	snapshot, _, err := readGameplayState(context.Background(), ses, wh, GameplayStateRequest{RequestID: "empty-coins", Current: true})
	if err != nil || snapshot.Coins != 0 || snapshot.Wallet.Pokedollars != 700 {
		t.Fatalf("absent coin row policy %+v %v", snapshot, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_coins RENAME TO unavailable_coins`)
	battleDispatch(t, wh, ses, opcodes.GameplayStateRequest, `{"requestId":"coin-read-failure","current":true}`)
	if len(messages.streams) != 1 {
		t.Fatalf("partial coin read publication %+v", messages.streams)
	}
	var rejected map[string]json.RawMessage
	if err := json.Unmarshal(messages.streams[0].payload, &rejected); err != nil {
		t.Fatal(err)
	}
	if string(rejected["success"]) != "false" || rejected["wallet"] != nil || rejected["coins"] != nil {
		t.Fatal(fmt.Sprintf("failed coin read became partial/zero success: %s", messages.streams[0].payload))
	}
}

func TestGameCornerScriptedCoinsWaitForSourceAndHonorCancellation(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprintf("cancelled=%t", cancelled), func(t *testing.T) {
			database, wh, ses, messages, script := scriptedCoinFixture(t)
			completion := issueScriptedCoins(t, database, script)
			holder, err := database.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer holder.Rollback()
			if err := db.LockCharacter(holder, 42); err != nil {
				t.Fatal(err)
			}
			var holderPID int
			if err := holder.QueryRow(`SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			results := make(chan error, 1)
			go func() {
				_, _, err := runCutsceneMutation(ctx, CutsceneActionContext{Database: database, Session: ses, WorldHandler: wh, issuedCompletion: completion}, "", nil, 42, nil)
				results <- err
			}()
			deadline := time.Now().Add(time.Second)
			for {
				var waiting bool
				if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND $1=ANY(pg_blocking_pids(pid)))`, holderPID).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("scripted purchase did not retain character source ownership")
				}
				time.Sleep(time.Millisecond)
			}
			if cancelled {
				cancel()
				if err := <-results; err == nil {
					t.Fatal("cancelled waiting purchase completed")
				}
				if err := holder.Rollback(); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := holder.Exec(`UPDATE character_data SET x=6 WHERE id=42`); err != nil {
					t.Fatal(err)
				}
				if err := holder.Commit(); err != nil {
					t.Fatal(err)
				}
				if err := <-results; err == nil {
					t.Fatal("source changed during lock wait but purchase completed")
				}
			}
			assertScriptedCoinBalances(t, database, 2000, 3)
			if len(messages.streams) != 0 {
				t.Fatal("rejected waiting purchase published effects")
			}
			var resolution string
			if err := database.QueryRow(`SELECT resolution FROM character_cutscene_plans WHERE completion_token=$1`, completion.Token).Scan(&resolution); err != nil || resolution != "pending" {
				t.Fatalf("waiting failure consumed receipt: %s %v", resolution, err)
			}
		})
	}
}

func TestGameCornerCoinRecoveryJoinsRetainedOwner(t *testing.T) {
	database, wh, ses, messages, _ := scriptedCoinFixture(t)
	database.SetMaxOpenConns(1)
	read := false
	err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, 42); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE character_wallet SET pokedollars=500 WHERE character_id=42;UPDATE character_coins SET coins=8 WHERE character_id=42`); err != nil {
			return err
		}
		snapshot, _, err := readGameplayStateFrom(context.Background(), tx, ses, wh, GameplayStateRequest{RequestID: "joined-coins", MapID: 135})
		if err != nil {
			return err
		}
		if snapshot.Wallet.Pokedollars != 500 || snapshot.Coins != 8 {
			return fmt.Errorf("joined read escaped owner: money=%d coins=%d", snapshot.Wallet.Pokedollars, snapshot.Coins)
		}
		read = true
		return fmt.Errorf("reject outer list transaction")
	})
	if err == nil || !read {
		t.Fatalf("joined read did not stay provisional: read=%t err=%v", read, err)
	}
	assertScriptedCoinBalances(t, database, 2000, 3)
	if len(messages.streams) != 0 {
		t.Fatal("joined resource read published before its outer commit")
	}
}
