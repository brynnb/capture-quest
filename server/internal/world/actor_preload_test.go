package world

import (
	"strings"
	"sync"
	"testing"

	"capturequest/internal/testdb"
)

func TestActorPreloadFailurePreservesActorsAndCollisionRetry(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	wh.ActorRegistry = NewActorRegistry()
	m := NewPhaserActorManager(wh)
	wh.ActorManager = m
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(40,'test',10,10);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,sprite_name) VALUES(101,40,2,3,'npc','SPRITE_TEST');
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type,raw_foot_tile_id) VALUES(40,2,3,1,1,7);`)
	database.SetMaxOpenConns(1)
	if err := m.Load(); err != nil {
		t.Fatal(err)
	}
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 101)
	previous := m.walkingActors[actorID]
	if previous == nil || m.collisionMap[40]["2,3"] != 1 {
		t.Fatal("actor family not loaded")
	}
	testdb.Exec(t, database, `ALTER TABLE phaser_tiles RENAME TO unavailable_tiles`)
	if err := m.Load(); err == nil || !strings.Contains(err.Error(), "map 40 collision") {
		t.Fatalf("collision failure not propagated: %v", err)
	}
	if m.walkingActors[actorID] != previous || m.collisionMap[40]["2,3"] != 1 {
		t.Fatal("failed preload replaced published family")
	}
	m.InvalidateCollisionMap(40)
	if err := m.ensureWalkableMapLoaded(40); err == nil {
		t.Fatal("failed lazy load accepted")
	}
	if _, ok := m.collisionMap[40]; ok {
		t.Fatal("failed query poisoned collision cache")
	}
	testdb.Exec(t, database, `ALTER TABLE unavailable_tiles RENAME TO phaser_tiles`)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := m.collisionMapForMap(40)["2,3"]; got != 1 {
				t.Errorf("collision retry=%d", got)
			}
		}()
	}
	wg.Wait()
	if m.rawFootTileMapForMap(40)["2,3"] != 7 {
		t.Fatal("collision retry lost foot-tile provenance")
	}
}
