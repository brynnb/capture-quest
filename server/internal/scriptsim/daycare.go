package scriptsim

import (
	"fmt"
	"strings"

	"capturequest/internal/world"
)

func runDayCareDeposit(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	deposit := world.TryDepositDayCarePokemon(applied.CharacterID, scenario.Trigger.PartySlot)
	detail := fmt.Sprintf("success=%t slot=%d", deposit.Success, scenario.Trigger.PartySlot)
	if deposit.Status.Active {
		detail = fmt.Sprintf("%s pokemon=#%d L%d start=%d cost=%d",
			detail,
			deposit.Status.PokemonID,
			deposit.Status.Level,
			deposit.Status.StartLevel,
			deposit.Status.Cost)
	}
	if deposit.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, deposit.Message)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:      scenario,
		CharacterID:   applied.CharacterID,
		Script:        basicScript("DayCareDeposit", scenario.Trigger.MapName, "daycare_deposit"),
		Initial:       initial,
		Final:         final,
		ActionEffects: []ActionEffect{{Type: "dayCareDeposit", Detail: detail, Changed: deposit.Success}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runDayCareStep(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	repeat := scenario.Trigger.Repeat
	if repeat <= 0 {
		repeat = 1
	}
	status, changed, err := world.AdvanceDayCareSteps(applied.CharacterID, repeat)
	if err != nil {
		return nil, err
	}
	detail := fmt.Sprintf("steps=%d active=%t", repeat, status.Active)
	if status.Active {
		detail = fmt.Sprintf("%s pokemon=#%d L%d start=%d grown=%d cost=%d exp=%d",
			detail,
			status.PokemonID,
			status.Level,
			status.StartLevel,
			status.LevelsGrown,
			status.Cost,
			status.Exp)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:      scenario,
		CharacterID:   applied.CharacterID,
		Script:        basicScript("DayCareStep", scenario.Trigger.MapName, "daycare_step"),
		Initial:       initial,
		Final:         final,
		ActionEffects: []ActionEffect{{Type: "dayCareStep", Detail: detail, Changed: changed}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func runDayCareWithdraw(scenario *Scenario, applied *AppliedFixture, initial *Snapshot) (*Result, error) {
	withdraw := world.TryWithdrawDayCarePokemon(applied.CharacterID)
	detail := fmt.Sprintf("success=%t cost=%d money=%d slot=%d",
		withdraw.Success,
		withdraw.Cost,
		withdraw.Money,
		withdraw.PartySlot)
	if len(withdraw.LearnedMoves) > 0 {
		detail = fmt.Sprintf("%s learned=%s", detail, strings.Join(withdraw.LearnedMoves, ","))
	}
	if withdraw.Status.Active {
		detail = fmt.Sprintf("%s remaining=#%d L%d",
			detail,
			withdraw.Status.PokemonID,
			withdraw.Status.Level)
	}
	if withdraw.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, withdraw.Message)
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:      scenario,
		CharacterID:   applied.CharacterID,
		Script:        basicScript("DayCareWithdraw", scenario.Trigger.MapName, "daycare_withdraw"),
		Initial:       initial,
		Final:         final,
		ActionEffects: []ActionEffect{{Type: "dayCareWithdraw", Detail: detail, Changed: withdraw.Success}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func validateDayCareExpectation(status world.DayCareStatus, expected DayCareExpected) error {
	if expected.Active != nil && status.Active != *expected.Active {
		return fmt.Errorf("expected Day Care active %t, got %t", *expected.Active, status.Active)
	}
	if expected.Active != nil && !*expected.Active && !status.Active {
		return nil
	}
	if expected.PokemonID != 0 && status.PokemonID != expected.PokemonID {
		return fmt.Errorf("expected Day Care pokemon #%d, got #%d", expected.PokemonID, status.PokemonID)
	}
	if expected.Level != 0 && status.Level != expected.Level {
		return fmt.Errorf("expected Day Care level %d, got %d", expected.Level, status.Level)
	}
	if expected.StartLevel != 0 && status.StartLevel != expected.StartLevel {
		return fmt.Errorf("expected Day Care start level %d, got %d", expected.StartLevel, status.StartLevel)
	}
	if expected.LevelsGrown != nil && status.LevelsGrown != *expected.LevelsGrown {
		return fmt.Errorf("expected Day Care levels grown %d, got %d", *expected.LevelsGrown, status.LevelsGrown)
	}
	if expected.Cost != nil && status.Cost != *expected.Cost {
		return fmt.Errorf("expected Day Care cost %d, got %d", *expected.Cost, status.Cost)
	}
	if expected.Exp != nil && status.Exp != *expected.Exp {
		return fmt.Errorf("expected Day Care exp %d, got %d", *expected.Exp, status.Exp)
	}
	return nil
}
