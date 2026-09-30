package world

import (
	"context"
	"database/sql"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
)

func buyGameCornerCoins(ctx context.Context, database *sql.DB, charID int64) (GameCornerCoinPurchaseResult, error) {
	var result GameCornerCoinPurchaseResult
	previousCoins, previousMoney := 0, 0
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var err error
		previousCoins, err = gameCornerCoinBalance(tx, charID)
		if err != nil {
			return err
		}
		money, err := cqitems.NewStore(tx).GetCharacterMoney(int32(charID))
		if err != nil {
			return err
		}
		previousMoney = int(money)
		result.Coins, result.Money = previousCoins, previousMoney
		hasCase, err := characterHasCQItemIn(tx, charID, CoinCaseItemID)
		if err != nil {
			return err
		}
		if !hasCase {
			result.Message = "You need a COIN CASE!"
			return nil
		}
		if previousCoins >= MaxCoins {
			result.Message = "Your COIN CASE is full!"
			return nil
		}
		if previousMoney < CoinPurchasePrice {
			result.Message = "Not enough money!"
			return nil
		}
		if _, err := tx.Exec(`UPDATE character_wallet SET pokedollars=pokedollars-$1 WHERE character_id=$2`, CoinPurchasePrice, charID); err != nil {
			return err
		}
		result.Coins, err = addCoinsInTransaction(tx, charID, CoinPurchaseAmount)
		if err != nil {
			return err
		}
		result.Money = previousMoney - CoinPurchasePrice
		result.Success = true
		result.Message = "Here are 50 coins!"
		return nil
	})
	if err != nil {
		return GameCornerCoinPurchaseResult{Message: "Could not buy coins.", Money: previousMoney, Coins: previousCoins}, err
	}
	return result, nil
}
