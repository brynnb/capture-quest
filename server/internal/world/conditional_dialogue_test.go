package world

import (
	"database/sql"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestCheckConditionalDialogueSupportsGeneratedMultiFlagRows(t *testing.T) {
	raw := newConditionalDialogueTestDB(t)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
	})

	efm := NewEventFlagManager(nil)
	const charID int64 = 7
	efm.flags[charID] = map[string]bool{}

	override := checkConditionalDialogue("TEXT_OAKSLAB_RIVAL", charID, efm)
	if override == nil || override.dialogue != "Gramps isn't around!" {
		t.Fatalf("no-flag override = %#v, want Gramps branch", override)
	}

	efm.flags[charID]["EVENT_FOLLOWED_OAK_INTO_LAB_2"] = true
	override = checkConditionalDialogue("TEXT_OAKSLAB_RIVAL", charID, efm)
	if override == nil || override.dialogue != "Go ahead and choose!" {
		t.Fatalf("followed override = %#v, want choose branch", override)
	}

	efm.flags[charID]["EVENT_GOT_STARTER"] = true
	override = checkConditionalDialogue("TEXT_OAKSLAB_RIVAL", charID, efm)
	if override == nil || override.dialogue != "My Pokemon looks stronger." {
		t.Fatalf("starter override = %#v, want stronger branch", override)
	}
}

func TestCheckConditionalDialogueKeepsScalarFlagCompatibility(t *testing.T) {
	raw := newConditionalDialogueTestDB(t)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
	})

	efm := NewEventFlagManager(nil)
	const charID int64 = 9
	efm.flags[charID] = map[string]bool{"EVENT_DONE": true}

	override := checkConditionalDialogue("TEXT_SCALAR", charID, efm)
	if override == nil || override.dialogue != "Scalar branch" {
		t.Fatalf("scalar override = %#v, want scalar branch", override)
	}
}

func newConditionalDialogueTestDB(t *testing.T) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })

	if _, err := raw.Exec(`
		CREATE TABLE phaser_conditional_dialogue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
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
		INSERT INTO phaser_conditional_dialogue
			(text_constant, priority, requires_flags_absent, override_dialogue, source)
		VALUES
			('TEXT_OAKSLAB_RIVAL', 300, '["EVENT_FOLLOWED_OAK_INTO_LAB_2"]', 'Gramps isn''t around!', 'extractor');
		INSERT INTO phaser_conditional_dialogue
			(text_constant, priority, requires_flags, override_dialogue, source)
		VALUES
			('TEXT_OAKSLAB_RIVAL', 200, '["EVENT_FOLLOWED_OAK_INTO_LAB_2","EVENT_GOT_STARTER"]', 'My Pokemon looks stronger.', 'extractor');
		INSERT INTO phaser_conditional_dialogue
			(text_constant, priority, requires_flags, requires_flags_absent, override_dialogue, source)
		VALUES
			('TEXT_OAKSLAB_RIVAL', 100, '["EVENT_FOLLOWED_OAK_INTO_LAB_2"]', '["EVENT_GOT_STARTER"]', 'Go ahead and choose!', 'extractor');
		INSERT INTO phaser_conditional_dialogue
			(text_constant, priority, requires_flag, override_dialogue, source)
		VALUES
			('TEXT_SCALAR', 10, 'EVENT_DONE', 'Scalar branch', 'manual');
	`); err != nil {
		t.Fatal(err)
	}
	return raw
}
