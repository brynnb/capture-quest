package world

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func pendingTrainerFixture(t *testing.T) (*WorldHandler, *session.Session, *recordingMessenger, protocol.TrainerEncounterNotifyPayload) {
	t.Helper()
	wh, ses, messages, attempt := stepEffectFixture(t, false)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	trainer := &trainerSightData{ObjectID: 77, MapID: 50, X: 8, Y: 7, Direction: "DOWN", SightRange: 1, RuntimeActorID: 707, Name: "TEST", TrainerClass: "TEST", PartyIndex: 0}
	wh.TrainerEncounter.byMap[50] = []*trainerSightData{trainer}
	testdb.Exec(t, wh.database, `INSERT INTO phaser_trainer_classes(id,constant_name,display_name) VALUES(1,'TEST','Test Trainer');
 INSERT INTO phaser_trainer_parties(id,trainer_class_id,party_index) VALUES(1,1,0);
 INSERT INTO phaser_trainer_party_pokemon(trainer_party_id,slot_index,pokemon_name,level) VALUES(1,0,'MAGIKARP',5);`)
	attempt()
	var notify protocol.TrainerEncounterNotifyPayload
	for _, message := range messages.streams {
		if message.opcode == opcodes.TrainerEncounterNotify {
			if err := json.Unmarshal(message.payload, &notify); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !validMovementToken(notify.EncounterToken) {
		t.Fatal("no committed trainer token")
	}
	messages.streams = nil
	return wh, ses, messages, notify
}

func assertTrainerResolution(t *testing.T, wh *WorldHandler, token, want string) {
	t.Helper()
	var got, gotToken string
	if err := wh.database.QueryRow(`SELECT resolution,encounter_token FROM character_trainer_encounters WHERE character_id=42`).Scan(&got, &gotToken); err != nil || got != want || gotToken != token {
		t.Fatalf("trainer resolution=%s token=%s %v", got, gotToken, err)
	}
}

func trainerReady(t *testing.T, wh *WorldHandler, ses *session.Session, notify protocol.TrainerEncounterNotifyPayload) {
	t.Helper()
	battleDispatch(t, wh, ses, opcodes.TrainerEncounterReady, fmt.Sprintf(`{"trainerActorId":%d,"encounterToken":%q}`, notify.TrainerActorID, notify.EncounterToken))
}

func TestPendingTrainerRejectsForgeryAndRollsBackReadyWithBattle(t *testing.T) {
	wh, ses, messages, notify := pendingTrainerFixture(t)
	wrong := notify
	wrong.TrainerActorID++
	trainerReady(t, wh, ses, wrong)
	assertTrainerResolution(t, wh, notify.EncounterToken, "pending")
	wrong = notify
	wrong.EncounterToken = strings.Repeat("a", 32)
	trainerReady(t, wh, ses, wrong)
	assertTrainerResolution(t, wh, notify.EncounterToken, "pending")
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_trainer_ready() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late trainer ready'; END $$;
 CREATE CONSTRAINT TRIGGER reject_trainer_ready AFTER UPDATE ON character_trainer_encounters DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.resolution='battle') EXECUTE FUNCTION reject_trainer_ready();`)
	trainerReady(t, wh, ses, notify)
	assertTrainerResolution(t, wh, notify.EncounterToken, "pending")
	var battles, seen int
	if err := wh.database.QueryRow(`SELECT (SELECT COUNT(*) FROM character_battle_state),(SELECT COUNT(*) FROM character_pokedex)`).Scan(&battles, &seen); err != nil || battles != 0 || seen != 0 || getBattle(42) != nil {
		t.Fatalf("failed ready battle=%d seen=%d %v", battles, seen, err)
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_trainer_ready ON character_trainer_encounters`)
	messages.streams = nil
	trainerReady(t, wh, ses, notify)
	assertTrainerResolution(t, wh, notify.EncounterToken, "battle")
	first := getBattle(42)
	if first == nil || first.Trainer == nil || first.Trainer.TrainerObjectID != 77 {
		t.Fatal("no committed trainer battle")
	}
	trainerReady(t, wh, ses, notify)
	if getBattle(42).BattleID != first.BattleID {
		t.Fatal("duplicate ready recreated battle")
	}
}

