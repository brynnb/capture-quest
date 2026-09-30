package world

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/db/pokedex"
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
	inventory    []cqitems.CQInventoryItem
	money        int64
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
	return gameCornerPrizePurchaseForSimulator(charID, prizeID, "")
}

func TryBuyGameCornerPrizeByName(charID int64, prizeName string) GameCornerPrizePurchaseResult {
	return gameCornerPrizePurchaseForSimulator(charID, 0, prizeName)
}

func gameCornerPrizePurchaseForSimulator(charID int64, prizeID int, prizeName string) GameCornerPrizePurchaseResult {
	result, err := buyGameCornerPrize(context.Background(), db.GlobalWorldDB.DB, charID, prizeID, prizeName)
	if err != nil {
		log.Printf("Game Corner prize purchase character %d: %v", charID, err)
	}
	return result
}

func GameCornerPrizeByID(prizeID int) (GameCornerPrize, error) {
	return readGameCornerPrize(prizeID, "")
}

func GameCornerPrizeByName(prizeName string) (GameCornerPrize, error) {
	return readGameCornerPrize(0, prizeName)
}

func readGameCornerPrize(prizeID int, prizeName string) (GameCornerPrize, error) {
	var prize GameCornerPrize
	err := db.Transaction(context.Background(), db.GlobalWorldDB.DB, func(tx db.DBTX) error {
		var err error
		prize, err = loadGameCornerPrize(tx, prizeID, prizeName)
		return err
	})
	return prize, err
}

func loadGameCornerPrize(database db.DBTX, prizeID int, prizeName string) (GameCornerPrize, error) {
	query := `SELECT id,prize_type,pokemon_id,tm_move_id,item_id,prize_name,coin_cost FROM phaser_game_corner_prizes WHERE id=$1`
	var identity interface{} = prizeID
	if prizeName != "" {
		query = `SELECT id,prize_type,pokemon_id,tm_move_id,item_id,prize_name,coin_cost FROM phaser_game_corner_prizes WHERE prize_name=$1 LIMIT 1`
		identity = prizeName
	}
	return scanGameCornerPrize(database.QueryRow(query, identity))
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

// Prize eligibility, reward, Pokédex registration and coin payment share one
// character lock and commit. Live callers inject their owned database.
func buyGameCornerPrize(ctx context.Context, database *sql.DB, charID int64, prizeID int, prizeName string) (GameCornerPrizePurchaseResult, error) {
	result := GameCornerPrizePurchaseResult{PCBox: -1, PCSlot: -1}
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
		prize, err := loadGameCornerPrize(tx, prizeID, prizeName)
		if err == sql.ErrNoRows {
			result.Message = "Prize not found"
			return nil
		}
		if err != nil {
			return err
		}
		result.Prize = &prize
		if prize.CoinCost <= 0 {
			return fmt.Errorf("prize %d has invalid coin_cost=%d", prize.ID, prize.CoinCost)
		}
		hasCase, err := characterHasCQItemIn(tx, charID, CoinCaseItemID)
		if err != nil {
			return err
		}
		if !hasCase {
			result.Message = "You need a COIN CASE!"
			return nil
		}
		if previousCoins < prize.CoinCost {
			result.Message = "You don't have enough coins!"
			return nil
		}
		switch prize.Type {
		case "pokemon":
			if prize.PokemonID == nil {
				return fmt.Errorf("prize %d missing pokemon_id", prize.ID)
			}
			level := getPrizePokemonLevel(*prize.PokemonID)
			result.AddedToParty, result.PCBox, result.PCSlot, err = pokebattle.AddPokemonToPartyOrPC(tx, charID, *prize.PokemonID, level)
			if err != nil {
				return err
			}
			if err := pokedex.MarkCaught(tx, charID, *prize.PokemonID); err != nil {
				return err
			}
			result.PrizeLevel = level
		case "tm":
			if prize.ItemID == nil {
				return fmt.Errorf("prize %d missing item_id", prize.ID)
			}
			if _, err := cqitems.NewStore(tx).AddItemToInventory(int32(charID), int32(*prize.ItemID), 1); err != nil {
				return err
			}
		default:
			return fmt.Errorf("prize %d has unsupported type %q", prize.ID, prize.Type)
		}
		result.Coins = previousCoins - prize.CoinCost
		if _, err := tx.Exec(`INSERT INTO character_coins(character_id,coins) VALUES($1,$2) ON CONFLICT(character_id) DO UPDATE SET coins=EXCLUDED.coins`, charID, result.Coins); err != nil {
			return err
		}
		store := cqitems.NewStore(tx)
		result.inventory, err = store.GetCharacterInventory(int32(charID))
		if err != nil {
			return err
		}
		result.money, err = store.GetCharacterMoney(int32(charID))
		if err != nil {
			return err
		}
		result.Success = true
		result.Message = "Here you go!"
		return nil
	})
	if err != nil {
		return GameCornerPrizePurchaseResult{Message: "Could not buy prize.", Coins: previousCoins, PCBox: -1, PCSlot: -1}, err
	}
	return result, nil
}

func gameCornerCoinBalance(database db.DBTX, charID int64) (int, error) {
	var coins int
	err := database.QueryRow(`SELECT COALESCE((SELECT coins FROM character_coins WHERE character_id=$1),0)`, charID).Scan(&coins)
	return coins, err
}
