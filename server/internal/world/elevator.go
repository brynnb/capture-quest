package world

import (
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
func AvailableElevatorFloors(charID int64, elevatorMapID int, efm *EventFlagManager) (ElevatorAccess, error) {
	rows, err := db.GlobalWorldDB.DB.Query(
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
		allowed, err := elevatorFloorAllowed(charID, floor, efm)
		if err != nil {
			return ElevatorAccess{}, err
		}
		if allowed {
			floors = append(floors, floor)
		}
	}
	if err := rows.Err(); err != nil {
		return ElevatorAccess{}, fmt.Errorf("iterate elevator floors: %w", err)
	}

	access := ElevatorAccess{Floors: floors}
	if len(floors) == 0 {
		access.Message = elevatorNeedsKeyMessage
	}
	return access, nil
}

// ElevatorDestination validates and returns the selected elevator destination.
func ElevatorDestination(charID int64, elevatorMapID, floorMapID int, efm *EventFlagManager) (*ElevatorFloor, error) {
	var floor ElevatorFloor
	err := db.GlobalWorldDB.DB.QueryRow(
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

	allowed, err := elevatorFloorAllowed(charID, floor, efm)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("floor %d is not accessible in elevator %d", floorMapID, elevatorMapID)
	}
	return &floor, nil
}

func elevatorFloorAllowed(charID int64, floor ElevatorFloor, efm *EventFlagManager) (bool, error) {
	if floor.RequiresFlag != "" {
		if efm == nil || charID <= 0 || !efm.CheckFlag(charID, floor.RequiresFlag) {
			return false, nil
		}
	}
	if floor.RequiresItemID > 0 {
		if charID <= 0 {
			return false, nil
		}
		hasItem, err := characterHasCQItem(charID, floor.RequiresItemID)
		if err != nil {
			return false, fmt.Errorf("check elevator required item %d: %w", floor.RequiresItemID, err)
		}
		if !hasItem {
			return false, nil
		}
	}
	return true, nil
}
