package pokebattle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"capturequest/internal/db"
	"github.com/google/uuid"
)

var ErrBattleConflict = errors.New("battle changed; reload its current state")

// StartBattle reserves the character's battle before the encounter is published.
// Reload under the character lock so preparation cannot overwrite a newer party.
func StartBattle(ctx context.Context, database *sql.DB, charID int64, battle *BattleState, prepare func(DBTX, *BattleState) error) (*BattleState, error) {
	var next *BattleState
	err := db.Transaction(ctx, database, func(tx db.DBTX) (err error) {
		next, err = StartBattleInTransaction(tx, charID, battle, prepare)
		return err
	})
	if err != nil {
		return nil, err
	}
	return next, nil
}

// StartBattleInTransaction joins a caller's reward/script transaction. The
// returned state remains private until the caller commits.
func StartBattleInTransaction(tx DBTX, charID int64, battle *BattleState, prepare func(DBTX, *BattleState) error) (*BattleState, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return nil, err
	}
	if battle == nil {
		return nil, fmt.Errorf("battle is required")
	}
	next := battle.Clone()
	if len(next.EnemyParty) == 0 || next.EnemyActive < 0 || next.EnemyActive >= len(next.EnemyParty) {
		return nil, fmt.Errorf("battle has no active enemy")
	}
	for _, enemy := range next.EnemyParty {
		if enemy == nil {
			return nil, fmt.Errorf("battle has nil enemy")
		}
	}

	err := func() error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		stored, err := LoadBattleState(tx, charID)
		if err != nil {
			return err
		}
		if stored != nil && (!stored.IsOver() || stored.PendingMoveLearn != nil) {
			return ErrBattleConflict
		}
		next.PlayerParty, err = LoadParty(tx, charID)
		if err != nil {
			return err
		}
		next.PlayerActive = FirstAlivePartyIndex(next.PlayerParty)
		if len(next.PlayerParty) == 0 || next.GetPlayerPokemon() == nil || next.GetPlayerPokemon().IsFainted() {
			return fmt.Errorf("player party has no battle-ready pokemon")
		}
		if prepare != nil {
			if err := prepare(tx, next); err != nil {
				return err
			}
		}
		next.BattleID = uuid.NewString()
		next.Revision = 1
		return SaveBattleState(tx, charID, next)
	}()
	if err != nil {
		return nil, err
	}
	return next, nil
}

// CommitBattle applies a turn to a private copy and commits every durable effect
// with the party and resumable battle. The caller publishes only the returned
// state. Stale copies cannot spend an item or award a reward a second time.
func CommitBattle(ctx context.Context, database *sql.DB, charID int64, current *BattleState, apply func(DBTX, *BattleState) error) (*BattleState, error) {
	if current == nil {
		return nil, fmt.Errorf("battle is required")
	}
	next := current.Clone()
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		stored, err := LoadBattleState(tx, charID)
		if err != nil {
			return err
		}
		if stored == nil || stored.BattleID != current.BattleID || stored.Revision != current.Revision {
			return ErrBattleConflict
		}
		if apply != nil {
			if err := apply(tx, next); err != nil {
				return err
			}
		}
		next.PlayerParty, err = SavePartyInTransaction(tx, charID, next.PlayerParty)
		if err != nil {
			return err
		}
		if next.BattleID == "" {
			next.BattleID = uuid.NewString()
		}
		next.Revision++
		return SaveBattleState(tx, charID, next)
	})
	if err != nil {
		return nil, err
	}
	return next, nil
}

