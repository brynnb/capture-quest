package world

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

// Entries are committed snapshots. Gameplay must clone through CommitBattle,
// never mutate a pointer returned by getBattle. Character command ownership will
// also serialize publication and connection replacement.
var (
	activeBattles   = make(map[int64]*pokebattle.BattleState)
	activeBattlesMu sync.RWMutex
)

func getBattle(charID int64) *pokebattle.BattleState {
	activeBattlesMu.RLock()
	defer activeBattlesMu.RUnlock()
	return activeBattles[charID]
}
func setBattle(charID int64, b *pokebattle.BattleState) {
	activeBattlesMu.Lock()
	defer activeBattlesMu.Unlock()
	activeBattles[charID] = b
}
func forgetBattle(charID int64, expected *pokebattle.BattleState) {
	activeBattlesMu.Lock()
	defer activeBattlesMu.Unlock()
	if activeBattles[charID] == expected {
		delete(activeBattles, charID)
	}
}

func startBattle(ctx context.Context, database *sql.DB, charID int64, battle *pokebattle.BattleState) (*pokebattle.BattleState, error) {
	committed, err := pokebattle.StartBattle(ctx, database, charID, battle, func(tx db.DBTX, next *pokebattle.BattleState) error {
		return markPokemonSeen(tx, charID, next.GetEnemyPokemon().ID)
	})
	if err != nil {
		return nil, err
	}
	setBattle(charID, committed)
	return committed, nil
}

// Explicit server-side reset (debug fixtures/home warp). Client battle dismissal
// uses CloseBattle instead, which requires a completed battle and no pending move.
func removeBattle(charID int64) {
	battle := getBattle(charID)
	err := db.Transaction(context.Background(), db.GlobalWorldDB.DB, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		return pokebattle.DeleteBattleState(tx, charID)
	})
	if err != nil {
		log.Printf("[PokeBattle] Clear failed for character %d: %v", charID, err)
		return
	}
	forgetBattle(charID, battle)
}

// Every start/turn/choice is already durable before publication. Disconnect must
// not rewrite an old connection's snapshot over a newer committed battle.
func saveBattleOnDisconnect(charID int64) {
	forgetBattle(charID, getBattle(charID))
}

func restoreBattleOnLogin(ctx context.Context, database *sql.DB, charID int64) (*pokebattle.BattleState, error) {
	battle, err := pokebattle.ResumeBattle(ctx, database, charID)
	if err != nil || battle == nil {
		return nil, err
	}
	configureBattleObedience(battle, charID, nil)
	setBattle(charID, battle)
	return battle, nil
}

var errBattleOwnership = errors.New("character is owned by a battle")

// A persisted battle retains gameplay ownership until dismissal, including its
// terminal and pending-choice states. Cache absence is not an admission grant.
// Call under the character lock inside the caller's transaction.
func requireNoOwnedBattleIn(tx db.DBTX, charID int64) error {
	battle, err := pokebattle.LoadBattleState(tx, charID)
	if err != nil {
		return err
	}
	if battle != nil {
		return fmt.Errorf("%w: finish the current battle first", errBattleOwnership)
	}
	visit, err := safariSessionIn(tx, charID)
	if err != nil {
		return err
	}
	if visit != nil && visit.Battle != nil {
		return fmt.Errorf("%w: finish the safari encounter first", errBattleOwnership)
	}
	return nil
}
