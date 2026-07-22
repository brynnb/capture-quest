package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestParseDungeonWarpLandingsASM(t *testing.T) {
	raw := `
DungeonWarpList:
	db SEAFOAM_ISLANDS_B4F, 1
	db VICTORY_ROAD_2F,     2
	db -1 ; end

DungeonWarpData:
	fly_warp SEAFOAM_ISLANDS_B4F,  4, 14
	fly_warp VICTORY_ROAD_2F,     22, 16
`

	landings, err := parseDungeonWarpLandingsASM(raw)
	if err != nil {
		t.Fatalf("parseDungeonWarpLandingsASM: %v", err)
	}
	if len(landings) != 2 {
		t.Fatalf("landings = %#v, want 2", landings)
	}
	if landings[0] != (dungeonWarpLanding{DestinationMap: "SEAFOAM_ISLANDS_B4F", WarpIndex: 1, X: 4, Y: 14}) {
		t.Fatalf("first landing = %#v, want Seafoam B4F #1 at (4,14)", landings[0])
	}
	if landings[1] != (dungeonWarpLanding{DestinationMap: "VICTORY_ROAD_2F", WarpIndex: 2, X: 22, Y: 16}) {
		t.Fatalf("second landing = %#v, want Victory Road 2F #2 at (22,16)", landings[1])
	}
}

func TestParseDungeonWarpSourceTriggersASM(t *testing.T) {
	raw := `
ExampleScript:
	ld a, SEAFOAM_ISLANDS_B4F
	ld [wDungeonWarpDestinationMap], a
	ld hl, ExampleHolesCoords
	call IsPlayerOnDungeonWarp

ExampleHolesCoords:
	dbmapcoord  3, 16
	dbmapcoord  6, 16
	db -1 ; end
`

	triggers, err := parseDungeonWarpSourceTriggersASM("SEAFOAM_ISLANDS_B3F", "Example.asm", raw)
	if err != nil {
		t.Fatalf("parseDungeonWarpSourceTriggersASM: %v", err)
	}
	if len(triggers) != 2 {
		t.Fatalf("triggers = %#v, want 2", triggers)
	}
	if triggers[0] != (dungeonWarpSourceTrigger{
		SourceMap:      "SEAFOAM_ISLANDS_B3F",
		DestinationMap: "SEAFOAM_ISLANDS_B4F",
		WarpIndex:      1,
		X:              3,
		Y:              16,
		SourceFile:     "Example.asm",
	}) {
		t.Fatalf("first trigger = %#v, want Seafoam B3F hole #1", triggers[0])
	}
}

func TestParseConditionalDungeonWarpSourceTriggersASM(t *testing.T) {
	raw := `
PokemonMansion3FDefaultScript:
	ld hl, .holeCoords
	call .isPlayerFallingDownHole
	ld a, [wWhichDungeonWarp]
	and a
	jp z, CheckFightingMapTrainers
	cp $3
	ld a, POKEMON_MANSION_1F
	jr nz, .fellDownHoleTo1F
	ld a, POKEMON_MANSION_2F
.fellDownHoleTo1F
	ld [wDungeonWarpDestinationMap], a
	ret

.holeCoords:
	dbmapcoord 16, 14
	dbmapcoord 17, 14
	dbmapcoord 19, 14
	db -1 ; end
`

	triggers, err := parseDungeonWarpSourceTriggersASM("POKEMON_MANSION_3F", "PokemonMansion3F.asm", raw)
	if err != nil {
		t.Fatalf("parseDungeonWarpSourceTriggersASM: %v", err)
	}
	if len(triggers) != 3 {
		t.Fatalf("triggers = %#v, want 3", triggers)
	}
	if triggers[0].DestinationMap != "POKEMON_MANSION_1F" || triggers[1].DestinationMap != "POKEMON_MANSION_1F" {
		t.Fatalf("first two mansion holes = %#v, want destination 1F", triggers[:2])
	}
	if triggers[2] != (dungeonWarpSourceTrigger{
		SourceMap:      "POKEMON_MANSION_3F",
		DestinationMap: "POKEMON_MANSION_2F",
		WarpIndex:      3,
		X:              19,
		Y:              14,
		SourceFile:     "PokemonMansion3F.asm",
	}) {
		t.Fatalf("third trigger = %#v, want mansion 2F hole #3", triggers[2])
	}
}

