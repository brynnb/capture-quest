package world

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestShutdownDrainsActiveCharacterAndAlreadyClaimedDisconnect(t *testing.T) {
	for _, disconnectFirst := range []bool{false, true} {
		name := "active"
		if disconnectFirst {
			name = "disconnect-already-claimed"
		}
		t.Run(name, func(t *testing.T) {
			database, wh, fixture, _ := battleTestWorld(t)
			wh.sessionManager = session.NewSessionManager()
			ses := wh.sessionManager.CreateNextSession(&recordingMessenger{}, "", nil)
			ses.Client = fixture.Client
			wh.ActorRegistry = NewActorRegistry()
			wh.ActorManager = NewPhaserActorManager(wh)
			wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
			wh.TrainerEncounter = NewTrainerEncounterManager(wh)
			wh.WildEncounter = NewWildEncounterManager(wh, wh.database)
			if err := wh.characterOwners.acquire(context.Background(), 42, ses, nil); err != nil {
				t.Fatal(err)
			}
			ses.StartPlaytime(time.Now().Add(-3*time.Second), 0, 42)
			entered, release, shutdownDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }); wh.Shutdown() })
			go func() { _ = ses.ExecuteCommand(context.Background(), func() { close(entered); <-release }) }()
			<-entered
			if disconnectFirst {
				go wh.RemoveSession(ses.SessionID)
			}
			if disconnectFirst {
				deadline := time.Now().Add(time.Second)
				for {
					if _, ok := wh.sessionManager.GetSession(ses.SessionID); !ok {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("disconnect did not claim session")
					}
					runtime.Gosched()
				}
			}
			go func() { wh.Shutdown(); close(shutdownDone) }()
			deadline := time.Now().Add(time.Second)
			for !ses.IsClosed() {
				if time.Now().After(deadline) {
					t.Fatal("shutdown did not close session")
				}
				runtime.Gosched()
			}
			select {
			case <-shutdownDone:
				t.Fatal("shutdown returned before command completed")
			default:
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			if err := wh.ShutdownContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("blocked command shutdown=%v", err)
			}
			if err := database.Ping(); err != nil {
				t.Fatal("storage unavailable during unfinished drain")
			}
			releaseOnce.Do(func() { close(release) })
			<-shutdownDone
			if err := wh.ShutdownContext(context.Background()); err != nil {
				t.Fatal(err)
			}
			if ses.HasValidClient() || wh.characterOwners.owns(42, ses) {
				t.Fatal("character survived shutdown cleanup")
			}
			if wh.sessionManager.CreateNextSession(nil, "", nil) != nil {
				t.Fatal("shutdown admitted connection")
			}
			var seconds int
			if err := database.QueryRow(`SELECT time_played FROM character_data WHERE id=42`).Scan(&seconds); err != nil {
				t.Fatal(err)
			}
			if seconds < 3 {
				t.Fatalf("playtime was not flushed: %d", seconds)
			}
		})
	}
}

func TestShutdownReturnsFinalPersistenceFailure(t *testing.T) {
	database, wh, fixture, _ := battleTestWorld(t)
	wh.sessionManager = session.NewSessionManager()
	ses := wh.sessionManager.CreateNextSession(&recordingMessenger{}, "", nil)
	ses.Client = fixture.Client
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	if err := wh.characterOwners.acquire(context.Background(), 42, ses, nil); err != nil {
		t.Fatal(err)
	}
	ses.StartPlaytime(time.Now().Add(-3*time.Second), 0, 42)
	testdb.Exec(t, database, `CREATE FUNCTION reject_shutdown_save() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject shutdown save'; END $$;
 CREATE CONSTRAINT TRIGGER reject_shutdown_save AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_shutdown_save();`)
	for i := 0; i < 2; i++ {
		if err := wh.ShutdownContext(context.Background()); err == nil || !strings.Contains(err.Error(), "final playtime") {
			t.Fatalf("shutdown persistence failure=%v", err)
		}
	}
	if ses.HasValidClient() || wh.characterOwners.owns(42, ses) {
		t.Fatal("failed final save left runtime alive")
	}
}
