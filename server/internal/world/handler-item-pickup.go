package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/session"
)

var errItemAlreadyCollected = errors.New("Already collected")

// HandleItemPickup handles a request to pick up an overworld item ball.
// The client sends the runtime actor ID; we reverse-map it to the DB object ID,
// verify it's an uncollected item object, add the item to the player's CQ inventory,
// mark it as collected, and respond with the item details.
func HandleItemPickup(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req struct {
		ActorID int `json:"actorId"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return false
	}
	fail := func(err error) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": err.Error()}, opcodes.ItemPickupResponse)
	}
	if wh.ActorRegistry == nil {
		fail(fmt.Errorf("Unknown actor"))
		return false
	}
	objectID := wh.ActorRegistry.GetOriginalID(ActorTypeNPC, req.ActorID)
	if objectID == 0 {
		fail(fmt.Errorf("Unknown actor"))
		return false
	}
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	actor, _, err := wh.scriptInteractionTargetContext(ctx, ses, objectID)
	if err != nil {
		fail(fmt.Errorf("Item unavailable or out of reach"))
		return false
	}
	// Item balls require immediate cardinal adjacency, even when a counter would
	// permit talking to a scripted NPC two tiles away.
	x, y, _ := wh.ownedPlayerPosition(ses)
	if !canReachItemForPickup(itemPickupTile{X: x, Y: y, MapID: actor.MapID}, itemPickupTile{X: *actor.X, Y: *actor.Y, MapID: actor.MapID}) {
		fail(fmt.Errorf("Move next to the item first."))
		return false
	}
	charID := int32(ses.Client.CharData().ID)
	result, err := collectItem(ctx, wh.database, charID, objectID)
	if err != nil {
		if errors.Is(err, errItemAlreadyCollected) {
			fail(errItemAlreadyCollected)
		} else {
			log.Printf("item pickup character %d object %d: %v", charID, objectID, err)
			fail(fmt.Errorf("Could not collect item"))
		}
		return false
	}
	ses.SendStreamJSON(map[string]interface{}{
		"success": true, "actorId": req.ActorID, "itemName": result.item.Name,
		"itemId": result.item.ID, "instanceId": result.instanceID, "message": itemPickupMessage(result.item.Name),
	}, opcodes.ItemPickupResponse)
	ses.SendStreamJSON(map[string]interface{}{"success": true, "items": result.inventory, "money": result.money}, opcodes.CQInventoryResponse)
	return false
}

type collectedItem struct {
	item       *cqitems.CQItem
	instanceID int32
	inventory  []cqitems.CQInventoryItem
	money      int64
}

// Collection identity and the grant share the same lock and commit. The snapshot
// comes from that transaction so publication never reloads through a global DB.
func collectItem(ctx context.Context, database *sql.DB, charID int32, objectID int) (*collectedItem, error) {
	result := &collectedItem{}
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		var objectType string
		var itemID sql.NullInt64
		if err := tx.QueryRow(`SELECT object_type,item_id FROM phaser_objects WHERE id=$1`, objectID).Scan(&objectType, &itemID); err != nil {
			return err
		}
		if objectType != "item" || !itemID.Valid {
			return fmt.Errorf("object %d has no item data", objectID)
		}
		var collected bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM character_collected_items WHERE character_id=$1 AND object_id=$2)`, charID, objectID).Scan(&collected); err != nil {
			return err
		}
		if collected {
			return errItemAlreadyCollected
		}
		store := cqitems.NewStore(tx)
		var err error
		result.item, err = store.GetItemByID(int32(itemID.Int64))
		if err != nil {
			return err
		}
		result.instanceID, err = store.AddItemToInventory(charID, int32(itemID.Int64), 1)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO character_collected_items(character_id,object_id) VALUES($1,$2)`, charID, objectID); err != nil {
			return err
		}
		result.inventory, err = store.GetCharacterInventory(charID)
		if err != nil {
			return err
		}
		result.money, err = store.GetCharacterMoney(charID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

type itemPickupTile struct {
	X     int
	Y     int
	MapID int
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

func itemPickupMessage(itemName string) string {
	if itemName == "" {
		itemName = "item"
	}
	return "Picked up " + itemName + "."
}
