package scriptsim

import (
	"encoding/json"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/world"
)

type Action = world.CutsceneAction
type ActionEffect = world.CutsceneActionEffect

func DecodeActions(cs *world.CutsceneScript) ([]Action, error) {
	actions, err := world.DecodeCutsceneActions(cs.Actions)
	if err != nil {
		return nil, fmt.Errorf("parse actions for %s: %w", cs.ScriptLabel, err)
	}
	return actions, nil
}

func ExecuteServerActions(charID int64, cs *world.CutsceneScript, efm *world.EventFlagManager) ([]ActionEffect, error) {
	return ExecuteServerActionsWithChoice(charID, cs, efm, nil)
}

func ExecuteServerActionsWithChoice(charID int64, cs *world.CutsceneScript, efm *world.EventFlagManager, choice *bool) ([]ActionEffect, error) {
	return ExecuteServerActionsWithChoiceAndWorld(charID, cs, efm, choice, nil)
}

func ExecuteServerActionsWithChoiceAndWorld(charID int64, cs *world.CutsceneScript, efm *world.EventFlagManager, choice *bool, wh *world.WorldHandler) ([]ActionEffect, error) {
	effects, completed, err := ExecuteActionListWithChoiceAndWorld(charID, cs.MapName, cs.Actions, efm, choice, wh)
	if err != nil {
		return nil, err
	}
	if !completed {
		return effects, nil
	}

	if cap(effects) < len(effects)+len(cs.SetsFlags)+1 {
		next := make([]ActionEffect, len(effects), len(effects)+len(cs.SetsFlags)+1)
		copy(next, effects)
		effects = next
	}
	for _, flag := range cs.SetsFlags {
		if flag == "" {
			continue
		}
		if err := efm.SetFlag(charID, flag); err != nil {
			return effects, err
		}
		effects = append(effects, ActionEffect{Type: "setsFlags", Detail: flag, Changed: true})
	}
	if cs.WarpToMapID != nil && cs.WarpToX != nil && cs.WarpToY != nil {
		if _, err := db.GlobalWorldDB.DB.Exec(
			`UPDATE character_data SET map_id = $1, x = $2, y = $3 WHERE id = $4`,
			*cs.WarpToMapID, *cs.WarpToX, *cs.WarpToY, charID); err != nil {
			return effects, fmt.Errorf("apply cutscene warp: %w", err)
		}
		effects = append(effects, ActionEffect{
			Type:    "warp",
			Detail:  fmt.Sprintf("map=%d x=%d y=%d", *cs.WarpToMapID, *cs.WarpToX, *cs.WarpToY),
			Changed: true,
		})
	}
	return effects, nil
}

func ExecuteActionList(charID int64, mapName string, rawActions json.RawMessage, efm *world.EventFlagManager) ([]ActionEffect, error) {
	effects, _, err := ExecuteActionListWithChoice(charID, mapName, rawActions, efm, nil)
	return effects, err
}

func ExecuteActionListWithChoice(charID int64, mapName string, rawActions json.RawMessage, efm *world.EventFlagManager, choice *bool) ([]ActionEffect, bool, error) {
	return ExecuteActionListWithChoiceAndWorld(charID, mapName, rawActions, efm, choice, nil)
}

func ExecuteActionListWithChoiceAndWorld(charID int64, mapName string, rawActions json.RawMessage, efm *world.EventFlagManager, choice *bool, wh *world.WorldHandler) ([]ActionEffect, bool, error) {
	return world.ApplyCutsceneActionList(world.CutsceneActionContext{
		WorldHandler: wh,
		EventFlags:   efm,
		Choice:       choice,
		StopAtChoice: true,
	}, mapName, rawActions, charID)
}
