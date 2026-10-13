package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
)

// ElevatorFloor represents one selectable floor in an elevator.
type ElevatorFloor struct {
	FloorMapID     int    `json:"floorMapId"`
	FloorLabel     string `json:"floorLabel"`
	DestX          int    `json:"destX"`
	DestY          int    `json:"destY"`
	RequiresFlag   string `json:"requiresFlag,omitempty"`
	RequiresItemID int    `json:"requiresItemId,omitempty"`
}

// ElevatorFloorsRequestPayload is sent by the client when clicking an elevator panel.
type ElevatorFloorsRequestPayload struct {
	MapID int `json:"mapId"` // The elevator room's map ID
}

// ElevatorSelectRequestPayload is sent when the client picks a floor.
type ElevatorSelectRequestPayload struct {
	FloorMapID int `json:"floorMapId"` // The selected floor's map ID
}

// HandleElevatorFloorsRequest returns available floors for the elevator the player is in.
func HandleElevatorFloorsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req ElevatorFloorsRequestPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Elevator] Invalid floors request: %v", err)
		return false
	}

	charID := int64(0)
	if char := ses.Client.CharData(); char != nil {
		charID = int64(char.ID)
	}
	source := wh.ownedPlayerSnapshot(ses, "")
	var access ElevatorAccess
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if req.MapID != source.MapID {
			return fmt.Errorf("elevator source map mismatch")
		}
		if err := requireTravelSourceIn(tx, charID, source.MapID, source.X, source.Y, func(id int) int { return normalizedVisiblePlayerMapID(wh, id) }); err != nil {
			return err
		}
		if err := wh.authorizeElevatorPanelIn(ses.CommandContext(), tx, ses, source.MapID, source.X, source.Y); err != nil {
			return err
		}
		var err error
		access, err = AvailableElevatorFloors(tx, charID, req.MapID)
		return err
	})
	if err != nil {
		log.Printf("[Elevator] Error querying floors for elevator %d: %v", req.MapID, err)
		return false
	}

	if len(access.Floors) == 0 {
		// No accessible floors (e.g. Rocket Hideout without LIFT KEY)
		ses.SendStreamJSON(map[string]interface{}{
			"floors":  []ElevatorFloor{},
			"message": access.Message,
		}, opcodes.ElevatorFloorsResponse)
		return false
	}

	ses.SendStreamJSON(map[string]interface{}{
		"floors": access.Floors,
	}, opcodes.ElevatorFloorsResponse)
	log.Printf("[Elevator] Sent %d floors for elevator %d", len(access.Floors), req.MapID)
	return false
}

// HandleElevatorSelectRequest teleports the player to the selected floor.
func HandleElevatorSelectRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req ElevatorSelectRequestPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Elevator] Invalid select request: %v", err)
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}

	// Get the player's current map to determine which elevator they're in
	source := wh.ownedPlayerSnapshot(ses, "")
	currentMapID := source.MapID

	var floor *ElevatorFloor
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(char.ID)); err != nil {
			return err
		}
		if err := wh.authorizeElevatorPanelIn(ses.CommandContext(), tx, ses, currentMapID, source.X, source.Y); err != nil {
			return err
		}
		var err error
		floor, err = UseElevator(ses.CommandContext(), tx, int64(char.ID), currentMapID, source.X, source.Y, req.FloorMapID, func(id int) int { return normalizedVisiblePlayerMapID(wh, id) })
		return err
	})
	if err != nil {
		log.Printf("[Elevator] Floor %d not found in elevator %d: %v", req.FloorMapID, currentMapID, err)
		return false
	}

	refreshSafariFlags(ses, wh, int64(char.ID))
	publishCommittedPlayerPosition(ses, wh, normalizedVisiblePlayerMapID(wh, floor.FloorMapID), floor.DestX, floor.DestY, "DOWN")

	// Send teleport notification to client
	sendCommittedWarpNotification(ses, req.FloorMapID, floor.DestX, floor.DestY, "DOWN")

	log.Printf("[Elevator] Player %d teleported to floor %d (%d,%d)", char.ID, req.FloorMapID, floor.DestX, floor.DestY)
	return false
}

// The current catalog has one sign in each elevator room. Resolve that source
// deterministically, then share actor position/visibility/reach authorization.
// Missing or ambiguous panels reject rather than guessing a sign or coordinate.
func (wh *WorldHandler) authorizeElevatorPanelIn(ctx context.Context, tx db.DBTX, ses *session.Session, mapID, x, y int) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	source := wh.ownedPlayerSnapshot(ses, "")
	if source.ServerMovementPending || source.MapID != mapID || source.X != x || source.Y != y {
		return errScriptInteractionDenied
	}
	var objectID *int
	var count int
	if err := tx.QueryRow(`SELECT MIN(o.id),COUNT(*) FROM phaser_objects o WHERE o.map_id=$1 AND o.object_type='sign' AND EXISTS(SELECT 1 FROM phaser_elevator_floors f WHERE f.elevator_map_id=o.map_id)`, mapID).Scan(&objectID, &count); err != nil {
		return err
	}
	if count != 1 || objectID == nil {
		return fmt.Errorf("elevator map %d has %d panel candidates", mapID, count)
	}
	flags, err := eventFlagSnapshotIn(tx, int64(ses.Client.CharData().ID))
	if err != nil {
		return err
	}
	_, _, err = wh.scriptInteractionTargetFrom(ctx, tx.(db.ContextDBTX), ses, *objectID, flags, interactionPosition{MapID: mapID, X: x, Y: y})
	return err
}
