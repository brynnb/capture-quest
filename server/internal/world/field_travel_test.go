package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"encoding/json"
	"fmt"
	"testing"
)

func TestAbilityTravelPermissionSourceAndDestinationShareCommit(t *testing.T) {
	for _, move := range []struct {
		name string
		id   int
	}{{"DIG", 91}, {"TELEPORT", 100}} {
		for _, kind := range []string{"success", "missing-move", "stale-source", "destination-read-failure", "late-failure", "durable-battle", "single-connection"} {
			t.Run(move.name+"/"+kind, func(t *testing.T) {
				database, wh, ses, messages := battleTestWorld(t)
				testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ROOM',10,10,0),(60,'SAFE',10,10,1);UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;INSERT INTO phaser_warps(source_map_id,x,y,destination_map_id,destination_x,destination_y) VALUES(50,0,0,60,3,4);INSERT INTO character_bind(id,slot,map_id,x,y,z,heading) VALUES(42,0,60,3,4,0,180)`)
				testdb.Exec(t, database, `INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES($1,$2,$2,$2,'NO_ADDITIONAL_EFFECT',0,'NORMAL',0,15)`, move.id, move.name)
				if kind != "missing-move" {
					testdb.Exec(t, database, `UPDATE character_pokemon SET move1_id=$1,move1_pp=15 WHERE character_id=42`, move.id)
				}
				ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 50, 7, 8
				switch kind {
				case "stale-source":
					testdb.Exec(t, database, `UPDATE character_data SET x=8 WHERE id=42`)
				case "destination-read-failure":
					if move.name == "DIG" {
						testdb.Exec(t, database, `ALTER TABLE phaser_warps RENAME TO unavailable_warps`)
					} else {
						testdb.Exec(t, database, `ALTER TABLE character_bind RENAME TO unavailable_bind`)
					}
				case "late-failure":
					testdb.Exec(t, database, `CREATE FUNCTION reject_ability_travel() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late travel failure';END $$;CREATE CONSTRAINT TRIGGER reject_ability_travel AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.map_id=60) EXECUTE FUNCTION reject_ability_travel()`)
				case "durable-battle":
					battleTestStart(t, database, false, nil)
					forgetBattle(42, getBattle(42))
				case "single-connection":
					database.SetMaxOpenConns(1)
				}
				previous := db.GlobalWorldDB
				db.GlobalWorldDB = nil
				t.Cleanup(func() { db.GlobalWorldDB = previous })
				battleDispatch(t, wh, ses, opcodes.FieldMoveUseRequest, fmt.Sprintf(`{"moveName":%q}`, move.name))
				want := kind == "success" || kind == "single-connection"
				var response FieldMoveUseResponsePayload
				for _, packet := range messages.streams {
					if packet.opcode == opcodes.FieldMoveUseResponse {
						if err := json.Unmarshal(packet.payload, &response); err != nil {
							t.Fatal(err)
						}
					} else if !want && packet.opcode == opcodes.WarpTileTeleportNotify {
						t.Fatal("failed travel published a destination")
					}
				}
				if response.Success != want {
					t.Fatalf("response=%+v kind=%s", response, kind)
				}
				var mapID, x, y int
				if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&mapID, &x, &y); err != nil {
					t.Fatal(err)
				}
				if want {
					if mapID != 60 || x != 3 || y != 4 {
						t.Fatalf("destination=%d %d %d", mapID, x, y)
					}
				} else {
					expectedX := 7
					if kind == "stale-source" {
						expectedX = 8
					}
					if mapID != 50 || x != expectedX || y != 8 {
						t.Fatalf("failed travel mutated pose=%d %d %d", mapID, x, y)
					}
				}
			})
		}
	}
}
