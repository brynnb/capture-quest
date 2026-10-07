package world

import (
	"context"
	"encoding/json"
	"testing"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestCenterHealingDurableCompletionAtomicityCancellationAndReplay(t *testing.T) {
	database, _, ses, messages := battleTestWorld(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(41,'CENTER',10,10);
 UPDATE character_data SET map_id=41,x=3,y=4,options='{"custom":"keep"}' WHERE id=42;
 INSERT INTO character_defeated_trainers(character_id,trainer_object_id) VALUES(42,77);`)
	testdb.Exec(t, database, `UPDATE character_pokemon SET move1_id=150,move1_pp=0,status=8 WHERE character_id=42`)
	script := CutsceneScript{ScriptLabel: "Nurse", MapName: "CENTER", Actions: json.RawMessage(`[{"type":"healParty","healingPolicy":"center"}]`)}
	issue := func() *cutsceneCompletion {
		t.Helper()
		var plan *durableCutscene
		err := db.Transaction(context.Background(), database, func(tx db.DBTX) (err error) {
			if err := db.LockCharacter(tx, 42); err != nil {
				return err
			}
			plan, err = issueCutsceneIn(tx, 42, issuedCutscene{Script: script, MapID: 41, X: 3, Y: 4})
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return &cutsceneCompletion{Token: plan.Token, Label: script.ScriptLabel}
	}
	apply := func(completion *cutsceneCompletion) error {
		_, _, err := runCutsceneMutation(context.Background(), CutsceneActionContext{Database: database, issuedCompletion: completion}, "", nil, 42, nil)
		return err
	}
	assertState := func(hp, trainers, center int) {
		t.Helper()
		var gotHP, gotTrainers, gotCenter int
		var custom string
		if err := database.QueryRow(`SELECT cur_hp FROM character_pokemon WHERE character_id=42`).Scan(&gotHP); err != nil {
			t.Fatal(err)
		}
		if err := database.QueryRow(`SELECT count(*) FROM character_defeated_trainers WHERE character_id=42`).Scan(&gotTrainers); err != nil {
			t.Fatal(err)
		}
		if err := database.QueryRow(`SELECT COALESCE((options->>'lastPokeCenterMapId')::integer,0),options->>'custom' FROM character_data WHERE id=42`).Scan(&gotCenter, &custom); err != nil {
			t.Fatal(err)
		}
		if gotHP != hp || gotTrainers != trainers || gotCenter != center || custom != "keep" {
			t.Fatalf("hp=%d trainers=%d center=%d custom=%s", gotHP, gotTrainers, gotCenter, custom)
		}
	}
	// Stale opcode cannot mutate or claim successful healing.
	HandlePokeCenterHeal(ses, []byte(`{"mapId":41}`), nil)
	var legacy struct{ Success bool }
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &legacy) != nil || legacy.Success {
		t.Fatal("missing legacy rejection")
	}
	assertState(1, 1, 0)
	cancelled := issue()
	cancelled.Cancel = true
	if err := apply(cancelled); err != nil {
		t.Fatal(err)
	}
	assertState(1, 1, 0)
	cancelled.Cancel = false
	if err := apply(cancelled); err == nil {
		t.Fatal("cancelled token healed party")
	}
	completion := issue()
	testdb.Exec(t, database, `CREATE FUNCTION reject_center_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'center commit rejected'; END $$;
 CREATE CONSTRAINT TRIGGER reject_center_commit AFTER UPDATE ON character_cutscene_plans DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.resolution='resolved') EXECUTE FUNCTION reject_center_commit();`)
	if err := apply(completion); err == nil {
		t.Fatal("accepted failed commit")
	}
	assertState(1, 1, 0)
	testdb.Exec(t, database, `DROP TRIGGER reject_center_commit ON character_cutscene_plans`)
	if err := apply(completion); err != nil {
		t.Fatal(err)
	}
	assertState(95, 0, 41)
	var pp, status int
	if err := database.QueryRow(`SELECT move1_pp,status FROM character_pokemon WHERE character_id=42`).Scan(&pp, &status); err != nil || pp != 40 || status != 0 {
		t.Fatalf("pp=%d status=%d err=%v", pp, status, err)
	}
	// A lost reply/retry must not heal new damage or reset subsequent wins.
	testdb.Exec(t, database, `UPDATE character_pokemon SET cur_hp=2 WHERE character_id=42; INSERT INTO character_defeated_trainers(character_id,trainer_object_id) VALUES(42,78)`)
	if err := apply(completion); err != nil || !completion.Replayed {
		t.Fatalf("replay=%t err=%v", completion.Replayed, err)
	}
	assertState(2, 1, 41)
}

func TestCenterHealingRejectsBattleChangedSourceAndMalformedOptions(t *testing.T) {
	for _, scenario := range []string{"battle", "source", "options", "policy"} {
		t.Run(scenario, func(t *testing.T) {
			database, _, _, _ := battleTestWorld(t)
			testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(41,'CENTER',10,10); UPDATE character_data SET map_id=41 WHERE id=42`)
			raw := json.RawMessage(`[{"type":"healParty","healingPolicy":"center"}]`)
			switch scenario {
			case "battle":
				battleTestStart(t, database, false, nil)
			case "source":
				testdb.Exec(t, database, `UPDATE character_data SET map_id=42 WHERE id=42`)
			case "options":
				testdb.Exec(t, database, `UPDATE character_data SET options='[]' WHERE id=42`)
			case "policy":
				raw = json.RawMessage(`[{"type":"healParty","healingPolicy":"unknown"}]`)
			}
			if _, _, err := ApplyCutsceneActionList(context.Background(), CutsceneActionContext{Database: database}, "CENTER", raw, 42); err == nil {
				t.Fatal("accepted invalid center healing")
			}
			var hp int
			if err := database.QueryRow(`SELECT cur_hp FROM character_pokemon WHERE character_id=42`).Scan(&hp); err != nil || hp != 1 {
				t.Fatalf("hp=%d err=%v", hp, err)
			}
		})
	}
}