func TestLoadDungeonHoleWarpSeedsUsesSourceDungeonWarpTables(t *testing.T) {
	seeds, err := loadDungeonHoleWarpSeeds()
	if err != nil {
		t.Fatalf("loadDungeonHoleWarpSeeds: %v", err)
	}
	if len(seeds) != 12 {
		t.Fatalf("loaded %d dungeon hole warps, want 12: %#v", len(seeds), seeds)
	}

	want := []dungeonHoleWarpSeed{
		{SourceMap: "SEAFOAM_ISLANDS_B3F", X: 3, Y: 16, DestinationMap: "SEAFOAM_ISLANDS_B4F", DestinationX: 4, DestinationY: 14},
		{SourceMap: "VICTORY_ROAD_3F", X: 23, Y: 15, DestinationMap: "VICTORY_ROAD_2F", DestinationX: 22, DestinationY: 16},
		{SourceMap: "POKEMON_MANSION_3F", X: 19, Y: 14, DestinationMap: "POKEMON_MANSION_2F", DestinationX: 18, DestinationY: 14},
	}
	for _, expected := range want {
		if !containsDungeonHoleWarpSeed(seeds, expected) {
			t.Fatalf("missing expected source-derived dungeon hole warp %#v in %#v", expected, seeds)
		}
	}
	if containsDungeonHoleWarpSeed(seeds, dungeonHoleWarpSeed{SourceMap: "SEAFOAM_ISLANDS_B3F", X: 20, Y: 17}) {
		t.Fatalf("ordinary Seafoam B3F warp (20,17) was incorrectly treated as a dungeon hole: %#v", seeds)
	}
}

func TestUpsertDungeonHoleWarpsPostgresDoesNotRewriteOrdinaryWarpRows(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE phaser_warps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_map_id INTEGER NOT NULL,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			destination_map_id INTEGER,
			destination_map TEXT,
			destination_x INTEGER,
			destination_y INTEGER,
			warp_type TEXT DEFAULT 'door',
			warp_direction TEXT
		)`,
		`INSERT INTO phaser_maps (id, name) VALUES
			(161, 'SEAFOAM_ISLANDS_B3F'),
			(162, 'SEAFOAM_ISLANDS_B4F')`,
		`INSERT INTO phaser_warps (
			source_map_id, x, y, destination_map_id, destination_map,
			destination_x, destination_y, warp_type
		) VALUES (161, 20, 17, 162, 'SEAFOAM_ISLANDS_B4F', 20, 17, 'door')`,
	)

	seeds := []dungeonHoleWarpSeed{
		{SourceMap: "SEAFOAM_ISLANDS_B3F", X: 3, Y: 16, DestinationMap: "SEAFOAM_ISLANDS_B4F", DestinationX: 4, DestinationY: 14},
	}
	if err := upsertDungeonHoleWarpsPostgres(db, seeds); err != nil {
		t.Fatalf("upsertDungeonHoleWarpsPostgres: %v", err)
	}

	var destinationX, destinationY int
	var warpType string
	if err := db.QueryRow(`
		SELECT destination_x, destination_y, warp_type
		FROM phaser_warps
		WHERE source_map_id = 161 AND x = 3 AND y = 16`).Scan(&destinationX, &destinationY, &warpType); err != nil {
		t.Fatalf("query inserted hole warp: %v", err)
	}
	if destinationX != 4 || destinationY != 14 || warpType != "carpet" {
		t.Fatalf("inserted hole warp = (%d,%d,%q), want (4,14,carpet)", destinationX, destinationY, warpType)
	}

	if err := db.QueryRow(`
		SELECT destination_x, destination_y, warp_type
		FROM phaser_warps
		WHERE source_map_id = 161 AND x = 20 AND y = 17`).Scan(&destinationX, &destinationY, &warpType); err != nil {
		t.Fatalf("query ordinary warp: %v", err)
	}
	if destinationX != 20 || destinationY != 17 || warpType != "door" {
		t.Fatalf("ordinary warp row was rewritten to (%d,%d,%q), want unchanged (20,17,door)", destinationX, destinationY, warpType)
	}
}

func containsDungeonHoleWarpSeed(seeds []dungeonHoleWarpSeed, expected dungeonHoleWarpSeed) bool {
	for _, seed := range seeds {
		if expected.SourceMap != "" && seed.SourceMap != expected.SourceMap {
			continue
		}
		if seed.X != expected.X || seed.Y != expected.Y {
			continue
		}
		if expected.DestinationMap != "" && seed.DestinationMap != expected.DestinationMap {
			continue
		}
		if expected.DestinationX != 0 && seed.DestinationX != expected.DestinationX {
			continue
		}
		if expected.DestinationY != 0 && seed.DestinationY != expected.DestinationY {
			continue
		}
		return true
	}
	return false
}
