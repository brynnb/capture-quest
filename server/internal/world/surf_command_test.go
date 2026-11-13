package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

const surfCommand = `{"requestId":"surf-receipt","characterId":42,"commandRevision":0,"fromX":7,"fromY":8,"mapId":50,"targetX":8,"targetY":8,"direction":"RIGHT"}`

func surfCommandFixture(t *testing.T) (*WorldHandler, *session.Session, *recordingMessenger) {
	t.Helper()
	wh, ses, messages, _ := stepEffectFixture(t, false)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES(57,'SURF','SURF','SURF','NO_ADDITIONAL_EFFECT',95,'WATER',255,15);
 UPDATE character_pokemon SET move1_id=57,move1_pp=15 WHERE character_id=42;
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_SOULBADGE');
 UPDATE phaser_tiles SET collision_type=3 WHERE map_id=50 AND x=8 AND y=8;`)
	wh.ActorManager.collisionMap[50][tileKey(8, 8)] = collisionWater
	return wh, ses, messages
}

func TestSurfReceiptReplayDoesNotReenterAfterSourceReturn(t *testing.T) {
	wh, ses, messages := surfCommandFixture(t)
	wh.database.SetMaxOpenConns(1)
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, surfCommand)
	assertStepPosition(t, wh, ses, 8)
	// A later owned move can return to the same source. Remove permission too:
	// replay must acknowledge the old result without executing any gameplay.
	testdb.Exec(t, wh.database, `UPDATE character_data SET x=7 WHERE id=42; DELETE FROM character_event_flags WHERE character_id=42; UPDATE character_pokemon SET move1_id=0 WHERE character_id=42`)
	wh.PlayerMovement.projectCommittedPosition(42, 7, 8, 50, "RIGHT", false)
	publishCommittedPlayerLocation(ses, wh, 50, 7, 8)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, surfCommand)
	assertStepPosition(t, wh, ses, 7)
	var reply SurfResponse
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || !reply.Success {
		t.Fatalf("missing historical acknowledgement: %+v", messages.streams)
	}
	var revision int
	if err := wh.database.QueryRow(`SELECT revision FROM character_field_command_state WHERE character_id=42 AND domain='surf'`).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("replay advanced revision: %d %v", revision, err)
	}
	testdb.Exec(t, wh.database, `UPDATE character_field_command_state SET result_json='{"success":true,"requestId":"surf-receipt","characterId":42}' WHERE character_id=42 AND domain='surf'`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, surfCommand)
	assertStepPosition(t, wh, ses, 7)
	var denied SurfError
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &denied) != nil || denied.Success || denied.Error == "" {
		t.Fatal("incomplete committed outcome accepted")
	}
}

func TestSurfReceiptLateFailureRollsBackGameplayAndPublication(t *testing.T) {
	wh, ses, messages := surfCommandFixture(t)
	seedStepEncounter(t, wh)
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_surf_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late SURF receipt failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_surf_receipt AFTER UPDATE ON character_field_command_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_surf_receipt()`)
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, surfCommand)
	assertStepPosition(t, wh, ses, 7)
	var reply SurfError
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success || reply.Error == "" {
		t.Fatal("failed receipt published success/effects")
	}
	var count int
	if err := wh.database.QueryRow(`SELECT count(*) FROM character_field_command_state WHERE character_id=42 AND domain='surf'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed receipt survived: %d %v", count, err)
	}
	status, err := wh.WildEncounter.RepelStatus(context.Background(), 42)
	if err != nil || status.StepsLeft != 1 {
		t.Fatalf("receipt failure changed Repel: %+v %v", status, err)
	}
	if getBattle(42) != nil {
		t.Fatal("uncommitted battle was published")
	}
	var battles int
	if err := wh.database.QueryRow(`SELECT count(*) FROM character_battle_state WHERE character_id=42`).Scan(&battles); err != nil || battles != 0 {
		t.Fatalf("battle survived receipt rollback: %d %v", battles, err)
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_surf_receipt ON character_field_command_state`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, surfCommand)
	assertStepPosition(t, wh, ses, 8)
	if getBattle(42) == nil {
		t.Fatal("successful retry did not publish its committed battle")
	}
}

func TestSurfReceiptRejectsChangedInputAndSupersededRevision(t *testing.T) {
	wh, ses, messages := surfCommandFixture(t)
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, surfCommand)
	testdb.Exec(t, wh.database, `UPDATE character_data SET x=7 WHERE id=42`)
	wh.PlayerMovement.projectCommittedPosition(42, 7, 8, 50, "RIGHT", false)
	publishCommittedPlayerLocation(ses, wh, 50, 7, 8)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, strings.Replace(surfCommand, `"RIGHT"`, `"UP"`, 1))
	assertStepPosition(t, wh, ses, 7)
	var denied SurfError
	if json.Unmarshal(messages.streams[0].payload, &denied) != nil || denied.Success || denied.Error == "" {
		t.Fatal("changed replay input accepted")
	}
	next := strings.Replace(strings.Replace(surfCommand, `"surf-receipt"`, `"surf-next"`, 1), `"commandRevision":0`, `"commandRevision":1`, 1)
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, next)
	assertStepPosition(t, wh, ses, 8)
	testdb.Exec(t, wh.database, `UPDATE character_data SET x=7 WHERE id=42`)
	wh.PlayerMovement.projectCommittedPosition(42, 7, 8, 50, "RIGHT", false)
	publishCommittedPlayerLocation(ses, wh, 50, 7, 8)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, surfCommand)
	assertStepPosition(t, wh, ses, 7)
	if json.Unmarshal(messages.streams[0].payload, &denied) != nil || denied.Success || denied.Error == "" {
		t.Fatal("superseded revision accepted")
	}
	var rows, revision int
	if err := wh.database.QueryRow(`SELECT count(*),max(revision) FROM character_field_command_state WHERE character_id=42 AND domain='surf'`).Scan(&rows, &revision); err != nil || rows != 1 || revision != 2 {
		t.Fatalf("unbounded/wrong receipt rows=%d revision=%d err=%v", rows, revision, err)
	}
}
