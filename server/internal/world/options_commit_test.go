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
	payload := fmt.Sprintf(`{"optionId":%d,"value":0}`, options.OptionShowNetworkStats)
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
