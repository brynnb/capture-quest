package world

import (
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

	floor, err := UseElevator(ses.CommandContext(), wh.database, int64(char.ID), currentMapID, source.X, source.Y, req.FloorMapID, func(id int) int { return normalizedVisiblePlayerMapID(wh, id) })
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
