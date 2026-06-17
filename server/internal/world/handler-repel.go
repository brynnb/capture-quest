package world

import (
	"encoding/json"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

// HandleRepelUse activates a repel when the player uses one from inventory.
func HandleRepelUse(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		ItemID int `json:"itemId"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Repel] Invalid use request: %v", err)
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	result, err := UseRepelInventoryItem(wh, int32(charID), int32(req.ItemID), nil)
	if err != nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.RepelUseResponse)
		return false
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":     true,
		"message":     result.Message,
		"newQuantity": result.NewQuantity,
		"stepsLeft":   result.StepsLeft,
	}, opcodes.RepelUseResponse)
	return false
}
