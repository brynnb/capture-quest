package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
	"encoding/json"
	"fmt"
	"testing"
)

func TestStateFieldMovesUseDurablePermissionMapAndBattle(t *testing.T) {
	for _, move := range []struct {
		name, flag string
		id         int
	}{{"STRENGTH", "EVENT_GOT_RAINBOWBADGE", 70}, {"FLASH", "EVENT_GOT_BOULDERBADGE", 148}} {
		for _, kind := range []string{"durable-permission", "cached-permission", "durable-battle", "cached-battle", "late-failure", "single-connection"} {
			t.Run(move.name+"/"+kind, func(t *testing.T) {
				database, wh, ses, messages := battleTestWorld(t)
				testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ROOM',10,10,0); UPDATE character_data SET map_id=50 WHERE id=42`)
				testdb.Exec(t, database, `INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES($1,$2,$2,$2,'NO_ADDITIONAL_EFFECT',0,'NORMAL',0,15)`, move.id, move.name)
				testdb.Exec(t, database, `UPDATE character_pokemon SET move1_id=$1,move1_pp=15 WHERE character_id=42`, move.id)
				ses.Client.CharData().MapID = 99 // The activation belongs to the saved map.
				wh.EventFlags.flags[42] = map[string]bool{move.flag: kind == "cached-permission"}
				if kind != "cached-permission" {
					testdb.Exec(t, database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,$1)`, move.flag)
				}
				if kind == "durable-battle" {
					battleTestStart(t, database, false, nil)
					forgetBattle(42, getBattle(42))
				}
				if kind == "cached-battle" {
					setBattle(42, pokebattle.NewWildBattle(nil, &pokebattle.Pokemon{ID: 129, CurHP: 10, MaxHP: 10}))
				}
				if kind == "late-failure" {
					testdb.Exec(t, database, `CREATE FUNCTION reject_field_state() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late field state failure'; END $$;CREATE CONSTRAINT TRIGGER reject_field_state AFTER INSERT ON character_field_move_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_field_state()`)
				}
				if kind == "single-connection" {
					database.SetMaxOpenConns(1)
				}
				previous := db.GlobalWorldDB
				db.GlobalWorldDB = nil
				t.Cleanup(func() { db.GlobalWorldDB = previous })
				battleDispatch(t, wh, ses, opcodes.FieldMoveUseRequest, fmt.Sprintf(`{"moveName":%q,"requestId":"state-test","characterId":42,"commandRevision":0,"mapId":50,"fromX":0,"fromY":0}`, move.name))
				var reply FieldMoveUseResponsePayload
				if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil {
					t.Fatal("missing state-field reply")
				}
				want := kind == "durable-permission" || kind == "cached-battle" || kind == "single-connection"
				if reply.Success != want {
					t.Fatalf("kind=%s response=%+v", kind, reply)
				}
				var count int
				if err := database.QueryRow(`SELECT count(*) FROM character_field_move_state WHERE character_id=42 AND map_id=50 AND move_name=$1 AND active=1`, move.name).Scan(&count); err != nil || count != map[bool]int{false: 0, true: 1}[want] {
					t.Fatalf("state count=%d err=%v", count, err)
				}
				if want && reply.MapID != 50 {
					t.Fatalf("cached map published: %+v", reply)
				}
			})
		}
	}
}

func TestStateFieldReceiptReplaysWithoutReactivationAndRollsBackLateFailure(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50 WHERE id=42; INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(50,'ROOM',10,10,0); INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES(70,'STRENGTH','STRENGTH','STRENGTH','NO_ADDITIONAL_EFFECT',0,'NORMAL',0,15);UPDATE character_pokemon SET move1_id=70,move1_pp=15 WHERE character_id=42;INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_RAINBOWBADGE');CREATE FUNCTION reject_state_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late state receipt failure';END $$;CREATE CONSTRAINT TRIGGER reject_state_receipt AFTER UPDATE ON character_field_command_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_state_receipt()`)
	request := `{"moveName":"STRENGTH","requestId":"state-first","characterId":42,"commandRevision":0,"mapId":50,"fromX":0,"fromY":0}`
	battleDispatch(t, wh, ses, opcodes.FieldMoveUseRequest, request)
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_field_move_state`).Scan(&count); err != nil || count != 0 {
		t.Fatal("late receipt failure kept state")
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_state_receipt ON character_field_command_state`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.FieldMoveUseRequest, request)
	testdb.Exec(t, database, `UPDATE character_field_move_state SET active=0;DELETE FROM character_event_flags WHERE character_id=42`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.FieldMoveUseRequest, request)
	var response StateFieldResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &response) != nil || !response.Success {
		t.Fatal("missing historical state acknowledgement")
	}
	var active, revision int
	if err := database.QueryRow(`SELECT (SELECT active FROM character_field_move_state WHERE character_id=42 AND move_name='STRENGTH'),(SELECT revision FROM character_field_command_state WHERE character_id=42 AND domain='field-state')`).Scan(&active, &revision); err != nil || active != 0 || revision != 1 {
		t.Fatalf("replay mutated state/revision: %d %d %v", active, revision, err)
	}
}
