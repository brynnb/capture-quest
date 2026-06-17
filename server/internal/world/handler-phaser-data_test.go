package world

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestBranchingDialogueForResponseSuppressesLegacyBranchForScriptOwnedClick(t *testing.T) {
	setupBranchingDialogueTestDB(t, "TEXT_CELADONMARTROOF_LITTLE_GIRL")

	cutscenes := NewCutsceneManager(nil)
	cutscenes.mu.Lock()
	cutscenes.byTriggerLabel["TEXT_CELADONMARTROOF_LITTLE_GIRL"] = []*CutsceneScript{{
		ScriptLabel:  "CeladonMartRoofTM13IceBeam",
		TriggerType:  "npc_click",
		TriggerLabel: stringPtr("TEXT_CELADONMARTROOF_LITTLE_GIRL"),
	}}
	cutscenes.mu.Unlock()

	wh := &WorldHandler{
		Cutscenes:  cutscenes,
		EventFlags: &EventFlagManager{},
	}
	if got := branchingDialogueForResponse("TEXT_CELADONMARTROOF_LITTLE_GIRL", 42, wh); got != nil {
		t.Fatalf("branching dialogue = %#v, want nil for script-owned text constant", got)
	}
}

func TestBranchingDialogueForResponseAllowsLegacyBranchWithoutScriptOwner(t *testing.T) {
	setupBranchingDialogueTestDB(t, "TEXT_POKEMONMANSION1F_SWITCH")

	wh := &WorldHandler{
		Cutscenes:  NewCutsceneManager(nil),
		EventFlags: &EventFlagManager{},
	}
	got := branchingDialogueForResponse("TEXT_POKEMONMANSION1F_SWITCH", 42, wh)
	if got == nil {
		t.Fatal("branching dialogue = nil, want legacy branch")
	}
	if got.PromptText != "A secret switch! Press it?" {
		t.Fatalf("prompt = %q", got.PromptText)
	}
}

func TestResolvePhaserDialogueEntriesAppliesGeneratedConditionalDialogue(t *testing.T) {
	setupGeneratedConditionalDialogueResolverTestDB(t)

	efm := NewEventFlagManager(nil)
	const charID int64 = 42
	efm.flags[charID] = map[string]bool{}

	entries, err := resolvePhaserDialogueEntries("TEXT_OAKSLAB_RIVAL", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %#v, want one generated conditional entry", entries)
	}
	if entries[0].Dialogue != "Gramps isn't around!" {
		t.Fatalf("dialogue = %q, want Gramps branch", entries[0].Dialogue)
	}

	efm.flags[charID]["EVENT_FOLLOWED_OAK_INTO_LAB_2"] = true
	entries, err = resolvePhaserDialogueEntries("TEXT_OAKSLAB_RIVAL", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Dialogue != "Go ahead and choose!" {
		t.Fatalf("followed entries = %#v, want choose branch", entries)
	}

	efm.flags[charID]["EVENT_GOT_STARTER"] = true
	entries, err = resolvePhaserDialogueEntries("TEXT_OAKSLAB_RIVAL", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Dialogue != "My Pokemon looks stronger." {
		t.Fatalf("starter entries = %#v, want stronger branch", entries)
	}
}

func setupBranchingDialogueTestDB(t *testing.T, textConstant string) {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_branching_dialogue (
			id INTEGER PRIMARY KEY,
			map_name TEXT,
			prompt_text_constant TEXT NOT NULL,
			prompt_text TEXT NOT NULL,
			yes_text_constant TEXT,
			no_text_constant TEXT,
			yes_dialogue TEXT,
			no_dialogue TEXT,
			requires_event_flag TEXT,
			sets_event_flag TEXT,
			yes_actions TEXT,
			no_actions TEXT
		);
		CREATE TABLE phaser_in_game_trades (
			trade_key TEXT,
			text_constant TEXT,
			map_name TEXT,
			source_file TEXT,
			script_label TEXT,
			requested_pokemon_id INTEGER,
			requested_pokemon_name TEXT,
			offered_pokemon_id INTEGER,
			offered_pokemon_name TEXT,
			offered_nickname TEXT,
			dialogue_set TEXT,
			original_trade_index INTEGER
		);
		CREATE TABLE character_in_game_trades (
			character_id INTEGER,
			trade_key TEXT
		);
		INSERT INTO phaser_branching_dialogue (
			prompt_text_constant, prompt_text, yes_dialogue, no_dialogue
		)
		VALUES (?, 'A secret switch! Press it?', 'Who would not?', 'Not yet.');
	`, textConstant); err != nil {
		raw.Close()
		t.Fatal(err)
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
		raw.Close()
	})
}

func setupGeneratedConditionalDialogueResolverTestDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`
		CREATE TABLE phaser_text_pointers (
			text_constant TEXT,
			dialogue_label TEXT,
			is_trainer INTEGER,
			map_name TEXT
		);
		CREATE TABLE phaser_dialogue_text (
			label TEXT,
			source_file TEXT,
			dialogue TEXT
		);
		CREATE TABLE phaser_conditional_dialogue (
			id INTEGER PRIMARY KEY,
			text_constant TEXT NOT NULL,
			priority INTEGER NOT NULL DEFAULT 0,
			requires_flag TEXT DEFAULT NULL,
			requires_flag_absent TEXT DEFAULT NULL,
			requires_flags TEXT DEFAULT NULL,
			requires_flags_absent TEXT DEFAULT NULL,
			override_dialogue TEXT NOT NULL,
			override_speaker TEXT DEFAULT NULL,
			dialogue_labels TEXT DEFAULT NULL,
			source TEXT DEFAULT 'manual'
		);
		INSERT INTO phaser_text_pointers (text_constant, dialogue_label, is_trainer, map_name)
		VALUES ('TEXT_OAKSLAB_RIVAL', '_OaksLabRivalMyPokemonLooksStrongerText', 0, 'OaksLab');
		INSERT INTO phaser_dialogue_text (label, source_file, dialogue)
		VALUES ('_OaksLabRivalMyPokemonLooksStrongerText', 'OaksLab.asm', 'Base pointer should be overridden.');
		INSERT INTO phaser_conditional_dialogue
			(text_constant, priority, requires_flags_absent, override_dialogue, dialogue_labels, source)
		VALUES
			('TEXT_OAKSLAB_RIVAL', 300, '["EVENT_FOLLOWED_OAK_INTO_LAB_2"]', 'Gramps isn''t around!', '["_OaksLabRivalGrampsIsntAroundText"]', 'extractor');
		INSERT INTO phaser_conditional_dialogue
			(text_constant, priority, requires_flags, override_dialogue, dialogue_labels, source)
		VALUES
			('TEXT_OAKSLAB_RIVAL', 200, '["EVENT_FOLLOWED_OAK_INTO_LAB_2","EVENT_GOT_STARTER"]', 'My Pokemon looks stronger.', '["_OaksLabRivalMyPokemonLooksStrongerText"]', 'extractor');
		INSERT INTO phaser_conditional_dialogue
			(text_constant, priority, requires_flags, requires_flags_absent, override_dialogue, dialogue_labels, source)
		VALUES
			('TEXT_OAKSLAB_RIVAL', 100, '["EVENT_FOLLOWED_OAK_INTO_LAB_2"]', '["EVENT_GOT_STARTER"]', 'Go ahead and choose!', '["_OaksLabRivalGoAheadAndChooseText"]', 'extractor');
	`); err != nil {
		raw.Close()
		t.Fatal(err)
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
		raw.Close()
	})
}

func stringPtr(value string) *string {
	return &value
}
