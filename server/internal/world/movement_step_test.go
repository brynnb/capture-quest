package world

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func stepEffectFixture(t *testing.T, forced bool) (*WorldHandler, *session.Session, *recordingMessenger, func() string) {
	t.Helper()
	wh, ses, messages := setupIssuedStep(t)
	if forced {
		next := wh.sessionManager.CreateNextSession(messages, "", nil)
		next.Client = ses.Client
		next.Authenticated = true
		ses = next
		wh.PlayerMovement.RegisterPlayer(ses, 42, 7, 8, 50, "RIGHT")
		wh.PlayerMovement.players[42].Path = []PathNode{{X: 8, Y: 8}}
	}
	attempt := func() string {
		if forced {
			state := wh.PlayerMovement.players[42]
			state.LastMoveTime = time.Time{}
			if err := ses.ExecuteCommand(context.Background(), func() { wh.PlayerMovement.processCharacterTick(ses.CommandContext(), 42, state) }); err != nil {
				t.Fatal(err)
			}
			return ""
		}
		step := issueStep(t, wh, ses, messages)
		payload := fmt.Sprintf(`{"stepToken":%q,"requestId":"effect-complete"}`, step.StepToken)
		battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, payload)
		return payload
	}
	return wh, ses, messages, attempt
}

func seedStepDaycare(t *testing.T, wh *WorldHandler) int64 {
	t.Helper()
	var rowID int64
	if err := wh.database.QueryRow(`INSERT INTO character_pokemon(character_id,box,box_slot,pokemon_id,level,exp,cur_hp,max_hp) VALUES(42,-2,0,129,5,125,10,10) RETURNING id`).Scan(&rowID); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, wh.database, `INSERT INTO character_daycare(character_id,pokemon_row_id,start_level) VALUES(42,$1,5)`, rowID)
	return rowID
}

func assertStepDaycare(t *testing.T, wh *WorldHandler, rowID int64, exp int) {
	t.Helper()
	var gotExp int
	var referenced int64
	if err := wh.database.QueryRow(`SELECT cp.exp,cd.pokemon_row_id FROM character_pokemon cp JOIN character_daycare cd ON cd.pokemon_row_id=cp.id WHERE cp.id=$1`, rowID).Scan(&gotExp, &referenced); err != nil || gotExp != exp || referenced != rowID {
		t.Fatalf("daycare row=%d exp=%d err=%v", referenced, gotExp, err)
	}
}

func assertNoStepSuccess(t *testing.T, messages *recordingMessenger) {
	t.Helper()
	for _, message := range messages.streams {
		if message.opcode == opcodes.PlayerStepResponse {
			continue
		} // private issuance, no movement
		if message.opcode == opcodes.PlayerStepCompleteResponse {
			var response protocol.PlayerStepError
			if err := json.Unmarshal(message.payload, &response); err == nil && !response.Success && response.Error != "" {
				continue
			}
		}
		t.Fatalf("failed step published opcode %d: %s", message.opcode, message.payload)
	}
}

func TestMovementStepDaycareLateFailureRetryAndDuplicate(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(fmt.Sprint(forced), func(t *testing.T) {
			wh, ses, messages, attempt := stepEffectFixture(t, forced)
			rowID := seedStepDaycare(t, wh)
			testdb.Exec(t, wh.database, `CREATE FUNCTION reject_step_exp() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late daycare step failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_step_exp AFTER UPDATE ON character_pokemon DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.box=-2) EXECUTE FUNCTION reject_step_exp();`)
			previous := db.GlobalWorldDB
			db.GlobalWorldDB = nil
			defer func() { db.GlobalWorldDB = previous }()
			attempt()
			assertStepPosition(t, wh, ses, 7)
			assertStepDaycare(t, wh, rowID, 125)
			assertNoStepSuccess(t, messages)
			if forced && len(wh.PlayerMovement.players[42].Path) != 1 {
				t.Fatal("rollback retired forced path")
			}
			testdb.Exec(t, wh.database, `DROP TRIGGER reject_step_exp ON character_pokemon`)
			messages.streams = nil
			payload := attempt()
			assertStepPosition(t, wh, ses, 8)
			assertStepDaycare(t, wh, rowID, 126)
			if forced {
				attempt()
			} else {
				battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, payload)
			}
			assertStepDaycare(t, wh, rowID, 126)
		})
	}
}

