package world

import (
	"context"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func seedRoute(t *testing.T, wh *WorldHandler, path []PathNode) {
	t.Helper()
	if err := db.Transaction(context.Background(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, 42); err != nil {
			return err
		}
		return saveMovementRouteIn(tx, 42, 50, 7, 8, path, false)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMovementRouteProgressRollbackFreshOwnerAndCompletion(t *testing.T) {
	wh, ses, _ := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(50,9,8,1,1)`)
	seedRoute(t, wh, []PathNode{{X: 8, Y: 8}, {X: 9, Y: 8}})
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_route_progress() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject route progress'; END $$; CREATE CONSTRAINT TRIGGER reject_route_progress AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.x=8) EXECUTE FUNCTION reject_route_progress();`)
	candidate := movementStepCandidate{SourceMap: 50, SourceX: 7, SourceY: 8, MapID: 50, X: 8, Y: 8, Direction: "RIGHT", Forced: true}
	if _, err := commitMovementStep(context.Background(), wh, 42, candidate); err == nil {
		t.Fatal("failed route progress committed")
	}
	route, err := readMovementRoute(context.Background(), wh.database, 42)
	if err != nil || route.X != 7 || len(route.Path) != 2 {
		t.Fatalf("rollback changed cursor: %+v %v", route, err)
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_route_progress ON character_data`)
	result, err := commitMovementStep(context.Background(), wh, 42, candidate)
	if err != nil {
		t.Fatal(err)
	}
	publishCommittedPlayerPosition(ses, wh, result.MapID, result.X, result.Y, result.Direction)
	fresh := NewPlayerMovementManager(wh, wh.ActorManager)
	wh.PlayerMovement = fresh
	fresh.RegisterPlayer(ses, 42, 8, 8, 50, "RIGHT")
	if err := fresh.restoreMovementRoute(ses); err != nil {
		t.Fatal(err)
	}
	if len(fresh.players[42].Path) != 1 || fresh.players[42].Path[0].X != 9 {
		t.Fatal("fresh owner did not recover remaining point")
	}
	if _, err := commitMovementStep(context.Background(), wh, 42, candidate); err == nil {
		t.Fatal("old point replay committed")
	}
	candidate.SourceX, candidate.X = 8, 9
	if _, err := commitMovementStep(context.Background(), wh, 42, candidate); err != nil {
		t.Fatal(err)
	}
	route, err = readMovementRoute(context.Background(), wh.database, 42)
	if err != nil || route != nil {
		t.Fatalf("finished route remains: %+v %v", route, err)
	}
}

func TestMovementRouteRetirementPreservesButTeleportAndBattleClear(t *testing.T) {
	for _, operation := range []string{"retire projection", "same-tile teleport", "battle"} {
		t.Run(operation, func(t *testing.T) {
			wh, ses, _ := setupIssuedStep(t)
			seedRoute(t, wh, []PathNode{{X: 8, Y: 8}})
			switch operation {
			case "retire projection":
				wh.PlayerMovement.unregisterPlayer(42)
			case "same-tile teleport":
				if err := db.Transaction(context.Background(), wh.database, func(tx db.DBTX) error {
					if err := db.LockCharacter(tx, 42); err != nil {
						return err
					}
					return saveFieldDestinationIn(tx, 42, 50, 7, 8)
				}); err != nil {
					t.Fatal(err)
				}
			case "battle":
				battleTestStart(t, wh.database, false, nil)
				if err := wh.PlayerMovement.restoreMovementRoute(ses); err != nil {
					t.Fatal(err)
				}
			}
			route, err := readMovementRoute(context.Background(), wh.database, 42)
			if err != nil || (operation == "retire projection") != (route != nil) {
				t.Fatalf("cursor lifetime wrong: %+v %v", route, err)
			}
		})
	}
}

func TestMovementRouteRejectsMalformedAndMismatchedSource(t *testing.T) {
	for _, damage := range []string{`UPDATE character_movement_routes SET path_json='{"version":99,"path":[{"x":8,"y":8}]}'`, `UPDATE character_movement_routes SET path_json='{"version":1,"path":[{"x":8}]}'`, `UPDATE character_movement_routes SET x=6`} {
		t.Run(damage, func(t *testing.T) {
			wh, ses, _ := setupIssuedStep(t)
			seedRoute(t, wh, []PathNode{{X: 8, Y: 8}})
			testdb.Exec(t, wh.database, damage)
			if err := wh.PlayerMovement.restoreMovementRoute(ses); err == nil {
				t.Fatal("malformed cursor silently restored")
			}
			if len(wh.PlayerMovement.players[42].Path) != 0 {
				t.Fatal("malformed cursor installed")
			}
		})
	}
}

func TestMovementRouteSchemaRequiredAtStartup(t *testing.T) {
	wh, _, _ := setupIssuedStep(t)
	testdb.Exec(t, wh.database, `DROP TABLE character_movement_routes`)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := wh.PlayerMovement.Load(ctx); err == nil {
		t.Fatal("missing route schema accepted")
	}
}

func TestMovementRouteRestoresSurfTraversalMode(t *testing.T) {
	wh, ses, _ := setupIssuedStep(t)
	seedRoute(t, wh, []PathNode{{X: 8, Y: 8}})
	testdb.Exec(t, wh.database, `UPDATE phaser_tiles SET collision_type=2 WHERE map_id=50`)
	testdb.Exec(t, wh.database, `UPDATE character_movement_routes SET path_json='{"version":1,"surfing":true,"path":[{"x":8,"y":8}]}' WHERE character_id=42`)
	if err := wh.PlayerMovement.restoreMovementRoute(ses); err != nil {
		t.Fatal(err)
	}
	if !wh.PlayerMovement.players[42].IsSurfing || len(wh.PlayerMovement.players[42].Path) != 1 {
		t.Fatal("route lost its traversal mode")
	}
}

func TestNPCBlockedRouteRetiresDurableProgressAtSource(t *testing.T) {
	wh, fixture, messages := setupIssuedStep(t)
	ses := wh.sessionManager.CreateNextSession(messages, "", nil)
	ses.Client, ses.Authenticated = fixture.Client, true
	t.Cleanup(ses.Close)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "RIGHT")
	seedRoute(t, wh, []PathNode{{X: 8, Y: 8}})
	if err := wh.PlayerMovement.restoreMovementRoute(ses); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, wh.database, `INSERT INTO phaser_objects(id,map_id,name,object_type,sprite_name,x,y) VALUES(777,50,'BLOCKER','npc','SPRITE_RED',8,8)`)
	state := wh.PlayerMovement.players[42]
	state.LastMoveTime = time.Time{}
	if err := ses.ExecuteCommand(context.Background(), func() { wh.PlayerMovement.processCharacterTick(ses.CommandContext(), 42, state) }); err != nil {
		t.Fatal(err)
	}
	route, err := readMovementRoute(context.Background(), wh.database, 42)
	if err != nil || route != nil || len(state.Path) != 0 {
		t.Fatalf("blocked route survived: %+v %v", route, err)
	}
	assertStepPosition(t, wh, ses, 7)
}
