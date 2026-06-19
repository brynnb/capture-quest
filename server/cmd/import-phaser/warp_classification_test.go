package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestClassifyWarpActivationsDirectionalMats(t *testing.T) {
	sqlite, err := sql.Open("sqlite", defaultSQLitePath())
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer sqlite.Close()

	classifications, err := classifyWarpActivations(sqlite)
	if err != nil {
		t.Fatalf("classifyWarpActivations: %v", err)
	}

	requireWarpClassification(t, classifications, 40, 4, 11, "carpet", "DOWN")
	requireWarpClassification(t, classifications, 40, 5, 11, "carpet", "DOWN")
	requireWarpClassification(t, classifications, 47, 4, 0, "carpet", "UP")
	requireWarpClassification(t, classifications, 6, 39, 19, "carpet", "UP")
}

func TestInferWarpDirectionPrefersSourceEdge(t *testing.T) {
	mapInfoByName := map[string]sqliteMapDimensions{
		"LANCESROOM": {
			ID:          113,
			WidthTiles:  26,
			HeightTiles: 26,
		},
	}
	warpPointsByMapID := map[int][]sqliteWarpPoint{
		113: {
			{X: 24, Y: 16},
		},
	}

	got := inferWarpDirection(
		4,
		0,
		10,
		12,
		"LANCES_ROOM",
		1,
		nil,
		mapInfoByName,
		warpPointsByMapID,
	)
	if got != "UP" {
		t.Fatalf("Agatha top-edge warp direction = %q, want UP", got)
	}
}

func requireWarpClassification(
	t *testing.T,
	classifications []warpActivationClassification,
	mapID int,
	x int,
	y int,
	wantType string,
	wantDirection string,
) {
	t.Helper()
	for _, classification := range classifications {
		if classification.MapID == mapID && classification.X == x && classification.Y == y {
			if classification.WarpType != wantType || classification.Direction != wantDirection {
				t.Fatalf(
					"warp map %d (%d,%d) = type %q direction %q, want type %q direction %q",
					mapID,
					x,
					y,
					classification.WarpType,
					classification.Direction,
					wantType,
					wantDirection,
				)
			}
			return
		}
	}
	t.Fatalf("warp classification for map %d (%d,%d) not found", mapID, x, y)
}
