package world

import (
	"database/sql"

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
	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		return gameCornerHiddenCoinFailure(charID, "Could not pick up hidden coins.")
	}
	defer tx.Rollback()

	coinID, amount, err := gameCornerHiddenCoinAt(tx, mapID, x, y)
	if err == sql.ErrNoRows {
		return gameCornerHiddenCoinFailure(charID, "No hidden coins.")
	}
	if err != nil {
		return gameCornerHiddenCoinFailure(charID, "Could not pick up hidden coins.")
	}
	result := GameCornerHiddenCoinPickupResult{
		CoinID: coinID,
		Amount: amount,
		Coins:  getCoins(charID),
	}
	if !hasCoinCase(tx, charID) {
		result.Message = "You need a COIN CASE!"
		return result
	}
	if hiddenCoinAlreadyCollected(tx, charID, coinID) {
		result.Message = "No hidden coins."
		result.AlreadyFound = true
		return result
	}
	if _, err := tx.Exec(
		`INSERT INTO character_collected_hidden_coins (character_id, hidden_coin_id) VALUES ($1, $2)`,
		charID, coinID); err != nil {
		return gameCornerHiddenCoinFailure(charID, "Could not pick up hidden coins.")
	}

	current := getCoinsForUpdate(tx, charID)
	newTotal := current + amount
	if newTotal > MaxCoins {
		newTotal = MaxCoins
	}
	if _, err := tx.Exec(
		`INSERT INTO character_coins (character_id, coins) VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET coins = EXCLUDED.coins`,
		charID, newTotal); err != nil {
		return gameCornerHiddenCoinFailure(charID, "Could not pick up hidden coins.")
	}
	if err := tx.Commit(); err != nil {
		return gameCornerHiddenCoinFailure(charID, "Could not pick up hidden coins.")
	}
	result.Success = true
	result.Message = "Found coins!"
	result.Coins = newTotal
	if newTotal >= MaxCoins {
		result.Message = "Found coins, but the COIN CASE is full!"
	}
	return result
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

func hiddenCoinAlreadyCollected(q sqlQueryer, charID int64, coinID int) bool {
	var exists int
	err := q.QueryRow(
		`SELECT 1 FROM character_collected_hidden_coins WHERE character_id = $1 AND hidden_coin_id = $2`,
		charID, coinID).Scan(&exists)
	return err == nil
}

func gameCornerHiddenCoinFailure(charID int64, message string) GameCornerHiddenCoinPickupResult {
	return GameCornerHiddenCoinPickupResult{
		Success: false,
		Message: message,
		Coins:   getCoins(charID),
	}
}
