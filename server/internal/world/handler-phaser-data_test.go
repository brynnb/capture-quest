package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/protocol"
	"capturequest/internal/testdb"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"runtime"
	"testing"
	"time"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestBranchingDialogueForResponseSuppressesLegacyBranchForScriptOwnedClick(t *testing.T) {
	raw := setupBranchingDialogueTestDB(t, "TEXT_CELADONMARTROOF_LITTLE_GIRL")

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
	if got, err := branchingDialogueForResponse(context.Background(), raw, "TEXT_CELADONMARTROOF_LITTLE_GIRL", 42, wh.EventFlags, wh.Cutscenes); got != nil || err != nil {
		t.Fatalf("branching dialogue = %#v, want nil for script-owned text constant", got)
	}
}

func TestBranchingDialogueForResponseAllowsLegacyBranchWithoutScriptOwner(t *testing.T) {
	raw := setupBranchingDialogueTestDB(t, "TEXT_POKEMONMANSION1F_SWITCH")

	wh := &WorldHandler{
		Cutscenes:  NewCutsceneManager(nil),
		EventFlags: &EventFlagManager{},
	}
	got, err := branchingDialogueForResponse(context.Background(), raw, "TEXT_POKEMONMANSION1F_SWITCH", 42, wh.EventFlags, wh.Cutscenes)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("branching dialogue = nil, want legacy branch")
	}
	if got.PromptText != "A secret switch! Press it?" {
		t.Fatalf("prompt = %q", got.PromptText)
	}
}

