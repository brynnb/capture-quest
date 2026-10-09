package world

import (
	"capturequest/internal/protocol"
	"context"
	"encoding/json"
	"sync"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
)

func safariTestWorld(t *testing.T) (*WorldHandler, *SafariZoneManager) {
	t.Helper()
	database, wh, _, _ := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	return wh, wh.Safari
}
func TestSafariConcurrentEntryChargesOnceAndRecovers(t *testing.T) {
	wh, m := safariTestWorld(t)
	testdb.Exec(t, wh.database, `UPDATE character_wallet SET pokedollars=2000 WHERE character_id=42`)
	var wg sync.WaitGroup
	results := make(chan SafariEntryResult, 4)
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := TryStartSafariZoneVisit(context.Background(), 42, NewSafariZoneManager(wh.database))
			results <- r
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	paid := 0
	for r := range results {
		if !r.Success || r.Money != 1500 {
			t.Fatalf("entry=%+v", r)
		}
		if !r.AlreadyActive {
			paid++
		}
	}
	if paid != 1 {
		t.Fatalf("new visits=%d", paid)
	}
	s, err := m.GetSession(context.Background(), 42)
	if err != nil || s == nil || s.StepsLeft != 500 {
		t.Fatalf("recovery=%+v %v", s, err)
	}
	s.StepsLeft = 1
	fresh, err := NewSafariZoneManager(wh.database).GetSession(context.Background(), 42)
	if err != nil || fresh.StepsLeft != 500 {
		t.Fatalf("snapshot mutated owner=%+v %v", fresh, err)
	}
}
func TestSafariEntryCommitFailureRollsBackPaymentAndFlags(t *testing.T) {
	wh, m := safariTestWorld(t)
	testdb.Exec(t, wh.database, `UPDATE character_wallet SET pokedollars=1000 WHERE character_id=42;
 CREATE FUNCTION reject_safari_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late safari failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_safari_commit AFTER INSERT ON character_safari_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_safari_commit();`)
	result, err := TryStartSafariZoneVisit(context.Background(), 42, m)
	if err == nil || result.Success {
		t.Fatalf("failure=%+v %v", result, err)
	}
	var money int
	if err := wh.database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil {
		t.Fatal(err)
	}
	s, err := m.GetSession(context.Background(), 42)
	if err != nil || s != nil || money != 1000 {
		t.Fatalf("partial entry=%+v money=%d err=%v", s, money, err)
	}
	on, err := queryEventFlag(wh.database, 42, EventInSafariZone)
	if err != nil || on {
		t.Fatalf("partial flag=%t %v", on, err)
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_safari_commit ON character_safari_state`)
	result, err = TryStartSafariZoneVisit(context.Background(), 42, m)
	if err != nil || !result.Success || result.Money != 500 {
		t.Fatalf("retry=%+v %v", result, err)
	}
}
func seedSafariBattle(t *testing.T, m *SafariZoneManager, balls int) {
	t.Helper()
	wild, err := pokebattle.BuildWildPokemon(m.database, 129, 5)
	if err != nil {
		t.Fatal(err)
	}
	// Eliminate flee and first-roll rejection without changing production RNG.
	wild.Speed = 0
	b := pokebattle.NewSafariBattle(wild, balls, 499)
	b.CurrentCatchRate = 255
	if err := m.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: balls, StepsLeft: 499, Battle: b}); err != nil {
		t.Fatal(err)
	}
}

func safariFixtureAction(t *testing.T, m *SafariZoneManager, action string) (safariActionResult, error) {
	t.Helper()
	s, err := m.GetSession(context.Background(), 42)
	if err != nil {
		return safariActionResult{}, err
	}
	if s == nil || s.Battle == nil {
		t.Fatal("fixture has no Safari encounter")
	}
	return m.act(context.Background(), 42, action, BattleCommandIdentity{BattleID: s.Battle.BattleID, Revision: s.Battle.Revision})
}

func TestSafariIdentitySerializesDuplicatesAndProtectsReplacement(t *testing.T) {
	_, m := safariTestWorld(t)
	seedSafariBattle(t, m, 30)
	before, err := m.GetSession(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	identity := BattleCommandIdentity{BattleID: before.Battle.BattleID, Revision: before.Battle.Revision}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := m.act(context.Background(), 42, "bait", identity); results <- err }()
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("duplicate commits=%d", successes)
	}
	after, err := m.GetSession(context.Background(), 42)
	if err != nil || after.Battle.Revision != identity.Revision+1 || after.Battle.TurnNum != 1 || after.BallsLeft != 30 {
		t.Fatal("duplicate advanced encounter more than once", err)
	}
	current := BattleCommandIdentity{BattleID: after.Battle.BattleID, Revision: after.Battle.Revision}
	if _, err := m.act(context.Background(), 42, "close", current); err == nil {
		t.Fatal("dismissed live encounter")
	}
	if _, err := m.act(context.Background(), 42, "run", current); err != nil {
		t.Fatal(err)
	}
	terminal, err := m.GetSession(context.Background(), 42)
	if err != nil || terminal.Battle == nil || !terminal.Battle.IsOver() || len(terminal.Battle.Events) != 1 || terminal.Battle.Events[0].Type != "run" {
		t.Fatal("terminal encounter not retained", err)
	}
	closing := BattleCommandIdentity{BattleID: terminal.Battle.BattleID, Revision: terminal.Battle.Revision}
	if _, err := m.act(context.Background(), 42, "close", closing); err != nil {
		t.Fatal(err)
	}
	seedSafariBattle(t, m, 30)
	if _, err := m.act(context.Background(), 42, "close", closing); err == nil {
		t.Fatal("old dismissal affected replacement encounter")
	}
	replacement, err := m.GetSession(context.Background(), 42)
	if err != nil || replacement.Battle.BattleID == identity.BattleID || replacement.Battle.Revision != 1 || replacement.Battle.TurnNum != 0 {
		t.Fatal("replacement changed after stale close", err)
	}
}

func TestLegacySafariIdentityUpgradeCommitsBeforeAdvertisement(t *testing.T) {
	wh, m := safariTestWorld(t)
	seedSafariBattle(t, m, 30)
	s, err := m.GetSession(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	s.Battle.BattleID, s.Battle.Revision = "", 0
	encoded, err := json.Marshal(storedSafariState{Version: 1, Visit: s})
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, wh.database, `UPDATE character_safari_state SET state_json=$1 WHERE character_id=42`, string(encoded))
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_safari_upgrade() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late upgrade failure'; END $$; CREATE CONSTRAINT TRIGGER reject_safari_upgrade AFTER UPDATE ON character_safari_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_safari_upgrade();`)
	if got, err := m.GetSession(context.Background(), 42); err == nil || got != nil {
		t.Fatal("advertised uncommitted legacy identity")
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_safari_upgrade ON character_safari_state`)
	upgraded, err := m.GetSession(context.Background(), 42)
	if err != nil || upgraded.Battle.BattleID == "" || upgraded.Battle.Revision != 1 || upgraded.Battle.TurnNum != 0 || upgraded.BallsLeft != 30 || upgraded.StepsLeft != 499 {
		t.Fatal("legacy upgrade changed gameplay", err)
	}
	again, err := NewSafariZoneManager(wh.database).GetSession(context.Background(), 42)
	if err != nil || again.Battle.BattleID != upgraded.Battle.BattleID {
		t.Fatal("identity was not durable", err)
	}
	var version int
	if err := wh.database.QueryRow(`SELECT (state_json::json->>'version')::int FROM character_safari_state WHERE character_id=42`).Scan(&version); err != nil || version != safariStateVersion {
		t.Fatal("legacy state not migrated", err)
	}
}
func TestSafariTurnCommitFailureDoesNotPublishOrMutateSnapshot(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	ses.MapID = 220
	ses.Client.CharData().MapID = 220
	seedSafariBattle(t, wh.Safari, 30)
	before, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	testdb.Exec(t, database, `CREATE FUNCTION reject_safari_turn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late safari failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_safari_turn AFTER UPDATE ON character_safari_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_safari_turn();`)
	battleDispatch(t, wh, ses, opcodes.SafariBattleActionRequest, `{"action":"run"}`)
	if len(messages.streams) != 1 {
		t.Fatalf("failure published %+v", messages.streams)
	}
	var response struct{ Success bool }
	if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || response.Success {
		t.Fatalf("failure=%+v %v", response, err)
	}
	saved, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || saved.Battle == nil || saved.Battle.IsOver() || saved.VisitID != before.VisitID || saved.Revision != before.Revision {
		t.Fatalf("partial turn=%+v %v", saved, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_safari_turn ON character_safari_state`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.SafariBattleActionRequest, `{"action":"run"}`)
	saved, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || saved.Battle == nil || !saved.Battle.IsOver() || len(saved.Battle.Events) != 1 || saved.Battle.Events[0].Type != "run" || !saved.Active {
		t.Fatalf("retry=%+v %v", saved, err)
	}
	battleDispatch(t, wh, ses, opcodes.SafariBattleActionRequest, `{"action":"close"}`)
	saved, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || saved.Battle != nil {
		t.Fatal("committed dismissal retained terminal encounter", err)
	}
}
func TestSafariCaptureCommitFailurePreservesPokemonAndDex(t *testing.T) {
	wh, m := safariTestWorld(t)
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_safari_capture() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late capture failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_safari_capture AFTER INSERT ON character_pokemon DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_safari_capture();`)
	// Every ball has the real 86/256 HP roll. Try independent prepared encounters
	// until a catch reaches the late-failure boundary; never alter its mechanics.
	failedCatch := false
	for i := 0; i < 128; i++ {
		seedSafariBattle(t, m, 1)
		_, err := safariFixtureAction(t, m, "ball")
		if err != nil {
			failedCatch = true
			break
		}
	}
	if !failedCatch {
		t.Fatal("no capture reached commit failure")
	}
	s, err := m.GetSession(context.Background(), 42)
	if err != nil || s.Battle == nil || s.BallsLeft != 1 || s.Battle.TurnNum != 0 {
		t.Fatalf("partial catch=%+v %v", s, err)
	}
	var count, dex int
	if err := wh.database.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id=42`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := wh.database.QueryRow(`SELECT COUNT(*) FROM character_pokedex WHERE character_id=42 AND pokemon_id=129`).Scan(&dex); err != nil {
		t.Fatal(err)
	}
	if count != 1 || dex != 0 {
		t.Fatalf("partial storage pokemon=%d dex=%d", count, dex)
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_safari_capture ON character_pokemon`)
	caught := false
	for i := 0; i < 128; i++ {
		seedSafariBattle(t, m, 1)
		r, err := safariFixtureAction(t, m, "ball")
		if err != nil {
			t.Fatal(err)
		}
		if r.Battle.Caught {
			caught = true
			break
		}
	}
	if !caught {
		t.Fatal("no committed catch")
	}
	s, err = m.GetSession(context.Background(), 42)
	if err != nil || s.Active || s.Battle == nil || !s.Battle.Caught || !s.Battle.IsOver() || s.Capture == nil || s.BallsLeft != 0 {
		t.Fatalf("last ball state=%+v %v", s, err)
	}
	if err := wh.database.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id=42`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if err := wh.database.QueryRow(`SELECT caught FROM character_pokedex WHERE character_id=42 AND pokemon_id=129`).Scan(&dex); err != nil {
		t.Fatal(err)
	}
	if count != 2 || dex != 1 {
		t.Fatalf("committed capture pokemon=%d dex=%d", count, dex)
	}
	if _, err := safariFixtureAction(t, m, "ball"); err == nil {
		t.Fatal("replayed capture accepted")
	}
}
func TestSafariExpiryCommitFailurePublishesNothing(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 3, StepsLeft: 1}); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `UPDATE character_data SET map_id=220,x=14,y=24 WHERE id=42;
 CREATE FUNCTION reject_safari_expiry() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late expiry failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_safari_expiry AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.map_id=156) EXECUTE FUNCTION reject_safari_expiry();`)
	if !CheckSafariStep(42, 14, 24, 220, ses, wh) {
		t.Fatal("failed step did not stop movement")
	}
	if len(messages.streams) != 0 {
		t.Fatalf("failure published=%+v", messages.streams)
	}
	s, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || !s.Active || s.StepsLeft != 1 {
		t.Fatalf("partial expiry=%+v %v", s, err)
	}
	var mapID int
	if err := database.QueryRow(`SELECT map_id FROM character_data WHERE id=42`).Scan(&mapID); err != nil || mapID != 220 {
		t.Fatalf("partial position=%d %v", mapID, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_safari_expiry ON character_data`)
	CheckSafariStep(42, 14, 24, 220, ses, wh)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.SafariZoneExitNotify {
		t.Fatalf("expiry=%+v", messages.streams)
	}
	var notice protocol.SafariZoneExitNotify
	saved, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || json.Unmarshal(messages.streams[0].payload, &notice) != nil || notice.CharacterID != 42 || notice.VisitID != saved.VisitID || notice.VisitRevision != saved.Revision || notice.Active || notice.Message != SafariExpiryMessage {
		t.Fatalf("unscoped/incorrect expiry=%+v state=%+v error=%v", notice, saved, err)
	}

	if err := database.QueryRow(`SELECT map_id FROM character_data WHERE id=42`).Scan(&mapID); err != nil || mapID != 156 {
		t.Fatalf("expiry position=%d %v", mapID, err)
	}
	s, err = NewSafariZoneManager(database).GetSession(context.Background(), 42)
	if err != nil || s.Active || s.StepsLeft != 0 {
		t.Fatalf("recovered expiry=%+v %v", s, err)
	}
}
func TestSafariCorruptionAndMissingSchemaFailClosed(t *testing.T) {
	wh, m := safariTestWorld(t)
	testdb.Exec(t, wh.database, `INSERT INTO character_safari_state(character_id,state_json) VALUES(42,'{"version":99,"visit":{}}')`)
	if _, err := m.GetSession(context.Background(), 42); err == nil {
		t.Fatal("unsupported version accepted")
	}
	if _, err := TryStartSafariZoneVisit(context.Background(), 42, m); err == nil {
		t.Fatal("corruption treated as inactive")
	}
	testdb.Exec(t, wh.database, `DROP TABLE character_safari_state`)
	if err := m.Load(context.Background()); err == nil {
		t.Fatal("readiness accepted missing state schema")
	}
	movement := &PlayerMovementManager{wh: wh}
	if !movement.isSafariEntryWarpBlocked(context.Background(), 42, 156, 220, nil) {
		t.Fatal("missing state table allowed entry")
	}
}

func TestSafariScriptEntryAndExitJoinOuterCommit(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	testdb.Exec(t, database, `UPDATE character_wallet SET pokedollars=1000 WHERE character_id=42;
 CREATE FUNCTION reject_safari_script() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late script failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_safari_script AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(NEW.map_id=220) EXECUTE FUNCTION reject_safari_script();`)
	ctx := CutsceneActionContext{Database: database, Session: ses, WorldHandler: wh, EventFlags: wh.EventFlags}
	actions := json.RawMessage(`[{"type":"startSafariSession"}]`)
	if _, _, err := ApplyCutsceneActionList(context.Background(), ctx, "SAFARI_ZONE_GATE", actions, 42); err == nil {
		t.Fatal("script ignored commit failure")
	}
	if len(messages.streams) != 0 {
		t.Fatalf("uncommitted script published=%+v", messages.streams)
	}
	s, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s != nil {
		t.Fatalf("script leaked visit=%+v %v", s, err)
	}
	var money int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 1000 {
		t.Fatalf("partial payment=%d %v", money, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_safari_script ON character_data`)
	if _, _, err := ApplyCutsceneActionList(context.Background(), ctx, "SAFARI_ZONE_GATE", actions, 42); err != nil {
		t.Fatal(err)
	}
	s, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s == nil || !s.Active || !wh.EventFlags.CheckFlag(42, EventInSafariZone) {
		t.Fatalf("committed entry=%+v %v", s, err)
	}
	testdb.Exec(t, database, `CREATE CONSTRAINT TRIGGER reject_safari_end AFTER DELETE ON character_safari_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_safari_script();`)
	messages.streams = nil
	exit := json.RawMessage(`[{"type":"endSafariSession"}]`)
	if _, _, err := ApplyCutsceneActionList(context.Background(), ctx, "SAFARI_ZONE_GATE", exit, 42); err == nil {
		t.Fatal("script exit ignored commit failure")
	}
	s, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s == nil || !s.Active || len(messages.streams) != 0 || !wh.EventFlags.CheckFlag(42, EventInSafariZone) {
		t.Fatalf("partial exit=%+v %v messages=%+v", s, err, messages.streams)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_safari_end ON character_safari_state`)
	if _, _, err := ApplyCutsceneActionList(context.Background(), ctx, "SAFARI_ZONE_GATE", exit, 42); err != nil {
		t.Fatal(err)
	}
	s, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s != nil || wh.EventFlags.CheckFlag(42, EventInSafariZone) {
		t.Fatalf("exit=%+v %v", s, err)
	}
}
func TestSafariFullPartyCaptureUsesPCAndRetainsExistingIdentities(t *testing.T) {
	wh, m := safariTestWorld(t)
	testdb.Exec(t, wh.database, `INSERT INTO character_pokemon(character_id,party_slot,box_slot,pokemon_id,level,exp,cur_hp,max_hp) SELECT 42,s,s,25,50,125000,1,95 FROM generate_series(1,5) s`)
	var original int64
	if err := wh.database.QueryRow(`SELECT id FROM character_pokemon WHERE character_id=42 AND party_slot=0`).Scan(&original); err != nil {
		t.Fatal(err)
	}
	var result safariActionResult
	for i := 0; i < 128; i++ {
		seedSafariBattle(t, m, 30)
		r, err := safariFixtureAction(t, m, "ball")
		if err != nil {
			t.Fatal(err)
		}
		if r.Battle.Caught {
			result = r
			break
		}
	}
	if result.Battle == nil || !result.SentToPC || result.PCBox != 0 {
		t.Fatalf("pc capture=%+v", result)
	}
	var retained int64
	var species int
	if err := wh.database.QueryRow(`SELECT id FROM character_pokemon WHERE character_id=42 AND party_slot=0 AND box=-1`).Scan(&retained); err != nil || retained != original {
		t.Fatalf("party identity=%d %v", retained, err)
	}
	if err := wh.database.QueryRow(`SELECT pokemon_id FROM character_pokemon WHERE character_id=42 AND box=0 AND box_slot=0`).Scan(&species); err != nil || species != 129 {
		t.Fatalf("PC pokemon=%d %v", species, err)
	}
}
func TestSafariDirectEntryCannotChargeRemotePlayer(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	testdb.Exec(t, database, `UPDATE character_wallet SET pokedollars=1000 WHERE character_id=42`)
	battleDispatch(t, wh, ses, opcodes.SafariZoneEnterRequest, `{}`)
	var money int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 1000 {
		t.Fatalf("remote charge=%d %v", money, err)
	}
	s, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s != nil {
		t.Fatalf("remote visit=%+v %v", s, err)
	}
	if len(messages.streams) == 0 {
		t.Fatal("missing rejection")
	}
}

func TestWarpHomeCommitsBattleSafariAndPositionTogether(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	battle := battleTestStart(t, database, false, nil)
	if err := wh.Safari.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `UPDATE character_data SET map_id=220,x=14,y=24 WHERE id=42;
 CREATE FUNCTION reject_home_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late home failure'; END $$;
 CREATE CONSTRAINT TRIGGER reject_home_commit AFTER DELETE ON character_safari_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_home_commit();`)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	battleDispatch(t, wh, ses, opcodes.WarpHomeRequest, `{}`)
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.WarpHomeResponse {
		t.Fatalf("failed warp published=%+v", messages.streams)
	}
	saved, err := pokebattle.ResumeBattle(context.Background(), database, 42)
	if err != nil || saved == nil || getBattle(42) != battle {
		t.Fatalf("partial battle removal=%+v %v", saved, err)
	}
	s, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s == nil || !s.Active {
		t.Fatalf("partial safari=%+v %v", s, err)
	}
	var mapID int
	if err := database.QueryRow(`SELECT map_id FROM character_data WHERE id=42`).Scan(&mapID); err != nil || mapID != 220 {
		t.Fatalf("partial warp=%d %v", mapID, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_home_commit ON character_safari_state`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.WarpHomeRequest, `{}`)
	saved, err = pokebattle.ResumeBattle(context.Background(), database, 42)
	if err != nil || saved != nil || getBattle(42) != nil {
		t.Fatalf("battle retained=%+v %v", saved, err)
	}
	s, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s != nil {
		t.Fatalf("safari retained=%+v %v", s, err)
	}
	if err := database.QueryRow(`SELECT map_id FROM character_data WHERE id=42`).Scan(&mapID); err != nil || mapID != RecoverySpawnMap {
		t.Fatalf("home=%d %v", mapID, err)
	}
}

