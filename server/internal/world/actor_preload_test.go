package world

import (
	"capturequest/internal/db"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"capturequest/internal/testdb"
)

func TestActorPreloadFailurePreservesActorsAndCollisionRetry(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	wh.ActorRegistry = NewActorRegistry()
	m := NewPhaserActorManager(wh)
	wh.ActorManager = m
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(40,'test',10,10);
 INSERT INTO phaser_objects(id,map_id,x,y,object_type,sprite_name) VALUES(101,40,2,3,'npc','SPRITE_TEST');
 INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type,raw_foot_tile_id) VALUES(40,2,3,1,1,7);`)
	database.SetMaxOpenConns(1)
	if err := m.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	actorID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, 101)
	previous := m.walkingActors[actorID]
	if previous == nil || m.collisionMap[40]["2,3"] != 1 {
		t.Fatal("actor family not loaded")
	}
	testdb.Exec(t, database, `ALTER TABLE phaser_tiles RENAME TO unavailable_tiles`)
	if err := m.Load(context.Background()); err == nil || !strings.Contains(err.Error(), "map 40 collision") {
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

func TestCollisionLoadDoesNotBlockInvalidationOrPublishOvertakenRead(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	manager := NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(40,'ROOM',4,4); INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type,raw_foot_tile_id) VALUES(40,2,3,1,1,7)`)
	holder, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	if _, err = holder.Exec(`LOCK TABLE phaser_tiles IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, _, err := manager.baseCollision(ctx, database, 40, true); done <- err }()
	deadline := time.Now().Add(time.Second)
	for {
		var waiting bool
		if err := database.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='phaser_tiles'::regclass AND NOT granted)`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("collision query did not reach held relation lock")
		}
		time.Sleep(5 * time.Millisecond)
	}
	invalidated := make(chan struct{})
	go func() { manager.InvalidateCollisionMap(40); close(invalidated) }()
	select {
	case <-invalidated:
	case <-time.After(200 * time.Millisecond):
		holder.Rollback()
		<-done
		<-invalidated
		t.Fatal("collision I/O held actor lock")
	}
	if _, err = holder.Exec(`UPDATE phaser_tiles SET collision_type=0,raw_foot_tile_id=9 WHERE map_id=40`); err != nil {
		t.Fatal(err)
	}
	if err = holder.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err == nil || !strings.Contains(err.Error(), "invalidated") {
		t.Fatalf("overtaken collision read accepted: %v", err)
	}
	if _, exists := manager.collisionMap[40]; exists {
		t.Fatal("overtaken read warmed cache")
	}
	collision, raw, err := manager.baseCollision(context.Background(), database, 40, true)
	if err != nil || collision["2,3"] != 0 || raw["2,3"] != 9 {
		t.Fatalf("fresh collision retry: %v %v %v", collision, raw, err)
	}
}

func TestCollisionTransactionReadDoesNotWarmSharedCache(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	manager := NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(40,'ROOM',4,4)`)
	rollback := errors.New("reject gameplay transaction")
	err := db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		if _, err := tx.Exec(`INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(40,2,3,1,1)`); err != nil {
			return err
		}
		collision, _, err := manager.characterCollision(context.Background(), tx.(db.ReadDBTX), 42, 40, 0, 0, nil)
		if err != nil || collision["2,3"] != 1 {
			t.Fatalf("transaction-local collision: %v %v", collision, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	if _, exists := manager.collisionMap[40]; exists {
		t.Fatal("transaction populated shared collision cache")
	}
	collision, _, err := manager.baseCollision(context.Background(), database, 40, true)
	if err != nil || len(collision) != 0 {
		t.Fatalf("rolled-back tile escaped: %v %v", collision, err)
	}
}

func TestCollisionInvalidationRetiresAllOverworldAliases(t *testing.T) {
	manager := NewPhaserActorManager(nil)
	manager.overworldMapIds[5] = true
	for _, id := range []int{0, 5, 9999, 40} {
		manager.collisionMap[id] = map[string]int{"2,3": 1}
		manager.rawFootTileMap[id] = map[string]int{"2,3": 7}
	}
	manager.InvalidateCollisionMap(5)
	for _, id := range []int{0, 5, 9999} {
		if _, ok := manager.collisionMap[id]; ok {
			t.Fatalf("overworld alias %d retained collision", id)
		}
		if _, ok := manager.rawFootTileMap[id]; ok {
			t.Fatalf("overworld alias %d retained raw-foot data", id)
		}
	}
	if manager.collisionMap[40]["2,3"] != 1 {
		t.Fatal("interior cache removed by overworld edit")
	}
}

func TestCollisionRejectsSnapshotThatPredatesInvalidation(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	manager := NewPhaserActorManager(wh)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(40,'ROOM',4,4); INSERT INTO phaser_tiles(map_id,x,y,tile_image_id,collision_type) VALUES(40,2,3,1,1)`)
	revision := manager.collisionRevision
	_, err := db.ReadSnapshot(context.Background(), database, func(ctx context.Context, q db.ReadDBTX) (bool, error) {
		var count int
		if err := q.QueryRow(`SELECT count(*) FROM phaser_tiles`).Scan(&count); err != nil {
			return false, err
		}
		testdb.Exec(t, database, `UPDATE phaser_tiles SET collision_type=0 WHERE map_id=40`)
		manager.InvalidateCollisionMap(40)
		_, _, err := manager.characterCollisionIn(ctx, q, 42, 40, 0, 0, nil, true, revision)
		return false, err
	})
	if err == nil || !strings.Contains(err.Error(), "snapshot retired") {
		t.Fatalf("old snapshot accepted newer cache generation: %v", err)
	}
	if _, ok := manager.collisionMap[40]; ok {
		t.Fatal("old snapshot repopulated invalidated cache")
	}
}

