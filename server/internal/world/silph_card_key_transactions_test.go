package world

import (
	"context"
	"testing"

	"capturequest/internal/db/cqitems"
	"capturequest/internal/testdb"
)

func TestSilphCardKeyDurableEligibilityAndCommitFailure(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	text := silphCardKeyDoors[0].TextConstant
	flag := silphCardKeyDoors[0].Flag
	outcome, err := handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err != nil || outcome.Opened || outcome.HasCardKey {
		t.Fatalf("missing key: %+v %v", outcome, err)
	}
	testdb.Exec(t, database, `INSERT INTO cq_items(id,name,short_name,is_key_item) VALUES(48,'CARD KEY','CARD_KEY',true)`)
	if _, err := cqitems.NewStore(database).AddItemToInventory(42, 48, 1); err != nil {
		t.Fatal(err)
	}
	testdb.Exec(t, database, `
 CREATE FUNCTION reject_door_commit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'reject door commit'; END $$;
 CREATE CONSTRAINT TRIGGER reject_door_commit AFTER INSERT
 ON character_event_flags DEFERRABLE INITIALLY DEFERRED
 FOR EACH ROW EXECUTE FUNCTION reject_door_commit();`)
	outcome, err = handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err == nil || outcome != nil {
		t.Fatalf("commit failure publication: %+v %v", outcome, err)
	}
	on, err := queryEventFlag(database, 42, flag)
	if err != nil || on || wh.EventFlags.CheckFlag(42, flag) {
		t.Fatalf("failure flag: %v %v", on, err)
	}
	testdb.Exec(t, database, `DROP TRIGGER reject_door_commit ON character_event_flags`)
	outcome, err = handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err != nil || !outcome.Opened || !outcome.Changed || !wh.EventFlags.CheckFlag(42, flag) {
		t.Fatalf("retry: %+v %v", outcome, err)
	}
	wh.EventFlags.UnloadFlags(42)
	outcome, err = handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err != nil || !outcome.AlreadyOpen || outcome.Changed {
		t.Fatalf("stale cache duplicate: %+v %v", outcome, err)
	}
	// A previously opened door still follows the existing no-key dialogue rule.
	testdb.Exec(t, database, `DELETE FROM cq_character_inventory WHERE character_id=42`)
	outcome, err = handleSilphCardKeyDoor(context.Background(), database, 42, text, wh.EventFlags)
	if err != nil || outcome.HasCardKey || outcome.Opened {
		t.Fatalf("removed key: %+v %v", outcome, err)
	}
}
