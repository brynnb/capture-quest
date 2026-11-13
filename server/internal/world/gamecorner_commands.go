package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
)

// Source coordinates are expectations, never a client-authored movement update.
// The inventory command revision owns both Game Corner mutations and their
// bounded latest outcome receipt.
type GameCornerSlotPlayRequest struct {
	RequestID string                    `json:"requestId"`
	Command   *InventoryCommandIdentity `json:"command" tstype:"InventoryCommandIdentity"`
	MapID     *int                      `json:"mapId" tstype:"number"`
	FromX     *int                      `json:"fromX" tstype:"number"`
	FromY     *int                      `json:"fromY" tstype:"number"`
	Bet       int                       `json:"bet"`
	MachineX  *int                      `json:"machineX" tstype:"number"`
	MachineY  *int                      `json:"machineY" tstype:"number"`
}

type GameCornerSlotResultResponse struct {
	Success         bool       `json:"success" tstype:"true"`
	Version         int        `json:"version" tstype:"1"`
	Kind            string     `json:"kind" tstype:"'slot'"`
	RequestID       string     `json:"requestId"`
	CharacterID     int64      `json:"characterId"`
	CommandRevision int64      `json:"commandRevision"`
	MachineX        int        `json:"machineX"`
	MachineY        int        `json:"machineY"`
	Bet             int        `json:"bet"`
	ReelPositions   []int      `json:"reelPositions"`
	Reels           [][]string `json:"reels"`
	Payout          int        `json:"payout"`
	MatchLine       string     `json:"matchLine"`
	IsLucky         bool       `json:"isLucky"`
}

type GameCornerPrizeBuyRequest struct {
	RequestID string                    `json:"requestId"`
	Command   *InventoryCommandIdentity `json:"command" tstype:"InventoryCommandIdentity"`
	MapID     *int                      `json:"mapId" tstype:"number"`
	FromX     *int                      `json:"fromX" tstype:"number"`
	FromY     *int                      `json:"fromY" tstype:"number"`
	PrizeID   int                       `json:"prizeId"`
}

type GameCornerPrizeBuyResponse struct {
	Success         bool            `json:"success" tstype:"true"`
	Version         int             `json:"version" tstype:"1"`
	Kind            string          `json:"kind" tstype:"'prize'"`
	RequestID       string          `json:"requestId"`
	CharacterID     int64           `json:"characterId"`
	CommandRevision int64           `json:"commandRevision"`
	PrizeID         int             `json:"prizeId"`
	Prize           GameCornerPrize `json:"prize"`
	PrizeName       string          `json:"prizeName"`
	PrizeLevel      int             `json:"prizeLevel"`
	AddedToParty    bool            `json:"addedToParty"`
	PCBox           int             `json:"pcBox"`
	PCSlot          int             `json:"pcSlot"`
	Message         string          `json:"message"`
}

type GameCornerPrizeListRequest struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	MapID       *int   `json:"mapId" tstype:"number"`
	FromX       *int   `json:"fromX" tstype:"number"`
	FromY       *int   `json:"fromY" tstype:"number"`
	PrizeWindow int    `json:"prizeWindow"`
}

type GameCornerPrizeListResponse struct {
	Success     bool                  `json:"success" tstype:"true"`
	RequestID   string                `json:"requestId"`
	CharacterID int64                 `json:"characterId"`
	PrizeWindow int                   `json:"prizeWindow"`
	Prizes      []GameCornerPrize     `json:"prizes"`
	Snapshot    GameplayStateResponse `json:"snapshot"`
}

func gameCornerSourcePresent(mapID, x, y *int, expectedMap int) bool {
	return mapID != nil && x != nil && y != nil && *mapID == expectedMap
}

// This check runs only within the command/read's retained character owner.
// Receipt replays intentionally bypass it: they return an old narrative outcome,
// while the client must read current resources and cannot replay any effects.
func gameCornerSourceMatches(wh *WorldHandler, ses *session.Session, mapID, x, y *int) bool {
	position := wh.ownedPlayerSnapshot(ses, "")
	return position.MapID == *mapID && position.X == *x && position.Y == *y
}

func sendGameCornerCommandError(ses *session.Session, requestID string, opcode opcodes.OpCode, fallback string, err error) {
	message := fallback
	var rejection *itemuse.Rejection
	if errors.As(err, &rejection) {
		message = rejection.Message
	}
	log.Printf("Game Corner command %d character %d: %v", opcode, ses.Client.CharData().ID, err)
	sendInventoryCommandError(ses, requestID, opcode, message)
}