func seedStepEncounter(t *testing.T, wh *WorldHandler) {
	t.Helper()
	wh.WildEncounter = NewWildEncounterManager(wh, wh.database)
	wh.WildEncounter.areas[1] = &encounterAreaData{ID: 1, Name: "TEST", EncounterRate: 255, Slots: []encounterSlot{{PokemonID: 129, Level: 5, Probability: 100}}}
	wh.WildEncounter.tileCache[[3]int{50, 8, 8}] = 1
	wh.WildEncounter.cacheLoaded = true
	wh.WildEncounter.encounterRoll = func(int) int { return 0 } // deterministic successful roll, unchanged production RNG
	if err := wh.WildEncounter.SetRepelSteps(context.Background(), 42, 1); err != nil {
		t.Fatal(err)
	}
}

func TestMovementStepBattleRepelAndDaycareRollbackTogether(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(fmt.Sprint(forced), func(t *testing.T) {
			wh, ses, messages, attempt := stepEffectFixture(t, forced)
			rowID := seedStepDaycare(t, wh)
			seedStepEncounter(t, wh)
			testdb.Exec(t, wh.database, `CREATE FUNCTION reject_step_battle() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late battle step failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_step_battle AFTER INSERT ON character_battle_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_step_battle();`)
			attempt()
			assertStepPosition(t, wh, ses, 7)
			assertStepDaycare(t, wh, rowID, 125)
			assertNoStepSuccess(t, messages)
			status, err := wh.WildEncounter.RepelStatus(context.Background(), 42)
			if err != nil || status.StepsLeft != 1 || getBattle(42) != nil {
				t.Fatalf("failed effects repel=%+v battle=%+v %v", status, getBattle(42), err)
			}
			var seen int
			if err := wh.database.QueryRow(`SELECT COUNT(*) FROM character_pokedex WHERE character_id=42 AND pokemon_id=129`).Scan(&seen); err != nil || seen != 0 {
				t.Fatalf("failed step marked seen: %d %v", seen, err)
			}
			testdb.Exec(t, wh.database, `DROP TRIGGER reject_step_battle ON character_battle_state`)
			messages.streams = nil
			attempt()
			assertStepPosition(t, wh, ses, 8)
			assertStepDaycare(t, wh, rowID, 126)
			status, err = wh.WildEncounter.RepelStatus(context.Background(), 42)
			if err != nil || status.Active || getBattle(42) == nil {
				t.Fatal("committed step did not publish durable battle/repel expiry")
			}
			stored, err := pokebattle.LoadBattleState(wh.database, 42)
			if err != nil || stored == nil || stored.BattleID != getBattle(42).BattleID {
				t.Fatal("published battle disagrees with durable result")
			}
		})
	}
}

