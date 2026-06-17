package world

type ledgeRule struct {
	Direction             string
	StandingRawFootTileID int
	FrontRawFootTileID    int
}

// Gen 1 HandleLedges uses overworld-only tile pairs:
// player direction, current 8x8 feet tile, tile in front, required input.
var ledgeRules = []ledgeRule{
	{Direction: "DOWN", StandingRawFootTileID: 0x2c, FrontRawFootTileID: 0x37},
	{Direction: "DOWN", StandingRawFootTileID: 0x39, FrontRawFootTileID: 0x36},
	{Direction: "DOWN", StandingRawFootTileID: 0x39, FrontRawFootTileID: 0x37},
	{Direction: "LEFT", StandingRawFootTileID: 0x2c, FrontRawFootTileID: 0x27},
	{Direction: "LEFT", StandingRawFootTileID: 0x39, FrontRawFootTileID: 0x27},
	{Direction: "RIGHT", StandingRawFootTileID: 0x2c, FrontRawFootTileID: 0x0d},
	{Direction: "RIGHT", StandingRawFootTileID: 0x2c, FrontRawFootTileID: 0x1d},
	{Direction: "RIGHT", StandingRawFootTileID: 0x39, FrontRawFootTileID: 0x0d},
}

func canJumpLedge(direction string, standingRawFootTileID, frontRawFootTileID int) bool {
	for _, rule := range ledgeRules {
		if rule.Direction == direction &&
			rule.StandingRawFootTileID == standingRawFootTileID &&
			rule.FrontRawFootTileID == frontRawFootTileID {
			return true
		}
	}
	return false
}
