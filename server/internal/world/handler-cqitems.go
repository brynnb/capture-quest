package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
)

// These tagged contracts replace map-shaped bag and shop mutation successes.
type CQInventoryResponse struct {
	Success bool                      `json:"success" tstype:"true"`
	Items   []cqitems.CQInventoryItem `json:"items" tstype:"import(\"./cqitems\").CQInventoryItem[]"`
	Money   int64                     `json:"money"`
}
type CQMerchantBuyResponse struct {
	Success    bool                        `json:"success" tstype:"true"`
	ItemID     int32                       `json:"itemId"`
	Quantity   uint16                      `json:"quantity"`
	InstanceID int32                       `json:"instanceId"`
	Money      int64                       `json:"money"`
	Inventory  cqitems.CQInventorySnapshot `json:"inventory" tstype:"import(\"./cqitems\").CQInventorySnapshot"`
}
type CQMerchantSellResponse struct {
	Success    bool                        `json:"success" tstype:"true"`
	InstanceID int32                       `json:"instanceId"`
	ItemName   string                      `json:"itemName"`
	SellPrice  int64                       `json:"sellPrice"`
	Money      int64                       `json:"money"`
	Inventory  cqitems.CQInventorySnapshot `json:"inventory" tstype:"import(\"./cqitems\").CQInventorySnapshot"`
}

// HandleCQInventoryRequest sends one coherent owned bag and balance.
func HandleCQInventoryRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	publishCQInventorySnapshot(ses, wh.database, int32(ses.Client.CharData().ID))
	return false
}

// HandleCQMerchantOpenRequest opens a merchant shop for the player.
// Supports opening by merchantId or mapId (for clicking clerk NPCs).
func HandleCQMerchantOpenRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req struct {
		MerchantID int32 `json:"merchantId"`
		MapID      int32 `json:"mapId"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[CQItems] Failed to unmarshal merchant open request: %v", err)
		return false
	}

	var merchant *cqitems.CQMerchant
	var err error

	if req.MerchantID > 0 {
		merchant, err = cqitems.NewStore(db.GlobalWorldDB.DB).GetMerchantByID(req.MerchantID)
	} else if req.MapID > 0 {
		// Look up merchant(s) by map ID — use the first one found
		merchants, merr := cqitems.NewStore(db.GlobalWorldDB.DB).GetMerchantsByMapID(req.MapID)
		if merr != nil || len(merchants) == 0 {
			log.Printf("[CQItems] No merchant on map %d: %v", req.MapID, merr)
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   "No shop on this map",
			}, opcodes.CQMerchantOpenResponse)
			return false
		}
		merchant = &merchants[0]
		err = nil
	} else {
		err = fmt.Errorf("no merchantId or mapId provided")
	}

	if err != nil || merchant == nil {
		log.Printf("[CQItems] Merchant not found: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Merchant not found",
		}, opcodes.CQMerchantOpenResponse)
		return false
	}

	// Collect items from all merchants on this map (for dept stores with multiple clerks)
	var allItems []cqitems.CQMerchantItem
	if req.MapID > 0 {
		merchants, _ := cqitems.NewStore(db.GlobalWorldDB.DB).GetMerchantsByMapID(req.MapID)
		for _, m := range merchants {
			items, _ := cqitems.NewStore(db.GlobalWorldDB.DB).GetMerchantItems(m.ID)
			allItems = append(allItems, items...)
		}
	} else {
		allItems, _ = cqitems.NewStore(db.GlobalWorldDB.DB).GetMerchantItems(merchant.ID)
	}

	money, _ := cqitems.NewStore(db.GlobalWorldDB.DB).GetCharacterMoney(int32(ses.Client.CharData().ID))

	ses.SendStreamJSON(map[string]interface{}{
		"success":    true,
		"merchantId": merchant.ID,
		"name":       merchant.Name,
		"items":      allItems,
		"money":      money,
	}, opcodes.CQMerchantOpenResponse)
	return false
}

// HandleCQMerchantBuyRequest handles buying an item from a merchant
func HandleCQMerchantBuyRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req struct {
		MerchantID int32  `json:"merchantId"`
		ItemID     int32  `json:"itemId"`
		Quantity   uint16 `json:"quantity"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[CQItems] Failed to unmarshal buy request: %v", err)
		return false
	}

	charID := int32(ses.Client.CharData().ID)
	purchase, err := wh.Economy.Buy(ses.CommandContext(), charID, int32(ses.MapID), req.MerchantID, req.ItemID, req.Quantity)
	if err != nil {
		log.Printf("[CQItems] Purchase failed for character %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Could not buy this item. Check the shop, quantity, and balance."}, opcodes.CQMerchantBuyResponse)
		return false
	}
	ses.SendStreamJSON(CQMerchantBuyResponse{Success: true, ItemID: purchase.ItemID,
		Quantity: purchase.Quantity, InstanceID: purchase.InstanceID, Money: purchase.Money,
		Inventory: purchase.Inventory}, opcodes.CQMerchantBuyResponse)
	// Compatibility publication is the same committed view; never reread after
	// commit or ask the browser to reconstruct a grant that may span stacks.
	sendCommittedCQInventory(ses, purchase.Inventory)
	return false
}

