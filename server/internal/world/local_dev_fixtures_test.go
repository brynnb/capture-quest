package world

import (
	"capturequest/internal/pokebattle"
	"testing"
)

func TestLocalDevPartyReentryPreservesSavedBattleIdentity(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	current := battleTestStart(t, database, true, nil)
	// The existing party needs no fixture catalog and must keep damage and row IDs.
	if err := ensureLocalDevPokemonParty(database, 42); err != nil {
		t.Fatal(err)
	}
	restored, err := pokebattle.LoadBattleState(database, 42)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.RestoreParty(database, 42); err != nil {
		t.Fatal(err)
	}
	if restored.BattleID != current.BattleID || restored.PlayerParty[0].CurHP != 1 {
		t.Fatal("local reentry changed the saved battle party")
	}
}
