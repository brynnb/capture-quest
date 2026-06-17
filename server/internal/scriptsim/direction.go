package scriptsim

import "fmt"

func headingForFixtureDirection(direction string) (float64, error) {
	switch normalizeFixtureMoveName(direction) {
	case "", "UP":
		return 0, nil
	case "RIGHT":
		return 90, nil
	case "DOWN":
		return 180, nil
	case "LEFT":
		return 270, nil
	default:
		return 0, fmt.Errorf("unknown fixture direction %q", direction)
	}
}

func fixtureDirectionFromHeading(heading float64) string {
	normalized := int(heading) % 360
	if normalized < 0 {
		normalized += 360
	}
	switch normalized {
	case 90:
		return "RIGHT"
	case 180:
		return "DOWN"
	case 270:
		return "LEFT"
	default:
		return "UP"
	}
}
