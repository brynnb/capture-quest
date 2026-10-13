package world

import (
	"context"
	"fmt"
	"time"

	"capturequest/internal/db"
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
	x, y, mapID := wh.ownedPlayerPosition(ses)
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
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
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
	text := fmt.Sprintf("TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_%d", window)
	return wh.authorizeSourceInteraction(ses, PrizeRoomMapID, text)
}

// Resolve source identity once for direct gameplay requests that do not carry
// an actor ID. Both prize windows and the coin clerk use the same bounded,
// fail-closed identity and runtime reach/visibility boundary.
func (wh *WorldHandler) authorizeSourceInteraction(ses *session.Session, mapID int, text string) error {
	return db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(ses.Client.CharData().ID)); err != nil {
			return err
		}
		return wh.authorizeSourceInteractionIn(ses.CommandContext(), tx, ses, mapID, text)
	})
}

// Source admission shares the mutation's retained character/query owner. Runtime
// position, durable pose, flags and actor overrides must agree before spending.
func (wh *WorldHandler) authorizeSourceInteractionIn(ctx context.Context, tx db.DBTX, ses *session.Session, mapID int, text string) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	charID := int64(ses.Client.CharData().ID)
	if err := requireNoOwnedBattleIn(tx, charID); err != nil {
		return err
	}
	position := wh.ownedPlayerSnapshot(ses, "")
	if position.ServerMovementPending {
		return errScriptInteractionDenied
	}
	var source interactionPosition
	if err := tx.QueryRow(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=$1`, charID).Scan(&source.MapID, &source.X, &source.Y); err != nil {
		return err
	}
	if normalizedVisiblePlayerMapID(wh, source.MapID) != position.MapID || source.X != position.X || source.Y != position.Y {
		return errScriptInteractionDenied
	}
	var objectID *int
	var count int
	if err := tx.QueryRow(`SELECT MIN(id),COUNT(*) FROM phaser_objects WHERE map_id=$1 AND text=$2`, mapID, text).Scan(&objectID, &count); err != nil {
		return err
	}
	if count != 1 || objectID == nil {
		return fmt.Errorf("source interaction map=%d text=%q has %d actors", mapID, text, count)
	}
	flags, err := eventFlagSnapshotIn(tx, charID)
	if err != nil {
		return err
	}
	_, _, err = wh.scriptInteractionTargetFrom(ctx, tx.(db.ContextDBTX), ses, *objectID, flags, source)
	return err
}
