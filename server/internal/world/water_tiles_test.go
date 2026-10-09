package world

import (
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSurfableWaterRejectsWarpMats(t *testing.T) {
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			37: {
				"2,7": collisionWater,
				"4,7": collisionWater,
			},
		},
	}
	warpManager := newPhaserWarpManager(nil)
	addPhaserWarpIndex(warpManager.byMap, 37, &phaserMapWarp{
		SourceMapID: 37,
		X:           2,
		Y:           7,
		WarpType:    "carpet",
	})
	wh := &WorldHandler{
		ActorManager: actorManager,
		phaserWarps:  warpManager,
	}

	if surfable, err := isSurfableWaterTile(context.Background(), wh, 37, 2, 7); err != nil || surfable {
		t.Fatalf("warp mat with water collision should not be surfable")
	}
	if surfable, err := isSurfableWaterTile(context.Background(), wh, 37, 4, 7); err != nil || !surfable {
		t.Fatalf("plain water collision tile should be surfable")
	}
}

func TestWaterPreflightReportsOwnedReadFailure(t *testing.T) {
	for _, family := range []string{"fishing", "surf"} {
		for _, failure := range []string{"pool-cancel", "missing-source"} {
			t.Run(family+"/"+failure, func(t *testing.T) {
				wh, ses, messages := setupIssuedStep(t)
				if family == "fishing" {
					wh.PlayerMovement.mu.Lock()
					wh.PlayerMovement.players[42].Direction = "RIGHT"
					wh.PlayerMovement.mu.Unlock()
					testdb.Exec(t, wh.database, `INSERT INTO cq_items(id,name,short_name) VALUES(76,'Old Rod','OLD_ROD'); INSERT INTO cq_item_instances(id,item_id,quantity,owner_type,owner_id) VALUES(100,76,1,0,42); INSERT INTO cq_character_inventory(character_id,item_instance_id) VALUES(42,100)`)
				}
				wh.ActorManager.InvalidateCollisionMap(50)
				old := db.GlobalWorldDB
				db.GlobalWorldDB = nil
				t.Cleanup(func() { db.GlobalWorldDB = old })
				ctx := context.Background()
				before := wh.database.Stats().WaitCount
				var release func()
				if failure == "pool-cancel" {
					wh.database.SetMaxOpenConns(1)
					lease, err := wh.database.Conn(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					release = func() { lease.Close() }
					defer release()
					owned, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
					defer cancel()
					ctx = owned
				} else {
					testdb.Exec(t, wh.database, `ALTER TABLE phaser_tiles RENAME TO unavailable_tiles`)
				}
				err := ses.ExecuteCommand(ctx, func() {
					if family == "fishing" {
						HandlePokeFishing(ses, []byte(`{"requestId":"fishing:water","characterId":42,"instanceId":100,"commandRevision":0,"rodType":"old_rod","mapId":50,"x":7,"y":8,"direction":"RIGHT"}`), wh)
					} else {
						x, y := 8, 8
						handlePokeSurfingTarget(ses, wh, 42, 50, 7, 8, PokeSurfingRequestPayload{TargetX: &x, TargetY: &y, Direction: "RIGHT"})
					}
				})
				if failure == "pool-cancel" {
					if !errors.Is(err, context.DeadlineExceeded) || wh.database.Stats().WaitCount <= before {
						t.Fatalf("preflight escaped owned pool wait: %v", err)
					}
					release()
				} else if err != nil {
					t.Fatal(err)
				}
				if len(messages.streams) != 1 {
					t.Fatalf("preflight packets=%+v", messages.streams)
				}
				var response struct {
					Success bool
					Error   string
				}
				if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil {
					t.Fatal(err)
				}
				wanted := "Unable to read fishing state."
				if family == "surf" {
					wanted = "Unable to read water."
				}
				if response.Success || response.Error != wanted {
					t.Fatalf("read failure disguised as gameplay result: %+v", response)
				}
				assertStepPosition(t, wh, ses, 7)
				if getBattle(42) != nil {
					t.Fatal("failed preflight started a battle")
				}
				if _, ok := wh.ActorManager.collisionMap[50]; ok {
					t.Fatal("failed preflight warmed collision cache")
				}
			})
		}
	}
}
