package world

import (
	"encoding/json"
	"log"

	"capturequest/internal/api/opcodes"
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
	access, err := AvailableElevatorFloors(charID, req.MapID, wh.EventFlags)
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
	currentMapID := int(char.MapID)

	floor, err := ElevatorDestination(int64(char.ID), currentMapID, req.FloorMapID, wh.EventFlags)
	if err != nil {
		log.Printf("[Elevator] Floor %d not found in elevator %d: %v", req.FloorMapID, currentMapID, err)
		return false
	}

	endSafariSessionIfLeavingMap(int64(char.ID), currentMapID, req.FloorMapID, wh)

	// Teleport the player to the selected floor
	wh.PlayerMovement.UpdatePosition(int(char.ID), floor.DestX, floor.DestY, req.FloorMapID, "DOWN")
	wh.PlayerMovement.FlushPlayerPosition(int(char.ID))

	// Update session state
	ses.X = float32(floor.DestX)
	ses.Y = float32(floor.DestY)
	if wh.ActorManager.IsOverworld(req.FloorMapID) {
		ses.MapID = UnifiedOverworldMapID
	} else {
		ses.MapID = req.FloorMapID
	}
	char.X = float64(floor.DestX)
	char.Y = float64(floor.DestY)
	char.MapID = uint32(ses.MapID) // Use normalized ID (9999 for overworld)

	// Send teleport notification to client
	ses.SendStreamJSON(map[string]interface{}{
		"mapId": req.FloorMapID,
		"x":     floor.DestX,
		"y":     floor.DestY,
	}, opcodes.WarpTileTeleportNotify)

	log.Printf("[Elevator] Player %d teleported to floor %d (%d,%d)", char.ID, req.FloorMapID, floor.DestX, floor.DestY)
	return false
}
