package world

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
)

type escapeRopeResult struct {
	MapID, X, Y int
	NewQuantity uint16
}

// Keep item ownership, the authoritative exit and saved position inside the
// same commit. The handler publishes movement only after this operation returns.
func useEscapeRope(ctx context.Context, database *sql.DB, charID, instanceID int32, sourceMapID int, normalizeMap func(int) int) (escapeRopeResult, error) {
	var result escapeRopeResult
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		store := cqitems.NewStore(tx)
		found, err := store.FindInventoryItemByInstanceID(charID, instanceID)
		if err != nil {
			return err
		}
		if itemuse.ShortName(found.Item) != "ESCAPE_ROPE" {
			return fmt.Errorf("item instance %d is not an Escape Rope", instanceID)
		}
		result.MapID, result.X, result.Y, err = escapeRopeDestinationIn(tx, sourceMapID)
		if err != nil {
			return err
		}
		result.MapID = normalizeMap(result.MapID)
		result.NewQuantity, err = store.DecrementItemQuantity(charID, instanceID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE character_data SET map_id=$1,x=$2,y=$3,z=0,heading=0 WHERE id=$4`, result.MapID, result.X, result.Y, charID)
		return err
	})
	if err != nil {
		return escapeRopeResult{}, err
	}
	return result, nil
}
