package world

import (
	"context"
	"database/sql"
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

type ItemPickupRequest struct {
	RequestID string                    `json:"requestId"`
	ActorID   int                       `json:"actorId"`
	Command   *InventoryCommandIdentity `json:"command" tstype:"InventoryCommandIdentity"`
}
type ItemPickupResponse struct {
	Success     bool                        `json:"success" tstype:"true"`
	RequestID   string                      `json:"requestId"`
	CharacterID int64                       `json:"characterId"`
	ActorID     int                         `json:"actorId"`
	ItemName    string                      `json:"itemName"`
	ItemID      int32                       `json:"itemId"`
	InstanceID  int32                       `json:"instanceId"`
	Message     string                      `json:"message"`
	Inventory   cqitems.CQInventorySnapshot `json:"inventory" tstype:"import(\"./cqitems\").CQInventorySnapshot"`
}

// HandleItemPickup handles a request to pick up an overworld item ball.
// The client sends the runtime actor ID; we reverse-map it to the DB object ID,
// verify it's an uncollected item object, add the item to the player's CQ inventory,
// mark it as collected, and respond with the item details.
func HandleItemPickup(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req ItemPickupRequest
	fail := func(err error) {
		sendInventoryCommandError(ses, req.RequestID, opcodes.ItemPickupResponse, err.Error())
	}
	if err := decodePlayerMovement(payload, &req); err != nil || !validInventoryCommand(ses, req.RequestID, req.Command) || req.ActorID <= 0 {
		fail(fmt.Errorf("Invalid item pickup command."))
		return false
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
	charID := int32(ses.Client.CharData().ID)
	var result *collectedItem
	snapshot, err := cqitems.NewStore(wh.database).ExecuteCommand(ctx, charID, *req.Command.Revision, func(tx db.DBTX) error {
		var err error
		result, err = collectOwnedItem(ctx, tx, wh, ses, charID, objectID)
		return err
	})
	if err != nil {
		if errors.Is(err, errScriptInteractionDenied) {
			fail(fmt.Errorf("Item unavailable or out of reach"))
		} else if errors.Is(err, errBattleOwnership) {
			fail(err)
		} else if errors.Is(err, errItemAlreadyCollected) {
			fail(errItemAlreadyCollected)
		} else {
			log.Printf("item pickup character %d object %d: %v", charID, objectID, err)
			fail(fmt.Errorf("Could not collect item"))
		}
		return false
	}
	ses.SendStreamJSON(ItemPickupResponse{Success: true, RequestID: req.RequestID, CharacterID: int64(charID), ActorID: req.ActorID, ItemName: result.item.Name, ItemID: result.item.ID, InstanceID: result.instanceID, Message: itemPickupMessage(result.item.Name), Inventory: snapshot}, opcodes.ItemPickupResponse)

	notifyResourceChange(ses)
	return false
}

type collectedItem struct {
	item       *cqitems.CQItem
	instanceID int32
}

// Collection identity and the grant share the same lock and commit. The snapshot
// comes from that transaction so publication never reloads through a global DB.
func collectItem(ctx context.Context, database db.DBTX, charID int32, objectID int) (*collectedItem, error) {
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
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// The runtime owns source/visibility admission and award in one character lock.
func collectOwnedItem(ctx context.Context, database db.DBTX, wh *WorldHandler, ses *session.Session, charID int32, objectID int) (*collectedItem, error) {
	var result *collectedItem
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		if err := requireNoOwnedBattleIn(tx, int64(charID)); err != nil {
			return err
		}
		var source interactionPosition
		if err := tx.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=$1`, charID).Scan(&source.X, &source.Y, &source.MapID); err != nil {
			return err
		}
		flags, err := eventFlagSnapshotIn(tx, int64(charID))
		if err != nil {
			return err
		}
		actor, _, err := wh.scriptInteractionTargetFrom(ctx, tx.(db.ContextDBTX), ses, objectID, flags, source)
		if err != nil {
			return err
		}
		// Item balls require one cardinal tile even if NPC talk-over permits two.
		if !canReachItemForPickup(interactionPosition{X: source.X, Y: source.Y, MapID: actor.MapID}, interactionPosition{X: *actor.X, Y: *actor.Y, MapID: actor.MapID}) {
			return errScriptInteractionDenied
		}
		result, err = collectItem(ctx, tx, charID, objectID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func canReachItemForPickup(player, item interactionPosition) bool {
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
