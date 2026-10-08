package content

import (
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"errors"
	"testing"
	"time"
)

func TestStaticSnapshotFailureIsRetryableAndNeverPublishesPartialData(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO poke_classes(id,name,class_type) VALUES(1,'Trainer','TRAINER'); INSERT INTO poke_factions(id,name,short_name,is_playable,is_starting) VALUES(1,'League','LG',1,1); INSERT INTO phaser_maps(id,name,width,height,is_overworld) VALUES(1,'PALLET_TOWN',20,20,1); INSERT INTO poke_start_cities(id,map_id,name,spawn_x,spawn_y) VALUES(1,1,'Pallet',1,2)`)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	service := New(database)
	database.SetMaxOpenConns(1)
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	data, err := service.StaticData(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || data != nil || database.Stats().WaitCount == before {
		t.Fatalf("cancelled initial load=%+v %v", data, err)
	}
	lease.Close()
	data, err = service.StaticData(context.Background())
	if err != nil || data == nil || len(data.Classes) != 1 || len(data.Factions) != 1 || len(data.Maps) != 1 || len(data.StartCities) != 1 || !data.Maps[0].IsOverworld {
		t.Fatalf("retry poisoned by initial cancellation: %+v %v", data, err)
	}
	testdb.Exec(t, database, `ALTER TABLE phaser_maps RENAME TO unavailable_maps`)
	data, err = service.StaticData(context.Background())
	if err == nil || data != nil {
		t.Fatalf("partial static snapshot escaped: %+v %v", data, err)
	}
	testdb.Exec(t, database, `ALTER TABLE unavailable_maps RENAME TO phaser_maps; UPDATE poke_classes SET name='Current' WHERE id=1`)
	data, err = service.StaticData(context.Background())
	if err != nil || data.Classes[0].Name != "Current" {
		t.Fatalf("repair did not permit a fresh read: %+v %v", data, err)
	}
}

func TestEmptyStaticSnapshotHasArrayLists(t *testing.T) {
	data, err := New(testdb.Postgres(t)).StaticData(context.Background())
	if err != nil || data == nil || data.Classes == nil || data.Factions == nil || data.Maps == nil || data.StartCities == nil {
		t.Fatalf("empty lists not represented as arrays: %+v %v", data, err)
	}
}
