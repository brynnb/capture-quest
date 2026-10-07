package pokebattle

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

func TestPCIdentityCommandsFollowRowsAcrossReorderAndSlotReuse(t *testing.T) {
	database := partyDatabase(t)
	seedPCPokemon(t, database, 42, 2, BoxParty, 2, 7)
	party := mustLoadParty(t, database, 42)
	a, b, c := party[0].RowID, party[1].RowID, party[2].RowID
	store := cqitems.NewStore(database)
	command := func(revision int64, apply func(db.DBTX) error, want bool) {
		t.Helper()
		snapshot, err := store.ExecuteCommand(context.Background(), 42, revision, apply)
		if (err == nil) != want {
			t.Fatalf("revision=%d err=%v", revision, err)
		}
		if want && snapshot.CommandRevision != revision+1 {
			t.Fatalf("revision=%d", snapshot.CommandRevision)
		}
	}
	// Another authoritative writer can change slots. The target remains b.
	if err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		_, err := ReorderPartyInTransaction(tx, 42, []int64{c, a, b})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	depositB := func(tx db.DBTX) error {
		_, err := DepositPokemonToPCInTransaction(tx, 42, b, 0)
		return err
	}
	command(0, depositB, true)
	command(0, depositB, false) // Shared identity rejects a delayed duplicate.
	box, err := LoadBox(database, 42, 0)
	if err != nil || len(box) != 1 || box[0].RowID != b {
		t.Fatalf("box=%+v err=%v", box, err)
	}
	if got := mustLoadParty(t, database, 42); got[0].RowID != c || got[1].RowID != a {
		t.Fatal("deposit followed an old slot")
	}
	withdrawB := func(tx db.DBTX) error { _, err := WithdrawPokemonFromPCInTransaction(tx, 42, b, 0); return err }
	command(1, withdrawB, true)
	command(2, func(tx db.DBTX) error { _, err := DepositPokemonToPCInTransaction(tx, 42, a, 0); return err }, true)
	// a now occupies b's old PC slot. Even with a current revision, b cannot
	// withdraw a merely because an earlier view associated b with that slot.
	command(3, withdrawB, false)
	box, err = LoadBox(database, 42, 0)
	if err != nil || len(box) != 1 || box[0].RowID != a {
		t.Fatal("stale target affected replacement slot occupant")
	}
	command(3, func(tx db.DBTX) error { _, err := WithdrawPokemonFromPCInTransaction(tx, 42, a, 0); return err }, true)
}

func TestPCIdentityReleaseCannotDeleteReplacementOrForeignPokemon(t *testing.T) {
	database := partyDatabase(t)
	seedPCPokemon(t, database, 42, 0, 0, 0, 7)
	seedPCPokemon(t, database, 43, 0, 0, 0, 7)
	box, err := LoadBox(database, 42, 0)
	if err != nil {
		t.Fatal(err)
	}
	old := box[0].RowID
	foreign, err := LoadBox(database, 43, 0)
	if err != nil {
		t.Fatal(err)
	}
	store := cqitems.NewStore(database)
	if _, err := store.ExecuteCommand(context.Background(), 42, 0, func(tx db.DBTX) error { return ReleasePokemonFromPCInTransaction(tx, 42, old, 0) }); err != nil {
		t.Fatal(err)
	}
	seedPCPokemon(t, database, 42, 0, 0, 0, 7)
	box, err = LoadBox(database, 42, 0)
	if err != nil {
		t.Fatal(err)
	}
	replacement := box[0].RowID
	for _, id := range []int64{old, foreign[0].RowID, mustLoadParty(t, database, 42)[0].RowID, 0} {
		if _, err := store.ExecuteCommand(context.Background(), 42, 1, func(tx db.DBTX) error { return ReleasePokemonFromPCInTransaction(tx, 42, id, 0) }); err == nil {
			t.Fatalf("released invalid target %d", id)
		}
	}
	box, err = LoadBox(database, 42, 0)
	if err != nil || len(box) != 1 || box[0].RowID != replacement {
		t.Fatal("released replacement Pokemon")
	}
	if _, err := store.ExecuteCommand(context.Background(), 42, 1, func(tx db.DBTX) error { return ReleasePokemonFromPCInTransaction(tx, 42, replacement, 0) }); err != nil {
		t.Fatal(err)
	}
}

