package scriptedevents

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestResolveEventItemRequirementsUsesItemNames(t *testing.T) {
	db := newItemRequirementTestDB(t)

	pokeFluteID := 70
	event := EventFile{
		ScriptLabel:            "FishingGuruOldRod",
		RequiresItemName:       "OLD_ROD",
		RequiresItemAbsentName: "GOOD_ROD",
		RequiresItemAbsentID:   &pokeFluteID,
	}

	requires, requiresAbsent, err := resolveEventItemRequirements(db, event)
	if err != nil {
		t.Fatal(err)
	}
	if requires == nil || *requires != 10 {
		t.Fatalf("requiresItemID = %#v, want 10", requires)
	}
	if requiresAbsent == nil || *requiresAbsent != 11 {
		t.Fatalf("requiresItemAbsentID = %#v, want 11 from name override", requiresAbsent)
	}
}

func TestResolveEventItemRequirementsReportsUnknownItem(t *testing.T) {
	db := newItemRequirementTestDB(t)

	_, _, err := resolveEventItemRequirements(db, EventFile{
		ScriptLabel:      "UnknownItemGate",
		RequiresItemName: "NOT_A_REAL_ITEM",
	})
	if err == nil {
		t.Fatal("expected unknown item error")
	}
}

func TestDeleteStaleExtractorTriggerScripts(t *testing.T) {
	db := newCutsceneCleanupTestDB(t)

	deleted, err := deleteStaleExtractorTriggerScripts(db, []EventFile{
		{
			ScriptLabel: "FishingGuruGift",
			MapName:     "VERMILION_OLD_ROD_HOUSE",
			Trigger: EventTrigger{
				Type:   "npc_click",
				Source: "extractor",
				Label:  "TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU",
			},
		},
		{
			ScriptLabel: "FishingGuruAlreadyGot",
			MapName:     "VERMILION_OLD_ROD_HOUSE",
			Trigger: EventTrigger{
				Type:   "npc_click",
				Source: "extractor",
				Label:  "TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU",
			},
		},
		{
			ScriptLabel: "CaptureQuestOverride",
			MapName:     "VERMILION_OLD_ROD_HOUSE",
			Trigger: EventTrigger{
				Type:   "coord",
				Source: "capturequest",
				Label:  "TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	rows, err := db.Query(`SELECT script_label FROM phaser_cutscene_scripts ORDER BY script_label`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			t.Fatal(err)
		}
		labels = append(labels, label)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{"CaptureQuestOverride", "FishingGuruAlreadyGot", "FishingGuruGift", "OtherTrigger"}
	if len(labels) != len(want) {
		t.Fatalf("labels = %#v, want %#v", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("labels = %#v, want %#v", labels, want)
		}
	}
}

func TestLoadEventTileOverridesExpandsBlocks(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, eventTilesFileName), []byte(`{
		"palettes": {
			"gate": [
				{ "dx": 0, "dy": 0, "tileImageId": 10, "collisionType": 1 },
				{ "dx": 1, "dy": 0, "tileImageId": 11, "collisionType": 0 }
			]
		},
		"blocks": [
			{
				"mapId": 7,
				"mapName": "TEST_MAP",
				"blockX": 4,
				"blockY": 5,
				"palette": "gate",
				"requiresFlag": "EVENT_OPEN",
				"labelPrefix": "TestGate"
			}
		],
		"tiles": [
			{
				"mapId": 7,
				"mapName": "TEST_MAP",
				"x": 1,
				"y": 2,
				"tileImageId": 12,
				"collisionType": 1,
				"requiresFlagAbsent": "EVENT_OPEN",
				"label": "DirectTile"
			}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rules, err := LoadEventTileOverrides(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("rules = %#v, want 3", rules)
	}
	if !containsEventTileRule(rules, EventTileOverrideRule{
		MapID:         7,
		MapName:       "TEST_MAP",
		X:             8,
		Y:             10,
		TileImageID:   10,
		CollisionType: 1,
		RequiresFlag:  "EVENT_OPEN",
		Label:         "TestGate_0_0",
	}) {
		t.Fatalf("missing expanded first cell in %#v", rules)
	}
	if !containsEventTileRule(rules, EventTileOverrideRule{
		MapID:         7,
		MapName:       "TEST_MAP",
		X:             9,
		Y:             10,
		TileImageID:   11,
		CollisionType: 0,
		RequiresFlag:  "EVENT_OPEN",
		Label:         "TestGate_1_0",
	}) {
		t.Fatalf("missing expanded second cell in %#v", rules)
	}
}

func TestLoadEventTileOverridesMergesGeneratedFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, eventTilesFileName), []byte(`{
		"tiles": [
			{
				"mapId": 7,
				"mapName": "TEST_MAP",
				"x": 1,
				"y": 2,
				"tileImageId": 12,
				"collisionType": 1,
				"requiresFlagAbsent": "EVENT_OPEN",
				"label": "ManualTile"
			}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, generatedEventTilesFileName), []byte(`{
		"tiles": [
			{
				"mapId": 7,
				"mapName": "TEST_MAP",
				"x": 3,
				"y": 4,
				"tileImageId": 13,
				"collisionType": 0,
				"requiresFlag": "EVENT_OPEN",
				"label": "GeneratedTile"
			}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rules, err := LoadEventTileOverrides(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Fatalf("rules = %#v, want 2", rules)
	}
	if rules[0].Label != "ManualTile" || rules[1].Label != "GeneratedTile" {
		t.Fatalf("rules order = %#v, want manual then generated", rules)
	}
}

func TestLoadConditionalDialogueGeneratedFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, conditionalDialogueFileName), []byte(`{
		"rows": [
			{
				"textConstant": "TEXT_OAKSLAB_RIVAL",
				"priority": 300,
				"requiresFlagsAbsent": ["EVENT_FOLLOWED_OAK_INTO_LAB_2"],
				"dialogueLabels": ["_OaksLabRivalGrampsIsntAroundText"]
			}
		]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}

	rules, err := LoadConditionalDialogue(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("rules = %#v, want 1", rules)
	}
	if rules[0].TextConstant != "TEXT_OAKSLAB_RIVAL" {
		t.Fatalf("textConstant = %q", rules[0].TextConstant)
	}
	if len(rules[0].RequiresFlagsAbsent) != 1 || rules[0].RequiresFlagsAbsent[0] != "EVENT_FOLLOWED_OAK_INTO_LAB_2" {
		t.Fatalf("requiresFlagsAbsent = %#v", rules[0].RequiresFlagsAbsent)
	}
}

func TestBuildConditionalDialogueRowsHydratesDialogueLabels(t *testing.T) {
	db := newConditionalDialogueHydrationTestDB(t)
	rows, err := buildConditionalDialogueRows(db, []ConditionalDialogueRule{
		{
			TextConstant:        "TEXT_OAKSLAB_RIVAL",
			Priority:            200,
			RequiresFlags:       []string{"EVENT_FOLLOWED_OAK_INTO_LAB_2", "EVENT_GOT_STARTER"},
			RequiresFlagsAbsent: []string{},
			DialogueLabels:      []string{"_OaksLabRivalMyPokemonLooksStrongerText"},
		},
		{
			TextConstant:        "TEXT_OAKSLAB_RIVAL",
			Priority:            300,
			RequiresFlagsAbsent: []string{"EVENT_FOLLOWED_OAK_INTO_LAB_2"},
			DialogueLabels:      []string{"_OaksLabRivalGrampsIsntAroundText"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want 2", rows)
	}
	byPriority := map[int]conditionalDialogueDBRow{}
	for _, row := range rows {
		byPriority[row.Priority] = row
	}
	if byPriority[200].OverrideDialogue != "My Pokemon looks stronger." {
		t.Fatalf("overrideDialogue = %q", byPriority[200].OverrideDialogue)
	}
	if byPriority[200].RequiresFlag != "" || byPriority[200].RequiresFlagAbsent != "" {
		t.Fatalf("scalar flags = %q/%q, want generated JSON columns only", byPriority[200].RequiresFlag, byPriority[200].RequiresFlagAbsent)
	}
	if string(byPriority[200].RequiresFlagsJSON) != `["EVENT_FOLLOWED_OAK_INTO_LAB_2","EVENT_GOT_STARTER"]` {
		t.Fatalf("requiresFlagsJSON = %s", byPriority[200].RequiresFlagsJSON)
	}
	if byPriority[300].RequiresFlag != "" || byPriority[300].RequiresFlagAbsent != "" {
		t.Fatalf("single scalar flags = %q/%q, want generated JSON columns only", byPriority[300].RequiresFlag, byPriority[300].RequiresFlagAbsent)
	}
	if string(byPriority[300].RequiresAbsentJSON) != `["EVENT_FOLLOWED_OAK_INTO_LAB_2"]` {
		t.Fatalf("requiresAbsentJSON = %s", byPriority[300].RequiresAbsentJSON)
	}
}

func TestSyncEventTileOverridesReplacesRuntimeRows(t *testing.T) {
	db := newEventTileOverrideTestDB(t)
	rules := []EventTileOverrideRule{
		{
			MapID:              7,
			MapName:            "TEST_MAP",
			X:                  8,
			Y:                  10,
			TileImageID:        10,
			CollisionType:      1,
			RequiresFlagAbsent: "EVENT_OPEN",
			Label:              "TestGateClosed",
		},
		{
			MapID:         7,
			MapName:       "TEST_MAP",
			X:             8,
			Y:             10,
			TileImageID:   11,
			CollisionType: 0,
			RequiresFlag:  "EVENT_OPEN",
			Label:         "TestGateOpen",
		},
	}

	changed, err := syncEventTileOverrides(db, rules)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("first sync changed = false, want true")
	}

	changed, err = syncEventTileOverrides(db, rules)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second sync changed = true, want false")
	}

	rows, err := loadEventTileOverrideRows(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(rules) {
		t.Fatalf("rows = %#v, want %#v", rows, rules)
	}
	for _, rule := range rules {
		if !containsEventTileRule(rows, rule) {
			t.Fatalf("missing synced rule %#v in %#v", rule, rows)
		}
	}
}

func newItemRequirementTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE cq_items (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			short_name TEXT NOT NULL
		);
		INSERT INTO cq_items (id, name, short_name)
		VALUES
			(10, 'Old Rod', 'OLD_ROD'),
			(11, 'Good Rod', 'GOOD_ROD'),
			(70, 'Poke Flute', 'POKE_FLUTE');
	`); err != nil {
		t.Fatal(err)
	}
	return db
}

func newEventTileOverrideTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE phaser_event_tile_overrides (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			map_id INTEGER NOT NULL,
			map_name TEXT NOT NULL,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			tile_image_id INTEGER NOT NULL,
			collision_type INTEGER NOT NULL DEFAULT 0,
			requires_flag TEXT DEFAULT NULL,
			requires_flag_absent TEXT DEFAULT NULL,
			label TEXT DEFAULT NULL
		);
		INSERT INTO phaser_event_tile_overrides
			(map_id, map_name, x, y, tile_image_id, collision_type, label)
		VALUES
			(1, 'STALE_MAP', 1, 1, 1, 0, 'StaleTile');
	`); err != nil {
		t.Fatal(err)
	}
	return db
}

func newConditionalDialogueHydrationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE phaser_dialogue_text (
			label TEXT PRIMARY KEY,
			dialogue TEXT NOT NULL
		);
		INSERT INTO phaser_dialogue_text (label, dialogue)
		VALUES
			('_OaksLabRivalMyPokemonLooksStrongerText', 'My Pokemon looks stronger.'),
			('_OaksLabRivalGrampsIsntAroundText', 'Gramps is not around.');
	`); err != nil {
		t.Fatal(err)
	}
	return db
}

func containsEventTileRule(rules []EventTileOverrideRule, want EventTileOverrideRule) bool {
	for _, rule := range rules {
		if rule == want {
			return true
		}
	}
	return false
}

func newCutsceneCleanupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE phaser_cutscene_scripts (
			script_label TEXT PRIMARY KEY,
			map_name TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			trigger_label TEXT NOT NULL,
			requires_money INTEGER,
			requires_money_below INTEGER
		);
		INSERT INTO phaser_cutscene_scripts (script_label, map_name, trigger_type, trigger_label)
		VALUES
			('FishingGuruGift', 'VERMILION_OLD_ROD_HOUSE', 'npc_click', 'TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU'),
			('FishingGuruAlreadyGot', 'VERMILION_OLD_ROD_HOUSE', 'npc_click', 'TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU'),
			('OldRodSingleBranch', 'VERMILION_OLD_ROD_HOUSE', 'npc_click', 'TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU'),
			('OtherTrigger', 'VERMILION_OLD_ROD_HOUSE', 'npc_click', 'TEXT_OTHER'),
			('CaptureQuestOverride', 'VERMILION_OLD_ROD_HOUSE', 'coord', 'TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU');
	`); err != nil {
		t.Fatal(err)
	}
	return db
}
