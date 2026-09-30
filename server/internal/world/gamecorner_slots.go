package world

import (
	"context"
	"database/sql"
	"log"

	"capturequest/internal/db"
)

type GameCornerSlotPlayResult struct {
	Success       bool
	Message       string
	Bet           int
	Payout        int
	MatchLine     string
	MatchSymbol   string
	Coins         int
	ReelPositions []int
	Reels         [][]string
}

func TryPlayGameCornerSlot(charID int64, bet int, isLucky bool, rng GameCornerRandom) GameCornerSlotPlayResult {
	result, err := playGameCornerSlot(context.Background(), db.GlobalWorldDB.DB, charID, bet, isLucky, rng)
	if err != nil {
		log.Printf("Game Corner slots character %d: %v", charID, err)
	}
	return result
}

func playGameCornerSlot(ctx context.Context, database *sql.DB, charID int64, bet int, isLucky bool, rng GameCornerRandom) (GameCornerSlotPlayResult, error) {
	if bet < 1 || bet > 3 {
		bet = 1
	}
	if rng == nil {
		rng = gameCornerRandSource{}
	}
	var result GameCornerSlotPlayResult
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
		result, err = applyGameCornerSlot(tx, charID, previousCoins, bet, isLucky, rng)
		return err
	})
	if err != nil {
		return GameCornerSlotPlayResult{Message: "Could not play slots.", Bet: bet, Coins: previousCoins}, err
	}
	return result, nil
}

func applyGameCornerSlot(tx db.DBTX, charID int64, coins, bet int, isLucky bool, rng GameCornerRandom) (GameCornerSlotPlayResult, error) {
	hasCase, err := characterHasCQItemIn(tx, charID, CoinCaseItemID)
	if err != nil {
		return GameCornerSlotPlayResult{}, err
	}
	if !hasCase {
		return GameCornerSlotPlayResult{Message: "You need a COIN CASE!", Bet: bet, Coins: coins}, nil
	}
	if coins < bet {
		return GameCornerSlotPlayResult{Message: "Not enough coins!", Bet: bet, Coins: coins}, nil
	}
	coins -= bet
	canWin, canWinSevenOrBar := determineWinFlagsWithRandom(isLucky, rng)

	var pos1, pos2, pos3 int
	var w1, w2, w3 [3]slotSymbol
	var matchSym slotSymbol
	var payout int
	var matchLine string

	for attempt := 0; attempt < 50; attempt++ {
		pos1 = rng.Intn(wheelSize)
		pos2 = rng.Intn(wheelSize)
		pos3 = rng.Intn(wheelSize)

		w1 = getVisibleSymbols(wheel1, pos1)
		w2 = getVisibleSymbols(wheel2, pos2)
		w3 = getVisibleSymbols(wheel3, pos3)

		matchSym, payout, matchLine = checkSlotMatch(w1, w2, w3, bet)
		if payout == 0 {
			break
		}

		isSevenOrBar := matchSym == symbol7 || matchSym == symbolBar
		if canWin && (!isSevenOrBar || canWinSevenOrBar) {
			break
		}

		payout = 0
		matchLine = ""
	}

	if payout > 0 {
		coins += payout
		if coins > MaxCoins {
			coins = MaxCoins
		}
	}

	if _, err := tx.Exec(
		`INSERT INTO character_coins (character_id, coins) VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET coins = EXCLUDED.coins`,
		charID, coins); err != nil {
		return GameCornerSlotPlayResult{}, err
	}

	matchSymbol := ""
	if payout > 0 {
		matchSymbol = symbolNames[matchSym]
	}
	return GameCornerSlotPlayResult{
		Success:       true,
		Bet:           bet,
		Payout:        payout,
		MatchLine:     matchLine,
		MatchSymbol:   matchSymbol,
		Coins:         coins,
		ReelPositions: []int{pos1, pos2, pos3},
		Reels:         slotReels(w1, w2, w3),
	}, nil
}

func slotReels(w1, w2, w3 [3]slotSymbol) [][]string {
	return [][]string{
		{symbolNames[w1[0]], symbolNames[w1[1]], symbolNames[w1[2]]},
		{symbolNames[w2[0]], symbolNames[w2[1]], symbolNames[w2[2]]},
		{symbolNames[w3[0]], symbolNames[w3[1]], symbolNames[w3[2]]},
	}
}