func TestSafariDirectEntryRequiresVisibleSourceWorker(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	wh.Safari = NewSafariZoneManager(database)
	testdb.Exec(t, database, `UPDATE character_wallet SET pokedollars=1000 WHERE character_id=42;
 INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(156,'SAFARI_ZONE_GATE',8,6,0);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,name,text) VALUES(10,156,6,2,'npc','WORKER1','TEXT_SAFARIZONEGATE_SAFARI_ZONE_WORKER1');
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,talk_over_tile) VALUES(156,5,2,1,true);
 INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(42,10,false,'test')`)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	ses.Client.CharData().MapID = 156
	ses.Client.CharData().X = 4
	ses.Client.CharData().Y = 2
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	battleDispatch(t, wh, ses, opcodes.SafariZoneEnterRequest, `{}`)
	s, err := wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s != nil {
		t.Fatalf("hidden worker granted visit=%+v %v", s, err)
	}
	testdb.Exec(t, database, `DELETE FROM character_object_visibility_overrides`)
	messages.streams = nil
	battleDispatch(t, wh, ses, opcodes.SafariZoneEnterRequest, `{}`)
	s, err = wh.Safari.GetSession(context.Background(), 42)
	if err != nil || s == nil || !s.Active {
		t.Fatalf("visible worker denied visit=%+v %v messages=%+v", s, err, messages.streams)
	}
	var money int
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 500 {
		t.Fatalf("entry money=%d %v", money, err)
	}
	// Repeated status/entry recovers the same visit without a second charge.
	battleDispatch(t, wh, ses, opcodes.SafariZoneEnterRequest, `{}`)
	if err := database.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id=42`).Scan(&money); err != nil || money != 500 {
		t.Fatalf("re-entry charge=%d %v", money, err)
	}
}

func TestSafariVisitIdentitySurvivesReadsAndRevisionsAndChangesForNewVisit(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	manager := NewSafariZoneManager(database)
	if err := manager.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	first, err := manager.GetSession(context.Background(), 42)
	if err != nil || first.VisitID == "" || first.Revision != 1 {
		t.Fatalf("first=%+v error=%v", first, err)
	}
	again, err := manager.GetSession(context.Background(), 42)
	if err != nil || again.VisitID != first.VisitID || again.Revision != first.Revision {
		t.Fatalf("read changed identity=%+v error=%v", again, err)
	}
	if _, _, _, err := manager.DecrementStep(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	stepped, err := manager.GetSession(context.Background(), 42)
	if err != nil || stepped.VisitID != first.VisitID || stepped.Revision != 2 || stepped.StepsLeft != 499 {
		t.Fatalf("stepped=%+v error=%v", stepped, err)
	}
	if err := manager.EndSession(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetSession(context.Background(), 42, SafariSession{Active: true, BallsLeft: 30, StepsLeft: 500}); err != nil {
		t.Fatal(err)
	}
	next, err := manager.GetSession(context.Background(), 42)
	if err != nil || next.VisitID == first.VisitID || next.Revision != 1 {
		t.Fatalf("new visit=%+v error=%v", next, err)
	}
}

func TestSafariV2VisitUpgradeIsCommittedAndCurrentIdentityCorruptionRejects(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	manager := NewSafariZoneManager(database)
	encoded, err := json.Marshal(storedSafariState{Version: 2, Visit: &SafariSession{Active: true, BallsLeft: 30, StepsLeft: 499}})
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `INSERT INTO character_safari_state(character_id,state_json) VALUES(42,$1)`, string(encoded))
	testdb.Exec(t, database, `CREATE FUNCTION reject_visit_upgrade() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'visit upgrade rejected'; END $$; CREATE CONSTRAINT TRIGGER reject_visit_upgrade AFTER UPDATE ON character_safari_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_visit_upgrade()`)
	if got, err := manager.GetSession(context.Background(), 42); err == nil || got != nil {
		t.Fatalf("published failed upgrade=%+v error=%v", got, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_visit_upgrade ON character_safari_state`)
	got, err := manager.GetSession(context.Background(), 42)
	if err != nil || got.VisitID == "" || got.Revision != 1 || got.StepsLeft != 499 {
		t.Fatalf("upgrade=%+v error=%v", got, err)
	}
	testdb.Exec(t, database, `UPDATE character_safari_state SET state_json=jsonb_set(state_json::jsonb,'{visit,visitId}','"broken"')::text WHERE character_id=42`)
	if got, err := manager.GetSession(context.Background(), 42); err == nil || got != nil {
		t.Fatalf("manufactured replacement identity=%+v error=%v", got, err)
	}
}
