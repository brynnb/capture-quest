package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

type EscapeRopeUseRequest struct {
	RequestID  string                    `json:"requestId"`
	Command    *InventoryCommandIdentity `json:"command" tstype:"InventoryCommandIdentity"`
	InstanceID int32                     `json:"instanceId"`
	MapID      int                       `json:"mapId"`
	X          *int                      `json:"x"`
	Y          *int                      `json:"y"`
}

// This acknowledges commit only. Current position and resources are recovered
// together through GameplayStateRequest; historical replies never teleport.
type EscapeRopeUseResponse struct {
	Success     bool   `json:"success" tstype:"true"`
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
}

func HandleEscapeRopeUse(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req EscapeRopeUseRequest
	fail := func() {
		sendInventoryCommandError(ses, req.RequestID, opcodes.EscapeRopeUseResponse, "Could not use the Escape Rope. Check your current location and bag.")
	}
	if decodePlayerMovement(payload, &req) != nil || !validInventoryCommand(ses, req.RequestID, req.Command) || req.InstanceID <= 0 || req.X == nil || req.Y == nil || wh.PlayerMovement == nil {
		fail()
		return false
	}
	charID := int(ses.Client.CharData().ID)
	m := wh.PlayerMovement
	m.mu.RLock()
	state := m.players[charID]
	owned := state != nil && state.SessionID == ses.SessionID && state.MapID == req.MapID && state.CurrentX == *req.X && state.CurrentY == *req.Y && len(state.Path) == 0 && state.pendingStep == nil
	m.mu.RUnlock()
	if !owned {
		fail()
		return false
	}
	result, err := useEscapeRope(ses.CommandContext(), wh.database, int32(charID), req.InstanceID, req.MapID, *req.X, *req.Y, *req.Command.Revision, func(id int) int { return normalizedVisiblePlayerMapID(wh, id) })
	if err != nil {
		fail()
		return false
	}
	// Refresh authoritative server ownership without an unsolicited client warp.
	// The client command owner projects a coherent current read after settlement.
	refreshSafariFlags(wh, int64(charID))
	publishCommittedPlayerPosition(ses, wh, result.MapID, result.X, result.Y, "DOWN")
	ses.SendStreamJSON(EscapeRopeUseResponse{Success: true, RequestID: req.RequestID, CharacterID: int64(charID)}, opcodes.EscapeRopeUseResponse)
	notifyResourceChange(ses)
	return false
}
