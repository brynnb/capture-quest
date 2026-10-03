package cqitems

// ItemTypeHM is the canonical importer category derived from source HM constants.
const ItemTypeHM = 6

// CQItem represents a row from cq_items (item template)
type CQItem struct {
	ID             int32   `json:"id"`
	Name           string  `json:"name"`
	ShortName      string  `json:"shortName"`
	Price          int32   `json:"price"`
	VendingPrice   *int32  `json:"vendingPrice,omitempty"`
	ItemType       uint8   `json:"itemType"`
	IsUsable       bool    `json:"isUsable"`
	UsesPartyMenu  bool    `json:"usesPartyMenu"`
	IsKeyItem      bool    `json:"isKeyItem"`
	IsGuardDrink   bool    `json:"isGuardDrink"`
	MoveID         *int32  `json:"moveId,omitempty"`
	Stackable      bool    `json:"stackable"`
	StackSize      int32   `json:"stackSize"`
	BonusHP        int32   `json:"bonusHp"`
	BonusAttack    int32   `json:"bonusAttack"`
	BonusDefense   int32   `json:"bonusDefense"`
	BonusSpeed     int32   `json:"bonusSpeed"`
	BonusSpecial   int32   `json:"bonusSpecial"`
	BonusAccuracy  int32   `json:"bonusAccuracy"`
	BonusEvasion   int32   `json:"bonusEvasion"`
	BonusCatchRate int32   `json:"bonusCatchRate"`
	BonusExp       int32   `json:"bonusExp"`
	BonusEncounter int32   `json:"bonusEncounterRate"`
	BonusCrit      int32   `json:"bonusCrit"`
	BonusFlee      int32   `json:"bonusFlee"`
	HealAmount     int32   `json:"healAmount"`
	StatusCure     *string `json:"statusCure,omitempty"`
	PPRestore      int32   `json:"ppRestore"`
	RevivePercent  int32   `json:"revivePercent"`
	BallModifier   float64 `json:"ballModifier"`
	LoreText       *string `json:"loreText,omitempty"`
	Icon           int32   `json:"icon"`
}

// CQItemInstance represents a row from cq_item_instances
type CQItemInstance struct {
	ID        int32  `json:"id"`
	ItemID    int32  `json:"itemId"`
	Charges   uint8  `json:"charges"`
	Quantity  uint16 `json:"quantity"`
	OwnerID   *int32 `json:"ownerId,omitempty"`
	OwnerType uint8  `json:"ownerType"`
}

// CQInventoryItem combines instance + template for client display
type CQInventoryItem struct {
	Instance CQItemInstance `json:"instance"`
	Item     CQItem         `json:"item"`
}

// CQInventorySnapshot is the complete owned bag and balance from one transaction.
type CQInventorySnapshot struct {
	CommandRevision int64             `json:"commandRevision"`
	Items           []CQInventoryItem `json:"items"`
	Money           int64             `json:"money"`
}

// CQMerchant represents a shop
type CQMerchant struct {
	ID      int32  `json:"id"`
	Name    string `json:"name"`
	MapName string `json:"mapName"`
}

// CQMerchantItem represents an item for sale
type CQMerchantItem struct {
	MerchantID    int32  `json:"merchantId"`
	ItemID        int32  `json:"itemId"`
	DisplayOrder  int32  `json:"displayOrder"`
	PriceOverride *int32 `json:"priceOverride,omitempty"`
	Quantity      int32  `json:"quantity"`
	Item          CQItem `json:"item"`
}
