package world

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

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

func TestFailedFinalSavesRejectCharacterHandoffAndRetireOldConnection(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "final position") || !strings.Contains(err.Error(), "final playtime") {
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
