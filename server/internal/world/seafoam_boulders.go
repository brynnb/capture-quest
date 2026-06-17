package world

import "fmt"

const seafoamBoulderScriptLabel = "SeafoamBoulderHole"

type SeafoamBoulderHole struct {
	MapName               string
	HoleIndex             int
	HoleX                 int
	HoleY                 int
	SourceObjectName      string
	DestinationMapName    string
	DestinationObjectName string
	Flag                  string
}

type SeafoamBoulderOutcome struct {
	Hole       SeafoamBoulderHole
	AlreadySet bool
	Changed    bool
	Dialogue   []string
}

func SeafoamBoulderScriptLabel() string {
	return seafoamBoulderScriptLabel
}

func SeafoamBoulderHoleForTrigger(mapName string, holeIndex int) (SeafoamBoulderHole, bool) {
	for _, hole := range seafoamBoulderHoles {
		if hole.MapName == mapName && hole.HoleIndex == holeIndex {
			return hole, true
		}
	}
	return SeafoamBoulderHole{}, false
}

func SeafoamBoulderHoleAt(mapName string, x, y int) (SeafoamBoulderHole, bool) {
	for _, hole := range seafoamBoulderHoles {
		if hole.MapName == mapName && hole.HoleX == x && hole.HoleY == y {
			return hole, true
		}
	}
	return SeafoamBoulderHole{}, false
}

func HandleSeafoamBoulderHole(charID int64, mapName string, holeIndex int, efm *EventFlagManager) (SeafoamBoulderOutcome, error) {
	hole, ok := SeafoamBoulderHoleForTrigger(mapName, holeIndex)
	if !ok {
		return SeafoamBoulderOutcome{}, fmt.Errorf("unknown Seafoam boulder hole %s #%d", mapName, holeIndex)
	}
	if efm == nil {
		return SeafoamBoulderOutcome{}, fmt.Errorf("event flags unavailable")
	}

	outcome := SeafoamBoulderOutcome{
		Hole:     hole,
		Dialogue: []string{"The boulder dropped through!"},
	}
	if efm.CheckFlag(charID, hole.Flag) {
		outcome.AlreadySet = true
		return outcome, nil
	}
	if err := efm.SetFlag(charID, hole.Flag); err != nil {
		return SeafoamBoulderOutcome{}, err
	}
	outcome.Changed = true
	return outcome, nil
}

var seafoamBoulderHoles = []SeafoamBoulderHole{
	{
		MapName:               "SEAFOAM_ISLANDS_1F",
		HoleIndex:             1,
		HoleX:                 17,
		HoleY:                 6,
		SourceObjectName:      "SeafoamIslands1F_NPC_1",
		DestinationMapName:    "SEAFOAM_ISLANDS_B1F",
		DestinationObjectName: "SeafoamIslandsB1F_NPC_1",
		Flag:                  "EVENT_SEAFOAM1_BOULDER1_DOWN_HOLE",
	},
	{
		MapName:               "SEAFOAM_ISLANDS_1F",
		HoleIndex:             2,
		HoleX:                 24,
		HoleY:                 6,
		SourceObjectName:      "SeafoamIslands1F_NPC_2",
		DestinationMapName:    "SEAFOAM_ISLANDS_B1F",
		DestinationObjectName: "SeafoamIslandsB1F_NPC_2",
		Flag:                  "EVENT_SEAFOAM1_BOULDER2_DOWN_HOLE",
	},
	{
		MapName:               "SEAFOAM_ISLANDS_B1F",
		HoleIndex:             1,
		HoleX:                 18,
		HoleY:                 6,
		SourceObjectName:      "SeafoamIslandsB1F_NPC_1",
		DestinationMapName:    "SEAFOAM_ISLANDS_B2F",
		DestinationObjectName: "SeafoamIslandsB2F_NPC_1",
		Flag:                  "EVENT_SEAFOAM2_BOULDER1_DOWN_HOLE",
	},
	{
		MapName:               "SEAFOAM_ISLANDS_B1F",
		HoleIndex:             2,
		HoleX:                 23,
		HoleY:                 6,
		SourceObjectName:      "SeafoamIslandsB1F_NPC_2",
		DestinationMapName:    "SEAFOAM_ISLANDS_B2F",
		DestinationObjectName: "SeafoamIslandsB2F_NPC_2",
		Flag:                  "EVENT_SEAFOAM2_BOULDER2_DOWN_HOLE",
	},
	{
		MapName:               "SEAFOAM_ISLANDS_B2F",
		HoleIndex:             1,
		HoleX:                 19,
		HoleY:                 6,
		SourceObjectName:      "SeafoamIslandsB2F_NPC_1",
		DestinationMapName:    "SEAFOAM_ISLANDS_B3F",
		DestinationObjectName: "SeafoamIslandsB3F_NPC_3",
		Flag:                  "EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE",
	},
	{
		MapName:               "SEAFOAM_ISLANDS_B2F",
		HoleIndex:             2,
		HoleX:                 22,
		HoleY:                 6,
		SourceObjectName:      "SeafoamIslandsB2F_NPC_2",
		DestinationMapName:    "SEAFOAM_ISLANDS_B3F",
		DestinationObjectName: "SeafoamIslandsB3F_NPC_4",
		Flag:                  "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE",
	},
	{
		MapName:               "SEAFOAM_ISLANDS_B3F",
		HoleIndex:             1,
		HoleX:                 3,
		HoleY:                 16,
		SourceObjectName:      "SeafoamIslandsB3F_NPC_1",
		DestinationMapName:    "SEAFOAM_ISLANDS_B4F",
		DestinationObjectName: "SeafoamIslandsB4F_NPC_1",
		Flag:                  "EVENT_SEAFOAM4_BOULDER1_DOWN_HOLE",
	},
	{
		MapName:               "SEAFOAM_ISLANDS_B3F",
		HoleIndex:             2,
		HoleX:                 6,
		HoleY:                 16,
		SourceObjectName:      "SeafoamIslandsB3F_NPC_2",
		DestinationMapName:    "SEAFOAM_ISLANDS_B4F",
		DestinationObjectName: "SeafoamIslandsB4F_NPC_2",
		Flag:                  "EVENT_SEAFOAM4_BOULDER2_DOWN_HOLE",
	},
}
