package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func seedCharacterCreationCatalog(t *testing.T, database *sql.DB) {
	t.Helper()
	t.Setenv("CAPTUREQUEST_TEST_MODE", "false")
	t.Setenv("CAPTUREQUEST_LOCAL_FIXTURES", "false")
	testdb.Exec(t, database, `INSERT INTO account(id,name) VALUES(7,'creation-owner') ON CONFLICT(id) DO NOTHING;
	INSERT INTO poke_classes(id,name,class_type) VALUES(1,'Trainer','PLAYER');
	INSERT INTO poke_factions(id,name,is_playable,is_starting) VALUES(1,'Player',1,1),(2,'Unavailable',1,0);
	INSERT INTO poke_start_cities(id,map_id,name) VALUES(1,0,'Pallet Town')`)
}

func validCharacterCreateRequest(name string) CharCreateRequest {
	return CharCreateRequest{Name: name, FactionID: 1, CharClass: 1, Gender: 2, HomeTownMapID: 0, RivalName: "Blue"}
}

func characterCreationFixture(t *testing.T) (*sql.DB, *WorldHandler, *session.Session, *entryRecordingMessenger) {
	t.Helper()
	database := testdb.Postgres(t)
	seedCharacterCreationCatalog(t, database)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	messages := &entryRecordingMessenger{}
	return database, &WorldHandler{database: database}, &session.Session{AccountID: 7, Authenticated: true, Messenger: messages}, messages
}

