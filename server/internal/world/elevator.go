package world

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/db"
)

const elevatorNeedsKeyMessage = "It appears to need a key."

// ElevatorAccess contains the result of opening an elevator panel.
type ElevatorAccess struct {
	Floors  []ElevatorFloor
	Message string
}

// AvailableElevatorFloors returns the floors selectable by the character in an elevator.
func AvailableElevatorFloors(q db.DBTX, charID int64, elevatorMapID int) (ElevatorAccess, error) {
	rows, err := q.Query(
		`SELECT floor_map_id, floor_label, dest_x, dest_y, COALESCE(requires_flag, ''), COALESCE(requires_item_id, 0)
		 FROM phaser_elevator_floors
		 WHERE elevator_map_id = $1
		 ORDER BY sort_order`, elevatorMapID)
	if err != nil {
		return ElevatorAccess{}, fmt.Errorf("query elevator floors for map %d: %w", elevatorMapID, err)
	}
	defer rows.Close()

	floors := []ElevatorFloor{}
	for rows.Next() {
		var floor ElevatorFloor
		if err := rows.Scan(
			&floor.FloorMapID,
			&floor.FloorLabel,
			&floor.DestX,
			&floor.DestY,
			&floor.RequiresFlag,
			&floor.RequiresItemID,
		); err != nil {
			return ElevatorAccess{}, fmt.Errorf("scan elevator floor: %w", err)
		}
		floors = append(floors, floor)
	}
	if err := rows.Err(); err != nil {
		return ElevatorAccess{}, fmt.Errorf("iterate elevator floors: %w", err)
	}
	// Release the catalog cursor before permission reads on the same query owner.
	if err := rows.Close(); err != nil {
		return ElevatorAccess{}, err
	}
	available := make([]ElevatorFloor, 0, len(floors))
	for _, floor := range floors {
		allowed, err := elevatorFloorAllowed(q, charID, floor)
		if err != nil {
			return ElevatorAccess{}, err
		}
		if allowed {
			available = append(available, floor)
		}
	}
	floors = available

	access := ElevatorAccess{Floors: floors}
	if len(floors) == 0 {
		access.Message = elevatorNeedsKeyMessage
	}
	return access, nil
}

// ElevatorDestination validates and returns the selected elevator destination.
func ElevatorDestination(q db.DBTX, charID int64, elevatorMapID, floorMapID int) (*ElevatorFloor, error) {
	var floor ElevatorFloor
	err := q.QueryRow(
		`SELECT floor_map_id, floor_label, dest_x, dest_y, COALESCE(requires_flag, ''), COALESCE(requires_item_id, 0)
		 FROM phaser_elevator_floors
		 WHERE elevator_map_id = $1 AND floor_map_id = $2`,
		elevatorMapID, floorMapID).Scan(
		&floor.FloorMapID,
		&floor.FloorLabel,
		&floor.DestX,
		&floor.DestY,
		&floor.RequiresFlag,
		&floor.RequiresItemID,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("floor %d not found in elevator %d", floorMapID, elevatorMapID)
	}
	if err != nil {
		return nil, fmt.Errorf("query elevator destination %d/%d: %w", elevatorMapID, floorMapID, err)
	}

	allowed, err := elevatorFloorAllowed(q, charID, floor)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("floor %d is not accessible in elevator %d", floorMapID, elevatorMapID)
	}
	return &floor, nil
}

func elevatorFloorAllowed(q db.DBTX, charID int64, floor ElevatorFloor) (bool, error) {
	if floor.RequiresFlag != "" {
		on, err := queryEventFlag(q, charID, floor.RequiresFlag)
		if err != nil {
			return false, err
		}
		if charID <= 0 || !on {
			return false, nil
		}
	}
	if floor.RequiresItemID > 0 {
		if charID <= 0 {
			return false, nil
		}
		hasItem, err := characterHasCQItemIn(q, charID, floor.RequiresItemID)
		if err != nil {
			return false, fmt.Errorf("check elevator required item %d: %w", floor.RequiresItemID, err)
		}
		if !hasItem {
			return false, nil
		}
	}
	return true, nil
}

// UseElevator joins source, durable access and catalog destination with the
// established movement commit. Supplied transactions retain outer ownership.
func UseElevator(ctx context.Context, database db.DBTX, charID int64, sourceMap, sourceX, sourceY, floorMap int, normalize func(int) int) (*ElevatorFloor, error) {
	var floor *ElevatorFloor
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if err := requireTravelSourceIn(tx, charID, sourceMap, sourceX, sourceY, normalize); err != nil {
			return err
		}
		var err error
		floor, err = ElevatorDestination(tx, charID, sourceMap, floorMap)
		if err != nil {
			return err
		}
		return saveFieldDestinationIn(tx, charID, normalize(floor.FloorMapID), floor.DestX, floor.DestY)
	})
	return floor, err
}
