package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

// HandleCQInventoryRequest sends the player's full CQ inventory
func HandleCQInventoryRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	charID := int32(ses.Client.CharData().ID)
	items, err := cqitems.GetCharacterInventory(charID)
	if err != nil {
		log.Printf("[CQItems] Failed to get inventory for char %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Failed to load inventory",
		}, opcodes.CQInventoryResponse)
		return false
	}

	money, _ := cqitems.GetCharacterMoney(charID)

	log.Printf("[CQItems] Sending inventory response for char %d: %d items", charID, len(items))
	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"items":   items,
		"money":   money,
	}, opcodes.CQInventoryResponse)
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
		merchant, err = cqitems.GetMerchantByID(req.MerchantID)
	} else if req.MapID > 0 {
		// Look up merchant(s) by map ID — use the first one found
		merchants, merr := cqitems.GetMerchantsByMapID(req.MapID)
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
		merchants, _ := cqitems.GetMerchantsByMapID(req.MapID)
		for _, m := range merchants {
			items, _ := cqitems.GetMerchantItems(m.ID)
			allItems = append(allItems, items...)
		}
	} else {
		allItems, _ = cqitems.GetMerchantItems(merchant.ID)
	}

	money, _ := cqitems.GetCharacterMoney(int32(ses.Client.CharData().ID))

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

	if req.Quantity == 0 {
		req.Quantity = 1
	}

	charID := int32(ses.Client.CharData().ID)

	// Get item template
	item, err := cqitems.GetItemByID(req.ItemID)
	if err != nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Item not found",
		}, opcodes.CQMerchantBuyResponse)
		return false
	}

	// Calculate cost
	totalCost := int64(item.Price) * int64(req.Quantity)

	// Check money
	money, err := cqitems.GetCharacterMoney(charID)
	if err != nil || money < totalCost {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Not enough money",
		}, opcodes.CQMerchantBuyResponse)
		return false
	}

	// Deduct Pokédollars.
	remaining := money - totalCost

	ctx := context.Background()
	err = db_character.SetPokedollars(ctx, charID, remaining)
	if err != nil {
		log.Printf("[CQItems] Failed to deduct currency: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Payment failed",
		}, opcodes.CQMerchantBuyResponse)
		return false
	}

	// Add item to inventory
	instanceID, err := cqitems.AddItemToInventory(charID, req.ItemID, req.Quantity)
	if err != nil {
		log.Printf("[CQItems] Failed to add item to inventory: %v", err)
		// Refund money
		db_character.SetPokedollars(ctx, charID, money)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Could not add item",
		}, opcodes.CQMerchantBuyResponse)
		return false
	}

	log.Printf("[CQItems] Char %d bought %dx %s for %d¥",
		charID, req.Quantity, item.Name, totalCost)

	ses.SendStreamJSON(map[string]interface{}{
		"success":    true,
		"itemId":     req.ItemID,
		"quantity":   req.Quantity,
		"instanceId": instanceID,
		"money":      remaining,
		"item":       item,
	}, opcodes.CQMerchantBuyResponse)
	return false
}

