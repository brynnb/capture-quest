package scriptsim

import (
	"fmt"

	"capturequest/internal/world"
)

type FieldMoveSummary struct {
	Allowed           bool
	Message           string
	MoveID            int
	MoveName          string
	RequiredBadgeFlag string
	KnownBySpeciesID  int
	KnownByName       string
}

func runFieldMovePermission(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	moveName := scenario.Trigger.MoveName
	if moveName == "" && scenario.Trigger.MoveID > 0 {
		var err error
		moveName, err = moveNameForID(scenario.Trigger.MoveID)
		if err != nil {
			return nil, err
		}
	}
	if moveName == "" {
		return nil, fmt.Errorf("field_move_permission trigger requires moveName or moveId")
	}

	permission := world.CanUseFieldMove(applied.CharacterID, moveName, efm)
	summary := fieldMoveSummary(permission)
	detail := fmt.Sprintf("move=%s allowed=%t", summary.MoveName, summary.Allowed)
	if summary.RequiredBadgeFlag != "" {
		detail = fmt.Sprintf("%s requiredBadge=%s", detail, summary.RequiredBadgeFlag)
	}
	if summary.KnownBySpeciesID != 0 {
		detail = fmt.Sprintf("%s knownBy=#%d %s", detail, summary.KnownBySpeciesID, summary.KnownByName)
	}
	if summary.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, summary.Message)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script:      basicScript("FieldMovePermission:"+summary.MoveName, scenario.Trigger.MapName, "field_move_permission"),
		Initial:     initial,
		Final:       final,
		FieldMove:   &summary,
		ActionEffects: []ActionEffect{{
			Type:    "fieldMovePermission",
			Detail:  detail,
			Changed: false,
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func fieldMoveSummary(result world.FieldMovePermissionResult) FieldMoveSummary {
	return FieldMoveSummary{
		Allowed:           result.Allowed,
		Message:           result.Message,
		MoveID:            result.MoveID,
		MoveName:          result.MoveName,
		RequiredBadgeFlag: result.RequiredBadgeFlag,
		KnownBySpeciesID:  result.KnownBySpeciesID,
		KnownByName:       result.KnownByName,
	}
}

func fieldMoveMatches(actual FieldMoveSummary, expected FieldMoveExpected) bool {
	if expected.Allowed != nil && actual.Allowed != *expected.Allowed {
		return false
	}
	if expected.Message != "" && actual.Message != expected.Message {
		return false
	}
	if expected.MoveName != "" && actual.MoveName != expected.MoveName {
		return false
	}
	if expected.RequiredBadgeFlag != "" && actual.RequiredBadgeFlag != expected.RequiredBadgeFlag {
		return false
	}
	if expected.KnownBySpeciesID != 0 && actual.KnownBySpeciesID != expected.KnownBySpeciesID {
		return false
	}
	return true
}
