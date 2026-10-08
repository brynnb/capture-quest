package world

import (
	"capturequest/internal/db"
	"context"
	"testing"
)

func TestResolveScriptDialogueFallbackEntriesRoute18Gate2FYoungster(t *testing.T) {
	raw := setupInGameTradeTestDB(t, 42, true)
	db.GlobalWorldDB = nil

	entries, err := resolveInGameTradeDialogueEntries(context.Background(), raw, route18Gate2FYoungsterTextConstant, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Label != "Route18Gate2FYoungsterText" {
		t.Fatalf("label = %q", entries[0].Label)
	}
	if entries[0].SourceFile != "scripts/Route18Gate2F.asm" {
		t.Fatalf("source file = %q", entries[0].SourceFile)
	}
	if entries[0].Dialogue != "I'm looking for\nSLOWBRO!" {
		t.Fatalf("dialogue = %q", entries[0].Dialogue)
	}
}

func TestResolveScriptDialogueFallbackEntriesRoute18Gate2FYoungsterAfterTrade(t *testing.T) {
	raw := setupInGameTradeTestDB(t, 42, true)
	if _, err := raw.Exec(`
		INSERT INTO character_in_game_trades (
			character_id, trade_key, given_pokemon_id, received_pokemon_id, received_nickname
		)
		VALUES (42, 'TRADE_FOR_MARC', 80, 108, 'MARC')`); err != nil {
		t.Fatal(err)
	}

	entries, err := resolveInGameTradeDialogueEntries(context.Background(), raw, route18Gate2FYoungsterTextConstant, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Dialogue != "Isn't my old\nLICKITUNG great?" {
		t.Fatalf("dialogue = %q", entries[0].Dialogue)
	}
	if bd := checkInGameTradeBranchingDialogue(route18Gate2FYoungsterTextConstant, 42); bd != nil {
		t.Fatalf("branching dialogue after completed trade = %#v", bd)
	}
}

func TestTradeDialogueRejectsCompletionLookupFailure(t *testing.T) {
	raw := setupInGameTradeTestDB(t, 42, true)
	if _, err := raw.Exec(`DROP TABLE character_in_game_trades`); err != nil {
		t.Fatal(err)
	}
	entries, err := resolveInGameTradeDialogueEntries(context.Background(), raw, route18Gate2FYoungsterTextConstant, 42)
	if err == nil || entries != nil {
		t.Fatalf("entries=%v error=%v", entries, err)
	}
}

func TestTradeDialogueMissingDefinitionIsNotQueryFailure(t *testing.T) {
	raw := setupInGameTradeTestDB(t, 42, true)
	entries, err := resolveInGameTradeDialogueEntries(context.Background(), raw, "UNKNOWN_TEXT", 42)
	if err != nil || entries != nil {
		t.Fatalf("absent entries=%v error=%v", entries, err)
	}
	if _, err := raw.Exec(`DROP TABLE phaser_in_game_trades`); err != nil {
		t.Fatal(err)
	}
	entries, err = resolveInGameTradeDialogueEntries(context.Background(), raw, "UNKNOWN_TEXT", 42)
	if err == nil || entries != nil {
		t.Fatalf("failed entries=%v error=%v", entries, err)
	}
}