type cqItemUseRequest struct {
	InstanceID int32 `json:"instanceId"` // Item instance ID in inventory
	PartySlot  int   `json:"partySlot"`  // Target Pokémon party slot (0-5)
	MoveSlot   int   `json:"moveSlot"`   // For move-targeted items: which move slot (0-3), -1 otherwise
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
	myDB := db.GlobalWorldDB.DB

	// Load the item instance from inventory
	inv, err := cqitems.GetCharacterInventory(charID)
	if err != nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Failed to load inventory",
		}, opcodes.CQItemUseResponse)
		return false
	}

	var found *cqitems.CQInventoryItem
	for i := range inv {
		if inv[i].Instance.ID == req.InstanceID {
			found = &inv[i]
			break
		}
	}
	if found == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Item not found in inventory",
		}, opcodes.CQItemUseResponse)
		return false
	}

	item := found.Item
	if tryHandleFieldItemUse(ses, wh, found, charID) {
		return false
	}

	if !item.IsUsable {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "That item can't be used like that.",
		}, opcodes.CQItemUseResponse)
		return false
	}

	if !itemUsableOnPartyOutsideBattle(item) {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "That item can't be used outside of battle",
		}, opcodes.CQItemUseResponse)
		return false
	}

	// Load party
	party, err := pokebattle.LoadParty(myDB, int64(charID))
	if err != nil || len(party) == 0 {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Failed to load party",
		}, opcodes.CQItemUseResponse)
		return false
	}

	if req.PartySlot < 0 || req.PartySlot >= len(party) || party[req.PartySlot] == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Invalid Pokémon",
		}, opcodes.CQItemUseResponse)
		return false
	}

	targetPoke := party[req.PartySlot]
	var msg string

	if isTMHM(item) {
		if item.MoveID == nil {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   "Invalid TM/HM (no move associated)",
			}, opcodes.CQItemUseResponse)
			return false
		}
		moveID := int(*item.MoveID)
		moveName := cqMoveNameForID(myDB, moveID, item.Name)

		// Check compatibility
		if !pokebattle.CanLearnTMHM(myDB, targetPoke.ID, moveID) {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("%s can't learn that move", targetPoke.Name),
			}, opcodes.CQItemUseResponse)
			return false
		}

		// Check if already knows the move
		for _, m := range targetPoke.Moves {
			if m.ID == moveID {
				ses.SendStreamJSON(map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("%s already knows %s", targetPoke.Name, m.Name),
				}, opcodes.CQItemUseResponse)
				return false
			}
		}

		// Try to learn (auto-fills empty slot)
		result := pokebattle.TryLearnMove(myDB, targetPoke, moveID)
		if result >= 0 {
			// Learned into empty slot
			moveName := targetPoke.Moves[result].Name
			msg = fmt.Sprintf("%s learned %s!", targetPoke.Name, moveName)
		} else if result == -1 {
			// All 4 slots full — need player to pick a move to forget
			if req.MoveSlot < 0 {
				// Client needs to show move selection modal
				ses.SendStreamJSON(tmhmNeedsMoveSlotResponse(req, targetPoke.Name, item.Name, moveName, moveID), opcodes.CQItemUseResponse)
				return false
			}
			// Player chose a move to forget — check it's not an HM move
			if req.MoveSlot < 0 || req.MoveSlot >= 4 {
				ses.SendStreamJSON(map[string]interface{}{
					"success": false,
					"error":   "Invalid move slot",
				}, opcodes.CQItemUseResponse)
				return false
			}
			forgottenMove := targetPoke.Moves[req.MoveSlot]
			if pokebattle.IsHMMove(myDB, forgottenMove.ID) {
				ses.SendStreamJSON(map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("HM moves can't be forgotten! (%s)", forgottenMove.Name),
				}, opcodes.CQItemUseResponse)
				return false
			}
			if err := pokebattle.ForgetAndLearnMove(myDB, targetPoke, req.MoveSlot, moveID); err != nil {
				ses.SendStreamJSON(map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("Failed to learn move: %v", err),
				}, opcodes.CQItemUseResponse)
				return false
			}
			newMoveName := targetPoke.Moves[req.MoveSlot].Name
			msg = fmt.Sprintf("1, 2, and… Poof!\n%s forgot %s.\nAnd…\n%s learned %s!", targetPoke.Name, forgottenMove.Name, targetPoke.Name, newMoveName)
		} else {
			// -2 = already knows (shouldn't reach here due to check above)
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("%s already knows that move", targetPoke.Name),
			}, opcodes.CQItemUseResponse)
			return false
		}

		// Consume item: TMs are consumed, HMs are not
		if item.ItemType == cqItemTypeTM {
			cqitems.DecrementItemQuantity(charID, req.InstanceID)
		}

		// Save party
		pokebattle.SaveParty(myDB, int64(charID), party)

		log.Printf("[CQItems] Char %d used %s on %s: %s", charID, item.Name, targetPoke.Name, msg)

		ses.SendStreamJSON(map[string]interface{}{
			"success":    true,
			"message":    msg,
			"instanceId": req.InstanceID,
			"partySlot":  req.PartySlot,
		}, opcodes.CQItemUseResponse)

		sendPartyUpdate(ses)
		return false
	}

	if isRareCandy(item) {
		// Rare Candy: level up by 1
		if targetPoke.IsFainted() {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("%s has fainted", targetPoke.Name),
			}, opcodes.CQItemUseResponse)
			return false
		}
		if targetPoke.Level >= 100 {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("%s is already at max level", targetPoke.Name),
			}, opcodes.CQItemUseResponse)
			return false
		}
		oldMaxHP := targetPoke.MaxHP
		targetPoke.Level++
		targetPoke.Exp = pokebattle.ExpForLevel(targetPoke.GrowthRt, targetPoke.Level)
		targetPoke.RecalculateStats()
		targetPoke.CurHP += targetPoke.MaxHP - oldMaxHP
		msg = fmt.Sprintf("%s grew to level %d!", targetPoke.Name, targetPoke.Level)

		// Check evolution
		evolvedID, evolvedName := pokebattle.CheckEvolution(myDB, targetPoke)
		if evolvedID > 0 {
			oldName := targetPoke.Name
			if err := pokebattle.EvolvePokemon(myDB, targetPoke, evolvedID); err != nil {
				log.Printf("[CQItems] Failed to evolve %s: %v", oldName, err)
			} else {
				msg += fmt.Sprintf("\nWhat? %s is evolving!\n%s evolved into %s!", oldName, oldName, evolvedName)
			}
		}

		// Check for new moves
		newMoves, _ := pokebattle.GetMovesLearnedInRange(myDB, targetPoke.ID, targetPoke.Level-1, targetPoke.Level)
		for _, lm := range newMoves {
			result := pokebattle.TryLearnMove(myDB, targetPoke, lm.MoveID)
			if result >= 0 {
				msg += fmt.Sprintf("\n%s learned %s!", targetPoke.Name, lm.MoveName)
			}
			// If all slots full, skip (no prompt outside battle for now)
		}
	} else if isEvolutionStone(item) {
		var applyErr error
		msg, applyErr = applyStoneEvolution(myDB, item, targetPoke)
		if applyErr != nil {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   applyErr.Error(),
			}, opcodes.CQItemUseResponse)
			return false
		}
	} else if _, _, ok := vitaminTarget(item); ok {
		var applyErr error
		msg, applyErr = applyVitamin(item, targetPoke)
		if applyErr != nil {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   applyErr.Error(),
			}, opcodes.CQItemUseResponse)
			return false
		}
	} else if isPPUp(item) {
		var applyErr error
		msg, applyErr = applyPPUp(targetPoke, req.MoveSlot)
		if applyErr != nil {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   applyErr.Error(),
			}, opcodes.CQItemUseResponse)
			return false
		}
	} else {
		// Medicine item
		eff := medicineEffectFromItem(item)
		var applyErr error
		msg, applyErr = pokebattle.ApplyItemEffect(targetPoke, eff, req.MoveSlot)
		if applyErr != nil {
			ses.SendStreamJSON(map[string]interface{}{
				"success": false,
				"error":   applyErr.Error(),
			}, opcodes.CQItemUseResponse)
			return false
		}
	}

	// Consume the item
	newQty, _ := cqitems.DecrementItemQuantity(charID, req.InstanceID)

	// Save party
	pokebattle.SaveParty(myDB, int64(charID), party)

	log.Printf("[CQItems] Char %d used %s on %s: %s", charID, item.Name, targetPoke.Name, msg)

	ses.SendStreamJSON(map[string]interface{}{
		"success":    true,
		"message":    msg,
		"instanceId": req.InstanceID,
		"newQty":     newQty,
		"partySlot":  req.PartySlot,
	}, opcodes.CQItemUseResponse)

	// Push updated party to client
	sendPartyUpdate(ses)
	return false
}

