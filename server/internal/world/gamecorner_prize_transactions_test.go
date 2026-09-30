package world

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

func TestGameCornerPrizeRewardAndPaymentRollback(t *testing.T) {
	for _, kind := range []string{"tm", "pokemon"} {
		t.Run(kind, func(t *testing.T) {
			database, _, _, _ := battleTestWorld(t)
			testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 INSERT INTO character_coins(character_id,coins) VALUES(42,100);
 INSERT INTO phaser_game_corner_prizes(id,prize_type,pokemon_id,item_id,prize_name,coin_cost) VALUES(7,'tm',NULL,1,'TM',50),(1,'pokemon',25,NULL,'PIKACHU',50);
 ALTER TABLE character_coins ADD CONSTRAINT reject_payment CHECK(coins=100)`)
			if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
				t.Fatal(err)
			}
			prizeID := 7
			if kind == "pokemon" {
				prizeID = 1
			}
			result, err := buyGameCornerPrize(context.Background(), database, 42, prizeID, "")
			if err == nil || result.Success || result.Prize != nil {
				t.Fatalf("failure published: %+v %v", result, err)
			}
			var pokemon, inventory, dex, coins int
			for query, target := range map[string]*int{
				`SELECT COUNT(*) FROM character_pokemon WHERE character_id=42`:      &pokemon,
				`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`: &inventory,
				`SELECT COUNT(*) FROM character_pokedex WHERE character_id=42`:      &dex,
				`SELECT coins FROM character_coins WHERE character_id=42`:           &coins,
			} {
				if err := database.QueryRow(query).Scan(target); err != nil {
					t.Fatal(err)
				}
			}
			if pokemon != 1 || inventory != 1 || dex != 0 || coins != 100 {
				t.Fatalf("rollback pokemon=%d inventory=%d dex=%d coins=%d", pokemon, inventory, dex, coins)
			}
			testdb.Exec(t, database, `ALTER TABLE character_coins DROP CONSTRAINT reject_payment`)
			result, err = buyGameCornerPrize(context.Background(), database, 42, prizeID, "")
			if err != nil || !result.Success || result.Coins != 50 {
				t.Fatalf("retry: %+v %v", result, err)
			}
		})
	}
}

func TestGameCornerPrizeCommitFailureDispatcherAndOwnedDatabase(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 INSERT INTO character_coins(character_id,coins) VALUES(42,100);
 INSERT INTO phaser_game_corner_prizes(id,prize_type,item_id,prize_name,coin_cost) VALUES(7,'tm',1,'TM',50);
 CREATE FUNCTION reject_prize_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject prize commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_prize_commit AFTER UPDATE ON character_coins DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_prize_commit();`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(137,'GAME_CORNER_PRIZE_ROOM',10,10,0),(135,'GAME_CORNER',20,20,0);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,text) VALUES(10,137,6,2,'sign','TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_3')`)
	ses.Client.CharData().X = 6
	ses.Client.CharData().Y = 3
	ses.Client.CharData().MapID = PrizeRoomMapID
	db.GlobalWorldDB = nil
	request := func(want bool) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.GameCornerPrizeBuyRequest, `{"prizeId":7}`)
		var result struct {
			Success bool
			Coins   int
		}
		if len(messages.streams) == 0 {
			t.Fatal("missing response")
		}
		if err := json.Unmarshal(messages.streams[0].payload, &result); err != nil || result.Success != want {
			t.Fatalf("response %+v %v", result, err)
		}
		if want {
			if len(messages.streams) != 2 || messages.streams[1].opcode != opcodes.CQInventoryResponse || result.Coins != 50 {
				t.Fatalf("success publication %+v %+v", messages.streams, result)
			}
		} else if len(messages.streams) != 1 {
			t.Fatalf("failure publication %+v", messages.streams)
		}
	}
	request(false)
	var quantity int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&quantity); err != nil || quantity != 1 {
		t.Fatalf("commit rollback quantity=%d %v", quantity, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_prize_commit ON character_coins`)
	ses.Client.CharData().MapID = GameCornerMapID
	request(false)
	ses.Client.CharData().MapID = PrizeRoomMapID
	ses.Client.CharData().X = 2
	request(false) // Wrong window in the correct room.
	ses.Client.CharData().X = 6
	testdb.Exec(t, database, `INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,false,'test')`)
	request(false)
	testdb.Exec(t, database, `DELETE FROM character_object_visibility_overrides`)
	request(true)
}

func TestGameCornerPrizeConcurrentFundsAndCancellation(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 INSERT INTO character_coins(character_id,coins) VALUES(42,50);
 INSERT INTO phaser_game_corner_prizes(id,prize_type,item_id,prize_name,coin_cost) VALUES(7,'tm',1,'TM',50)`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := db.LockCharacter(tx, 42); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result, err := buyGameCornerPrize(ctx, database, 42, 7, "")
	if err == nil || result.Success {
		t.Fatalf("cancelled purchase %+v %v", result, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	results := make(chan GameCornerPrizePurchaseResult, 4)
	errors := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := buyGameCornerPrize(context.Background(), database, 42, 7, "")
			results <- result
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	successes := 0
	for result := range results {
		if result.Success {
			successes++
		} else if result.Message != "You don't have enough coins!" {
			t.Fatalf("failure %+v", result)
		}
	}
	coins, err := gameCornerCoinBalance(database, 42)
	if err != nil {
		t.Fatal(err)
	}
	var inventory int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&inventory); err != nil {
		t.Fatal(err)
	}
	if successes != 1 || coins != 0 || inventory != 2 {
		t.Fatalf("successes=%d coins=%d inventory=%d", successes, coins, inventory)
	}
}
