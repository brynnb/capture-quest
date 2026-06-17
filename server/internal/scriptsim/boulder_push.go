package scriptsim

import (
	"fmt"

	"capturequest/internal/world"
)

type BoulderPushSummary struct {
	Success      bool
	Message      string
	ObjectID     int
	ObjectName   string
	MapID        int
	MapName      string
	Direction    string
	FromX        int
	FromY        int
	ToX          int
	ToY          int
	Dropped      bool
	FlagSet      string
	StrengthUsed *FieldMoveSummary
	Boulders     []BoulderObjectSummary
}

type BoulderObjectSummary struct {
	ObjectID int
	Name     string
	Text     string
	X        int
	Y        int
	Visible  bool
	Label    string
}

func runBoulderPush(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	x, y := triggerOrFixturePosition(scenario)
	outcome, err := world.TryPushBoulder(
		applied.CharacterID,
		applied.MapID,
		x,
		y,
		scenario.Trigger.Direction,
		scenario.Trigger.ActivateStrength,
		efm,
	)
	if err != nil {
		return nil, err
	}

	boulders, err := world.BoulderObjectsForCharacter(applied.CharacterID, applied.MapID, efm)
	if err != nil {
		return nil, err
	}
	summary := boulderPushSummary(outcome, boulders)
	detail := fmt.Sprintf("success=%t object=%s from=(%d,%d) to=(%d,%d) direction=%s",
		summary.Success,
		summary.ObjectName,
		summary.FromX,
		summary.FromY,
		summary.ToX,
		summary.ToY,
		summary.Direction)
	if summary.Dropped {
		detail = fmt.Sprintf("%s dropped=true", detail)
	}
	if summary.FlagSet != "" {
		detail = fmt.Sprintf("%s flag=%s", detail, summary.FlagSet)
	}
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
		Script:       basicScript("BoulderPush:"+scenario.Trigger.MapName, scenario.Trigger.MapName, "boulder_push"),
		Initial:      initial,
		Final:        final,
		BoulderPush:  &summary,
		ObjectStates: objectStates,
		TileStates:   tileStates,
		ActionEffects: []ActionEffect{{
			Type:    "boulderPush",
			Detail:  detail,
			Changed: summary.Success,
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func boulderPushSummary(result world.BoulderPushResult, boulders []world.BoulderObjectState) BoulderPushSummary {
	summary := BoulderPushSummary{
		Success:    result.Success,
		Message:    result.Message,
		ObjectID:   result.ObjectID,
		ObjectName: result.ObjectName,
		MapID:      result.MapID,
		MapName:    result.MapName,
		Direction:  result.Direction,
		FromX:      result.FromX,
		FromY:      result.FromY,
		ToX:        result.ToX,
		ToY:        result.ToY,
		Dropped:    result.Dropped,
		FlagSet:    result.FlagSet,
		Boulders:   make([]BoulderObjectSummary, 0, len(boulders)),
	}
	if result.StrengthUsed != nil {
		used := fieldMoveSummary(result.StrengthUsed.Permission)
		used.Allowed = result.StrengthUsed.Success
		used.Message = result.StrengthUsed.Message
		summary.StrengthUsed = &used
	}
	for _, boulder := range boulders {
		summary.Boulders = append(summary.Boulders, BoulderObjectSummary{
			ObjectID: boulder.ObjectID,
			Name:     boulder.Name,
			Text:     boulder.Text,
			X:        boulder.X,
			Y:        boulder.Y,
			Visible:  boulder.Visible,
			Label:    boulder.Label,
		})
	}
	return summary
}

func boulderPushMatches(actual BoulderPushSummary, expected BoulderPushExpected) bool {
	if expected.Success != nil && actual.Success != *expected.Success {
		return false
	}
	if expected.Message != "" && actual.Message != expected.Message {
		return false
	}
	if expected.ObjectName != "" && actual.ObjectName != expected.ObjectName {
		return false
	}
	if expected.FromX != 0 && actual.FromX != expected.FromX {
		return false
	}
	if expected.FromY != 0 && actual.FromY != expected.FromY {
		return false
	}
	if expected.ToX != 0 && actual.ToX != expected.ToX {
		return false
	}
	if expected.ToY != 0 && actual.ToY != expected.ToY {
		return false
	}
	if expected.Dropped != nil && actual.Dropped != *expected.Dropped {
		return false
	}
	if expected.FlagSet != "" && actual.FlagSet != expected.FlagSet {
		return false
	}
	return true
}
