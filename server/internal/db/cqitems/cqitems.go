package cqitems

import (
	"database/sql"
	"fmt"
	"log"

	"capturequest/internal/db"
)

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

// CQMerchant represents a shop
type CQMerchant struct {
	ID      int32  `json:"id"`
	Name    string `json:"name"`
	MapName string `json:"mapName"`
}

// CQMerchantItem represents an item for sale
type CQMerchantItem struct {
	ItemID        int32  `json:"itemId"`
	DisplayOrder  int32  `json:"displayOrder"`
	PriceOverride *int32 `json:"priceOverride,omitempty"`
	Quantity      int32  `json:"quantity"`
	Item          CQItem `json:"item"`
}

// itemCache caches item templates by ID
var itemCache = map[int32]*CQItem{}

func nullableInt32Ptr(v sql.NullInt64) *int32 {
	if !v.Valid {
		return nil
	}
	n := int32(v.Int64)
	return &n
}

func nullableStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
}

func assignNullableItemFields(item *CQItem, vendingPrice sql.NullInt64, moveID sql.NullInt64, statusCure sql.NullString, loreText sql.NullString) {
	item.VendingPrice = nullableInt32Ptr(vendingPrice)
	item.MoveID = nullableInt32Ptr(moveID)
	item.StatusCure = nullableStringPtr(statusCure)
	item.LoreText = nullableStringPtr(loreText)
}

// GetItemByID returns an item template by ID (cached)
func GetItemByID(itemID int32) (*CQItem, error) {
	if cached, ok := itemCache[itemID]; ok {
		return cached, nil
	}

	item := &CQItem{}
	var vendingPrice, moveID sql.NullInt64
	var statusCure, loreText sql.NullString
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, name, short_name, price, vending_price, item_type,
			is_usable, uses_party_menu, is_key_item, is_guard_drink, move_id,
			stackable, stack_size,
			bonus_hp, bonus_attack, bonus_defense, bonus_speed, bonus_special,
			bonus_accuracy, bonus_evasion, bonus_catch_rate, bonus_exp,
			bonus_encounter_rate, bonus_crit, bonus_flee,
			heal_amount, status_cure, pp_restore, revive_percent, ball_modifier,
			lore_text, icon
		FROM cq_items WHERE id = $1
	`, itemID).Scan(
		&item.ID, &item.Name, &item.ShortName, &item.Price, &vendingPrice,
		&item.ItemType,
		&item.IsUsable, &item.UsesPartyMenu, &item.IsKeyItem, &item.IsGuardDrink, &moveID,
		&item.Stackable, &item.StackSize,
		&item.BonusHP, &item.BonusAttack, &item.BonusDefense, &item.BonusSpeed, &item.BonusSpecial,
		&item.BonusAccuracy, &item.BonusEvasion, &item.BonusCatchRate, &item.BonusExp,
		&item.BonusEncounter, &item.BonusCrit, &item.BonusFlee,
		&item.HealAmount, &statusCure, &item.PPRestore, &item.RevivePercent, &item.BallModifier,
		&loreText, &item.Icon,
	)
	if err != nil {
		return nil, fmt.Errorf("GetItemByID(%d): %w", itemID, err)
	}
	assignNullableItemFields(item, vendingPrice, moveID, statusCure, loreText)

	itemCache[itemID] = item
	return item, nil
}

// GetCharacterInventory returns all inventory items for a character
func GetCharacterInventory(charID int32) ([]CQInventoryItem, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT ii.id, ii.item_id, ii.charges, ii.quantity, ii.owner_type,
			i.id, i.name, i.short_name, i.price, i.vending_price, i.item_type,
			i.is_usable, i.uses_party_menu, i.is_key_item, i.is_guard_drink, i.move_id,
			i.stackable, i.stack_size,
			i.bonus_hp, i.bonus_attack, i.bonus_defense, i.bonus_speed, i.bonus_special,
			i.bonus_accuracy, i.bonus_evasion, i.bonus_catch_rate, i.bonus_exp,
			i.bonus_encounter_rate, i.bonus_crit, i.bonus_flee,
			i.heal_amount, i.status_cure, i.pp_restore, i.revive_percent, i.ball_modifier,
			i.lore_text, i.icon
		FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		JOIN cq_items i ON i.id = ii.item_id
		WHERE ci.character_id = $1
		ORDER BY i.name, ii.item_id, ii.id
	`, charID)
	if err != nil {
		return nil, fmt.Errorf("GetCharacterInventory: %w", err)
	}
	defer rows.Close()

	var items []CQInventoryItem
	for rows.Next() {
		var inv CQInventoryItem
		var vendingPrice, moveID sql.NullInt64
		var statusCure, loreText sql.NullString
		err := rows.Scan(
			&inv.Instance.ID, &inv.Instance.ItemID, &inv.Instance.Charges, &inv.Instance.Quantity, &inv.Instance.OwnerType,
			&inv.Item.ID, &inv.Item.Name, &inv.Item.ShortName, &inv.Item.Price, &vendingPrice,
			&inv.Item.ItemType,
			&inv.Item.IsUsable, &inv.Item.UsesPartyMenu, &inv.Item.IsKeyItem, &inv.Item.IsGuardDrink, &moveID,
			&inv.Item.Stackable, &inv.Item.StackSize,
			&inv.Item.BonusHP, &inv.Item.BonusAttack, &inv.Item.BonusDefense, &inv.Item.BonusSpeed, &inv.Item.BonusSpecial,
			&inv.Item.BonusAccuracy, &inv.Item.BonusEvasion, &inv.Item.BonusCatchRate, &inv.Item.BonusExp,
			&inv.Item.BonusEncounter, &inv.Item.BonusCrit, &inv.Item.BonusFlee,
			&inv.Item.HealAmount, &statusCure, &inv.Item.PPRestore, &inv.Item.RevivePercent, &inv.Item.BallModifier,
			&loreText, &inv.Item.Icon,
		)
		if err != nil {
			log.Printf("GetCharacterInventory scan error: %v", err)
			continue
		}
		assignNullableItemFields(&inv.Item, vendingPrice, moveID, statusCure, loreText)
		items = append(items, inv)
	}
	return items, nil
}