func tmhmNeedsMoveSlotResponse(req cqItemUseRequest, pokemonName, itemName, moveName string, moveID int) map[string]interface{} {
	if moveName == "" {
		moveName = itemName
	}
	return map[string]interface{}{
		"success":       true,
		"needsMoveSlot": true,
		"instanceId":    req.InstanceID,
		"partySlot":     req.PartySlot,
		"moveId":        moveID,
		"moveName":      moveName,
		"message":       fmt.Sprintf("%s wants to learn %s, but already knows 4 moves. Choose a move to forget.", pokemonName, moveName),
	}
}

func cqMoveNameForID(db pokebattle.DBTX, moveID int, fallback string) string {
	var moveName string
	if err := db.QueryRow(`
		SELECT COALESCE(NULLIF(short_name, ''), NULLIF(name, ''), $1)
		FROM phaser_moves WHERE id = $2`, fallback, moveID).Scan(&moveName); err != nil || moveName == "" {
		return fallback
	}
	return moveName
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

	// Look up the item
	inv, err := cqitems.GetCharacterInventory(charID)
	if err != nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Failed to load inventory",
		}, opcodes.CQMerchantSellResponse)
		return false
	}

	var found *cqitems.CQInventoryItem
	for i := range inv {
		if inv[i].Instance.ID == req.InstanceID {
			found = &inv[i]
			break
		}
	}

	if found == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Item not found in inventory",
		}, opcodes.CQMerchantSellResponse)
		return false
	}

	// Can't sell key items
	if found.Item.IsKeyItem {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Cannot sell key items",
		}, opcodes.CQMerchantSellResponse)
		return false
	}

	// Sell price = price / 2
	sellPrice := int64(found.Item.Price) / 2
	if sellPrice <= 0 {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "This item has no value",
		}, opcodes.CQMerchantSellResponse)
		return false
	}

	// Remove item
	err = cqitems.RemoveItemFromInventory(charID, req.InstanceID)
	if err != nil {
		log.Printf("[CQItems] Failed to remove item: %v", err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "Failed to remove item",
		}, opcodes.CQMerchantSellResponse)
		return false
	}

	// Add money
	money, _ := cqitems.GetCharacterMoney(charID)
	newTotal := money + sellPrice
	db_character.SetPokedollars(context.Background(), charID, newTotal)

	log.Printf("[CQItems] Char %d sold %s for %d¥", charID, found.Item.Name, sellPrice)

	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"itemName":  found.Item.Name,
		"sellPrice": sellPrice,
		"money":     newTotal,
	}, opcodes.CQMerchantSellResponse)
	return false
}
