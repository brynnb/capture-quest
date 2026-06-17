package world

type SeafoamCurrent struct {
	MapName       string
	Label         string
	X             int
	Y             int
	RequiredFlags []string
	Movements     []SpinMovement
}

func SeafoamCurrentAt(charID int64, mapName string, x, y int, efm *EventFlagManager) (SeafoamCurrent, bool) {
	for _, current := range seafoamCurrents {
		if current.MapName != mapName || current.X != x || current.Y != y {
			continue
		}
		if !seafoamFlagsSet(charID, efm, current.RequiredFlags...) {
			continue
		}
		return current, true
	}
	return SeafoamCurrent{}, false
}

func SeafoamCurrentPath(startX, startY int, movements []SpinMovement) []PathNode {
	directions := ExpandMovements(movements)
	path := make([]PathNode, 0, len(directions))
	x, y := startX, startY
	for _, direction := range directions {
		x, y = stepDirection(x, y, direction)
		path = append(path, PathNode{X: x, Y: y})
	}
	return path
}

func SeafoamCurrentFinalPosition(startX, startY int, movements []SpinMovement) (int, int) {
	x, y := startX, startY
	for _, direction := range ExpandMovements(movements) {
		x, y = stepDirection(x, y, direction)
	}
	return x, y
}

func SeafoamSurfBlocked(charID int64, mapName string, x, y int, efm *EventFlagManager) bool {
	if mapName != "SEAFOAM_ISLANDS_B4F" || x != 7 || y != 11 {
		return false
	}
	return seafoamFlagsSet(
		charID,
		efm,
		"EVENT_SEAFOAM4_BOULDER1_DOWN_HOLE",
		"EVENT_SEAFOAM4_BOULDER2_DOWN_HOLE",
	)
}

func seafoamFlagsSet(charID int64, efm *EventFlagManager, flags ...string) bool {
	if efm == nil || charID == 0 {
		return false
	}
	for _, flag := range flags {
		if !efm.CheckFlag(charID, flag) {
			return false
		}
	}
	return true
}

func stepDirection(x, y int, direction string) (int, int) {
	switch direction {
	case "UP":
		return x, y - 1
	case "DOWN":
		return x, y + 1
	case "LEFT":
		return x - 1, y
	case "RIGHT":
		return x + 1, y
	default:
		return x, y
	}
}

var seafoamCurrents = []SeafoamCurrent{
	{
		MapName:       "SEAFOAM_ISLANDS_B3F",
		Label:         "SeafoamB3FCurrentNearSteps",
		X:             15,
		Y:             8,
		RequiredFlags: []string{"EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE"},
		Movements: []SpinMovement{
			{Direction: "DOWN", Count: 6},
			{Direction: "RIGHT", Count: 5},
			{Direction: "DOWN", Count: 3},
		},
	},
	{
		MapName:       "SEAFOAM_ISLANDS_B3F",
		Label:         "SeafoamB3FCurrentNearLeftBoulder",
		X:             18,
		Y:             6,
		RequiredFlags: []string{"EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE"},
		Movements: []SpinMovement{
			{Direction: "DOWN", Count: 6},
			{Direction: "RIGHT", Count: 2},
			{Direction: "DOWN", Count: 4},
		},
	},
	{
		MapName:       "SEAFOAM_ISLANDS_B3F",
		Label:         "SeafoamB3FCurrentNearRightBoulder",
		X:             19,
		Y:             6,
		RequiredFlags: []string{"EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE"},
		Movements: []SpinMovement{
			{Direction: "DOWN", Count: 6},
			{Direction: "RIGHT", Count: 2},
			{Direction: "DOWN", Count: 4},
			{Direction: "LEFT", Count: 1},
		},
	},
	{
		MapName:       "SEAFOAM_ISLANDS_B4F",
		Label:         "SeafoamB4FCurrentSurfExitLeft",
		X:             20,
		Y:             17,
		RequiredFlags: []string{"EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE"},
		Movements:     []SpinMovement{{Direction: "UP", Count: 2}},
	},
	{
		MapName:       "SEAFOAM_ISLANDS_B4F",
		Label:         "SeafoamB4FCurrentSurfExitRight",
		X:             21,
		Y:             17,
		RequiredFlags: []string{"EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE"},
		Movements:     []SpinMovement{{Direction: "UP", Count: 2}},
	},
	{
		MapName:       "SEAFOAM_ISLANDS_B4F",
		Label:         "SeafoamB4FCurrentSurfExitUpperLeft",
		X:             20,
		Y:             16,
		RequiredFlags: []string{"EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE"},
		Movements:     []SpinMovement{{Direction: "UP", Count: 1}},
	},
	{
		MapName:       "SEAFOAM_ISLANDS_B4F",
		Label:         "SeafoamB4FCurrentSurfExitUpperRight",
		X:             21,
		Y:             16,
		RequiredFlags: []string{"EVENT_SEAFOAM3_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM3_BOULDER2_DOWN_HOLE"},
		Movements:     []SpinMovement{{Direction: "UP", Count: 1}},
	},
	{
		MapName:       "SEAFOAM_ISLANDS_B4F",
		Label:         "SeafoamB4FCurrentNearLeftBoulder",
		X:             4,
		Y:             14,
		RequiredFlags: []string{"EVENT_SEAFOAM4_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM4_BOULDER2_DOWN_HOLE"},
		Movements: []SpinMovement{
			{Direction: "UP", Count: 3},
			{Direction: "RIGHT", Count: 3},
			{Direction: "UP", Count: 1},
		},
	},
	{
		MapName:       "SEAFOAM_ISLANDS_B4F",
		Label:         "SeafoamB4FCurrentNearRightBoulder",
		X:             5,
		Y:             14,
		RequiredFlags: []string{"EVENT_SEAFOAM4_BOULDER1_DOWN_HOLE", "EVENT_SEAFOAM4_BOULDER2_DOWN_HOLE"},
		Movements: []SpinMovement{
			{Direction: "UP", Count: 3},
			{Direction: "RIGHT", Count: 2},
			{Direction: "UP", Count: 1},
		},
	},
}
