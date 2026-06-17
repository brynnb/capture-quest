package main

import "database/sql"

// Red/Blue stores three "talking over" tiles per tileset. These are most often
// counters/register desks; pressing A while facing one extends sprite
// interaction one tile past the counter.
var talkOverRawFootTileIDsByTileset = map[int64]map[int64]bool{
	2:  {0x18: true, 0x19: true, 0x1e: true}, // MART
	5:  {0x3a: true},                         // DOJO
	6:  {0x18: true, 0x19: true, 0x1e: true}, // POKECENTER
	7:  {0x3a: true},                         // GYM
	9:  {0x17: true, 0x32: true},             // FOREST_GATE
	10: {0x17: true, 0x32: true},             // MUSEUM
	12: {0x17: true, 0x32: true},             // GATE
	15: {0x12: true},                         // CEMETERY
	18: {0x15: true, 0x36: true},             // LOBBY
	21: {0x07: true, 0x17: true},             // CLUB
	22: {0x12: true},                         // FACILITY
}

func isTalkOverTile(tilesetID int64, rawFootTileID sql.NullInt64) bool {
	if !rawFootTileID.Valid {
		return false
	}
	return talkOverRawFootTileIDsByTileset[tilesetID][rawFootTileID.Int64]
}
