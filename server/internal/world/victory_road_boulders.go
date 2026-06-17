package world

import (
	"fmt"

	"capturequest/internal/db"
)

type VictoryRoadBoulderTarget struct {
	MapName               string
	X                     int
	Y                     int
	Flag                  string
	DropsThroughHole      bool
	SourceObjectName      string
	DestinationMapName    string
	DestinationObjectName string
}

type VictoryRoadBoulderOutcome struct {
	Target     VictoryRoadBoulderTarget
	AlreadySet bool
	Changed    bool
}

func VictoryRoadBoulderTargetAt(mapName string, x, y int) (VictoryRoadBoulderTarget, bool) {
	for _, target := range victoryRoadBoulderTargetsForLookup() {
		if target.MapName == mapName && target.X == x && target.Y == y {
			return target, true
		}
	}
	return VictoryRoadBoulderTarget{}, false
}

func HandleVictoryRoadBoulderTarget(charID int64, target VictoryRoadBoulderTarget, efm *EventFlagManager) (VictoryRoadBoulderOutcome, error) {
	if target.Flag == "" {
		return VictoryRoadBoulderOutcome{}, fmt.Errorf("Victory Road boulder target missing flag at %s (%d,%d)", target.MapName, target.X, target.Y)
	}
	if efm == nil {
		return VictoryRoadBoulderOutcome{}, fmt.Errorf("event flags unavailable")
	}

	outcome := VictoryRoadBoulderOutcome{Target: target}
	if efm.CheckFlag(charID, target.Flag) {
		outcome.AlreadySet = true
		return outcome, nil
	}
	if err := efm.SetFlag(charID, target.Flag); err != nil {
		return VictoryRoadBoulderOutcome{}, err
	}
	outcome.Changed = true
	return outcome, nil
}

func victoryRoadBoulderTargetsForLookup() []VictoryRoadBoulderTarget {
	targets, ok := queryVictoryRoadBoulderTargets()
	if ok && len(targets) > 0 {
		return targets
	}
	return victoryRoadBoulderTargets
}

func queryVictoryRoadBoulderTargets() ([]VictoryRoadBoulderTarget, bool) {
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		return nil, false
	}
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT map_name, x, y, flag, drops_through_hole,
		       COALESCE(source_object_name, ''),
		       COALESCE(destination_map_name, ''),
		       COALESCE(destination_object_name, '')
		FROM phaser_boulder_targets
		WHERE target_family = 'victory_road'
		ORDER BY map_name, x, y`)
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	targets := []VictoryRoadBoulderTarget{}
	for rows.Next() {
		var target VictoryRoadBoulderTarget
		if err := rows.Scan(
			&target.MapName,
			&target.X,
			&target.Y,
			&target.Flag,
			&target.DropsThroughHole,
			&target.SourceObjectName,
			&target.DestinationMapName,
			&target.DestinationObjectName,
		); err != nil {
			return nil, false
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, false
	}
	return targets, true
}

var victoryRoadBoulderTargets = []VictoryRoadBoulderTarget{
	{
		MapName: "VICTORY_ROAD_1F",
		X:       17,
		Y:       13,
		Flag:    "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH",
	},
	{
		MapName: "VICTORY_ROAD_2F",
		X:       1,
		Y:       16,
		Flag:    "EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH1",
	},
	{
		MapName: "VICTORY_ROAD_2F",
		X:       9,
		Y:       16,
		Flag:    "EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH2",
	},
	{
		MapName: "VICTORY_ROAD_3F",
		X:       3,
		Y:       5,
		Flag:    "EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH1",
	},
	{
		MapName:               "VICTORY_ROAD_3F",
		X:                     23,
		Y:                     15,
		Flag:                  "EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH2",
		DropsThroughHole:      true,
		SourceObjectName:      "VictoryRoad3F_NPC_10",
		DestinationMapName:    "VICTORY_ROAD_2F",
		DestinationObjectName: "VictoryRoad2F_NPC_13",
	},
}
