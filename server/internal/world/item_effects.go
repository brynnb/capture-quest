package world

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
)

func tryHandleFieldItemUse(ses *session.Session, wh *WorldHandler, found *cqitems.CQInventoryItem, charID int32, req CQItemUseRequest) bool {
	item := found.Item
	switch itemuse.ShortName(item) {
	case "ESCAPE_ROPE":
		sendCQItemUseError(ses, "Use the current Escape Rope command.")
		return true
	case "OLD_ROD", "GOOD_ROD", "SUPER_ROD":
		fishingReq := map[string]interface{}{
			"itemId":  item.ID,
			"rodType": itemuse.ShortName(item),
		}
		if req.MapID != nil {
			fishingReq["mapId"] = *req.MapID
		}
		if req.X != nil {
			fishingReq["x"] = *req.X
		}
		if req.Y != nil {
			fishingReq["y"] = *req.Y
		}
		if strings.TrimSpace(req.Direction) != "" {
			fishingReq["direction"] = req.Direction
		}
		payload, _ := json.Marshal(fishingReq)
		HandlePokeFishing(ses, payload, wh)
		return true
	case "BICYCLE":
		sendCQItemUseError(ses, "Use the current Bicycle command.")
		return true
	case "TOWN_MAP":
		sendCQItemUseSuccess(ses, found, currentMapMessage(ses), found.Instance.Quantity)
		return true
	case "COIN_CASE":
		sendCQItemUseSuccess(ses, found, coinCaseMessage(getCoins(int64(charID))), found.Instance.Quantity)
		return true
	case "ITEMFINDER":
		sendCQItemUseSuccess(ses, found, itemfinderMessage(ses, wh), found.Instance.Quantity)
		return true
	case "POKEDEX":
		sendCQItemUseSuccess(ses, found, "You checked your Pokédex.", found.Instance.Quantity)
		return true
	case "EXP_ALL":
		sendCQItemUseSuccess(ses, found, "EXP.ALL is ready.", found.Instance.Quantity)
		return true
	default:
		return false
	}
}

func coinCaseMessage(coins int) string {
	if coins == 1 {
		return "You have 1 coin."
	}
	return fmt.Sprintf("You have %d coins.", coins)
}

func currentMapMessage(ses *session.Session) string {
	mapID := ses.MapID
	if ses.HasValidClient() && ses.Client.CharData() != nil {
		mapID = int(ses.Client.CharData().MapID)
	}
	var name string
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT name FROM phaser_maps WHERE id = $1`, mapID).Scan(&name); err == nil && name != "" {
		return "You're currently at " + name + "."
	}
	return "You checked the Town Map."
}

func itemfinderMessage(ses *session.Session, wh *WorldHandler) string {
	x, y, mapID := currentTilePosition(ses, wh)
	var count int
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT (
			SELECT COUNT(*) FROM phaser_hidden_items WHERE map_id = $1 AND ABS(x - $2) <= 7 AND ABS(y - $3) <= 7
		) + (
			SELECT COUNT(*) FROM phaser_hidden_coins WHERE map_id = $4 AND ABS(x - $5) <= 7 AND ABS(y - $6) <= 7
		) + (
			SELECT COUNT(*) FROM phaser_hidden_objects WHERE map_id = $7 AND ABS(x - $8) <= 7 AND ABS(y - $9) <= 7
		)
	`, mapID, x, y, mapID, x, y, mapID, x, y).Scan(&count)
	if err == nil && count > 0 {
		return "The ITEMFINDER's responding!"
	}
	return "Nope! There's no response."
}

func escapeRopeDestination(ses *session.Session, wh *WorldHandler) (int, int, int, error) {
	_, _, mapID := currentTilePosition(ses, wh)
	if wh != nil {
		return escapeRopeDestinationIn(wh.database, mapID)
	}
	return escapeRopeDestinationIn(db.GlobalWorldDB.DB, mapID)
}

