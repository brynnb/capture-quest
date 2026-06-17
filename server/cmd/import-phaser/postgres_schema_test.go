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
