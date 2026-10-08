package world

import (
	"capturequest/internal/db"
	"capturequest/internal/testdb"
	"context"
	"errors"
	"testing"
	"time"
)

func TestWarpReadUsesOneInjectedSnapshotAndResolvesPreviousMapPerViewer(t *testing.T) {
	database := testdb.Postgres(t)
	testdb.Exec(t, database, `INSERT INTO phaser_maps(id,name,width,height) VALUES(33,'ROUTE_22',20,20),(34,'ROUTE_23',20,20),(50,'GATE',20,20); INSERT INTO phaser_warps(id,source_map_id,source_warp_index,x,y,destination_map_id,destination_x,destination_y) VALUES(1,33,1,8,5,50,1,1),(2,34,1,7,139,50,1,1); INSERT INTO phaser_warps(id,source_map_id,x,y,destination_kind,destination_warp_id) VALUES(3,50,4,4,'last-map',1)`)
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	database.SetMaxOpenConns(1)
	for _, previous := range []int{33, 34} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		warps, err := readPlayerWarps(ctx, database, 50, previous)
		cancel()
		if err != nil || len(warps) != 1 || warps[0].DestinationMapID == nil || *warps[0].DestinationMapID != previous {
			t.Fatalf("single-connection viewer %d read: %+v %v", previous, warps, err)
		}
	}
	warps, err := readPlayerWarps(context.Background(), database, 50, 99)
	if err == nil || warps != nil {
		t.Fatalf("unresolved warp hidden as partial success: %+v %v", warps, err)
	}
	empty, err := readPlayerWarps(context.Background(), database, 999, 33)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty warp catalog=%+v %v", empty, err)
	}
	lease, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	before := database.Stats().WaitCount
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = readPlayerWarps(ctx, database, 50, 33)
	if !errors.Is(err, context.DeadlineExceeded) || database.Stats().WaitCount == before {
		t.Fatalf("warp pool deadline escaped: %v", err)
	}
}
