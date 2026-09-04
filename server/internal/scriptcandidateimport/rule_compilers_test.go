package scriptcandidateimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

func TestRunWritesGeneratedEventTileOverrides(t *testing.T) {
	dbPath := createTileOverrideSQLite(t)
	root := t.TempDir()
	outputDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TileOverrideRead != 1 || stats.TileOverrideRules != 4 || stats.TileOverrideWritten != 1 {
		t.Fatalf("stats = %+v, want one tile candidate/four rules/one written file", stats)
	}

	path := filepath.Join(root, generatedEventTilesFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file scriptedevents.EventTileOverrideFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	want := scriptedevents.EventTileOverrideRule{
		MapID:         7,
		MapName:       "TEST_MAP",
		X:             4,
		Y:             6,
		TileImageID:   42,
		CollisionType: 1,
		RequiresFlag:  "EVENT_OPEN_TEST_GATE",
		Label:         "TestGateOpen_0_0",
	}
	if !containsImportedEventTileRule(file.Tiles, want) {
		t.Fatalf("generated rules = %#v, missing %#v", file.Tiles, want)
	}
}

func TestRunWritesGeneratedConditionalDialogue(t *testing.T) {
	dbPath := createConditionalDialogueSQLite(t)
	root := t.TempDir()
	outputDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.ConditionalDialogueRead != 1 || stats.ConditionalDialogueRules != 1 || stats.ConditionalDialogueWritten != 1 {
		t.Fatalf("stats = %+v, want one generated conditional dialogue rule", stats)
	}

	path := filepath.Join(root, generatedConditionalDialogueFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file scriptedevents.ConditionalDialogueFile
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Rows) != 1 {
		t.Fatalf("rows = %#v, want 1", file.Rows)
	}
	row := file.Rows[0]
	if row.TextConstant != "TEXT_OAKSLAB_RIVAL" || row.Priority != 300 {
		t.Fatalf("row = %+v, want Oak rival priority 300", row)
	}
	if len(row.RequiresFlagsAbsent) != 1 || row.RequiresFlagsAbsent[0] != "EVENT_FOLLOWED_OAK_INTO_LAB_2" {
		t.Fatalf("requiresFlagsAbsent = %#v", row.RequiresFlagsAbsent)
	}
	if len(row.DialogueLabels) != 1 || row.DialogueLabels[0] != "_OaksLabRivalGrampsIsntAroundText" {
		t.Fatalf("dialogueLabels = %#v", row.DialogueLabels)
	}
}

func TestRunWritesGeneratedObjectVisibility(t *testing.T) {
	dbPath := createObjectVisibilitySQLite(t)
	root := t.TempDir()
	outputDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.ObjectVisibilityRead != 1 || stats.ObjectVisibilityRules != 1 || stats.ObjectVisibilityWritten != 1 {
		t.Fatalf("stats = %+v, want one generated object visibility rule", stats)
	}

	path := filepath.Join(root, generatedObjectVisibilityFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rules []scriptedevents.ObjectVisibilityRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		t.Fatal(err)
	}
	want := scriptedevents.ObjectVisibilityRule{
		MapID:        1,
		MapName:      "VIRIDIAN_CITY",
		ObjectName:   "ViridianCity_NPC_5",
		Visible:      false,
		RequiresFlag: "EVENT_GOT_POKEDEX",
		Label:        "OaksLabOakGivesPokedexScript:EVENT_GOT_POKEDEX:HS_LYING_OLD_MAN:HideObject",
	}
	if len(rules) != 1 || rules[0] != want {
		t.Fatalf("rules = %#v, want %#v", rules, want)
	}
}

func TestRunSkipsGeneratedEventTileOverrideOwnedByManualFile(t *testing.T) {
	dbPath := createTileOverrideSQLite(t)
	root := t.TempDir()
	outputDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "event_tile_overrides.json"), []byte(`{
  "tiles": [
    {
      "mapId": 7,
      "mapName": "TEST_MAP",
      "x": 4,
      "y": 6,
      "tileImageId": 99,
      "collisionType": 0,
      "requiresFlag": "EVENT_OPEN_TEST_GATE",
      "label": "ManualTestGateOpen"
    }
  ]
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	reportPath := filepath.Join(root, "script_candidate_import_diagnostics.json")
	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir, DiagnosticsPath: reportPath})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TileOverrideSkippedOverrides != 1 || stats.TileOverrideRules != 0 {
		t.Fatalf("stats = %+v, want generated tile candidate skipped by manual file", stats)
	}

	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var report importReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Decisions) != 1 {
		t.Fatalf("report decisions = %d, want 1", len(report.Decisions))
	}
	var details struct {
		ManualTileKey      string                                 `json:"manualTileKey"`
		ManualPath         string                                 `json:"manualPath"`
		GeneratedRules     []scriptedevents.EventTileOverrideRule `json:"generatedRules"`
		SourceReplacements []tileOverrideReplacement              `json:"sourceReplacements"`
	}
	if err := json.Unmarshal(report.Decisions[0].Details, &details); err != nil {
		t.Fatal(err)
	}
	if details.ManualTileKey != "7|TEST_MAP|4|6|EVENT_OPEN_TEST_GATE|" {
		t.Fatalf("manual tile key = %q", details.ManualTileKey)
	}
	if details.ManualPath != filepath.Join(root, "event_tile_overrides.json") {
		t.Fatalf("manual path = %q", details.ManualPath)
	}
	if len(details.GeneratedRules) != 4 {
		t.Fatalf("generated rules = %d, want 4", len(details.GeneratedRules))
	}
}

func TestRunPreservesGeneratedEventTileOverridesWhenAllCandidatesFailResolution(t *testing.T) {
	dbPath := createTileOverrideSQLite(t)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM tile_images`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	outputDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	generatedPath := filepath.Join(root, generatedEventTilesFileName)
	existing := []byte(`{
  "tiles": [
    {
      "mapId": 7,
      "mapName": "TEST_MAP",
      "x": 4,
      "y": 6,
      "tileImageId": 99,
      "collisionType": 0,
      "requiresFlag": "EVENT_OPEN_TEST_GATE",
      "label": "ExistingGeneratedRule"
    }
  ]
}
`)
	if err := os.WriteFile(generatedPath, existing, 0o644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.TileOverrideUnsupported != 1 || stats.TileOverrideRules != 0 || stats.TileOverrideWritten != 0 {
		t.Fatalf("stats = %+v, want one unsupported tile candidate and no generated file write", stats)
	}
	raw, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(existing) {
		t.Fatalf("generated event tile file was overwritten:\n%s", raw)
	}
}
