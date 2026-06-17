package world

import (
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/session"
)

// HandleWarpHomeRequest is an emergency recovery action.
func HandleWarpHomeRequest(ses *session.Session, _ []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}

	const direction = RecoverySpawnDirection
	charID := int64(char.ID)
	mapID := RecoverySpawnMap
	x := int(RecoverySpawnX)
	y := int(RecoverySpawnY)

	ClearBattleForCharacter(charID)
	if wh != nil && wh.Safari != nil {
		wh.Safari.EndSession(charID)
	}
	if wh != nil && wh.EventFlags != nil {
		if err := wh.EventFlags.ResetFlag(charID, EventInSafariZone); err != nil {
			log.Printf("[WarpHome] Failed to reset %s for char %d: %v", EventInSafariZone, charID, err)
		}
		if err := wh.EventFlags.ResetFlag(charID, EventSafariGameOver); err != nil {
			log.Printf("[WarpHome] Failed to reset %s for char %d: %v", EventSafariGameOver, charID, err)
		}
	}

	setServerTeleportedPlayerPosition(ses, wh, mapID, x, y, direction)

	ses.SendStreamJSON(map[string]interface{}{
		"mapId": mapID,
		"x":     x,
		"y":     y,
	}, opcodes.WarpTileTeleportNotify)

	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"mapId":   mapID,
		"x":       x,
		"y":       y,
		"message": "Warped home.",
	}, opcodes.WarpHomeResponse)
	log.Printf("[WarpHome] Warped char %d to map %d (%d,%d)", charID, mapID, x, y)

	return false
}
