package world

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"capturequest/internal/session"
	"capturequest/internal/testdb"
)

func TestScriptAndWarpLoadersReturnQueryFailures(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.Close()
	wh := &WorldHandler{database: database}
	loaders := map[string]func(context.Context) error{
		"coordinates":     NewCoordinateTriggerManager(database).Load,
		"map scripts":     NewMapScriptManager(database).Load,
		"spin tiles":      NewSpinTileManager(database).Load,
		"warp tiles":      NewWarpTileManager(database).Load,
		"map warps":       newPhaserWarpManager(database).load,
		"trainers":        NewTrainerEncounterManager(wh).Load,
		"wild encounters": NewWildEncounterManager(wh).Load,
	}
	for name, load := range loaders {
		t.Run(name, func(t *testing.T) {
			if err := load(context.Background()); err == nil {
				t.Fatal("loader swallowed database failure")
			}
		})
	}
}

func TestMalformedSpinLoadKeepsPreviousCompleteCache(t *testing.T) {
	database := openSpinTileManagerTestDB(t)
	m := NewSpinTileManager(database)
	if err := m.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	previous := m.CheckTile("ROCKET_HIDEOUT_B2F", 4, 15)
	if _, err := database.Exec(`INSERT INTO phaser_spin_tiles(map_name,x,y,movements) VALUES('VIRIDIAN_GYM',19,11,'not-json')`); err != nil {
		t.Fatal(err)
	}
	if err := m.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "VIRIDIAN_GYM (19,11)") {
		t.Fatalf("missing malformed record identity: %v", err)
	}
	if m.CheckTile("ROCKET_HIDEOUT_B2F", 4, 15) != previous {
		t.Fatal("failed reload replaced previous cache")
	}
	if m.CheckTile("VIRIDIAN_GYM", 19, 11) != nil {
		t.Fatal("published malformed record")
	}
}

func TestWorldConstructionRejectsRequiredPreloadFailure(t *testing.T) {
	database, _, _, _ := battleTestWorld(t)
	testdb.Exec(t, database, `ALTER TABLE phaser_coordinate_triggers RENAME TO unavailable_coordinate_triggers`)
	wh, err := NewWorldHandler(context.Background(), session.NewSessionManager())
	if wh != nil || err == nil || !strings.Contains(err.Error(), "preload CoordTriggers") {
		t.Fatalf("construction result=(%v,%v)", wh, err)
	}
}
