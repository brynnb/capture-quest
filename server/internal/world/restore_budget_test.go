package world

import (
	"capturequest/internal/db"
	"context"
	"strings"
	"testing"
	"time"
)

func TestBattleRestoreUsesRemainingSessionBudget(t *testing.T) {
	for _, stage := range []string{"ownership", "pool-admission"} {
		t.Run(stage, func(t *testing.T) {
			database, _, ses, _ := battleTestWorld(t)
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			var restoreErr error
			var budget time.Duration
			before := database.Stats().WaitCount
			err := ses.ExecuteCommand(ctx, func() {
				// Model actual preceding entry work on the same command context. Restore
				// must inherit what remains, rather than starting another five-second owner.
				var ignored any
				if err := database.QueryRowContext(ses.CommandContext(), `SELECT pg_sleep(0.05)`).Scan(&ignored); err != nil {
					t.Fatal(err)
				}
				var release func()
				if stage == "ownership" {
					holder, err := database.Begin()
					if err != nil {
						t.Fatal(err)
					}
					if err := db.LockCharacter(holder, 42); err != nil {
						holder.Rollback()
						t.Fatal(err)
					}
					release = func() { holder.Rollback() }
				} else {
					database.SetMaxOpenConns(1)
					holder, err := database.Conn(ses.CommandContext())
					if err != nil {
						t.Fatal(err)
					}
					release = func() { holder.Close() }
				}
				defer release()
				// Bound even a regressed implementation that mistakenly drops its parent.
				guard := time.AfterFunc(time.Second, release)
				defer guard.Stop()
				deadline, ok := ses.CommandContext().Deadline()
				if !ok {
					t.Fatal("session lost inherited deadline")
				}
				budget = time.Until(deadline)
				_, restoreErr = restoreBattleOnLogin(ses.CommandContext(), database, 42)
			})
			if ctx.Err() != context.DeadlineExceeded || restoreErr == nil {
				t.Fatalf("restore escaped inherited budget: command=%v restore=%v budget=%s", err, restoreErr, budget)
			}
			label := "resume character ownership"
			if stage == "pool-admission" {
				label = "begin transaction"
			}
			if !strings.Contains(restoreErr.Error(), label) {
				t.Fatalf("expected %s failure, got %v", label, restoreErr)
			}
			if stage == "pool-admission" && database.Stats().WaitCount <= before {
				t.Fatal("restore did not wait for the occupied pool")
			}
			if getBattle(42) != nil {
				t.Fatal("failed restore published a battle")
			}
			t.Logf("stage=%s restore_deadline_remaining_at_start=%s command_error=%v pool_waits=%d", stage, budget, ctx.Err(), database.Stats().WaitCount-before)
		})
	}
}