type cqItemUseRequest struct {
	InstanceID int32  `json:"instanceId"` // Item instance ID in inventory
	PartySlot  int    `json:"partySlot"`  // Target Pokémon party slot (0-5)
	MoveSlot   int    `json:"moveSlot"`   // For move-targeted items: which move slot (0-3), -1 otherwise
	MapID      *int   `json:"mapId,omitempty"`
	X          *int   `json:"x,omitempty"`
	Y          *int   `json:"y,omitempty"`
	Direction  string `json:"direction,omitempty"`
}

// HandleCQItemUse handles using an item from inventory outside of battle.
// Supports: potions (heal HP), status cures, revives, PP restores, Rare Candy.
func HandleCQItemUse(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req cqItemUseRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[CQItems] Failed to unmarshal item use request: %v", err)
		return false
	}

	charID := int32(ses.Client.CharData().ID)
	if battle := getBattle(int64(charID)); battle != nil && !battle.IsOver() {
		sendCQItemUseError(ses, "Use the battle item menu during a battle")
		return false
	}
	found, err := cqitems.NewStore(wh.database).FindInventoryItemByInstanceID(charID, req.InstanceID)
	if err != nil {
		sendCQItemUseError(ses, "Item not found in inventory")
		return false
	}
	if tryHandleFieldItemUse(ses, wh, found, charID, req) {
		return false
	}
	result, err := wh.Items.UsePartyItem(ses.CommandContext(), charID, req.InstanceID, req.PartySlot, req.MoveSlot)
	if err != nil {
		message := "Could not use this item. Please try again."
		var rejection *itemuse.Rejection
		if errors.As(err, &rejection) {
			message = rejection.Message
		} else {
			log.Printf("[CQItems] Item use failed for character %d instance %d: %v", charID, req.InstanceID, err)
		}
		sendCQItemUseError(ses, message)
		return false
	}
	ses.SendStreamJSON(result, opcodes.CQItemUseResponse)
	if !result.NeedsMoveSlot {
		sendPokemonPartySnapshot(ses, result.Party)
		sendCQInventorySnapshot(ses, charID)
	}
	return false
}

// HandleCQMerchantSellRequest handles selling an item to a merchant
func HandleCQMerchantSellRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req struct {
		InstanceID int32 `json:"instanceId"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[CQItems] Failed to unmarshal sell request: %v", err)
		return false
	}

	charID := int32(ses.Client.CharData().ID)
	sale, err := wh.Economy.Sell(ses.CommandContext(), charID, req.InstanceID)
	if err != nil {
		log.Printf("[CQItems] Sale failed for character %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Could not sell this item."}, opcodes.CQMerchantSellResponse)
		return false
	}
	ses.SendStreamJSON(CQMerchantSellResponse{Success: true, InstanceID: sale.InstanceID,
		ItemName: sale.ItemName, SellPrice: sale.SellPrice, Money: sale.Money,
		Inventory: sale.Inventory}, opcodes.CQMerchantSellResponse)
	sendCommittedCQInventory(ses, sale.Inventory)
	return false
}
