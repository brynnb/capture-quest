package itemuse

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"

	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
)

func itemDatabase(t *testing.T) (*sql.DB, *Service) {
	t.Helper()
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(1,'one'),(2,'two');
	INSERT INTO phaser_pokemon(id,name,type_1,hp,atk,def,spd,spc,catch_rate,base_exp) VALUES
	(25,'PIKACHU','ELECTRIC',35,55,30,90,50,190,82),(26,'RAICHU','ELECTRIC',60,90,55,100,90,75,122);
	INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,cur_hp,max_hp,nickname) VALUES(1,0,0,25,50,1,95,'Buddy');
	INSERT INTO cq_items(id,name,short_name,item_type,is_usable,heal_amount,move_id) VALUES
	(1,'Potion','POTION',2,true,20,NULL),(2,'TM05','TM05',5,true,0,5),(3,'HM06','HM06',6,true,0,6),
	(4,'Thunder Stone','THUNDER_STONE',9,true,0,NULL),(5,'Poke Flute','POKE_FLUTE',4,true,0,NULL);
	INSERT INTO phaser_moves(id,constant_name,name,short_name,pp) VALUES
	(1,'ONE','ONE','ONE',10),(2,'TWO','TWO','TWO',10),(3,'THREE','THREE','THREE',10),
	(4,'FOUR','FOUR','FOUR',10),(5,'FIVE','FIVE','FIVE',10),(6,'SIX','SIX','SIX',10);
	INSERT INTO phaser_pokemon_tmhm(pokemon_id,pokemon_name,tm_hm_name,move_name,move_id) VALUES
	(25,'PIKACHU','TM05','FIVE',5),(25,'PIKACHU','HM06','SIX',6);`)
	return database, New(database)
}
func grant(t *testing.T, database *sql.DB, item int32, quantity uint16) int32 {
	t.Helper()
	id, err := cqitems.NewStore(database).AddItemToInventory(1, item, quantity)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func assertPartyAndQuantity(t *testing.T, database *sql.DB, instanceID int32, wantHP, wantQuantity int) {
	t.Helper()
	var hp, quantity int
	if err := database.QueryRow(`SELECT cur_hp FROM character_pokemon WHERE character_id=1 AND party_slot=0`).Scan(&hp); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COALESCE(SUM(quantity),0) FROM cq_item_instances WHERE id=$1`, instanceID).Scan(&quantity); err != nil {
		t.Fatal(err)
	}
	if hp != wantHP || quantity != wantQuantity {
		t.Fatalf("HP/quantity=%d/%d, want %d/%d", hp, quantity, wantHP, wantQuantity)
	}
}

func TestPartyItemRollbackAndRetry(t *testing.T) {
	database, service := itemDatabase(t)
	id := grant(t, database, 1, 1)
	testdb.Exec(t, database, `ALTER TABLE character_pokemon ADD CONSTRAINT fail_heal CHECK(cur_hp=1)`)
	result, err := service.UsePartyItem(context.Background(), 1, id, 0, -1)
	if err == nil || result.Success || result.Party != nil {
		t.Fatalf("failed effect published: %+v %v", result, err)
	}
	assertPartyAndQuantity(t, database, id, 1, 1)
	testdb.Exec(t, database, `ALTER TABLE character_pokemon DROP CONSTRAINT fail_heal`)
	result, err = service.UsePartyItem(context.Background(), 1, id, 0, -1)
	if err != nil || !result.Success || result.NewQuantity != 0 || result.Party[0].CurHP != 21 {
		t.Fatalf("retry=%+v %v", result, err)
	}
	assertPartyAndQuantity(t, database, id, 21, 0)
	if _, err := service.UsePartyItem(context.Background(), 1, id, 0, -1); err == nil {
		t.Fatal("used already-consumed instance")
	}
	assertPartyAndQuantity(t, database, id, 21, 0)
}

func TestConcurrentPartyItemsDoNotLoseEffectsOrConsumeExtraItems(t *testing.T) {
	database, service := itemDatabase(t)
	id := grant(t, database, 1, 2)
	var wg sync.WaitGroup
	var successes atomic.Int32
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := service.UsePartyItem(context.Background(), 1, id, 0, -1); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 2 {
		t.Fatalf("successful uses=%d", successes.Load())
	}
	assertPartyAndQuantity(t, database, id, 41, 0)
}