func HandleGameCornerSlotPlay(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req GameCornerSlotPlayRequest
	if err := decodePlayerMovement(payload, &req); err != nil || !validInventoryCommand(ses, req.RequestID, req.Command) || !gameCornerSourcePresent(req.MapID, req.FromX, req.FromY, GameCornerMapID) || req.MachineX == nil || req.MachineY == nil || req.Bet < 1 || req.Bet > 3 {
		sendInventoryCommandError(ses, req.RequestID, opcodes.GameCornerSlotResultResponse, "Invalid slot command.")
		return false
	}
	input, err := json.Marshal(req)
	if err != nil {
		sendInventoryCommandError(ses, req.RequestID, opcodes.GameCornerSlotResultResponse, "Invalid slot command.")
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	encoded, _, err := cqitems.NewStore(wh.database).ExecuteCommandWithReceipt(ses.CommandContext(), int32(charID), *req.Command.Revision, "gamecorner-slot", req.RequestID, input, func(tx db.DBTX) ([]byte, error) {
		if !gameCornerSourceMatches(wh, ses, req.MapID, req.FromX, req.FromY) {
			return nil, errScriptInteractionDenied
		}
		isLucky, err := wh.authorizeSlotMachineIn(ses.CommandContext(), tx, ses, req)
		if err != nil {
			return nil, err
		}
		result, err := playGameCornerSlot(ses.CommandContext(), tx, charID, req.Bet, isLucky, gameCornerRandSource{})
		if err != nil {
			return nil, err
		}
		if !result.Success {
			return nil, &itemuse.Rejection{Message: result.Message}
		}
		reply := GameCornerSlotResultResponse{Success: true, Version: 1, Kind: "slot", RequestID: req.RequestID, CharacterID: charID, CommandRevision: *req.Command.Revision + 1, MachineX: *req.MachineX, MachineY: *req.MachineY, Bet: result.Bet, ReelPositions: result.ReelPositions, Reels: result.Reels, Payout: result.Payout, MatchLine: result.MatchLine, IsLucky: isLucky}
		data, err := json.Marshal(reply)
		if err != nil {
			return nil, err
		}
		if _, err := decodeGameCornerSlotReceipt(data, req); err != nil {
			return nil, err
		}
		return data, nil
	})
	if err != nil {
		sendGameCornerCommandError(ses, req.RequestID, opcodes.GameCornerSlotResultResponse, "Could not play slots.", err)
		return false
	}
	reply, err := decodeGameCornerSlotReceipt(encoded, req)
	if err != nil {
		sendGameCornerCommandError(ses, req.RequestID, opcodes.GameCornerSlotResultResponse, "Could not recover this spin. Read your current state.", err)
		return false
	}
	// Only frozen narrative is returned. A current owned read supplies coins,
	// wallet and inventory, including after a replay or an uncertain response.
	ses.SendStreamJSON(reply, opcodes.GameCornerSlotResultResponse)
	return false
}

func HandleGameCornerPrizeBuy(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req GameCornerPrizeBuyRequest
	if err := decodePlayerMovement(payload, &req); err != nil || !validInventoryCommand(ses, req.RequestID, req.Command) || !gameCornerSourcePresent(req.MapID, req.FromX, req.FromY, PrizeRoomMapID) || GameCornerPrizeWindowForID(req.PrizeID) == 0 {
		sendInventoryCommandError(ses, req.RequestID, opcodes.GameCornerPrizeBuyResponse, "Invalid prize command.")
		return false
	}
	input, err := json.Marshal(req)
	if err != nil {
		sendInventoryCommandError(ses, req.RequestID, opcodes.GameCornerPrizeBuyResponse, "Invalid prize command.")
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	encoded, _, err := cqitems.NewStore(wh.database).ExecuteCommandWithReceipt(ses.CommandContext(), int32(charID), *req.Command.Revision, "gamecorner-prize", req.RequestID, input, func(tx db.DBTX) ([]byte, error) {
		if !gameCornerSourceMatches(wh, ses, req.MapID, req.FromX, req.FromY) {
			return nil, errScriptInteractionDenied
		}
		if err := wh.authorizePrizeWindowIn(ses.CommandContext(), tx, ses, req.PrizeID); err != nil {
			return nil, err
		}
		result, err := buyGameCornerPrize(ses.CommandContext(), tx, charID, req.PrizeID, "")
		if err != nil {
			return nil, err
		}
		if !result.Success {
			return nil, &itemuse.Rejection{Message: result.Message}
		}
		if result.Prize == nil {
			return nil, fmt.Errorf("successful prize purchase has no prize")
		}
		reply := GameCornerPrizeBuyResponse{Success: true, Version: 1, Kind: "prize", RequestID: req.RequestID, CharacterID: charID, CommandRevision: *req.Command.Revision + 1, PrizeID: req.PrizeID, Prize: *result.Prize, PrizeName: result.Prize.Name, PrizeLevel: result.PrizeLevel, AddedToParty: result.AddedToParty, PCBox: result.PCBox, PCSlot: result.PCSlot, Message: result.Message}
		data, err := json.Marshal(reply)
		if err != nil {
			return nil, err
		}
		if _, err := decodeGameCornerPrizeReceipt(data, req); err != nil {
			return nil, err
		}
		return data, nil
	})
	if err != nil {
		sendGameCornerCommandError(ses, req.RequestID, opcodes.GameCornerPrizeBuyResponse, "Could not buy prize.", err)
		return false
	}
	reply, err := decodeGameCornerPrizeReceipt(encoded, req)
	if err != nil {
		sendGameCornerCommandError(ses, req.RequestID, opcodes.GameCornerPrizeBuyResponse, "Could not recover this prize. Read your current state.", err)
		return false
	}
	ses.SendStreamJSON(reply, opcodes.GameCornerPrizeBuyResponse)
	return false
}

func HandleGameCornerPrizeList(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}
	var req GameCornerPrizeListRequest
	charID := int64(ses.Client.CharData().ID)
	if err := decodePlayerMovement(payload, &req); err != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != charID || !gameCornerSourcePresent(req.MapID, req.FromX, req.FromY, PrizeRoomMapID) || req.PrizeWindow < 1 || req.PrizeWindow > 3 {
		sendInventoryCommandError(ses, req.RequestID, opcodes.GameCornerPrizeListResponse, "Invalid prize window request.")
		return false
	}
	result := GameCornerPrizeListResponse{Success: true, RequestID: req.RequestID, CharacterID: charID, PrizeWindow: req.PrizeWindow}
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if !gameCornerSourceMatches(wh, ses, req.MapID, req.FromX, req.FromY) {
			return errScriptInteractionDenied
		}
		text := fmt.Sprintf("TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_%d", req.PrizeWindow)
		if err := wh.authorizeSourceInteractionIn(ses.CommandContext(), tx, ses, PrizeRoomMapID, text); err != nil {
			return err
		}
		hasCase, err := characterHasCQItemIn(tx, charID, CoinCaseItemID)
		if err != nil {
			return err
		}
		if !hasCase {
			return &itemuse.Rejection{Message: "You need a COIN CASE!"}
		}
		result.Prizes, err = gameCornerPrizesForWindowIn(tx, req.PrizeWindow)
		if err != nil {
			return err
		}
		result.Snapshot, _, err = readGameplayStateFrom(ses.CommandContext(), tx, ses, wh, GameplayStateRequest{RequestID: req.RequestID, MapID: *req.MapID})
		return err
	})
	if err != nil {
		sendGameCornerCommandError(ses, req.RequestID, opcodes.GameCornerPrizeListResponse, "Move to the window selling these prizes first.", err)
		return false
	}
	ses.SendStreamJSON(result, opcodes.GameCornerPrizeListResponse)
	return false
}

