package cqitems

import (
	"context"
	"fmt"

	"capturequest/internal/db"
)

// GetCharacterSnapshot joins an existing transaction or owns a bounded one.
// The character lock serializes both reads with gameplay writers. Query errors
// must never turn a wallet failure into a plausible zero-balance success.
func (s *Store) GetCharacterSnapshot(ctx context.Context, charID int32) (CQInventorySnapshot, error) {
	var result CQInventorySnapshot
	err := db.Transaction(ctx, s.database, func(tx db.DBTX) error {
		// Snapshot reads must not fire UPDATE triggers or perform even a no-op
		// character write. PostgreSQL takes the same ownership lock with SELECT.
		var lockedID int32
		if err := tx.QueryRow(`SELECT id FROM character_data WHERE id=$1 FOR UPDATE`, charID).Scan(&lockedID); err != nil {
			return fmt.Errorf("lock inventory snapshot character %d: %w", charID, err)
		}
		store := NewStore(tx)
		var err error
		if err := tx.QueryRow(`SELECT COALESCE((SELECT revision FROM character_shop_state WHERE character_id=$1),0)`, charID).Scan(&result.CommandRevision); err != nil {
			return err
		}
		result.Money, err = store.GetCharacterMoney(charID)
		if err != nil {
			return fmt.Errorf("inventory wallet: %w", err)
		}
		if result.Money < 0 || result.Money > int64(^uint32(0)) {
			return fmt.Errorf("invalid inventory wallet balance %d for character %d", result.Money, charID)
		}
		result.Items, err = store.GetCharacterInventory(charID)
		if err != nil {
			return err
		}
		if result.Items == nil {
			result.Items = []CQInventoryItem{}
		}
		return nil
	})
	if err != nil {
		return CQInventorySnapshot{}, err
	}
	return result, nil
}

// AddItemToInventory grants the entire quantity, splitting overflow into new
// stacks. The returned ID identifies the first affected stack; callers should
// send a fresh inventory snapshot because a grant can affect several stacks.
func (s *Store) AddItemToInventory(charID, itemID int32, quantity uint16) (int32, error) {
	var firstID int32
	err := db.Transaction(context.Background(), s.database, func(tx db.DBTX) error {
		if quantity == 0 {
			return fmt.Errorf("item quantity must be positive")
		}
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		item, err := NewStore(tx).GetItemByID(itemID)
		if err != nil {
			return err
		}
		limit := 1
		if item.Stackable {
			if item.StackSize <= 0 || item.StackSize > 65535 {
				return fmt.Errorf("item %d has invalid stack size %d", itemID, item.StackSize)
			}
			limit = int(item.StackSize)
		}
		remaining := int(quantity)
		rows, err := tx.Query(`SELECT ii.id, ii.quantity FROM cq_character_inventory ci
			JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
			WHERE ci.character_id = $1 AND ii.item_id = $2 AND ii.owner_id = $1 AND ii.owner_type = 0
			ORDER BY ii.id`, charID, itemID)
		if err != nil {
			return err
		}
		type stack struct {
			id       int32
			quantity int
		}
		var stacks []stack
		for rows.Next() {
			var existing stack
			if err := rows.Scan(&existing.id, &existing.quantity); err != nil {
				rows.Close()
				return err
			}
			if existing.quantity < 1 || existing.quantity > limit {
				rows.Close()
				return fmt.Errorf("item instance %d has invalid quantity %d", existing.id, existing.quantity)
			}
			stacks = append(stacks, existing)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, existing := range stacks {
			if remaining == 0 {
				break
			}
			added := min(remaining, limit-existing.quantity)
			if added == 0 {
				continue
			}
			if _, err := tx.Exec(`UPDATE cq_item_instances SET quantity = quantity + $1 WHERE id = $2`, added, existing.id); err != nil {
				return err
			}
			if firstID == 0 {
				firstID = existing.id
			}
			remaining -= added
		}
		for remaining > 0 {
			added := min(remaining, limit)
			var id int32
			if err := tx.QueryRow(`INSERT INTO cq_item_instances (item_id, quantity, owner_id, owner_type)
				VALUES ($1, $2, $3, 0) RETURNING id`, itemID, added, charID).Scan(&id); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO cq_character_inventory (character_id, item_instance_id) VALUES ($1, $2)`, charID, id); err != nil {
				return err
			}
			if firstID == 0 {
				firstID = id
			}
			remaining -= added
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return firstID, nil
}

// RemoveItemFromInventory removes an entire owned stack atomically. Ownership
// must agree in both tables; a foreign instance must never be deleted merely
// because removing its inventory link happened to affect zero rows.
func (s *Store) RemoveItemFromInventory(charID, instanceID int32) error {
	return db.Transaction(context.Background(), s.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		if _, err := ownedQuantity(tx, charID, instanceID); err != nil {
			return err
		}
		return removeOwnedStack(tx, charID, instanceID)
	})
}

func (s *Store) DecrementItemQuantity(charID, instanceID int32) (uint16, error) {
	var remaining uint16
	err := db.Transaction(context.Background(), s.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		quantity, err := ownedQuantity(tx, charID, instanceID)
		if err != nil {
			return err
		}
		if quantity == 1 {
			return removeOwnedStack(tx, charID, instanceID)
		}
		remaining = quantity - 1
		_, err = tx.Exec(`UPDATE cq_item_instances SET quantity = $1 WHERE id = $2`, remaining, instanceID)
		return err
	})
	if err != nil {
		return 0, err
	}
	return remaining, nil
}

func ownedQuantity(tx db.DBTX, charID, instanceID int32) (uint16, error) {
	var quantity uint16
	err := tx.QueryRow(`SELECT ii.quantity FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		WHERE ci.character_id = $1 AND ii.id = $2 AND ii.owner_id = $1 AND ii.owner_type = 0`, charID, instanceID).Scan(&quantity)
	if err != nil {
		return 0, fmt.Errorf("find owned item %d for character %d: %w", instanceID, charID, err)
	}
	if quantity == 0 {
		return 0, fmt.Errorf("item instance %d has zero quantity", instanceID)
	}
	return quantity, nil
}

func removeOwnedStack(tx db.DBTX, charID, instanceID int32) error {
	if _, err := tx.Exec(`DELETE FROM cq_character_inventory WHERE character_id = $1 AND item_instance_id = $2`, charID, instanceID); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM cq_item_instances WHERE id = $1 AND owner_id = $2 AND owner_type = 0`, instanceID, charID)
	return err
}
