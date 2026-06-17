package world

import (
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
)

type GameCornerPrize struct {
	ID          int    `json:"id"`
	Type        string `json:"type"`
	PokemonID   *int   `json:"pokemonId,omitempty"`
	TMMoveID    *int   `json:"tmMoveId,omitempty"`
	ItemID      *int   `json:"itemId,omitempty"`
	Name        string `json:"name"`
	CoinCost    int    `json:"coinCost"`
	PrizeWindow int    `json:"prizeWindow,omitempty"`
}

type GameCornerPrizeListResult struct {
	Success bool
	Message string
	Coins   int
	Prizes  []GameCornerPrize
}

type GameCornerPrizePurchaseResult struct {
	Success      bool
	Message      string
	Coins        int
	Prize        *GameCornerPrize
	PrizeLevel   int
	AddedToParty bool
	PCBox        int
	PCSlot       int
}

func AvailableGameCornerPrizes(charID int64) (GameCornerPrizeListResult, error) {
	return AvailableGameCornerPrizesForWindow(charID, 0)
}

func AvailableGameCornerPrizesForWindow(charID int64, prizeWindow int) (GameCornerPrizeListResult, error) {
	coins := getCoins(charID)
	if !hasCoinCase(db.GlobalWorldDB.DB, charID) {
		return GameCornerPrizeListResult{
			Success: false,
			Message: "You need a COIN CASE!",
			Coins:   coins,
			Prizes:  []GameCornerPrize{},
		}, nil
	}
	prizes, err := GameCornerPrizes()
	if err != nil {
		return GameCornerPrizeListResult{}, err
	}
	if prizeWindow > 0 {
		prizes = FilterGameCornerPrizesForWindow(prizes, prizeWindow)
	}
	return GameCornerPrizeListResult{
		Success: true,
		Coins:   coins,
		Prizes:  prizes,
	}, nil
}

func FilterGameCornerPrizesForWindow(prizes []GameCornerPrize, prizeWindow int) []GameCornerPrize {
	if prizeWindow <= 0 {
		return prizes
	}
	filtered := make([]GameCornerPrize, 0, len(prizes))
	for _, prize := range prizes {
		if GameCornerPrizeWindowForID(prize.ID) == prizeWindow {
			filtered = append(filtered, prize)
		}
	}
	return filtered
}

func GameCornerPrizeWindowForID(prizeID int) int {
	switch {
	case prizeID >= 1 && prizeID <= 3:
		return 1
	case prizeID >= 4 && prizeID <= 6:
		return 2
	case prizeID >= 7 && prizeID <= 9:
		return 3
	default:
		return 0
	}
}

func GameCornerPrizes() ([]GameCornerPrize, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT id, prize_type, pokemon_id, tm_move_id, item_id, prize_name, coin_cost
		FROM phaser_game_corner_prizes
		ORDER BY sort_order`)
	if err != nil {
		return nil, fmt.Errorf("load Game Corner prizes: %w", err)
	}
	defer rows.Close()

	prizes := []GameCornerPrize{}
	for rows.Next() {
		prize, err := scanGameCornerPrize(rows)
		if err != nil {
			return nil, err
		}
		prizes = append(prizes, prize)
	}
	return prizes, rows.Err()
}

func TryBuyGameCornerPrize(charID int64, prizeID int) GameCornerPrizePurchaseResult {
	prize, err := GameCornerPrizeByID(prizeID)
	if err != nil {
		return gameCornerPrizeFailure(charID, "Prize not found", nil)
	}
	return tryBuyGameCornerPrize(charID, prize)
}

func TryBuyGameCornerPrizeByName(charID int64, prizeName string) GameCornerPrizePurchaseResult {
	prize, err := GameCornerPrizeByName(prizeName)
	if err != nil {
		return gameCornerPrizeFailure(charID, "Prize not found", nil)
	}
	return tryBuyGameCornerPrize(charID, prize)
}

func GameCornerPrizeByID(prizeID int) (GameCornerPrize, error) {
	row := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, prize_type, pokemon_id, tm_move_id, item_id, prize_name, coin_cost
		FROM phaser_game_corner_prizes
		WHERE id = $1`, prizeID)
	return scanGameCornerPrize(row)
}

func GameCornerPrizeByName(prizeName string) (GameCornerPrize, error) {
	row := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, prize_type, pokemon_id, tm_move_id, item_id, prize_name, coin_cost
		FROM phaser_game_corner_prizes
		WHERE prize_name = $1
		LIMIT 1`, prizeName)
	return scanGameCornerPrize(row)
}

type prizeScanner interface {
	Scan(dest ...interface{}) error
}

func scanGameCornerPrize(scanner prizeScanner) (GameCornerPrize, error) {
	var prize GameCornerPrize
	var pokemonID, tmMoveID, itemID sql.NullInt64
	if err := scanner.Scan(
		&prize.ID,
		&prize.Type,
		&pokemonID,
		&tmMoveID,
		&itemID,
		&prize.Name,
		&prize.CoinCost,
	); err != nil {
		return GameCornerPrize{}, err
	}
	prize.PokemonID = nullablePrizeInt(pokemonID)
	prize.TMMoveID = nullablePrizeInt(tmMoveID)
	prize.ItemID = nullablePrizeInt(itemID)
	prize.PrizeWindow = GameCornerPrizeWindowForID(prize.ID)
	return prize, nil
}

func nullablePrizeInt(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func tryBuyGameCornerPrize(charID int64, prize GameCornerPrize) GameCornerPrizePurchaseResult {
	coins := getCoins(charID)
	if !hasCoinCase(db.GlobalWorldDB.DB, charID) {
		return gameCornerPrizeFailure(charID, "You need a COIN CASE!", &prize)
	}
	if coins < prize.CoinCost {
		return gameCornerPrizeFailure(charID, "You don't have enough coins!", &prize)
	}

	result := GameCornerPrizePurchaseResult{
		Success: true,
		Message: "Here you go!",
		Coins:   coins - prize.CoinCost,
		Prize:   &prize,
		PCBox:   -1,
		PCSlot:  -1,
	}
	switch prize.Type {
	case "pokemon":
		if prize.PokemonID == nil {
			return gameCornerPrizeFailure(charID, "Prize unavailable.", &prize)
		}
		level := getPrizePokemonLevel(*prize.PokemonID)
		added, box, slot, err := pokebattle.AddPokemonToPartyOrPC(db.GlobalWorldDB.DB, charID, *prize.PokemonID, level)
		if err != nil {
			return gameCornerPrizeFailure(charID, "Failed to create Pokemon.", &prize)
		}
		result.PrizeLevel = level
		result.AddedToParty = added
		result.PCBox = box
		result.PCSlot = slot
	case "tm":
		if prize.ItemID == nil {
			return gameCornerPrizeFailure(charID, "Prize unavailable.", &prize)
		}
		if _, err := cqitems.AddItemToInventory(int32(charID), int32(*prize.ItemID), 1); err != nil {
			return gameCornerPrizeFailure(charID, "Could not add prize.", &prize)
		}
	default:
		return gameCornerPrizeFailure(charID, "Prize unavailable.", &prize)
	}

	if err := setCoins(charID, result.Coins); err != nil {
		return gameCornerPrizeFailure(charID, "Could not buy prize.", &prize)
	}
	return result
}

func gameCornerPrizeFailure(charID int64, message string, prize *GameCornerPrize) GameCornerPrizePurchaseResult {
	return GameCornerPrizePurchaseResult{
		Success: false,
		Message: message,
		Coins:   getCoins(charID),
		Prize:   prize,
		PCBox:   -1,
		PCSlot:  -1,
	}
}
