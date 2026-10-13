package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestCutPreparationFailureCannotPublishTransientStateOrFallbackArt(t *testing.T) {
	for _, kind := range []string{"durable-permission", "cached-permission", "missing-source", "missing-replacement", "sql-error", "durable-battle", "dynamic-override", "single-connection", "pool-timeout"} {
		t.Run(kind, func(t *testing.T) {
			database, wh, ses, messages := battleTestWorld(t)
			wh.CutTiles = NewCutTileManager()
			testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ROOM',10,10,0);UPDATE character_data SET map_id=50,x=0,y=0 WHERE id=42;
    INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES(15,'CUT','CUT','CUT','NO_ADDITIONAL_EFFECT',0,'NORMAL',0,15);UPDATE character_pokemon SET move1_id=15,move1_pp=15 WHERE character_id=42;
    INSERT INTO phaser_tile_images(id,image_path,tileset_id,raw_foot_tile_id) VALUES(100,'tree.png',0,61),(200,'ground.png',0,82);
    INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,raw_foot_tile_id,collision_type) VALUES(50,1,0,100,61,0);
    INSERT INTO phaser_tile_properties(tile_image_id,collision_type) VALUES(200,0)`)
			ses.Client.CharData().MapID = 99
			wh.EventFlags.flags[42] = map[string]bool{"EVENT_GOT_CASCADEBADGE": kind == "cached-permission"}
			if kind != "cached-permission" {
				testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_CASCADEBADGE')`)
			}
			switch kind {
			case "missing-source":
				testdb.Exec(t, database, `DELETE FROM phaser_tile_images WHERE id=100`)
			case "missing-replacement":
				testdb.Exec(t, database, `DELETE FROM phaser_tile_images WHERE id=200`)
			case "sql-error":
				testdb.Exec(t, database, `ALTER TABLE phaser_tile_images RENAME TO unavailable_images`)
			case "durable-battle":
				battleTestStart(t, database, false, nil)
				forgetBattle(42, getBattle(42))
			case "dynamic-override":
				testdb.Exec(t, database, `INSERT INTO phaser_event_tile_overrides(map_id,map_name,x,y,tile_image_id,collision_type) VALUES(50,'ROOM',1,0,200,1)`)
			case "single-connection":
				database.SetMaxOpenConns(1)
			}
			previous := db.GlobalWorldDB
			db.GlobalWorldDB = nil
			t.Cleanup(func() { db.GlobalWorldDB = previous })
			request := `{"moveName":"CUT","mapId":50,"targetX":1,"targetY":0}`
			if kind == "pool-timeout" {
				database.SetMaxOpenConns(1)
				held, err := database.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				defer held.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
				before := database.Stats().WaitCount
				err = ses.ExecuteCommand(ctx, func() { HandleFieldMoveUse(ses, []byte(request), wh) })
				if err == nil || ctx.Err() != context.DeadlineExceeded || database.Stats().WaitCount <= before {
					t.Fatalf("CUT escaped pool deadline: %v", err)
				}
				held.Close()
			} else {
				battleDispatch(t, wh, ses, opcodes.FieldMoveUseRequest, request)
			}
			var response FieldMoveUseResponsePayload
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil {
				t.Fatal("missing CUT outcome")
			}
			want := kind == "durable-permission" || kind == "single-connection"
			if response.Success != want {
				t.Fatalf("kind=%s response=%+v", kind, response)
			}
			overrides := wh.CutTiles.CollisionOverrides(42, 50)
			if want {
				if response.Tile == nil || response.Tile.TileImageID != 200 || response.Tile.CollisionType != collisionLand || overrides[tileKey(1, 0)] != collisionLand {
					t.Fatalf("inconsistent art/collision: %+v %+v", response, overrides)
				}
			} else if len(overrides) != 0 || response.Tile != nil {
				t.Fatalf("failed read published transient CUT: %+v %+v", response, overrides)
			}
		})
	}
}
