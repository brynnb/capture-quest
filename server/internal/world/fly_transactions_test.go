package world

import (
	"encoding/json"
	"fmt"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
)

func TestFlyDispatcherValidatesDurableEligibilityAndCatalogBeforeCommit(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES(19,'FLY','FLY','FLY','FLY_EFFECT',70,'FLYING',95,15);
 INSERT INTO poke_start_cities(id,map_id,name,spawn_x,spawn_y) VALUES(1,60,'Town',3,4);
 UPDATE character_data SET map_id=50,x=7,y=8 WHERE id=42;`)
	ses.Client.CharData().MapID, ses.Client.CharData().X, ses.Client.CharData().Y = 50, 7, 8
	wh.PlayerMovement = NewPlayerMovementManager(wh, nil)
	wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "UP")
	db.GlobalWorldDB = nil
	deny := func(mapID, x, y int, want string) {
		t.Helper()
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.FieldMoveUseRequest, fmt.Sprintf(`{"moveName":"FLY","mapId":%d,"targetX":%d,"targetY":%d}`, mapID, x, y))
		if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.FieldMoveUseResponse {
			t.Fatalf("denial messages %+v", messages.streams)
		}
		var response FieldMoveUseResponsePayload
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success || response.Error != want {
			t.Fatalf("denial %+v, want %q: %v", response, want, err)
		}
		var savedMap int
		var savedX, savedY float64
		if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&savedMap, &savedX, &savedY); err != nil || savedMap != 50 || savedX != 7 || savedY != 8 {
			t.Fatalf("denial saved position (%d,%v,%v): %v", savedMap, savedX, savedY, err)
		}
		mx, my, mm, ok := wh.PlayerMovement.GetPosition(42)
		char := ses.Client.CharData()
		if !ok || mx != 7 || my != 8 || mm != 50 || char.MapID != 50 || char.X != 7 || char.Y != 8 {
			t.Fatalf("denial changed live position (%d,%d,%d), char %+v", mm, mx, my, char)
		}
	}
	deny(60, 3, 4, "No POKEMON knows that move.")
	testdb.Exec(t, database, `UPDATE character_pokemon SET move1_id=19,move1_pp=15 WHERE character_id=42`)
	deny(60, 3, 4, NewBadgeRequiredMessage)
	// The cache remains stale; durable eligibility must determine the effect.
	testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_THUNDERBADGE')`)
	deny(9999, 3, 4, "Choose a valid FLY destination.")
	deny(60, -999, 4, "Choose a valid FLY destination.")
	testdb.Exec(t, database, `INSERT INTO poke_start_cities(id,map_id,name,spawn_x,spawn_y) VALUES(2,60,'Duplicate',3,4)`)
	deny(60, 3, 4, "Could not FLY. Please try again.")
	testdb.Exec(t, database, `DELETE FROM poke_start_cities WHERE id=2;
 CREATE FUNCTION reject_fly_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject fly commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_fly_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN (NEW.map_id=60) EXECUTE FUNCTION reject_fly_commit();`)
	deny(60, 3, 4, "Could not FLY. Please try again.")
	testdb.Exec(t, database, `DROP TRIGGER reject_fly_commit ON character_data`)
	setBattle(42, &pokebattle.BattleState{Phase: pokebattle.PhaseActionSelect})
	deny(60, 3, 4, "Field moves can't be used during a battle.")
	forgetBattle(42, getBattle(42))
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.FieldMoveUseRequest, `{"moveName":"FLY","mapId":60,"targetX":3,"targetY":4}`)
	if len(messages.streams) != 2 || messages.streams[0].opcode != opcodes.WarpTileTeleportNotify || messages.streams[1].opcode != opcodes.FieldMoveUseResponse {
		t.Fatalf("success messages %+v", messages.streams)
	}
	var response FieldMoveUseResponsePayload
	if err := json.Unmarshal(messages.streams[1].payload, &response); err != nil || !response.Success || response.MapID != 60 || response.TargetX != 3 || response.TargetY != 4 {
		t.Fatalf("success response %+v: %v", response, err)
	}
	var savedMap int
	var savedX, savedY float64
	if err := database.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=42`).Scan(&savedMap, &savedX, &savedY); err != nil || savedMap != 60 || savedX != 3 || savedY != 4 {
		t.Fatalf("success saved position (%d,%v,%v): %v", savedMap, savedX, savedY, err)
	}
	mx, my, mm, ok := wh.PlayerMovement.GetPosition(42)
	if !ok || mx != 3 || my != 4 || mm != 60 || ses.Client.CharData().MapID != 60 {
		t.Fatalf("success live position (%d,%d,%d)", mm, mx, my)
	}
}
