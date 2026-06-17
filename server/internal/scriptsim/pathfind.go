package scriptsim

import (
	"fmt"

	"capturequest/internal/world"
)

type PathfindSummary struct {
	Found  bool
	Length int
	Path   []world.PathNode
}

func runPathfind(scenario *Scenario, applied *AppliedFixture, initial *Snapshot, efm *world.EventFlagManager) (*Result, error) {
	startX, startY := triggerOrFixturePosition(scenario)
	endX, endY := scenario.Trigger.DestX, scenario.Trigger.DestY
	if endX == 0 && endY == 0 {
		return nil, fmt.Errorf("pathfind trigger requires destX/destY")
	}

	actorManager := world.NewPhaserActorManager(nil)
	path := actorManager.FindPathForCharacter(applied.CharacterID, applied.MapID, startX, startY, endX, endY, efm)
	summary := &PathfindSummary{
		Found:  len(path) > 0 || (startX == endX && startY == endY),
		Length: len(path),
		Path:   path,
	}

	final, err := CaptureSnapshot(applied.CharacterID, scenario.Fixture.MapName)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Scenario:    scenario,
		CharacterID: applied.CharacterID,
		Script:      basicScript("Pathfind:"+scenario.Trigger.MapName, scenario.Trigger.MapName, "pathfind"),
		Initial:     initial,
		Final:       final,
		Pathfind:    summary,
		ActionEffects: []ActionEffect{{
			Type:    "pathfind",
			Detail:  fmt.Sprintf("from=(%d,%d) to=(%d,%d) found=%t length=%d", startX, startY, endX, endY, summary.Found, summary.Length),
			Changed: false,
		}},
	}
	if err := result.ValidateExpectations(); err != nil {
		return result, err
	}
	return result, nil
}

func pathfindMatches(actual PathfindSummary, expected PathfindExpected) bool {
	if expected.Found != nil && actual.Found != *expected.Found {
		return false
	}
	if expected.Length != 0 && actual.Length != expected.Length {
		return false
	}
	for _, avoided := range expected.Avoids {
		if pathContainsNode(actual.Path, avoided.X, avoided.Y) {
			return false
		}
	}
	for _, required := range expected.Contains {
		if !pathContainsNode(actual.Path, required.X, required.Y) {
			return false
		}
	}
	return true
}

func pathContainsNode(path []world.PathNode, x, y int) bool {
	for _, node := range path {
		if node.X == x && node.Y == y {
			return true
		}
	}
	return false
}
