package world

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
)

const (
	GameCornerMapID    = 135 // GAME_CORNER
	PrizeRoomMapID     = 137 // GAME_CORNER_PRIZE_ROOM
	MaxCoins           = 9999
	CoinCaseItemID     = 69
	CoinPurchasePrice  = 1000 // ₽1000 per 50 coins
	CoinPurchaseAmount = 50
)

// --- Slot machine reel data from original Game Boy ROM ---
// Source: data/events/slot_machine_wheels.asm, engine/slots/slot_machine.asm
type slotSymbol int

type GameCornerCoinPurchaseResult struct {
	Success bool
	Message string
	Money   int
	Coins   int
}

type GameCornerRandom interface {
	Intn(n int) int
}

type gameCornerRandSource struct{}

func (gameCornerRandSource) Intn(n int) int {
	return rand.Intn(n)
}

const (
	symbol7      slotSymbol = 0
	symbolBar    slotSymbol = 1
	symbolCherry slotSymbol = 2
	symbolFish   slotSymbol = 3 // Staryu/Goldeen
	symbolBird   slotSymbol = 4 // Pidgey
	symbolMouse  slotSymbol = 5 // Pikachu
)

var symbolNames = map[slotSymbol]string{
	symbol7:      "7",
	symbolBar:    "BAR",
	symbolCherry: "cherry",
	symbolFish:   "staryu",
	symbolBird:   "pidgey",
	symbolMouse:  "pikachu",
}

// Exact reel layouts from data/events/slot_machine_wheels.asm
const wheelSize = 18

var wheel1 = [wheelSize]slotSymbol{
	symbol7, symbolMouse, symbolFish,
	symbolBar, symbolCherry, symbol7,
	symbolFish, symbolBird, symbolBar,
	symbolCherry, symbol7, symbolMouse,
	symbolBird, symbolBar, symbolCherry,
	symbol7, symbolMouse, symbolFish,
}

var wheel2 = [wheelSize]slotSymbol{
	symbol7, symbolFish, symbolCherry,
	symbolBird, symbolMouse, symbolBar,
	symbolCherry, symbolFish, symbolBird,
	symbolCherry, symbolBar, symbolFish,
	symbolBird, symbolCherry, symbolMouse,
	symbol7, symbolFish, symbolCherry,
}

var wheel3 = [wheelSize]slotSymbol{
	symbol7, symbolBird, symbolFish,
	symbolCherry, symbolMouse, symbolBird,
	symbolFish, symbolCherry, symbolMouse,
	symbolBird, symbolFish, symbolCherry,
	symbolMouse, symbolBird, symbolBar,
	symbol7, symbolBird, symbolFish,
}

// getVisibleSymbols returns [bottom, middle, top] for a wheel at a given offset.
func getVisibleSymbols(wheel [wheelSize]slotSymbol, offset int) [3]slotSymbol {
	return [3]slotSymbol{
		wheel[offset%wheelSize],
		wheel[(offset+1)%wheelSize],
		wheel[(offset+2)%wheelSize],
	}
}

// Payout table from SlotRewardPointers in slot_machine.asm
var payoutTable = map[slotSymbol]int{
	symbol7:      300,
	symbolBar:    100,
	symbolCherry: 8,
	symbolFish:   15,
	symbolBird:   15,
	symbolMouse:  15,
}

// checkSlotMatch checks for a 3-symbol match on the active lines.
// Bet determines lines: 1=middle, 2=+top/bottom, 3=+diagonals.
// Returns (matched symbol, payout, line name) or (0, 0, "") for no match.
func checkSlotMatch(w1, w2, w3 [3]slotSymbol, bet int) (slotSymbol, int, string) {
	// [0]=bottom, [1]=middle, [2]=top
	check := func(a, b, c slotSymbol, line string) (slotSymbol, int, string) {
		if a == b && b == c {
			return a, payoutTable[a], line
		}
		return 0, 0, ""
	}

	// 3 coin bet: diagonals
	if bet >= 3 {
		if s, p, l := check(w1[0], w2[1], w3[2], "diagonal-up"); p > 0 {
			return s, p, l
		}
		if s, p, l := check(w1[2], w2[1], w3[0], "diagonal-down"); p > 0 {
			return s, p, l
		}
	}
	// 2 coin bet: top and bottom rows
	if bet >= 2 {
		if s, p, l := check(w1[2], w2[2], w3[2], "top"); p > 0 {
			return s, p, l
		}
		if s, p, l := check(w1[0], w2[0], w3[0], "bottom"); p > 0 {
			return s, p, l
		}
	}
	// 1 coin bet: middle row (always active)
	if s, p, l := check(w1[1], w2[1], w3[1], "middle"); p > 0 {
		return s, p, l
	}
	return 0, 0, ""
}

