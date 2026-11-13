package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestElevatorTravelJoinsDurableSourcePermissionAndCommit(t *testing.T) {
	for _, kind := range []string{"success", "stale-source", "missing-flag", "catalog-error", "commit-failure", "durable-battle", "pool-cancel", "flag-error"} {
		t.Run(kind, func(t *testing.T) {
			database, _, ses, _ := battleTestWorld(t)
			testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;
 INSERT INTO phaser_elevator_floors(elevator_map_id,floor_map_id,floor_label,dest_x,dest_y,sort_order,requires_flag) VALUES(50,60,'2F',3,4,2,'EVENT_ELEVATOR_READY');
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_ELEVATOR_READY')`)
			database.SetMaxOpenConns(1)
			switch kind {
			case "stale-source":
				testdb.Exec(t, database, `UPDATE character_data SET x=8 WHERE id=42`)
			case "missing-flag":
				testdb.Exec(t, database, `DELETE FROM character_event_flags WHERE character_id=42`)
			case "flag-error":
				testdb.Exec(t, database, `ALTER TABLE character_event_flags RENAME TO unavailable_elevator_flags`)
			case "catalog-error":
				testdb.Exec(t, database, `ALTER TABLE phaser_elevator_floors RENAME TO unavailable_elevator_catalog`)
			case "commit-failure":
				testdb.Exec(t, database, `CREATE FUNCTION reject_elevator_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late elevator failure';END $$; CREATE CONSTRAINT TRIGGER reject_elevator_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.map_id=60) EXECUTE FUNCTION reject_elevator_commit()`)
			case "durable-battle":
				battleTestStart(t, database, false, nil)
				forgetBattle(42, getBattle(42))
			}
			old := db.GlobalWorldDB
			db.GlobalWorldDB = nil
			t.Cleanup(func() { db.GlobalWorldDB = old })
			ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 50, 7, 8
			ctx := context.Background()
			release := func() {}
			if kind == "pool-cancel" {
				held, err := database.Conn(ctx)
				if err != nil {
					t.Fatal(err)
				}
				limited, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()
				ctx = limited
				release = func() { held.Close() }
				defer release()
			}
			floor, err := UseElevator(ctx, database, 42, 50, 7, 8, 60, func(id int) int { return id })
			if (err == nil) != (kind == "success") {
				t.Fatalf("floor=%+v err=%v", floor, err)
			}
			if kind == "pool-cancel" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("pool deadline %v", err)
				}
				release()
			}
			var m, x, y int
			if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&m, &x, &y); err != nil {
				t.Fatal(err)
			}
			if kind == "success" {
				if m != 60 || x != 3 || y != 4 {
					t.Fatalf("saved %d,%d,%d", m, x, y)
				}
			} else {
				wantX := 7
				if kind == "stale-source" {
					wantX = 8
				}
				if m != 50 || x != wantX || y != 8 {
					t.Fatalf("failed travel changed pose %d,%d,%d", m, x, y)
				}
			}
		})
	}
}

func TestElevatorDispatcherPublishesOnlyCommittedDestination(t *testing.T) {
	for _, rejectCommit := range []bool{false, true} {
		t.Run(fmt.Sprint(rejectCommit), func(t *testing.T) {
			database, wh, ses, messages := battleTestWorld(t)
			testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;INSERT INTO phaser_elevator_floors(elevator_map_id,floor_map_id,floor_label,dest_x,dest_y) VALUES(50,60,'2F',3,4); INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ELEVATOR',10,10,0),(60,'FLOOR',10,10,0);INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text) VALUES(100,50,7,7,'sign','PANEL','PANEL')`)
			if rejectCommit {
				testdb.Exec(t, database, `CREATE FUNCTION reject_elevator_publication() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late elevator failure';END $$;CREATE CONSTRAINT TRIGGER reject_elevator_publication AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.map_id=60) EXECUTE FUNCTION reject_elevator_publication()`)
			}
			ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 50, 7, 8
			wh.sessionManager = session.NewSessionManager()
			wh.ActorRegistry = NewActorRegistry()
			wh.ActorManager = NewPhaserActorManager(wh)
			wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
			wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
			battleDispatch(t, wh, ses, opcodes.ElevatorSelectRequest, `{"requestId":"elevator-test","characterId":42,"mapId":50,"fromX":7,"fromY":8,"commandRevision":0,"floorMapId":60}`)
			x, y, m, ok := wh.PlayerMovement.GetPosition(42)
			if !ok {
				t.Fatal("lost movement owner")
			}
			if rejectCommit {
				if m != 50 || x != 7 || y != 8 || ses.Client.CharData().MapID != 50 {
					t.Fatal("failed commit published destination")
				}
				for _, packet := range messages.streams {
					if packet.opcode == opcodes.WarpTileTeleportNotify {
						t.Fatal("failed commit sent warp")
					}
				}
			} else {
				if m != 60 || x != 3 || y != 4 || ses.Client.CharData().MapID != 60 {
					t.Fatal("committed destination not published")
				}
				if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.ElevatorFloorsResponse {
					t.Fatalf("warp messages %+v", messages.streams)
				}

				historical := string(messages.streams[0].payload)
				testdb.Exec(t, database, `UPDATE character_data SET x=9,y=8 WHERE id=42;DELETE FROM phaser_objects WHERE id=100;DELETE FROM phaser_elevator_floors`)
				publishCommittedPlayerPosition(ses, wh, 60, 9, 8, "DOWN")
				messages.streams = nil
				battleDispatch(t, wh, ses, opcodes.ElevatorSelectRequest, `{"requestId":"elevator-test","characterId":42,"mapId":50,"fromX":7,"fromY":8,"commandRevision":0,"floorMapId":60}`)
				if len(messages.streams) != 1 || string(messages.streams[0].payload) != historical {
					t.Fatal("historical elevator receipt not replayed")
				}
				x, y, m, ok = wh.PlayerMovement.GetPosition(42)
				if !ok || m != 60 || x != 9 || y != 8 {
					t.Fatal("replay replaced current live pose")
				}
				var revision, savedX int
				if err := database.QueryRow(`SELECT revision FROM character_field_command_state WHERE character_id=42 AND domain='travel'`).Scan(&revision); err != nil || revision != 1 {
					t.Fatalf("replay revision %d %v", revision, err)
				}
				if err := database.QueryRow(`SELECT x FROM character_data WHERE id=42`).Scan(&savedX); err != nil || savedX != 9 {
					t.Fatal("replay replaced durable pose")
				}
			}
		})
	}
}

