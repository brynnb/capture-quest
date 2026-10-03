package world

import (
	"context"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
)

// Dismissal retires the terminal battle only when its eligible follow-up plan
// is durable too. Publication belongs to the caller after this commit succeeds.
func commitBattleDismissal(ctx context.Context, wh *WorldHandler, charID int64, battle *pokebattle.BattleState, position protocol.OwnedPlayerPositionResponse) (*durableCutscene, error) {
	var plan *durableCutscene
	err := db.Transaction(ctx, wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var mapID, x, y int
		if err := tx.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=$1`, charID).Scan(&mapID, &x, &y); err != nil {
			return err
		}
		if mapID != position.MapID || x != position.X || y != position.Y {
			return fmt.Errorf("battle dismissal source differs from owned source")
		}
		if err := pokebattle.CloseBattleIn(tx, charID, battle); err != nil {
			return err
		}
		if !battleShouldSendPostBattleMapScript(battle) || wh.Cutscenes == nil {
			return nil
		}
		mapName, err := nativeScriptMapAt(tx.QueryRow, mapID, x, y)
		if err != nil {
			return err
		}
		flags, err := eventFlagSnapshotIn(tx, charID)
		if err != nil {
			return err
		}
		script, err := wh.Cutscenes.findEligibleMapScriptCutsceneIn(tx, mapName, charID, flags, position.Direction)
		if err != nil {
			return err
		}
		if script == nil {
			return nil
		}
		plan, err = issueCutsceneIn(tx, charID, issuedCutscene{Script: *script, MapID: mapID, X: x, Y: y})
		return err
	})
	return plan, err
}

func battleShouldSendPostBattleMapScript(battle *pokebattle.BattleState) bool {
	if battle == nil || !battle.IsOver() || battle.Trainer == nil {
		return false
	}
	if battle.PlayerWon() {
		return battle.Trainer.WinFlag != ""
	}
	return battle.Trainer.NoBlackoutOnLoss && battle.Trainer.LoseFlag != ""
}
