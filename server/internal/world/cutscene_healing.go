package world

import (
	"fmt"
)

// Center healing joins the interpreter's transaction and party writer. Other
// healing scripts (including battle scripts) keep their existing policy.
func applyCenterHealingPolicy(ctx CutsceneActionContext, mapName string, charID int64) error {
	tx := ctx.mutation.database
	if ctx.mutation.boundBattle {
		return fmt.Errorf("finish the current battle before center healing")
	}
	if err := requireNoOwnedBattleIn(tx, charID); err != nil {
		return err
	}
	var centerMapID int
	if err := tx.QueryRow(`SELECT id FROM phaser_maps WHERE name=$1`, mapName).Scan(&centerMapID); err != nil {
		return fmt.Errorf("resolve healing center %q: %w", mapName, err)
	}
	_, _, currentMapID, err := ctx.mutation.currentPosition(ctx)
	if err != nil {
		return err
	}
	if currentMapID != centerMapID {
		return fmt.Errorf("healing center source changed: current=%d center=%d", currentMapID, centerMapID)
	}
	var validOptions bool
	if err := tx.QueryRow(`SELECT options IS NULL OR jsonb_typeof(options)='object' FROM character_data WHERE id=$1`, charID).Scan(&validOptions); err != nil {
		return err
	}
	if !validOptions {
		return fmt.Errorf("center options for character %d must be an object", charID)
	}
	// Preserve CaptureQuest's existing interior arrival rule. The original game's
	// outdoor FlyWarpData destination is a separate fidelity change, not a guessed
	// inbound warp. Merge only these keys so unrelated/unknown options survive.
	if _, err := tx.Exec(`UPDATE character_data SET options=COALESCE(options,'{}'::jsonb) || jsonb_build_object('lastPokeCenterMapId',$2::integer,'lastPokeCenterX',3,'lastPokeCenterY',4) WHERE id=$1`, charID, centerMapID); err != nil {
		return err
	}
	// Trainer resets are CaptureQuest policy, not original cartridge behavior.
	if _, err := tx.Exec(`DELETE FROM character_defeated_trainers WHERE character_id=$1`, charID); err != nil {
		return err
	}
	ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error {
		if p.WorldHandler != nil && p.WorldHandler.TrainerEncounter != nil {
			p.WorldHandler.TrainerEncounter.ClearPlayer(charID)
		}
		return nil
	})
	return nil
}