// GetMerchantByMapName finds a merchant by map name
func GetMerchantByMapName(mapName string) (*CQMerchant, error) {
	m := &CQMerchant{}
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, name, COALESCE(map_name, '') FROM cq_merchants WHERE map_name = $1
	`, mapName).Scan(&m.ID, &m.Name, &m.MapName)
	if err != nil {
		return nil, fmt.Errorf("GetMerchantByMapName(%s): %w", mapName, err)
	}
	return m, nil
}

// GetMerchantByID finds a merchant by ID
func GetMerchantByID(merchantID int32) (*CQMerchant, error) {
	m := &CQMerchant{}
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, name, COALESCE(map_name, '') FROM cq_merchants WHERE id = $1
	`, merchantID).Scan(&m.ID, &m.Name, &m.MapName)
	if err != nil {
		return nil, fmt.Errorf("GetMerchantByID(%d): %w", merchantID, err)
	}
	return m, nil
}

// GetMerchantsByMapID finds all merchants on a given map
func GetMerchantsByMapID(mapID int32) ([]CQMerchant, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT cm.id, cm.name, COALESCE(cm.map_name, '')
		FROM cq_merchants cm
		LEFT JOIN phaser_maps pm ON pm.id = $1
		WHERE cm.map_id = $1
		   OR (
				pm.name IS NOT NULL
				AND regexp_replace(lower(COALESCE(cm.map_name, '')), '[^a-z0-9]', '', 'g')
					= regexp_replace(lower(pm.name), '[^a-z0-9]', '', 'g')
		   )
		ORDER BY cm.id
	`, mapID)
	if err != nil {
		return nil, fmt.Errorf("GetMerchantsByMapID(%d): %w", mapID, err)
	}
	defer rows.Close()

	var merchants []CQMerchant
	for rows.Next() {
		var m CQMerchant
		if err := rows.Scan(&m.ID, &m.Name, &m.MapName); err != nil {
			return nil, fmt.Errorf("GetMerchantsByMapID(%d) scan: %w", mapID, err)
		}
		merchants = append(merchants, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetMerchantsByMapID(%d) rows: %w", mapID, err)
	}
	return merchants, nil
}

// GetMerchantItems returns all items for sale at a merchant
func GetMerchantItems(merchantID int32) ([]CQMerchantItem, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT mi.item_id, mi.display_order, mi.price_override, mi.quantity,
			i.id, i.name, i.short_name, i.price, i.vending_price, i.item_type,
			i.is_usable, i.uses_party_menu, i.is_key_item, i.is_guard_drink, i.move_id,
			i.stackable, i.stack_size,
			i.bonus_hp, i.bonus_attack, i.bonus_defense, i.bonus_speed, i.bonus_special,
			i.bonus_accuracy, i.bonus_evasion, i.bonus_catch_rate, i.bonus_exp,
			i.bonus_encounter_rate, i.bonus_crit, i.bonus_flee,
			i.heal_amount, i.status_cure, i.pp_restore, i.revive_percent, i.ball_modifier,
			i.lore_text, i.icon
		FROM cq_merchant_items mi
		JOIN cq_items i ON i.id = mi.item_id
		WHERE mi.merchant_id = $1
		ORDER BY mi.display_order
	`, merchantID)
	if err != nil {
		return nil, fmt.Errorf("GetMerchantItems(%d): %w", merchantID, err)
	}
	defer rows.Close()

	var items []CQMerchantItem
	for rows.Next() {
		var mi CQMerchantItem
		var priceOverride, vendingPrice, moveID sql.NullInt64
		var statusCure, loreText sql.NullString
		err := rows.Scan(
			&mi.ItemID, &mi.DisplayOrder, &priceOverride, &mi.Quantity,
			&mi.Item.ID, &mi.Item.Name, &mi.Item.ShortName, &mi.Item.Price, &vendingPrice,
			&mi.Item.ItemType,
			&mi.Item.IsUsable, &mi.Item.UsesPartyMenu, &mi.Item.IsKeyItem, &mi.Item.IsGuardDrink, &moveID,
			&mi.Item.Stackable, &mi.Item.StackSize,
			&mi.Item.BonusHP, &mi.Item.BonusAttack, &mi.Item.BonusDefense, &mi.Item.BonusSpeed, &mi.Item.BonusSpecial,
			&mi.Item.BonusAccuracy, &mi.Item.BonusEvasion, &mi.Item.BonusCatchRate, &mi.Item.BonusExp,
			&mi.Item.BonusEncounter, &mi.Item.BonusCrit, &mi.Item.BonusFlee,
			&mi.Item.HealAmount, &statusCure, &mi.Item.PPRestore, &mi.Item.RevivePercent, &mi.Item.BallModifier,
			&loreText, &mi.Item.Icon,
		)
		if err != nil {
			log.Printf("GetMerchantItems scan error: %v", err)
			continue
		}
		mi.PriceOverride = nullableInt32Ptr(priceOverride)
		assignNullableItemFields(&mi.Item, vendingPrice, moveID, statusCure, loreText)
		items = append(items, mi)
	}
	return items, nil
}

