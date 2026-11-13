package scriptsim

import (
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
	"context"
	"errors"
	"testing"
	"time"
)

func TestSnapshotUsesInjectedCoherentReadAndReturnsNoPartialFailure(t *testing.T) {
	database := testdb.Postgres(t)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(50,'ROOM',10,10); INSERT INTO character_data(id,name,map_id,x,y) VALUES(42,'snapshot',50,2,3); INSERT INTO character_wallet(character_id,pokedollars) VALUES(42,100); INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'BEFORE'); INSERT INTO character_coins(character_id,coins) VALUES(42,10)`)
	battle := &pokebattle.BattleState{BattleType: pokebattle.BattleWild, WildWinFlag: "DURABLE_BATTLE"}
	if err := pokebattle.SaveBattleState(database, 42, battle); err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	view, err := CaptureSnapshot(context.Background(), database, 42, "ROOM")
	if err != nil || view == nil || view.Money != 100 || view.ActiveBattle == nil || view.ActiveBattle.WinFlag != "DURABLE_BATTLE" {
		t.Fatalf("injected snapshot without global/cache: %+v %v", view, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_vermilion_gym_trash_state RENAME TO unavailable_trash`)
	view, err = CaptureSnapshot(context.Background(), database, 42, "ROOM")
	if err == nil || view != nil {
		t.Fatalf("late failure returned partial aggregate: %+v %v", view, err)
	}
}

func TestSnapshotHonorsPoolDeadline(t *testing.T) {
	database := testdb.Postgres(t)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	view, err := CaptureSnapshot(ctx, database, 42, "ROOM")
	if !errors.Is(err, context.DeadlineExceeded) || view != nil || database.Stats().WaitCount <= before {
		t.Fatalf("snapshot escaped caller pool deadline: %+v %v", view, err)
	}
}

func TestSnapshotCannotMixConcurrentCommitAcrossTables(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(50,'ROOM',10,10); INSERT INTO character_data(id,name,map_id) VALUES(42,'before',50); INSERT INTO character_wallet(character_id,pokedollars) VALUES(42,100); INSERT INTO character_coins(character_id,coins) VALUES(42,10); INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'BEFORE')`)
	holder, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	if _, err = holder.Exec(`LOCK TABLE character_coins IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	type result struct {
		view *Snapshot
		err  error
	}
	done := make(chan result, 1)
	go func() { view, err := CaptureSnapshot(ctx, database, 42, "ROOM"); done <- result{view, err} }()
	deadline := time.Now().Add(time.Second)
	for {
		var waiting bool
		if err = database.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='character_coins'::regclass AND NOT granted)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("snapshot did not reach held coins read")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, err = holder.Exec(`UPDATE character_data SET name='after' WHERE id=42; UPDATE character_wallet SET pokedollars=200 WHERE character_id=42; UPDATE character_coins SET coins=20 WHERE character_id=42; UPDATE character_event_flags SET flag_name='AFTER' WHERE character_id=42`); err != nil {
		t.Fatal(err)
	}
	if err = holder.Commit(); err != nil {
		t.Fatal(err)
	}
	captured := <-done
	if captured.err != nil || captured.view == nil || captured.view.CharacterName != "before" || captured.view.Money != 100 || captured.view.Coins != 10 || len(captured.view.Flags) != 1 || captured.view.Flags[0] != "BEFORE" {
		t.Fatalf("snapshot mixed commit: %+v %v", captured.view, captured.err)
	}
	fresh, err := CaptureSnapshot(context.Background(), database, 42, "ROOM")
	if err != nil || fresh.CharacterName != "after" || fresh.Money != 200 || fresh.Coins != 20 || fresh.Flags[0] != "AFTER" {
		t.Fatalf("fresh snapshot missed commit: %+v %v", fresh, err)
	}
}
