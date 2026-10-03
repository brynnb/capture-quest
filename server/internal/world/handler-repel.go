package world

import (
	"errors"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
)

type RepelUseRequest struct {
	RequestID  string                    `json:"requestId"`
	Command    *InventoryCommandIdentity `json:"command" tstype:"InventoryCommandIdentity"`
	InstanceID int32                     `json:"instanceId"`
}

type RepelUseResponse struct {
	Success    bool                        `json:"success" tstype:"true"`
	RequestID  string                      `json:"requestId"`
	InstanceID int32                       `json:"instanceId"`
	Message    string                      `json:"message"`
	StepsLeft  int                         `json:"stepsLeft"`
	Inventory  cqitems.CQInventorySnapshot `json:"inventory" tstype:"import(\"./cqitems\").CQInventorySnapshot"`
}

// HandleRepelUse activates a repel when the player uses one from inventory.
func HandleRepelUse(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req RepelUseRequest
	if err := decodePlayerMovement(payload, &req); err != nil || !validInventoryCommand(ses, req.RequestID, req.Command) {
		sendInventoryCommandError(ses, req.RequestID, opcodes.RepelUseResponse, "Invalid repel command identity.")
		return false
	}
	charID := int64(ses.Client.CharData().ID)

	result, err := UseRepelInventoryItem(ses.CommandContext(), wh, int32(charID), req.InstanceID, *req.Command.Revision)
	if err != nil {
		sendInventoryCommandError(ses, req.RequestID, opcodes.RepelUseResponse, repelUseErrorMessage(charID, err))
		return false
	}

	ses.SendStreamJSON(RepelUseResponse{Success: true, RequestID: req.RequestID, InstanceID: req.InstanceID,
		Message: result.Message, StepsLeft: result.StepsLeft, Inventory: result.Inventory}, opcodes.RepelUseResponse)
	return false
}

func repelUseErrorMessage(charID int64, err error) string {
	var rejection *itemuse.Rejection
	if errors.As(err, &rejection) {
		return rejection.Message
	}
	log.Printf("[Repel] Use failed for character %d: %v", charID, err)
	return "Could not use the repel. Please try again."
}
