package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestResolveWarpDestinationUpdatesRoute18OverworldEntrance(t *testing.T) {
	updates := resolveWarpDestinationUpdates(
		map[int]importedMapInfo{
			29:  {Name: "ROUTE_18", IsOverworld: true},
			190: {Name: "ROUTE_18_GATE_1F", IsOverworld: false},
		},
		map[string]coordinateOffset{
			"route18": {X: 60, Y: 44},
		},
		[]importedWarpEvent{
			{MapID: 29, MapName: "Route18", X: 33, Y: 8, DestWarpIndex: 1},
			{MapID: 190, MapName: "Route18Gate1F", X: 0, Y: 4, DestWarpIndex: 1},
		},
		[]importedRuntimeWarp{
			{ID: 294, SourceMapID: 29, X: 93, Y: 52, DestinationMapID: 190, HasDestination: true},
		},
	)

	if len(updates) != 1 {
		t.Fatalf("updates = %#v, want one update", updates)
	}
	if updates[0] != (warpDestinationUpdate{WarpID: 294, X: 0, Y: 4}) {
		t.Fatalf("update = %#v, want Route18Gate1F warp 1 destination", updates[0])
	}
}

func TestResolveWarpDestinationUpdatesRoute18GateExitToOverworld(t *testing.T) {
	updates := resolveWarpDestinationUpdates(
		map[int]importedMapInfo{
			29:  {Name: "ROUTE_18", IsOverworld: true},
			190: {Name: "ROUTE_18_GATE_1F", IsOverworld: false},
		},
		map[string]coordinateOffset{
			"route18": {X: 60, Y: 44},
		},
		[]importedWarpEvent{
			{MapID: 29, MapName: "Route18", X: 33, Y: 8, DestWarpIndex: 1},
			{MapID: 190, MapName: "Route18Gate1F", X: 0, Y: 4, DestWarpIndex: 1},
		},
		[]importedRuntimeWarp{
			{ID: 501, SourceMapID: 190, X: 0, Y: 4, DestinationMapID: 29, HasDestination: true},
		},
	)

	if len(updates) != 1 {
		t.Fatalf("updates = %#v, want one update", updates)
	}
	if updates[0] != (warpDestinationUpdate{WarpID: 501, X: 93, Y: 52}) {
		t.Fatalf("update = %#v, want Route18 global overworld destination", updates[0])
	}
}

func TestResolveWarpDestinationUpdatesSSAnneDockAndFirstFloor(t *testing.T) {
	updates := resolveWarpDestinationUpdates(
		map[int]importedMapInfo{
			5:  {Name: "VERMILION_CITY", IsOverworld: true},
			94: {Name: "VERMILION_DOCK", IsOverworld: false},
			95: {Name: "SS_ANNE_1F", IsOverworld: false},
		},
		map[string]coordinateOffset{
			"vermilioncity": {X: 170, Y: -54},
		},
		[]importedWarpEvent{
			{MapID: 5, MapName: "VermilionCity", X: 11, Y: 3, DestWarpIndex: 1},
			{MapID: 5, MapName: "VermilionCity", X: 9, Y: 13, DestWarpIndex: 1},
			{MapID: 5, MapName: "VermilionCity", X: 23, Y: 13, DestWarpIndex: 1},
			{MapID: 5, MapName: "VermilionCity", X: 12, Y: 19, DestWarpIndex: 1},
			{MapID: 5, MapName: "VermilionCity", X: 23, Y: 19, DestWarpIndex: 1},
			{MapID: 5, MapName: "VermilionCity", X: 18, Y: 31, DestWarpIndex: 1},
			{MapID: 5, MapName: "VermilionCity", X: 19, Y: 31, DestWarpIndex: 1},
			{MapID: 94, MapName: "VermilionDock", X: 14, Y: 0, DestWarpIndex: 6},
			{MapID: 94, MapName: "VermilionDock", X: 14, Y: 2, DestWarpIndex: 2},
			{MapID: 95, MapName: "SSAnne1F", X: 26, Y: 0, DestWarpIndex: 2},
			{MapID: 95, MapName: "SSAnne1F", X: 27, Y: 0, DestWarpIndex: 2},
		},
		[]importedRuntimeWarp{
			{ID: 235, SourceMapID: 94, X: 14, Y: 0, DestinationMapID: 5, HasDestination: true},
			{ID: 236, SourceMapID: 94, X: 14, Y: 2, DestinationMapID: 95, HasDestination: true},
			{ID: 335, SourceMapID: 95, X: 26, Y: 0, DestinationMapID: 94, HasDestination: true},
			{ID: 336, SourceMapID: 95, X: 27, Y: 0, DestinationMapID: 94, HasDestination: true},
		},
	)

	got := make(map[int]warpDestinationUpdate)
	for _, update := range updates {
		got[update.WarpID] = update
	}

	want := map[int]warpDestinationUpdate{
		235: {WarpID: 235, X: 188, Y: -23},
		236: {WarpID: 236, X: 27, Y: 0},
		335: {WarpID: 335, X: 14, Y: 2},
		336: {WarpID: 336, X: 14, Y: 2},
	}
	for warpID, expected := range want {
		if got[warpID] != expected {
			t.Fatalf("warp %d resolved to %#v, want %#v (all updates: %#v)", warpID, got[warpID], expected, updates)
		}
	}
}

