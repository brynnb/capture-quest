package world

import (
	"context"
	"errors"
	"testing"
	"time"

	"capturequest/internal/session"
)

func TestAllWorldPreloadsRespectCancelledContext(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	wh.ActorManager = NewPhaserActorManager(wh)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loaders := map[string]func(context.Context) error{
		"actors":      wh.ActorManager.Load,
		"trainers":    NewTrainerEncounterManager(wh).Load,
		"wild":        NewWildEncounterManager(wh).Load,
		"coordinates": NewCoordinateTriggerManager(database).Load,
		"map scripts": NewMapScriptManager(database).Load,
		"spin tiles":  NewSpinTileManager(database).Load,
		"warp tiles":  NewWarpTileManager(database).Load,
		"map warps":   newPhaserWarpManager(database).load,
		"cutscenes":   NewCutsceneManager(database).Load,
	}
	for name, load := range loaders {
		t.Run(name, func(t *testing.T) {
			if err := load(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled preload=%v", err)
			}
		})
	}
	if world, err := NewWorldHandler(ctx, session.NewSessionManager()); world != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled construction=%v", err)
	}
}

func TestPreloadDeadlineInterruptsDatabaseLockAndCanRetry(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	m := NewCutsceneManager(database)
	if err := m.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	tx, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`LOCK TABLE phaser_maps IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := m.Load(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("locked preload=%v", err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("preload did not promptly observe deadline")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := m.Load(context.Background()); err != nil {
		t.Fatalf("retry after cancellation: %v", err)
	}
}
