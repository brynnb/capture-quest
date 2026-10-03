package world

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
)

type ShopCommandIdentity struct {
	CharacterID int64  `json:"characterId"`
	Revision    *int64 `json:"revision" tstype:"number"`
}
type CQMerchantBuyRequest struct {
	ActorID    int                  `json:"actorId"`
	RequestID  string               `json:"requestId"`
	Shop       *ShopCommandIdentity `json:"shop" tstype:"ShopCommandIdentity"`
	MerchantID int32                `json:"merchantId"`
	ItemID     int32                `json:"itemId"`
	Quantity   uint16               `json:"quantity"`
}
type CQMerchantSellRequest struct {
	ActorID    int                  `json:"actorId"`
	RequestID  string               `json:"requestId"`
	Shop       *ShopCommandIdentity `json:"shop" tstype:"ShopCommandIdentity"`
	InstanceID int32                `json:"instanceId"`
}
type ShopCommandError struct {
	Success   bool   `json:"success" tstype:"false"`
	RequestID string `json:"requestId"`
	Error     string `json:"error"`
}

func validShopCommand(ses *session.Session, requestID string, identity *ShopCommandIdentity) bool {
	return requestID != "" && len(requestID) <= 64 && identity != nil && identity.Revision != nil && *identity.Revision >= 0 && identity.CharacterID == int64(ses.Client.CharData().ID)
}
func sendShopCommandError(ses *session.Session, requestID string, opcode opcodes.OpCode, message string) {
	if len(requestID) > 64 {
		requestID = ""
	}
	ses.SendStreamJSON(ShopCommandError{RequestID: requestID, Error: message}, opcode)
}

// These tagged contracts replace map-shaped bag and shop mutation successes.
type CQInventoryResponse struct {
	ShopRevision int64                     `json:"shopRevision"`
	Success      bool                      `json:"success" tstype:"true"`
	Items        []cqitems.CQInventoryItem `json:"items" tstype:"import(\"./cqitems\").CQInventoryItem[]"`
	Money        int64                     `json:"money"`
}
type CQMerchantBuyResponse struct {
	RequestID  string                      `json:"requestId"`
	Success    bool                        `json:"success" tstype:"true"`
	ItemID     int32                       `json:"itemId"`
	Quantity   uint16                      `json:"quantity"`
	InstanceID int32                       `json:"instanceId"`
	Money      int64                       `json:"money"`
	Inventory  cqitems.CQInventorySnapshot `json:"inventory" tstype:"import(\"./cqitems\").CQInventorySnapshot"`
}
type CQMerchantSellResponse struct {
	RequestID  string                      `json:"requestId"`
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

type CQMerchantOpenRequest struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	ActorID     int    `json:"actorId"`
}
type CQMerchantOpenResponse struct {
	Success     bool                     `json:"success" tstype:"true"`
	RequestID   string                   `json:"requestId"`
	CharacterID int64                    `json:"characterId"`
	MerchantID  int32                    `json:"merchantId"`
	Name        string                   `json:"name"`
	Items       []cqitems.CQMerchantItem `json:"items" tstype:"import(\"./cqitems\").CQMerchantItem[]"`
	Money       int64                    `json:"money"`
}

// Opening a menu does not grant a reusable permission. Revalidate this same
// source boundary for every command, including after movement or flag changes.
func (wh *WorldHandler) authorizeMerchantInteraction(ctx context.Context, ses *session.Session, actorID int) (PhaserActor, error) {
	if actorID <= 0 {
		return PhaserActor{}, errors.New("Invalid shop actor")
	}
	charID := int64(ses.Client.CharData().ID)
	// Match field-item admission: world shop commands cannot race the active
	// battle's inventory/money decisions merely because the UI hides the clerk.
	if battle := getBattle(charID); battle != nil && !battle.IsOver() {
		return PhaserActor{}, errors.New("Shop unavailable during battle")
	}
	if wh.Economy == nil || wh.database == nil || wh.ActorRegistry == nil || wh.Cutscenes == nil {
		return PhaserActor{}, errors.New("Shop unavailable")
	}
	flags := NewEventFlagManager(wh.database)
	if err := flags.LoadFlagsContext(ctx, charID); err != nil {
		return PhaserActor{}, errors.New("Shop eligibility unavailable")
	}
	objectID := wh.ActorRegistry.GetOriginalID(ActorTypeNPC, actorID)
	if objectID == 0 {
		return PhaserActor{}, errors.New("Unknown shop actor")
	}
	actor, mapName, err := wh.scriptInteractionTargetWithFlags(ctx, ses, objectID, flags)
	if err != nil || actor.SpriteName == nil || *actor.SpriteName != "SPRITE_CLERK" {
		return PhaserActor{}, errors.New("Shop actor unavailable or out of reach")
	}
	facing := ""
	if wh.PlayerMovement != nil {
		facing, _ = wh.PlayerMovement.GetDirection(int(charID))
	}
	script, err := wh.Cutscenes.FindEligibleClickCutsceneContext(ctx, mapName, scriptedEventTriggerKeys(actor), charID, flags, facing)
	if err != nil {
		return PhaserActor{}, errors.New("Shop eligibility unavailable")
	}
	if script != nil {
		return PhaserActor{}, errors.New("Complete the clerk interaction first")
	}
	return actor, nil
}