func characterCreatePayload(t *testing.T, req CharCreateRequest) string {
	t.Helper()
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func creationAcknowledgement(t *testing.T, messages *entryRecordingMessenger, success bool) {
	t.Helper()
	replies := messages.snapshot()
	expected := 1
	if success {
		expected = 2
	}
	if len(replies) != expected || replies[0].opcode != opcodes.CharacterCreateResponse {
		t.Fatalf("creation streams=%+v", replies)
	}
	var reply SimpleSuccessResponse
	if err := json.Unmarshal(replies[0].payload, &reply); err != nil || (reply.Value == 1) != success {
		t.Fatalf("creation response=%s error=%v", replies[0].payload, err)
	}
	if success && replies[1].opcode != opcodes.SendCharInfo {
		t.Fatalf("creation did not publish current roster: %+v", replies)
	}
}

func TestCharacterCreateCommitsNewIdentityBeforePublication(t *testing.T) {
	database, wh, ses, messages := characterCreationFixture(t)
	database.SetMaxOpenConns(1)
	messages.afterSend = func(opcode opcodes.OpCode) {
		if opcode != opcodes.CharacterCreateResponse {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var characters, binds int
		if err := database.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM character_data),(SELECT COUNT(*) FROM character_bind)`).Scan(&characters, &binds); err != nil || characters != 1 || binds != 5 {
			t.Errorf("success preceded complete committed creation: characters=%d binds=%d error=%v", characters, binds, err)
		}
	}
	battleDispatch(t, wh, ses, opcodes.CharacterCreate, characterCreatePayload(t, validCharacterCreateRequest("Newowner")))
	creationAcknowledgement(t, messages, true)
	var id, mapID, faction, class, gender int
	var x, y float64
	var rival string
	if err := database.QueryRow(`SELECT id,map_id,x,y,faction_id,class,gender,options->>'rivalName' FROM character_data WHERE name='Newowner' AND account_id=7`).Scan(&id, &mapID, &x, &y, &faction, &class, &gender, &rival); err != nil {
		t.Fatal(err)
	}
	if id <= 0 || mapID != DefaultSpawnMap || x != DefaultSpawnX || y != DefaultSpawnY || faction != 1 || class != 1 || gender != 2 || rival != "Blue" {
		t.Fatalf("created profile id=%d map=%d pose=%f,%f selections=%d/%d/%d rival=%q", id, mapID, x, y, faction, class, gender, rival)
	}
}

func TestCharacterCreateFailureRollsBackIdentityBindsAndFixtures(t *testing.T) {
	for _, mode := range []string{"bind", "deferred_commit", "local_fixture"} {
		t.Run(mode, func(t *testing.T) {
			database, wh, ses, messages := characterCreationFixture(t)
			switch mode {
			case "bind":
				testdb.Exec(t, database, `CREATE FUNCTION reject_creation_bind() RETURNS trigger LANGUAGE plpgsql AS $fn$ BEGIN RAISE EXCEPTION 'reject creation bind'; END $fn$;
				CREATE TRIGGER reject_creation_bind AFTER INSERT ON character_bind FOR EACH ROW EXECUTE FUNCTION reject_creation_bind()`)
			case "deferred_commit":
				testdb.Exec(t, database, `CREATE FUNCTION reject_creation_commit() RETURNS trigger LANGUAGE plpgsql AS $fn$ BEGIN RAISE EXCEPTION 'reject creation commit'; END $fn$;
				CREATE CONSTRAINT TRIGGER reject_creation_commit AFTER INSERT ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_creation_commit()`)
			case "local_fixture":
				cfg, err := config.Get()
				if err != nil {
					t.Fatal(err)
				}
				previous := cfg.Local
				cfg.Local = true
				t.Cleanup(func() { cfg.Local = previous })
				t.Setenv("CAPTUREQUEST_LOCAL_FIXTURES", "true")
				testdb.Exec(t, database, `INSERT INTO phaser_pokemon(id,name,type_1,hp,atk,def,spd,spc,catch_rate,base_exp,growth_rate)
				SELECT id,'TEST','NORMAL',35,55,30,90,50,190,82,'MEDIUM_FAST' FROM unnest(ARRAY[4,25,7,1,16,39]) id;
				INSERT INTO phaser_moves(id,constant_name,name,short_name,effect,power,type,accuracy,pp) VALUES(150,'SPLASH','SPLASH','SPLASH','SPLASH_EFFECT',0,'NORMAL',0,40);
				INSERT INTO cq_items(id,name,short_name,is_usable,heal_amount) VALUES(1,'Potion','POTION',true,20);
				CREATE FUNCTION reject_creation_inventory() RETURNS trigger LANGUAGE plpgsql AS $fn$ BEGIN RAISE EXCEPTION 'reject creation inventory'; END $fn$;
				CREATE TRIGGER reject_creation_inventory AFTER INSERT ON cq_character_inventory FOR EACH ROW EXECUTE FUNCTION reject_creation_inventory()`)
			}
			battleDispatch(t, wh, ses, opcodes.CharacterCreate, characterCreatePayload(t, validCharacterCreateRequest("Failowner")))
			creationAcknowledgement(t, messages, false)
			var characters, binds, party, inventory int
			if err := database.QueryRow(`SELECT (SELECT COUNT(*) FROM character_data),(SELECT COUNT(*) FROM character_bind),(SELECT COUNT(*) FROM character_pokemon),(SELECT COUNT(*) FROM cq_character_inventory)`).Scan(&characters, &binds, &party, &inventory); err != nil {
				t.Fatal(err)
			}
			if characters != 0 || binds != 0 || party != 0 || inventory != 0 {
				t.Fatalf("failed creation leaked rows: characters=%d binds=%d party=%d inventory=%d", characters, binds, party, inventory)
			}
		})
	}
}

func TestCharacterCreateNeverResetsExistingIdentity(t *testing.T) {
	database, _, _, _ := characterCreationFixture(t)
	id, err := CharacterCreate(context.Background(), database, 7, validCharacterCreateRequest("Existing"))
	if err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `UPDATE character_data SET map_id=50,x=21,y=22,gm=1,time_played=777 WHERE id=$1`, id)
	var before, after string
	if err := database.QueryRow(`SELECT row_to_json(c)::text FROM character_data c WHERE id=$1`, id).Scan(&before); err != nil {
		t.Fatal(err)
	}
	req := validCharacterCreateRequest("Existing")
	req.Gender, req.RivalName = 0, "Other"
	if repeated, err := CharacterCreate(context.Background(), database, 7, req); err == nil || repeated != 0 {
		t.Fatalf("same-name creation reused identity %d error=%v", repeated, err)
	}
	if err := database.QueryRow(`SELECT row_to_json(c)::text FROM character_data c WHERE id=$1`, id).Scan(&after); err != nil || after != before {
		t.Fatalf("creation rewrote earned profile: before=%s after=%s error=%v", before, after, err)
	}
}

func TestCharacterCreateSerializesNameAndLastAccountSlot(t *testing.T) {
	for _, mode := range []string{"same_name", "other_account_same_name", "last_slot"} {
		t.Run(mode, func(t *testing.T) {
			database, _, _, _ := characterCreationFixture(t)
			secondAccount := int64(7)
			names := []string{"Concurrent", "Concurrent"}
			if mode == "other_account_same_name" {
				testdb.Exec(t, database, `INSERT INTO account(id,name) VALUES(8,'second-owner')`)
				secondAccount = 8
			}
			if mode == "last_slot" {
				testdb.Exec(t, database, `INSERT INTO character_data(id,account_id,name) SELECT id,7,'Saved'||id FROM generate_series(100,106) AS id`)
				names = []string{"Lastfirst", "Lastsecond"}
			}
			start := make(chan struct{})
			type result struct {
				id  int64
				err error
			}
			done := make(chan result, 2)
			for i, account := range []int64{7, secondAccount} {
				req := validCharacterCreateRequest(names[i])
				go func(account int64, req CharCreateRequest) {
					<-start
					id, err := CharacterCreate(context.Background(), database, account, req)
					done <- result{id, err}
				}(account, req)
			}
			close(start)
			successes := 0
			for range 2 {
				select {
				case result := <-done:
					if result.err == nil && result.id > 0 {
						successes++
					} else if result.err == nil || result.id != 0 {
						t.Fatalf("invalid failed creation=%+v", result)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("concurrent creation did not settle")
				}
			}
			if successes != 1 {
				t.Fatalf("concurrent creation successes=%d", successes)
			}
			var count int
			if err := database.QueryRow(`SELECT COUNT(*) FROM character_data WHERE deleted_at IS NULL`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 1
			if mode == "last_slot" {
				want = maximumAccountCharacters
			}
			if count != want {
				t.Fatalf("created count=%d want=%d", count, want)
			}
		})
	}
}

func TestCharacterCreateRejectsUnavailableSelections(t *testing.T) {
	database, wh, ses, messages := characterCreationFixture(t)
	for _, mode := range []string{"faction", "class", "hometown", "gender", "name", "oversized_class"} {
		req := validCharacterCreateRequest("Rejected")
		switch mode {
		case "faction":
			req.FactionID = 2
		case "class":
			req.CharClass = 9
		case "hometown":
			req.HomeTownMapID = 999
		case "gender":
			req.Gender = 3
		case "name":
			req.Name = "lowercase"
		case "oversized_class":
			req.CharClass = 256
		}
		messages.streams = nil
		battleDispatch(t, wh, ses, opcodes.CharacterCreate, characterCreatePayload(t, req))
		creationAcknowledgement(t, messages, false)
	}
	if id, err := CharacterCreate(context.Background(), database, 999, validCharacterCreateRequest("Noaccount")); err == nil || id != 0 {
		t.Fatalf("missing account created=%d error=%v", id, err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_data`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("denied creation rows=%d error=%v", count, err)
	}
}

func TestCharacterCreateAccountWaitHonorsCancellation(t *testing.T) {
	database, _, _, _ := characterCreationFixture(t)
	holder, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var holderPID, id int
	if err := holder.QueryRow(`SELECT pg_backend_pid()`).Scan(&holderPID); err != nil {
		t.Fatal(err)
	}
	if err := holder.QueryRow(`SELECT id FROM account WHERE id=7 FOR UPDATE`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	finished := false
	go func() {
		id, err := CharacterCreate(ctx, database, 7, validCharacterCreateRequest("Cancelled"))
		if id != 0 {
			err = fmt.Errorf("cancelled creation returned ID %d", id)
		}
		done <- err
	}()
	defer func() {
		cancel()
		_ = holder.Rollback()
		if !finished {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Error("creation did not drain")
			}
		}
	}()
	waitCharacterDeleteRowWait(t, database, holderPID)
	cancel()
	select {
	case err := <-done:
		finished = true
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled creation=%v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("creation ignored account cancellation")
	}
	_ = holder.Rollback()
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_data`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cancelled creation left identity=%d error=%v", count, err)
	}
}