// AddItemToInventory creates an item instance and places it in the character's inventory.
// If the item is stackable and already exists in inventory, it increments the quantity.
// Returns the instance ID of the placed item.
func AddItemToInventory(charID int32, itemID int32, quantity uint16) (instanceID int32, err error) {
	item, err := GetItemByID(itemID)
	if err != nil {
		return 0, err
	}

	// If stackable, try to find an existing stack with room
	if item.Stackable {
		var existingInstanceID int32
		var existingQty uint16
		scanErr := db.GlobalWorldDB.DB.QueryRow(`
			SELECT ii.id, ii.quantity
			FROM cq_character_inventory ci
			JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
			WHERE ci.character_id = $1 AND ii.item_id = $2 AND ii.quantity < $3
			LIMIT 1
		`, charID, itemID, item.StackSize).Scan(&existingInstanceID, &existingQty)

		if scanErr == nil {
			// Found a partial stack — add to it
			newQty := existingQty + quantity
			if newQty > uint16(item.StackSize) {
				newQty = uint16(item.StackSize)
			}
			_, err = db.GlobalWorldDB.DB.Exec(`
				UPDATE cq_item_instances SET quantity = $1 WHERE id = $2
			`, newQty, existingInstanceID)
			if err != nil {
				return 0, fmt.Errorf("update stack quantity: %w", err)
			}
			return existingInstanceID, nil
		}
	}

	// Create new item instance
	var lastID int64
	err = db.GlobalWorldDB.DB.QueryRow(`
		INSERT INTO cq_item_instances (item_id, quantity, owner_id, owner_type)
		VALUES ($1, $2, $3, 0)
		RETURNING id
	`, itemID, quantity, charID).Scan(&lastID)
	if err != nil {
		return 0, fmt.Errorf("create item instance: %w", err)
	}
	instanceID = int32(lastID)

	// Place in inventory
	_, err = db.GlobalWorldDB.DB.Exec(`
		INSERT INTO cq_character_inventory (character_id, item_instance_id)
		VALUES ($1, $2)
	`, charID, instanceID)
	if err != nil {
		db.GlobalWorldDB.DB.Exec("DELETE FROM cq_item_instances WHERE id = $1", instanceID)
		return 0, fmt.Errorf("insert inventory row: %w", err)
	}

	return instanceID, nil
}

