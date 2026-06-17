package world

import (
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
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
	sessionMapID := mapID
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		sessionMapID = UnifiedOverworldMapID
	}

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

	if _, err := db.GlobalWorldDB.DB.Exec(`
		UPDATE character_data
		SET map_id = $1,
		    x = $2,
		    y = $3,
		    z = $4,
		    heading = 0
		WHERE id = $5`,
		mapID,
		RecoverySpawnX,
		RecoverySpawnY,
		RecoverySpawnZ,
		charID,
	); err != nil {
		log.Printf("[WarpHome] Failed to persist home warp for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, opcodes.WarpHomeResponse)
		return false
	}

	ses.X = float32(x)
	ses.Y = float32(y)
	ses.MapID = sessionMapID
	char.X = float64(x)
	char.Y = float64(y)
	char.MapID = uint32(sessionMapID)

	if wh != nil && wh.PlayerMovement != nil {
		wh.PlayerMovement.UpdatePosition(int(charID), x, y, sessionMapID, direction)
	}

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