func TestPCIdentityCommandRollsBackMembershipAndRevisionOnProjectionOrCommitFailure(t *testing.T) {
	for _, stage := range []string{"projection", "boxProjection", "commit"} {
		t.Run(stage, func(t *testing.T) {
			database := partyDatabase(t)
			party := mustLoadParty(t, database, 42)
			store := cqitems.NewStore(database)
			if stage == "commit" {
				testdb.Exec(t, database, `CREATE FUNCTION reject_pc_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'PC commit rejected'; END $$; CREATE CONSTRAINT TRIGGER reject_pc_commit AFTER UPDATE ON character_pokemon DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_pc_commit();`)
			} else if stage == "projection" {
				testdb.Exec(t, database, `ALTER TABLE cq_character_inventory RENAME TO unavailable_inventory`)
			} else {
				seedPCPokemon(t, database, 42, 0, 0, 1, 7)
				testdb.Exec(t, database, `UPDATE character_pokemon SET pokemon_id=999 WHERE character_id=42 AND box=0`)
			}
			apply := func(tx db.DBTX) error {
				_, err := DepositPokemonToPCInTransaction(tx, 42, party[1].RowID, 0)
				if err == nil && stage == "boxProjection" {
					_, err = LoadBox(tx, 42, 0)
				}
				return err
			}
			if _, err := store.ExecuteCommand(context.Background(), 42, 0, apply); err == nil {
				t.Fatal("accepted failed transaction")
			}
			if got := mustLoadParty(t, database, 42); !reflect.DeepEqual(got, party) {
				t.Fatal("failed transaction changed party")
			}
			if stage == "commit" {
				testdb.Exec(t, database, `DROP TRIGGER reject_pc_commit ON character_pokemon`)
			} else if stage == "projection" {
				testdb.Exec(t, database, `ALTER TABLE unavailable_inventory RENAME TO cq_character_inventory`)
			} else {
				testdb.Exec(t, database, `UPDATE character_pokemon SET pokemon_id=7 WHERE character_id=42 AND box=0`)
			}
			if snapshot, err := store.ExecuteCommand(context.Background(), 42, 0, apply); err != nil || snapshot.CommandRevision != 1 {
				t.Fatalf("retry revision=%d err=%v", snapshot.CommandRevision, err)
			}
		})
	}
}

func TestPCIdentityPrimitivesRequireTransactionAndRespectCommandCancellation(t *testing.T) {
	database := partyDatabase(t)
	party := mustLoadParty(t, database, 42)
	if _, err := DepositPokemonToPCInTransaction(database, 42, party[1].RowID, 0); err == nil {
		t.Fatal("deposit accepted pool")
	}
	if _, err := WithdrawPokemonFromPCInTransaction(database, 42, party[1].RowID, 0); err == nil {
		t.Fatal("withdraw accepted pool")
	}
	if err := ReleasePokemonFromPCInTransaction(database, 42, party[1].RowID, 0); err == nil {
		t.Fatal("release accepted pool")
	}
	lock, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Rollback()
	if err := db.LockCharacter(lock, 42); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := cqitems.NewStore(database).ExecuteCommand(ctx, 42, 0, func(tx db.DBTX) error {
		_, err := DepositPokemonToPCInTransaction(tx, 42, party[1].RowID, 0)
		return err
	}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation=%v", err)
	}
	if err := lock.Rollback(); err != nil {
		t.Fatal(err)
	}
	if len(mustLoadParty(t, database, 42)) != 2 {
		t.Fatal("cancelled command changed party")
	}
}

func TestPCIdentityCommandsPreserveCapacityAndSourceMembership(t *testing.T) {
	database := partyDatabase(t)
	party := mustLoadParty(t, database, 42)
	for slot := 0; slot < 20; slot++ {
		seedPCPokemon(t, database, 42, 0, 0, slot, 7)
	}
	box, err := LoadBox(database, 42, 0)
	if err != nil {
		t.Fatal(err)
	}
	store := cqitems.NewStore(database)
	for _, apply := range []func(db.DBTX) error{
		func(tx db.DBTX) error {
			_, err := DepositPokemonToPCInTransaction(tx, 42, party[1].RowID, 0)
			return err
		},
		func(tx db.DBTX) error {
			_, err := DepositPokemonToPCInTransaction(tx, 42, party[1].RowID, 12)
			return err
		},
		func(tx db.DBTX) error {
			_, err := WithdrawPokemonFromPCInTransaction(tx, 42, box[0].RowID, 1)
			return err
		},
		func(tx db.DBTX) error { return ReleasePokemonFromPCInTransaction(tx, 42, box[0].RowID, BoxDayCare) },
	} {
		if _, err := store.ExecuteCommand(context.Background(), 42, 0, apply); err == nil {
			t.Fatal("accepted invalid capacity/source")
		}
	}
	testdb.Exec(t, database, `UPDATE character_pokemon SET box_slot=20 WHERE id=$1`, box[0].RowID)
	if _, err := store.ExecuteCommand(context.Background(), 42, 0, func(tx db.DBTX) error {
		_, err := DepositPokemonToPCInTransaction(tx, 42, party[1].RowID, 0)
		return err
	}); err == nil || !strings.Contains(err.Error(), "malformed") {
		t.Fatalf("malformed occupied slot=%v", err)
	}
	testdb.Exec(t, database, `UPDATE character_pokemon SET box_slot=0 WHERE id=$1`, box[0].RowID)
	if _, err := store.ExecuteCommand(context.Background(), 42, 0, func(tx db.DBTX) error {
		_, err := DepositPokemonToPCInTransaction(tx, 42, party[1].RowID, 1)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ExecuteCommand(context.Background(), 42, 1, func(tx db.DBTX) error {
		_, err := DepositPokemonToPCInTransaction(tx, 42, party[0].RowID, 1)
		return err
	}); err == nil {
		t.Fatal("deposited last party member")
	}
	for slot := 1; slot < 6; slot++ {
		seedPCPokemon(t, database, 42, slot, BoxParty, slot, 7)
	}
	if _, err := store.ExecuteCommand(context.Background(), 42, 1, func(tx db.DBTX) error {
		_, err := WithdrawPokemonFromPCInTransaction(tx, 42, party[1].RowID, 1)
		return err
	}); err == nil {
		t.Fatal("overfilled party")
	}
	stored, err := LoadBox(database, 42, 1)
	if err != nil || len(stored) != 1 || stored[0].RowID != party[1].RowID {
		t.Fatalf("stored=%v err=%v", stored, err)
	}
}
