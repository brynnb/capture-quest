package world

import (
	"database/sql"
	"strings"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestMalformedConditionalFlagsRejectInsteadOfEnablingOverride(t *testing.T) {
	raw := newConditionalDialogueTestDB(t)
	if _, err := raw.Exec(`CREATE TABLE phaser_text_pointers(text_constant TEXT,dialogue_label TEXT,is_trainer INTEGER,map_name TEXT);
 CREATE TABLE phaser_dialogue_text(label TEXT,source_file TEXT,dialogue TEXT);
 INSERT INTO phaser_text_pointers VALUES('TEXT_OAKSLAB_RIVAL','Base',0,NULL);
 INSERT INTO phaser_dialogue_text VALUES('Base','fixture','Original dialogue');`); err != nil {
		t.Fatal(err)
	}
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	if _, err := raw.Exec(`UPDATE phaser_conditional_dialogue SET requires_flags='{"broken":true}' WHERE text_constant='TEXT_OAKSLAB_RIVAL'`); err != nil {
		t.Fatal(err)
	}
	flags := NewEventFlagManager(nil)
	flags.flags[7] = map[string]bool{}
	override, err := checkConditionalDialogue("TEXT_OAKSLAB_RIVAL", 7, flags)
	if err == nil || override != nil || !strings.Contains(err.Error(), "requires_flags") {
		t.Fatalf("malformed conditions=%+v error=%v", override, err)
	}
	entries, err := resolvePhaserDialogueEntries("TEXT_OAKSLAB_RIVAL", 7, flags)
	if err == nil || entries != nil || !strings.Contains(err.Error(), "requires_flags") {
		t.Fatalf("resolver hid malformed condition: entries=%+v error=%v", entries, err)
	}
}

func TestConditionalQueryFailureCannotBecomeDefaultDialogue(t *testing.T) {
	raw := newConditionalDialogueTestDB(t)
	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	if _, err := raw.Exec(`DROP TABLE phaser_conditional_dialogue`); err != nil {
		t.Fatal(err)
	}
	override, err := checkConditionalDialogue("TEXT_SCALAR", 7, NewEventFlagManager(nil))
	if err == nil || override != nil {
		t.Fatalf("query failure became default: override=%+v error=%v", override, err)
	}
}

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

	override, err := checkConditionalDialogue("TEXT_OAKSLAB_RIVAL", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
	if override == nil || override.dialogue != "Gramps isn't around!" {
		t.Fatalf("no-flag override = %#v, want Gramps branch", override)
	}

	efm.flags[charID]["EVENT_FOLLOWED_OAK_INTO_LAB_2"] = true
	override, err = checkConditionalDialogue("TEXT_OAKSLAB_RIVAL", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
	if override == nil || override.dialogue != "Go ahead and choose!" {
		t.Fatalf("followed override = %#v, want choose branch", override)
	}

	efm.flags[charID]["EVENT_GOT_STARTER"] = true
	override, err = checkConditionalDialogue("TEXT_OAKSLAB_RIVAL", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
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

	override, err := checkConditionalDialogue("TEXT_SCALAR", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
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