func TestResolvePhaserDialogueEntriesAppliesGeneratedConditionalDialogue(t *testing.T) {
	raw := setupGeneratedConditionalDialogueResolverTestDB(t)
	db.GlobalWorldDB = nil

	efm := NewEventFlagManager(nil)
	const charID int64 = 42
	efm.flags[charID] = map[string]bool{}

	entries, err := resolvePhaserDialogueEntries(context.Background(), raw, "TEXT_OAKSLAB_RIVAL", charID, efm)
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
	entries, err = resolvePhaserDialogueEntries(context.Background(), raw, "TEXT_OAKSLAB_RIVAL", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Dialogue != "Go ahead and choose!" {
		t.Fatalf("followed entries = %#v, want choose branch", entries)
	}

	efm.flags[charID]["EVENT_GOT_STARTER"] = true
	entries, err = resolvePhaserDialogueEntries(context.Background(), raw, "TEXT_OAKSLAB_RIVAL", charID, efm)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Dialogue != "My Pokemon looks stronger." {
		t.Fatalf("starter entries = %#v, want stronger branch", entries)
	}
}

func setupBranchingDialogueTestDB(t *testing.T, textConstant string) *sql.DB {
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
	return raw
}

func setupGeneratedConditionalDialogueResolverTestDB(t *testing.T) *sql.DB {
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
	return raw
}

func stringPtr(value string) *string {
	return &value
}

func TestResolveDialogueRejectsScanFailureWithoutPartialEntries(t *testing.T) {
	raw := setupGeneratedConditionalDialogueResolverTestDB(t)
	if _, err := raw.Exec(`UPDATE phaser_text_pointers SET is_trainer='malformed'`); err != nil {
		t.Fatal(err)
	}
	entries, err := resolvePhaserDialogueEntries(context.Background(), raw, "TEXT_OAKSLAB_RIVAL", 42, NewEventFlagManager(nil))
	if err == nil || entries != nil {
		t.Fatalf("entries=%v error=%v", entries, err)
	}
}

func TestDialogueResponseUsesDurableFlagsAndOnePublication(t *testing.T) {
	database, wh, ses, messages := battleTestWorld(t)
	db.GlobalWorldDB = nil
	wh.EventFlags.flags[42] = map[string]bool{"READY": true}
	testdb.Exec(t, database, `INSERT INTO phaser_text_pointers(map_name,text_constant,local_label,dialogue_label) VALUES('ROOM','PROMPT','Local','Base');
 INSERT INTO phaser_dialogue_text(label,source_file,dialogue) VALUES('Base','fixture','Before');
 INSERT INTO phaser_conditional_dialogue(text_constant,requires_flag,override_dialogue) VALUES('PROMPT','READY','Ready');
 INSERT INTO phaser_branching_dialogue(prompt_text_constant,prompt_text,requires_event_flag) VALUES('PROMPT','Choose?','READY')`)
	writer, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.Exec(`LOCK TABLE phaser_text_pointers IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		battleDispatch(t, wh, ses, opcodes.PhaserDialogueRequest, `{"requestId":"dialogue:owned","textConstant":"PROMPT"}`)
	}()
	deadline := time.Now().Add(time.Second)
	for {
		var waiting bool
		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='phaser_text_pointers'::regclass AND NOT granted)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			writer.Rollback()
			<-done
			t.Fatal("dialogue did not reach text read")
		}
		runtime.Gosched()
	}
	if _, err := writer.Exec(`UPDATE phaser_dialogue_text SET dialogue='After' WHERE label='Base'; INSERT INTO character_event_flags(character_id,flag_name) VALUES(42,'READY')`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	<-done
	check := func(want string, branch bool) {
		t.Helper()
		var reply struct {
			Success bool                  `json:"success"`
			Entries []PhaserDialogueEntry `json:"dialogueEntries"`
			Branch  bool                  `json:"hasBranching"`
		}
		if len(messages.streams) == 0 {
			t.Fatal("missing response")
		}
		if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &reply); err != nil {
			t.Fatal(err)
		}
		if !reply.Success || len(reply.Entries) != 1 || reply.Entries[0].Dialogue != want || reply.Branch != branch {
			t.Fatalf("response=%+v want=%q branch=%v", reply, want, branch)
		}
	}
	check("Before", false)
	var identity protocol.PhaserDialogueResponse
	if err := json.Unmarshal(messages.streams[0].payload, &identity); err != nil || identity.RequestID != "dialogue:owned" || identity.CharacterID != 42 || identity.TextConstant != "PROMPT" {
		t.Fatalf("identity=%+v error=%v", identity, err)
	}

	battleDispatch(t, wh, ses, opcodes.PhaserDialogueRequest, `{"requestId":"dialogue:owned","textConstant":"PROMPT"}`)
	check("Ready", true)
	testdb.Exec(t, database, `DROP TABLE phaser_branching_dialogue`)
	battleDispatch(t, wh, ses, opcodes.PhaserDialogueRequest, `{"requestId":"dialogue:owned","textConstant":"PROMPT"}`)
	var failure map[string]any
	if err := json.Unmarshal(messages.streams[len(messages.streams)-1].payload, &failure); err != nil {
		t.Fatal(err)
	}
	if failure["success"] != false || failure["dialogueEntries"] != nil {
		t.Fatalf("partial response=%v", failure)
	}
}

func TestDialogueSnapshotCancelsHeldPoolAndRetries(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	database.SetMaxOpenConns(1)
	held, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	result, err := readPhaserDialogue(ctx, database, "ABSENT", 42, nil)
	if !errors.Is(err, context.DeadlineExceeded) || result.entries != nil || result.branch != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	held.Close()
	if _, err := readPhaserDialogue(context.Background(), database, "ABSENT", 42, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDialogueInvalidRequestReturnsTaggedFailure(t *testing.T) {
	_, wh, ses, messages := battleTestWorld(t)
	battleDispatch(t, wh, ses, opcodes.PhaserDialogueRequest, `{"requestId":"dialogue:bad","textConstant":42}`)
	var reply protocol.PhaserDialogueError
	if len(messages.streams) != 1 || json.Unmarshal(messages.streams[0].payload, &reply) != nil || reply.Success || reply.RequestID != "dialogue:bad" || reply.CharacterID != 42 || reply.Error == "" {
		t.Fatalf("reply=%+v", reply)
	}
}
