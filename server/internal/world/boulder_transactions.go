package world

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/db"
)

// One character lock owns Strength, object relocation, puzzle flags and the
// movement cursor. The returned result and flag view are published after commit.
func pushBoulder(ctx context.Context, database *sql.DB, charID int64, mapID, playerX, playerY int, direction string, activate bool, efm *EventFlagManager) (BoulderPushResult, error) {
	result := BoulderPushResult{MapID: mapID, Direction: normalizeBoulderDirection(direction)}
	if result.Direction == "" {
		result.Message = "Unknown push direction."
		return result, nil
	}
	var flags *EventFlagManager
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var savedMap, x, y int
		if err := tx.QueryRow(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=$1`, charID).Scan(&savedMap, &x, &y); err != nil {
			return err
		}
		if savedMap != mapID || x != playerX || y != playerY {
			return fmt.Errorf("boulder source ownership changed")
		}
		if err := requireNoOwnedBattleIn(tx, charID); err != nil {
			return err
		}
		route, err := loadMovementRouteIn(tx, charID)
		if err != nil {
			return err
		}
		if route != nil {
			return fmt.Errorf("finish current movement before pushing")
		}
		if err := tx.QueryRow(`SELECT name FROM phaser_maps WHERE id=$1`, mapID).Scan(&result.MapName); err != nil {
			return err
		}
		flags, err = eventFlagSnapshotIn(tx, charID)
		if err != nil {
			return err
		}
		boulders, err := boulderObjectsForCharacterContext(ctx, tx.(db.ContextDBTX), charID, mapID, flags)
		if err != nil {
			return err
		}
		dx, dy := boulderDirectionDelta(result.Direction)
		boulder, ok := visibleBoulderAt(boulders, playerX+dx, playerY+dy, 0)
		if !ok {
			result.Message = boulderNoBoulderMessage
			return nil
		}
		result.ObjectID, result.ObjectName = boulder.ObjectID, boulder.Name
		result.FromX, result.FromY = boulder.X, boulder.Y
		result.ToX, result.ToY = boulder.X+dx, boulder.Y+dy
		if activate {
			permission, err := fieldMovePermissionIn(tx, charID, "STRENGTH", func(flag string) (bool, error) { return queryEventFlag(tx, charID, flag) })
			if err != nil {
				return err
			}
			used := FieldMoveUseResult{Permission: permission, MoveName: permission.MoveName, MapID: mapID, Message: permission.Message}
			result.StrengthUsed = &used
			if !permission.Allowed {
				result.Message = permission.Message
				return nil
			}
			if _, err := tx.Exec(`INSERT INTO character_field_move_state(character_id,move_name,map_id,active) VALUES($1,'STRENGTH',$2,1) ON CONFLICT(character_id,move_name) DO UPDATE SET map_id=EXCLUDED.map_id,active=1`, charID, mapID); err != nil {
				return err
			}
			used.Success = true
			used.Message = fmt.Sprintf("%s used STRENGTH. %s can move boulders.", permission.KnownByName, permission.KnownByName)
		}
		var active int
		err = tx.QueryRow(`SELECT active FROM character_field_move_state WHERE character_id=$1 AND map_id=$2 AND move_name='STRENGTH'`, charID, mapID).Scan(&active)
		if err == sql.ErrNoRows || err == nil && active == 0 {
			result.Message = BoulderNeedsStrengthMessage
			return nil
		}
		if err != nil {
			return err
		}
		tile, err := baseEventTileStateIn(tx, mapID, result.ToX, result.ToY)
		if err != nil {
			return err
		}
		overrides, err := eventTileOverridesForMapContext(ctx, tx.(db.ContextDBTX), mapID)
		if err != nil {
			return err
		}
		for _, override := range overrides {
			if override.X == result.ToX && override.Y == result.ToY && override.eventTileEligible(charID, flags) {
				tile.CollisionType = override.CollisionType
			}
		}
		if tile.CollisionType <= 0 {
			result.Message = "The boulder won't budge."
			return nil
		}
		if _, occupied := visibleBoulderAt(boulders, result.ToX, result.ToY, boulder.ObjectID); occupied {
			result.Message = "The boulder won't budge."
			return nil
		}
		if _, err := tx.Exec(`INSERT INTO character_object_positions(character_id,object_id,map_id,x,y) VALUES($1,$2,$3,$4,$5) ON CONFLICT(character_id,object_id) DO UPDATE SET map_id=EXCLUDED.map_id,x=EXCLUDED.x,y=EXCLUDED.y`, charID, boulder.ObjectID, mapID, result.ToX, result.ToY); err != nil {
			return err
		}
		if hole, ok := SeafoamBoulderHoleAt(result.MapName, result.ToX, result.ToY); ok {
			result.Dropped, result.FlagSet = true, hole.Flag
			result.AffectedMaps = append(result.AffectedMaps, hole.DestinationMapName)
		}
		targets, err := victoryRoadBoulderTargetsIn(tx)
		if err != nil {
			return err
		}
		for _, target := range targets {
			if target.MapName == result.MapName && target.X == result.ToX && target.Y == result.ToY {
				if target.Flag == "" {
					return fmt.Errorf("boulder target has no flag")
				}
				result.FlagSet = target.Flag
				result.Dropped = result.Dropped || target.DropsThroughHole
				if target.DestinationMapName != "" {
					result.AffectedMaps = append(result.AffectedMaps, target.DestinationMapName)
				}
			}
		}
		if result.FlagSet != "" {
			if err := writeEventFlag(tx, charID, result.FlagSet, true); err != nil {
				return err
			}
			flags.flags[charID][result.FlagSet] = true
		}
		if result.Dropped {
			if _, err := tx.Exec(`DELETE FROM character_object_positions WHERE character_id=$1 AND object_id=$2`, charID, boulder.ObjectID); err != nil {
				return err
			}
		}
		if err := saveMovementRouteIn(tx, charID, mapID, playerX, playerY, []PathNode{{X: result.FromX, Y: result.FromY}}, false); err != nil {
			return err
		}
		result.Success, result.Message = true, "The boulder moved."
		return nil
	})
	if err != nil {
		return BoulderPushResult{}, err
	}
	if efm != nil && flags != nil {
		efm.publishCommittedFlags(charID, flags.flags[charID])
	}
	return result, nil
}
