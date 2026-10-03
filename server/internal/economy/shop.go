// Package economy owns durable wallet/inventory operations, independently of
// sockets and world managers. Success is returned only after the commit.
package economy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
)

type Service struct{ database *sql.DB }

func New(database *sql.DB) *Service { return &Service{database: database} }

type Purchase struct {
	Inventory  cqitems.CQInventorySnapshot `json:"inventory"`
	ItemID     int32                       `json:"itemId"`
	Quantity   uint16                      `json:"quantity"`
	InstanceID int32                       `json:"instanceId"`
	Money      int64                       `json:"money"`
	Item       *cqitems.CQItem             `json:"item"`
}

// Buy checks the authoritative offer and stock. mapID comes from the server's
// selected character, never the purchase payload. The UI supports 1..99 items.
func (s *Service) Buy(ctx context.Context, charID, mapID, merchantID, itemID int32, quantity uint16) (Purchase, error) {
	var result Purchase
	if quantity < 1 || quantity > 99 {
		return result, fmt.Errorf("choose between 1 and 99 items")
	}
	err := db.Transaction(ctx, s.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		var price int64
		var stock int
		err := tx.QueryRow(`SELECT COALESCE(mi.price_override, i.price), mi.quantity
			FROM cq_merchant_items mi JOIN cq_items i ON i.id = mi.item_id
			JOIN cq_merchants m ON m.id = mi.merchant_id
			WHERE mi.merchant_id = $1 AND mi.item_id = $2 AND m.map_id = $3
			FOR UPDATE OF mi`, merchantID, itemID, mapID).Scan(&price, &stock)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("item is not sold by this shop here")
		}
		if err != nil {
			return err
		}
		if price < 0 || stock < -1 {
			return fmt.Errorf("merchant %d item %d has invalid price or stock", merchantID, itemID)
		}
		if stock >= 0 && stock < int(quantity) {
			return fmt.Errorf("shop has insufficient stock")
		}
		if _, err := tx.Exec(`INSERT INTO character_wallet(character_id, pokedollars) VALUES ($1, 0) ON CONFLICT DO NOTHING`, charID); err != nil {
			return err
		}
		err = tx.QueryRow(`UPDATE character_wallet SET pokedollars = pokedollars - $2
			WHERE character_id = $1 AND pokedollars >= $2 RETURNING pokedollars`, charID, price*int64(quantity)).Scan(&result.Money)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("not enough money")
		}
		if err != nil {
			return err
		}
		items := cqitems.NewStore(tx)
		result.Item, err = items.GetItemByID(itemID)
		if err != nil {
			return err
		}
		result.InstanceID, err = items.AddItemToInventory(charID, itemID, quantity)
		if err != nil {
			return err
		}
		if stock >= 0 {
			if _, err := tx.Exec(`UPDATE cq_merchant_items SET quantity = quantity - $3 WHERE merchant_id = $1 AND item_id = $2`, merchantID, itemID, quantity); err != nil {
				return err
			}
		}
		result.ItemID, result.Quantity = itemID, quantity
		result.Inventory, err = items.GetCharacterSnapshot(ctx, charID)
		return err
	})
	if err != nil {
		return Purchase{}, err
	}
	return result, nil
}

type Sale struct {
	Inventory  cqitems.CQInventorySnapshot `json:"inventory"`
	InstanceID int32                       `json:"instanceId"`
	ItemName   string                      `json:"itemName"`
	SellPrice  int64                       `json:"sellPrice"`
	Money      int64                       `json:"money"`
}

// Sell sells the selected whole stack. The previous handler removed the entire
// stack but credited only one unit; credit and removal now commit together.
func (s *Service) Sell(ctx context.Context, charID, instanceID int32) (Sale, error) {
	var result Sale
	err := db.Transaction(ctx, s.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		items := cqitems.NewStore(tx)
		owned, err := items.FindInventoryItemByInstanceID(charID, instanceID)
		if err != nil {
			return err
		}
		if owned.Item.IsKeyItem || owned.Item.Price < 2 {
			return fmt.Errorf("this item cannot be sold")
		}
		result = Sale{InstanceID: instanceID, ItemName: owned.Item.Name, SellPrice: int64(owned.Item.Price/2) * int64(owned.Instance.Quantity)}
		if err := items.RemoveItemFromInventory(charID, instanceID); err != nil {
			return err
		}
		err = tx.QueryRow(`INSERT INTO character_wallet(character_id, pokedollars) VALUES ($1, $2)
			ON CONFLICT(character_id) DO UPDATE SET pokedollars = character_wallet.pokedollars + $2
			RETURNING pokedollars`, charID, result.SellPrice).Scan(&result.Money)
		if err != nil {
			return err
		}
		result.Inventory, err = items.GetCharacterSnapshot(ctx, charID)
		return err
	})
	if err != nil {
		return Sale{}, err
	}
	return result, nil
}