// RemoveItemFromInventory removes an item instance from inventory and deletes it.
func RemoveItemFromInventory(charID int32, instanceID int32) error {
	_, err := db.GlobalWorldDB.DB.Exec(`
		DELETE FROM cq_character_inventory WHERE character_id = $1 AND item_instance_id = $2
	`, charID, instanceID)
	if err != nil {
		return fmt.Errorf("remove from inventory: %w", err)
	}
	_, err = db.GlobalWorldDB.DB.Exec(`
		DELETE FROM cq_item_instances WHERE id = $1
	`, instanceID)
	if err != nil {
		return fmt.Errorf("delete item instance: %w", err)
	}
	return nil
}

// DecrementItemQuantity reduces quantity by 1. If quantity reaches 0, removes the item.
// Returns the new quantity (0 means removed).
func DecrementItemQuantity(charID int32, instanceID int32) (uint16, error) {
	var currentQty uint16
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT quantity FROM cq_item_instances WHERE id = $1
	`, instanceID).Scan(&currentQty)
	if err != nil {
		return 0, fmt.Errorf("get quantity: %w", err)
	}

	if currentQty <= 1 {
		return 0, RemoveItemFromInventory(charID, instanceID)
	}

	newQty := currentQty - 1
	_, err = db.GlobalWorldDB.DB.Exec(`
		UPDATE cq_item_instances SET quantity = $1 WHERE id = $2
	`, newQty, instanceID)
	if err != nil {
		return 0, fmt.Errorf("decrement quantity: %w", err)
	}
	return newQty, nil
}

// GetCharacterMoney returns the character's Pokédollars.
func GetCharacterMoney(charID int32) (int64, error) {
	var money sql.NullInt64
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT pokedollars
		FROM character_wallet WHERE character_id = $1
	`, charID).Scan(&money)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get currency: %w", err)
	}
	if !money.Valid {
		return 0, nil
	}
	return money.Int64, nil
}

