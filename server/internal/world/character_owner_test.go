package world

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"capturequest/internal/api/opcodes"
	model "capturequest/internal/db/models"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestCharacterHandoffDrainsCommandsBeforeReplacement(t *testing.T) {
	var owners characterOwners
	old, next := &session.Session{}, &session.Session{}
	if err := owners.acquire(context.Background(), 42, old, nil); err != nil {
		t.Fatal(err)
	}
	entered, finish := make(chan struct{}), make(chan struct{})
	value := 0
	go func() { _ = old.ExecuteCommand(context.Background(), func() { close(entered); <-finish; value = 7 }) }()
	<-entered
	done := make(chan error, 1)
	go func() {
		done <- owners.acquire(context.Background(), 42, next, func(ctx context.Context, s *session.Session) error {
			if value != 7 {
				t.Error("cleanup ran before command completed")
			}
			owners.release(42, s)
			return nil
		})
	}()
	deadline := time.Now().Add(time.Second)
	for !old.IsClosed() {
		if time.Now().After(deadline) {
			t.Fatal("old connection not closed")
		}
		runtime.Gosched()
	}
	if owners.owns(42, next) {
		t.Fatal("replacement admitted before cleanup")
	}
	if err := owners.acquire(context.Background(), 42, &session.Session{}, nil); !errors.Is(err, errCharacterHandoff) {
		t.Fatalf("concurrent handoff: %v", err)
	}
	close(finish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !owners.owns(42, next) {
		t.Fatal("replacement missing")
	}
	owners.release(42, old)
	if !owners.owns(42, next) {
		t.Fatal("late release removed replacement")
	}
	owners.release(42, next)
	if len(owners.entries) != 0 {
		t.Fatal("retained inactive character")
	}
}

func TestCharacterHandoffCancellationRetainsOldOwnerUntilCleanup(t *testing.T) {
	var owners characterOwners
	old, next := &session.Session{}, &session.Session{}
	if err := owners.acquire(context.Background(), 42, old, nil); err != nil {
		t.Fatal(err)
	}
	entered, finish, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		_ = old.ExecuteCommand(context.Background(), func() { close(entered); <-finish })
	}()
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := owners.acquire(ctx, 42, next, func(context.Context, *session.Session) error { t.Error("cancelled cleanup ran"); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	if !owners.owns(42, old) || owners.owns(42, next) {
		t.Fatal("cancelled handoff changed owner")
	}
	close(finish)
	<-stopped
	old.DrainCommands(func() { owners.release(42, old) })
	if err := owners.acquire(context.Background(), 42, next, nil); err != nil {
		t.Fatal(err)
	}
}

func TestLateCharacterCleanupCannotEvictReplacementState(t *testing.T) {
	wh := &WorldHandler{}
	next := &session.Session{}
	if err := wh.characterOwners.acquire(context.Background(), 42, next, nil); err != nil {
		t.Fatal(err)
	}
	old := &session.Session{Client: &testSessionClient{char: &model.CharacterData{ID: 42}}, CharacterName: "old", MapID: 1}
	// Managers are deliberately absent: a stale cleanup must not touch any of
	// the global character caches, position writers, actors or battle registry.
	wh.cleanupCharacterSession(context.Background(), old)
	if old.Client != nil || old.CharacterName != "" || old.MapID != -1 {
		t.Fatal("stale local client not retired")
	}
	if !wh.characterOwners.owns(42, next) {
		t.Fatal("replacement lost ownership")
	}
}

func TestEnterWorldHandoffReloadsAfterOldCommandCommits(t *testing.T) {
	database, wh, old, _ := battleTestWorld(t)
	testdb.Exec(t, database, `UPDATE character_data SET x=10,y=10,map_id=1 WHERE id=42`)
	wh.sessionManager = session.NewSessionManager()
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	wh.WildEncounter = NewWildEncounterManager(wh, wh.database)
	if err := wh.characterOwners.acquire(context.Background(), 42, old, nil); err != nil {
		t.Fatal(err)
	}
	entered, finish := make(chan struct{}), make(chan struct{})
	go func() {
		_ = old.ExecuteCommand(context.Background(), func() {
			close(entered)
			<-finish
			if _, err := database.Exec(`UPDATE character_data SET x=21 WHERE id=42`); err != nil {
				t.Error(err)
			}
		})
	}()
	<-entered
	messages := &recordingMessenger{}
	next := &session.Session{Authenticated: true, Messenger: messages}
	done := make(chan struct{})
	go func() { defer close(done); battleDispatch(t, wh, next, opcodes.EnterWorld, `{"name":"battle"}`) }()
	deadline := time.Now().Add(time.Second)
	for !old.IsClosed() {
		if time.Now().After(deadline) {
			close(finish)
			<-done
			t.Fatal("old session not closed")
		}
		runtime.Gosched()
	}
	close(finish)
	<-done
	if !next.HasValidClient() || next.Client.CharData().X != 21 {
		t.Fatal("entry did not reload committed old-session position")
	}
	if x, y, mapID, ok := wh.PlayerMovement.GetPosition(42); !ok || x != 21 || y != 10 || mapID != 1 {
		t.Fatal("entry did not register the reloaded owned movement source")
	}
	if old.HasValidClient() || !wh.characterOwners.owns(42, next) {
		t.Fatal("handoff did not retire previous owner")
	}
	// The transport's eventual disconnect callback cannot clear the new owner.
	old.DrainCommands(func() { wh.cleanupCharacterSession(context.Background(), old) })
	if !wh.characterOwners.owns(42, next) {
		t.Fatal("late disconnect removed new owner")
	}
	next.Close()
	next.DrainCommands(func() { wh.cleanupCharacterSession(context.Background(), next) })
}

func TestEntryInvalidOptionsClosesPartialOwnerWithoutPresence(t *testing.T) {
	database, wh, _, messages := battleTestWorld(t)
	testdb.Exec(t, database, `UPDATE character_data SET x=7,y=8,map_id=50,options='[]' WHERE id=42`)
	ses := &session.Session{Authenticated: true, Messenger: messages}
	battleDispatch(t, wh, ses, opcodes.EnterWorld, `{"name":"battle"}`)
	if !ses.IsClosed() || ses.HasValidClient() || ses.Presence().CharacterID != 0 || wh.characterOwners.owns(42, ses) {
		t.Fatal("invalid option read left partial visible character owner")
	}
}
