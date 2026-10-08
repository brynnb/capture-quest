package world

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/testdb"
)

func TestVermilionTrashRollbackAndRetry(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	first, second := 0, 1
	picker := FixedVermilionGymTrashPicker{&first, &second}
	// The flag insert succeeds first; reject the later state write.
	testdb.Exec(t, database, `ALTER TABLE character_vermilion_gym_trash_state ADD CONSTRAINT reject_second CHECK(second_lock_can_index IS NULL)`)
	outcome, err := handleVermilionGymTrashCan(context.Background(), database, 42, 0, wh.EventFlags, picker)
	if err == nil || outcome != nil {
		t.Fatalf("failure published outcome: %+v, %v", outcome, err)
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_vermilion_gym_trash_state`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("state escaped rollback: %d %v", count, err)
	}
	flag, err := queryEventFlag(database, 42, EventVermilionGymFirstLockOpened)
	if err != nil || flag || wh.EventFlags.CheckFlag(42, EventVermilionGymFirstLockOpened) {
		t.Fatalf("flag escaped rollback: %v %v", flag, err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_vermilion_gym_trash_state DROP CONSTRAINT reject_second`)
	outcome, err = handleVermilionGymTrashCan(context.Background(), database, 42, 0, wh.EventFlags, picker)
	if err != nil || !outcome.OpenedFirst || !wh.EventFlags.CheckFlag(42, EventVermilionGymFirstLockOpened) {
		t.Fatalf("retry: %+v %v", outcome, err)
	}
	// An unloaded cache must not send a durable first-open state down the wrong branch.
	wh.EventFlags.UnloadFlags(42)
	outcome, err = handleVermilionGymTrashCan(context.Background(), database, 42, 1, wh.EventFlags, picker)
	if err != nil || !outcome.OpenedSecond || !wh.EventFlags.CheckFlag(42, EventVermilionGymSecondLockOpened) {
		t.Fatalf("durable second lock: %+v %v", outcome, err)
	}
}

func TestVermilionTrashResetRollback(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	first, second := 0, 1
	picker := FixedVermilionGymTrashPicker{&first, &second}
	if _, err := handleVermilionGymTrashCan(context.Background(), database, 42, 0, wh.EventFlags, picker); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `ALTER TABLE character_vermilion_gym_trash_state ADD CONSTRAINT reject_reset CHECK(second_lock_can_index IS NOT NULL)`)
	outcome, err := handleVermilionGymTrashCan(context.Background(), database, 42, 14, wh.EventFlags, picker)
	if err == nil || outcome != nil {
		t.Fatalf("reset failure: %+v %v", outcome, err)
	}
	flag, err := queryEventFlag(database, 42, EventVermilionGymFirstLockOpened)
	state, loadErr := ReadVermilionGymTrashState(database, 42)
	if err != nil || !flag || loadErr != nil || state.SecondLockCanIndex == nil || *state.SecondLockCanIndex != 1 || !wh.EventFlags.CheckFlag(42, EventVermilionGymFirstLockOpened) {
		t.Fatalf("reset leaked: %+v %v %v %v", state, flag, err, loadErr)
	}
	testdb.Exec(t, database, `ALTER TABLE character_vermilion_gym_trash_state DROP CONSTRAINT reject_reset`)
	outcome, err = handleVermilionGymTrashCan(context.Background(), database, 42, 14, wh.EventFlags, picker)
	if err != nil || !outcome.ResetLocks || wh.EventFlags.CheckFlag(42, EventVermilionGymFirstLockOpened) {
		t.Fatalf("reset retry: %+v %v", outcome, err)
	}
}

func TestVermilionTrashConcurrentInitializationAndCancellation(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	first, second := 0, 1
	picker := FixedVermilionGymTrashPicker{&first, &second}
	var wg sync.WaitGroup
	errors := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outcome, err := handleVermilionGymTrashCan(context.Background(), database, 42, 14, wh.EventFlags, picker)
			if err == nil && (outcome.Changed || outcome.State.FirstLockCanIndex != 0) {
				err = fmt.Errorf("unexpected outcome %+v", outcome)
			}
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM character_vermilion_gym_trash_state`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("initialization: %d %v", count, err)
	}
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := db.LockCharacter(tx, 42); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	outcome, err := handleVermilionGymTrashCan(ctx, database, 42, 0, wh.EventFlags, picker)
	if err == nil || outcome != nil {
		t.Fatalf("cancelled lock wait: %+v %v", outcome, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	outcome, err = handleVermilionGymTrashCan(context.Background(), database, 42, 0, wh.EventFlags, picker)
	if err != nil || !outcome.OpenedFirst {
		t.Fatalf("retry after cancellation: %+v %v", outcome, err)
	}
}

func TestVermilionTrashCommitFailureDoesNotPublish(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	first, second := 0, 1
	picker := FixedVermilionGymTrashPicker{&first, &second}
	testdb.Exec(t, database, `
 CREATE FUNCTION reject_trash_commit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'reject puzzle commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_trash_commit AFTER INSERT OR UPDATE
 ON character_vermilion_gym_trash_state DEFERRABLE INITIALLY DEFERRED
 FOR EACH ROW EXECUTE FUNCTION reject_trash_commit();`)
	outcome, err := handleVermilionGymTrashCan(context.Background(), database, 42, 0, wh.EventFlags, picker)
	if err == nil || outcome != nil {
		t.Fatalf("commit failure published: %+v %v", outcome, err)
	}
	state, err := ReadVermilionGymTrashState(database, 42)
	flag, flagErr := queryEventFlag(database, 42, EventVermilionGymFirstLockOpened)
	if err != nil || state != nil || flagErr != nil || flag || wh.EventFlags.CheckFlag(42, EventVermilionGymFirstLockOpened) {
		t.Fatalf("commit failure escaped: %+v %v %v %v", state, flag, err, flagErr)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_trash_commit ON character_vermilion_gym_trash_state`)
	outcome, err = handleVermilionGymTrashCan(context.Background(), database, 42, 0, wh.EventFlags, picker)
	if err != nil || !outcome.OpenedFirst {
		t.Fatalf("commit retry: %+v %v", outcome, err)
	}
}

func TestVermilionTrashConcurrentTransitions(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	first, second := 0, 1
	picker := FixedVermilionGymTrashPicker{&first, &second}
	outcomes := make(chan *VermilionGymTrashOutcome, 4)
	errors := make(chan error, 4)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			outcome, err := handleVermilionGymTrashCan(context.Background(), database, 42, 0, wh.EventFlags, picker)
			outcomes <- outcome
			errors <- err
		}()
	}
	wg.Wait()
	close(outcomes)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	opened, reset := 0, 0
	for outcome := range outcomes {
		if outcome.OpenedFirst {
			opened++
		}
		if outcome.ResetLocks {
			reset++
		}
	}
	// Repeated clicks are separate gameplay actions: first-can clicks alternate
	// opening and resetting, even when callers all begin with an empty cache.
	state, err := ReadVermilionGymTrashState(database, 42)
	flag, flagErr := queryEventFlag(database, 42, EventVermilionGymFirstLockOpened)
	if opened != 2 || reset != 2 || err != nil || state.SecondLockCanIndex != nil || flagErr != nil || flag || wh.EventFlags.CheckFlag(42, EventVermilionGymFirstLockOpened) {
		t.Fatalf("transitions opened=%d reset=%d state=%+v flag=%v errors=%v,%v", opened, reset, state, flag, err, flagErr)
	}
}
