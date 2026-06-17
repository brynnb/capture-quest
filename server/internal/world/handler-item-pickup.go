package world

import (
	"database/sql"
	"encoding/json"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/session"
)

// HandleItemPickup handles a request to pick up an overworld item ball.
// The client sends the runtime actor ID; we reverse-map it to the DB object ID,
// verify it's an uncollected item object, add the item to the player's CQ inventory,
// mark it as collected, and respond with the item details.
func HandleItemPickup(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req struct {
		ActorID int `json:"actorId"` // Runtime actor ID (remapped by ActorRegistry)
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[ItemPickup] Failed to unmarshal request: %v", err)
		return false
	}

	charID := int32(ses.Client.CharData().ID)
	myDB := db.GlobalWorldDB.DB

	// Reverse-map runtime actor ID to original DB object ID
	dbID := wh.ActorRegistry.GetOriginalID(ActorTypeNPC, req.ActorID)
	if dbID == 0 {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Unknown actor",
		}, opcodes.ItemPickupResponse)
		return false
	}

	// Look up the object in phaser_objects
	var objectType string
	var itemID *int
	var objectName *string
	var objectX sql.NullInt64
	var objectY sql.NullInt64
	var objectMapID sql.NullInt64
	err := myDB.QueryRow(`
		SELECT object_type, item_id, name, COALESCE(x, local_x), COALESCE(y, local_y), map_id
		FROM phaser_objects
		WHERE id = $1`, dbID).Scan(&objectType, &itemID, &objectName, &objectX, &objectY, &objectMapID)
	if err != nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Object not found",
		}, opcodes.ItemPickupResponse)
		return false
	}

	if objectType != "item" {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Not an item",
		}, opcodes.ItemPickupResponse)
		return false
	}

	if itemID == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Item has no associated item data",
		}, opcodes.ItemPickupResponse)
		return false
	}

	objectTile, ok := itemPickupObjectTile(objectX, objectY, objectMapID, wh)
	if !ok {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Item position is unavailable",
		}, opcodes.ItemPickupResponse)
		return false
	}
	playerTile, ok := itemPickupPlayerTile(ses, wh, charID)
	if !ok {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Player position is unavailable",
		}, opcodes.ItemPickupResponse)
		return false
	}
	if !canReachItemForPickup(playerTile, objectTile) {
		log.Printf("[ItemPickup] Char %d tried to pick up %s from map %d (%d,%d) while at map %d (%d,%d)",
			charID, itemPickupObjectName(objectName), objectTile.MapID, objectTile.X, objectTile.Y,
			playerTile.MapID, playerTile.X, playerTile.Y)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Move next to the item first.",
		}, opcodes.ItemPickupResponse)
		return false
	}

	// Check if already collected
	var count int
	myDB.QueryRow(`
		SELECT COUNT(*) FROM character_collected_items
		WHERE character_id = $1 AND object_id = $2`, charID, dbID).Scan(&count)
	if count > 0 {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Already collected",
		}, opcodes.ItemPickupResponse)
		return false
	}

	// Look up the CQ item template
	item, err := cqitems.GetItemByID(int32(*itemID))
	if err != nil {
		log.Printf("[ItemPickup] Item template %d not found: %v", *itemID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Item data not found",
		}, opcodes.ItemPickupResponse)
		return false
	}

	// Add item to player's CQ inventory (quantity 1)
	instanceID, err := cqitems.AddItemToInventory(charID, int32(*itemID), 1)
	if err != nil {
		log.Printf("[ItemPickup] Failed to add item to inventory: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Failed to add item to inventory",
		}, opcodes.ItemPickupResponse)
		return false
	}

	// Mark as collected
	_, err = myDB.Exec(`
		INSERT INTO character_collected_items (character_id, object_id)
		VALUES ($1, $2)`, charID, dbID)
	if err != nil {
		log.Printf("[ItemPickup] Failed to mark item as collected: %v", err)
		// Item was already added to inventory, so still report success
	}

	log.Printf("[ItemPickup] Char %d picked up %s (object %d, item %d, instance %d)",
		charID, item.Name, dbID, *itemID, instanceID)

	ses.SendStreamJSON(map[string]interface{}{
		"success":    true,
		"actorId":    req.ActorID,
		"itemName":   item.Name,
		"itemId":     *itemID,
		"instanceId": instanceID,
		"message":    itemPickupMessage(item.Name),
	}, opcodes.ItemPickupResponse)

	// Refresh the player's CQ inventory
	inv, invErr := cqitems.GetCharacterInventory(charID)
	if invErr == nil {
		money, _ := cqitems.GetCharacterMoney(charID)
		ses.SendStreamJSON(map[string]interface{}{
			"success": true,
			"items":   inv,
			"money":   money,
		}, opcodes.CQInventoryResponse)
	}

	return false
}

type itemPickupTile struct {
	X     int
	Y     int
	MapID int
}

func itemPickupObjectTile(objectX, objectY, objectMapID sql.NullInt64, wh *WorldHandler) (itemPickupTile, bool) {
	if !objectX.Valid || !objectY.Valid || !objectMapID.Valid {
		return itemPickupTile{}, false
	}
	return itemPickupTile{
		X:     int(objectX.Int64),
		Y:     int(objectY.Int64),
		MapID: normalizeItemPickupMapID(int(objectMapID.Int64), wh),
	}, true
}

func itemPickupPlayerTile(ses *session.Session, wh *WorldHandler, charID int32) (itemPickupTile, bool) {
	if wh != nil && wh.PlayerMovement != nil {
		if x, y, mapID, ok := wh.PlayerMovement.GetPosition(int(charID)); ok {
			return itemPickupTile{
				X:     x,
				Y:     y,
				MapID: normalizeItemPickupMapID(mapID, wh),
			}, true
		}
	}
	if ses == nil {
		return itemPickupTile{}, false
	}

	x := int(ses.X)
	y := int(ses.Y)
	mapID := ses.MapID
	if ses.HasValidClient() {
		char := ses.Client.CharData()
		if char != nil {
			if mapID <= 0 {
				mapID = int(char.MapID)
			}
			if x == 0 && y == 0 && (char.X != 0 || char.Y != 0) {
				x = int(char.X)
				y = int(char.Y)
			}
		}
	}
	if mapID <= 0 {
		return itemPickupTile{}, false
	}
	return itemPickupTile{
		X:     x,
		Y:     y,
		MapID: normalizeItemPickupMapID(mapID, wh),
	}, true
}

func normalizeItemPickupMapID(mapID int, wh *WorldHandler) int {
	if mapID == UnifiedOverworldMapID {
		return UnifiedOverworldMapID
	}
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		return UnifiedOverworldMapID
	}
	return mapID
}

func canReachItemForPickup(player, item itemPickupTile) bool {
	if player.MapID != item.MapID {
		return false
	}
	dx := player.X - item.X
	if dx < 0 {
		dx = -dx
	}
	dy := player.Y - item.Y
	if dy < 0 {
		dy = -dy
	}
	return dx+dy == 1
}

func itemPickupObjectName(name *string) string {
	if name == nil || *name == "" {
		return "item"
	}
	return *name
}

func itemPickupMessage(itemName string) string {
	if itemName == "" {
		itemName = "item"
	}
	return "Picked up " + itemName + "."
}
