package world

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

// Existing domain fixtures bind to current authority explicitly; tests of missing
// or historical envelopes dispatch their original bytes without this helper.
func gameCornerFixtureRequest(t *testing.T, wh *WorldHandler, ses *session.Session, body string) string {
	t.Helper()
	var request map[string]interface{}
	if err := json.Unmarshal([]byte(body), &request); err != nil {
		t.Fatal(err)
	}
	var revision int64
	if err := wh.database.QueryRow(`SELECT COALESCE((SELECT revision FROM character_shop_state WHERE character_id=$1),0)`, ses.Client.CharData().ID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	char := ses.Client.CharData()
	request["requestId"] = fmt.Sprintf("fixture-%d", revision)
	request["command"] = map[string]interface{}{"characterId": int64(char.ID), "revision": revision}
	request["mapId"], request["fromX"], request["fromY"] = char.MapID, int(char.X), int(char.Y)
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func gameCornerCommandFixture(t *testing.T, kind string) (*sql.DB, *WorldHandler, *session.Session, *recordingMessenger, opcodes.OpCode, string) {
	t.Helper()
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 INSERT INTO character_coins(character_id,coins) VALUES(42,100);
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(135,'GAME_CORNER',20,20,0),(137,'GAME_CORNER_PRIZE_ROOM',10,10,0);
 INSERT INTO phaser_hidden_objects(id,map_constant,map_id,x,y,item_or_direction,routine) VALUES(1,'GAME_CORNER',135,6,8,'ANY_FACING','StartSlotMachine');
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,text) VALUES(10,137,6,2,'sign','TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_3');
 INSERT INTO phaser_game_corner_prizes(id,prize_type,item_id,prize_name,coin_cost) VALUES(7,'tm',1,'TM',50);`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	mapID, x, y := 135, 5, 8
	opcode, body := opcodes.GameCornerSlotPlayRequest, `{"bet":1,"machineX":6,"machineY":8}`
	if kind == "prize" {
		mapID, x, y = 137, 6, 3
		opcode, body = opcodes.GameCornerPrizeBuyRequest, `{"prizeId":7}`
	}
	ses.Client.CharData().MapID = uint32(mapID)
	ses.Client.CharData().X, ses.Client.CharData().Y = float64(x), float64(y)
	testdb.Exec(t, database, `UPDATE character_data SET map_id=$1,x=$2,y=$3 WHERE id=42`, mapID, x, y)
	return database, wh, ses, messages, opcode, gameCornerFixtureRequest(t, wh, ses, body)
}

func gameCornerOutcome(t *testing.T, messages *recordingMessenger, success bool) map[string]interface{} {
	t.Helper()
	if len(messages.streams) != 1 {
		t.Fatalf("command published independent/partial state: %+v", messages.streams)
	}
	var reply map[string]interface{}
	if err := json.Unmarshal(messages.streams[0].payload, &reply); err != nil || reply["success"] != success {
		t.Fatalf("command response=%s success=%t error=%v", messages.streams[0].payload, success, err)
	}
	for _, key := range []string{"coins", "money", "wallet", "inventory"} {
		if _, present := reply[key]; present {
			t.Fatalf("historical outcome contains live resource %s", key)
		}
	}
	return reply
}

func TestGameCornerCommandsRetainSourceAndPaymentOwner(t *testing.T) {
	for _, kind := range []string{"slot", "prize"} {
		for _, mode := range []string{"single_connection", "saved_source_changed", "cold_durable_battle", "deferred_commit", "held_source_change"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				database, wh, ses, messages, opcode, request := gameCornerCommandFixture(t, kind)
				db.GlobalWorldDB = nil // Every live reader must use the retained owner.
				if mode == "single_connection" {
					database.SetMaxOpenConns(1)
				}
				if mode == "saved_source_changed" {
					testdb.Exec(t, database, `UPDATE character_data SET x=x+3 WHERE id=42`)
				}
				if mode == "cold_durable_battle" {
					current := battleTestStart(t, database, false, nil)
					forgetBattle(42, current)
				}
				if mode == "deferred_commit" {
					testdb.Exec(t, database, `CREATE FUNCTION reject_gamecorner_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late Game Corner receipt rejection'; END $$;
 CREATE CONSTRAINT TRIGGER reject_gamecorner_receipt AFTER UPDATE ON character_shop_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.result_json IS NOT NULL) EXECUTE FUNCTION reject_gamecorner_receipt();`)
				}
				if mode == "held_source_change" {
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
					done := make(chan struct{})
					go func() { defer close(done); battleDispatch(t, wh, ses, opcode, request) }()
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
							t.Fatal("purchase did not wait on retained character owner")
						}
						time.Sleep(time.Millisecond)
					}
					if _, err := holder.Exec(`UPDATE character_data SET x=x+3 WHERE id=42`); err != nil {
						t.Fatal(err)
					}
					if err := holder.Commit(); err != nil {
						t.Fatal(err)
					}
					select {
					case <-done:
					case <-time.After(2 * time.Second):
						t.Fatal("purchase did not settle after source change")
					}
				} else {
					battleDispatch(t, wh, ses, opcode, request)
				}
				success := mode == "single_connection"
				reply := gameCornerOutcome(t, messages, success)
				var coins, inventory int
				var revision int64
				if err := database.QueryRow(`SELECT coins FROM character_coins WHERE character_id=42`).Scan(&coins); err != nil {
					t.Fatal(err)
				}
				if err := database.QueryRow(`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&inventory); err != nil {
					t.Fatal(err)
				}
				if err := database.QueryRow(`SELECT COALESCE((SELECT revision FROM character_shop_state WHERE character_id=42),0)`).Scan(&revision); err != nil {
					t.Fatal(err)
				}
				if !success {
					if coins != 100 || inventory != 1 || revision != 0 {
						t.Fatalf("failed command changed coins=%d inventory=%d revision=%d", coins, inventory, revision)
					}
					for _, field := range []string{"prize", "reelPositions", "reels", "isLucky"} {
						if _, present := reply[field]; present {
							t.Fatalf("failed command published %s", field)
						}
					}
				} else if revision != 1 || (kind == "prize" && (coins != 50 || inventory != 2)) {
					t.Fatalf("committed owner coins=%d inventory=%d revision=%d", coins, inventory, revision)
				}
			})
		}
	}
}

