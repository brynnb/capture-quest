package world

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestAmbiguousLastMapRoute22GateExitResolvesPerPlayer(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	statements := []string{
		`CREATE TABLE phaser_maps (id INTEGER PRIMARY KEY, name TEXT)`,
		`CREATE TABLE phaser_warps (id INTEGER PRIMARY KEY, source_map_id INTEGER, source_warp_index INTEGER, x INTEGER, y INTEGER)`,
		`INSERT INTO phaser_maps VALUES (33, 'ROUTE_22'), (34, 'ROUTE_23')`,
		`INSERT INTO phaser_warps VALUES (1, 33, 1, 8, 5), (2, 34, 1, 7, 139)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	dynamic := PhaserWarp{DestinationKind: "last-map", DestinationWarpID: 1}
	warpA, err := resolvePhaserWarpForPlayer(database, 33, dynamic)
	if err != nil {
		t.Fatal(err)
	}
	warpB, err := resolvePhaserWarpForPlayer(database, 34, dynamic)
	if err != nil {
		t.Fatal(err)
	}
	if *warpA.DestinationMapID != 33 || *warpA.DestinationMap != "ROUTE_22" || *warpA.DestinationX != 8 || *warpA.DestinationY != 5 {
		t.Fatalf("player A destination = (%d,%s,%d,%d)", *warpA.DestinationMapID, *warpA.DestinationMap, *warpA.DestinationX, *warpA.DestinationY)
	}
	if *warpB.DestinationMapID != 34 || *warpB.DestinationMap != "ROUTE_23" || *warpB.DestinationX != 7 || *warpB.DestinationY != 139 {
		t.Fatalf("player B destination = (%d,%s,%d,%d)", *warpB.DestinationMapID, *warpB.DestinationMap, *warpB.DestinationX, *warpB.DestinationY)
	}
	if dynamic.DestinationMapID != nil || dynamic.DestinationMap != nil || dynamic.DestinationX != nil || dynamic.DestinationY != nil {
		t.Fatal("per-player resolution mutated the shared dynamic warp")
	}
}

func TestLastMapWarpRequiresPreviousMapContext(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, _, _, _, err := resolveLastMapWarpForPlayer(database, -1, 1); err == nil {
		t.Fatal("expected missing previous-map context to fail")
	}
}

func TestResolvedLastMapStarterHouseExitIgnoresInternalPreviousMap(t *testing.T) {
	mapID, x, y := 0, 5, 5
	mapName := "PALLET_TOWN"
	warp := PhaserWarp{
		DestinationKind:  "last-map",
		DestinationMapID: &mapID,
		DestinationMap:   &mapName,
		DestinationX:     &x,
		DestinationY:     &y,
	}
	got, err := resolvePhaserWarpForPlayer(nil, 38, warp)
	if err != nil {
		t.Fatal(err)
	}
	if *got.DestinationMapID != 0 || *got.DestinationMap != "PALLET_TOWN" || *got.DestinationX != 5 || *got.DestinationY != 5 {
		t.Fatalf(
			"resolved Red house exit = (%d,%s,%d,%d), want Pallet Town despite previous map 38",
			*got.DestinationMapID,
			*got.DestinationMap,
			*got.DestinationX,
			*got.DestinationY,
		)
	}

	warp.DestinationX = nil
	if !shouldResolveLastMapForPlayer(warp) {
		t.Fatal("incomplete LAST_MAP warp should use per-player previous-map context")
	}
}