func TestClearWarpDestinationCoordinatePlaceholdersPostgres(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, destination_x INTEGER, destination_y INTEGER)`,
		`INSERT INTO phaser_warps (id, destination_x, destination_y) VALUES
			(1, 0, 0),
			(2, 14, 2),
			(3, NULL, NULL)`,
	)

	if err := clearWarpDestinationCoordinatePlaceholdersPostgres(db); err != nil {
		t.Fatalf("clearWarpDestinationCoordinatePlaceholdersPostgres: %v", err)
	}

	var destinationX, destinationY sql.NullInt64
	if err := db.QueryRow(`SELECT destination_x, destination_y FROM phaser_warps WHERE id = 1`).Scan(&destinationX, &destinationY); err != nil {
		t.Fatalf("query placeholder warp: %v", err)
	}
	if destinationX.Valid || destinationY.Valid {
		t.Fatalf("placeholder destination = (%v,%v), want NULL,NULL", destinationX, destinationY)
	}

	if err := db.QueryRow(`SELECT destination_x, destination_y FROM phaser_warps WHERE id = 2`).Scan(&destinationX, &destinationY); err != nil {
		t.Fatalf("query resolved warp: %v", err)
	}
	if !destinationX.Valid || !destinationY.Valid || destinationX.Int64 != 14 || destinationY.Int64 != 2 {
		t.Fatalf("resolved destination = (%v,%v), want 14,2", destinationX, destinationY)
	}
}

func TestBakeOverworldCoordinatesUsesTileSourceMapID(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT, is_overworld INTEGER)`,
		`CREATE TABLE phaser_tiles (x INTEGER, y INTEGER, local_x INTEGER, local_y INTEGER, map_id INTEGER, source_map_id INTEGER)`,
		`CREATE TABLE phaser_objects (map_id INTEGER, x INTEGER, y INTEGER, local_x INTEGER, local_y INTEGER)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, x INTEGER, y INTEGER)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES (17, 'ROUTE_6', 1)`,
		`INSERT INTO phaser_tiles (x, y, local_x, local_y, map_id, source_map_id) VALUES
			(180, -90, 0, 0, NULL, 17),
			(189, -83, 9, 7, NULL, 17)`,
		`INSERT INTO phaser_warps (id, source_map_id, x, y) VALUES (85, 17, 10, 7)`,
	)

	if err := bakeOverworldCoordinatesPostgres(db); err != nil {
		t.Fatalf("bakeOverworldCoordinatesPostgres: %v", err)
	}

	var x, y int
	if err := db.QueryRow(`SELECT x, y FROM phaser_warps WHERE id = 85`).Scan(&x, &y); err != nil {
		t.Fatalf("query baked warp: %v", err)
	}
	if x != 190 || y != -83 {
		t.Fatalf("baked Route 6 warp = (%d,%d), want global (190,-83)", x, y)
	}
}

