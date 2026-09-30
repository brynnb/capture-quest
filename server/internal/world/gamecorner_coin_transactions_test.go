package world

import (
	"capturequest/internal/api/opcodes"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

type losingGameCornerRandom struct{}

func (losingGameCornerRandom) Intn(n int) int { return 1 % n }

func TestGameCornerCoinPurchaseRollbackAndRetry(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 UPDATE character_wallet SET pokedollars=2000 WHERE character_id=42;
 INSERT INTO character_coins(character_id,coins) VALUES(42,0);
 ALTER TABLE character_coins ADD CONSTRAINT reject_coins CHECK(coins=0)`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	result, err := buyGameCornerCoins(context.Background(), database, 42)
	if err == nil || result.Success || result.Money != 2000 || result.Coins != 0 {
		t.Fatalf("failed purchase %+v %v", result, err)
	}
	money, err := cqitems.NewStore(database).GetCharacterMoney(42)
	if err != nil || money != 2000 {
		t.Fatalf("payment escaped: %d %v", money, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_coins DROP CONSTRAINT reject_coins`)
	result, err = buyGameCornerCoins(context.Background(), database, 42)
	if err != nil || !result.Success || result.Money != 1000 || result.Coins != 50 {
		t.Fatalf("retry %+v %v", result, err)
	}
	// Cancellation of a competing operation must not report an accepted purchase.
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
	result, err = buyGameCornerCoins(ctx, database, 42)
	if err == nil || result.Success {
		t.Fatalf("cancelled purchase %+v %v", result, err)
	}
}

