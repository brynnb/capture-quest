package main

import (
	"database/sql"
	"fmt"
	"strings"
)

const (
	extractorSchemaName    = "pokemon-gameboy-extractor"
	extractorSchemaVersion = 2
	extractorReaderVersion = 2
)

var supportedPokemonReleases = map[string]struct{}{
	"blue": {},
	"red":  {},
}

var requiredExtractorTables = []string{
	"audio_assets", "dialogue_text", "encounter_slots", "event_flags",
	"extraction_run_releases", "extraction_runs", "game_releases",
	"hidden_coins", "hidden_items", "hidden_objects", "items", "map_music",
	"map_scripts", "maps", "moves", "npc_movement_data", "objects",
	"pokemon", "pokemon_default_moves", "pokemon_learnset", "pokemon_tmhm",
	"schema_metadata", "text_pointers", "tile_images", "tiles", "trainer_classes",
	"trainer_headers", "trainer_parties", "trainer_party_pokemon", "warps",
	"warp_events", "wild_encounters",
}

// extractorImportContext is the immutable source contract that is negotiated
// before CaptureQuest opens or mutates Postgres.
type extractorImportContext struct {
	SchemaName           string
	SchemaVersion        int
	MinimumReaderVersion int
	AppliedEpoch         int64
	RunID                string
	ExtractorRevision    string
	SourceRevision       string
	SourceDateEpoch      int64
	SourceRoot           string
	SourceTreeSHA256     string
	ReleaseCode          string
	ReleaseTitle         string
	ReleaseVariant       string
	ReleasePlatform      string
	ReleaseRegion        string
	ReleaseLanguage      string
	ReleaseBuildDefine   string
}

func normalizePokemonRelease(release string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(release))
	if _, ok := supportedPokemonReleases[normalized]; !ok {
		return "", fmt.Errorf("unsupported Pokemon release %q; choose red or blue", release)
	}
	return normalized, nil
}

func negotiateExtractorSource(sqlite *sql.DB, requestedRelease string) (extractorImportContext, error) {
	release, err := normalizePokemonRelease(requestedRelease)
	if err != nil {
		return extractorImportContext{}, err
	}

	var quickCheck string
	if err := sqlite.QueryRow(`PRAGMA quick_check`).Scan(&quickCheck); err != nil {
		return extractorImportContext{}, fmt.Errorf("check extractor SQLite integrity: %w", err)
	}
	if quickCheck != "ok" {
		return extractorImportContext{}, fmt.Errorf("extractor SQLite quick_check failed: %s", quickCheck)
	}
	if err := requireExtractorTables(sqlite); err != nil {
		return extractorImportContext{}, err
	}

	context := extractorImportContext{}
	metadataRows, err := sqlite.Query(`
		SELECT schema_name, schema_version, minimum_reader_version, applied_epoch
		FROM schema_metadata`)
	if err != nil {
		return extractorImportContext{}, fmt.Errorf("read extractor schema metadata: %w", err)
	}
	if !metadataRows.Next() {
		metadataRows.Close()
		return extractorImportContext{}, fmt.Errorf("extractor schema_metadata must contain exactly one row")
	}
	if err := metadataRows.Scan(
		&context.SchemaName,
		&context.SchemaVersion,
		&context.MinimumReaderVersion,
		&context.AppliedEpoch,
	); err != nil {
		metadataRows.Close()
		return extractorImportContext{}, fmt.Errorf("scan extractor schema metadata: %w", err)
	}
	if metadataRows.Next() {
		metadataRows.Close()
		return extractorImportContext{}, fmt.Errorf("extractor schema_metadata must contain exactly one row")
	}
	if err := metadataRows.Err(); err != nil {
		metadataRows.Close()
		return extractorImportContext{}, fmt.Errorf("read extractor schema metadata: %w", err)
	}
	if err := metadataRows.Close(); err != nil {
		return extractorImportContext{}, fmt.Errorf("close extractor schema metadata rows: %w", err)
	}

	if context.SchemaName != extractorSchemaName {
		return extractorImportContext{}, fmt.Errorf(
			"unsupported extractor schema %q; expected %q",
			context.SchemaName,
			extractorSchemaName,
		)
	}
	if context.SchemaVersion != extractorSchemaVersion {
		return extractorImportContext{}, fmt.Errorf(
			"unsupported extractor schema version %d; CaptureQuest requires version %d",
			context.SchemaVersion,
			extractorSchemaVersion,
		)
	}
	if context.MinimumReaderVersion < 1 || context.MinimumReaderVersion > context.SchemaVersion {
		return extractorImportContext{}, fmt.Errorf(
			"invalid extractor minimum_reader_version %d for schema version %d",
			context.MinimumReaderVersion,
			context.SchemaVersion,
		)
	}
	if context.MinimumReaderVersion > extractorReaderVersion {
		return extractorImportContext{}, fmt.Errorf(
			"extractor requires reader version %d; CaptureQuest implements reader version %d",
			context.MinimumReaderVersion,
			extractorReaderVersion,
		)
	}

	runRows, err := sqlite.Query(`
		SELECT run_id, extractor_revision, source_revision, source_date_epoch,
		       source_root, source_tree_sha256
		FROM extraction_runs
		WHERE schema_name = ? AND schema_version = ?`,
		context.SchemaName,
		context.SchemaVersion,
	)
	if err != nil {
		return extractorImportContext{}, fmt.Errorf("read extractor run metadata: %w", err)
	}
	if !runRows.Next() {
		runRows.Close()
		return extractorImportContext{}, fmt.Errorf("extraction_runs must contain exactly one run for the negotiated schema")
	}
	if err := runRows.Scan(
		&context.RunID,
		&context.ExtractorRevision,
		&context.SourceRevision,
		&context.SourceDateEpoch,
		&context.SourceRoot,
		&context.SourceTreeSHA256,
	); err != nil {
		runRows.Close()
		return extractorImportContext{}, fmt.Errorf("scan extractor run metadata: %w", err)
	}
	if runRows.Next() {
		runRows.Close()
		return extractorImportContext{}, fmt.Errorf("extraction_runs must contain exactly one run for the negotiated schema")
	}
	if err := runRows.Err(); err != nil {
		runRows.Close()
		return extractorImportContext{}, fmt.Errorf("read extractor run metadata: %w", err)
	}
	if err := runRows.Close(); err != nil {
		return extractorImportContext{}, fmt.Errorf("close extractor run metadata rows: %w", err)
	}

	if err := sqlite.QueryRow(`
		SELECT release.release_code, release.title, release.variant,
		       release.platform, release.region, release.language,
		       release.build_define
		FROM game_releases AS release
		JOIN extraction_run_releases AS link
		  ON link.release_code = release.release_code
		WHERE link.run_id = ? AND release.release_code = ?`,
		context.RunID,
		release,
	).Scan(
		&context.ReleaseCode,
		&context.ReleaseTitle,
		&context.ReleaseVariant,
		&context.ReleasePlatform,
		&context.ReleaseRegion,
		&context.ReleaseLanguage,
		&context.ReleaseBuildDefine,
	); err != nil {
		if err == sql.ErrNoRows {
			return extractorImportContext{}, fmt.Errorf(
				"extractor run %s does not publish the %s release",
				context.RunID,
				release,
			)
		}
		return extractorImportContext{}, fmt.Errorf("read extractor release metadata: %w", err)
	}

	var foreignKeyTable string
	var foreignKeyRowID sql.NullInt64
	var foreignKeyParent string
	var foreignKeyID int
	foreignKeyErr := sqlite.QueryRow(`PRAGMA foreign_key_check`).Scan(
		&foreignKeyTable,
		&foreignKeyRowID,
		&foreignKeyParent,
		&foreignKeyID,
	)
	if foreignKeyErr == nil {
		return extractorImportContext{}, fmt.Errorf(
			"extractor database has a foreign-key violation in %s row %v referencing %s",
			foreignKeyTable,
			foreignKeyRowID,
			foreignKeyParent,
		)
	}
	if foreignKeyErr != sql.ErrNoRows {
		return extractorImportContext{}, fmt.Errorf("check extractor foreign keys: %w", foreignKeyErr)
	}

	if err := validateDefaultMoveConstants(sqlite); err != nil {
		return extractorImportContext{}, err
	}

	return context, nil
}

