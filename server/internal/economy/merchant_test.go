package economy

import (
	"context"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestMerchantMenuUsesOwnedMapAndCombinedOffers(t *testing.T) {
	database, service := shopDatabase(t)
	testdb.Exec(t, database, `INSERT INTO cq_merchants(id,name,map_id) VALUES(2,'TMs',38),(3,'Remote',39);
		INSERT INTO cq_merchant_items(merchant_id,item_id) VALUES(2,2),(3,2)`)
	menu, err := service.Open(context.Background(), 1, 38, 0)
	if err != nil || menu.MerchantID != 1 || len(menu.Items) != 2 || menu.Money != 1000 || menu.Items[1].MerchantID != 2 {
		t.Fatalf("combined menu=%+v err=%v", menu, err)
	}
	menu, err = service.Open(context.Background(), 1, 38, 2)
	if err != nil || menu.MerchantID != 2 || len(menu.Items) != 1 {
		t.Fatalf("selected menu=%+v err=%v", menu, err)
	}
	for _, args := range [][3]int32{{1, 38, 3}, {1, 40, 0}, {99, 38, 0}} {
		if menu, err := service.Open(context.Background(), args[0], args[1], args[2]); err == nil || menu.Items != nil {
			t.Fatalf("accepted unavailable menu %+v: %+v %v", args, menu, err)
		}
	}
	// The importer's linked ID is authoritative even when a stale name matches.
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(40,'STALE',1,1);
		UPDATE cq_merchants SET map_name='STALE' WHERE id=3`)
	if _, err := service.Open(context.Background(), 1, 40, 0); err == nil {
		t.Fatal("display name granted a remote shop")
	}
}

func TestMerchantReadFailuresNeverPublishPartialMenu(t *testing.T) {
	// Disable the disposable fixture's constraint only to exercise the reader's
	// independent validation of corrupt legacy data; production keeps it intact.
	for _, failure := range []string{"DROP TABLE cq_merchant_items", "DROP TABLE character_wallet", "ALTER TABLE character_wallet DROP CONSTRAINT character_wallet_pokedollars_check; UPDATE character_wallet SET pokedollars=-1"} {
		t.Run(failure, func(t *testing.T) {
			database, service := shopDatabase(t)
			testdb.Exec(t, database, failure)
			if menu, err := service.Open(context.Background(), 1, 38, 0); err == nil || menu.Items != nil || menu.MerchantID != 0 {
				t.Fatalf("partial menu=%+v error=%v", menu, err)
			}
		})
	}
}

func TestMerchantReadCancellationWhileWaitingForCharacter(t *testing.T) {
	database, service := shopDatabase(t)
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`SELECT id FROM character_data WHERE id=1 FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	if menu, err := service.Open(ctx, 1, 38, 0); err == nil || menu.Items != nil {
		t.Fatalf("cancelled read=%+v %v", menu, err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("reader ignored caller deadline")
	}
}

func TestMerchantEmptyOffersAndMissingWalletAreExplicit(t *testing.T) {
	database, service := shopDatabase(t)
	testdb.Exec(t, database, `DELETE FROM cq_merchant_items; DELETE FROM character_wallet;
		CREATE FUNCTION reject_menu_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'read wrote character'; END $$;
		CREATE TRIGGER reject_menu_write BEFORE UPDATE ON character_data FOR EACH ROW EXECUTE FUNCTION reject_menu_write()`)
	menu, err := service.Open(context.Background(), 1, 38, 0)
	if err != nil || menu.Items == nil || len(menu.Items) != 0 || menu.Money != 0 {
		t.Fatalf("empty read=%+v %v", menu, err)
	}
}
