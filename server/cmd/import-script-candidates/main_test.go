package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

func TestRunNoopsWhenCandidateTableMissing(t *testing.T) {
	dbPath := createSQLite(t, false, nil)
	outputDir := t.TempDir()

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Read != 0 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want no-op", stats)
	}
	if entries, err := os.ReadDir(outputDir); err != nil || len(entries) != 0 {
		t.Fatalf("output entries = %v, err = %v; want empty", entries, err)
	}
}

func TestRunWritesSupportedCandidate(t *testing.T) {
	candidate := safariCandidate("SafariZoneGateEntryOffer", "EVENT_IN_SAFARI_ZONE", "")
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Read != 1 || stats.Written != 1 || stats.Unchanged != 0 {
		t.Fatalf("stats = %+v, want one write", stats)
	}

	path := filepath.Join(outputDir, "safari_zone_gate_entry_offer.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var event scriptedevents.EventFile
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	if event.ScriptLabel != "SafariZoneGateEntryOffer" {
		t.Fatalf("scriptLabel = %q", event.ScriptLabel)
	}
	if event.MapName != "SAFARI_ZONE_GATE" {
		t.Fatalf("mapName = %q", event.MapName)
	}
	if event.Trigger.Source != extractorSource || event.Trigger.Label != "SafariZoneGateEntryOffer" {
		t.Fatalf("trigger = %+v", event.Trigger)
	}
	if event.RequiresFlagAbsent != "EVENT_IN_SAFARI_ZONE" {
		t.Fatalf("requiresFlagAbsent = %q", event.RequiresFlagAbsent)
	}
	if len(event.Actions) != 6 {
		t.Fatalf("actions len = %d, want 6: %s", len(event.Actions), string(raw))
	}
}

func TestMapCandidateSupportsMultiFlagConditions(t *testing.T) {
	candidate := safariCandidate("ViridianCityGamblerGymClosed", "", "")
	candidate.Conditions = candidateCondition{
		RequiresEventsAbsent: []string{"EVENT_BEAT_VIRIDIAN_GYM_GIOVANNI"},
		RequiresBadgesAbsent: []string{"EARTHBADGE"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresFlag != "" || event.RequiresFlagAbsent != "" {
		t.Fatalf("scalar flags = %q/%q, want array-only absent conditions", event.RequiresFlag, event.RequiresFlagAbsent)
	}
	wantAbsent := []string{"EVENT_BEAT_VIRIDIAN_GYM_GIOVANNI", "EVENT_GOT_EARTHBADGE"}
	if len(event.RequiresFlagsAbsent) != len(wantAbsent) {
		t.Fatalf("requiresFlagsAbsent = %#v, want %#v", event.RequiresFlagsAbsent, wantAbsent)
	}
	for i := range wantAbsent {
		if event.RequiresFlagsAbsent[i] != wantAbsent[i] {
			t.Fatalf("requiresFlagsAbsent = %#v, want %#v", event.RequiresFlagsAbsent, wantAbsent)
		}
	}
}

func TestMapCandidateMapsWarpActionToEventWarp(t *testing.T) {
	candidate := safariCandidate("PokemonTower7FMrFujiRescue", "EVENT_RESCUED_MR_FUJI", "")
	candidate.MapName = "PokemonTower7F"
	candidate.Trigger = candidateTrigger{
		Type:  "npc_click",
		Label: "TEXT_POKEMONTOWER7F_MR_FUJI",
	}
	candidate.Actions = []candidateAction{
		{Type: "lockInput"},
		{Type: "dialogue", Speaker: "MR. FUJI", Lines: []string{"Follow me to my home."}},
		{Type: "setEvent", Event: "EVENT_RESCUED_MR_FUJI"},
		{Type: "warp", MapID: 149, X: 3, Y: 7, Direction: "UP"},
		{Type: "unlockInput"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if event.Warp == nil {
		t.Fatal("event.Warp is nil")
	}
	if *event.Warp != (scriptedevents.EventWarp{MapID: 149, X: 3, Y: 7}) {
		t.Fatalf("warp = %+v", *event.Warp)
	}
	if len(event.Actions) != 4 {
		t.Fatalf("actions len = %d, want 4", len(event.Actions))
	}
	for _, raw := range event.Actions {
		var action map[string]any
		if err := json.Unmarshal(raw, &action); err != nil {
			t.Fatal(err)
		}
		if action["type"] == "warp" {
			t.Fatalf("warp action should not remain in actions: %s", string(raw))
		}
	}
}

func TestMapActionsPreservesAudioActions(t *testing.T) {
	actions, err := mapActions([]candidateAction{
		{Type: "playSFX", SFXConstant: "SFX_GET_ITEM_1", Volume: 0.6},
		{Type: "playCry", PokemonName: "PIKACHU", SFXConstant: "SFX_CRY_0F"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Fatalf("actions len = %d, want 2", len(actions))
	}

	var sfx map[string]any
	if err := json.Unmarshal(actions[0], &sfx); err != nil {
		t.Fatal(err)
	}
	if got, want := sfx["type"], "playSFX"; got != want {
		t.Fatalf("sfx type = %v, want %q", got, want)
	}
	if got, want := sfx["sfxConstant"], "SFX_GET_ITEM_1"; got != want {
		t.Fatalf("sfx constant = %v, want %q", got, want)
	}
	if got, want := sfx["volume"], 0.6; got != want {
		t.Fatalf("sfx volume = %v, want %v", got, want)
	}

	var cry map[string]any
	if err := json.Unmarshal(actions[1], &cry); err != nil {
		t.Fatal(err)
	}
	if got, want := cry["type"], "playCry"; got != want {
		t.Fatalf("cry type = %v, want %q", got, want)
	}
	if got, want := cry["pokemonName"], "PIKACHU"; got != want {
		t.Fatalf("cry pokemonName = %v, want %q", got, want)
	}
	if got, want := cry["sfxConstant"], "SFX_CRY_0F"; got != want {
		t.Fatalf("cry fallback = %v, want %q", got, want)
	}
}

func TestMapActionsPreservesGameCornerPrizeVendorWindow(t *testing.T) {
	actions, err := mapActions([]candidateAction{
		{
			Type:         "gameCornerPrizeVendor",
			TextConstant: "TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_2",
			PrizeWindow:  2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions len = %d, want 1", len(actions))
	}

	var action map[string]any
	if err := json.Unmarshal(actions[0], &action); err != nil {
		t.Fatal(err)
	}
	if got, want := action["type"], "gameCornerPrizeVendor"; got != want {
		t.Fatalf("type = %v, want %q", got, want)
	}
	if got, want := action["textConstant"], "TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_2"; got != want {
		t.Fatalf("textConstant = %v, want %q", got, want)
	}
	if got, want := action["prizeWindow"], float64(2); got != want {
		t.Fatalf("prizeWindow = %v, want %v", got, want)
	}
}

func TestCanShareMapSetFlagWithExistingExtractorBattle(t *testing.T) {
	event := scriptedevents.EventFile{
		ScriptLabel: "RocketHideoutB4FGiovanniEncounter",
		MapName:     "ROCKET_HIDEOUT_B4F",
		Trigger: scriptedevents.EventTrigger{
			Type:   "npc_click",
			Source: extractorSource,
			Label:  "TEXT_ROCKETHIDEOUTB4F_GIOVANNI",
		},
		Actions: []json.RawMessage{
			rawAction(map[string]any{
				"type":         "startTrainerBattle",
				"trainerClass": "GIOVANNI",
				"partyIndex":   1,
				"winFlag":      "EVENT_BEAT_ROCKET_HIDEOUT_GIOVANNI",
				"postWinActions": []json.RawMessage{
					rawAction(map[string]any{"type": "setFlag", "flag": "EVENT_ROCKET_HIDEOUT_GIOVANNI_LEFT"}),
				},
			}),
		},
	}

	if !canShareMapSetFlagWithExistingExtractorBattle(event, existingScript{ScriptLabel: "RocketHideoutB4FGiovanniDefeated"}) {
		t.Fatal("extractor battle should share a post-win map flag with a legacy non-CaptureQuest owner")
	}
	if canShareMapSetFlagWithExistingExtractorBattle(event, existingScript{ScriptLabel: "ManualOverride", Source: capturequestSource}) {
		t.Fatal("CaptureQuest-owned scripts must still block generated map flag sharing")
	}
}

func TestRunWritesGeneratedEventTileOverrides(t *testing.T) {
	dbPath := createTileOverrideSQLite(t)
	root := t.TempDir()
	outputDir := filepath.Join(root, "scripts")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
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

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
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

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
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
	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir, DiagnosticsPath: reportPath})
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

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
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

func TestRunPreservesCaptureQuestOverride(t *testing.T) {
	candidate := safariCandidate("SafariZoneGateExit", "", "EVENT_IN_SAFARI_ZONE")
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	path := filepath.Join(outputDir, "safari_zone_gate_exit.json")
	override := []byte(`{
  "scriptLabel": "SafariZoneGateExit",
  "mapName": "SAFARI_ZONE_GATE",
  "trigger": {
    "type": "coord",
    "source": "capturequest",
    "label": "SafariZoneGateExit",
    "coordinates": [{"mapName": "SafariZoneGate", "mapId": 156, "x": 3, "y": 2}]
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(path, override, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want one skipped override", stats)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(override) {
		t.Fatalf("override was modified:\n%s", after)
	}
}

func TestRunPreservesCaptureQuestTriggerOverride(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "BikeShop",
		ScriptLabel: "GeneratedBikeShopClerk",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_BIKESHOP_CLERK",
		},
		Actions:    []candidateAction{{Type: "lockInput"}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	overridePath := filepath.Join(outputDir, "bike_shop_exchange_voucher.json")
	override := []byte(`{
  "scriptLabel": "BikeShopExchangeVoucher",
  "mapName": "BIKE_SHOP",
  "trigger": {
    "type": "npc_click",
    "source": "capturequest",
    "label": "TEXT_BIKESHOP_CLERK"
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(overridePath, override, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want trigger override skipped", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_bike_shop_clerk.json")); !os.IsNotExist(err) {
		t.Fatalf("generated trigger duplicate exists or stat failed: %v", err)
	}
	after, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(override) {
		t.Fatalf("override was modified:\n%s", after)
	}
}

func TestRunSkipsExistingExtractorTriggerOwner(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route12Gate2F",
		ScriptLabel: "GeneratedRoute12Gift",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE12GATE2F_BRUNETTE_GIRL",
		},
		Actions:    []candidateAction{{Type: "giveItem", ItemConstant: "TM_SWIFT"}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route12_gate2_ftm39_swift.json")
	existing := []byte(`{
  "scriptLabel": "Route12Gate2FTM39Swift",
  "mapName": "ROUTE12_GATE2_F",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_ROUTE12GATE2F_BRUNETTE_GIRL"
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want existing trigger skipped", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_route12_gift.json")); !os.IsNotExist(err) {
		t.Fatalf("generated trigger duplicate exists or stat failed: %v", err)
	}
}

func TestRunMergesSourceAudioIntoExistingExtractorReward(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route12Gate2F",
		ScriptLabel: "GeneratedRoute12Gift",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE12GATE2F_BRUNETTE_GIRL",
		},
		Conditions: candidateCondition{
			RequiresEventAbsent: "EVENT_GOT_TM39",
		},
		Actions: []candidateAction{
			{Type: "dialogue", Lines: []string{"TM39 is SWIFT!"}},
			{Type: "giveItem", ItemConstant: "TM_SWIFT"},
			{Type: "playSFX", SFXConstant: "SFX_GET_ITEM_1"},
			{Type: "setEvent", Event: "EVENT_GOT_TM39"},
			{Type: "unlockInput"},
		},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route12_gate2_ftm39_swift.json")
	existing := []byte(`{
  "scriptLabel": "Route12Gate2FTM39Swift",
  "mapName": "ROUTE12_GATE2_F",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_ROUTE12GATE2F_BRUNETTE_GIRL"
  },
  "setsFlags": ["EVENT_GOT_TM39"],
  "actions": [
    {"type": "lockInput"},
    {"type": "dialogue", "lines": ["TM39 is SWIFT!"]},
    {"type": "setFlag", "flag": "EVENT_GOT_TM39"},
    {"type": "unlockInput"},
    {"type": "giveItem", "itemName": "TM_SWIFT", "quantity": 1}
  ]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.SkippedOverrides != 0 {
		t.Fatalf("stats = %+v, want existing extractor reward enriched in place", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_route12_gift.json")); !os.IsNotExist(err) {
		t.Fatalf("generated duplicate exists or stat failed: %v", err)
	}

	raw, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	var event scriptedevents.EventFile
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[3], "playSFX", "sfxConstant", "SFX_GET_ITEM_1")
	assertMappedAction(t, actions[4], "unlockInput", "type", "unlockInput")
	assertMappedAction(t, actions[5], "giveItem", "itemName", "TM_SWIFT")
}

func TestRunAllowsConditionedExtractorTriggerBranch(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "FightingDojo",
		ScriptLabel: "FightingDojoHitmonleeAlreadyGot",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_FIGHTINGDOJO_HITMONLEE_POKE_BALL",
		},
		Conditions: candidateCondition{
			RequiresEvent: "EVENT_GOT_FIGHTING_DOJO_POKEMON",
		},
		Actions:    []candidateAction{{Type: "dialogue", Lines: []string{"Better not get greedy..."}}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "fighting_dojo_hitmonlee_choice.json")
	existing := []byte(`{
  "scriptLabel": "FightingDojoHitmonleeChoice",
  "mapName": "FIGHTING_DOJO",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_FIGHTINGDOJO_HITMONLEE_POKE_BALL"
  },
  "requiresFlagAbsent": "EVENT_GOT_FIGHTING_DOJO_POKEMON",
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.SkippedOverrides != 0 {
		t.Fatalf("stats = %+v, want conditioned branch written", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "fighting_dojo_hitmonlee_already_got.json")); err != nil {
		t.Fatalf("conditioned branch was not written: %v", err)
	}
}

func TestRunAllowsFacingConditionedExtractorTriggerBranch(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route11Gate2F",
		ScriptLabel: "Route11Gate2FLeftBinocularsSnorlax",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE11GATE2F_LEFT_BINOCULARS",
		},
		Conditions: candidateCondition{
			RequiresPlayerFacing: "up",
		},
		Actions:    []candidateAction{{Type: "dialogue", Lines: []string{"A big POKEMON is asleep on a road!"}}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route11_gate2f_left_binoculars_view.json")
	existing := []byte(`{
  "scriptLabel": "Route11Gate2FLeftBinocularsView",
  "mapName": "ROUTE_11_GATE_2F",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_ROUTE11GATE2F_LEFT_BINOCULARS"
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.SkippedOverrides != 0 {
		t.Fatalf("stats = %+v, want facing-conditioned branch written", stats)
	}

	generatedPath := filepath.Join(outputDir, "route_11_gate_2_f_left_binoculars_snorlax.json")
	raw, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated branch: %v", err)
	}
	var event scriptedevents.EventFile
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("decode generated event: %v", err)
	}
	if event.RequiresPlayerFacing != "UP" {
		t.Fatalf("requiresPlayerFacing = %q, want UP", event.RequiresPlayerFacing)
	}
}

func TestRunSkipsExistingMapFlagOwner(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "MrFujisHouse",
		ScriptLabel: "GeneratedMrFujiPokeFlute",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_MRFUJISHOUSE_MR_FUJI",
		},
		Actions: []candidateAction{
			{Type: "giveItem", ItemConstant: "POKE_FLUTE"},
			{Type: "setEvent", Event: "EVENT_GOT_POKE_FLUTE"},
		},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "mr_fujis_house_poke_flute.json")
	existing := []byte(`{
  "scriptLabel": "MrFujisHousePokeFlute",
  "mapName": "MR_FUJIS_HOUSE",
  "trigger": {
    "type": "map_script",
    "source": "extractor"
  },
  "setsFlags": ["EVENT_GOT_POKE_FLUTE"],
  "actions": [{"type": "setFlag", "flag": "EVENT_GOT_POKE_FLUTE"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want existing map flag owner skipped", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_mr_fuji_poke_flute.json")); !os.IsNotExist(err) {
		t.Fatalf("generated map-flag duplicate exists or stat failed: %v", err)
	}
}

func TestRunSkipsExistingMapFlagOwnerForBattleWinFlag(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route24",
		ScriptLabel: "GeneratedRoute24RocketBattle",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE24_COOLTRAINER_M1",
		},
		Actions: []candidateAction{
			{
				Type:              "startTrainerBattle",
				TrainerClass:      "ROCKET",
				TrainerPartyIndex: 6,
				WinFlag:           "EVENT_BEAT_ROUTE24_ROCKET",
			},
		},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route24_nugget_bridge_rocket.json")
	existing := []byte(`{
  "scriptLabel": "Route24NuggetBridgeRocket",
  "mapName": "ROUTE_24",
  "trigger": {
    "type": "coord",
    "source": "capturequest",
    "label": "Route24NuggetBridgeRocketCoords",
    "coordinates": [{"mapName": "Route24", "mapId": 9999, "x": 190, "y": -219}]
  },
  "setsFlags": ["EVENT_BEAT_ROUTE24_ROCKET"],
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want existing battle win-flag owner skipped", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_route24_rocket_battle.json")); !os.IsNotExist(err) {
		t.Fatalf("generated battle duplicate exists or stat failed: %v", err)
	}
}

func TestRunUpdatesExistingExtractorLabelInPlace(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route2Gate",
		ScriptLabel: "Route2GateOaksAideHM05Reward",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE2GATE_OAKS_AIDE",
		},
		Conditions: candidateCondition{
			RequiresEventAbsent:   "EVENT_GOT_HM05",
			RequiresPokedexCaught: 10,
		},
		Actions:    []candidateAction{{Type: "giveItem", ItemConstant: "HM_FLASH"}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route2_gate_oaks_aide_hm05_reward.json")
	existing := []byte(`{
  "scriptLabel": "Route2GateOaksAideHM05Reward",
  "mapName": "ROUTE_2_GATE",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_ROUTE2GATE_OAKS_AIDE"
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.SkippedOverrides != 0 {
		t.Fatalf("stats = %+v, want one in-place write", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "route_2_gate_oaks_aide_hm_05_reward.json")); !os.IsNotExist(err) {
		t.Fatalf("canonical duplicate exists or stat failed: %v", err)
	}
	raw, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	var event scriptedevents.EventFile
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	if event.RequiresPokedexCaught == nil || *event.RequiresPokedexCaught != 10 {
		t.Fatalf("requiresPokedexCaught = %#v, want 10", event.RequiresPokedexCaught)
	}
}

func TestMapCandidateSupportsItemNameConditions(t *testing.T) {
	event, err := mapCandidate(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "VermilionOldRodHouse",
		ScriptLabel: "VermilionOldRodHouseFishingGuruGift",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU",
		},
		Conditions: candidateCondition{
			RequiresItem:       "OLD_ROD",
			RequiresItemAbsent: "GOOD_ROD",
		},
		Actions: []candidateAction{{Type: "dialogue", Lines: []string{"How are the fish biting?"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresItemName != "OLD_ROD" {
		t.Fatalf("requiresItemName = %q, want OLD_ROD", event.RequiresItemName)
	}
	if event.RequiresItemAbsentName != "GOOD_ROD" {
		t.Fatalf("requiresItemAbsentName = %q, want GOOD_ROD", event.RequiresItemAbsentName)
	}
}

func TestMapCandidateNormalizesOverworldCoordinates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE maps (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			tileset_id INTEGER NOT NULL,
			is_overworld INTEGER NOT NULL
		);
		CREATE TABLE overworld_map_positions (
			id INTEGER PRIMARY KEY,
			map_name TEXT NOT NULL,
			x_offset INTEGER NOT NULL,
			y_offset INTEGER NOT NULL
		);
		INSERT INTO maps (id, name, tileset_id, is_overworld) VALUES (5, 'VERMILION_CITY', 0, 1);
		INSERT INTO overworld_map_positions (id, map_name, x_offset, y_offset) VALUES (5, 'VERMILION_CITY', 170, -54);
	`); err != nil {
		t.Fatal(err)
	}

	resolver, err := newCoordinateResolver(db)
	if err != nil {
		t.Fatal(err)
	}
	event, err := mapCandidateWithResolver(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "VermilionCity",
		ScriptLabel: "VermilionCitySSAnneGuardNoTicketBlocked",
		Trigger: candidateTrigger{
			Type:        "coord",
			Label:       "SSAnneTicketCheckCoords",
			Coordinates: []scriptedevents.EventCoordinate{{MapName: "VermilionCity", X: 18, Y: 30}},
		},
		Actions: []candidateAction{{Type: "dialogue", Lines: []string{"You need a ticket."}}},
	}, resolver)
	if err != nil {
		t.Fatal(err)
	}

	if len(event.Trigger.Coordinates) != 1 {
		t.Fatalf("coordinates = %#v, want one coordinate", event.Trigger.Coordinates)
	}
	got := event.Trigger.Coordinates[0]
	if got.MapName != "VERMILION_CITY" || got.MapID != 9999 || got.X != 188 || got.Y != -24 {
		t.Fatalf("coordinate = %#v, want VERMILION_CITY map 9999 at (188,-24)", got)
	}
}

func TestMapCandidateMapsKnownEventAliases(t *testing.T) {
	event, err := mapCandidate(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "MtMoonB2F",
		ScriptLabel: "MtMoonB2FDomeFossilChoice",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_MTMOONB2F_DOME_FOSSIL",
		},
		Conditions: candidateCondition{
			RequiresEvent: "EVENT_BEAT_MT_MOON_EXIT_SUPER_NERD",
		},
		Actions: []candidateAction{
			{Type: "setEvent", Event: "EVENT_BEAT_MT_MOON_EXIT_SUPER_NERD"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresFlag != "EVENT_BEAT_MT_MOON_SUPER_NERD" {
		t.Fatalf("requiresFlag = %q, want runtime alias", event.RequiresFlag)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[0], "setFlag", "flag", "EVENT_BEAT_MT_MOON_SUPER_NERD")
}

func TestMapCandidateSupportsBadgeConditionsAndRoute23Aliases(t *testing.T) {
	event, err := mapCandidate(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route23",
		ScriptLabel: "Route23BadgeCheck2Pass",
		Trigger: candidateTrigger{
			Type:  "coord",
			Label: "Route23BadgeCheckCascadeCoords",
			Coordinates: []scriptedevents.EventCoordinate{
				{MapName: "Route23", X: 8, Y: 136},
			},
		},
		Conditions: candidateCondition{
			RequiresBadge:       "CASCADEBADGE",
			RequiresEventAbsent: "EVENT_PASSED_CASCADEBADGE_CHECK",
		},
		Actions: []candidateAction{
			{Type: "setEvent", Event: "EVENT_PASSED_CASCADEBADGE_CHECK"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresFlag != "EVENT_GOT_CASCADEBADGE" {
		t.Fatalf("requiresFlag = %q, want badge flag", event.RequiresFlag)
	}
	if event.RequiresFlagAbsent != "EVENT_ROUTE23_BADGE2_CHECKED" {
		t.Fatalf("requiresFlagAbsent = %q, want route23 alias", event.RequiresFlagAbsent)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[0], "setFlag", "flag", "EVENT_ROUTE23_BADGE2_CHECKED")
}

func TestMapCandidatePreservesChoiceTextConstant(t *testing.T) {
	event, err := mapCandidate(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "MtMoonB2F",
		ScriptLabel: "MtMoonB2FHelixFossilChoice",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_MTMOONB2F_HELIX_FOSSIL",
		},
		Actions: []candidateAction{
			{
				Type:         "choice",
				Prompt:       "You want the HELIX FOSSIL?",
				TextConstant: "TEXT_MTMOONB2F_HELIX_FOSSIL",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[0], "choice", "textConstant", "TEXT_MTMOONB2F_HELIX_FOSSIL")
	assertMappedAction(t, actions[0], "choice", "prompt", "You want the HELIX FOSSIL?")
}

func TestRunWritesDiagnosticsReport(t *testing.T) {
	candidate := safariCandidate("SafariZoneGateEntryOffer", "EVENT_IN_SAFARI_ZONE", "")
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	insertExtractorDiagnostic(t, dbPath, extractorDiagnostic{
		MapName:     "BikeShop",
		ScriptLabel: "BikeShopClerkText",
		Status:      "unsupported",
		Reason:      "item_reward,event_flags",
		Details:     json.RawMessage(`{"features":{"hasGiveItem":true}}`),
	})
	insertExtractorDiagnostic(t, dbPath, extractorDiagnostic{
		MapName:     "Museum1F",
		ScriptLabel: "Museum1FScript",
		Status:      "ambiguous",
		Reason:      "choice,item_reward,event_flags",
		Details:     json.RawMessage(`{"features":{"hasChoice":true}}`),
	})
	outputDir := t.TempDir()
	diagnosticsPath := filepath.Join(t.TempDir(), "diagnostics.json")

	stats, err := run(importOptions{SQLitePath: dbPath, OutputDir: outputDir, DiagnosticsPath: diagnosticsPath})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.ExtractorUnsupported != 1 || stats.ExtractorAmbiguous != 1 {
		t.Fatalf("stats = %+v, want written=1 unsupported=1 ambiguous=1", stats)
	}

	raw, err := os.ReadFile(diagnosticsPath)
	if err != nil {
		t.Fatal(err)
	}
	var report importReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.Stats.ExtractorUnsupported != 1 || report.Stats.ExtractorAmbiguous != 1 {
		t.Fatalf("report stats = %+v", report.Stats)
	}
	if report.Summary.DecisionsByStatus["generated"] != 1 {
		t.Fatalf("decision summary = %+v, want one generated decision", report.Summary.DecisionsByStatus)
	}
	if report.Summary.ExtractorByStatus["unsupported"] != 1 || report.Summary.ExtractorByStatus["ambiguous"] != 1 {
		t.Fatalf("extractor status summary = %+v", report.Summary.ExtractorByStatus)
	}
	if report.Summary.ExtractorByReason["item_reward,event_flags"] != 1 {
		t.Fatalf("extractor reason summary = %+v, want item_reward,event_flags", report.Summary.ExtractorByReason)
	}
	if report.Summary.UnsupportedByReason["item_reward,event_flags"] != 1 {
		t.Fatalf("unsupported reason summary = %+v, want item_reward,event_flags", report.Summary.UnsupportedByReason)
	}
	if len(report.Decisions) != 1 || report.Decisions[0].Status != "generated" {
		t.Fatalf("decisions = %+v, want one generated decision", report.Decisions)
	}
	if len(report.ExtractorDiagnostics) != 2 {
		t.Fatalf("extractor diagnostics len = %d, want 2", len(report.ExtractorDiagnostics))
	}
}

func TestMapCandidateRejectsUnsupportedAction(t *testing.T) {
	candidate := safariCandidate("UnsupportedScript", "EVENT_TEST", "")
	candidate.Actions = append(candidate.Actions, candidateAction{Type: "unknownAction"})
	if _, err := mapCandidate(candidate); err == nil {
		t.Fatal("mapCandidate returned nil error for unsupported action")
	}
}

func TestMapCandidateSupportsNeutralActionFamilies(t *testing.T) {
	candidate := safariCandidate("BroadNeutralScript", "EVENT_TEST", "")
	candidate.Actions = []candidateAction{
		{Type: "setEvent", Event: "EVENT_GOT_TEST_ITEM"},
		{Type: "resetEvent", Event: "EVENT_TEST_RESET"},
		{Type: "giveItem", ItemConstant: "POTION", Quantity: 2},
		{Type: "takeItem", ItemID: 42},
		{Type: "givePokemon", SpeciesID: 25, Level: 12, Message: "Received PIKACHU!"},
		{Type: "hideObject", ObjectKey: "TestMap_NPC_1"},
		{Type: "showObject", TriggerLabel: "TestTrigger"},
		{Type: "movePlayer", Movements: []string{"UP", "LEFT"}},
		{Type: "startWildBattle", PokemonID: 150, Level: 70, WinFlag: "EVENT_BEAT_MEWTWO", AllowedActions: []string{"item"}, GuaranteedCatch: true},
		{
			Type:              "startTrainerBattle",
			TrainerClass:      "RIVAL1",
			TrainerPartyIndex: 1,
			WinFlag:           "EVENT_BEAT_RIVAL",
			PostWinActions: []candidateAction{
				{Type: "setEvent", Event: "EVENT_RIVAL_LEFT"},
			},
		},
		{Type: "healParty"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	actions := decodeMappedActions(t, event.Actions)
	if len(actions) != len(candidate.Actions) {
		t.Fatalf("mapped %d actions, want %d: %#v", len(actions), len(candidate.Actions), actions)
	}
	assertMappedAction(t, actions[0], "setFlag", "flag", "EVENT_GOT_TEST_ITEM")
	assertMappedAction(t, actions[1], "resetFlag", "flag", "EVENT_TEST_RESET")
	assertMappedAction(t, actions[2], "giveItem", "itemName", "POTION")
	assertMappedNumber(t, actions[2], "quantity", 2)
	assertMappedAction(t, actions[3], "takeItem", "type", "takeItem")
	assertMappedNumber(t, actions[3], "itemId", 42)
	assertMappedAction(t, actions[4], "givePokemon", "message", "Received PIKACHU!")
	assertMappedNumber(t, actions[4], "pokemonId", 25)
	assertMappedAction(t, actions[5], "hideObject", "objectKey", "TestMap_NPC_1")
	assertMappedAction(t, actions[6], "showObject", "triggerLabel", "TestTrigger")
	assertMappedAction(t, actions[7], "movePlayer", "type", "movePlayer")
	assertMappedAction(t, actions[8], "startWildBattle", "winFlag", "EVENT_BEAT_MEWTWO")
	assertMappedBool(t, actions[8], "guaranteedCatch", true)
	assertMappedStringList(t, actions[8], "allowedActions", []string{"item"})
	assertMappedAction(t, actions[9], "startTrainerBattle", "trainerClass", "RIVAL1")
	assertMappedAction(t, actions[10], "healParty", "type", "healParty")

	postWin, ok := actions[9]["postWinActions"].([]any)
	if !ok || len(postWin) != 1 {
		t.Fatalf("postWinActions = %#v, want one nested action", actions[9]["postWinActions"])
	}
	nested, ok := postWin[0].(map[string]any)
	if !ok {
		t.Fatalf("nested post-win action = %#v, want object", postWin[0])
	}
	assertMappedAction(t, nested, "setFlag", "flag", "EVENT_RIVAL_LEFT")
}

func TestMapCandidateSupportsPokemonNameActions(t *testing.T) {
	candidate := safariCandidate("PokemonNameScript", "EVENT_TEST", "")
	candidate.Actions = []candidateAction{
		{Type: "givePokemon", PokemonConstant: "LAPRAS", Level: 15},
		{Type: "startWildBattle", PokemonName: "SNORLAX", Level: 30, WinFlag: "EVENT_BEAT_SNORLAX"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	actions := decodeMappedActions(t, event.Actions)
	if len(actions) != 2 {
		t.Fatalf("mapped %d actions, want 2: %#v", len(actions), actions)
	}
	assertMappedAction(t, actions[0], "givePokemon", "pokemonName", "LAPRAS")
	assertMappedNumber(t, actions[0], "level", 15)
	assertMappedAction(t, actions[1], "startWildBattle", "pokemonName", "SNORLAX")
	assertMappedNumber(t, actions[1], "level", 30)
	assertMappedAction(t, actions[1], "startWildBattle", "winFlag", "EVENT_BEAT_SNORLAX")
}

func TestMapCandidateSupportsPokedexCaughtCondition(t *testing.T) {
	candidate := safariCandidate("Route2GateOaksAideHM05Reward", "EVENT_GOT_HM05", "")
	candidate.MapName = "Route2Gate"
	candidate.Trigger = candidateTrigger{
		Type:  "npc_click",
		Label: "TEXT_ROUTE2GATE_OAKS_AIDE",
	}
	candidate.Conditions.RequiresPokedexCaught = 10
	candidate.Actions = []candidateAction{{Type: "giveItem", ItemConstant: "HM_FLASH"}}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresPokedexCaught == nil || *event.RequiresPokedexCaught != 10 {
		t.Fatalf("requiresPokedexCaught = %#v, want 10", event.RequiresPokedexCaught)
	}
}

func TestMapCandidateSupportsCoinConditionsAndActions(t *testing.T) {
	candidate := safariCandidate("GameCornerClerk2CoinsGift", "", "EVENT_GOT_20_COINS_2")
	candidate.MapName = "GameCorner"
	candidate.Trigger = candidateTrigger{
		Type:  "npc_click",
		Label: "TEXT_GAMECORNER_CLERK2",
	}
	candidate.Conditions.RequiresItem = "COIN_CASE"
	candidate.Conditions.RequiresCoinsBelow = 9990
	candidate.Actions = []candidateAction{
		{Type: "giveCoins", Coins: 20},
		{Type: "setEvent", Event: "EVENT_GOT_20_COINS_2"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresItemName != "COIN_CASE" {
		t.Fatalf("requiresItemName = %q, want COIN_CASE", event.RequiresItemName)
	}
	if event.RequiresCoinsBelow == nil || *event.RequiresCoinsBelow != 9990 {
		t.Fatalf("requiresCoinsBelow = %#v, want 9990", event.RequiresCoinsBelow)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[0], "giveCoins", "type", "giveCoins")
	assertMappedNumber(t, actions[0], "coins", 20)
	assertMappedAction(t, actions[1], "setFlag", "flag", "EVENT_GOT_20_COINS_2")
}

func insertExtractorDiagnostic(t *testing.T, dbPath string, diagnostic extractorDiagnostic) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS script_event_candidate_diagnostics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			map_name TEXT NOT NULL,
			script_label TEXT NOT NULL,
			status TEXT NOT NULL,
			reason TEXT NOT NULL,
			details_json TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	details := string(diagnostic.Details)
	if details == "" {
		details = "{}"
	}
	if _, err := db.Exec(
		`INSERT INTO script_event_candidate_diagnostics
			(map_name, script_label, status, reason, details_json)
		 VALUES (?, ?, ?, ?, ?)`,
		diagnostic.MapName,
		diagnostic.ScriptLabel,
		diagnostic.Status,
		diagnostic.Reason,
		details,
	); err != nil {
		t.Fatal(err)
	}
}

func decodeMappedActions(t *testing.T, rawActions []json.RawMessage) []map[string]any {
	t.Helper()
	actions := make([]map[string]any, 0, len(rawActions))
	for i, raw := range rawActions {
		var action map[string]any
		if err := json.Unmarshal(raw, &action); err != nil {
			t.Fatalf("decode action %d: %v", i, err)
		}
		actions = append(actions, action)
	}
	return actions
}

func assertMappedAction(t *testing.T, action map[string]any, wantType, key, want string) {
	t.Helper()
	if got, ok := action["type"].(string); !ok || got != wantType {
		t.Fatalf("action type = %#v, want %q in %#v", action["type"], wantType, action)
	}
	if got, ok := action[key].(string); !ok || got != want {
		t.Fatalf("action[%s] = %#v, want %q in %#v", key, action[key], want, action)
	}
}

func assertMappedNumber(t *testing.T, action map[string]any, key string, want float64) {
	t.Helper()
	if got, ok := action[key].(float64); !ok || got != want {
		t.Fatalf("action[%s] = %#v, want %.0f in %#v", key, action[key], want, action)
	}
}

func assertMappedBool(t *testing.T, action map[string]any, key string, want bool) {
	t.Helper()
	if got, ok := action[key].(bool); !ok || got != want {
		t.Fatalf("action[%s] = %#v, want %t in %#v", key, action[key], want, action)
	}
}

func assertMappedStringList(t *testing.T, action map[string]any, key string, want []string) {
	t.Helper()
	values, ok := action[key].([]any)
	if !ok || len(values) != len(want) {
		t.Fatalf("action[%s] = %#v, want %#v in %#v", key, action[key], want, action)
	}
	for i, expected := range want {
		if got, ok := values[i].(string); !ok || got != expected {
			t.Fatalf("action[%s][%d] = %#v, want %q in %#v", key, i, values[i], expected, action)
		}
	}
}

func TestNameConversions(t *testing.T) {
	if got := camelToUpperSnake("SafariZoneGate"); got != "SAFARI_ZONE_GATE" {
		t.Fatalf("camelToUpperSnake SafariZoneGate = %q", got)
	}
	if got := mapNameToUpperSnake("Route12Gate2F"); got != "ROUTE_12_GATE_2F" {
		t.Fatalf("mapNameToUpperSnake Route12Gate2F = %q", got)
	}
	if got := mapNameToUpperSnake("SilphCo11F"); got != "SILPH_CO_11F" {
		t.Fatalf("mapNameToUpperSnake SilphCo11F = %q", got)
	}
	if got := mapNameToUpperSnake("CeruleanCaveB1F"); got != "CERULEAN_CAVE_B1F" {
		t.Fatalf("mapNameToUpperSnake CeruleanCaveB1F = %q", got)
	}
	if got := camelToSnake("PokemonTower2FRivalEncounter"); got != "pokemon_tower_2_f_rival_encounter" {
		t.Fatalf("camelToSnake PokemonTower2FRivalEncounter = %q", got)
	}
}

func createSQLite(t *testing.T, withCandidates bool, candidates []scriptCandidate) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if !withCandidates {
		if _, err := db.Exec(`CREATE TABLE placeholder (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatal(err)
		}
		return path
	}
	if _, err := db.Exec(`
		CREATE TABLE script_event_candidates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			map_name TEXT NOT NULL,
			script_label TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			trigger_label TEXT NOT NULL,
			confidence TEXT NOT NULL,
			candidate_json TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		raw, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(
			`INSERT INTO script_event_candidates
				(map_name, script_label, trigger_type, trigger_label, confidence, candidate_json)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			candidate.MapName,
			candidate.ScriptLabel,
			candidate.Trigger.Type,
			candidate.Trigger.Label,
			candidate.Confidence,
			string(raw),
		); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func createTileOverrideSQLite(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE script_event_tile_overrides (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			map_name TEXT NOT NULL,
			script_label TEXT NOT NULL,
			candidate_json TEXT NOT NULL
		);
		CREATE TABLE maps (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			tileset_id INTEGER NOT NULL
		);
		CREATE TABLE blocksets (
			tileset_id INTEGER NOT NULL,
			block_index INTEGER NOT NULL,
			block_data BLOB NOT NULL
		);
		CREATE TABLE tileset_tiles (
			tileset_id INTEGER NOT NULL,
			tile_index INTEGER NOT NULL,
			tile_data BLOB NOT NULL
		);
		CREATE TABLE tile_images (
			id INTEGER PRIMARY KEY,
			tileset_id INTEGER NOT NULL,
			block_index INTEGER NOT NULL,
			position INTEGER NOT NULL
		);
		CREATE TABLE collision_tiles (
			tileset_id INTEGER NOT NULL,
			tile_id INTEGER NOT NULL
		);
		INSERT INTO maps (id, name, tileset_id) VALUES (7, 'TEST_MAP', 3);
		INSERT INTO tile_images (id, tileset_id, block_index, position) VALUES (42, 3, 1, 0);
		INSERT INTO collision_tiles (tileset_id, tile_id) VALUES (3, 1);
	`); err != nil {
		t.Fatal(err)
	}

	blockData := []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	tileData := make([]byte, 16)
	if _, err := db.Exec(`INSERT INTO blocksets (tileset_id, block_index, block_data) VALUES (3, 1, ?)`, blockData); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO blocksets (tileset_id, block_index, block_data) VALUES (3, 9, ?)`, blockData); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO tileset_tiles (tileset_id, tile_index, tile_data) VALUES (3, 1, ?)`, tileData); err != nil {
		t.Fatal(err)
	}

	candidate := tileOverrideCandidate{
		Version:     1,
		Kind:        "eventTileOverrideCandidate",
		MapName:     "TestMap",
		ScriptLabel: "TestGateTiles",
		Replacements: []tileOverrideReplacement{
			{
				BlockX:        2,
				BlockY:        3,
				BlockID:       9,
				RequiresEvent: "EVENT_OPEN_TEST_GATE",
				LabelPrefix:   "TestGateOpen",
			},
		},
		Confidence: "adapter",
	}
	raw, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO script_event_tile_overrides (map_name, script_label, candidate_json) VALUES (?, ?, ?)`,
		candidate.MapName,
		candidate.ScriptLabel,
		string(raw),
	); err != nil {
		t.Fatal(err)
	}
	return path
}

func createConditionalDialogueSQLite(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE script_event_conditional_dialogue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			text_constant TEXT NOT NULL,
			map_name TEXT NOT NULL,
			script_label TEXT NOT NULL,
			priority INTEGER NOT NULL,
			requires_flags_json TEXT NOT NULL,
			requires_flags_absent_json TEXT NOT NULL,
			dialogue_labels_json TEXT NOT NULL,
			source_json TEXT NOT NULL,
			row_json TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	candidate := conditionalDialogueCandidate{
		Version:      1,
		Kind:         "conditionalDialogue",
		MapName:      "OaksLab",
		ScriptLabel:  "OaksLabRivalTextConditionalDialogue300",
		TextConstant: "TEXT_OAKSLAB_RIVAL",
		Priority:     300,
		Conditions: candidateCondition{
			RequiresEventsAbsent: []string{"EVENT_FOLLOWED_OAK_INTO_LAB_2"},
		},
		DialogueLabels: []string{"_OaksLabRivalGrampsIsntAroundText"},
		Source: map[string]any{
			"adapter": "text_asm_nested_event_dialogue_v1",
		},
		Confidence: "adapter",
	}
	raw, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO script_event_conditional_dialogue (
			text_constant, map_name, script_label, priority,
			requires_flags_json, requires_flags_absent_json,
			dialogue_labels_json, source_json, row_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		candidate.TextConstant,
		candidate.MapName,
		candidate.ScriptLabel,
		candidate.Priority,
		`[]`,
		`["EVENT_FOLLOWED_OAK_INTO_LAB_2"]`,
		`["_OaksLabRivalGrampsIsntAroundText"]`,
		`{"adapter":"text_asm_nested_event_dialogue_v1"}`,
		string(raw),
	); err != nil {
		t.Fatal(err)
	}
	return path
}

func createObjectVisibilitySQLite(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE script_event_object_visibility (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			map_name TEXT NOT NULL,
			map_id INTEGER NOT NULL,
			object_name TEXT NOT NULL,
			object_key TEXT NOT NULL,
			script_label TEXT NOT NULL,
			requires_event TEXT NOT NULL,
			visible INTEGER NOT NULL,
			label TEXT NOT NULL,
			rule_json TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	candidate := objectVisibilityCandidate{
		Version:       1,
		Kind:          "objectVisibility",
		MapName:       "VIRIDIAN_CITY",
		MapID:         1,
		ObjectName:    "ViridianCity_NPC_5",
		ObjectKey:     "HS_LYING_OLD_MAN",
		Visible:       false,
		RequiresEvent: "EVENT_GOT_POKEDEX",
		Label:         "OaksLabOakGivesPokedexScript:EVENT_GOT_POKEDEX:HS_LYING_OLD_MAN:HideObject",
		SourceMapName: "OaksLab",
		ScriptLabel:   "OaksLabOakGivesPokedexScript",
		Source: map[string]any{
			"adapter": "flagged_missable_object_visibility_v1",
		},
		Confidence: "adapter",
	}
	raw, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO script_event_object_visibility (
			map_name, map_id, object_name, object_key, script_label,
			requires_event, visible, label, rule_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		candidate.MapName,
		candidate.MapID,
		candidate.ObjectName,
		candidate.ObjectKey,
		candidate.ScriptLabel,
		candidate.RequiresEvent,
		0,
		candidate.Label,
		string(raw),
	); err != nil {
		t.Fatal(err)
	}
	return path
}

func containsImportedEventTileRule(rules []scriptedevents.EventTileOverrideRule, want scriptedevents.EventTileOverrideRule) bool {
	for _, rule := range rules {
		if rule == want {
			return true
		}
	}
	return false
}

func safariCandidate(label, requiresAbsent, requires string) scriptCandidate {
	return scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "SafariZoneGate",
		ScriptLabel: label,
		Trigger: candidateTrigger{
			Type:  "coord",
			Label: label,
		},
		Conditions: candidateCondition{
			RequiresEventAbsent: requiresAbsent,
			RequiresEvent:       requires,
		},
		Actions: []candidateAction{
			{Type: "lockInput"},
			{Type: "dialogue", Speaker: "SAFARI ZONE WORKER", Lines: []string{"Welcome to the SAFARI ZONE!"}},
			{
				Type:        "choice",
				Speaker:     "SAFARI ZONE WORKER",
				PromptLines: []string{"For just 500 Pokedollars,", "Would you like to", "join the hunt?"},
				NoLines:     []string{"OK! Please come again!"},
			},
			{
				Type: "startSafariSession",
				Destination: &candidateDestination{
					MapName:   "SafariZoneCenter",
					MapID:     220,
					X:         14,
					Y:         25,
					Direction: "UP",
				},
			},
			{Type: "unlockInput"},
		},
		Confidence: "adapter",
	}
}
