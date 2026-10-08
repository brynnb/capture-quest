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
func useEscapeRope(ctx context.Context, database *sql.DB, charID, instanceID int32, sourceMapID, sourceX, sourceY int, expectedRevision int64, normalizeMap func(int) int) (escapeRopeResult, error) {
	var result escapeRopeResult
	_, err := cqitems.NewStore(database).ExecuteCommand(ctx, charID, expectedRevision, func(tx db.DBTX) error {
		if err := requireNoOwnedBattleIn(tx, int64(charID)); err != nil {
			return err
		}
		// Eligibility and exit choice must use the same source that ownership
		// advertised before the lock wait, not a later committed teleport.
		var savedMapID, savedX, savedY int
		if err := tx.QueryRow(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=$1`, charID).Scan(&savedMapID, &savedX, &savedY); err != nil {
			return err
		}
		if savedMapID != sourceMapID || savedX != sourceX || savedY != sourceY {
			return &itemuse.Rejection{Message: "Your position changed. Check your current location before using the Escape Rope."}
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
	// An explicit destination retires route intent, including same-tile teleports.
	// Movement transactions write their next cursor before the outer commit.
	if _, err := tx.Exec(`DELETE FROM character_movement_routes WHERE character_id=$1`, charID); err != nil {
		return err
	}
	if _, err := endSafariForDestinationIn(tx, charID, mapID); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE character_trainer_encounters SET resolution='cancelled',updated_at=CURRENT_TIMESTAMP WHERE character_id=$1 AND resolution='pending' AND (map_id<>$2 OR x<>$3 OR y<>$4)`, charID, mapID, x, y); err != nil {
		return err
	}
	if err := cancelCutsceneSourcesIn(tx, charID, mapID, x, y, ""); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE character_data SET map_id=$1,x=$2,y=$3,z=0,heading=0 WHERE id=$4`, mapID, x, y, charID)
	return err
}

// Position writers share one bounded character transaction. The caller owns
// the session gate; publication must follow successful return.
func commitPlayerPosition(ctx context.Context, database *sql.DB, charID int64, mapID, x, y int) error {
	return commitPosition(ctx, database, charID, mapID, x, y, false)
}

// Client coordinates must name a visible catalog tile. Trusted runtime destinations
// retain their own source-specific validation before calling commitPlayerPosition.
func commitClientPlayerPosition(ctx context.Context, database *sql.DB, charID int64, mapID, x, y int) error {
	return commitPosition(ctx, database, charID, mapID, x, y, true)
}

func commitPosition(ctx context.Context, database *sql.DB, charID int64, mapID, x, y int, validateCatalog bool) error {
	return db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if validateCatalog {
			if err := validateClientDestinationIn(tx, mapID, x, y); err != nil {
				return err
			}
		}

		// A same-position persistence flush is not a new destination intent.
		// Preserve its cursor only when storage, cursor and requested pose agree.
		route, err := loadMovementRouteIn(tx, charID)
		if err != nil {
			return err
		}
		var same bool
		if err := tx.QueryRow(`SELECT map_id=$2 AND CAST(x AS INTEGER)=$3 AND CAST(y AS INTEGER)=$4 FROM character_data WHERE id=$1`, charID, mapID, x, y).Scan(&same); err != nil {
			return err
		}
		if err := saveFieldDestinationIn(tx, charID, mapID, x, y); err != nil {
			return err
		}
		if same && route != nil && route.MapID == mapID && route.X == x && route.Y == y {
			return saveMovementRouteIn(tx, charID, mapID, x, y, route.Path, route.Surfing)
		}
		return nil
	})
}

func validateClientDestinationIn(tx db.DBTX, mapID, x, y int) error {
	// UnifiedOverworldMapID is synthetic; its catalog rows use NULL map_id.
	var valid bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM phaser_tiles
    WHERE (($4 AND map_id IS NULL) OR (NOT $4 AND map_id=$1)) AND x=$2 AND y=$3 AND is_tile_erased=0)
    AND ($4 OR EXISTS(SELECT 1 FROM phaser_maps WHERE id=$1))`, mapID, x, y, mapID == UnifiedOverworldMapID).Scan(&valid); err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("destination map %d tile (%d,%d) is absent or erased", mapID, x, y)
	}
	return nil
}