// Merchant opening is the fallback after source scripted interaction, not an
// alternate way to skip an eligible clerk script or select a remote map.
func HandleCQMerchantOpenRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req CQMerchantOpenRequest
	fail := func(message string) {
		sendShopCommandError(ses, req.RequestID, opcodes.CQMerchantOpenResponse, message)
	}
	if err := decodePlayerMovement(payload, &req); err != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != int64(ses.Client.CharData().ID) || req.ActorID <= 0 {
		fail("Invalid shop interaction")
		return false
	}
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	actor, err := wh.authorizeMerchantInteraction(ctx, ses, req.ActorID)
	if err != nil {
		fail(err.Error())
		return false
	}
	menu, err := wh.Economy.Open(ctx, int32(req.CharacterID), int32(actor.MapID), 0)
	if err != nil {
		log.Printf("[CQItems] Merchant read failed: %v", err)
		fail("Could not read this shop")
		return false
	}
	ses.SendStreamJSON(CQMerchantOpenResponse{Success: true, RequestID: req.RequestID, CharacterID: req.CharacterID,
		MerchantID: menu.MerchantID, Name: menu.Name, Items: menu.Items, Money: menu.Money}, opcodes.CQMerchantOpenResponse)
	return false
}

// HandleCQMerchantBuyRequest handles buying an item from a merchant
func HandleCQMerchantBuyRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req CQMerchantBuyRequest
	if err := decodePlayerMovement(payload, &req); err != nil || !validShopCommand(ses, req.RequestID, req.Shop) {
		sendShopCommandError(ses, req.RequestID, opcodes.CQMerchantBuyResponse, "Invalid shop command identity.")
		return false
	}

	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	actor, err := wh.authorizeMerchantInteraction(ctx, ses, req.ActorID)
	if err != nil {
		sendShopCommandError(ses, req.RequestID, opcodes.CQMerchantBuyResponse, err.Error())
		return false
	}
	charID := int32(ses.Client.CharData().ID)
	purchase, err := wh.Economy.Buy(ctx, charID, int32(actor.MapID), req.MerchantID, req.ItemID, req.Quantity, *req.Shop.Revision)
	if err != nil {
		log.Printf("[CQItems] Purchase failed for character %d: %v", charID, err)
		sendShopCommandError(ses, req.RequestID, opcodes.CQMerchantBuyResponse, "Could not buy this item. Read current inventory before trying again.")
		return false
	}
	ses.SendStreamJSON(CQMerchantBuyResponse{RequestID: req.RequestID, Success: true, ItemID: purchase.ItemID,
		Quantity: purchase.Quantity, InstanceID: purchase.InstanceID, Money: purchase.Money,
		Inventory: purchase.Inventory}, opcodes.CQMerchantBuyResponse)
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
	var req CQMerchantSellRequest
	if err := decodePlayerMovement(payload, &req); err != nil || !validShopCommand(ses, req.RequestID, req.Shop) {
		sendShopCommandError(ses, req.RequestID, opcodes.CQMerchantSellResponse, "Invalid shop command identity.")
		return false
	}

	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	actor, err := wh.authorizeMerchantInteraction(ctx, ses, req.ActorID)
	if err != nil {
		sendShopCommandError(ses, req.RequestID, opcodes.CQMerchantSellResponse, err.Error())
		return false
	}
	charID := int32(ses.Client.CharData().ID)
	sale, err := wh.Economy.Sell(ctx, charID, int32(actor.MapID), req.InstanceID, *req.Shop.Revision)
	if err != nil {
		log.Printf("[CQItems] Sale failed for character %d: %v", charID, err)
		sendShopCommandError(ses, req.RequestID, opcodes.CQMerchantSellResponse, "Could not sell this item. Read current inventory before trying again.")
		return false
	}
	ses.SendStreamJSON(CQMerchantSellResponse{RequestID: req.RequestID, Success: true, InstanceID: sale.InstanceID,
		ItemName: sale.ItemName, SellPrice: sale.SellPrice, Money: sale.Money,
		Inventory: sale.Inventory}, opcodes.CQMerchantSellResponse)
	return false
}
