package world

import (
	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
	"capturequest/internal/testdb"
	"context"
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

func TestLocalInventoryCreationFixtureDoesNotReplenishOnReentry(t *testing.T) {
	t.Setenv("CAPTUREQUEST_TEST_MODE", "true")
	cfg, err := config.Get()
	if err != nil {
		t.Fatal(err)
	}
	previousLocal := cfg.Local
	cfg.Local = true
	t.Cleanup(func() { cfg.Local = previousLocal })
	database, _, _, _ := battleTestWorld(t)
	if err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, 42); err != nil {
			return err
		}
		return seedLocalDevInventoryIn(tx, 42)
	}); err != nil {
		t.Fatal(err)
	}
	var initial int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&initial); err != nil || initial == 0 {
		t.Fatalf("creation fixture missing: %d %v", initial, err)
	}
	testdb.Exec(t, database, `DELETE FROM cq_character_inventory WHERE character_id=42`)
	// The remaining world-entry fixture can initialize an empty party, but cannot
	// replenish even a fully consumed inventory. This fixture already has a party.
	ensureLocalDevFixtures(42)
	var after int
	if err := database.QueryRow(`SELECT COUNT(*) FROM cq_character_inventory WHERE character_id=42`).Scan(&after); err != nil || after != 0 {
		t.Fatalf("reentry refilled inventory: %d %v", after, err)
	}
}