func TestElevatorPanelUsesOwnedActorReachAndVisibility(t *testing.T) {
	for _, kind := range []string{"valid", "remote", "hidden", "moved", "ambiguous", "missing"} {
		t.Run(kind, func(t *testing.T) {
			database, wh, ses, messages := battleTestWorld(t)
			wh.ActorRegistry = NewActorRegistry()
			wh.ActorManager = NewPhaserActorManager(wh)
			testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ELEVATOR',10,10,0);UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text) VALUES(100,50,7,7,'sign','PANEL','PANEL');INSERT INTO phaser_elevator_floors(elevator_map_id,floor_map_id,floor_label,dest_x,dest_y) VALUES(50,60,'2F',3,4)`)
			switch kind {
			case "remote":
				testdb.Exec(t, database, `UPDATE character_data SET x=1,y=1 WHERE id=42`)
				ses.Client.CharData().X, ses.Client.CharData().Y = 1, 1
			case "hidden":
				testdb.Exec(t, database, `INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,100,false,'test')`)
			case "moved":
				testdb.Exec(t, database, `INSERT INTO character_object_positions(character_id,object_id,map_id,x,y) VALUES(42,100,50,1,1)`)
			case "ambiguous":
				testdb.Exec(t, database, `INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text) VALUES(101,50,7,9,'sign','OTHER','OTHER')`)
			case "missing":
				testdb.Exec(t, database, `DELETE FROM phaser_objects WHERE id=100`)
			}
			ses.Client.CharData().MapID = 50
			if kind != "remote" {
				ses.Client.CharData().X, ses.Client.CharData().Y = 7, 8
			}
			database.SetMaxOpenConns(1)
			battleDispatch(t, wh, ses, opcodes.ElevatorFloorsRequest, `{"requestId":"elevator-read","characterId":42,"mapId":50,"fromX":7,"fromY":8}`)
			var reply ElevatorResponse
			if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success != (kind == "valid") {
				t.Fatalf("panel %s messages %+v", kind, messages.streams)
			}
			if len(messages.streams) == 1 && messages.streams[0].opcode != opcodes.ElevatorFloorsResponse {
				t.Fatal("unexpected panel reply")
			}
			messages.streams = nil
			if kind != "valid" {
				battleDispatch(t, wh, ses, opcodes.ElevatorSelectRequest, `{"requestId":"elevator-test","characterId":42,"mapId":50,"fromX":7,"fromY":8,"commandRevision":0,"floorMapId":60}`)
			}
			if kind != "valid" {
				var m int
				if err := database.QueryRow(`SELECT map_id FROM character_data WHERE id=42`).Scan(&m); err != nil || m != 50 {
					t.Fatalf("unavailable panel traveled: %d %v", m, err)
				}
			}
		})
	}
}

func TestElevatorRejectsIdentityFreeAndWrongCharacterEnvelopes(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	for _, request := range []struct {
		opcode  opcodes.OpCode
		payload string
	}{
		{opcodes.ElevatorFloorsRequest, `{"mapId":50}`},
		{opcodes.ElevatorSelectRequest, `{"floorMapId":60}`},
		{opcodes.ElevatorSelectRequest, `{"requestId":"wrong","characterId":99,"mapId":50,"fromX":7,"fromY":8,"commandRevision":0,"floorMapId":60}`},
	} {
		messages.streams = nil
		battleDispatch(t, wh, ses, request.opcode, request.payload)
		var reply struct{ Success bool }
		if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success {
			t.Fatalf("invalid elevator accepted %+v", messages.streams)
		}
	}
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_field_command_state WHERE character_id=42 AND domain='travel'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("invalid elevator mutated revision %d %v", count, err)
	}
}
