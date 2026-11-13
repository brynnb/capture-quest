package world

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

func TestDurableEffectsRespectCallerCancellationWhileCharacterLocked(t *testing.T) {
	for _, name := range []string{"SafariEntry", "SafariTurn", "RepelActivation", "CutsceneCompletion", "ScriptedBattleStart"} {
		t.Run(name, func(t *testing.T) {
			database, wh, _, _ := battleTestWorld(t)
			wh.Safari = NewSafariZoneManager(database)
			wh.WildEncounter = NewWildEncounterManager(wh, database)
			testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_usable,stackable,stack_size) VALUES(30,'Repel','REPEL',true,true,99)`)
			instance, err := cqitems.NewStore(database).AddItemToInventory(42, 30, 2)
			if err != nil {
				t.Fatal(err)
			}
			if name == "SafariTurn" {
				seedSafariBattle(t, wh.Safari, 30)
			}
			var safariIdentity BattleCommandIdentity
			if name == "SafariTurn" {
				visit, err := wh.Safari.GetSession(context.Background(), 42)
				if err != nil {
					t.Fatal(err)
				}
				safariIdentity = BattleCommandIdentity{BattleID: visit.Battle.BattleID, Revision: visit.Battle.Revision}
			}
			lock, err := database.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Rollback()
			if _, err := lock.Exec(`UPDATE character_data SET id=id WHERE id=42`); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				switch name {
				case "SafariEntry":
					_, err = TryStartSafariZoneVisit(ctx, 42, wh.Safari)
				case "SafariTurn":
					_, err = wh.Safari.act(ctx, 42, "ball", safariIdentity)
				case "RepelActivation":
					_, err = UseRepelInventoryItem(ctx, wh, 42, instance, 0)
				case "CutsceneCompletion":
					_, _, err = ApplyCutsceneScript(ctx, CutsceneActionContext{Database: database}, &CutsceneScript{SetsFlags: []string{"CANCELLED_REWARD"}, Actions: json.RawMessage(`[{"type":"giveItem","itemId":1,"quantity":1}]`)}, 42)
				case "ScriptedBattleStart":
					_, _, err = StartScriptedWildBattle(ctx, database, 42, ScriptedWildBattleSpec{PokemonID: 129, Level: 5})
				}
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil || ctx.Err() == nil {
					t.Fatalf("cancelled effect=%v context=%v", err, ctx.Err())
				}
			case <-time.After(time.Second):
				t.Fatal("effect replaced caller deadline with an independent timeout")
			}
			if err := lock.Rollback(); err != nil {
				t.Fatal(err)
			}
			var money, repels, battles, flags, rewards int
			if err := database.QueryRow(`SELECT pokedollars,
    (SELECT count(*) FROM character_repels WHERE character_id=42),
    (SELECT count(*) FROM character_battle_state WHERE character_id=42),
    (SELECT count(*) FROM character_event_flags WHERE character_id=42),
    (SELECT count(*) FROM cq_character_inventory ci JOIN cq_item_instances ii ON ii.id=ci.item_instance_id WHERE ci.character_id=42 AND ii.item_id=1)
    FROM character_wallet WHERE character_id=42`).Scan(&money, &repels, &battles, &flags, &rewards); err != nil {
				t.Fatal(err)
			}
			if money != 100 || repels != 0 || battles != 0 || flags != 0 || rewards != 0 {
				t.Fatalf("cancelled effect changed state: money=%d repels=%d battles=%d flags=%d rewards=%d", money, repels, battles, flags, rewards)
			}
			item, err := cqitems.NewStore(database).FindInventoryItemByInstanceID(42, instance)
			if err != nil || item == nil || item.Instance.Quantity != 2 {
				t.Fatalf("cancelled effect consumed repel: %+v %v", item, err)
			}
			visit, err := wh.Safari.GetSession(context.Background(), 42)
			if err != nil {
				t.Fatal(err)
			}
			if name == "SafariTurn" {
				if visit == nil || visit.BallsLeft != 30 || visit.Battle == nil || visit.Battle.BallsLeft != 30 {
					t.Fatalf("cancelled turn changed visit: %+v", visit)
				}
			} else if visit != nil {
				t.Fatalf("cancelled effect created Safari: %+v", visit)
			}
			if getBattle(42) != nil {
				t.Fatal("cancelled battle published a cache snapshot")
			}
		})
	}
}
