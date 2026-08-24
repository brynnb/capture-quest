package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhaserMovesSchemaAndImportIncludeBattleAudioMetadata(t *testing.T) {
	requiredColumns := []string{
		"battle_sound_pitch",
		"battle_sound_tempo",
		"battle_subanimation",
		"battle_tileset",
		"battle_delay",
	}

	importColumns := map[string]bool{}
	for _, column := range phaserMoveImportColumns {
		importColumns[column] = true
	}

	schemaPath := filepath.Join("..", "..", "schema", "postgres_runtime_schema.sql")
	rawSchema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read postgres schema: %v", err)
	}
	schema := string(rawSchema)

	for _, column := range requiredColumns {
		if !importColumns[column] {
			t.Fatalf("phaserMoveImportColumns missing %s", column)
		}
		if !strings.Contains(schema, column) {
			t.Fatalf("postgres_runtime_schema.sql missing %s", column)
		}
	}
}

func TestPhaserTilesSchemaPreservesSourceMapID(t *testing.T) {
	schemaPath := filepath.Join("..", "..", "schema", "postgres_runtime_schema.sql")
	rawSchema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read postgres schema: %v", err)
	}
	if !strings.Contains(string(rawSchema), "source_map_id integer") {
		t.Fatal("postgres_runtime_schema.sql missing phaser_tiles.source_map_id")
	}
}

func TestPhaserTilesSchemaAndImportPreserveTileProvenance(t *testing.T) {
	schemaPath := filepath.Join("..", "..", "schema", "postgres_runtime_schema.sql")
	rawSchema, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read postgres schema: %v", err)
	}
	schema := string(rawSchema)
	for _, fragment := range []string{
		"is_native_game_data boolean",
		"coordinate_origin varchar(16)",
		"content_origin varchar(16)",
		"coordinate_origin IN ('native', 'generated', 'user')",
		"content_origin IN ('native', 'generated', 'user', 'event')",
	} {
		if !strings.Contains(schema, fragment) {
			t.Fatalf("postgres_runtime_schema.sql missing provenance fragment %q", fragment)
		}
	}

	const importerSource = "is_native_game_data, coordinate_origin, content_origin"
	rawImporter, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatalf("read postgres importer: %v", err)
	}
	if !strings.Contains(string(rawImporter), importerSource) ||
		!strings.Contains(string(rawImporter), "TRUE, 'native', 'native'") {
		t.Fatal("tile importer does not explicitly mark extracted rows as native")
	}
}
