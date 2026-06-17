package scriptsim

import (
	"fmt"

	"capturequest/internal/world"
)

func runRuntimeBoulderPush(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	x, y := triggerOrFixturePosition(scenario)
	var (
		outcome   world.BoulderPushResult
		attempted bool
		err       error
	)
	if scenario.Trigger.DestX != 0 || scenario.Trigger.DestY != 0 {
		outcome, attempted, err = world.TryPushBoulderFromMoveRequest(
			applied.CharacterID,
			applied.MapID,
			x,
			y,
			scenario.Trigger.DestX,
			scenario.Trigger.DestY,
			true,
			efm,
		)
	} else {
		outcome, attempted, err = world.TryPushBoulderFromFacingAttempt(
			applied.CharacterID,
			applied.MapID,
			x,
			y,
			scenario.Trigger.Direction,
			true,
			efm,
		)
	}
	if err != nil {
		return nil, err
	}

	boulders, err := world.BoulderObjectsForCharacter(applied.CharacterID, applied.MapID, efm)
	if err != nil {
		return nil, err
	}
	summary := boulderPushSummary(outcome, boulders)
	detail := fmt.Sprintf("attempted=%t success=%t object=%s from=(%d,%d) to=(%d,%d) direction=%s",
		attempted,
		summary.Success,
		summary.ObjectName,
		summary.FromX,
		summary.FromY,
		summary.ToX,
		summary.ToY,
		summary.Direction)
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}

	objectMapNames := append([]string{scenario.Trigger.MapName}, outcome.AffectedMaps...)
	objectStates, err := objectStatesForMaps(applied, efm, objectMapNames...)
	if err != nil {
		return nil, err
	}
	tileStates, err := eventTileStates(applied, efm)
	if err != nil {
		return nil, err
	}
	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Scenario:     scenario,
		CharacterID:  applied.CharacterID,
		Script:       basicScript("RuntimeBoulderPush:"+scenario.Trigger.MapName, scenario.Trigger.MapName, "runtime_boulder_push"),
		Initial:      initial,
		Final:        final,
		BoulderPush:  &summary,
		ObjectStates: objectStates,
		TileStates:   tileStates,
		ActionEffects: []ActionEffect{{
			Type:    "runtimeBoulderPush",
			Detail:  detail,
			Changed: summary.Success,
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}