func gameCornerPrizesForWindowIn(tx db.DBTX, window int) ([]GameCornerPrize, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT id,prize_type,pokemon_id,tm_move_id,item_id,prize_name,coin_cost FROM phaser_game_corner_prizes ORDER BY sort_order,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	prizes := []GameCornerPrize{}
	for rows.Next() {
		prize, err := scanGameCornerPrize(rows)
		if err != nil {
			return nil, err
		}
		if prize.PrizeWindow != window {
			continue
		}
		if !validGameCornerPrize(prize) {
			return nil, fmt.Errorf("invalid prize catalog row %d", prize.ID)
		}
		prizes = append(prizes, prize)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(prizes) == 0 {
		return nil, fmt.Errorf("prize window %d has no catalog entries", window)
	}
	return prizes, nil
}

func gameCornerReceiptHasFields(encoded []byte, fields ...string) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(encoded, &object) != nil {
		return false
	}
	for _, key := range fields {
		value, ok := object[key]
		if !ok || len(value) == 0 || string(value) == "null" {
			return false
		}
	}
	return true
}

func decodeGameCornerSlotReceipt(encoded []byte, req GameCornerSlotPlayRequest) (GameCornerSlotResultResponse, error) {
	var reply GameCornerSlotResultResponse
	if decodePlayerMovement(encoded, &reply) != nil || !gameCornerReceiptHasFields(encoded, "success", "version", "kind", "requestId", "characterId", "commandRevision", "machineX", "machineY", "bet", "reelPositions", "reels", "payout", "matchLine", "isLucky") || !reply.Success || reply.Version != 1 || reply.Kind != "slot" || reply.RequestID != req.RequestID || reply.CharacterID != req.Command.CharacterID || reply.CommandRevision != *req.Command.Revision+1 || reply.MachineX != *req.MachineX || reply.MachineY != *req.MachineY || reply.Bet != req.Bet || len(reply.ReelPositions) != 3 || len(reply.Reels) != 3 || reply.Payout < 0 {
		return reply, fmt.Errorf("invalid slot command outcome")
	}
	for i, position := range reply.ReelPositions {
		if position < 0 || position >= wheelSize || len(reply.Reels[i]) != 3 {
			return reply, fmt.Errorf("invalid slot reel outcome")
		}
	}
	first := getVisibleSymbols(wheel1, reply.ReelPositions[0])
	second := getVisibleSymbols(wheel2, reply.ReelPositions[1])
	third := getVisibleSymbols(wheel3, reply.ReelPositions[2])
	expected := slotReels(first, second, third)
	for i := range expected {
		for j := range expected[i] {
			if reply.Reels[i][j] != expected[i][j] {
				return reply, fmt.Errorf("slot outcome symbols disagree with positions")
			}
		}
	}
	if reply.Payout == 0 {
		if reply.MatchLine != "" {
			return reply, fmt.Errorf("nonpaying slot outcome has a winning line")
		}
	} else {
		_, payout, line := checkSlotMatch(first, second, third, reply.Bet)
		if payout != reply.Payout || line != reply.MatchLine {
			return reply, fmt.Errorf("slot outcome payout disagrees with reels")
		}
	}
	return reply, nil
}

