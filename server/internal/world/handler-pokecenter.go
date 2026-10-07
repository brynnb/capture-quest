package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

// Opcode 79 is reserved to reject stale clients explicitly. Center healing now
// requires a source-authorized scripted interaction and its durable completion.
func HandlePokeCenterHeal(ses *session.Session, _ []byte, _ *WorldHandler) bool {
	ses.SendStreamJSON(map[string]interface{}{
		"success": false,
		"error":   "Interact with the nurse using the current client.",
	}, opcodes.PokeCenterHealResponse)
	return false
}
