package world

import (
	"capturequest/internal/testdb"
	"context"
	"errors"
	"testing"
	"time"
)

func TestFlagReadPoolWaitDoesNotHoldCacheLockAndUnloadSupersedesIt(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'flags'); INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'STORED')`)
	manager := NewEventFlagManager(database)
	manager.publishCommittedFlags(7, map[string]bool{"OTHER": true})
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	finished := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() { finished <- manager.LoadFlagsContext(ctx, 42) }()
	deadline := time.Now().Add(time.Second)
	for database.Stats().WaitCount == before {
		if time.Now().After(deadline) {
			t.Fatal("flag query did not wait on pool")
		}
		time.Sleep(time.Millisecond)
	}
	cached := make(chan bool, 1)
	go func() { cached <- manager.CheckFlag(7, "OTHER"); manager.UnloadFlags(42) }()
	select {
	case present := <-cached:
		if !present {
			t.Fatal("other character cache unavailable")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("database wait held shared cache lock")
	}
	// Ensure retirement has completed before releasing the blocked read.
	manager.UnloadFlags(42)
	lease.Close()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if manager.CheckFlag(42, "STORED") {
		t.Fatal("old read resurrected unloaded cache")
	}
	manager.mu.RLock()
	pending := len(manager.readTokens)
	manager.mu.RUnlock()
	if pending != 0 {
		t.Fatal("retained completed load tokens")
	}
}

func TestFlagLoadDeadlineAndCommittedPublicationOwnCacheView(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'flags'); INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'STORED')`)
	manager := NewEventFlagManager(database)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := manager.LoadFlagsContext(ctx, 42); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("flag load escaped caller deadline: %v", err)
	}
	snapshot := map[string]bool{"COMMITTED": true}
	manager.publishCommittedFlags(42, snapshot)
	snapshot["LATER_CALLER_MUTATION"] = true
	if !manager.CheckFlag(42, "COMMITTED") || manager.CheckFlag(42, "LATER_CALLER_MUTATION") {
		t.Fatal("cache snapshot shares caller mutation")
	}
	manager.mu.RLock()
	pending := len(manager.readTokens)
	manager.mu.RUnlock()
	if pending != 0 {
		t.Fatal("failed read retained token")
	}
}

func TestCommittedFlagRefreshUsesOwnerDeadline(t *testing.T) {
	for _, publisher := range []string{"safari", "cutscene"} {
		t.Run(publisher, func(t *testing.T) {
			database, wh, ses, _ := battleTestWorld(t)
			database.SetMaxOpenConns(1)
			lease, err := database.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			before := database.Stats().WaitCount
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			defer cancel()
			finished := make(chan error, 1)
			go func() {
				finished <- ses.ExecuteCommand(ctx, func() {
					if publisher == "safari" {
						refreshSafariFlags(ses, wh, 42)
					} else {
						mutation := &cutsceneMutation{characterID: 42, flagsChanged: true}
						mutation.publish(ses.CommandContext(), CutsceneActionContext{Session: ses, EventFlags: wh.EventFlags})
					}
				})
			}()
			select {
			case err := <-finished:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("owner deadline: %v", err)
				}
			case <-time.After(300 * time.Millisecond):
				// Release our exact lease before reporting failure so the owner drains.
				lease.Close()
				<-finished
				t.Fatal("committed refresh outlived owner deadline")
			}
			if database.Stats().WaitCount == before {
				t.Fatal("refresh did not exercise database pool contention")
			}
		})
	}
}

func TestFlagWriterCancellationWhileCharacterLockedDoesNotCommit(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'flags')`)
	lock, err := database.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if _, err := lock.Exec(`UPDATE character_data SET id=id WHERE id=42`); err != nil {
		t.Fatal(err)
	}
	manager := NewEventFlagManager(database)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- manager.SetFlagBatch(ctx, 42, []string{"ONE", "TWO"}) }()
	select {
	case err := <-finished:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("blocked writer escaped cancellation: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		lock.Rollback()
		<-finished
		t.Fatal("flag mutation outlived caller deadline")
	}
	lock.Rollback()
	var count int
	if err := database.QueryRow(`SELECT count(*) FROM character_event_flags WHERE character_id=42`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cancelled mutation persisted %d flags: %v", count, err)
	}
	if len(manager.GetAllFlags(42)) != 0 {
		t.Fatal("cancelled mutation published cache")
	}
}

func TestFlagCommitPublishesSnapshotAfterCallerCancellation(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO character_data(id,name) VALUES(42,'flags'); INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'EXISTING')`)
	manager := NewEventFlagManager(database)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Hold only the cache publication lock. The database transaction must still
	// commit, then wait here; cancellation at that point cannot reject the write.
	manager.mu.Lock()
	locked := true
	defer func() {
		if locked {
			manager.mu.Unlock()
		}
	}()
	finished := make(chan error, 1)
	go func() { finished <- manager.SetFlag(ctx, 42, "COMMITTED") }()
	deadline := time.Now().Add(time.Second)
	for {
		var committed bool
		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM character_event_flags WHERE character_id=42 AND flag_name='COMMITTED')`).Scan(&committed); err != nil {
			t.Fatal(err)
		}
		if committed {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("writer did not commit while cache publication was blocked")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	manager.mu.Unlock()
	locked = false
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("committed flag reported rejection after cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("committed writer did not finish publication")
	}
	if !manager.CheckFlag(42, "EXISTING") || !manager.CheckFlag(42, "COMMITTED") {
		t.Fatal("committed complete snapshot was not published")
	}
}