func escapeRopeDestinationIn(database db.DBTX, mapID int) (int, int, int, error) {
	if mapID == UnifiedOverworldMapID {
		return 0, 0, 0, &itemuse.Rejection{Message: "Can't use that here"}
	}
	var isOverworld int
	if err := database.QueryRow(`SELECT COALESCE(is_overworld, 0) FROM phaser_maps WHERE id = $1`, mapID).Scan(&isOverworld); err != nil {
		return 0, 0, 0, fmt.Errorf("read escape map %d: %w", mapID, err)
	}
	if isOverworld != 0 {
		return 0, 0, 0, &itemuse.Rejection{Message: "Can't use that here"}
	}

	var destMapID, destX, destY int
	err := database.QueryRow(`
		SELECT pw.destination_map_id, pw.destination_x, pw.destination_y
		FROM phaser_warps pw
		LEFT JOIN phaser_maps pm ON pm.id = pw.destination_map_id
		WHERE pw.source_map_id = $1
			AND pw.destination_map_id IS NOT NULL
			AND pw.destination_x IS NOT NULL
			AND pw.destination_y IS NOT NULL
			AND COALESCE(pw.warp_type, 'door') NOT IN ('elevator', 'inactive')
		ORDER BY CASE WHEN COALESCE(pm.is_overworld, 0) = 1 OR pw.destination_map_id = $2 THEN 0 ELSE 1 END, pw.id
		LIMIT 1
	`, mapID, UnifiedOverworldMapID).Scan(&destMapID, &destX, &destY)
	if err == sql.ErrNoRows {
		return 0, 0, 0, &itemuse.Rejection{Message: "Can't use that here"}
	}
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read escape exit for map %d: %w", mapID, err)
	}
	return destMapID, destX, destY, nil
}

func currentTilePosition(ses *session.Session, wh *WorldHandler) (int, int, int) {
	charID := 0
	if ses.HasValidClient() && ses.Client.CharData() != nil {
		char := ses.Client.CharData()
		charID = int(char.ID)
	}
	if charID > 0 && wh != nil && wh.PlayerMovement != nil {
		if x, y, mapID, ok := wh.PlayerMovement.GetPosition(charID); ok {
			return x, y, mapID
		}
	}
	if ses.HasValidClient() && ses.Client.CharData() != nil {
		char := ses.Client.CharData()
		return int(math.Round(char.X)), int(math.Round(char.Y)), int(char.MapID)
	}
	return int(math.Round(float64(ses.X))), int(math.Round(float64(ses.Y))), ses.MapID
}

func teleportPlayerTo(ses *session.Session, wh *WorldHandler, mapID int, x int, y int) error {
	if _, err := setServerTeleportedPlayerPosition(ses, wh, mapID, x, y, "DOWN"); err != nil {
		return err
	}
	sendCommittedWarpNotification(ses, mapID, x, y, "DOWN")
	return nil
}

func publishCommittedTeleport(ses *session.Session, wh *WorldHandler, mapID, x, y int) {
	// The shared position transaction can end a Safari visit and clear its flags.
	if ses != nil && ses.HasValidClient() {
		refreshSafariFlags(wh, int64(ses.Client.CharData().ID))
	}
	publishCommittedPlayerPosition(ses, wh, mapID, x, y, "DOWN")
	sendCommittedWarpNotification(ses, mapID, x, y, "DOWN")
}

func sendCQItemUseSuccess(ses *session.Session, found *cqitems.CQInventoryItem, message string, newQty uint16, extra ...map[string]interface{}) {
	payload := map[string]interface{}{
		"success":    true,
		"message":    message,
		"instanceId": found.Instance.ID,
		"newQty":     newQty,
	}
	for _, fields := range extra {
		for key, value := range fields {
			payload[key] = StructToMap(value)
		}
	}
	ses.SendStreamJSON(payload, opcodes.CQItemUseResponse)
}

func sendCQItemUseError(ses *session.Session, err string) {
	ses.SendStreamJSON(map[string]interface{}{
		"success": false,
		"error":   err,
	}, opcodes.CQItemUseResponse)
}