// determineWinFlags applies the original game's win probability logic.
// From SlotMachine_SetFlags in slot_machine.asm.
//
// Original ASM logic (non-lucky, sevenBarChance=253):
//
//	random == 0        → counter mode (guaranteed wins for 60 spins) ~0.4%
//	random > 253       → allowSevenAndBarMatches (254,255) ~0.8%
//	random > 210       → allowMatches (can win, not 7/BAR) (211-253) ~16.8%
//	random 1-210       → flags=0, can't win ~82.0%
//
// Lucky machines use sevenBarChance=250, giving slightly better 7/BAR odds.
func determineWinFlags(isLucky bool) (canWin bool, canWinSevenOrBar bool) {
	return determineWinFlagsWithRandom(isLucky, gameCornerRandSource{})
}

func determineWinFlagsWithRandom(isLucky bool, rng GameCornerRandom) (canWin bool, canWinSevenOrBar bool) {
	randByte := rng.Intn(256)
	sevenBarChance := 253
	if isLucky {
		sevenBarChance = 250
	}
	if randByte == 0 {
		return true, true // ~0.4% counter mode
	}
	if randByte > sevenBarChance {
		return true, true // can win with 7 or BAR
	}
	if randByte > 210 {
		return true, false // can win, but not 7/BAR
	}
	return false, false // ~82% of spins: can't win
}

// --- Coin balance helpers ---

func getCoins(charID int64) int {
	myDB := db.GlobalWorldDB.DB
	var coins int
	err := myDB.QueryRow("SELECT coins FROM character_coins WHERE character_id = $1", charID).Scan(&coins)
	if err != nil {
		return 0
	}
	return coins
}

func addCoinsInTransaction(database db.DBTX, charID int64, amount int) (int, error) {
	if err := db.RequireTransaction(database); err != nil {
		return 0, err
	}
	if amount <= 0 {
		return 0, fmt.Errorf("coin grant must be positive")
	}
	var total int
	err := database.QueryRow(`INSERT INTO character_coins(character_id,coins) VALUES($1,$2)
 ON CONFLICT(character_id) DO UPDATE SET coins=CASE WHEN character_coins.coins+EXCLUDED.coins>$3 THEN $3 ELSE character_coins.coins+EXCLUDED.coins END RETURNING coins`, charID, min(amount, MaxCoins), MaxCoins).Scan(&total)
	return total, err
}

type sqlQueryer interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

func hasCoinCase(q sqlQueryer, charID int64) bool {
	var exists int
	err := q.QueryRow(`
		SELECT 1
		FROM cq_character_inventory ci
		JOIN cq_item_instances ii ON ii.id = ci.item_instance_id
		WHERE ci.character_id = $1 AND ii.item_id = $2
		LIMIT 1`, charID, CoinCaseItemID).Scan(&exists)
	return err == nil
}

func TryBuyGameCornerCoins(charID int64) GameCornerCoinPurchaseResult {
	result, err := buyGameCornerCoins(context.Background(), db.GlobalWorldDB.DB, charID)
	if err != nil {
		log.Printf("Game Corner coin purchase character %d: %v", charID, err)
	}
	return result
}

// --- Handlers ---

// Opcodes 133 and 135 remain reserved for explicit stale-client rejection.
// The coin clerk sells through its source-issued script and durable completion;
// balances belong to the same owned gameplay snapshot as the ordinary wallet.
func HandleGameCornerCoinBalance(ses *session.Session, _ []byte, _ *WorldHandler) bool {
	ses.SendStreamJSON(map[string]interface{}{
		"success": false,
		"error":   "Use current gameplay recovery to read coins.",
	}, opcodes.GameCornerCoinBalanceResponse)
	return false
}

func HandleGameCornerBuyCoins(ses *session.Session, _ []byte, _ *WorldHandler) bool {
	ses.SendStreamJSON(map[string]interface{}{
		"success": false,
		"error":   "Interact with the coin clerk using the current client.",
	}, opcodes.GameCornerCoinBalanceResponse)
	return false
}

// getPrizePokemonLevel returns the level for a Game Corner prize Pokémon.
// Gen 1: all prize Pokémon are given at specific levels.
func getPrizePokemonLevel(pokemonID int) int {
	switch pokemonID {
	case 63: // Abra
		return 9
	case 35: // Clefairy
		return 8
	case 30: // Nidorina
		return 17
	case 147: // Dratini
		return 18
	case 123: // Scyther
		return 25
	case 137: // Porygon
		return 26
	default:
		return 10
	}
}
