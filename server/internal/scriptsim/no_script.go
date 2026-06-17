package scriptsim

import (
	"fmt"

	"capturequest/internal/world"
)

func runClickNoScript(
	scenario *Scenario,
	applied *AppliedFixture,
	initial *Snapshot,
	cutscenes *world.CutsceneManager,
	efm *world.EventFlagManager,
) (*Result, error) {
	keys := clickKeys(scenario.Trigger)
	if err := ensureClickTargetExists(scenario.Trigger.MapName, keys); err != nil {
		return nil, err
	}
	if cs := cutscenes.FindEligibleClickCutscene(scenario.Trigger.MapName, keys, applied.CharacterID, efm, scenario.Fixture.Direction); cs != nil {
		return nil, fmt.Errorf("expected no eligible click cutscene for %s keys=%v, got %s",
			scenario.Trigger.MapName, keys, cs.ScriptLabel)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script:      basicScript("NoScript:"+scenario.Trigger.MapName, scenario.Trigger.MapName, "click_no_script"),
		Initial:     initial,
		Final:       final,
		ActionEffects: []ActionEffect{{
			Type:   "noScript",
			Detail: fmt.Sprintf("%s keys=%v", scenario.Trigger.MapName, keys),
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}