func TestPendingTrainerSurvivesFreshOwnerAndBlocksWalking(t *testing.T) {
	wh, old, messages, notify := pendingTrainerFixture(t)
	battleDispatch(t, wh, old, opcodes.PlayerStepRequest, `{"mapId":50,"fromX":8,"fromY":8,"direction":"LEFT","requestId":"blocked-intent"}`)
	var step protocol.PlayerStepError
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &step); err != nil || step.Success || step.Error == "" {
		t.Fatalf("pending trainer accepted ordinary intent %+v %v", step, err)
	}
	assertStepPosition(t, wh, old, 8)
	old.Close()
	wh.TrainerEncounter.ClearPlayer(42)
	trainer := *wh.TrainerEncounter.byMap[50][0]
	trainer.RuntimeActorID = 808 // Startup remapping must never reuse a saved runtime actor ID.
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	wh.TrainerEncounter.byMap[50] = []*trainerSightData{&trainer}
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	next := wh.sessionManager.CreateNextSession(messages, "", nil)
	next.Authenticated, next.Client = true, old.Client
	wh.PlayerMovement.RegisterPlayer(next, 42, 8, 8, 50, "RIGHT")
	publishCommittedPlayerLocation(next, wh, 50, 8, 8)
	messages.streams = nil
	battleDispatch(t, wh, next, opcodes.OwnedPlayerPositionRequest, `{"requestId":"resume"}`)
	var resumed protocol.TrainerEncounterNotifyPayload
	for _, message := range messages.streams {
		if message.opcode == opcodes.TrainerEncounterNotify {
			if err := json.Unmarshal(message.payload, &resumed); err != nil {
				t.Fatal(err)
			}
		}
	}
	if resumed.EncounterToken != notify.EncounterToken || resumed.TrainerActorID != 808 {
		t.Fatalf("fresh owner plan=%+v", resumed)
	}
	trainerReady(t, wh, next, resumed)
	assertTrainerResolution(t, wh, notify.EncounterToken, "battle")
}

func TestPendingTrainerBlackoutAndTeleportCancellationAreAtomic(t *testing.T) {
	wh, ses, _, notify := pendingTrainerFixture(t)
	testdb.Exec(t, wh.database, `UPDATE character_pokemon SET cur_hp=0 WHERE character_id=42;
 CREATE FUNCTION reject_trainer_blackout() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late trainer blackout'; END $$;
 CREATE CONSTRAINT TRIGGER reject_trainer_blackout AFTER UPDATE ON character_trainer_encounters DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.resolution='blackout') EXECUTE FUNCTION reject_trainer_blackout();`)
	trainerReady(t, wh, ses, notify)
	assertTrainerResolution(t, wh, notify.EncounterToken, "pending")
	assertStepPosition(t, wh, ses, 8)
	var money, hp int
	if err := wh.database.QueryRow(`SELECT (SELECT pokedollars FROM character_wallet WHERE character_id=42),(SELECT cur_hp FROM character_pokemon WHERE character_id=42)`).Scan(&money, &hp); err != nil || money != 100 || hp != 0 {
		t.Fatalf("failed recovery money=%d hp=%d %v", money, hp, err)
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_trainer_blackout ON character_trainer_encounters`)
	trainerReady(t, wh, ses, notify)
	assertTrainerResolution(t, wh, notify.EncounterToken, "blackout")
	trainerReady(t, wh, ses, notify)
	if err := wh.database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 50 {
		t.Fatalf("duplicate blackout money=%d %v", money, err)
	}
}

func TestPendingTrainerCancellationAndCatalogMismatch(t *testing.T) {
	wh, ses, _, notify := pendingTrainerFixture(t)
	wh.TrainerEncounter.byMap[50][0].PartyIndex++
	if _, err := wh.TrainerEncounter.resumePendingEncounter(ses, wh); err == nil {
		t.Fatal("accepted changed trainer identity")
	}
	wh.TrainerEncounter.byMap[50][0].PartyIndex--
	if _, err := setServerTeleportedPlayerPosition(ses, wh, 50, 7, 8, "LEFT"); err != nil {
		t.Fatal(err)
	}
	assertTrainerResolution(t, wh, notify.EncounterToken, "cancelled")
	trainerReady(t, wh, ses, notify)
	if getBattle(42) != nil {
		t.Fatal("cancelled encounter started battle")
	}
	if err := wh.TrainerEncounter.requireEncounterSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, wh.database, `DROP TABLE character_trainer_encounters`)
	if err := wh.TrainerEncounter.requireEncounterSchema(context.Background()); err == nil {
		t.Fatal("missing schema accepted")
	}
}