func TestCollisionColdReadCancelsWhileRelationLocked(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	manager := NewPhaserActorManager(wh)
	holder, err := database.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Rollback()
	if _, err = holder.Exec(`LOCK TABLE phaser_tiles IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _, err = manager.baseCollision(ctx, database, 40, true)
	if err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("cold query escaped caller deadline: %v", err)
	}
	if _, ok := manager.collisionMap[40]; ok {
		t.Fatal("cancelled read warmed cache")
	}
	manager.InvalidateCollisionMap(40)
}

func TestOverworldReloadWarmsNPCSourceAliases(t *testing.T) {
	database, wh, _, _ := battleTestWorld(t)
	manager := NewPhaserActorManager(wh)
	manager.overworldMapIds[31], manager.overworldMapIds[32] = true, true
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(31,'AREA_A',20,20,1),(32,'AREA_B',20,20,1),(40,'ROOM',4,4,0);
 INSERT INTO phaser_tiles(x,y,tile_image_id,collision_type,raw_foot_tile_id,source_map_id) VALUES(2,3,1,1,7,31),(3,3,1,1,8,31)`)
	manager.collisionMap[40] = map[string]int{"2,3": 0}
	if _, _, err := manager.baseCollision(context.Background(), database, 9999, true); err != nil {
		t.Fatal(err)
	}
	x, y := 2, 3
	direction, terrain := "LEFT_RIGHT", "LAND"
	actor := &PhaserActor{ID: 1, MapID: 31, X: &x, Y: &y, ActionDirection: &direction, MovementType: &terrain}
	manager.mu.Lock()
	nx, ny, dir := manager.calculateNextMove(actor)
	manager.mu.Unlock()
	if nx != 3 || ny != 3 || dir != "RIGHT" {
		t.Fatalf("NPC source alias stayed cold: %d,%d %s", nx, ny, dir)
	}
	for _, id := range []int{0, 31, 32, 9999} {
		if manager.collisionMap[id]["3,3"] != 1 || manager.rawFootTileMap[id]["3,3"] != 8 {
			t.Fatalf("alias %d differs from stitched view", id)
		}
	}
	testdb.Exec(t, database, `UPDATE phaser_tiles SET collision_type=0,raw_foot_tile_id=9 WHERE x=3 AND y=3 AND map_id IS NULL; INSERT INTO phaser_tiles(x,y,tile_image_id,collision_type,source_map_id) VALUES(1,3,1,1,31)`)
	manager.InvalidateCollisionMap(31)
	if _, _, err := manager.baseCollision(context.Background(), database, 9999, true); err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	nx, ny, dir = manager.calculateNextMove(actor)
	manager.mu.Unlock()
	if nx != 1 || ny != 3 || dir != "LEFT" {
		t.Fatalf("NPC reused pre-edit or absent collision: %d,%d %s", nx, ny, dir)
	}
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	for _, id := range []int{0, 31, 32, 9999} {
		collision, raw, err := manager.baseCollision(context.Background(), database, id, true)
		if err != nil || collision["3,3"] != 0 || raw["3,3"] != 9 {
			t.Fatalf("cached alias %d reread or retained old data: %v", id, err)
		}
	}
	if database.Stats().WaitCount != before {
		t.Fatal("shared aliases triggered another SQL read")
	}
	if manager.collisionMap[40]["2,3"] != 0 {
		t.Fatal("overworld reload replaced interior view")
	}
}
