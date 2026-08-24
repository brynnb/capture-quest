package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestImportStaticTableMapsSchemaV2DefaultMoveNames(t *testing.T) {
	source, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()

	execStatements(t, source,
		`CREATE TABLE pokemon (id INTEGER, default_move_1_id INTEGER, default_move_1_name TEXT)`,
		`INSERT INTO pokemon VALUES (1, 28, 'SAND_ATTACK')`,
	)
	execStatements(t, target,
		`CREATE TABLE phaser_pokemon (id INTEGER, default_move_1_id TEXT)`,
	)
	_, err = importStaticTable(source, target, staticTableSpec{
		SourceTable:   "pokemon",
		TargetTable:   "phaser_pokemon",
		Columns:       []string{"id", "default_move_1_id"},
		SourceColumns: []string{"id", "default_move_1_name"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var constant string
	if err := target.QueryRow(`SELECT default_move_1_id FROM phaser_pokemon`).Scan(&constant); err != nil {
		t.Fatal(err)
	}
	if constant != "SAND_ATTACK" {
		t.Fatalf("default constant = %q", constant)
	}
}

func TestValidatePokemonDefaultMovesUsesConstantName(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	execStatements(t, db,
		`CREATE TABLE phaser_pokemon (default_move_1_id TEXT, default_move_2_id TEXT, default_move_3_id TEXT, default_move_4_id TEXT)`,
		`CREATE TABLE phaser_moves (id INTEGER, constant_name TEXT, short_name TEXT)`,
		`INSERT INTO phaser_pokemon VALUES ('SAND_ATTACK', 'PSYCHIC_M', 'NO_MOVE', NULL)`,
		`INSERT INTO phaser_moves VALUES (28, 'SAND_ATTACK', 'SAND-ATTACK')`,
		`INSERT INTO phaser_moves VALUES (94, 'PSYCHIC_M', 'PSYCHIC')`,
	)
	if err := validatePokemonDefaultMovesPostgres(db); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM phaser_moves WHERE constant_name = 'PSYCHIC_M'`); err != nil {
		t.Fatal(err)
	}
	if err := validatePokemonDefaultMovesPostgres(db); err == nil || !strings.Contains(err.Error(), "PSYCHIC_M") {
		t.Fatalf("error = %v", err)
	}
}

func TestImportTrainerClassesNormalizesBCD3Money(t *testing.T) {
	source, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()

	execStatements(t, source,
		`CREATE TABLE trainer_classes (id INTEGER, constant_name TEXT, display_name TEXT, base_money INTEGER, is_gym_leader INTEGER, is_elite_four INTEGER, is_rival INTEGER)`,
		`INSERT INTO trainer_classes VALUES (9, 'RIVAL1', 'RIVAL', 3500, 0, 0, 1)`,
	)
	execStatements(t, target,
		`CREATE TABLE phaser_trainer_classes (id INTEGER, constant_name TEXT, display_name TEXT, base_money INTEGER, is_gym_leader INTEGER, is_elite_four INTEGER, is_rival INTEGER)`,
	)

	_, err = importStaticTable(source, target, staticTableSpec{
		SourceTable: "trainer_classes",
		TargetTable: "phaser_trainer_classes",
		Columns:     []string{"id", "constant_name", "display_name", "base_money", "is_gym_leader", "is_elite_four", "is_rival"},
		Transform:   normalizeTrainerClassImportValues,
	})
	if err != nil {
		t.Fatalf("import trainer classes: %v", err)
	}

	var baseMoney int
	if err := target.QueryRow(`SELECT base_money FROM phaser_trainer_classes WHERE constant_name = 'RIVAL1'`).Scan(&baseMoney); err != nil {
		t.Fatal(err)
	}
	if baseMoney != 35 {
		t.Fatalf("RIVAL1 base_money = %d, want 35", baseMoney)
	}
}

func TestValidatePokemonDefaultMovesRejectsMissingConstants(t *testing.T) {
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	execStatements(t, raw,
		`CREATE TABLE phaser_pokemon (default_move_1_id TEXT, default_move_2_id TEXT, default_move_3_id TEXT, default_move_4_id TEXT)`,
		`CREATE TABLE phaser_moves (id INTEGER, constant_name TEXT)`,
		`INSERT INTO phaser_pokemon VALUES ('POISON_STING', 'STRING_SHOT', 'NO_MOVE', NULL)`,
		`INSERT INTO phaser_moves VALUES (40, 'POISON_STING')`,
	)

	err = validatePokemonDefaultMovesPostgres(raw)
	if err == nil || err.Error() != "Pokemon defaults reference missing move constants: STRING_SHOT" {
		t.Fatalf("validation error = %v, want missing STRING_SHOT", err)
	}

	if _, err := raw.Exec(`INSERT INTO phaser_moves VALUES (81, 'STRING_SHOT')`); err != nil {
		t.Fatal(err)
	}
	if err := validatePokemonDefaultMovesPostgres(raw); err != nil {
		t.Fatalf("validation with complete moves: %v", err)
	}
}

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
		`CREATE TABLE phaser_tiles (
			x INTEGER,
			y INTEGER,
			local_x INTEGER,
			local_y INTEGER,
			map_id INTEGER,
			source_map_id INTEGER,
			original_local_x INTEGER,
			original_local_y INTEGER,
			original_source_map_id INTEGER
		)`,
		`CREATE TABLE phaser_objects (map_id INTEGER, x INTEGER, y INTEGER, local_x INTEGER, local_y INTEGER)`,
		`CREATE TABLE phaser_warp_events (id INTEGER PRIMARY KEY, map_id INTEGER, x INTEGER, y INTEGER, dest_map TEXT)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, x INTEGER, y INTEGER, destination_map TEXT)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES (17, 'ROUTE_6', 1)`,
		`INSERT INTO phaser_tiles (x, y, local_x, local_y, map_id, source_map_id) VALUES
			(180, -90, 0, 0, NULL, 17),
			(189, -83, 9, 7, NULL, 17)`,
		`INSERT INTO phaser_warp_events (id, map_id, x, y, dest_map) VALUES
			(441, 17, 10, 7, 'ROUTE_6_GATE')`,
		`INSERT INTO phaser_warps (id, source_map_id, x, y, destination_map) VALUES
			(85, 17, 10, 7, 'ROUTE_6_GATE'),
			(86, 17, 190, -83, 'ROUTE_6_GATE')`,
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

	if err := db.QueryRow(`SELECT x, y FROM phaser_warps WHERE id = 86`).Scan(&x, &y); err != nil {
		t.Fatalf("query already-global baked warp: %v", err)
	}
	if x != 190 || y != -83 {
		t.Fatalf("already-global Route 6 warp = (%d,%d), want unchanged global (190,-83)", x, y)
	}
}

func TestMergeImportedTilesPreservesEditedCurrentState(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db, `
		CREATE TABLE phaser_tiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			tile_image_id INTEGER NOT NULL,
			local_x INTEGER,
			local_y INTEGER,
			map_id INTEGER,
			source_map_id INTEGER,
			collision_type INTEGER DEFAULT 0,
			raw_foot_tile_id INTEGER,
			talk_over_tile BOOLEAN NOT NULL DEFAULT false,
			encounter_area_id INTEGER,
			is_native_game_data BOOLEAN NOT NULL DEFAULT false,
			coordinate_origin TEXT NOT NULL DEFAULT 'user',
			content_origin TEXT NOT NULL DEFAULT 'user',
			is_original_tile_location INTEGER NOT NULL DEFAULT 0,
			has_tile_edit INTEGER NOT NULL DEFAULT 0,
			is_tile_erased INTEGER NOT NULL DEFAULT 0,
			original_tile_image_id INTEGER,
			original_collision_type INTEGER,
			original_raw_foot_tile_id INTEGER,
			original_talk_over_tile BOOLEAN,
			original_encounter_area_id INTEGER,
			original_local_x INTEGER,
			original_local_y INTEGER,
			original_source_map_id INTEGER,
			is_user_placed INTEGER NOT NULL DEFAULT 0,
			placed_by_char_id INTEGER,
			placed_at TEXT,
			last_edited_by_char_id INTEGER,
			last_edited_at TEXT,
			last_edit_source TEXT
		);
		CREATE UNIQUE INDEX phaser_tiles_coord_unique_idx ON phaser_tiles (x, y, COALESCE(map_id, -1));
		INSERT INTO phaser_tiles (
			x, y, tile_image_id, map_id, collision_type,
			is_original_tile_location, has_tile_edit, is_tile_erased,
			original_tile_image_id, original_collision_type, last_edit_source
		) VALUES
			(1, 1, 9, NULL, 0, 1, 1, 0, 1, 1, 'admin_editor'),
			(4, 4, 4, NULL, 1, 1, 0, 0, 4, 1, NULL),
			(8, 8, 8, NULL, 1, 0, 1, 0, NULL, NULL, 'admin_editor');
	`)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`
		CREATE TEMP TABLE phaser_tiles_import_stage (
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			tile_image_id INTEGER NOT NULL,
			local_x INTEGER,
			local_y INTEGER,
			map_id INTEGER,
			source_map_id INTEGER,
			collision_type INTEGER DEFAULT 0,
			raw_foot_tile_id INTEGER,
			talk_over_tile BOOLEAN NOT NULL DEFAULT false
		)`,
	); err != nil {
		t.Fatalf("create stage: %v", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO phaser_tiles_import_stage
			(x, y, tile_image_id, local_x, local_y, map_id, source_map_id, collision_type, raw_foot_tile_id, talk_over_tile)
		VALUES
			(1, 1, 2, 1, 1, NULL, 17, 1, NULL, false),
			(2, 2, 3, 2, 2, NULL, 17, 1, NULL, false)`,
	); err != nil {
		t.Fatalf("seed stage: %v", err)
	}
	if err := mergeImportedTilesPostgres(tx); err != nil {
		t.Fatalf("mergeImportedTilesPostgres: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	var currentTile, originalTile, hasEdit int
	if err := db.QueryRow(`
		SELECT tile_image_id, original_tile_image_id, has_tile_edit
		FROM phaser_tiles
		WHERE map_id IS NULL AND x = 1 AND y = 1`,
	).Scan(&currentTile, &originalTile, &hasEdit); err != nil {
		t.Fatalf("query edited imported tile: %v", err)
	}
	if currentTile != 9 || originalTile != 2 || hasEdit != 1 {
		t.Fatalf("edited row current/original/hasEdit = %d/%d/%d, want 9/2/1", currentTile, originalTile, hasEdit)
	}

	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM phaser_tiles WHERE map_id IS NULL AND x = 4 AND y = 4`).Scan(&rows); err != nil {
		t.Fatalf("count stale base row: %v", err)
	}
	if rows != 0 {
		t.Fatalf("stale unedited base rows = %d, want 0", rows)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM phaser_tiles WHERE map_id IS NULL AND x = 8 AND y = 8`).Scan(&rows); err != nil {
		t.Fatalf("count added edit row: %v", err)
	}
	if rows != 1 {
		t.Fatalf("added edited rows = %d, want 1", rows)
	}
	if err := db.QueryRow(`SELECT tile_image_id, original_tile_image_id FROM phaser_tiles WHERE map_id IS NULL AND x = 2 AND y = 2`).Scan(&currentTile, &originalTile); err != nil {
		t.Fatalf("query new imported row: %v", err)
	}
	if currentTile != 3 || originalTile != 3 {
		t.Fatalf("new imported row current/original = %d/%d, want 3/3", currentTile, originalTile)
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
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, source_warp_index INTEGER, x INTEGER, y INTEGER, destination_map_id INTEGER, destination_map TEXT, destination_x INTEGER, destination_y INTEGER, destination_kind TEXT)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES
			(17, 'ROUTE_6', 1),
			(73, 'ROUTE_6_GATE', 0)`,
		`INSERT INTO phaser_warp_events (id, map_name, map_id, x, y, dest_map, dest_warp_index) VALUES
			(439, 'Route6', 17, 9, 1, 'ROUTE_6_GATE', 3),
			(440, 'Route6', 17, 10, 1, 'ROUTE_6_GATE', 3),
			(441, 'Route6', 17, 10, 7, 'ROUTE_6_GATE', 1),
			(445, 'Route6Gate', 73, 3, 0, 'LAST_MAP', 2)`,
		`INSERT INTO phaser_warps VALUES
			(460, 73, 1, 3, 0, NULL, 'LAST_MAP', NULL, NULL, 'last-map')`,
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

func TestResolveLastMapWarpDestinationsIgnoresFixedInternalIncomingMap(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT, is_overworld INTEGER)`,
		`CREATE TABLE phaser_warp_events (id INTEGER PRIMARY KEY, map_name TEXT, map_id INTEGER, x INTEGER, y INTEGER, dest_map TEXT, dest_warp_index INTEGER)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, source_warp_index INTEGER, x INTEGER, y INTEGER, destination_map_id INTEGER, destination_map TEXT, destination_x INTEGER, destination_y INTEGER, destination_kind TEXT)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES
			(17, 'ROUTE_6', 1),
			(74, 'UNDERGROUND_PATH_ROUTE_6', 0),
			(119, 'UNDERGROUND_PATH_NORTH_SOUTH', 0)`,
		`INSERT INTO phaser_warp_events (id, map_name, map_id, x, y, dest_map, dest_warp_index) VALUES
			(442, 'Route6', 17, 17, 13, 'UNDERGROUND_PATH_ROUTE_6', 1),
			(720, 'UndergroundPathNorthSouth', 119, 2, 41, 'UNDERGROUND_PATH_ROUTE_6', 3),
			(724, 'UndergroundPathRoute6', NULL, 3, 7, 'LAST_MAP', 4),
			(725, 'UndergroundPathRoute6', NULL, 4, 7, 'LAST_MAP', 4)`,
		`INSERT INTO phaser_warps VALUES
			(114, 74, 1, 3, 7, NULL, 'LAST_MAP', NULL, NULL, 'last-map'),
			(115, 74, 2, 4, 7, NULL, 'LAST_MAP', NULL, NULL, 'last-map'),
			(116, 74, 3, 2, 3, 119, 'UNDERGROUND_PATH_NORTH_SOUTH', 2, 41, 'fixed')`,
	)

	if err := resolveLastMapWarpDestinationsPostgres(db); err != nil {
		t.Fatalf("resolveLastMapWarpDestinationsPostgres: %v", err)
	}

	rows, err := db.Query(`SELECT id, destination_map_id, destination_map FROM phaser_warps WHERE id IN (114, 115) ORDER BY id`)
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
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, source_warp_index INTEGER, x INTEGER, y INTEGER, destination_map_id INTEGER, destination_map TEXT, destination_x INTEGER, destination_y INTEGER, destination_kind TEXT)`,
		`INSERT INTO phaser_maps (id, name, is_overworld) VALUES
			(8, 'CINNABAR_ISLAND', 1),
			(167, 'CINNABAR_LAB', 0),
			(168, 'CINNABAR_LAB_TRADE_ROOM', 0)`,
		`INSERT INTO phaser_warp_events (id, map_name, map_id, x, y, dest_map, dest_warp_index) VALUES
			(125, 'CinnabarIsland', 8, 6, 9, 'CINNABAR_LAB', 1),
			(128, 'CinnabarLab', 167, 2, 7, 'LAST_MAP', 3),
			(137, 'CinnabarLabTradeRoom', 168, 2, 7, 'CINNABAR_LAB', 3)`,
		`INSERT INTO phaser_warps VALUES
			(63, 167, 1, 2, 7, 8, 'CINNABAR_ISLAND', 6, 9, 'last-map')`,
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

func TestResolveImportedWarpDestinationsResolvesStarterHouseExit(t *testing.T) {
	source, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()

	execStatements(t, source,
		`CREATE TABLE overworld_map_positions (map_name TEXT, x_offset INTEGER, y_offset INTEGER)`,
		`INSERT INTO overworld_map_positions VALUES ('PalletTown', 0, 0)`,
	)
	execStatements(t, target,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT, is_overworld INTEGER)`,
		`CREATE TABLE phaser_warp_events (id INTEGER PRIMARY KEY, map_name TEXT, map_id INTEGER, x INTEGER, y INTEGER, dest_map TEXT, dest_warp_index INTEGER)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, source_warp_index INTEGER, x INTEGER, y INTEGER, destination_map_id INTEGER, destination_map TEXT, destination_x INTEGER, destination_y INTEGER, destination_kind TEXT)`,
		`INSERT INTO phaser_maps VALUES
			(0, 'PALLET_TOWN', 1),
			(37, 'REDS_HOUSE_1F', 0),
			(38, 'REDS_HOUSE_2F', 0)`,
		`INSERT INTO phaser_warp_events VALUES
			(245, 'PalletTown', 0, 5, 5, 'REDS_HOUSE_1F', 1),
			(300, 'RedsHouse1F', 37, 2, 7, 'LAST_MAP', 1),
			(301, 'RedsHouse1F', 37, 3, 7, 'LAST_MAP', 1),
			(302, 'RedsHouse1F', 37, 7, 1, 'REDS_HOUSE_2F', 1),
			(303, 'RedsHouse2F', 38, 7, 1, 'REDS_HOUSE_1F', 3)`,
		`INSERT INTO phaser_warps VALUES
			(300, 37, 1, 2, 7, NULL, 'LAST_MAP', NULL, NULL, 'last-map'),
			(301, 37, 2, 3, 7, NULL, 'LAST_MAP', NULL, NULL, 'last-map'),
			(302, 37, 3, 7, 1, 38, 'REDS_HOUSE_2F', 7, 1, 'fixed')`,
	)

	if err := validateDeterministicLastMapWarpDestinationsPostgres(target); err == nil {
		t.Fatal("expected unresolved deterministic starter-house exits to fail validation")
	}
	if err := resolveImportedWarpDestinationsPostgres(source, target); err != nil {
		t.Fatal(err)
	}

	rows, err := target.Query(`
		SELECT destination_map_id, destination_map, destination_x, destination_y
		FROM phaser_warps WHERE id IN (300, 301) ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var destinationMapID, destinationX, destinationY int
		var destinationMap string
		if err := rows.Scan(&destinationMapID, &destinationMap, &destinationX, &destinationY); err != nil {
			t.Fatal(err)
		}
		if destinationMapID != 0 || destinationMap != "PALLET_TOWN" || destinationX != 5 || destinationY != 5 {
			t.Fatalf(
				"starter house exit = (%d,%s,%d,%d), want (0,PALLET_TOWN,5,5)",
				destinationMapID,
				destinationMap,
				destinationX,
				destinationY,
			)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("resolved starter house exits = %d, want 2", count)
	}
}

func TestResolveLastMapWarpDestinationsLeavesRoute22GateDynamic(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	execStatements(t, db,
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT, is_overworld INTEGER)`,
		`CREATE TABLE phaser_warp_events (id INTEGER PRIMARY KEY, map_name TEXT, map_id INTEGER, x INTEGER, y INTEGER, dest_map TEXT, dest_warp_index INTEGER)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, source_warp_index INTEGER, x INTEGER, y INTEGER, destination_map_id INTEGER, destination_map TEXT, destination_x INTEGER, destination_y INTEGER, destination_kind TEXT)`,
		`INSERT INTO phaser_maps VALUES
			(33, 'ROUTE_22', 1),
			(34, 'ROUTE_23', 1),
			(193, 'ROUTE_22_GATE', 0)`,
		`INSERT INTO phaser_warp_events VALUES
			(1, 'Route22', 33, 8, 5, 'ROUTE_22_GATE', 1),
			(2, 'Route23', 34, 7, 139, 'ROUTE_22_GATE', 3),
			(3, 'Route23', 34, 8, 139, 'ROUTE_22_GATE', 4)`,
		`INSERT INTO phaser_warps VALUES
			(1, 193, 1, 4, 7, NULL, 'LAST_MAP', NULL, NULL, 'last-map'),
			(2, 193, 2, 5, 7, NULL, 'LAST_MAP', NULL, NULL, 'last-map'),
			(3, 193, 3, 4, 0, NULL, 'LAST_MAP', NULL, NULL, 'last-map'),
			(4, 193, 4, 5, 0, NULL, 'LAST_MAP', NULL, NULL, 'last-map')`,
	)

	if err := resolveLastMapWarpDestinationsPostgres(db); err != nil {
		t.Fatal(err)
	}

	var total, unresolved int
	if err := db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN destination_map_id IS NULL
			AND destination_map = 'LAST_MAP'
			AND destination_x IS NULL
			AND destination_y IS NULL THEN 1 ELSE 0 END)
		FROM phaser_warps WHERE source_map_id = 193`).Scan(&total, &unresolved); err != nil {
		t.Fatal(err)
	}
	if total != 4 || unresolved != 4 {
		t.Fatalf("Route 22 Gate dynamic exits = %d/%d unresolved, want 4/4", unresolved, total)
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