func TestMovementStepSafariExpiryAndDaycareRollbackTogether(t *testing.T) {
	for _, forced := range []bool{false, true} {
		t.Run(fmt.Sprint(forced), func(t *testing.T) {
			wh, ses, messages, attempt := stepEffectFixture(t, forced)
			rowID := seedStepDaycare(t, wh)
			// Keep the issued-step fixture identity while making this catalog map Safari.
			testdb.Exec(t, wh.database, `UPDATE phaser_maps SET id=220 WHERE id=50; UPDATE phaser_tiles SET map_id=220 WHERE map_id=50; UPDATE character_data SET map_id=220 WHERE id=42;`)
			wh.ActorManager = NewPhaserActorManager(wh)
			wh.PlayerMovement.actorManager = wh.ActorManager
			wh.PlayerMovement.UpdatePosition(42, 7, 8, 220, "RIGHT")
			ses.Client.CharData().MapID = 220
			ses.MapID = 220
			if forced {
				wh.PlayerMovement.players[42].Path = []PathNode{{X: 8, Y: 8}}
			} else {
				attempt = func() string {
					battleDispatch(t, wh, ses, opcodes.PlayerStepRequest, `{"mapId":220,"fromX":7,"fromY":8,"direction":"RIGHT","requestId":"safari-step"}`)
					var response protocol.PlayerStepResponse
					if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &response); err != nil || !response.Success {
						t.Fatalf("Safari issuance %+v %v", response, err)
					}
					payload := fmt.Sprintf(`{"stepToken":%q,"requestId":"safari-complete"}`, response.StepToken)
					battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, payload)
					return payload
				}
			}
			wh.Safari = NewSafariZoneManager(wh.database)
			if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 1}); err != nil {
				t.Fatal(err)
			}
			testdb.Exec(t, wh.database, `CREATE FUNCTION reject_combined_expiry() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late combined expiry'; END $$;
 CREATE CONSTRAINT TRIGGER reject_combined_expiry AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.map_id=156) EXECUTE FUNCTION reject_combined_expiry();`)
			attempt()
			assertStepDaycare(t, wh, rowID, 125)
			assertNoStepSuccess(t, messages)
			x, y, mapID, _ := wh.PlayerMovement.GetPosition(42)
			if x != 7 || y != 8 || mapID != 220 {
				t.Fatal("expiry failure moved live player")
			}
			visit, err := wh.Safari.GetSession(context.Background(), 42)
			if err != nil || visit == nil || !visit.Active || visit.StepsLeft != 1 {
				t.Fatalf("expiry failure visit=%+v %v", visit, err)
			}
			testdb.Exec(t, wh.database, `DROP TRIGGER reject_combined_expiry ON character_data`)
			messages.streams = nil
			attempt()
			assertStepDaycare(t, wh, rowID, 126)
			visit, err = wh.Safari.GetSession(context.Background(), 42)
			if err != nil || visit.Active || visit.StepsLeft != 0 {
				t.Fatalf("committed expiry visit=%+v %v", visit, err)
			}
			if err := wh.database.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=42`).Scan(&x, &y, &mapID); err != nil || x != SafariZoneGateReturnX || y != SafariZoneGateReturnY || mapID != SafariZoneGateMapID {
				t.Fatalf("expiry destination=%d %d %d %v", x, y, mapID, err)
			}
			x, y, mapID, _ = wh.PlayerMovement.GetPosition(42)
			if x != SafariZoneGateReturnX || y != SafariZoneGateReturnY || mapID != SafariZoneGateMapID {
				t.Fatal("committed expiry live position diverged")
			}
			if forced && wh.PlayerMovement.players[42].PreviousMapID != 220 {
				t.Fatal("forced expiry lost departure map provenance")
			}
		})
	}
}

func TestMovementStepTrainerAndCutscenePlansPublishOnlyAfterCommit(t *testing.T) {
	for _, kind := range []string{"trainer", "cutscene"} {
		t.Run(kind, func(t *testing.T) {
			wh, ses, messages, attempt := stepEffectFixture(t, false)
			wh.EventFlags = NewEventFlagManager(wh.database)
			if kind == "trainer" {
				wh.TrainerEncounter = NewTrainerEncounterManager(wh)
				trainer := &trainerSightData{ObjectID: 77, MapID: 50, X: 8, Y: 7, Direction: "DOWN", SightRange: 1, RuntimeActorID: 707, Name: "TEST"}
				wh.TrainerEncounter.byMap[50] = []*trainerSightData{trainer}
			} else {
				required := "STEP_ALLOWED"
				testdb.Exec(t, wh.database, `INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,$1)`, required)
				// Cached flags deliberately disagree with storage: transaction reads win.
				wh.CoordTriggers = NewCoordinateTriggerManager(wh.database)
				wh.CoordTriggers.byMap[50] = map[string][]CoordinateTrigger{tileKey(8, 8): {{MapID: 50, MapName: "ROOM", Label: "TEST_STEP", X: 8, Y: 8}}}
				wh.Cutscenes = NewCutsceneManager(wh.database)
				wh.Cutscenes.byLabel["TEST_STEP"] = &CutsceneScript{ScriptLabel: "TEST_STEP", MapName: "ROOM", TriggerType: "coord", RequiresFlag: &required, Actions: json.RawMessage(`[]`)}
			}
			testdb.Exec(t, wh.database, `CREATE FUNCTION reject_step_plan() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late plan failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_step_plan AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.x=8) EXECUTE FUNCTION reject_step_plan();`)
			attempt()
			assertStepPosition(t, wh, ses, 7)
			assertNoStepSuccess(t, messages)
			if kind == "trainer" && (len(wh.TrainerEncounter.pending) != 0 || len(wh.TrainerEncounter.spottedBy) != 0) {
				t.Fatal("rollback published trainer cache")
			}
			testdb.Exec(t, wh.database, `DROP TRIGGER reject_step_plan ON character_data`)
			messages.streams = nil
			payload := attempt()
			assertStepPosition(t, wh, ses, 8)
			expected := opcodes.CutsceneStartNotify
			if kind == "trainer" {
				expected = opcodes.TrainerEncounterNotify
			}
			count := 0
			for _, message := range messages.streams {
				if message.opcode == expected {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("commit published %d plans of kind %s", count, kind)
			}
			if kind == "cutscene" && !wh.EventFlags.CheckFlag(42, "STEP_ALLOWED") {
				t.Fatal("publication retained stale flag cache")
			}
			battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, payload)
			again := 0
			for _, message := range messages.streams {
				if message.opcode == expected {
					again++
				}
			}
			if again != count {
				t.Fatal("duplicate reissued committed plan")
			}
		})
	}
}

func TestMovementStepBlackoutRollsBackRecoveryWithSourceStep(t *testing.T) {
	wh, ses, messages, attempt := stepEffectFixture(t, false)
	seedStepEncounter(t, wh)
	testdb.Exec(t, wh.database, `UPDATE character_pokemon SET cur_hp=0 WHERE character_id=42;
 CREATE FUNCTION reject_step_heal() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late blackout heal failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_step_heal AFTER UPDATE ON character_pokemon DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.cur_hp>0) EXECUTE FUNCTION reject_step_heal();`)
	attempt()
	assertStepPosition(t, wh, ses, 7)
	assertNoStepSuccess(t, messages)
	var money, hp int
	if err := wh.database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 100 {
		t.Fatalf("failed blackout money=%d %v", money, err)
	}
	if err := wh.database.QueryRow(`SELECT cur_hp FROM character_pokemon WHERE character_id=42`).Scan(&hp); err != nil || hp != 0 {
		t.Fatalf("failed blackout hp=%d %v", hp, err)
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_step_heal ON character_pokemon`)
	messages.streams = nil
	attempt()
	if err := wh.database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 50 {
		t.Fatalf("committed blackout money=%d %v", money, err)
	}
	if err := wh.database.QueryRow(`SELECT cur_hp FROM character_pokemon WHERE character_id=42`).Scan(&hp); err != nil || hp <= 0 {
		t.Fatalf("committed blackout hp=%d %v", hp, err)
	}
	if getBattle(42) != nil {
		t.Fatal("blackout installed battle")
	}
	got := false
	for _, message := range messages.streams {
		if message.opcode == opcodes.PokeBattleEndNotify {
			got = true
		}
	}
	if !got {
		t.Fatal("committed blackout not published")
	}
}

