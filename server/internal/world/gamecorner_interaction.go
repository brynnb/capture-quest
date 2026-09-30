package world

import (
	"context"
	"fmt"
	"time"

	"capturequest/internal/session"
)

// Coordinates identify a generated hidden-object record; the client supplies
// neither machine availability nor luck. Pointers reject old requests that omit
// the target rather than interpreting them as the tile at zero, zero.
type GameCornerSlotPlayRequest struct {
	Bet      int  `json:"bet"`
	MachineX *int `json:"machineX"`
	MachineY *int `json:"machineY"`
}

type GameCornerSlotResultResponse struct {
	Success       bool       `json:"success"`
	Error         string     `json:"error,omitempty"`
	ReelPositions []int      `json:"reelPositions,omitempty"`
	Reels         [][]string `json:"reels,omitempty"`
	Payout        int        `json:"payout"`
	MatchLine     string     `json:"matchLine"`
	Coins         *int       `json:"coins,omitempty"`
	Bet           int        `json:"bet"`
	IsLucky       *bool      `json:"isLucky,omitempty"`
}

func (wh *WorldHandler) authorizeSlotMachine(ses *session.Session, req GameCornerSlotPlayRequest) (bool, error) {
	if req.MachineX == nil || req.MachineY == nil {
		return false, fmt.Errorf("Select a slot machine first.")
	}
	x, y, mapID := wh.scriptPlayerPosition(ses)
	dx, dy := x-*req.MachineX, y-*req.MachineY
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	// Retain the current client's two-tile Manhattan reach, but enforce it from
	// owned player state and recheck on every spin (including after movement).
	if mapID != GameCornerMapID || dx+dy > 2 {
		return false, fmt.Errorf("Move next to the slot machine first.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := wh.database.QueryContext(ctx, `SELECT x,y,routine,item_or_direction FROM phaser_hidden_objects WHERE map_id=$1 ORDER BY id`, GameCornerMapID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	index, targetIndex := 0, 0
	var targetArgument string
	for rows.Next() {
		var machineX, machineY int
		var routine, argument *string
		if err := rows.Scan(&machineX, &machineY, &routine, &argument); err != nil {
			return false, err
		}
		index++
		if machineX != *req.MachineX || machineY != *req.MachineY {
			continue
		}
		if targetIndex != 0 {
			return false, fmt.Errorf("ambiguous slot machine at %d,%d", machineX, machineY)
		}
		if routine == nil || *routine != "StartSlotMachine" {
			return false, fmt.Errorf("No slot machine here.")
		}
		if argument == nil {
			return false, fmt.Errorf("slot machine at %d,%d missing item_or_direction", machineX, machineY)
		}
		targetIndex, targetArgument = index, *argument
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	if targetIndex == 0 {
		return false, fmt.Errorf("No slot machine here.")
	}
	switch targetArgument {
	case "SLOTS_OUTOFORDER":
		return false, fmt.Errorf("This machine is out of order.")
	case "SLOTS_OUTTOLUNCH":
		return false, fmt.Errorf("Someone is using this machine.")
	case "SLOTS_SOMEONESKEYS":
		return false, fmt.Errorf("Someone's keys are on this machine.")
	case "ANY_FACING":
	default:
		return false, fmt.Errorf("unsupported slot machine argument %q", targetArgument)
	}
	luckyIndex, err := ses.GameCorner.LuckyIndex(int64(ses.Client.CharData().ID), GameCornerMapID, func() (int, error) {
		// Original GameCornerSelectLuckySlotMachine draws a byte, promotes values
		// below seven to eight, then shifts three bits. StartSlotMachine compares
		// that value with the one-based hidden-object index. Export/import preserves
		// the source row order through its original IDs.
		n, err := cryptoRandomInt(256)
		if err != nil {
			return 0, err
		}
		if n < 7 {
			n = 8
		}
		return n >> 3, nil
	})
	return luckyIndex == targetIndex, err
}

// A prize ID selects an existing source window. The server resolves its sign
// identity and authorizes current actor visibility/reach; being in the room is
// insufficient. Catalog-only list requests do not authorize purchases.
func (wh *WorldHandler) authorizePrizeWindow(ses *session.Session, prizeID int) error {
	window := GameCornerPrizeWindowForID(prizeID)
	if window == 0 {
		return errScriptInteractionDenied
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	text := fmt.Sprintf("TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_%d", window)
	var objectID *int
	var count int
	if err := wh.database.QueryRowContext(ctx, `SELECT MIN(id),COUNT(*) FROM phaser_objects WHERE map_id=$1 AND text=$2`, PrizeRoomMapID, text).Scan(&objectID, &count); err != nil {
		return err
	}
	if count != 1 || objectID == nil {
		return fmt.Errorf("prize window %d has %d source actors", window, count)
	}
	_, _, err := wh.scriptInteractionTargetContext(ctx, ses, *objectID)
	return err
}
