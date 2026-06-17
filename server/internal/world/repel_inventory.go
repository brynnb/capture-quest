package world

import (
	"database/sql"
	"fmt"

	"capturequest/internal/db/cqitems"
)

type RepelUseResult struct {
	Message     string
	NewQuantity uint16
	StepsLeft   int
}

func UseRepelInventoryItem(wh *WorldHandler, charID int32, itemID int32, found *cqitems.CQInventoryItem) (RepelUseResult, error) {
	if wh == nil || wh.WildEncounter == nil {
		return RepelUseResult{}, fmt.Errorf("Repel can't be used right now")
	}
	steps, ok := RepelStepsForItem(int(itemID))
	if !ok {
		return RepelUseResult{}, fmt.Errorf("Not a repel item")
	}
	if wh.WildEncounter.isRepelActive(int64(charID)) {
		return RepelUseResult{}, fmt.Errorf("A repel is already active!")
	}
	if found == nil {
		invItem, err := cqitems.FindInventoryItemByItemID(charID, itemID)
		if err != nil {
			if err == sql.ErrNoRows {
				return RepelUseResult{}, fmt.Errorf("You don't have that item.")
			}
			return RepelUseResult{}, fmt.Errorf("find repel item: %w", err)
		}
		found = invItem
	}
	if found.Item.ID != itemID {
		return RepelUseResult{}, fmt.Errorf("inventory item mismatch: got %d, want %d", found.Item.ID, itemID)
	}
	newQty, err := cqitems.DecrementItemQuantity(charID, found.Instance.ID)
	if err != nil {
		return RepelUseResult{}, fmt.Errorf("consume repel item: %w", err)
	}
	wh.WildEncounter.ActivateRepel(int64(charID), int(itemID))
	return RepelUseResult{
		Message:     found.Item.Name + "'s effect started!",
		NewQuantity: newQty,
		StepsLeft:   steps,
	}, nil
}
