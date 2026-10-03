package world

import (
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
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

	previousBattle := getBattle(charID)
	if err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if err := endSafariSessionIn(tx, charID); err != nil {
			return err
		}
		if err := pokebattle.DeleteBattleState(tx, charID); err != nil {
			return err
		}
		return saveFieldDestinationIn(tx, charID, mapID, x, y)
	}); err != nil {
		log.Printf("[WarpHome] Character %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "message": "Could not warp home. Please try again."}, opcodes.WarpHomeResponse)
		return false
	}
	forgetBattle(charID, previousBattle)
	refreshSafariFlags(wh, charID)
	publishCommittedPlayerPosition(ses, wh, mapID, x, y, direction)

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
