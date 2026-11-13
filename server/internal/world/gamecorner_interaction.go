package world

import (
	"context"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/session"
)

func (wh *WorldHandler) authorizeSlotMachineIn(ctx context.Context, tx db.DBTX, ses *session.Session, req GameCornerSlotPlayRequest) (bool, error) {
	if req.MachineX == nil || req.MachineY == nil {
		return false, fmt.Errorf("Select a slot machine first.")
	}
	source, err := wh.ownedSourceInteractionPositionIn(tx, ses)
	if err != nil {
		return false, err
	}
	dx, dy := source.X-*req.MachineX, source.Y-*req.MachineY
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	// Retain the current client's two-tile Manhattan reach, but enforce it from
	// owned player state and recheck on every spin (including after movement).
	if normalizedVisiblePlayerMapID(wh, source.MapID) != GameCornerMapID || dx+dy > 2 {
		return false, fmt.Errorf("Move next to the slot machine first.")
	}
	rows, err := tx.(db.ContextDBTX).QueryContext(ctx, `SELECT x,y,routine,item_or_direction FROM phaser_hidden_objects WHERE map_id=$1 ORDER BY id`, GameCornerMapID)
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
func (wh *WorldHandler) authorizePrizeWindowIn(ctx context.Context, tx db.DBTX, ses *session.Session, prizeID int) error {
	window := GameCornerPrizeWindowForID(prizeID)
	if window == 0 {
		return errScriptInteractionDenied
	}
	text := fmt.Sprintf("TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_%d", window)
	return wh.authorizeSourceInteractionIn(ctx, tx, ses, PrizeRoomMapID, text)
}

// Read the same retained character owner for hidden-object and actor purchases.
// Source eligibility cannot be checked on a separate pool connection before the
// mutation obtains its character lock.
func (wh *WorldHandler) ownedSourceInteractionPositionIn(tx db.DBTX, ses *session.Session) (interactionPosition, error) {
	var source interactionPosition
	if err := db.RequireTransaction(tx); err != nil {
		return source, err
	}
	if ses == nil || !ses.HasValidClient() {
		return source, errScriptInteractionDenied
	}
	charID := int64(ses.Client.CharData().ID)
	if err := requireNoOwnedBattleIn(tx, charID); err != nil {
		return source, err
	}
	position := wh.ownedPlayerSnapshot(ses, "")
	if position.ServerMovementPending {
		return source, errScriptInteractionDenied
	}
	if err := tx.QueryRow(`SELECT map_id,CAST(x AS INTEGER),CAST(y AS INTEGER) FROM character_data WHERE id=$1`, charID).Scan(&source.MapID, &source.X, &source.Y); err != nil {
		return source, err
	}
	if normalizedVisiblePlayerMapID(wh, source.MapID) != position.MapID || source.X != position.X || source.Y != position.Y {
		return source, errScriptInteractionDenied
	}
	return source, nil
}

// Resolve a unique actor from the transaction-owned catalog and apply current
// character-specific reach, flags and visibility before the purchase can spend.
func (wh *WorldHandler) authorizeSourceInteractionIn(ctx context.Context, tx db.DBTX, ses *session.Session, mapID int, text string) error {
	source, err := wh.ownedSourceInteractionPositionIn(tx, ses)
	if err != nil {
		return err
	}
	charID := int64(ses.Client.CharData().ID)
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
