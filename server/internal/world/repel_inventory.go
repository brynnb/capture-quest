package world

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
)

type RepelUseResult struct {
	Message     string
	NewQuantity uint16
	StepsLeft   int
}

func UseRepelInventoryItem(ctx context.Context, wh *WorldHandler, charID int32, itemID int32, found *cqitems.CQInventoryItem) (RepelUseResult, error) {
	if wh == nil || wh.WildEncounter == nil {
		return RepelUseResult{}, &itemuse.Rejection{Message: "Repel can't be used right now"}
	}
	if battle := getBattle(int64(charID)); battle != nil && !battle.IsOver() {
		return RepelUseResult{}, &itemuse.Rejection{Message: "Use the battle item menu during a battle"}
	}
	var result RepelUseResult
	err := db.Transaction(ctx, wh.WildEncounter.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		steps, ok := RepelStepsForItem(int(itemID))
		if !ok {
			return &itemuse.Rejection{Message: "Not a repel item"}
		}
		status, err := repelStatusIn(tx, int64(charID))
		if err != nil {
			return err
		}
		if status.Active {
			return &itemuse.Rejection{Message: "A repel is already active!"}
		}
		store := cqitems.NewStore(tx)
		var current *cqitems.CQInventoryItem
		if found == nil {
			current, err = store.FindInventoryItemByItemID(charID, itemID)
		} else {
			current, err = store.FindInventoryItemByInstanceID(charID, found.Instance.ID)
		}
		if err == sql.ErrNoRows {
			return &itemuse.Rejection{Message: "You don't have that item."}
		}
		if err != nil {
			return err
		}
		if current.Item.ID != itemID {
			return fmt.Errorf("repel instance %d item mismatch: got %d, want %d", current.Instance.ID, current.Item.ID, itemID)
		}
		result.NewQuantity, err = store.DecrementItemQuantity(charID, current.Instance.ID)
		if err != nil {
			return err
		}
		if err := setRepelStepsIn(tx, int64(charID), steps); err != nil {
			return err
		}
		result.Message = current.Item.Name + "'s effect started!"
		result.StepsLeft = steps
		return nil
	})
	if err != nil {
		return RepelUseResult{}, err
	}
	return result, nil
}

func repelStatusIn(database db.DBTX, charID int64) (RepelStatus, error) {
	var steps int
	err := database.QueryRow(`SELECT steps_left FROM character_repels WHERE character_id=$1`, charID).Scan(&steps)
	if err == sql.ErrNoRows {
		return RepelStatus{}, nil
	}
	if err != nil {
		return RepelStatus{}, err
	}
	return RepelStatus{Active: steps > 0, StepsLeft: steps}, nil
}

func loadRepelStatus(ctx context.Context, database *sql.DB, charID int64) (RepelStatus, error) {
	var result RepelStatus
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		var err error
		result, err = repelStatusIn(tx, charID)
		return err
	})
	if err != nil {
		return RepelStatus{}, err
	}
	return result, nil
}

func setRepelStepsIn(tx db.DBTX, charID int64, steps int) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	if steps <= 0 {
		_, err := tx.Exec(`DELETE FROM character_repels WHERE character_id=$1`, charID)
		return err
	}
	_, err := tx.Exec(`INSERT INTO character_repels(character_id,steps_left) VALUES($1,$2) ON CONFLICT(character_id) DO UPDATE SET steps_left=EXCLUDED.steps_left,updated_at=CURRENT_TIMESTAMP`, charID, steps)
	return err
}

// Fixture setup and live effects use the same durable counter. Disconnect does
// not erase it, and callers must propagate failure before publishing state.
func setRepelSteps(ctx context.Context, database *sql.DB, charID int64, steps int) error {
	return db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		return setRepelStepsIn(tx, charID, steps)
	})
}

func advanceRepelStep(ctx context.Context, database *sql.DB, charID int64) (RepelStatus, bool, error) {
	var status RepelStatus
	var wore bool
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var err error
		status, err = repelStatusIn(tx, charID)
		if err != nil || !status.Active {
			return err
		}
		status.StepsLeft--
		status.Active = status.StepsLeft > 0
		wore = !status.Active
		return setRepelStepsIn(tx, charID, status.StepsLeft)
	})
	if err != nil {
		return RepelStatus{}, false, err
	}
	return status, wore, nil
}
