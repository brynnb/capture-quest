package world

import (
	"testing"
)

func TestResolveScriptDialogueFallbackEntriesRoute18Gate2FYoungster(t *testing.T) {
	setupInGameTradeTestDB(t, 42, true)

	entries := resolveScriptDialogueFallbackEntries(route18Gate2FYoungsterTextConstant, 42, nil)
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

	entries := resolveScriptDialogueFallbackEntries(route18Gate2FYoungsterTextConstant, 42, nil)
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
