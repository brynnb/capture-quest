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
		return saveFieldDestinationIn(tx, int64(charID), result.MapID, result.X, result.Y)
	})
	if err != nil {
		return escapeRopeResult{}, err
	}
	return result, nil
}

type flyResult struct {
	MapID, X, Y int
	Permission  FieldMovePermissionResult
}

func useFly(ctx context.Context, database *sql.DB, charID int64, mapID, x, y int, normalizeMap func(int) int) (flyResult, error) {
	var result flyResult
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var err error
		result.Permission, err = fieldMovePermissionIn(tx, charID, "FLY", func(flag string) (bool, error) {
			return queryEventFlag(tx, charID, flag)
		})
		if err != nil {
			return err
		}
		if !result.Permission.Allowed {
			return &itemuse.Rejection{Message: result.Permission.Message}
		}
		// This is the catalog served to the current UI. Coordinates identify a
		// catalog destination; they never authorize an arbitrary teleport.
		var count int
		var spawnX, spawnY sql.NullInt64
		if err := tx.QueryRow(`SELECT COUNT(*),MIN(spawn_x),MIN(spawn_y) FROM poke_start_cities WHERE map_id=$1`, mapID).Scan(&count, &spawnX, &spawnY); err != nil {
			return err
		}
		if count == 0 || (count == 1 && (int64(x) != spawnX.Int64 || int64(y) != spawnY.Int64)) {
			return &itemuse.Rejection{Message: "Choose a valid FLY destination."}
		}
		if count != 1 || !spawnX.Valid || !spawnY.Valid {
			return fmt.Errorf("FLY catalog map %d has %d destinations or missing coordinates", mapID, count)
		}
		result.MapID, result.X, result.Y = normalizeMap(mapID), int(spawnX.Int64), int(spawnY.Int64)
		return saveFieldDestinationIn(tx, charID, result.MapID, result.X, result.Y)
	})
	if err != nil {
		return flyResult{}, err
	}
	return result, nil
}

func saveFieldDestinationIn(tx db.DBTX, charID int64, mapID, x, y int) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE character_data SET map_id=$1,x=$2,y=$3,z=0,heading=0 WHERE id=$4`, mapID, x, y, charID)
	return err
}
