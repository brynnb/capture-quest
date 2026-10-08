package world

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestPlaytimeUnknownCommitRetryUsesCumulativeOwnedTotal(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	testdb.Exec(t, database, `UPDATE character_data SET time_played=20 WHERE id=42`)
	started := time.Now()
	ses.StartPlaytime(started, 20, 42)
	unknown := errors.New("commit acknowledgement lost")
	_, err := ses.PersistPlaytime(started.Add(3*time.Second), func(id int32, total uint32) error {
		if err := db_character.SaveCharacterPlaytime(context.Background(), database, id, ses.AccountID, total); err != nil {
			t.Fatal(err)
		}
		// The durable commit happened, but the session did not learn that fact.
		return unknown
	})
	if !errors.Is(err, unknown) || ses.CurrentPlaytime(started.Add(3*time.Second)) != 23 {
		t.Fatalf("unknown save=%v current=%d", err, ses.CurrentPlaytime(started.Add(3*time.Second)))
	}
	for _, seconds := range []int{3, 3, 5} {
		if err := wh.persistSessionPlaytime(context.Background(), ses, started.Add(time.Duration(seconds)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	var total uint32
	if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id=42`).Scan(&total); err != nil || total != 25 {
		t.Fatalf("retried total=%d error=%v", total, err)
	}
	// A subsequent owner starts from the post-drain durable total.
	ses.StopPlaytime()
	next := &session.Session{}
	next.StartPlaytime(started.Add(10*time.Second), total, 42)
	if err := wh.persistSessionPlaytime(context.Background(), next, started.Add(12*time.Second)); err != nil {
		t.Fatal(err)
	}
	// An older cumulative save cannot rewind that owner's contribution.
	if err := db_character.SaveCharacterPlaytime(context.Background(), database, 42, ses.AccountID, 23); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id=42`).Scan(&total); err != nil || total != 27 {
		t.Fatalf("next owner total=%d error=%v", total, err)
	}
}

func TestPlaytimeCancellationKeepsIntervalForRetryOnCapturedDatabase(t *testing.T) {
	database, wh, ses, _ := battleTestWorld(t)
	now := time.Now()
	ses.StartPlaytime(now.Add(-3*time.Second), 0, 42)
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`UPDATE character_data SET id=id WHERE id=42`); err != nil {
		t.Fatal(err)
	}
	db.GlobalWorldDB = nil
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- wh.persistSessionPlaytime(ctx, ses, now) }()
	select {
	case err := <-done:
		if err == nil || ctx.Err() == nil {
			t.Fatalf("cancelled save=%v context=%v", err, ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("playtime ignored caller deadline")
	}
	if ses.CurrentPlaytime(now) != 3 {
		t.Fatal("failed save advanced playtime boundary")
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := wh.persistSessionPlaytime(context.Background(), ses, now); err != nil {
			t.Fatal(err)
		}
	}
	var seconds int
	if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id=42`).Scan(&seconds); err != nil || seconds != 3 {
		t.Fatalf("retry playtime=%d %v", seconds, err)
	}
}

func TestFailedFinalSaveRejectsCharacterHandoffAndRetiresOldConnection(t *testing.T) {
	database, wh, old, _ := battleTestWorld(t)
	wh.sessionManager = session.NewSessionManager()
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	old.StartPlaytime(time.Now().Add(-3*time.Second), 0, 42)
	wh.PlayerMovement.RegisterPlayer(old, 42, 7, 8, 50, "UP")
	if err := wh.characterOwners.acquire(context.Background(), 42, old, nil); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `CREATE FUNCTION reject_final_save() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject final save'; END $$;
 CREATE CONSTRAINT TRIGGER reject_final_save AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_final_save();`)
	next := &session.Session{}
	err := wh.characterOwners.acquire(context.Background(), 42, next, wh.cleanupCharacterSession)
	if err == nil || strings.Contains(err.Error(), "final position") || !strings.Contains(err.Error(), "final playtime") {
		t.Fatalf("cleanup errors=%v", err)
	}
	if !old.IsClosed() || old.HasValidClient() || wh.characterOwners.owns(42, old) || wh.characterOwners.owns(42, next) {
		t.Fatal("failed cleanup admitted replacement or retained live connection")
	}
	if _, _, _, ok := wh.PlayerMovement.GetPosition(42); ok {
		t.Fatal("failed cleanup left movement writer registered")
	}
	var seconds int
	if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id=42`).Scan(&seconds); err != nil || seconds != 0 {
		t.Fatalf("failed final save changed playtime=%d %v", seconds, err)
	}
	frozen := old.CurrentPlaytime(time.Now().Add(time.Hour))
	if frozen < 3 || frozen != old.CurrentPlaytime(time.Now()) {
		t.Fatalf("retired playtime continued accumulating: %d", frozen)
	}
	// A second attempt must recover the same obligations, not silently
	// enter after the failed owner's client and movement were discarded.
	if err := wh.characterOwners.acquire(context.Background(), 42, next, wh.cleanupCharacterSession); err == nil || !strings.Contains(err.Error(), "final playtime") {
		t.Fatalf("repeated failed admission=%v", err)
	}
	if wh.characterOwners.owns(42, next) {
		t.Fatal("replacement bypassed pending cleanup")
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_final_save ON character_data`)
	if err := wh.characterOwners.acquire(context.Background(), 42, next, wh.cleanupCharacterSession); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id=42`).Scan(&seconds); err != nil || seconds != int(frozen) {
		t.Fatalf("recovered final total=%d want=%d error=%v", seconds, frozen, err)
	}

	if err := wh.cleanupCharacterSession(context.Background(), old); err != nil || !wh.characterOwners.owns(42, next) {
		t.Fatalf("late retired cleanup=%v removed replacement", err)
	}
	wh.characterOwners.release(42, next)
	if len(wh.characterOwners.entries) != 0 {
		t.Fatal("successful recovery retained pending obligations")
	}
}

func TestEnterWorldRecoversFailedDisconnectBeforeLoadingPlaytimeBaseline(t *testing.T) {
	database, wh, old, _ := battleTestWorld(t)
	testdb.Exec(t, database, `UPDATE character_data SET x=7,y=8,map_id=50 WHERE id=42`)
	wh.sessionManager = session.NewSessionManager()
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	wh.WildEncounter = NewWildEncounterManager(wh, wh.database)
	old.StartPlaytime(time.Now().Add(-3*time.Second), 0, 42)
	wh.PlayerMovement.RegisterPlayer(old, 42, 7, 8, 50, "UP")
	if err := wh.characterOwners.acquire(context.Background(), 42, old, nil); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `CREATE FUNCTION reject_playtime_save() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject playtime'; END $$;
 CREATE CONSTRAINT TRIGGER reject_playtime_save AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_playtime_save();`)
	if err := wh.cleanupCharacterSession(context.Background(), old); err == nil {
		t.Fatal("failed disconnect did not report persistence error")
	}
	frozen := old.CurrentPlaytime(time.Now())
	testdb.Exec(t, database, `DROP TRIGGER reject_playtime_save ON character_data`)
	next := &session.Session{Authenticated: true, AccountID: old.AccountID, Messenger: &recordingMessenger{}}
	battleDispatch(t, wh, next, opcodes.EnterWorld, `{"name":"battle"}`)
	if !next.HasValidClient() || next.Client.CharData().TimePlayed != frozen || next.CurrentPlaytime(time.Now()) < frozen || !wh.characterOwners.owns(42, next) {
		t.Fatal("entry did not reload the recovered playtime baseline before starting its tracker")
	}
	next.Close()
	next.DrainCommands(func() {
		if err := wh.cleanupCharacterSession(context.Background(), next); err != nil {
			t.Error(err)
		}
	})
}

