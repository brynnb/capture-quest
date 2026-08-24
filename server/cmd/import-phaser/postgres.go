package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"capturequest/internal/phaserdata"
	"capturequest/internal/pokebattle"
)

var phaserMoveImportColumns = []string{
	"id", "constant_name", "name", "short_name", "effect", "power", "type", "accuracy", "pp",
	"battle_animation", "battle_sound", "battle_sound_pitch", "battle_sound_tempo",
	"battle_subanimation", "battle_tileset", "battle_delay", "is_hm", "field_move_effect",
}

func importPhaserToPostgres(sqlite, pg *sql.DB, context extractorImportContext) error {
	if err := ensurePostgresSchema(pg); err != nil {
		return err
	}
	if err := ensurePhaserTileMutationColumnsPostgres(pg); err != nil {
		return err
	}
	if err := truncateImportedPostgresTables(pg); err != nil {
		return err
	}
	if err := persistExtractorImportContext(pg, context); err != nil {
		return err
	}

	if _, err := importStaticTable(sqlite, pg, staticTableSpec{
		SourceTable: "maps",
		TargetTable: "phaser_maps",
		Columns: []string{
			"id", "name", "width", "height", "tileset_id", "is_overworld",
			"north_connection", "south_connection", "west_connection", "east_connection",
		},
	}); err != nil {
		return err
	}
	tileImageRawFootTileIDs, err := importTileImagesPostgres(sqlite, pg)
	if err != nil {
		return err
	}
	if err := importTilesPostgres(sqlite, pg, tileImageRawFootTileIDs); err != nil {
		return err
	}

	specs := []staticTableSpec{
		{
			SourceTable: "objects",
			TargetTable: "phaser_objects",
			Columns: []string{
				"id", "x", "y", "map_id", "object_type", "sprite_name", "name", "item_id",
				"action_type", "action_direction", "local_x", "local_y", "movement_type",
				"text", "trainer_class", "trainer_party_index",
			},
		},
		{
			SourceTable: "warps",
			TargetTable: "phaser_warps",
			Columns: []string{
				"id", "source_map_id", "source_warp_index", "x", "y", "destination_map_id", "destination_map",
				"destination_x", "destination_y", "destination_kind", "destination_warp_id",
			},
		},
		{
			SourceTable: "items",
			TargetTable: "phaser_items",
			Columns: []string{
				"id", "name", "short_name", "price", "is_usable", "uses_party_menu",
				"vending_price", "move_id", "is_guard_drink", "is_key_item",
			},
		},
		{
			SourceTable: "pokemon",
			TargetTable: "phaser_pokemon",
			Columns: []string{
				"id", "name", "hp", "atk", "def", "spd", "spc", "type_1", "type_2",
				"catch_rate", "base_exp", "default_move_1_id", "default_move_2_id",
				"default_move_3_id", "default_move_4_id", "base_cry", "cry_pitch",
				"cry_length", "pokedex_type", "height", "weight", "pokedex_text",
				"evolve_level", "evolve_pokemon", "evolves_from_trade", "icon_image", "palette_type",
			},
			SourceColumns: []string{
				"id", "name", "hp", "atk", "def", "spd", "spc", "type_1", "type_2",
				"catch_rate", "base_exp", "default_move_1_name", "default_move_2_name",
				"default_move_3_name", "default_move_4_name", "base_cry", "cry_pitch",
				"cry_length", "pokedex_type", "height", "weight", "pokedex_text",
				"evolve_level", "evolve_pokemon", "evolves_from_trade", "icon_image", "palette_type",
			},
		},
		{
			SourceTable: "moves",
			TargetTable: "phaser_moves",
			Columns:     phaserMoveImportColumns,
		},
		{
			SourceTable: "dialogue_text",
			TargetTable: "phaser_dialogue_text",
			Columns:     []string{"id", "label", "source_file", "dialogue"},
		},
		{
			SourceTable: "text_pointers",
			TargetTable: "phaser_text_pointers",
			Columns: []string{
				"id", "map_name", "text_constant", "local_label", "dialogue_label",
				"pointer_index", "is_trainer",
			},
		},
		{
			SourceTable: "pokemon_learnset",
			TargetTable: "phaser_pokemon_learnset",
			Columns:     []string{"id", "pokemon_id", "pokemon_name", "level", "move_name", "move_id"},
		},
		{
			SourceTable: "pokemon_tmhm",
			TargetTable: "phaser_pokemon_tmhm",
			Columns:     []string{"id", "pokemon_id", "pokemon_name", "tm_hm_name", "move_name", "move_id", "is_hm"},
		},
		{
			SourceTable: "wild_encounters",
			TargetTable: "phaser_wild_encounters",
			Columns: []string{
				"id", "map_name", "map_id", "encounter_type", "encounter_rate",
				"slot_index", "pokemon_name", "level", "version",
			},
			Where: "version IN (?, 'both')",
			Args:  []any{context.ReleaseCode},
		},
		{
			SourceTable: "encounter_slots",
			TargetTable: "phaser_encounter_slots",
			Columns:     []string{"id", "slot_index", "probability", "cumulative_probability"},
		},
		{
			SourceTable: "trainer_classes",
			TargetTable: "phaser_trainer_classes",
			Columns:     []string{"id", "constant_name", "display_name", "base_money", "is_gym_leader", "is_elite_four", "is_rival"},
			Transform:   normalizeTrainerClassImportValues,
		},
		{
			SourceTable: "trainer_parties",
			TargetTable: "phaser_trainer_parties",
			Columns:     []string{"id", "trainer_class_id", "party_index", "location_comment", "is_variable_level"},
		},
		{
			SourceTable: "trainer_party_pokemon",
			TargetTable: "phaser_trainer_party_pokemon",
			Columns:     []string{"id", "trainer_party_id", "slot_index", "pokemon_name", "level"},
		},
		{
			SourceTable: "hidden_items",
			TargetTable: "phaser_hidden_items",
			Columns:     []string{"id", "map_constant", "map_id", "x", "y"},
		},
		{
			SourceTable: "hidden_coins",
			TargetTable: "phaser_hidden_coins",
			Columns:     []string{"id", "map_constant", "map_id", "x", "y"},
		},
		{
			SourceTable: "hidden_objects",
			TargetTable: "phaser_hidden_objects",
			Columns:     []string{"id", "map_constant", "map_id", "x", "y", "item_or_direction", "routine", "object_type"},
		},
		{
			SourceTable: "map_music",
			TargetTable: "phaser_map_music",
			Columns:     []string{"id", "map_constant", "map_id", "music_constant"},
		},
		{
			SourceTable: "map_scripts",
			TargetTable: "phaser_map_scripts",
			Columns:     []string{"id", "map_name", "script_index", "script_label", "script_constant", "raw_asm"},
		},
		{
			SourceTable: "npc_movement_data",
			TargetTable: "phaser_npc_movement_data",
			Columns:     []string{"id", "map_name", "label", "movements"},
		},
		{
			SourceTable: "event_flags",
			TargetTable: "phaser_event_flags",
			Columns:     []string{"id", "map_name", "flag_name", "operation", "context_label"},
		},
		{
			SourceTable: "warp_events",
			TargetTable: "phaser_warp_events",
			Columns:     []string{"id", "map_name", "map_id", "x", "y", "dest_map", "dest_warp_index"},
		},
	}
	for _, spec := range specs {
		if _, err := importStaticTable(sqlite, pg, spec); err != nil {
			return err
		}
	}
	if err := validatePokemonDefaultMovesPostgres(pg); err != nil {
		return err
	}
	repairedPokemon, err := pokebattle.RepairPokemonWithNoMoves(pg)
	if err != nil {
		return fmt.Errorf("repair persisted pokemon moves: %w", err)
	}
	if repairedPokemon > 0 {
		log.Printf("Repaired moves for %d persisted Pokemon", repairedPokemon)
	}
	if err := refreshImportedMapIDsPostgres(pg, "phaser_warp_events"); err != nil {
		return err
	}
	if err := resolveImportedWarpDestinationsPostgres(sqlite, pg); err != nil {
		return err
	}
	if err := deriveEncounterAreasPostgres(sqlite, pg, context.ReleaseCode); err != nil {
		return err
	}
	if err := bakeOverworldCoordinatesPostgres(pg); err != nil {
		return err
	}
	if err := classifyWarpActivationsPostgres(sqlite, pg); err != nil {
		return err
	}
	if err := markOrphanedBlockedCarpetWarpsPostgres(pg); err != nil {
		return err
	}
	if err := seedDungeonHoleWarpsPostgres(sqlite, pg); err != nil {
		return err
	}

	if err := importTrainerHeadersPostgres(sqlite, pg); err != nil {
		return err
	}
	populateGrowthRatesPostgres(pg)
	applyGameCornerHiddenCoinAmountsPostgres(pg)
	if err := resetPostgresIdentitySequences(pg); err != nil {
		return err
	}
	if err := importMissableObjectsPostgres(sqlite, pg); err != nil {
		return err
	}
	if err := seedCaptureQuestRuntimeDataPostgres(pg, sqlite); err != nil {
		return err
	}
	if err := markDynamicElevatorWarpPlaceholdersPostgres(pg); err != nil {
		return err
	}
	if err := markUnresolvedLastMapWarpPlaceholdersPostgres(pg); err != nil {
		return err
	}
	if err := importCoordinateTriggersPostgres(sqlite, pg); err != nil {
		return err
	}
	if err := importSpinTilesPostgres(sqlite, pg); err != nil {
		return err
	}
	if err := refreshImportedMapIDsPostgres(pg, "phaser_map_scripts"); err != nil {
		return err
	}
	if err := resetPostgresIdentitySequences(pg); err != nil {
		return err
	}

	return nil
}

func resolveImportedWarpDestinationsPostgres(sqlite, pg *sql.DB) error {
	if err := clearWarpDestinationCoordinatePlaceholdersPostgres(pg); err != nil {
		return err
	}
	// LAST_MAP is dynamic only when an interior can be entered from more than
	// one map. Resolve ordinary building exits (including Red's house) from their
	// unique external origin before calculating destination coordinates.
	if err := resolveLastMapWarpDestinationsPostgres(pg); err != nil {
		return err
	}
	if err := resolveWarpDestinationCoordinatesPostgres(sqlite, pg); err != nil {
		return err
	}
	return validateDeterministicLastMapWarpDestinationsPostgres(pg)
}

