package world

import (
	"capturequest/internal/logutil"
	"context"
	"encoding/json"
	"fmt"

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

// ElevatorFloorsRequestPayload binds reads and selections to their expected source.
type ElevatorFloorsRequestPayload struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	MapID       *int   `json:"mapId"`
	FromX       *int   `json:"fromX"`
	FromY       *int   `json:"fromY"`
}
type ElevatorSelectRequestPayload struct {
	RequestID       string `json:"requestId"`
	CharacterID     int64  `json:"characterId"`
	MapID           *int   `json:"mapId"`
	FromX           *int   `json:"fromX"`
	FromY           *int   `json:"fromY"`
	CommandRevision *int64 `json:"commandRevision"`
	FloorMapID      int    `json:"floorMapId"`
}

// Both policies use the existing response opcode, with explicit kind identity.
type ElevatorResponse struct {
	Success     bool            `json:"success" tstype:"true"`
	RequestID   string          `json:"requestId"`
	CharacterID int64           `json:"characterId"`
	Kind        string          `json:"kind"`
	MapID       int             `json:"mapId"`
	Floors      []ElevatorFloor `json:"floors"`
	Message     string          `json:"message"`
}

func validElevatorSource(ses *session.Session, requestID string, charID int64, m, x, y *int) bool {
	return ses.HasValidClient() && requestID != "" && len(requestID) <= 64 && charID == int64(ses.Client.CharData().ID) && m != nil && x != nil && y != nil
}
func HandleElevatorFloorsRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req ElevatorFloorsRequestPayload
	reject := func() {
		sendInventoryCommandError(ses, req.RequestID, opcodes.ElevatorFloorsResponse, "Elevator source unavailable. Check your current location.")
	}
	if err := decodePlayerMovement(payload, &req); err != nil || !validElevatorSource(ses, req.RequestID, req.CharacterID, req.MapID, req.FromX, req.FromY) {
		reject()
		return false
	}
	var access ElevatorAccess
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, req.CharacterID); err != nil {
			return err
		}
		if err := requireTravelSourceIn(tx, req.CharacterID, *req.MapID, *req.FromX, *req.FromY, func(id int) int { return normalizedVisiblePlayerMapID(wh, id) }); err != nil {
			return err
		}
		if err := wh.authorizeElevatorPanelIn(ses.CommandContext(), tx, ses, *req.MapID, *req.FromX, *req.FromY); err != nil {
			return err
		}
		var err error
		access, err = AvailableElevatorFloors(tx, req.CharacterID, *req.MapID)
		return err
	})
	if err != nil {
		logutil.Debugf("[Elevator] read character=%d: %v", req.CharacterID, err)
		reject()
		return false
	}
	ses.SendStreamJSON(ElevatorResponse{Success: true, RequestID: req.RequestID, CharacterID: req.CharacterID, Kind: "list", MapID: *req.MapID, Floors: access.Floors, Message: access.Message}, opcodes.ElevatorFloorsResponse)
	return false
}
func HandleElevatorSelectRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req ElevatorSelectRequestPayload
	reject := func() {
		sendInventoryCommandError(ses, req.RequestID, opcodes.ElevatorFloorsResponse, "Elevator travel state changed or could not be read. Recover before trying again.")
	}
	if err := decodePlayerMovement(payload, &req); err != nil || !validElevatorSource(ses, req.RequestID, req.CharacterID, req.MapID, req.FromX, req.FromY) || req.CommandRevision == nil || req.FloorMapID <= 0 {
		reject()
		return false
	}
	input, _ := json.Marshal(req)
	var floor *ElevatorFloor
	encoded, replay, err := db.ExecuteFieldCommand(ses.CommandContext(), wh.database, req.CharacterID, "travel", req.RequestID, req.CommandRevision, input, func(tx db.DBTX) ([]byte, error) {
		if err := wh.authorizeElevatorPanelIn(ses.CommandContext(), tx, ses, *req.MapID, *req.FromX, *req.FromY); err != nil {
			return nil, err
		}
		var err error
		floor, err = UseElevator(ses.CommandContext(), tx, req.CharacterID, *req.MapID, *req.FromX, *req.FromY, req.FloorMapID, func(id int) int { return normalizedVisiblePlayerMapID(wh, id) })
		if err != nil {
			return nil, err
		}
		return json.Marshal(ElevatorResponse{Success: true, RequestID: req.RequestID, CharacterID: req.CharacterID, Kind: "select", MapID: normalizedVisiblePlayerMapID(wh, floor.FloorMapID), Floors: []ElevatorFloor{}, Message: "Elevator travel committed."})
	})
	if err != nil {
		logutil.Debugf("[Elevator] select character=%d: %v", req.CharacterID, err)
		reject()
		return false
	}
	var reply ElevatorResponse
	if err := json.Unmarshal(encoded, &reply); err != nil || !reply.Success || reply.RequestID != req.RequestID || reply.CharacterID != req.CharacterID || reply.Kind != "select" || reply.MapID <= 0 || reply.Message == "" {
		reject()
		return false
	}
	if !replay {
		refreshSafariFlags(ses, wh, req.CharacterID)
		publishCommittedPlayerPosition(ses, wh, reply.MapID, floor.DestX, floor.DestY, "DOWN")
	}
	ses.SendStreamJSON(reply, opcodes.ElevatorFloorsResponse)
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