func validGameCornerPrize(prize GameCornerPrize) bool {
	if prize.ID <= 0 || prize.Name == "" || prize.CoinCost <= 0 || prize.PrizeWindow != GameCornerPrizeWindowForID(prize.ID) || prize.PrizeWindow == 0 {
		return false
	}
	switch prize.Type {
	case "pokemon":
		return prize.PokemonID != nil && *prize.PokemonID > 0 && *prize.PokemonID <= 151
	case "tm":
		return prize.ItemID != nil && *prize.ItemID > 0
	default:
		return false
	}
}

func decodeGameCornerPrizeReceipt(encoded []byte, req GameCornerPrizeBuyRequest) (GameCornerPrizeBuyResponse, error) {
	var reply GameCornerPrizeBuyResponse
	if decodePlayerMovement(encoded, &reply) != nil || !gameCornerReceiptHasFields(encoded, "success", "version", "kind", "requestId", "characterId", "commandRevision", "prizeId", "prize", "prizeName", "prizeLevel", "addedToParty", "pcBox", "pcSlot", "message") || !reply.Success || reply.Version != 1 || reply.Kind != "prize" || reply.RequestID != req.RequestID || reply.CharacterID != req.Command.CharacterID || reply.CommandRevision != *req.Command.Revision+1 || reply.PrizeID != req.PrizeID || reply.Prize.ID != req.PrizeID || !validGameCornerPrize(reply.Prize) || reply.PrizeName != reply.Prize.Name || reply.Message == "" {
		return reply, fmt.Errorf("invalid prize command outcome")
	}
	if reply.Prize.Type == "pokemon" {
		if reply.PrizeLevel != getPrizePokemonLevel(*reply.Prize.PokemonID) || (reply.AddedToParty && (reply.PCBox != -1 || reply.PCSlot != -1)) || (!reply.AddedToParty && (reply.PCBox < 0 || reply.PCBox >= pcBoxCount || reply.PCSlot < 0 || reply.PCSlot >= pcBoxSize)) {
			return reply, fmt.Errorf("invalid prize Pokemon placement outcome")
		}
	} else if reply.PrizeLevel != 0 || reply.AddedToParty || reply.PCBox != -1 || reply.PCSlot != -1 {
		return reply, fmt.Errorf("invalid item prize placement outcome")
	}
	return reply, nil
}
