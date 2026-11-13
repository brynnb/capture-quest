package world

import (
	"context"
	"database/sql"
	"log"

	"capturequest/internal/db"
)

type GameCornerHiddenCoinPickupResult struct {
	Success      bool
	Message      string
	CoinID       int
	Amount       int
	Coins        int
	AlreadyFound bool
}

func TryPickUpGameCornerHiddenCoin(charID int64, mapID, x, y int) GameCornerHiddenCoinPickupResult {
	result, err := collectGameCornerHiddenCoin(context.Background(), db.GlobalWorldDB.DB, charID, mapID, x, y)
	if err != nil {
		log.Printf("Game Corner hidden coin character %d: %v", charID, err)
	}
	return result
}

func collectGameCornerHiddenCoin(ctx context.Context, database *sql.DB, charID int64, mapID, x, y int) (GameCornerHiddenCoinPickupResult, error) {
	var result GameCornerHiddenCoinPickupResult
	previousCoins := 0
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var err error
		previousCoins, err = gameCornerCoinBalance(tx, charID)
		if err != nil {
			return err
		}
		result.Coins = previousCoins
		coinID, amount, err := gameCornerHiddenCoinAt(tx, mapID, x, y)
		if err == sql.ErrNoRows {
			result.Message = "No hidden coins."
			return nil
		}
		if err != nil {
			return err
		}
		result.CoinID, result.Amount = coinID, amount
		hasCase, err := characterHasCQItemIn(tx, charID, CoinCaseItemID)
		if err != nil {
			return err
		}
		if !hasCase {
			result.Message = "You need a COIN CASE!"
			return nil
		}
		var collected bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM character_collected_hidden_coins WHERE character_id=$1 AND hidden_coin_id=$2)`, charID, coinID).Scan(&collected); err != nil {
			return err
		}
		if collected {
			result.Message = "No hidden coins."
			result.AlreadyFound = true
			return nil
		}
		if _, err := tx.Exec(`INSERT INTO character_collected_hidden_coins(character_id,hidden_coin_id) VALUES($1,$2)`, charID, coinID); err != nil {
			return err
		}
		result.Coins, err = addCoinsInTransaction(tx, charID, amount)
		if err != nil {
			return err
		}
		result.Success = true
		result.Message = "Found coins!"
		if result.Coins >= MaxCoins {
			result.Message = "Found coins, but the COIN CASE is full!"
		}
		return nil
	})
	if err != nil {
		return GameCornerHiddenCoinPickupResult{Message: "Could not pick up hidden coins.", Coins: previousCoins}, err
	}
	return result, nil
}

func gameCornerHiddenCoinAt(q sqlQueryer, mapID, x, y int) (int, int, error) {
	var coinID int
	var amount int
	if err := q.QueryRow(
		`SELECT id, coin_amount FROM phaser_hidden_coins WHERE map_id = $1 AND x = $2 AND y = $3`,
		mapID, x, y).Scan(&coinID, &amount); err != nil {
		return 0, 0, err
	}
	return coinID, amount, nil
}
