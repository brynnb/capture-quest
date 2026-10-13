package world

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
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
			testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;INSERT INTO phaser_elevator_floors(elevator_map_id,floor_map_id,floor_label,dest_x,dest_y) VALUES(50,60,'2F',3,4)`)
			if rejectCommit {
				testdb.Exec(t, database, `CREATE FUNCTION reject_elevator_publication() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late elevator failure';END $$;CREATE CONSTRAINT TRIGGER reject_elevator_publication AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.map_id=60) EXECUTE FUNCTION reject_elevator_publication()`)
			}
			ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 50, 7, 8
			wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
			wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
			battleDispatch(t, wh, ses, opcodes.ElevatorSelectRequest, `{"floorMapId":60}`)
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
				if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.WarpTileTeleportNotify {
					t.Fatalf("warp messages %+v", messages.streams)
				}
			}
		})
	}
}