func requireExtractorTables(sqlite *sql.DB) error {
	missing := make([]string, 0)
	for _, table := range requiredExtractorTables {
		var present int
		if err := sqlite.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&present); err != nil {
			return fmt.Errorf("inspect extractor table %s: %w", table, err)
		}
		if present != 1 {
			missing = append(missing, table)
		}
	}
	if len(missing) != 0 {
		return fmt.Errorf("extractor database is missing required tables: %s", strings.Join(missing, ", "))
	}
	return nil
}

func validateDefaultMoveConstants(sqlite *sql.DB) error {
	row := sqlite.QueryRow(`
		WITH default_moves (pokemon_id, slot_index, move_id, move_name) AS (
			SELECT id, 1, default_move_1_id, default_move_1_name FROM pokemon
			UNION ALL
			SELECT id, 2, default_move_2_id, default_move_2_name FROM pokemon
			UNION ALL
			SELECT id, 3, default_move_3_id, default_move_3_name FROM pokemon
			UNION ALL
			SELECT id, 4, default_move_4_id, default_move_4_name FROM pokemon
		)
		SELECT defaults.pokemon_id, defaults.slot_index,
		       defaults.move_id, defaults.move_name,
		       COALESCE(move.constant_name, '')
		FROM default_moves AS defaults
		LEFT JOIN moves AS move ON move.id = defaults.move_id
		WHERE (defaults.move_id IS NULL) <> (defaults.move_name = 'NO_MOVE')
		   OR (defaults.move_id IS NOT NULL AND move.constant_name IS NULL)
		   OR (defaults.move_id IS NOT NULL AND move.constant_name <> defaults.move_name)
		ORDER BY defaults.pokemon_id, defaults.slot_index
		LIMIT 1`)

	var pokemonID, slot int
	var moveID sql.NullInt64
	var moveName, constantName string
	if err := row.Scan(&pokemonID, &slot, &moveID, &moveName, &constantName); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("validate Pokemon default-move constants: %w", err)
	}
	return fmt.Errorf(
		"pokemon %d default move slot %d is inconsistent: move_id=%v, move_name=%q, moves.constant_name=%q",
		pokemonID,
		slot,
		moveID,
		moveName,
		constantName,
	)
}
