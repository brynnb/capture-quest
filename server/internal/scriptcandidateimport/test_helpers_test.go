package scriptcandidateimport

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"capturequest/internal/scriptedevents"
)

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
		installExtractorContractFixture(t, db)
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
	installExtractorContractFixture(t, db)
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
	installExtractorContractFixture(t, db)
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
	installExtractorContractFixture(t, db)
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
	installExtractorContractFixture(t, db)
	return path
}

func installExtractorContractFixture(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_metadata (
			schema_name TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			minimum_reader_version INTEGER NOT NULL,
			applied_epoch INTEGER NOT NULL
		);
		CREATE TABLE IF NOT EXISTS extraction_runs (
			run_id TEXT PRIMARY KEY,
			schema_name TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			extractor_revision TEXT NOT NULL,
			source_revision TEXT NOT NULL,
			source_date_epoch INTEGER NOT NULL,
			source_root TEXT NOT NULL,
			source_tree_sha256 TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS game_releases (
			release_code TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			variant TEXT NOT NULL,
			platform TEXT NOT NULL,
			region TEXT NOT NULL,
			language TEXT NOT NULL,
			build_define TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS extraction_run_releases (
			run_id TEXT NOT NULL,
			release_code TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS moves (id INTEGER PRIMARY KEY, constant_name TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS pokemon (
			id INTEGER PRIMARY KEY,
			default_move_1_id INTEGER, default_move_1_name TEXT NOT NULL DEFAULT 'NO_MOVE',
			default_move_2_id INTEGER, default_move_2_name TEXT NOT NULL DEFAULT 'NO_MOVE',
			default_move_3_id INTEGER, default_move_3_name TEXT NOT NULL DEFAULT 'NO_MOVE',
			default_move_4_id INTEGER, default_move_4_name TEXT NOT NULL DEFAULT 'NO_MOVE'
		);
		CREATE TABLE IF NOT EXISTS tilesets (id INTEGER PRIMARY KEY, grass_tile_id INTEGER);
		CREATE TABLE IF NOT EXISTS tiles (
			id INTEGER PRIMARY KEY,
			raw_foot_tile_id INTEGER,
			raw_encounter_tile_id INTEGER
		);
	`); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{
		"audio_assets", "dialogue_text", "encounter_slots", "event_flags",
		"hidden_coins", "hidden_items", "hidden_objects", "items", "map_music",
		"map_scripts", "npc_movement_data", "objects", "pokemon_default_moves",
		"pokemon_learnset", "pokemon_tmhm", "text_pointers", "trainer_classes",
		"trainer_headers", "trainer_parties", "trainer_party_pokemon", "warps",
		"warp_events", "wild_encounters",
	} {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS ` + table + ` (id INTEGER PRIMARY KEY)`); err != nil {
			t.Fatalf("create extractor contract table %s: %v", table, err)
		}
	}
	// Some focused fixtures own these tables with richer schemas.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS maps (
		id INTEGER PRIMARY KEY, name TEXT NOT NULL, tileset_id INTEGER, is_overworld INTEGER NOT NULL DEFAULT 0
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS tile_images (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO schema_metadata VALUES ('pokemon-gameboy-extractor', 2, 2, 1);
		INSERT INTO extraction_runs VALUES ('test-run', 'pokemon-gameboy-extractor', 2, 'extractor-rev', 'source-rev', 1, '/source', 'sha256');
		INSERT INTO game_releases VALUES ('red', 'Pokemon Red', 'red', 'gameboy', 'US', 'en', '_RED');
		INSERT INTO game_releases VALUES ('blue', 'Pokemon Blue', 'blue', 'gameboy', 'US', 'en', '_BLUE');
		INSERT INTO extraction_run_releases VALUES ('test-run', 'red');
		INSERT INTO extraction_run_releases VALUES ('test-run', 'blue');
	`); err != nil {
		t.Fatal(err)
	}
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
