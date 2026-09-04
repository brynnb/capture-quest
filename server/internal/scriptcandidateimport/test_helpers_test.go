package scriptcandidateimport

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
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