func validatePokemonDefaultMovesPostgres(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT DISTINCT defaults.move_constant
		FROM (
			SELECT default_move_1_id AS move_constant FROM phaser_pokemon
			UNION ALL SELECT default_move_2_id FROM phaser_pokemon
			UNION ALL SELECT default_move_3_id FROM phaser_pokemon
			UNION ALL SELECT default_move_4_id FROM phaser_pokemon
		) defaults
		LEFT JOIN phaser_moves moves
		  ON moves.constant_name = defaults.move_constant
		WHERE defaults.move_constant IS NOT NULL
		  AND defaults.move_constant <> 'NO_MOVE'
		  AND moves.id IS NULL
		ORDER BY defaults.move_constant`)
	if err != nil {
		return fmt.Errorf("validate Pokemon default moves: %w", err)
	}
	defer rows.Close()
	var missing []string
	for rows.Next() {
		var constant string
		if err := rows.Scan(&constant); err != nil {
			return fmt.Errorf("scan unresolved Pokemon default move: %w", err)
		}
		missing = append(missing, constant)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read unresolved Pokemon default moves: %w", err)
	}
	if len(missing) != 0 {
		return fmt.Errorf(
			"Pokemon defaults reference missing move constants: %s",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

func persistExtractorImportContext(pg *sql.DB, context extractorImportContext) error {
	_, err := pg.Exec(`
		INSERT INTO phaser_import_metadata (
			singleton, schema_name, schema_version, minimum_reader_version,
			extraction_run_id, extractor_revision, source_revision,
			source_date_epoch, source_tree_sha256, release_code
		) VALUES (true, $1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (singleton) DO UPDATE SET
			schema_name = EXCLUDED.schema_name,
			schema_version = EXCLUDED.schema_version,
			minimum_reader_version = EXCLUDED.minimum_reader_version,
			extraction_run_id = EXCLUDED.extraction_run_id,
			extractor_revision = EXCLUDED.extractor_revision,
			source_revision = EXCLUDED.source_revision,
			source_date_epoch = EXCLUDED.source_date_epoch,
			source_tree_sha256 = EXCLUDED.source_tree_sha256,
			release_code = EXCLUDED.release_code`,
		context.SchemaName,
		context.SchemaVersion,
		context.MinimumReaderVersion,
		context.RunID,
		context.ExtractorRevision,
		context.SourceRevision,
		context.SourceDateEpoch,
		context.SourceTreeSHA256,
		context.ReleaseCode,
	)
	if err != nil {
		return fmt.Errorf("persist extractor import metadata: %w", err)
	}
	return nil
}

func ensurePostgresSchema(pg *sql.DB) error {
	path, err := findPostgresSchemaPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read postgres schema %s: %w", path, err)
	}
	for _, stmt := range splitSQLStatements(string(raw)) {
		if _, err := pg.Exec(stmt); err != nil {
			return fmt.Errorf("apply postgres schema statement %.80q: %w", stmt, err)
		}
	}
	log.Printf("Postgres schema ensured from %s", path)
	return nil
}

func ensurePhaserTileMutationColumnsPostgres(pg *sql.DB) error {
	statements := []string{
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS is_original_tile_location smallint NOT NULL DEFAULT 0`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS has_tile_edit smallint NOT NULL DEFAULT 0`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS is_tile_erased smallint NOT NULL DEFAULT 0`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_tile_image_id integer DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_collision_type integer DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_raw_foot_tile_id integer DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_talk_over_tile boolean DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_encounter_area_id integer DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_local_x integer DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_local_y integer DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS original_source_map_id integer DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS last_edited_by_char_id integer DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS last_edited_at timestamp DEFAULT NULL`,
		`ALTER TABLE phaser_tiles ADD COLUMN IF NOT EXISTS last_edit_source varchar(32) DEFAULT NULL`,
	}
	for _, stmt := range statements {
		if _, err := pg.Exec(stmt); err != nil {
			return fmt.Errorf("ensure phaser_tiles mutation column: %w", err)
		}
	}
	return nil
}

func findPostgresSchemaPath() (string, error) {
	candidates := []string{
		filepath.Join("schema", "postgres_runtime_schema.sql"),
		filepath.Join("server", "schema", "postgres_runtime_schema.sql"),
		filepath.Join("..", "server", "schema", "postgres_runtime_schema.sql"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("server/schema/postgres_runtime_schema.sql not found")
}

func splitSQLStatements(sqlText string) []string {
	parts := strings.Split(sqlText, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		statements = append(statements, stmt)
	}
	return statements
}

func truncateImportedPostgresTables(pg *sql.DB) error {
	tables := []string{
		"phaser_coordinate_triggers",
		"phaser_spin_tiles",
		"phaser_missable_objects",
		"phaser_warp_events",
		"phaser_event_flags",
		"phaser_npc_movement_data",
		"phaser_map_scripts",
		"phaser_map_music",
		"phaser_hidden_objects",
		"phaser_hidden_coins",
		"phaser_hidden_items",
		"phaser_trainer_party_pokemon",
		"phaser_trainer_parties",
		"phaser_trainer_classes",
		"phaser_encounter_slots",
		"phaser_encounter_area_slots",
		"phaser_encounter_areas",
		"phaser_wild_encounters",
		"phaser_pokemon_tmhm",
		"phaser_pokemon_learnset",
		"phaser_trainer_headers",
		"phaser_text_pointers",
		"phaser_dialogue_text",
		"phaser_moves",
		"phaser_pokemon",
		"phaser_items",
		"phaser_warps",
		"phaser_objects",
		"phaser_maps",
	}
	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY", strings.Join(tables, ", "))
	if _, err := pg.Exec(query); err != nil {
		return fmt.Errorf("truncate imported phaser tables: %w", err)
	}
	if _, err := pg.Exec(`DELETE FROM phaser_event_object_visibility WHERE label LIKE 'SourceMissableInitial:%'`); err != nil {
		return fmt.Errorf("clear source missable visibility rows: %w", err)
	}
	return nil
}

type staticTableSpec struct {
	SourceTable   string
	TargetTable   string
	Columns       []string
	SourceColumns []string
	Where         string
	Args          []any
	Optional      bool
	Transform     func([]any) error
}

func importStaticTable(sqlite, pg *sql.DB, spec staticTableSpec) (int, error) {
	log.Printf("Importing %s -> %s...", spec.SourceTable, spec.TargetTable)
	sourceColumns := spec.SourceColumns
	if len(sourceColumns) == 0 {
		sourceColumns = spec.Columns
	}
	if len(sourceColumns) != len(spec.Columns) {
		return 0, fmt.Errorf("source/target column count mismatch for %s", spec.SourceTable)
	}
	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(sourceColumns, ", "), spec.SourceTable)
	if spec.Where != "" {
		query += " WHERE " + spec.Where
	}
	rows, err := sqlite.Query(query, spec.Args...)
	if err != nil {
		if spec.Optional {
			log.Printf("  ! Optional source table %s not imported: %v", spec.SourceTable, err)
			return 0, nil
		}
		return 0, fmt.Errorf("query %s: %w", spec.SourceTable, err)
	}
	defer rows.Close()

	stmt, err := pg.Prepare(insertPostgresSQL(spec.TargetTable, spec.Columns, spec.Optional))
	if err != nil {
		return 0, fmt.Errorf("prepare insert %s: %w", spec.TargetTable, err)
	}
	defer stmt.Close()

	count := 0
	values := make([]any, len(spec.Columns))
	scanArgs := make([]any, len(spec.Columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	for rows.Next() {
		clear(values)
		if err := rows.Scan(scanArgs...); err != nil {
			return count, fmt.Errorf("scan %s row: %w", spec.SourceTable, err)
		}
		convertSQLiteValues(values)
		if spec.Transform != nil {
			if err := spec.Transform(values); err != nil {
				return count, fmt.Errorf("transform %s row %d: %w", spec.SourceTable, count+1, err)
			}
		}
		if _, err := stmt.Exec(values...); err != nil {
			return count, fmt.Errorf("insert %s row %d: %w", spec.TargetTable, count+1, err)
		}
		count++
		if count%10000 == 0 {
			log.Printf("  -> Processed %d %s rows...", count, spec.SourceTable)
		}
	}
	if err := rows.Err(); err != nil {
		return count, fmt.Errorf("read %s rows: %w", spec.SourceTable, err)
	}
	log.Printf("  -> Imported %d rows into %s", count, spec.TargetTable)
	return count, nil
}

// pokered stores trainer prize rates as bcd3 fixed-point literals: the final
// two digits are fractional storage precision, not Pokédollars. Runtime tables
// use actual Pokédollars per level, so 3500 becomes 35 at this boundary.
func normalizeTrainerClassImportValues(values []any) error {
	const baseMoneyColumn = 3
	if len(values) <= baseMoneyColumn {
		return fmt.Errorf("trainer class row has %d columns, want at least %d", len(values), baseMoneyColumn+1)
	}
	raw, ok := values[baseMoneyColumn].(int64)
	if !ok {
		return fmt.Errorf("base_money has type %T, want int64", values[baseMoneyColumn])
	}
	if raw < 0 || raw%100 != 0 {
		return fmt.Errorf("base_money %d is not a non-negative bcd3 money literal", raw)
	}
	values[baseMoneyColumn] = raw / 100
	return nil
}

func insertPostgresSQL(table string, columns []string, upsert bool) string {
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), strings.Join(placeholders, ", "))
	if !upsert {
		return query
	}
	assignments := make([]string, 0, len(columns)-1)
	for _, column := range columns {
		if column == "id" {
			continue
		}
		assignments = append(assignments, fmt.Sprintf("%s = EXCLUDED.%s", column, column))
	}
	if len(assignments) == 0 {
		return query + " ON CONFLICT (id) DO NOTHING"
	}
	return query + " ON CONFLICT (id) DO UPDATE SET " + strings.Join(assignments, ", ")
}

func convertSQLiteValues(values []any) {
	for i, value := range values {
		if data, ok := value.([]byte); ok {
			values[i] = string(data)
		}
	}
}

type tileImageRuntimeMetadata struct {
	RawFootTileID sql.NullInt64
	TalkOverTile  bool
}

func importTileImagesPostgres(sqlite, pg *sql.DB) (map[int64]tileImageRuntimeMetadata, error) {
	log.Println("Importing tile_images -> phaser_tile_images...")
	rows, err := sqlite.Query(`
		SELECT ti.id, ti.image_path, ti.tileset_id, ti.block_index, ti.position, bs.block_data
		FROM tile_images ti
		LEFT JOIN blocksets bs
		  ON bs.tileset_id = ti.tileset_id AND bs.block_index = ti.block_index
		ORDER BY ti.id`)
	if err != nil {
		return nil, fmt.Errorf("query tile_images: %w", err)
	}
	defer rows.Close()

	stmt, err := pg.Prepare(`
		INSERT INTO phaser_tile_images
			(id, image_path, tileset_id, block_index, position, raw_foot_tile_id, talk_over_tile)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			image_path = EXCLUDED.image_path,
			tileset_id = EXCLUDED.tileset_id,
			block_index = EXCLUDED.block_index,
			position = EXCLUDED.position,
			raw_foot_tile_id = EXCLUDED.raw_foot_tile_id,
			talk_over_tile = EXCLUDED.talk_over_tile`)
	if err != nil {
		return nil, fmt.Errorf("prepare phaser_tile_images insert: %w", err)
	}
	defer stmt.Close()

	metadataByTileImageID := make(map[int64]tileImageRuntimeMetadata)
	count := 0
	for rows.Next() {
		var (
			id, tilesetID, blockIndex, position int64
			imagePath                           string
			blockData                           []byte
			rawFootTileID                       sql.NullInt64
		)
		if err := rows.Scan(&id, &imagePath, &tilesetID, &blockIndex, &position, &blockData); err != nil {
			return nil, fmt.Errorf("scan tile_image row: %w", err)
		}
		if raw, ok := phaserdata.RawFootTileIDFromBlockData(blockData, int(position)); ok {
			rawFootTileID = sql.NullInt64{Int64: int64(raw), Valid: true}
		}
		talkOverTile := isTalkOverTile(tilesetID, rawFootTileID)
		metadataByTileImageID[id] = tileImageRuntimeMetadata{
			RawFootTileID: rawFootTileID,
			TalkOverTile:  talkOverTile,
		}
		if _, err := stmt.Exec(
			id,
			imagePath,
			tilesetID,
			blockIndex,
			position,
			nullInt64ToParam(rawFootTileID),
			talkOverTile,
		); err != nil {
			return nil, fmt.Errorf("insert tile_image %d: %w", id, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tile_image rows: %w", err)
	}
	log.Printf("  -> Imported/merged %d tile images", count)
	return metadataByTileImageID, nil
}

func nullInt64ToParam(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func importTilesPostgres(sqlite, pg *sql.DB, tileImageMetadata map[int64]tileImageRuntimeMetadata) error {
	log.Println("Importing tiles -> phaser_tiles...")
	tx, err := pg.Begin()
	if err != nil {
		return fmt.Errorf("begin phaser_tiles import: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DROP TABLE IF EXISTS phaser_tiles_import_stage`); err != nil {
		return fmt.Errorf("drop phaser_tiles import stage: %w", err)
	}
	if _, err := tx.Exec(`
		CREATE TEMP TABLE phaser_tiles_import_stage (
			x integer NOT NULL,
			y integer NOT NULL,
			tile_image_id integer NOT NULL,
			local_x integer DEFAULT NULL,
			local_y integer DEFAULT NULL,
			map_id integer DEFAULT NULL,
			source_map_id integer DEFAULT NULL,
			collision_type integer DEFAULT 0,
			raw_foot_tile_id integer DEFAULT NULL,
			talk_over_tile boolean NOT NULL DEFAULT false
		) ON COMMIT DROP`); err != nil {
		return fmt.Errorf("create phaser_tiles import stage: %w", err)
	}

	overworldMaps := make(map[int64]bool)
	rows, err := tx.Query(`SELECT id FROM phaser_maps WHERE is_overworld = 1`)
	if err != nil {
		return fmt.Errorf("query overworld maps: %w", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		overworldMaps[id] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	log.Printf("  -> Found %d overworld maps", len(overworldMaps))

	mapTilesetIDs, err := loadSQLiteMapTilesetIDs(sqlite)
	if err != nil {
		return err
	}
	mapBlocks, err := loadSQLiteMapBlockMetadata(sqlite)
	if err != nil {
		return err
	}
	blocksets, err := loadSQLiteBlocksets(sqlite)
	if err != nil {
		return err
	}

	sourceRows, err := sqlite.Query(`SELECT id, x, y, tile_image_id, local_x, local_y, map_id, collision_type FROM tiles`)
	if err != nil {
		return fmt.Errorf("query tiles: %w", err)
	}
	defer sourceRows.Close()

	stmt, err := tx.Prepare(`
		INSERT INTO phaser_tiles_import_stage
			(x, y, tile_image_id, local_x, local_y, map_id, source_map_id, collision_type, raw_foot_tile_id, talk_over_tile)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
	if err != nil {
		return fmt.Errorf("prepare phaser_tiles stage insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for sourceRows.Next() {
		var id, x, y, tileImageID, mapID int64
		var localX, localY, collisionType sql.NullInt64
		if err := sourceRows.Scan(&id, &x, &y, &tileImageID, &localX, &localY, &mapID, &collisionType); err != nil {
			return fmt.Errorf("scan tile row: %w", err)
		}
		var mapIDParam any = mapID
		if overworldMaps[mapID] {
			mapIDParam = nil
		}
		metadata := tileImageMetadata[tileImageID]
		rawFootTileID := metadata.RawFootTileID
		talkOverRawFootTileID := rawFootTileID
		if raw, ok := rawFootTileIDForPlacedTile(
			mapBlocks[mapID],
			blocksets,
			tileCoordinateForMetadata(x, localX),
			tileCoordinateForMetadata(y, localY),
		); ok {
			talkOverRawFootTileID = sql.NullInt64{Int64: int64(raw), Valid: true}
		}
		talkOverTile := isTalkOverTile(mapTilesetIDs[mapID], talkOverRawFootTileID)
		if _, err := stmt.Exec(
			x,
			y,
			tileImageID,
			nullToPtr(localX),
			nullToPtr(localY),
			mapIDParam,
			mapID,
			nullToPtr(collisionType),
			nullInt64ToParam(rawFootTileID),
			talkOverTile,
		); err != nil {
			return fmt.Errorf("stage tile %d: %w", id, err)
		}
		count++
		if count%10000 == 0 {
			log.Printf("  -> Processed %d tiles...", count)
		}
	}
	if err := sourceRows.Err(); err != nil {
		return fmt.Errorf("read tile rows: %w", err)
	}
	if err := mergeImportedTilesPostgres(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit phaser_tiles import: %w", err)
	}
	log.Printf("  -> Imported/merged %d tiles", count)
	return nil
}

func mergeImportedTilesPostgres(tx *sql.Tx) error {
	result, err := tx.Exec(`
		UPDATE phaser_tiles
		SET
			is_original_tile_location = 1,
			is_native_game_data = TRUE,
			coordinate_origin = 'native',
			content_origin = CASE WHEN has_tile_edit = 1 THEN 'user' ELSE 'native' END,
			original_tile_image_id = (
				SELECT s.tile_image_id FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			),
			original_collision_type = (
				SELECT s.collision_type FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			),
			original_raw_foot_tile_id = (
				SELECT s.raw_foot_tile_id FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			),
			original_talk_over_tile = (
				SELECT s.talk_over_tile FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			),
			original_encounter_area_id = NULL,
			original_local_x = (
				SELECT s.local_x FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			),
			original_local_y = (
				SELECT s.local_y FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			),
			original_source_map_id = (
				SELECT s.source_map_id FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			),
			tile_image_id = CASE WHEN has_tile_edit = 1 THEN tile_image_id ELSE (
				SELECT s.tile_image_id FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			) END,
			collision_type = CASE WHEN has_tile_edit = 1 THEN collision_type ELSE (
				SELECT s.collision_type FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			) END,
			raw_foot_tile_id = CASE WHEN has_tile_edit = 1 THEN raw_foot_tile_id ELSE (
				SELECT s.raw_foot_tile_id FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			) END,
			talk_over_tile = CASE WHEN has_tile_edit = 1 THEN talk_over_tile ELSE (
				SELECT s.talk_over_tile FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			) END,
			encounter_area_id = CASE WHEN has_tile_edit = 1 THEN encounter_area_id ELSE NULL END,
			local_x = CASE WHEN has_tile_edit = 1 THEN local_x ELSE (
				SELECT s.local_x FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			) END,
			local_y = CASE WHEN has_tile_edit = 1 THEN local_y ELSE (
				SELECT s.local_y FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			) END,
			source_map_id = CASE WHEN has_tile_edit = 1 THEN source_map_id ELSE (
				SELECT s.source_map_id FROM phaser_tiles_import_stage AS s
				WHERE s.x = phaser_tiles.x AND s.y = phaser_tiles.y AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
			) END,
			is_tile_erased = CASE WHEN has_tile_edit = 1 THEN is_tile_erased ELSE 0 END,
			is_user_placed = CASE WHEN has_tile_edit = 1 THEN is_user_placed ELSE 0 END,
			placed_by_char_id = CASE WHEN has_tile_edit = 1 THEN placed_by_char_id ELSE NULL END,
			placed_at = CASE WHEN has_tile_edit = 1 THEN placed_at ELSE NULL END,
			last_edited_by_char_id = CASE WHEN has_tile_edit = 1 THEN last_edited_by_char_id ELSE NULL END,
			last_edited_at = CASE WHEN has_tile_edit = 1 THEN last_edited_at ELSE NULL END,
			last_edit_source = CASE WHEN has_tile_edit = 1 THEN last_edit_source ELSE NULL END
		WHERE EXISTS (
			SELECT 1
			FROM phaser_tiles_import_stage AS s
			WHERE s.x = phaser_tiles.x
			  AND s.y = phaser_tiles.y
			  AND COALESCE(s.map_id, -1) = COALESCE(phaser_tiles.map_id, -1)
		)`)
	if err != nil {
		return fmt.Errorf("update existing phaser_tiles from import stage: %w", err)
	}
	updated, _ := result.RowsAffected()

	result, err = tx.Exec(`
		INSERT INTO phaser_tiles (
			x, y, tile_image_id, local_x, local_y, map_id, source_map_id,
			collision_type, raw_foot_tile_id, talk_over_tile, encounter_area_id,
			is_native_game_data, coordinate_origin, content_origin,
			is_original_tile_location, has_tile_edit, is_tile_erased,
			original_tile_image_id, original_collision_type, original_raw_foot_tile_id,
			original_talk_over_tile, original_encounter_area_id, original_local_x,
			original_local_y, original_source_map_id,
			is_user_placed, placed_by_char_id, placed_at,
			last_edited_by_char_id, last_edited_at, last_edit_source
		)
		SELECT
			x, y, tile_image_id, local_x, local_y, map_id, source_map_id,
			collision_type, raw_foot_tile_id, talk_over_tile, NULL,
			TRUE, 'native', 'native',
			1, 0, 0,
			tile_image_id, collision_type, raw_foot_tile_id,
			talk_over_tile, NULL, local_x, local_y, source_map_id,
			0, NULL, NULL,
			NULL, NULL, NULL
		FROM phaser_tiles_import_stage
		WHERE NOT EXISTS (
			SELECT 1
			FROM phaser_tiles AS t
			WHERE t.x = phaser_tiles_import_stage.x
			  AND t.y = phaser_tiles_import_stage.y
			  AND COALESCE(t.map_id, -1) = COALESCE(phaser_tiles_import_stage.map_id, -1)
		)`)
	if err != nil {
		return fmt.Errorf("insert new phaser_tiles from import stage: %w", err)
	}
	inserted, _ := result.RowsAffected()

	result, err = tx.Exec(`
		DELETE FROM phaser_tiles AS t
		WHERE t.is_original_tile_location = 1
		  AND t.has_tile_edit = 0
		  AND NOT EXISTS (
			SELECT 1
			FROM phaser_tiles_import_stage AS s
			WHERE s.x = t.x
			  AND s.y = t.y
			  AND COALESCE(s.map_id, -1) = COALESCE(t.map_id, -1)
		  )`)
	if err != nil {
		return fmt.Errorf("delete stale unedited phaser_tiles: %w", err)
	}
	deleted, _ := result.RowsAffected()
	log.Printf("  -> Merged %d updated and %d inserted tile rows, deleted %d stale unedited base rows", updated, inserted, deleted)
	return nil
}

type sqliteMapBlockMetadata struct {
	Width     int64
	Height    int64
	TilesetID int64
	BlkData   []byte
}

func loadSQLiteMapBlockMetadata(sqlite *sql.DB) (map[int64]sqliteMapBlockMetadata, error) {
	rows, err := sqlite.Query(`SELECT id, width, height, tileset_id, blk_data FROM maps`)
	if err != nil {
		return nil, fmt.Errorf("query map block metadata: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]sqliteMapBlockMetadata)
	for rows.Next() {
		var (
			id        int64
			width     sql.NullInt64
			height    sql.NullInt64
			tilesetID sql.NullInt64
			blkData   []byte
		)
		if err := rows.Scan(&id, &width, &height, &tilesetID, &blkData); err != nil {
			return nil, fmt.Errorf("scan map block metadata: %w", err)
		}
		if width.Valid && height.Valid && tilesetID.Valid && len(blkData) > 0 {
			result[id] = sqliteMapBlockMetadata{
				Width:     width.Int64,
				Height:    height.Int64,
				TilesetID: tilesetID.Int64,
				BlkData:   blkData,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read map block metadata: %w", err)
	}
	return result, nil
}

func loadSQLiteBlocksets(sqlite *sql.DB) (map[int64]map[int64][]byte, error) {
	rows, err := sqlite.Query(`SELECT tileset_id, block_index, block_data FROM blocksets`)
	if err != nil {
		return nil, fmt.Errorf("query blocksets: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]map[int64][]byte)
	for rows.Next() {
		var tilesetID, blockIndex int64
		var blockData []byte
		if err := rows.Scan(&tilesetID, &blockIndex, &blockData); err != nil {
			return nil, fmt.Errorf("scan blockset: %w", err)
		}
		if result[tilesetID] == nil {
			result[tilesetID] = make(map[int64][]byte)
		}
		result[tilesetID][blockIndex] = blockData
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read blocksets: %w", err)
	}
	return result, nil
}

func tileCoordinateForMetadata(global int64, local sql.NullInt64) int64 {
	if local.Valid {
		return local.Int64
	}
	return global
}

func rawFootTileIDForPlacedTile(
	mapMeta sqliteMapBlockMetadata,
	blocksets map[int64]map[int64][]byte,
	localX int64,
	localY int64,
) (int, bool) {
	if mapMeta.Width <= 0 || mapMeta.Height <= 0 || len(mapMeta.BlkData) == 0 {
		return 0, false
	}
	if localX < 0 || localY < 0 {
		return 0, false
	}

	blockX := localX / 2
	blockY := localY / 2
	if blockX >= mapMeta.Width || blockY >= mapMeta.Height {
		return 0, false
	}

	blockOffset := blockY*mapMeta.Width + blockX
	if blockOffset < 0 || blockOffset >= int64(len(mapMeta.BlkData)) {
		return 0, false
	}

	blockIndex := int64(mapMeta.BlkData[blockOffset])
	blockData := blocksets[blocksetTilesetID(mapMeta.TilesetID)][blockIndex]
	if len(blockData) == 0 {
		return 0, false
	}

	position := int((localY%2)*2 + (localX % 2))
	return phaserdata.RawFootTileIDFromBlockData(blockData, position)
}

func blocksetTilesetID(tilesetID int64) int64 {
	switch tilesetID {
	case 2:
		// MART uses POKECENTER graphics/blocksets in the extracted SQLite DB.
		return 6
	case 5:
		// DOJO uses GYM graphics/blocksets in the extracted SQLite DB.
		return 7
	default:
		return tilesetID
	}
}

func loadSQLiteMapTilesetIDs(sqlite *sql.DB) (map[int64]int64, error) {
	rows, err := sqlite.Query(`SELECT id, tileset_id FROM maps`)
	if err != nil {
		return nil, fmt.Errorf("query map tilesets: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]int64)
	for rows.Next() {
		var id int64
		var tilesetID sql.NullInt64
		if err := rows.Scan(&id, &tilesetID); err != nil {
			return nil, fmt.Errorf("scan map tileset: %w", err)
		}
		if tilesetID.Valid {
			result[id] = tilesetID.Int64
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read map tilesets: %w", err)
	}
	return result, nil
}

type overworldEncounterMap struct {
	Name      string
	Width     int
	Height    int
	AreaID    int
	Offset    coordinateOffset
	HasOffset bool
}

func deriveEncounterAreasPostgres(sqlite, pg *sql.DB, release string) error {
	log.Println("Deriving grass encounter areas from wild encounter tables...")
	offsets := loadOverworldMapOffsets(sqlite)
	tx, err := pg.Begin()
	if err != nil {
		return fmt.Errorf("begin encounter area derivation: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`UPDATE phaser_tiles SET original_encounter_area_id = NULL WHERE is_original_tile_location = 1`); err != nil {
		return fmt.Errorf("clear original tile encounter area ids: %w", err)
	}
	if _, err := tx.Exec(`UPDATE phaser_tiles SET encounter_area_id = NULL WHERE has_tile_edit = 0`); err != nil {
		return fmt.Errorf("clear tile encounter area ids: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM phaser_encounter_area_slots`); err != nil {
		return fmt.Errorf("clear encounter area slots: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM phaser_encounter_areas`); err != nil {
		return fmt.Errorf("clear encounter areas: %w", err)
	}

	areaResult, err := tx.Exec(`
		INSERT INTO phaser_encounter_areas (name, encounter_rate, encounter_type)
		SELECT
			we.map_name || '_' || UPPER(we.encounter_type) AS name,
			we.encounter_rate,
			we.encounter_type
		FROM phaser_wild_encounters we
		WHERE we.map_id IS NOT NULL
		  AND we.encounter_type = 'grass'
		  AND we.encounter_rate > 0
		GROUP BY we.map_name, we.encounter_type, we.encounter_rate
		ORDER BY we.map_name, we.encounter_type`)
	if err != nil {
		return fmt.Errorf("derive encounter areas: %w", err)
	}
	areasCreated, _ := areaResult.RowsAffected()

	slotResult, err := tx.Exec(`
		WITH selected_slots AS (
			SELECT
				we.map_name,
				we.encounter_type,
				we.pokemon_name,
				we.level,
				ROW_NUMBER() OVER (
					PARTITION BY we.map_name, we.encounter_type
					ORDER BY we.slot_index
				) AS normalized_slot_index
			FROM phaser_wild_encounters we
			WHERE we.map_id IS NOT NULL
			  AND we.encounter_type = 'grass'
			  AND we.encounter_rate > 0
			  AND we.version IN ($1, 'both')
		)
		INSERT INTO phaser_encounter_area_slots (
			encounter_area_id, slot_index, pokemon_id, level, probability
		)
		SELECT
			ea.id,
			rs.normalized_slot_index,
			pp.id,
			rs.level,
			es.probability
		FROM selected_slots rs
		JOIN phaser_encounter_areas ea
		  ON ea.name = rs.map_name || '_' || UPPER(rs.encounter_type)
		JOIN phaser_pokemon pp ON pp.name = rs.pokemon_name
		JOIN phaser_encounter_slots es ON es.slot_index = rs.normalized_slot_index
		WHERE rs.normalized_slot_index <= 10
		ORDER BY ea.id, rs.normalized_slot_index`, release)
	if err != nil {
		return fmt.Errorf("derive encounter area slots: %w", err)
	}
	slotsCreated, _ := slotResult.RowsAffected()

	taggedNonOverworld, err := tagNonOverworldEncounterTilesPostgres(tx)
	if err != nil {
		return err
	}
	overworldMaps, err := loadOverworldEncounterMapsPostgres(tx, offsets)
	if err != nil {
		return err
	}
	taggedOverworld, err := tagOverworldEncounterTilesPostgres(tx, overworldMaps)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit encounter area derivation: %w", err)
	}
	log.Printf("  -> Derived %d encounter areas, %d slots, tagged %d map tiles and %d overworld tiles",
		areasCreated, slotsCreated, taggedNonOverworld, taggedOverworld)
	return nil
}

func tagNonOverworldEncounterTilesPostgres(tx *sql.Tx) (int64, error) {
	result, err := tx.Exec(`
		UPDATE phaser_tiles AS t
		SET original_encounter_area_id = ea.id,
			encounter_area_id = CASE WHEN t.has_tile_edit = 0 THEN ea.id ELSE t.encounter_area_id END
		FROM phaser_maps AS pm
		JOIN phaser_encounter_areas AS ea
		  ON ea.name = pm.name || '_GRASS'
		WHERE t.map_id = pm.id
		  AND t.is_original_tile_location = 1
		  AND ea.encounter_type = 'grass'
		  AND COALESCE(t.original_collision_type, t.collision_type) = 1
		  AND (
		      COALESCE(t.original_tile_image_id, t.tile_image_id) IN (25, 142, 144, 149, 825)
		      OR (pm.id >= 37 AND COALESCE(pm.tileset_id, -1) <> 3)
		  )`)
	if err != nil {
		return 0, fmt.Errorf("tag non-overworld encounter tiles: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
}

func loadOverworldEncounterMapsPostgres(tx *sql.Tx, offsets map[string]coordinateOffset) ([]overworldEncounterMap, error) {
	rows, err := tx.Query(`
		SELECT pm.name, pm.width, pm.height, ea.id
		FROM phaser_maps AS pm
		JOIN phaser_encounter_areas AS ea ON ea.name = pm.name || '_GRASS'
		WHERE pm.is_overworld = 1
		  AND ea.encounter_type = 'grass'
		ORDER BY pm.id`)
	if err != nil {
		return nil, fmt.Errorf("load overworld encounter maps: %w", err)
	}
	defer rows.Close()

	var maps []overworldEncounterMap
	for rows.Next() {
		var m overworldEncounterMap
		if err := rows.Scan(&m.Name, &m.Width, &m.Height, &m.AreaID); err != nil {
			return nil, fmt.Errorf("scan overworld encounter map: %w", err)
		}
		m.Offset, m.HasOffset = offsets[normalizeMapName(m.Name)]
		if !m.HasOffset {
			log.Printf("  ! No overworld offset found for encounter map %s", m.Name)
		}
		maps = append(maps, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read overworld encounter maps: %w", err)
	}
	return maps, nil
}

func tagOverworldEncounterTilesPostgres(tx *sql.Tx, maps []overworldEncounterMap) (int64, error) {
	stmt, err := tx.Prepare(`
		UPDATE phaser_tiles
		SET original_encounter_area_id = $1,
			encounter_area_id = CASE WHEN has_tile_edit = 0 THEN $1 ELSE encounter_area_id END
		WHERE map_id IS NULL
		  AND is_original_tile_location = 1
		  AND x >= $2 AND x < $3
		  AND y >= $4 AND y < $5
		  AND COALESCE(original_collision_type, collision_type) = 1
		  AND COALESCE(original_tile_image_id, tile_image_id) IN (25, 142, 144, 149, 825)`)
	if err != nil {
		return 0, fmt.Errorf("prepare overworld encounter tile tagging: %w", err)
	}
	defer stmt.Close()

	var total int64
	for _, m := range maps {
		if !m.HasOffset {
			continue
		}
		result, err := stmt.Exec(
			m.AreaID,
			m.Offset.X, m.Offset.X+m.Width,
			m.Offset.Y, m.Offset.Y+m.Height,
		)
		if err != nil {
			return total, fmt.Errorf("tag overworld encounter tiles for %s: %w", m.Name, err)
		}
		affected, _ := result.RowsAffected()
		total += affected
	}
	return total, nil
}

type importedMapInfo struct {
	Name        string
	IsOverworld bool
}

type importedWarpEvent struct {
	MapID         int
	MapName       string
	X             int
	Y             int
	DestMap       string
	DestWarpIndex int
}

type importedRuntimeWarp struct {
	ID               int
	SourceMapID      int
	X                int
	Y                int
	DestinationMapID int
	HasDestination   bool
}

type warpDestinationUpdate struct {
	WarpID int
	X      int
	Y      int
}

func resolveWarpDestinationCoordinatesPostgres(sqlite, pg *sql.DB) error {
	maps, err := loadImportedMapInfoPostgres(pg)
	if err != nil {
		return err
	}
	events, err := loadImportedWarpEventsPostgres(pg)
	if err != nil {
		return err
	}
	warps, err := loadImportedRuntimeWarpsPostgres(pg)
	if err != nil {
		return err
	}

	updates := resolveWarpDestinationUpdates(maps, loadOverworldMapOffsets(sqlite), events, warps)
	if len(updates) == 0 {
		log.Printf("  -> Resolved 0 warp destination coordinates")
		return nil
	}

	stmt, err := pg.Prepare(`UPDATE phaser_warps SET destination_x = $1, destination_y = $2 WHERE id = $3`)
	if err != nil {
		return fmt.Errorf("prepare warp destination coordinate update: %w", err)
	}
	defer stmt.Close()

	for _, update := range updates {
		if _, err := stmt.Exec(update.X, update.Y, update.WarpID); err != nil {
			return fmt.Errorf("update warp %d destination coordinates: %w", update.WarpID, err)
		}
	}
	log.Printf("  -> Resolved %d warp destination coordinates from warp events", len(updates))
	return nil
}

func loadImportedMapInfoPostgres(pg *sql.DB) (map[int]importedMapInfo, error) {
	rows, err := pg.Query(`SELECT id, name, is_overworld FROM phaser_maps`)
	if err != nil {
		return nil, fmt.Errorf("load imported map info: %w", err)
	}
	defer rows.Close()

	maps := make(map[int]importedMapInfo)
	for rows.Next() {
		var id int
		var info importedMapInfo
		var isOverworld int
		if err := rows.Scan(&id, &info.Name, &isOverworld); err != nil {
			return nil, fmt.Errorf("scan imported map info: %w", err)
		}
		info.IsOverworld = isOverworld != 0
		maps[id] = info
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read imported map info: %w", err)
	}
	return maps, nil
}

func loadImportedWarpEventsPostgres(pg *sql.DB) ([]importedWarpEvent, error) {
	rows, err := pg.Query(`
		SELECT map_id, map_name, x, y, dest_map, dest_warp_index
		FROM phaser_warp_events
		WHERE map_id IS NOT NULL
		ORDER BY map_id, id`)
	if err != nil {
		return nil, fmt.Errorf("load imported warp events: %w", err)
	}
	defer rows.Close()

	var events []importedWarpEvent
	for rows.Next() {
		var event importedWarpEvent
		if err := rows.Scan(&event.MapID, &event.MapName, &event.X, &event.Y, &event.DestMap, &event.DestWarpIndex); err != nil {
			return nil, fmt.Errorf("scan imported warp event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read imported warp events: %w", err)
	}
	return events, nil
}

func loadImportedRuntimeWarpsPostgres(pg *sql.DB) ([]importedRuntimeWarp, error) {
	rows, err := pg.Query(`
		SELECT id, source_map_id, x, y, destination_map_id
		FROM phaser_warps
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load imported runtime warps: %w", err)
	}
	defer rows.Close()

	var warps []importedRuntimeWarp
	for rows.Next() {
		var warp importedRuntimeWarp
		var destinationMapID sql.NullInt64
		if err := rows.Scan(&warp.ID, &warp.SourceMapID, &warp.X, &warp.Y, &destinationMapID); err != nil {
			return nil, fmt.Errorf("scan imported runtime warp: %w", err)
		}
		if destinationMapID.Valid {
			warp.HasDestination = true
			warp.DestinationMapID = int(destinationMapID.Int64)
		}
		warps = append(warps, warp)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read imported runtime warps: %w", err)
	}
	return warps, nil
}

const lastMapOriginCTE = `
	WITH last_map_source_maps AS (
			SELECT DISTINCT
				pw.source_map_id,
				source_map.name AS source_map_name
			FROM phaser_warps AS pw
			JOIN phaser_maps AS source_map
			  ON source_map.id = pw.source_map_id
			WHERE pw.destination_kind = 'last-map'
		),
		candidate_origins AS (
			SELECT
				last_map_source_maps.source_map_id,
				incoming.map_id AS destination_map_id
			FROM last_map_source_maps
			JOIN phaser_warp_events AS incoming
			  ON incoming.map_id IS NOT NULL
			 AND LOWER(REPLACE(incoming.dest_map, '_', '')) = LOWER(REPLACE(last_map_source_maps.source_map_name, '_', ''))
			JOIN phaser_warps AS target_entry
			  ON target_entry.source_map_id = last_map_source_maps.source_map_id
			 AND target_entry.source_warp_index = incoming.dest_warp_index
			 AND target_entry.destination_kind = 'last-map'
		),
		unique_origins AS (
			SELECT
				source_map_id,
				MIN(destination_map_id) AS destination_map_id
			FROM candidate_origins
			GROUP BY source_map_id
			HAVING COUNT(DISTINCT destination_map_id) = 1
		)`

func resolveLastMapWarpDestinationsPostgres(pg *sql.DB) error {
	result, err := pg.Exec(lastMapOriginCTE + `,
		resolved AS (
			SELECT
				unique_origins.source_map_id,
				unique_origins.destination_map_id,
				pm.name AS destination_map
			FROM unique_origins
			JOIN phaser_maps AS pm ON pm.id = unique_origins.destination_map_id
		)
		UPDATE phaser_warps AS pw
		SET destination_map_id = resolved.destination_map_id,
			destination_map = resolved.destination_map,
			destination_x = NULL,
			destination_y = NULL
		FROM resolved
		WHERE pw.source_map_id = resolved.source_map_id
		  AND pw.destination_kind = 'last-map'
		  AND pw.destination_map_id IS NULL
		  AND UPPER(COALESCE(pw.destination_map, '')) = 'LAST_MAP'`)
	if err != nil {
		return fmt.Errorf("resolve LAST_MAP warp destinations: %w", err)
	}
	affected, _ := result.RowsAffected()
	log.Printf("  -> Resolved %d deterministic LAST_MAP warp destinations", affected)
	return nil
}

func validateDeterministicLastMapWarpDestinationsPostgres(pg *sql.DB) error {
	var invalid int
	err := pg.QueryRow(lastMapOriginCTE + `
		SELECT COUNT(*)
		FROM phaser_warps AS pw
		JOIN unique_origins
		  ON unique_origins.source_map_id = pw.source_map_id
		WHERE pw.destination_kind = 'last-map'
		  AND (
			pw.destination_map_id IS NULL
			OR pw.destination_map_id <> unique_origins.destination_map_id
			OR pw.destination_x IS NULL
			OR pw.destination_y IS NULL
		  )`).Scan(&invalid)
	if err != nil {
		return fmt.Errorf("validate deterministic LAST_MAP warp destinations: %w", err)
	}
	if invalid != 0 {
		return fmt.Errorf("%d deterministic LAST_MAP warp destinations remain unresolved or incorrect", invalid)
	}
	return nil
}

func clearWarpDestinationCoordinatePlaceholdersPostgres(pg *sql.DB) error {
	result, err := pg.Exec(`
		UPDATE phaser_warps
		SET destination_x = NULL,
			destination_y = NULL
		WHERE destination_x = 0
		  AND destination_y = 0`)
	if err != nil {
		return fmt.Errorf("clear placeholder warp destination coordinates: %w", err)
	}
	affected, _ := result.RowsAffected()
	log.Printf("  -> Cleared %d placeholder warp destination coordinates", affected)
	return nil
}

func resolveWarpDestinationUpdates(
	maps map[int]importedMapInfo,
	offsets map[string]coordinateOffset,
	events []importedWarpEvent,
	warps []importedRuntimeWarp,
) []warpDestinationUpdate {
	eventsByMapIndex := make(map[int]map[int]importedWarpEvent)
	sourceEvents := make(map[string]importedWarpEvent)

	for _, event := range events {
		index := len(eventsByMapIndex[event.MapID]) + 1
		if eventsByMapIndex[event.MapID] == nil {
			eventsByMapIndex[event.MapID] = make(map[int]importedWarpEvent)
			index = 1
		}
		eventsByMapIndex[event.MapID][index] = event

		sourceEvents[warpCoordKey(event.MapID, event.X, event.Y)] = event
		globalX, globalY := globalWarpEventCoordinates(maps, offsets, event.MapID, event.MapName, event.X, event.Y)
		sourceEvents[warpCoordKey(event.MapID, globalX, globalY)] = event
	}

	updates := make([]warpDestinationUpdate, 0, len(warps))
	for _, warp := range warps {
		if !warp.HasDestination {
			continue
		}
		sourceEvent, ok := sourceEvents[warpCoordKey(warp.SourceMapID, warp.X, warp.Y)]
		if !ok || sourceEvent.DestWarpIndex <= 0 {
			continue
		}
		destinationEvent, ok := eventsByMapIndex[warp.DestinationMapID][sourceEvent.DestWarpIndex]
		if !ok {
			continue
		}
		x, y := globalWarpEventCoordinates(
			maps,
			offsets,
			warp.DestinationMapID,
			destinationEvent.MapName,
			destinationEvent.X,
			destinationEvent.Y,
		)
		updates = append(updates, warpDestinationUpdate{WarpID: warp.ID, X: x, Y: y})
	}
	return updates
}

func globalWarpEventCoordinates(
	maps map[int]importedMapInfo,
	offsets map[string]coordinateOffset,
	mapID int,
	eventMapName string,
	x int,
	y int,
) (int, int) {
	info, ok := maps[mapID]
	if !ok || !info.IsOverworld {
		return x, y
	}
	mapName := eventMapName
	if mapName == "" {
		mapName = info.Name
	}
	if offset, ok := offsets[normalizeMapName(mapName)]; ok {
		return x + offset.X, y + offset.Y
	}
	return x, y
}

func warpCoordKey(mapID, x, y int) string {
	return fmt.Sprintf("%d:%d,%d", mapID, x, y)
}

func bakeOverworldCoordinatesPostgres(pg *sql.DB) error {
	result, err := pg.Exec(`
		UPDATE phaser_objects AS po
		SET x = po.local_x + offsets.offset_x,
			y = po.local_y + offsets.offset_y
		FROM phaser_maps AS pm,
			(
				SELECT COALESCE(original_source_map_id, source_map_id) AS map_id,
					MIN(x) - MIN(COALESCE(original_local_x, local_x)) AS offset_x,
					MIN(y) - MIN(COALESCE(original_local_y, local_y)) AS offset_y
				FROM phaser_tiles
				WHERE COALESCE(original_local_x, local_x) IS NOT NULL
				  AND COALESCE(original_local_y, local_y) IS NOT NULL
				  AND COALESCE(original_source_map_id, source_map_id) IS NOT NULL
				GROUP BY COALESCE(original_source_map_id, source_map_id)
			) AS offsets
		WHERE po.map_id = pm.id
		  AND offsets.map_id = po.map_id
		  AND pm.is_overworld = 1
		  AND po.local_x IS NOT NULL
		  AND po.local_y IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("bake overworld object coordinates: %w", err)
	}
	affected, _ := result.RowsAffected()
	log.Printf("  -> Updated %d overworld objects with global coordinates", affected)

	result, err = pg.Exec(`
		UPDATE phaser_warps AS pw
		SET x = source_event.x + offsets.offset_x,
			y = source_event.y + offsets.offset_y
		FROM phaser_maps AS pm,
			phaser_warp_events AS source_event,
			(
				SELECT COALESCE(original_source_map_id, source_map_id) AS map_id,
					MIN(x) - MIN(COALESCE(original_local_x, local_x)) AS offset_x,
					MIN(y) - MIN(COALESCE(original_local_y, local_y)) AS offset_y
				FROM phaser_tiles
				WHERE COALESCE(original_local_x, local_x) IS NOT NULL
				  AND COALESCE(original_local_y, local_y) IS NOT NULL
				  AND COALESCE(original_source_map_id, source_map_id) IS NOT NULL
				GROUP BY COALESCE(original_source_map_id, source_map_id)
			) AS offsets
		WHERE pw.source_map_id = pm.id
		  AND offsets.map_id = pw.source_map_id
		  AND source_event.map_id = pw.source_map_id
		  AND LOWER(REPLACE(source_event.dest_map, '_', '')) = LOWER(REPLACE(COALESCE(pw.destination_map, ''), '_', ''))
		  AND (
		  	(pw.x = source_event.x AND pw.y = source_event.y)
		  	OR (pw.x = source_event.x + offsets.offset_x AND pw.y = source_event.y + offsets.offset_y)
		  )
		  AND pm.is_overworld = 1`)
	if err != nil {
		return fmt.Errorf("bake overworld warp coordinates: %w", err)
	}
	affected, _ = result.RowsAffected()
	log.Printf("  -> Updated %d overworld warps with global coordinates", affected)
	return nil
}

func importTrainerHeadersPostgres(sqlite, pg *sql.DB) error {
	log.Println("Importing trainer_headers -> phaser_trainer_headers...")
	mapNameToID := buildPostgresMapNameLookup(pg)
	rows, err := sqlite.Query(`SELECT id, map_name, header_label, header_index, event_flag, sight_range, battle_text_label, end_battle_text_label, after_battle_text_label FROM trainer_headers`)
	if err != nil {
		return fmt.Errorf("query trainer_headers: %w", err)
	}
	defer rows.Close()

	stmt, err := pg.Prepare(`
		INSERT INTO phaser_trainer_headers
			(id, map_name, map_id, header_label, header_index, event_flag, sight_range, battle_text_label, end_battle_text_label, after_battle_text_label)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
	if err != nil {
		return fmt.Errorf("prepare trainer_headers insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	unmatched := 0
	for rows.Next() {
		var id, headerIndex int
		var mapName, headerLabel string
		var eventFlag, battleTextLabel, endBattleTextLabel, afterBattleTextLabel sql.NullString
		var sightRange sql.NullInt64
		if err := rows.Scan(&id, &mapName, &headerLabel, &headerIndex, &eventFlag, &sightRange, &battleTextLabel, &endBattleTextLabel, &afterBattleTextLabel); err != nil {
			return fmt.Errorf("scan trainer_header row: %w", err)
		}
		var mapID any
		if matched, ok := mapNameToID[normalizeMapName(mapName)]; ok {
			mapID = matched
		} else {
			unmatched++
			log.Printf("  ! No map_id match for trainer_header map_name=%q", mapName)
		}
		if _, err := stmt.Exec(
			id, mapName, mapID, headerLabel, headerIndex, nullStrToPtr(eventFlag), nullToPtr(sightRange),
			nullStrToPtr(battleTextLabel), nullStrToPtr(endBattleTextLabel), nullStrToPtr(afterBattleTextLabel),
		); err != nil {
			return fmt.Errorf("insert trainer_header %s: %w", headerLabel, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read trainer_headers: %w", err)
	}
	log.Printf("  -> Imported %d trainer_headers (%d unmatched map names)", count, unmatched)
	return nil
}

func buildPostgresMapNameLookup(pg *sql.DB) map[string]int {
	result := make(map[string]int)
	rows, err := pg.Query("SELECT id, name FROM phaser_maps")
	if err != nil {
		log.Printf("  ! Failed to load phaser_maps for name lookup: %v", err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			result[strings.ToLower(strings.ReplaceAll(name, "_", ""))] = id
		}
	}
	return result
}

func importMissableObjectsPostgres(sqlite, pg *sql.DB) error {
	log.Println("Importing missable_objects -> phaser_missable_objects...")
	rows, err := sqlite.Query(`
		SELECT id, hs_index, hs_constant, map_constant, map_id, object_constant,
			object_index, object_name, object_type, initial_state, initial_visible, label
		FROM missable_objects`)
	if err != nil {
		return fmt.Errorf("query missable_objects: %w", err)
	}
	defer rows.Close()

	stmt, err := pg.Prepare(`
		INSERT INTO phaser_missable_objects
			(id, hs_index, hs_constant, map_constant, map_id, object_constant,
			 object_index, object_name, object_type, initial_state, initial_visible, label)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`)
	if err != nil {
		return fmt.Errorf("prepare missable_objects insert: %w", err)
	}
	defer stmt.Close()

	visibilityStmt, err := pg.Prepare(`
		INSERT INTO phaser_event_object_visibility
			(map_id, map_name, object_name, visible, label)
		VALUES ($1, $2, $3, $4, $5)`)
	if err != nil {
		return fmt.Errorf("prepare source visibility insert: %w", err)
	}
	defer visibilityStmt.Close()

	count := 0
	visibilityCount := 0
	for rows.Next() {
		var id, hsIndex, initialVisible int
		var hsConstant, mapConstant, objectConstant, initialState, label string
		var mapID, objectIndex sql.NullInt64
		var objectName, objectType sql.NullString
		if err := rows.Scan(&id, &hsIndex, &hsConstant, &mapConstant, &mapID, &objectConstant, &objectIndex, &objectName, &objectType, &initialState, &initialVisible, &label); err != nil {
			return fmt.Errorf("scan missable_objects row: %w", err)
		}
		if _, err := stmt.Exec(
			id, hsIndex, hsConstant, mapConstant, nullToPtr(mapID), objectConstant,
			nullToPtr(objectIndex), nullStrToPtr(objectName), nullStrToPtr(objectType),
			initialState, initialVisible, label,
		); err != nil {
			return fmt.Errorf("insert missable object %s: %w", hsConstant, err)
		}
		count++

		if !mapID.Valid || !objectName.Valid || objectName.String == "" {
			continue
		}
		var existingRules int
		if err := pg.QueryRow(`
			SELECT COUNT(*)
			FROM phaser_event_object_visibility
			WHERE map_id = $1
			  AND object_name = $2
			  AND (label IS NULL OR label NOT LIKE 'SourceMissableInitial:%')`,
			mapID.Int64, objectName.String).Scan(&existingRules); err != nil {
			return fmt.Errorf("check visibility rules for %s: %w", objectName.String, err)
		}
		if existingRules > 0 {
			continue
		}
		if _, err := visibilityStmt.Exec(mapID.Int64, mapConstant, objectName.String, initialVisible != 0, label); err != nil {
			return fmt.Errorf("insert source visibility row for %s: %w", hsConstant, err)
		}
		visibilityCount++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read missable_objects: %w", err)
	}
	log.Printf("  -> Imported %d missable_objects (%d source visibility rows)", count, visibilityCount)
	return nil
}

func importSpinTilesPostgres(sqlite, pg *sql.DB) error {
	rows, exists, err := loadSpinTileImportRows(sqlite)
	if err != nil {
		return err
	}
	if !exists {
		log.Println("Importing spin_tiles -> phaser_spin_tiles... skipped (source table missing)")
		return nil
	}

	log.Println("Importing spin_tiles -> phaser_spin_tiles...")
	stmt, err := pg.Prepare(`
		INSERT INTO phaser_spin_tiles (id, map_name, x, y, movements)
		VALUES ($1, $2, $3, $4, $5)`)
	if err != nil {
		return fmt.Errorf("prepare spin_tiles insert: %w", err)
	}
	defer stmt.Close()

	for _, row := range rows {
		if _, err := stmt.Exec(row.ID, row.MapName, row.X, row.Y, row.Movements); err != nil {
			return fmt.Errorf("insert spin tile %s (%d,%d): %w", row.MapName, row.X, row.Y, err)
		}
	}
	if err := resetPostgresIdentitySequence(pg, "phaser_spin_tiles"); err != nil {
		return err
	}
	log.Printf("  -> Imported %d spin_tiles", len(rows))
	return nil
}

type spinTileImportRow struct {
	ID        int
	MapName   string
	X         int
	Y         int
	Movements string
}

func loadSpinTileImportRows(sqlite *sql.DB) ([]spinTileImportRow, bool, error) {
	exists, err := sqliteTableExists(sqlite, "spin_tiles")
	if err != nil {
		return nil, false, fmt.Errorf("check spin_tiles: %w", err)
	}
	if !exists {
		return nil, false, nil
	}

	rows, err := sqlite.Query(`SELECT id, map_name, x, y, movements FROM spin_tiles ORDER BY id`)
	if err != nil {
		return nil, true, fmt.Errorf("query spin_tiles: %w", err)
	}
	defer rows.Close()

	result := []spinTileImportRow{}
	for rows.Next() {
		var row spinTileImportRow
		if err := rows.Scan(&row.ID, &row.MapName, &row.X, &row.Y, &row.Movements); err != nil {
			return nil, true, fmt.Errorf("scan spin_tiles row: %w", err)
		}
		row.MapName = mapNameToUpperSnake(row.MapName)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, true, fmt.Errorf("read spin_tiles: %w", err)
	}
	return result, true, nil
}

func importCoordinateTriggersPostgres(sqlite, pg *sql.DB) error {
	log.Println("Importing coordinate_triggers -> phaser_coordinate_triggers...")
	offsets := loadOverworldMapOffsets(sqlite)
	rows, err := sqlite.Query(`SELECT id, map_name, label, x, y FROM coordinate_triggers`)
	if err != nil {
		return fmt.Errorf("query coordinate_triggers: %w", err)
	}
	defer rows.Close()

	stmt, err := pg.Prepare(`INSERT INTO phaser_coordinate_triggers (id, map_name, label, x, y) VALUES ($1, $2, $3, $4, $5)`)
	if err != nil {
		return fmt.Errorf("prepare coordinate_triggers insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for rows.Next() {
		var id, x, y int
		var mapName, label string
		if err := rows.Scan(&id, &mapName, &label, &x, &y); err != nil {
			return fmt.Errorf("scan coordinate trigger row: %w", err)
		}
		if offset, ok := offsets[normalizeMapName(mapName)]; ok {
			x += offset.X
			y += offset.Y
		}
		if _, err := stmt.Exec(id, mapName, label, x, y); err != nil {
			return fmt.Errorf("insert coordinate trigger %s: %w", label, err)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read coordinate_triggers: %w", err)
	}
	if err := refreshImportedMapIDsPostgres(pg, "phaser_coordinate_triggers"); err != nil {
		return err
	}
	if err := resetPostgresIdentitySequence(pg, "phaser_coordinate_triggers"); err != nil {
		return err
	}
	if err := seedCaptureQuestCoordinateTriggersPostgres(pg); err != nil {
		return err
	}
	log.Printf("  -> Imported %d coordinate_triggers", count)
	return nil
}

func refreshImportedMapIDsPostgres(pg *sql.DB, tableName string) error {
	switch tableName {
	case "phaser_map_scripts":
		if _, err := pg.Exec(`
			UPDATE phaser_map_scripts AS ms
			SET map_id = pm.id
			FROM phaser_maps AS pm
			WHERE LOWER(REPLACE(pm.name, '_', '')) = LOWER(ms.map_name)`); err != nil {
			return fmt.Errorf("refresh phaser_map_scripts map_id: %w", err)
		}
		if _, err := pg.Exec(`
			UPDATE phaser_map_scripts
			SET map_id = 9999
			WHERE map_id IN (SELECT id FROM phaser_maps WHERE is_overworld = 1 AND id != 9999)`); err != nil {
			return fmt.Errorf("normalize phaser_map_scripts overworld map_id: %w", err)
		}
	case "phaser_coordinate_triggers":
		if _, err := pg.Exec(`
			UPDATE phaser_coordinate_triggers AS ct
			SET map_id = pm.id
			FROM phaser_maps AS pm
			WHERE LOWER(REPLACE(pm.name, '_', '')) = LOWER(ct.map_name)`); err != nil {
			return fmt.Errorf("refresh phaser_coordinate_triggers map_id: %w", err)
		}
		if _, err := pg.Exec(`
			UPDATE phaser_coordinate_triggers
			SET map_id = 9999
			WHERE map_id IN (SELECT id FROM phaser_maps WHERE is_overworld = 1 AND id != 9999)`); err != nil {
			return fmt.Errorf("normalize phaser_coordinate_triggers overworld map_id: %w", err)
		}
	case "phaser_warp_events":
		if _, err := pg.Exec(`
			UPDATE phaser_warp_events AS we
			SET map_id = pm.id
			FROM phaser_maps AS pm
			WHERE LOWER(REPLACE(pm.name, '_', '')) = LOWER(we.map_name)`); err != nil {
			return fmt.Errorf("refresh phaser_warp_events map_id: %w", err)
		}
	default:
		return fmt.Errorf("unsupported imported map id refresh table %s", tableName)
	}
	return nil
}

func seedCaptureQuestCoordinateTriggersPostgres(pg *sql.DB) error {
	if _, err := pg.Exec(`
		DELETE FROM phaser_coordinate_triggers
		WHERE label IN (
			'PalletTownOakStopsPlayer',
			'Route22RivalBattleCoords',
			'Route23BadgeCheckCascadeCoords',
			'Route23BadgeCheckThunderCoords',
			'Route23BadgeCheckRainbowCoords',
			'Route23BadgeCheckSoulCoords',
			'Route23BadgeCheckMarshCoords',
			'Route23BadgeCheckVolcanoCoords',
			'Route23BadgeCheckEarthCoords',
			'Route16Gate1FStopsPlayerCoords',
			'Route18Gate1FStopsPlayerCoords',
			'TEXT_ROUTE12_SNORLAX',
			'TEXT_ROUTE16_SNORLAX',
			'PokemonTower2FRivalRightSideCoords',
			'PokemonTower2FRivalBelowCoords',
			'SilphCo7FRivalUpperCoords',
			'SilphCo7FRivalLowerCoords',
			'SSAnne2FRivalCoords',
			'ChampionsRoomRivalEntranceCoords',
			'Route24NuggetBridgeRocketCoords'
		)
		OR (
			map_id = 143
			AND label = 'PokemonTower2FRivalEncounterEventCoords'
			AND ((x = 15 AND y = 5) OR (x = 14 AND y = 6))
		)`); err != nil {
		return fmt.Errorf("clear CaptureQuest coordinate trigger rows: %w", err)
	}

	rows := []coordinateTriggerSeed{
		{"PalletTown", 9999, "PalletTownOakStopsPlayer", 10, 1},
		{"PalletTown", 9999, "PalletTownOakStopsPlayer", 11, 1},
		{"Route22", 9999, "Route22RivalBattleCoords", -21, -60},
		{"Route22", 9999, "Route22RivalBattleCoords", -21, -59},
		{"Route23", 34, "Route23BadgeCheckCascadeCoords", 8, 136},
		{"Route23", 34, "Route23BadgeCheckThunderCoords", 8, 119},
		{"Route23", 34, "Route23BadgeCheckRainbowCoords", 12, 105},
		{"Route23", 34, "Route23BadgeCheckSoulCoords", 11, 96},
		{"Route23", 34, "Route23BadgeCheckMarshCoords", 8, 85},
		{"Route23", 34, "Route23BadgeCheckVolcanoCoords", 10, 56},
		{"Route23", 34, "Route23BadgeCheckEarthCoords", 4, 35},
		{"Route16Gate1F", 186, "Route16Gate1FStopsPlayerCoords", 4, 7},
		{"Route16Gate1F", 186, "Route16Gate1FStopsPlayerCoords", 4, 8},
		{"Route16Gate1F", 186, "Route16Gate1FStopsPlayerCoords", 4, 9},
		{"Route16Gate1F", 186, "Route16Gate1FStopsPlayerCoords", 4, 10},
		{"Route18Gate1F", 190, "Route18Gate1FStopsPlayerCoords", 4, 3},
		{"Route18Gate1F", 190, "Route18Gate1FStopsPlayerCoords", 4, 4},
		{"Route18Gate1F", 190, "Route18Gate1FStopsPlayerCoords", 4, 5},
		{"Route18Gate1F", 190, "Route18Gate1FStopsPlayerCoords", 4, 6},
		{"Route12", 9999, "TEXT_ROUTE12_SNORLAX", 280, -38},
		{"Route16", 9999, "TEXT_ROUTE16_SNORLAX", 86, -108},
		{"PokemonTower2F", 143, "PokemonTower2FRivalRightSideCoords", 15, 5},
		{"PokemonTower2F", 143, "PokemonTower2FRivalBelowCoords", 14, 6},
		{"SilphCo7F", 212, "SilphCo7FRivalUpperCoords", 3, 2},
		{"SilphCo7F", 212, "SilphCo7FRivalLowerCoords", 3, 3},
		{"SSAnne2F", 96, "SSAnne2FRivalCoords", 36, 8},
		{"SSAnne2F", 96, "SSAnne2FRivalCoords", 37, 8},
		{"ChampionsRoom", 120, "ChampionsRoomRivalEntranceCoords", 3, 7},
		{"ChampionsRoom", 120, "ChampionsRoomRivalEntranceCoords", 4, 7},
		{"Route24", 9999, "Route24NuggetBridgeRocketCoords", 190, -219},
	}
	stmt, err := pg.Prepare(`
		INSERT INTO phaser_coordinate_triggers (map_name, map_id, label, x, y)
		VALUES ($1, $2, $3, $4, $5)`)
	if err != nil {
		return fmt.Errorf("prepare CaptureQuest coordinate trigger seed: %w", err)
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, err := stmt.Exec(row.MapName, row.MapID, row.Label, row.X, row.Y); err != nil {
			return fmt.Errorf("seed CaptureQuest coordinate trigger %s (%d,%d): %w", row.Label, row.X, row.Y, err)
		}
	}
	return nil
}

func applyGameCornerHiddenCoinAmountsPostgres(pg *sql.DB) {
	amounts := []struct {
		id     int
		amount int
	}{
		{id: 1, amount: 10},
		{id: 2, amount: 10},
		{id: 3, amount: 20},
		{id: 4, amount: 10},
		{id: 5, amount: 10},
		{id: 6, amount: 20},
		{id: 7, amount: 10},
		{id: 8, amount: 10},
		{id: 9, amount: 10},
		{id: 10, amount: 20},
		{id: 11, amount: 100},
		{id: 12, amount: 10},
	}
	stmt, err := pg.Prepare(`UPDATE phaser_hidden_coins SET coin_amount = $1 WHERE id = $2`)
	if err != nil {
		log.Printf("  ! Could not prepare hidden coin amount update: %v", err)
		return
	}
	defer stmt.Close()
	for _, amount := range amounts {
		if _, err := stmt.Exec(amount.amount, amount.id); err != nil {
			log.Printf("  ! Could not update hidden coin %d amount: %v", amount.id, err)
		}
	}
}

func populateGrowthRatesPostgres(pg *sql.DB) {
	log.Println("Populating growth rates...")
	mediumSlow := []int{
		1, 2, 3, 4, 5, 6, 7, 8, 9,
		29, 30, 31, 32, 33, 34,
		41, 42,
		43, 44, 45,
		58, 59,
		60, 61, 62,
		63, 64, 65,
		66, 67, 68,
		69, 70, 71,
		72, 73,
		74, 75, 76,
		79, 80,
		92, 93, 94,
		95,
		98, 99,
		100, 101,
		102, 103,
		104, 105,
		108,
		109, 110,
		111, 112,
		113,
		114,
		116, 117,
		123,
		127,
		131,
		137,
		138, 139,
		140, 141,
		142,
		143,
		147, 148, 149,
	}
	fast := []int{35, 36, 39, 40}
	slow := []int{144, 145, 146, 150, 151}
	updateGrowthRatesPostgres(pg, "MEDIUM_SLOW", mediumSlow)
	updateGrowthRatesPostgres(pg, "FAST", fast)
	updateGrowthRatesPostgres(pg, "SLOW", slow)
	log.Printf("  -> Set growth rates: %d MEDIUM_SLOW, %d FAST, %d SLOW, rest MEDIUM_FAST", len(mediumSlow), len(fast), len(slow))
}

func updateGrowthRatesPostgres(pg *sql.DB, growthRate string, ids []int) {
	stmt, err := pg.Prepare(`UPDATE phaser_pokemon SET growth_rate = $1 WHERE id = $2`)
	if err != nil {
		log.Printf("  ! Could not prepare growth rate update: %v", err)
		return
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.Exec(growthRate, id); err != nil {
			log.Printf("  ! Could not update growth rate for pokemon %d: %v", id, err)
		}
	}
}

func resetPostgresIdentitySequences(pg *sql.DB) error {
	tables := []string{
		"phaser_tile_images",
		"phaser_tiles",
		"phaser_objects",
		"phaser_warps",
		"phaser_dialogue_text",
		"phaser_text_pointers",
		"phaser_trainer_headers",
		"phaser_pokemon_learnset",
		"phaser_pokemon_tmhm",
		"phaser_wild_encounters",
		"phaser_encounter_slots",
		"phaser_encounter_areas",
		"phaser_encounter_area_slots",
		"phaser_trainer_parties",
		"phaser_trainer_party_pokemon",
		"phaser_hidden_items",
		"phaser_hidden_coins",
		"phaser_hidden_objects",
		"phaser_missable_objects",
		"phaser_map_music",
		"phaser_map_scripts",
		"phaser_npc_movement_data",
		"phaser_event_flags",
		"phaser_coordinate_triggers",
		"phaser_spin_tiles",
		"phaser_warp_events",
		"phaser_game_corner_prizes",
		"poke_start_cities",
		"cq_merchants",
		"cq_merchant_items",
	}
	for _, table := range tables {
		if err := resetPostgresIdentitySequence(pg, table); err != nil {
			return err
		}
	}
	return nil
}

func resetPostgresIdentitySequence(pg *sql.DB, table string) error {
	query := fmt.Sprintf(`
		SELECT setval(
			pg_get_serial_sequence('%s', 'id'),
			GREATEST(COALESCE((SELECT MAX(id) FROM %s), 1), 1),
			COALESCE((SELECT MAX(id) FROM %s), 0) > 0
		)`, table, table, table)
	if _, err := pg.Exec(query); err != nil {
		return fmt.Errorf("reset identity for %s: %w", table, err)
	}
	return nil
}

func nullToPtr(n sql.NullInt64) any {
	if n.Valid {
		return n.Int64
	}
	return nil
}

func nullStrToPtr(n sql.NullString) any {
	if n.Valid {
		return n.String
	}
	return nil
}

// normalizeMapName converts a CamelCase map name from pokered ASM to the
// normalized lookup form used for phaser_maps: lowercase with no separators.
func normalizeMapName(camelCase string) string {
	var b strings.Builder
	for _, r := range camelCase {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

type coordinateOffset struct {
	X int
	Y int
}

func loadOverworldMapOffsets(sqlite *sql.DB) map[string]coordinateOffset {
	offsets := make(map[string]coordinateOffset)
	rows, err := sqlite.Query(`SELECT map_name, x_offset, y_offset FROM overworld_map_positions`)
	if err != nil {
		log.Printf("  ! Failed to load overworld map offsets for coordinate triggers: %v", err)
		return offsets
	}
	defer rows.Close()

	for rows.Next() {
		var mapName string
		var x, y int
		if err := rows.Scan(&mapName, &x, &y); err != nil {
			log.Printf("  ! Failed to scan overworld map offset: %v", err)
			continue
		}
		offsets[normalizeMapName(mapName)] = coordinateOffset{X: x, Y: y}
	}
	return offsets
}

type coordinateTriggerSeed struct {
	MapName string
	MapID   int
	Label   string
	X       int
	Y       int
}
