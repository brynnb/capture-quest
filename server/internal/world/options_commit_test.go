package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/options"
	"capturequest/internal/testdb"
	"capturequest/internal/zone/client"
	"context"
	"fmt"
	"testing"
)

func TestPreferenceCacheChangesOnlyAfterPatchCommit(t *testing.T) {
	wh, ses, _ := setupIssuedStep(t)
	actual, err := client.NewClient(context.Background(), wh.database, ses.Client.CharData(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ses.Client = actual
	payload := fmt.Sprintf(`{"requestId":"pref:1","characterId":42,"revision":0,"optionId":%d,"value":0}`, options.OptionShowNetworkStats)
	testdb.Exec(t, wh.database, `CREATE FUNCTION reject_preference_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reject preference'; END $$; CREATE CONSTRAINT TRIGGER reject_preference_commit AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION reject_preference_commit();`)
	battleDispatch(t, wh, ses, opcodes.SetOption, payload)
	if !ses.Client.ShowNetworkStatsEnabled() {
		t.Fatal("failed preference commit changed cache")
	}
	testdb.Exec(t, wh.database, `DROP TRIGGER reject_preference_commit ON character_data`)
	battleDispatch(t, wh, ses, opcodes.SetOption, payload)
	if ses.Client.ShowNetworkStatsEnabled() {
		t.Fatal("committed preference not cached")
	}
}

func TestPreferenceRevisionRejectsDuplicateAndOldCharacterWrites(t *testing.T) {
	wh, ses, _ := setupIssuedStep(t)
	actual, err := client.NewClient(context.Background(), wh.database, ses.Client.CharData(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ses.Client = actual
	dispatch := func(id int, revision int, value int) {
		battleDispatch(t, wh, ses, opcodes.SetOption, fmt.Sprintf(`{"requestId":"pref:%d","characterId":%d,"revision":%d,"optionId":16,"value":%d}`, revision, id, revision, value))
	}
	dispatch(43, 0, 1)
	dispatch(42, 0, 1)
	dispatch(42, 0, 0) // Duplicate/late revision cannot reverse the newer desired state.
	if !ses.Client.AllowTrainerRebattles() {
		t.Fatal("late preference reversed committed state")
	}
	var revision int
	if err := wh.database.QueryRow(`SELECT (options->>'preferenceRevision')::int FROM character_data WHERE id=42`).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("revision=%d: %v", revision, err)
	}
	dispatch(42, 1, 0)
	if ses.Client.AllowTrainerRebattles() {
		t.Fatal("new desired preference did not commit")
	}
	// A current-state request must be read-only and return the durable preference.
	battleDispatch(t, wh, ses, opcodes.SetOption, `{"requestId":"pref:read","characterId":42,"current":true}`)
	if err := wh.database.QueryRow(`SELECT (options->>'preferenceRevision')::int FROM character_data WHERE id=42`).Scan(&revision); err != nil || revision != 2 {
		t.Fatalf("current read mutated revision=%d: %v", revision, err)
	}
}
