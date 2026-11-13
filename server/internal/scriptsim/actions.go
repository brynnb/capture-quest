package scriptsim

import (
	"context"
	"encoding/json"
	"fmt"

	"capturequest/internal/scriptedactions"
	"capturequest/internal/world"
)

type Action = scriptedactions.Action
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
	effects, _, err := world.ApplyCutsceneScript(context.Background(), world.CutsceneActionContext{WorldHandler: wh, EventFlags: efm, Choice: choice, StopAtChoice: true}, cs, charID)
	return effects, err
}

func ExecuteActionList(charID int64, mapName string, rawActions json.RawMessage, efm *world.EventFlagManager) ([]ActionEffect, error) {
	effects, _, err := ExecuteActionListWithChoice(charID, mapName, rawActions, efm, nil)
	return effects, err
}

func ExecuteActionListWithChoice(charID int64, mapName string, rawActions json.RawMessage, efm *world.EventFlagManager, choice *bool) ([]ActionEffect, bool, error) {
	return ExecuteActionListWithChoiceAndWorld(charID, mapName, rawActions, efm, choice, nil)
}

func ExecuteActionListWithChoiceAndWorld(charID int64, mapName string, rawActions json.RawMessage, efm *world.EventFlagManager, choice *bool, wh *world.WorldHandler) ([]ActionEffect, bool, error) {
	return world.ApplyCutsceneActionList(context.Background(), world.CutsceneActionContext{
		WorldHandler: wh,
		EventFlags:   efm,
		Choice:       choice,
		StopAtChoice: true,
	}, mapName, rawActions, charID)
}