// FindInventoryItemByItemID finds the first inventory item matching a given item template ID
func FindInventoryItemByItemID(charID int32, itemID int32) (*CQInventoryItem, error) {
	inv := &CQInventoryItem{}
	var vendingPrice, moveID sql.NullInt64
	var statusCure, loreText sql.NullString
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT ii.id, ii.item_id, ii.charges, ii.quantity, ii.owner_type,
			i.id, i.name, i.short_name, i.price, i.vending_price, i.item_type,
			i.is_usable, i.uses_party_menu, i.is_key_item, i.is_guard_drink, i.move_id,
			i.stackable, i.stack_size,
			i.bonus_hp, i.bonus_attack, i.bonus_defense, i.bonus_speed, i.bonus_special,
			i.bonus_accuracy, i.bonus_evasion, i.bonus_catch_rate, i.bonus_exp,
			i.bonus_encounter_rate, i.bonus_crit, i.bonus_flee,
			i.heal_amount, i.status_cure, i.pp_restore, i.revive_percent, i.ball_modifier,
			i.lore_text, i.icon
		FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		JOIN cq_items i ON i.id = ii.item_id
		WHERE ci.character_id = $1 AND ii.item_id = $2
		LIMIT 1
	`, charID, itemID).Scan(
		&inv.Instance.ID, &inv.Instance.ItemID, &inv.Instance.Charges, &inv.Instance.Quantity, &inv.Instance.OwnerType,
		&inv.Item.ID, &inv.Item.Name, &inv.Item.ShortName, &inv.Item.Price, &vendingPrice,
		&inv.Item.ItemType,
		&inv.Item.IsUsable, &inv.Item.UsesPartyMenu, &inv.Item.IsKeyItem, &inv.Item.IsGuardDrink, &moveID,
		&inv.Item.Stackable, &inv.Item.StackSize,
		&inv.Item.BonusHP, &inv.Item.BonusAttack, &inv.Item.BonusDefense, &inv.Item.BonusSpeed, &inv.Item.BonusSpecial,
		&inv.Item.BonusAccuracy, &inv.Item.BonusEvasion, &inv.Item.BonusCatchRate, &inv.Item.BonusExp,
		&inv.Item.BonusEncounter, &inv.Item.BonusCrit, &inv.Item.BonusFlee,
		&inv.Item.HealAmount, &statusCure, &inv.Item.PPRestore, &inv.Item.RevivePercent, &inv.Item.BallModifier,
		&loreText, &inv.Item.Icon,
	)
	if err != nil {
		return nil, err
	}
	assignNullableItemFields(&inv.Item, vendingPrice, moveID, statusCure, loreText)
	return inv, nil
}

// FindInventoryItemByInstanceID finds one inventory item by its concrete instance ID.
func FindInventoryItemByInstanceID(charID int32, instanceID int32) (*CQInventoryItem, error) {
	inv := &CQInventoryItem{}
	var vendingPrice, moveID sql.NullInt64
	var statusCure, loreText sql.NullString
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT ii.id, ii.item_id, ii.charges, ii.quantity, ii.owner_type,
			i.id, i.name, i.short_name, i.price, i.vending_price, i.item_type,
			i.is_usable, i.uses_party_menu, i.is_key_item, i.is_guard_drink, i.move_id,
			i.stackable, i.stack_size,
			i.bonus_hp, i.bonus_attack, i.bonus_defense, i.bonus_speed, i.bonus_special,
			i.bonus_accuracy, i.bonus_evasion, i.bonus_catch_rate, i.bonus_exp,
			i.bonus_encounter_rate, i.bonus_crit, i.bonus_flee,
			i.heal_amount, i.status_cure, i.pp_restore, i.revive_percent, i.ball_modifier,
			i.lore_text, i.icon
		FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		JOIN cq_items i ON i.id = ii.item_id
		WHERE ci.character_id = $1 AND ii.id = $2
		LIMIT 1
	`, charID, instanceID).Scan(
		&inv.Instance.ID, &inv.Instance.ItemID, &inv.Instance.Charges, &inv.Instance.Quantity, &inv.Instance.OwnerType,
		&inv.Item.ID, &inv.Item.Name, &inv.Item.ShortName, &inv.Item.Price, &vendingPrice,
		&inv.Item.ItemType,
		&inv.Item.IsUsable, &inv.Item.UsesPartyMenu, &inv.Item.IsKeyItem, &inv.Item.IsGuardDrink, &moveID,
		&inv.Item.Stackable, &inv.Item.StackSize,
		&inv.Item.BonusHP, &inv.Item.BonusAttack, &inv.Item.BonusDefense, &inv.Item.BonusSpeed, &inv.Item.BonusSpecial,
		&inv.Item.BonusAccuracy, &inv.Item.BonusEvasion, &inv.Item.BonusCatchRate, &inv.Item.BonusExp,
		&inv.Item.BonusEncounter, &inv.Item.BonusCrit, &inv.Item.BonusFlee,
		&inv.Item.HealAmount, &statusCure, &inv.Item.PPRestore, &inv.Item.RevivePercent, &inv.Item.BallModifier,
		&loreText, &inv.Item.Icon,
	)
	if err != nil {
		return nil, err
	}
	assignNullableItemFields(&inv.Item, vendingPrice, moveID, statusCure, loreText)
	return inv, nil
}