func TestGameCornerHiddenCoinsAtomicCollection(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 INSERT INTO character_coins(character_id,coins) VALUES(42,0);
 INSERT INTO phaser_hidden_coins(id,map_constant,map_id,x,y,coin_amount) VALUES(1,'GAME_CORNER',135,1,1,10);
 ALTER TABLE character_coins ADD CONSTRAINT reject_hidden CHECK(coins=0)`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	result, err := collectGameCornerHiddenCoin(context.Background(), database, 42, 135, 1, 1)
	if err == nil || result.Success || result.Coins != 0 {
		t.Fatalf("failure %+v %v", result, err)
	}
	var markers int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_collected_hidden_coins`).Scan(&markers); err != nil || markers != 0 {
		t.Fatalf("collection escaped rollback %d %v", markers, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_coins DROP CONSTRAINT reject_hidden`)
	results := make(chan GameCornerHiddenCoinPickupResult, 4)
	errors := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := collectGameCornerHiddenCoin(context.Background(), database, 42, 135, 1, 1)
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
		} else if !result.AlreadyFound {
			t.Fatalf("duplicate %+v", result)
		}
	}
	coins, err := gameCornerCoinBalance(database, 42)
	if err != nil || successes != 1 || coins != 10 {
		t.Fatalf("successes=%d coins=%d %v", successes, coins, err)
	}
}

func TestGameCornerSlotsRollbackAndConcurrentSpending(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 INSERT INTO character_coins(character_id,coins) VALUES(42,3);
 ALTER TABLE character_coins ADD CONSTRAINT reject_spin CHECK(coins=3)`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	result, err := playGameCornerSlot(context.Background(), database, 42, 3, false, losingGameCornerRandom{})
	if err == nil || result.Success || result.Reels != nil || result.Coins != 3 {
		t.Fatalf("failed spin %+v %v", result, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_coins DROP CONSTRAINT reject_spin`)
	results := make(chan GameCornerSlotPlayResult, 4)
	errors := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := playGameCornerSlot(context.Background(), database, 42, 3, false, losingGameCornerRandom{})
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
			if result.Payout != 0 || result.Reels == nil {
				t.Fatalf("spin %+v", result)
			}
		} else if result.Message != "Not enough coins!" {
			t.Fatalf("failure %+v", result)
		}
	}
	coins, err := gameCornerCoinBalance(database, 42)
	if err != nil || successes != 1 || coins != 0 {
		t.Fatalf("successes=%d coins=%d %v", successes, coins, err)
	}
}

func TestGameCornerCoinPurchaseRequiresSourceClerkReach(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 UPDATE character_wallet SET pokedollars=2000 WHERE character_id=42;
 INSERT INTO character_coins(character_id,coins) VALUES(42,3);
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(135,'GAME_CORNER',20,20,0);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text) VALUES(10,135,5,6,'npc','CLERK1','TEXT_GAMECORNER_CLERK1'),(11,135,10,6,'npc','CLERK2','TEXT_GAMECORNER_CLERK2');
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,talk_over_tile) VALUES(135,5,7,1,true),(135,10,7,1,true)`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	db.GlobalWorldDB = nil
	ses.Client.CharData().MapID = GameCornerMapID
	ses.Client.CharData().Y = 8
	buy := func(wantSuccess bool) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.GameCornerBuyCoinsRequest, `{}`)
		if len(messages.streams) == 0 {
			t.Fatal("missing coin purchase response")
		}
		var response struct {
			Success bool
			Error   string
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success != wantSuccess {
			t.Fatalf("response %+v, want success=%v: %v", response, wantSuccess, err)
		}
		if !wantSuccess && (len(messages.streams) != 1 || response.Error != "Move to the coin clerk first.") {
			t.Fatalf("denied purchase published unexpected result: %+v", messages.streams)
		}
	}
	for _, x := range []float64{15, 10} {
		ses.Client.CharData().X = x
		buy(false)
	}
	ses.Client.CharData().X = 5
	testdb.Exec(t, database, `INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,false,'test')`)
	buy(false)
	money, err := cqitems.NewStore(database).GetCharacterMoney(42)
	coins, coinErr := gameCornerCoinBalance(database, 42)
	if err != nil || coinErr != nil || money != 2000 || coins != 3 {
		t.Fatalf("denied purchase changed balances money=%d coins=%d %v %v", money, coins, err, coinErr)
	}
	testdb.Exec(t, database, `DELETE FROM character_object_visibility_overrides`)
	buy(true)
	money, err = cqitems.NewStore(database).GetCharacterMoney(42)
	coins, coinErr = gameCornerCoinBalance(database, 42)
	if err != nil || coinErr != nil || money != 1000 || coins != 53 {
		t.Fatalf("valid purchase balances money=%d coins=%d %v %v", money, coins, err, coinErr)
	}
}

func TestGameCornerCoinHandlersDoNotPublishFailedChanges(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(69,'COIN CASE','COIN_CASE',true);
 UPDATE character_wallet SET pokedollars=2000 WHERE character_id=42;
 INSERT INTO character_coins(character_id,coins) VALUES(42,3);
 ALTER TABLE character_coins ADD CONSTRAINT reject_handler_change CHECK(coins=3)`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 69, 1); err != nil {
		t.Fatal(err)
	}
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(135,'GAME_CORNER',20,20,0),(137,'GAME_CORNER_PRIZE_ROOM',10,10,0);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text) VALUES(10,135,5,6,'npc','CLERK1','TEXT_GAMECORNER_CLERK1');
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,talk_over_tile) VALUES(135,5,7,1,true);
 INSERT INTO phaser_hidden_objects(id,map_constant,map_id,x,y,item_or_direction,routine) VALUES(1,'GAME_CORNER',135,6,8,'ANY_FACING','StartSlotMachine')`)
	ses.Client.CharData().X = 5
	ses.Client.CharData().Y = 8
	db.GlobalWorldDB = nil
	for _, mapID := range []int{PrizeRoomMapID, GameCornerMapID} {
		ses.Client.CharData().MapID = uint32(mapID)
		for _, opcode := range []opcodes.OpCode{opcodes.GameCornerBuyCoinsRequest, opcodes.GameCornerSlotPlayRequest} {
			messages.streams = nil
			battleDispatch(t, wh, ses, opcode, `{"bet":3,"machineX":6,"machineY":8,"isLucky":true}`)
			if len(messages.streams) != 1 {
				t.Fatalf("failed handler publication %+v", messages.streams)
			}
			var response struct {
				Success bool
				Error   string
			}
			if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error == "" {
				t.Fatalf("failure %+v %v", response, err)
			}
			if mapID == GameCornerMapID {
				expected := "Could not buy coins."
				if opcode == opcodes.GameCornerSlotPlayRequest {
					expected = "Could not play slots."
				}
				if response.Error != expected {
					t.Fatalf("transaction failure was masked by earlier denial: %+v", response)
				}
			}
		}
	}
	money, err := cqitems.NewStore(database).GetCharacterMoney(42)
	coins, coinErr := gameCornerCoinBalance(database, 42)
	if err != nil || coinErr != nil || money != 2000 || coins != 3 {
		t.Fatalf("failed handlers changed balances money=%d coins=%d %v %v", money, coins, err, coinErr)
	}
}