func TestMovementStepDisconnectCancelsBlockedEffectAfterPrivatePositionSave(t *testing.T) {
	wh, ses, messages, _ := stepEffectFixture(t, false)
	rowID := seedStepDaycare(t, wh)
	step := issueStep(t, wh, ses, messages)
	lock, err := wh.database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`UPDATE character_pokemon SET id=id WHERE id=$1`, rowID); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		registry := NewWorldOpCodeRegistry()
		registry.WH = wh
		registry.HandleWorldPacket(ses, clientPacket(opcodes.PlayerStepCompleteRequest, fmt.Sprintf(`{"stepToken":%q,"requestId":"blocked-effect"}`, step.StepToken)))
	}()
	deadline := time.Now().Add(time.Second)
	for {
		var waiting int
		if err := wh.database.QueryRow(`SELECT COUNT(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%UPDATE character_pokemon%'`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting > 0 {
			break
		}
		if time.Now().After(deadline) {
			ses.Close()
			<-done
			t.Fatal("step never reached blocked effect")
		}
		time.Sleep(10 * time.Millisecond)
	}
	ses.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("disconnect did not cancel blocked effect")
	}
	// The daycare row is still locked; source rollback must already be visible.
	assertStepPosition(t, wh, ses, 7)
	assertStepDaycare(t, wh, rowID, 125)
	cleaned := false
	ses.DrainCommands(func() { cleaned = true })
	if !cleaned {
		t.Fatal("cancelled effect kept owner cleanup blocked")
	}
}