func TestResolveLastMapWarpDestinationsFallsBackToUniqueIncomingMap(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT, is_overworld INTEGER)`,
		`CREATE TABLE phaser_warp_events (id INTEGER PRIMARY KEY, map_name TEXT, map_id INTEGER, x INTEGER, y INTEGER, dest_map TEXT, dest_warp_index INTEGER)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, x INTEGER, y INTEGER, destination_map_id INTEGER, destination_map TEXT, destination_x INTEGER, destination_y INTEGER)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES
			(17, 'ROUTE_6', 1),
			(73, 'ROUTE_6_GATE', 0)`,
		`INSERT INTO phaser_warp_events (id, map_name, map_id, x, y, dest_map, dest_warp_index) VALUES
			(439, 'Route6', 17, 9, 1, 'ROUTE_6_GATE', 3),
			(440, 'Route6', 17, 10, 1, 'ROUTE_6_GATE', 3),
			(441, 'Route6', 17, 10, 7, 'ROUTE_6_GATE', 1),
			(445, 'Route6Gate', 73, 3, 0, 'LAST_MAP', 2)`,
		`INSERT INTO phaser_warps (id, source_map_id, x, y, destination_map_id, destination_map) VALUES
			(460, 73, 3, 0, NULL, 'LAST_MAP')`,
	)

	if err := resolveLastMapWarpDestinationsPostgres(db); err != nil {
		t.Fatalf("resolveLastMapWarpDestinationsPostgres: %v", err)
	}

	var destinationMapID int
	var destinationMap string
	if err := db.QueryRow(`SELECT destination_map_id, destination_map FROM phaser_warps WHERE id = 460`).Scan(&destinationMapID, &destinationMap); err != nil {
		t.Fatalf("query resolved warp: %v", err)
	}
	if destinationMapID != 17 || destinationMap != "ROUTE_6" {
		t.Fatalf("resolved destination = (%d, %q), want (17, ROUTE_6)", destinationMapID, destinationMap)
	}
}

func TestResolveLastMapWarpDestinationsMatchesNullMapIDBySourceMapName(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT, is_overworld INTEGER)`,
		`CREATE TABLE phaser_warp_events (id INTEGER PRIMARY KEY, map_name TEXT, map_id INTEGER, x INTEGER, y INTEGER, dest_map TEXT, dest_warp_index INTEGER)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, x INTEGER, y INTEGER, destination_map_id INTEGER, destination_map TEXT, destination_x INTEGER, destination_y INTEGER)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES
			(17, 'ROUTE_6', 1),
			(74, 'UNDERGROUND_PATH_ROUTE_6', 0),
			(119, 'UNDERGROUND_PATH_NORTH_SOUTH', 0)`,
		`INSERT INTO phaser_warp_events (id, map_name, map_id, x, y, dest_map, dest_warp_index) VALUES
			(442, 'Route6', 17, 17, 13, 'UNDERGROUND_PATH_ROUTE_6', 1),
			(720, 'UndergroundPathNorthSouth', 119, 2, 41, 'UNDERGROUND_PATH_ROUTE_6', 3),
			(724, 'UndergroundPathRoute6', NULL, 3, 7, 'LAST_MAP', 4),
			(725, 'UndergroundPathRoute6', NULL, 4, 7, 'LAST_MAP', 4)`,
		`INSERT INTO phaser_warps (id, source_map_id, x, y, destination_map_id, destination_map) VALUES
			(114, 74, 3, 7, NULL, 'LAST_MAP'),
			(115, 74, 4, 7, NULL, 'LAST_MAP')`,
	)

	if err := resolveLastMapWarpDestinationsPostgres(db); err != nil {
		t.Fatalf("resolveLastMapWarpDestinationsPostgres: %v", err)
	}

	rows, err := db.Query(`SELECT id, destination_map_id, destination_map FROM phaser_warps ORDER BY id`)
	if err != nil {
		t.Fatalf("query resolved warps: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id               int
			destinationMapID int
			destinationMap   string
		)
		if err := rows.Scan(&id, &destinationMapID, &destinationMap); err != nil {
			t.Fatalf("scan resolved warp: %v", err)
		}
		if destinationMapID != 17 || destinationMap != "ROUTE_6" {
			t.Fatalf("warp %d resolved destination = (%d, %q), want (17, ROUTE_6)", id, destinationMapID, destinationMap)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read resolved warps: %v", err)
	}
}

func TestResolveLastMapWarpDestinationsDoesNotOverwriteConcreteDestination(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT, is_overworld INTEGER)`,
		`CREATE TABLE phaser_warp_events (id INTEGER PRIMARY KEY, map_name TEXT, map_id INTEGER, x INTEGER, y INTEGER, dest_map TEXT, dest_warp_index INTEGER)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, x INTEGER, y INTEGER, destination_map_id INTEGER, destination_map TEXT, destination_x INTEGER, destination_y INTEGER)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES
			(8, 'CINNABAR_ISLAND', 1),
			(167, 'CINNABAR_LAB', 0),
			(168, 'CINNABAR_LAB_TRADE_ROOM', 0)`,
		`INSERT INTO phaser_warp_events (id, map_name, map_id, x, y, dest_map, dest_warp_index) VALUES
			(125, 'CinnabarIsland', 8, 6, 9, 'CINNABAR_LAB', 1),
			(128, 'CinnabarLab', 167, 2, 7, 'LAST_MAP', 3),
			(137, 'CinnabarLabTradeRoom', 168, 2, 7, 'CINNABAR_LAB', 3)`,
		`INSERT INTO phaser_warps (id, source_map_id, x, y, destination_map_id, destination_map) VALUES
			(63, 167, 2, 7, 8, 'CINNABAR_ISLAND')`,
	)

	if err := resolveLastMapWarpDestinationsPostgres(db); err != nil {
		t.Fatalf("resolveLastMapWarpDestinationsPostgres: %v", err)
	}

	var destinationMapID int
	var destinationMap string
	if err := db.QueryRow(`SELECT destination_map_id, destination_map FROM phaser_warps WHERE id = 63`).Scan(&destinationMapID, &destinationMap); err != nil {
		t.Fatalf("query resolved warp: %v", err)
	}
	if destinationMapID != 8 || destinationMap != "CINNABAR_ISLAND" {
		t.Fatalf("resolved destination = (%d, %q), want original concrete Cinnabar Island destination", destinationMapID, destinationMap)
	}
}

func execStatements(t *testing.T, db *sql.DB, statements ...string) {
	t.Helper()
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec %q: %v", statement, err)
		}
	}
}
