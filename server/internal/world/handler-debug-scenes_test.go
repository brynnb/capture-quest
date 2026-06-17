package world

import (
	"encoding/json"
	"testing"
)

func TestDebugSceneCategoryTrade(t *testing.T) {
	scene := debugScenario{
		Name:        "route18_gate_2f_marc_trade_ready",
		Description: "Route 18 Gate 2F Youngster is ready to trade Lickitung named MARC for Slowbro.",
		Trigger: debugTrigger{
			Type:         "fixture_state",
			MapName:      "ROUTE_18_GATE_2F",
			TextConstant: route18Gate2FYoungsterTextConstant,
		},
	}
	if got := debugSceneCategory(scene, "FixtureState"); got != "trade" {
		t.Fatalf("category = %q, want trade", got)
	}
}

func TestDebugSceneCategoryDefault(t *testing.T) {
	scene := debugScenario{
		Name:        "oak_starter_bulbasaur",
		Description: "Choose Bulbasaur in Oak's Lab.",
		Trigger: debugTrigger{
			Type:    "npc_click",
			MapName: "OAKS_LAB",
		},
	}
	if got := debugSceneCategory(scene, "OakStarterBulbasaur"); got != "" {
		t.Fatalf("category = %q, want empty", got)
	}
}

func TestDebugSceneCategoryFieldMove(t *testing.T) {
	scene := debugScenario{
		Name:        "debug_field_move_cut_ready",
		Description: "Cut test setup.",
		Trigger: debugTrigger{
			Type: "field_move_permission",
		},
	}
	if got := debugSceneCategory(scene, "FieldMovePermission:CUT"); got != "field" {
		t.Fatalf("category = %q, want field", got)
	}
}

func TestDebugFixtureParsesSafariState(t *testing.T) {
	var scene debugScenario
	err := json.Unmarshal([]byte(`{
		"name": "safari_step_ongoing_session",
		"fixture": {
			"mapName": "SAFARI_ZONE_CENTER",
			"x": 14,
			"y": 24,
			"safari": {
				"active": true,
				"ballsLeft": 30,
				"stepsLeft": 2,
				"battle": {
					"pokemonId": 127,
					"level": 23
				}
			}
		}
	}`), &scene)
	if err != nil {
		t.Fatalf("unmarshal debug scenario: %v", err)
	}
	if scene.Fixture.Safari == nil {
		t.Fatal("fixture safari state was not parsed")
	}
	if !scene.Fixture.Safari.Active ||
		scene.Fixture.Safari.BallsLeft != 30 ||
		scene.Fixture.Safari.StepsLeft != 2 {
		t.Fatalf("unexpected safari fixture: %+v", scene.Fixture.Safari)
	}
	if scene.Fixture.Safari.Battle == nil ||
		scene.Fixture.Safari.Battle.PokemonID != 127 ||
		scene.Fixture.Safari.Battle.Level != 23 {
		t.Fatalf("unexpected safari battle fixture: %+v", scene.Fixture.Safari.Battle)
	}
}