func TestMovementTransactionContextReadsKeepOuterDeadline(t *testing.T) {
	wh, _, _, _ := stepEffectFixture(t, false)
	for _, query := range []string{"row", "rows", "exec"} {
		t.Run(query, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			started := time.Now()
			err := db.Transaction(ctx, wh.database, func(tx db.DBTX) error {
				q := tx.(db.ContextDBTX)
				switch query {
				case "row":
					var result string
					return q.QueryRowContext(context.Background(), `SELECT pg_sleep(5)`).Scan(&result)
				case "rows":
					rows, err := q.QueryContext(context.Background(), `SELECT pg_sleep(5)`)
					if rows != nil {
						rows.Close()
					}
					return err
				default:
					_, err := q.ExecContext(context.Background(), `SELECT pg_sleep(5)`)
					return err
				}
			})
			if err == nil || ctx.Err() == nil || time.Since(started) > time.Second {
				t.Fatalf("context adapter exceeded deadline: %v ctx=%v elapsed=%s", err, ctx.Err(), time.Since(started))
			}
		})
	}
}

func TestSurfEntryPositionRepelAndEncounterCommitTogether(t *testing.T) {
	wh, ses, messages, _ := stepEffectFixture(t, false)
	seedStepEncounter(t, wh)
	testdb.Exec(t, wh.database, `INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES(57,'SURF','SURF','SURF','NO_ADDITIONAL_EFFECT',95,'WATER',255,15);
 UPDATE character_pokemon SET move1_id=57,move1_pp=15 WHERE character_id=42;
 INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EVENT_GOT_SOULBADGE');
 UPDATE phaser_tiles SET collision_type=3 WHERE map_id=50 AND x=8 AND y=8;
 CREATE FUNCTION reject_surf_battle() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late surf encounter failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_surf_battle AFTER INSERT ON character_battle_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_surf_battle();`)
	wh.ActorManager.collisionMap[50][tileKey(8, 8)] = collisionWater
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, `{"mapId":50,"targetX":8,"targetY":8,"direction":"RIGHT"}`)
	assertStepPosition(t, wh, ses, 7)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PokeSurfingResponse {
		t.Fatalf("failed Surf packets=%+v", messages.streams)
	}
	var response struct {
		Success  bool
		Blackout bool
	}
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success {
		t.Fatal("failed Surf published success")
	}
	status, err := wh.WildEncounter.RepelStatus(context.Background(), 42)
	if err != nil || status.StepsLeft != 1 || getBattle(42) != nil {
		t.Fatal("failed Surf changed encounter/repel")
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_surf_battle ON character_battle_state`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.PokeSurfingRequest, `{"mapId":50,"targetX":8,"targetY":8,"direction":"RIGHT"}`)
	assertStepPosition(t, wh, ses, 8)
	if getBattle(42) == nil {
		t.Fatal("Surf did not publish committed encounter")
	}
}

func TestMovementStepBlackoutReturningToSourceConsumesToken(t *testing.T) {
	wh, ses, messages, attempt := stepEffectFixture(t, false)
	seedStepEncounter(t, wh)
	testdb.Exec(t, wh.database, `UPDATE character_pokemon SET cur_hp=0 WHERE character_id=42;
 UPDATE character_data SET options='{"lastPokeCenterMapId":50,"lastPokeCenterX":7,"lastPokeCenterY":8}' WHERE id=42;`)
	payload := attempt()
	assertStepPosition(t, wh, ses, 7)
	if wh.PlayerMovement.players[42].pendingStep != nil {
		t.Fatal("recovery to source retained issued token")
	}
	before := len(messages.streams)
	battleDispatch(t, wh, ses, opcodes.PlayerStepCompleteRequest, payload)
	for _, message := range messages.streams[before:] {
		if message.opcode != opcodes.PlayerStepCompleteResponse {
			t.Fatalf("duplicate recovery published opcode %d", message.opcode)
		}
		var response protocol.PlayerStepError
		if err := json.Unmarshal(message.payload, &response); err != nil || response.Success || response.Error == "" {
			t.Fatalf("duplicate recovery result %+v %v", response, err)
		}
	}
}