func TestCharacterHandoffReturnsPersistenceFailure(t *testing.T) {
	var owners characterOwners
	old, next := &session.Session{}, &session.Session{}
	if err := owners.acquire(context.Background(), 42, old, nil); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("cleanup persistence failure")
	err := owners.acquire(context.Background(), 42, next, func(ctx context.Context, s *session.Session) error { owners.release(42, s); return failure })
	if !errors.Is(err, failure) || owners.owns(42, next) {
		t.Fatalf("handoff=%v replacement=%v", err, owners.owns(42, next))
	}
}

func TestCleanupRetiresProjectionWithoutRewritingDurablePositionOrRoute(t *testing.T) {
	database, wh, old, _ := battleTestWorld(t)
	wh.sessionManager = session.NewSessionManager()
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	wh.PlayerMovement.RegisterPlayer(old, 42, 7, 8, 50, "UP")
	if err := wh.characterOwners.acquire(context.Background(), 42, old, nil); err != nil {
		t.Fatal(err)
	}
	seedRoute(t, wh, []PathNode{{X: 8, Y: 8}})
	// A projection can lag durable state. Cleanup must never replay that pose.
	testdb.Exec(t, database, `UPDATE character_data SET x=9,y=8,map_id=50 WHERE id=42`)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := wh.cleanupCharacterSession(ctx, old); err != nil || database.Stats().WaitCount != before {
		t.Fatalf("cleanup rewrote position: %v", err)
	}
	if old.HasValidClient() || wh.characterOwners.owns(42, old) {
		t.Fatal("cleanup retained live character owner")
	}
	if _, _, _, ok := wh.PlayerMovement.GetPosition(42); ok {
		t.Fatal("cleanup retained movement projection")
	}
	lease.Close()
	var x int
	if err := database.QueryRow(`SELECT x FROM character_data WHERE id=42`).Scan(&x); err != nil || x != 9 {
		t.Fatalf("cleanup rewound durable position=%d error=%v", x, err)
	}
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_movement_routes WHERE character_id=42`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cleanup changed durable route count=%d error=%v", count, err)
	}
}
