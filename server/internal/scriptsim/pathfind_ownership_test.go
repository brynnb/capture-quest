package scriptsim

import (
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"testing"
)

func TestPathfindReadFailureCannotBecomeSuccessfulNotFoundResult(t *testing.T) {
	database := testdb.Postgres(t)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	testdb.Exec(t, database, `ALTER TABLE phaser_tiles RENAME TO unavailable_tiles`)
	scenario := &Scenario{}
	scenario.Trigger.DestX, scenario.Trigger.DestY = 8, 8
	applied := &AppliedFixture{CharacterID: 42, MapID: 50}
	result, err := runPathfind(context.Background(), database, scenario, applied, nil, nil)
	if err == nil || result != nil {
		t.Fatalf("pathfind source failure accepted: %+v %v", result, err)
	}
}
