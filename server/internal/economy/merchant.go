package economy

import (
	"context"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
)

type MerchantMenu struct {
	MerchantID int32                    `json:"merchantId"`
	Name       string                   `json:"name"`
	Items      []cqitems.CQMerchantItem `json:"items"`
	Money      int64                    `json:"money"`
}

// Open reads the menu using the server-owned map. A specific merchant selects
// its own offers; opening the map preserves the department store's combined menu.
// Reach and script eligibility belong to the interaction boundary, not this reader.
func (s *Service) Open(ctx context.Context, charID, mapID, merchantID int32) (MerchantMenu, error) {
	var result MerchantMenu
	err := db.Transaction(ctx, s.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return fmt.Errorf("lock merchant reader character: %w", err)
		}
		store := cqitems.NewStore(tx)
		merchants, err := store.GetMerchantsByMapID(mapID)
		if err != nil {
			return err
		}
		if merchantID > 0 {
			var selected []cqitems.CQMerchant
			for _, merchant := range merchants {
				if merchant.ID == merchantID {
					selected = append(selected, merchant)
				}
			}
			merchants = selected
		}
		if len(merchants) == 0 {
			return fmt.Errorf("no selected merchant on owned map %d", mapID)
		}
		result.MerchantID, result.Name = merchants[0].ID, merchants[0].Name
		result.Items = []cqitems.CQMerchantItem{}
		for _, merchant := range merchants {
			items, err := store.GetMerchantItems(merchant.ID)
			if err != nil {
				return err
			}
			result.Items = append(result.Items, items...)
		}
		result.Money, err = store.GetCharacterMoney(charID)
		if err != nil {
			return fmt.Errorf("merchant wallet: %w", err)
		}
		if result.Money < 0 || result.Money > int64(^uint32(0)) {
			return fmt.Errorf("invalid merchant wallet balance for character %d", charID)
		}
		return nil
	})
	if err != nil {
		return MerchantMenu{}, err
	}
	return result, nil
}
