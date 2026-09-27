package world

import (
	"strings"
	"testing"

	"capturequest/internal/testdb"
)

func TestEncounterPreloadRejectsOrphansAndPreservesCompleteCache(t *testing.T) {
	database := testdb.Postgres(t)
	wh := &WorldHandler{database: database}
	wh.ActorManager = NewPhaserActorManager(wh)
	m := NewWildEncounterManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_encounter_areas(id,name,encounter_rate) VALUES(1,'test',25);
 INSERT INTO phaser_encounter_area_slots(encounter_area_id,slot_index,pokemon_id,level,probability) VALUES(1,0,25,5,1);
 INSERT INTO phaser_tiles(x,y,tile_image_id,encounter_area_id) VALUES(10,11,1,1);`)
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	previous := m.areas[1]
	if previous == nil || len(previous.Slots) != 1 || m.tileCache[[3]int{UnifiedOverworldMapID, 10, 11}] != 1 || !m.cacheLoaded {
		t.Fatal("complete encounter family not loaded")
	}
	checkUnchanged := func() {
		t.Helper()
		if m.areas[1] != previous || len(m.areas) != 1 || len(m.tileCache) != 1 {
			t.Fatal("failed load published partial encounter state")
		}
	}
	testdb.Exec(t, database, `INSERT INTO phaser_encounter_area_slots(encounter_area_id,slot_index,pokemon_id,level,probability) VALUES(999,0,25,5,1)`)
	if err := m.Load(); err == nil || !strings.Contains(err.Error(), "absent area 999") {
		t.Fatalf("orphan slot: %v", err)
	}
	checkUnchanged()
	testdb.Exec(t, database, `DELETE FROM phaser_encounter_area_slots WHERE encounter_area_id=999;
 INSERT INTO phaser_tiles(x,y,tile_image_id,encounter_area_id) VALUES(12,13,1,999);`)
	if err := m.Load(); err == nil || !strings.Contains(err.Error(), "(12,13)") {
		t.Fatalf("orphan tile identity: %v", err)
	}
	checkUnchanged()
	testdb.Exec(t, database, `ALTER TABLE phaser_tiles RENAME TO unavailable_tiles`)
	if err := m.Load(); err == nil {
		t.Fatal("missing tile table accepted")
	}
	checkUnchanged()
}