func TestGameCornerReceiptsReplayOutcomeWithoutSpendingOrOldResourceProjection(t *testing.T) {
	for _, kind := range []string{"slot", "prize"} {
		t.Run(kind, func(t *testing.T) {
			database, wh, ses, messages, opcode, request := gameCornerCommandFixture(t, kind)
			battleDispatch(t, wh, ses, opcode, request)
			reply := gameCornerOutcome(t, messages, true)
			if reply["version"] != float64(1) || reply["kind"] != kind || reply["commandRevision"] != float64(1) {
				t.Fatalf("missing receipt identity: %+v", reply)
			}
			first := append([]byte(nil), messages.streams[0].payload...)
			var quantity int
			if err := database.QueryRow(`SELECT COALESCE(SUM(quantity),0) FROM cq_item_instances WHERE owner_id=42 AND item_id=1`).Scan(&quantity); err != nil {
				t.Fatal(err)
			}

			// Independent committed gameplay can change resources and source while
			// the old command remains the latest receipt. Replay returns narrative only.
			testdb.Exec(t, database, `UPDATE character_coins SET coins=222 WHERE character_id=42;
 UPDATE character_wallet SET pokedollars=777 WHERE character_id=42;
 UPDATE character_data SET x=0,y=0 WHERE id=42;`)
			ses.Client.CharData().X, ses.Client.CharData().Y = 0, 0
			messages.streams = nil
			battleDispatch(t, wh, ses, opcode, request)
			gameCornerOutcome(t, messages, true)
			if !bytes.Equal(first, messages.streams[0].payload) {
				t.Fatal("receipt rerolled or rewrote historical outcome")
			}
			var coins, money, after int
			if err := database.QueryRow(`SELECT c.coins,w.pokedollars FROM character_coins c JOIN character_wallet w USING(character_id) WHERE character_id=42`).Scan(&coins, &money); err != nil || coins != 222 || money != 777 {
				t.Fatalf("replay rewrote balances %d/%d %v", coins, money, err)
			}
			if err := database.QueryRow(`SELECT COALESCE(SUM(quantity),0) FROM cq_item_instances WHERE owner_id=42 AND item_id=1`).Scan(&after); err != nil || after != quantity {
				t.Fatalf("replay repeated prize=%d/%d %v", quantity, after, err)
			}

			var modified map[string]interface{}
			if err := json.Unmarshal([]byte(request), &modified); err != nil {
				t.Fatal(err)
			}
			modified["fromX"] = float64(1)
			changed, _ := json.Marshal(modified)
			messages.streams = nil
			battleDispatch(t, wh, ses, opcode, string(changed))
			gameCornerOutcome(t, messages, false)

			// Every other inventory command invalidates the old receipt rather than
			// letting it overwrite a later revision.
			if _, err := cqitems.NewStore(database).ExecuteCommand(context.Background(), 42, 1, func(db.DBTX) error { return nil }); err != nil {
				t.Fatal(err)
			}
			messages.streams = nil
			battleDispatch(t, wh, ses, opcode, request)
			gameCornerOutcome(t, messages, false)
		})
	}
}

