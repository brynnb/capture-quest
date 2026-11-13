package world

import (
	"context"
	"database/sql"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
)

type RepelUseResult struct {
	Message   string
	StepsLeft int
	Inventory cqitems.CQInventorySnapshot `tstype:"import(\"./cqitems\").CQInventorySnapshot"`
}

func UseRepelInventoryItem(ctx context.Context, wh *WorldHandler, charID, instanceID int32, expectedRevision int64) (RepelUseResult, error) {
	if wh == nil || wh.WildEncounter == nil {
		return RepelUseResult{}, &itemuse.Rejection{Message: "Repel can't be used right now"}
	}
	if battle := getBattle(int64(charID)); battle != nil && !battle.IsOver() {
		return RepelUseResult{}, &itemuse.Rejection{Message: "Use the battle item menu during a battle"}
	}
	var result RepelUseResult
	snapshot, err := cqitems.NewStore(wh.WildEncounter.database).ExecuteCommand(ctx, charID, expectedRevision, func(tx db.DBTX) error {
		store := cqitems.NewStore(tx)
		current, err := store.FindInventoryItemByInstanceID(charID, instanceID)
		if err == sql.ErrNoRows {
			return &itemuse.Rejection{Message: "You don't have that item."}
		}
		if err != nil {
			return err
		}
		steps, ok := RepelStepsForItem(int(current.Item.ID))
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
		_, err = store.DecrementItemQuantity(charID, current.Instance.ID)
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
	result.Inventory = snapshot
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
		status, wore, err = advanceRepelStepIn(tx, charID)
		return err
	})
	if err != nil {
		return RepelStatus{}, false, err
	}
	return status, wore, nil
}

func advanceRepelStepIn(tx db.DBTX, charID int64) (RepelStatus, bool, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return RepelStatus{}, false, err
	}
	status, err := repelStatusIn(tx, charID)
	if err != nil || !status.Active {
		return status, false, err
	}
	status.StepsLeft--
	status.Active = status.StepsLeft > 0
	if err := setRepelStepsIn(tx, charID, status.StepsLeft); err != nil {
		return RepelStatus{}, false, err
	}
	return status, !status.Active, nil
}
