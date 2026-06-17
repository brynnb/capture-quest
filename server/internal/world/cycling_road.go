package world

const (
	route16MapID = 27
	route18MapID = 29
)

// isForcedBicycleEntryTile mirrors the original ForcedBikeOrSurfMaps entries
// for Cycling Road. CaptureQuest uses stitched global overworld coordinates at
// runtime, while extractor/source data also appears in local map coordinates in
// tests and tooling.
func isForcedBicycleEntryTile(mapID, x, y int) bool {
	switch mapID {
	case UnifiedOverworldMapID:
		return (x == 77 && (y == -108 || y == -107)) ||
			(x == 93 && (y == 52 || y == 53))
	case route16MapID:
		return x == 17 && (y == 10 || y == 11)
	case route18MapID:
		return x == 33 && (y == 8 || y == 9)
	default:
		return false
	}
}