func TestGameCornerRejectsLegacyForeignAndCorruptOutcomes(t *testing.T) {
	for _, kind := range []string{"slot", "prize"} {
		t.Run(kind, func(t *testing.T) {
			database, wh, ses, messages, opcode, request := gameCornerCommandFixture(t, kind)
			for _, payload := range []string{`{}`, `{"bet":1,"machineX":6,"machineY":8}`, `{"prizeId":7}`} {
				messages.streams = nil
				battleDispatch(t, wh, ses, opcode, payload)
				gameCornerOutcome(t, messages, false)
			}
			var foreign map[string]interface{}
			if err := json.Unmarshal([]byte(request), &foreign); err != nil {
				t.Fatal(err)
			}
			foreign["command"] = map[string]interface{}{"characterId": 43, "revision": 0}
			bad, _ := json.Marshal(foreign)
			messages.streams = nil
			battleDispatch(t, wh, ses, opcode, string(bad))
			gameCornerOutcome(t, messages, false)
			messages.streams = nil
			battleDispatch(t, wh, ses, opcode, request)
			gameCornerOutcome(t, messages, true)
			var coins int
			if err := database.QueryRow(`SELECT coins FROM character_coins WHERE character_id=42`).Scan(&coins); err != nil {
				t.Fatal(err)
			}

			// The outer envelope can remain well formed while its domain outcome
			// is missing identity; the handler must reject it without another mutation.
			testdb.Exec(t, database, `UPDATE character_shop_state SET result_json=jsonb_set(result_json::jsonb,'{result,version}','2'::jsonb)::text WHERE character_id=42`)
			messages.streams = nil
			battleDispatch(t, wh, ses, opcode, request)
			gameCornerOutcome(t, messages, false)
			var after int
			if err := database.QueryRow(`SELECT coins FROM character_coins WHERE character_id=42`).Scan(&after); err != nil || after != coins {
				t.Fatalf("corrupt outcome mutated coins=%d/%d %v", coins, after, err)
			}
		})
	}
}

func TestGameCornerPrizeListUsesScopedRetainedCurrentSnapshot(t *testing.T) {
	database, wh, ses, messages, _, _ := gameCornerCommandFixture(t, "prize")
	db.GlobalWorldDB = nil
	database.SetMaxOpenConns(1)
	request := `{"requestId":"window","characterId":42,"mapId":137,"fromX":6,"fromY":3,"prizeWindow":3}`
	battleDispatch(t, wh, ses, opcodes.GameCornerPrizeListRequest, request)
	if len(messages.streams) != 1 {
		t.Fatalf("list publications=%+v", messages.streams)
	}
	var reply GameCornerPrizeListResponse
	if err := json.Unmarshal(messages.streams[0].payload, &reply); err != nil || !reply.Success || reply.PrizeWindow != 3 || len(reply.Prizes) != 1 || reply.Prizes[0].ID != 7 || reply.Snapshot.Coins != 100 || reply.Snapshot.Wallet.Pokedollars != 100 || reply.Snapshot.Position.X != 6 || reply.Snapshot.CommandRevision != 0 {
		t.Fatalf("scoped list=%+v %v raw=%s", reply, err, messages.streams[0].payload)
	}

	testdb.Exec(t, database, `UPDATE character_data SET x=1 WHERE id=42`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameCornerPrizeListRequest, request)
	gameCornerOutcome(t, messages, false)
	testdb.Exec(t, database, `UPDATE character_data SET x=6 WHERE id=42;
 INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,false,'test')`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameCornerPrizeListRequest, request)
	gameCornerOutcome(t, messages, false)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.GameCornerPrizeListRequest, `{}`)
	gameCornerOutcome(t, messages, false)
}