func TestTMSelectionRevalidatesAndHMIsReusable(t *testing.T) {
	database, service := itemDatabase(t)
	testdb.Exec(t, database, `UPDATE character_pokemon SET move1_id=1,move2_id=2,move3_id=3,move4_id=4 WHERE character_id=1`)
	tm := grant(t, database, 2, 1)
	result, err := service.UsePartyItem(context.Background(), 1, tm, 0, -1)
	if err != nil || !result.NeedsMoveSlot || result.MoveID != 5 || result.MoveName != "FIVE" || result.InstanceID != tm || result.PartySlot != 0 || result.Party != nil {
		t.Fatalf("move-selection context=%+v %v", result, err)
	}
	assertPartyAndQuantity(t, database, tm, 1, 1)
	// A later selection must re-check the move which currently occupies the slot.
	testdb.Exec(t, database, `INSERT INTO phaser_pokemon_tmhm(pokemon_id,pokemon_name,tm_hm_name,move_name,move_id,is_hm) VALUES(25,'PIKACHU','HM01','ONE',1,1)`)
	if _, err := service.UsePartyItem(context.Background(), 1, tm, 0, 0); err == nil {
		t.Fatal("forgot an HM after the prompt")
	}
	assertPartyAndQuantity(t, database, tm, 1, 1)
	result, err = service.UsePartyItem(context.Background(), 1, tm, 0, 1)
	if err != nil || result.NeedsMoveSlot || result.Party[0].Moves[1].ID != 5 {
		t.Fatalf("TM effect=%+v %v", result, err)
	}
	assertPartyAndQuantity(t, database, tm, 1, 0)
	hm := grant(t, database, 3, 1)
	result, err = service.UsePartyItem(context.Background(), 1, hm, 0, 2)
	if err != nil || result.Party[0].Moves[2].ID != 6 || result.NewQuantity != 1 {
		t.Fatalf("HM effect=%+v %v", result, err)
	}
	assertPartyAndQuantity(t, database, hm, 1, 1)
	// An inventory link must not make someone else's reusable item usable.
	testdb.Exec(t, database, `UPDATE cq_item_instances SET owner_id=2 WHERE id=$1`, hm)
	if _, err := service.UsePartyItem(context.Background(), 1, hm, 0, 2); err == nil {
		t.Fatal("used another owner's HM")
	}
}

func TestEvolutionRollsBackPokedexConsumptionAndPokemonTogether(t *testing.T) {
	database, service := itemDatabase(t)
	stone := grant(t, database, 4, 1)
	before, err := pokebattle.LoadParty(database, 1)
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_pokemon ADD CONSTRAINT fail_evolution CHECK(pokemon_id<>26)`)
	if result, err := service.UsePartyItem(context.Background(), 1, stone, 0, -1); err == nil || result.Success {
		t.Fatalf("failed evolution=%+v %v", result, err)
	}
	assertPartyAndQuantity(t, database, stone, 1, 1)
	var caught int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_pokedex WHERE character_id=1 AND pokemon_id=26`).Scan(&caught); err != nil {
		t.Fatal(err)
	}
	if caught != 0 {
		t.Fatal("Pokedex registration escaped rollback")
	}
	testdb.Exec(t, database, `ALTER TABLE character_pokemon DROP CONSTRAINT fail_evolution`)
	result, err := service.UsePartyItem(context.Background(), 1, stone, 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Party[0].ID != 26 || result.Party[0].RowID != before[0].RowID || result.Party[0].Name != "Buddy" {
		t.Fatalf("evolution identity=%+v", result.Party[0])
	}
	if err := database.QueryRow(`SELECT caught FROM character_pokedex WHERE character_id=1 AND pokemon_id=26`).Scan(&caught); err != nil || caught != 1 {
		t.Fatalf("caught=%d %v", caught, err)
	}
}

func TestFluteSavesWholePartyWithoutConsumingItem(t *testing.T) {
	database, service := itemDatabase(t)
	flute := grant(t, database, 5, 1)
	testdb.Exec(t, database, `UPDATE character_pokemon SET status=6 WHERE character_id=1`)
	result, err := service.UsePartyItem(context.Background(), 1, flute, -1, -1)
	if err != nil || len(result.Party) != 1 || result.Party[0].Status != pokebattle.StatusNone || result.NewQuantity != 1 {
		t.Fatalf("flute=%+v %v", result, err)
	}
	assertPartyAndQuantity(t, database, flute, 1, 1)
}
