package world

import (
	"database/sql"
	"encoding/json"
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

func setCoins(charID int64, coins int) error {
	if coins < 0 {
		coins = 0
	}
	if coins > MaxCoins {
		coins = MaxCoins
	}
	myDB := db.GlobalWorldDB.DB
	_, err := myDB.Exec(`INSERT INTO character_coins (character_id, coins) VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET coins = EXCLUDED.coins`, charID, coins)
	return err
}

func addCoins(charID int64, amount int) (int, error) {
	current := getCoins(charID)
	newTotal := current + amount
	if newTotal > MaxCoins {
		newTotal = MaxCoins
	}
	return newTotal, setCoins(charID, newTotal)
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

func gameCornerMoneyBalance(q sqlQueryer, charID int64) int {
	var money sql.NullInt64
	if err := q.QueryRow(
		`SELECT pokedollars FROM character_wallet WHERE character_id = $1`, charID).Scan(&money); err != nil {
		return 0
	}
	if !money.Valid {
		return 0
	}
	return int(money.Int64)
}

func getCoinsForUpdate(q sqlQueryer, charID int64) int {
	var coins int
	if err := q.QueryRow(
		`SELECT coins FROM character_coins WHERE character_id = $1 FOR UPDATE`, charID).Scan(&coins); err != nil {
		return 0
	}
	return coins
}

func TryBuyGameCornerCoins(charID int64) GameCornerCoinPurchaseResult {
	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		log.Printf("[GameCorner] Failed to start coin purchase transaction for char %d: %v", charID, err)
		return GameCornerCoinPurchaseResult{
			Success: false,
			Message: "Could not buy coins.",
			Money:   gameCornerMoneyBalance(db.GlobalWorldDB.DB, charID),
			Coins:   getCoins(charID),
		}
	}
	defer tx.Rollback()

	current := getCoinsForUpdate(tx, charID)
	money := gameCornerMoneyBalance(tx, charID)
	if !hasCoinCase(tx, charID) {
		return GameCornerCoinPurchaseResult{
			Success: false,
			Message: "You need a COIN CASE!",
			Money:   money,
			Coins:   current,
		}
	}
	if current >= MaxCoins {
		return GameCornerCoinPurchaseResult{
			Success: false,
			Message: "Your COIN CASE is full!",
			Money:   money,
			Coins:   current,
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO character_wallet (character_id, pokedollars)
		VALUES ($1, 0)
		ON CONFLICT (character_id) DO NOTHING`,
		charID); err != nil {
		log.Printf("[GameCorner] Failed to ensure wallet row for char %d: %v", charID, err)
		return GameCornerCoinPurchaseResult{Success: false, Message: "Could not buy coins.", Money: money, Coins: current}
	}
	result, err := tx.Exec(
		`UPDATE character_wallet SET pokedollars = pokedollars - $1 WHERE character_id = $2 AND pokedollars >= $3`,
		CoinPurchasePrice, charID, CoinPurchasePrice)
	if err != nil {
		log.Printf("[GameCorner] Failed to deduct coin purchase price for char %d: %v", charID, err)
		return GameCornerCoinPurchaseResult{Success: false, Message: "Could not buy coins.", Money: money, Coins: current}
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return GameCornerCoinPurchaseResult{
			Success: false,
			Message: "Not enough money!",
			Money:   gameCornerMoneyBalance(tx, charID),
			Coins:   current,
		}
	}

	newTotal := current + CoinPurchaseAmount
	if newTotal > MaxCoins {
		newTotal = MaxCoins
	}
	if _, err := tx.Exec(`INSERT INTO character_coins (character_id, coins) VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET coins = EXCLUDED.coins`, charID, newTotal); err != nil {
		log.Printf("[GameCorner] Failed to add purchased coins for char %d: %v", charID, err)
		return GameCornerCoinPurchaseResult{Success: false, Message: "Could not buy coins.", Money: money, Coins: current}
	}
	money = gameCornerMoneyBalance(tx, charID)
	if err := tx.Commit(); err != nil {
		log.Printf("[GameCorner] Failed to commit coin purchase for char %d: %v", charID, err)
		return GameCornerCoinPurchaseResult{Success: false, Message: "Could not buy coins.", Money: money, Coins: newTotal}
	}
	return GameCornerCoinPurchaseResult{
		Success: true,
		Message: "Here are 50 coins!",
		Money:   money,
		Coins:   newTotal,
	}
}

// --- Handlers ---

// HandleGameCornerCoinBalance returns the player's current coin count.
func HandleGameCornerCoinBalance(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	coins := getCoins(int64(char.ID))
	ses.SendStreamJSON(map[string]interface{}{
		"coins": coins,
	}, opcodes.GameCornerCoinBalanceResponse)
	return false
}

// HandleGameCornerBuyCoins lets the player buy 50 coins for ₽1000.
func HandleGameCornerBuyCoins(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	result := TryBuyGameCornerCoins(charID)
	if !result.Success {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   result.Message,
			"coins":   result.Coins,
			"money":   result.Money,
		}, opcodes.GameCornerCoinBalanceResponse)
		return false
	}

	log.Printf("[GameCorner] Player %d bought %d coins (total: %d)", charID, CoinPurchaseAmount, result.Coins)
	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"coins":   result.Coins,
		"money":   result.Money,
		"message": result.Message,
	}, opcodes.GameCornerCoinBalanceResponse)
	return false
}

// HandleGameCornerSlotPlay processes a slot machine spin.
// The server is the source of truth: it picks reel positions, checks matches,
// applies win probability flags, deducts/awards coins, and returns the result.
func HandleGameCornerSlotPlay(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		Bet     int  `json:"bet"`     // 1, 2, or 3 coins
		IsLucky bool `json:"isLucky"` // whether this machine was flagged lucky
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[GameCorner] Invalid slot request: %v", err)
		return false
	}

	if req.Bet < 1 || req.Bet > 3 {
		req.Bet = 1
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	result := TryPlayGameCornerSlot(charID, req.Bet, req.IsLucky, gameCornerRandSource{})
	if !result.Success {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   result.Message,
			"coins":   result.Coins,
			"bet":     result.Bet,
		}, opcodes.GameCornerSlotResultResponse)
		return false
	}
	if result.Payout > 0 {
		log.Printf("[GameCorner] Player %d won %d coins! (%s-%s-%s on %s) bet=%d",
			charID, result.Payout, result.MatchSymbol, result.MatchSymbol, result.MatchSymbol, result.MatchLine, result.Bet)
	}

	// Return reel positions so the client can animate the reels faithfully
	// Also return the visible symbols as names for display
	ses.SendStreamJSON(map[string]interface{}{
		"success":       true,
		"reelPositions": result.ReelPositions,
		"reels":         result.Reels,
		"payout":        result.Payout,
		"matchLine":     result.MatchLine,
		"coins":         result.Coins,
		"bet":           result.Bet,
	}, opcodes.GameCornerSlotResultResponse)
	return false
}

// HandleGameCornerPrizeList sends the list of available prizes.
func HandleGameCornerPrizeList(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	char := ses.Client.CharData()
	if char == nil {
		return false
	}

	if _, err := sendGameCornerPrizeList(ses, int64(char.ID), 0); err != nil {
		return false
	}
	return false
}

func sendGameCornerPrizeList(ses *session.Session, charID int64, prizeWindow int) (string, error) {
	if ses == nil {
		return "no session", nil
	}
	result, err := AvailableGameCornerPrizesForWindow(charID, prizeWindow)
	if err != nil {
		log.Printf("[GameCorner] Failed to load prizes: %v", err)
		return "", err
	}

	payload := map[string]interface{}{
		"success": result.Success,
		"prizes":  result.Prizes,
		"coins":   result.Coins,
		"error":   result.Message,
	}
	if prizeWindow > 0 {
		payload["prizeWindow"] = prizeWindow
	}
	ses.SendStreamJSON(payload, opcodes.GameCornerPrizeListResponse)
	return fmt.Sprintf("window=%d prizes=%d success=%t", prizeWindow, len(result.Prizes), result.Success), nil
}

// HandleGameCornerPrizeBuy processes a prize purchase.
func HandleGameCornerPrizeBuy(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		PrizeID int `json:"prizeId"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[GameCorner] Invalid prize buy request: %v", err)
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	result := TryBuyGameCornerPrize(charID, req.PrizeID)
	if !result.Success {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   result.Message,
			"coins":   result.Coins,
		}, opcodes.GameCornerPrizeBuyResponse)
		return false
	}

	prizeName := ""
	if result.Prize != nil {
		prizeName = result.Prize.Name
	}
	log.Printf("[GameCorner] Player %d bought %s for coins (remaining: %d)", charID, prizeName, result.Coins)

	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"coins":     result.Coins,
		"prizeName": prizeName,
		"message":   result.Message,
	}, opcodes.GameCornerPrizeBuyResponse)
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

// HandleGameCornerCoinPickup handles picking up hidden coins on the floor.
// Called when the player interacts with a hidden object that is a coin.
func HandleGameCornerCoinPickup(charID int64, coinAmount int, ses *session.Session) {
	newTotal, err := addCoins(charID, coinAmount)
	if err != nil {
		log.Printf("[GameCorner] Failed to add hidden coins for char %d: %v", charID, err)
		return
	}

	log.Printf("[GameCorner] Player %d picked up %d hidden coins (total: %d)", charID, coinAmount, newTotal)
	ses.SendStreamJSON(map[string]interface{}{
		"coins":      newTotal,
		"coinAmount": coinAmount,
		"message":    "Found coins!",
	}, opcodes.GameCornerCoinPickupNotify)
}