// ResumeBattle reads a coherent party/battle pair and keeps the durable record.
func ResumeBattle(ctx context.Context, database *sql.DB, charID int64) (*BattleState, error) {
	var battle *BattleState
	err := db.Transaction(ctx, database, func(tx db.DBTX) (err error) {
		if err := db.LockCharacter(tx, charID); err != nil {
			return fmt.Errorf("resume character ownership: %w", err)
		}
		battle, err = LoadBattleState(tx, charID)
		if err != nil {
			return fmt.Errorf("resume saved state: %w", err)
		}
		if battle == nil {
			return nil
		}
		if err := battle.RestoreParty(tx, charID); err != nil {
			return fmt.Errorf("resume party: %w", err)
		}
		// Version zero predates durable command identity. Upgrade exactly that
		// supported format before advertising a playable battle, under the same lock.
		// No turn, party save or reward occurs, and failed commit publishes no identity.
		if battle.persistedVersion == 0 {
			battle.BattleID = uuid.NewString()
			battle.Revision = 1
			if err := SaveBattleState(tx, charID, battle); err != nil {
				return fmt.Errorf("resume legacy identity: %w", err)
			}
			battle.persistedVersion = 2
			battle.playerVolatile = make([]playerVolatileState, 0, len(battle.PlayerParty))
			for _, p := range battle.PlayerParty {
				battle.playerVolatile = append(battle.playerVolatile, volatileState(p))
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return battle, nil
}

// CloseBattle cannot discard an active turn, pending choice, or newer battle.
func CloseBattle(ctx context.Context, database *sql.DB, charID int64, current *BattleState) error {
	if current == nil || !current.IsOver() || current.PendingMoveLearn != nil {
		return ErrBattleConflict
	}
	return db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		return CloseBattleIn(tx, charID, current)
	})
}

// CloseBattleIn joins dismissal to a caller-owned character transaction, so
// post-battle plan issuance cannot fail after the resumable battle is deleted.
// Caller must hold the character lock; durable identity and terminal eligibility
// are rechecked here, independently of the transport's cached battle.
func CloseBattleIn(tx db.DBTX, charID int64, current *BattleState) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	if current == nil || !current.IsOver() || current.PendingMoveLearn != nil {
		return ErrBattleConflict
	}
	stored, err := LoadBattleState(tx, charID)
	if err != nil {
		return err
	}
	if stored == nil {
		return nil
	}
	if stored.BattleID != current.BattleID || stored.Revision != current.Revision || !stored.IsOver() || stored.PendingMoveLearn != nil {
		return ErrBattleConflict
	}
	return DeleteBattleState(tx, charID)
}

// Clone separates every mutable battle object, including parties, action lists,
// pending choices and event slices. Pokemon currently contains only value fields.
func (b *BattleState) Clone() *BattleState {
	if b == nil {
		return nil
	}
	next := *b
	if b.Capture != nil {
		placement := *b.Capture
		next.Capture = &placement
	}
	pokemon := make(map[*Pokemon]*Pokemon)
	copyParty := func(party []*Pokemon) []*Pokemon {
		cloned := make([]*Pokemon, len(party))
		for i, p := range party {
			if p == nil {
				continue
			}
			if pokemon[p] == nil {
				value := *p
				pokemon[p] = &value
			}
			cloned[i] = pokemon[p]
		}
		return cloned
	}
	next.PlayerParty = copyParty(b.PlayerParty)
	next.EnemyParty = copyParty(b.EnemyParty)
	next.Events = append([]BattleEvent(nil), b.Events...)
	next.PostMoveLearnEvents = append([]BattleEvent(nil), b.PostMoveLearnEvents...)
	next.AllowedActions = append([]string(nil), b.AllowedActions...)
	next.WildPostWinActions = append([]byte(nil), b.WildPostWinActions...)
	next.playerVolatile = append([]playerVolatileState(nil), b.playerVolatile...)
	if b.Trainer != nil {
		trainer := *b.Trainer
		trainer.PostWinActions = append([]byte(nil), b.Trainer.PostWinActions...)
		trainer.PostLoseActions = append([]byte(nil), b.Trainer.PostLoseActions...)
		next.Trainer = &trainer
	}
	if b.PendingMoveLearn != nil {
		pending := *b.PendingMoveLearn
		next.PendingMoveLearn = &pending
	}
	return &next
}

// Player durable stats remain in character_pokemon. Only battle-local state is
// stored beside the enemy/battle metadata, bound to stable owned row IDs.
type playerVolatileState struct {
	RowID          int64  `json:"rowId"`
	SleepTurns     int    `json:"sleepTurns"`
	BadPoisonTurns int    `json:"badPoisonTurns"`
	ConfusionTurns int    `json:"confusionTurns"`
	IsSeeded       bool   `json:"isSeeded"`
	SubstituteHP   int    `json:"substituteHp"`
	DireHit        bool   `json:"direHit"`
	GuardSpec      bool   `json:"guardSpec"`
	Stages         [6]int `json:"stages"`
}

func volatileState(p *Pokemon) playerVolatileState {
	return playerVolatileState{
		RowID: p.RowID, SleepTurns: p.SleepTurns, BadPoisonTurns: p.BadPoisonTurns,
		ConfusionTurns: p.ConfusionTurns, IsSeeded: p.IsSeeded, SubstituteHP: p.SubstituteHP,
		DireHit: p.DireHit, GuardSpec: p.GuardSpec,
		Stages: [6]int{p.AtkStage, p.DefStage, p.SpcStage, p.SpdStage, p.AccStage, p.EvaStage},
	}
}

// RestoreParty reloads the durable party and attaches battle-local state by row
// identity, never by guessed slot. Legacy version-zero saves had no player
// volatile data; accept their original behavior only for that explicit version.
func (b *BattleState) RestoreParty(database DBTX, charID int64) error {
	party, err := LoadParty(database, charID)
	if err != nil {
		return err
	}
	if len(party) == 0 {
		return fmt.Errorf("saved battle for character %d has no party", charID)
	}
	if b.persistedVersion == 2 {
		if b.BattleID == "" || b.Revision < 1 {
			return fmt.Errorf("saved battle has invalid durable identity")
		}
		states := make(map[int64]playerVolatileState)
		for _, v := range b.playerVolatile {
			if v.RowID <= 0 {
				return fmt.Errorf("saved battle has invalid player row ID %d", v.RowID)
			}
			if _, exists := states[v.RowID]; exists {
				return fmt.Errorf("saved battle duplicates player row %d", v.RowID)
			}
			states[v.RowID] = v
		}
		if len(states) != len(party) {
			return fmt.Errorf("saved battle party membership changed for character %d", charID)
		}
		for _, p := range party {
			v, ok := states[p.RowID]
			if !ok {
				return fmt.Errorf("saved battle has no volatile state for pokemon row %d", p.RowID)
			}
			p.SleepTurns = v.SleepTurns
			p.BadPoisonTurns = v.BadPoisonTurns
			p.ConfusionTurns = v.ConfusionTurns
			p.IsSeeded = v.IsSeeded
			p.SubstituteHP = v.SubstituteHP
			p.DireHit = v.DireHit
			p.GuardSpec = v.GuardSpec
			p.AtkStage = v.Stages[0]
			p.DefStage = v.Stages[1]
			p.SpcStage = v.Stages[2]
			p.SpdStage = v.Stages[3]
			p.AccStage = v.Stages[4]
			p.EvaStage = v.Stages[5]
		}
	}
	b.PlayerParty = party
	if b.PlayerActive < 0 || b.PlayerActive >= len(party) || b.EnemyActive < 0 || b.EnemyActive >= len(b.EnemyParty) {
		return fmt.Errorf("saved battle has invalid active party indices")
	}
	if pending := b.PendingMoveLearn; pending != nil && (pending.PokemonIndex < 0 || pending.PokemonIndex >= len(party)) {
		return fmt.Errorf("saved battle has invalid pending move pokemon index")
	}
	return nil
}
