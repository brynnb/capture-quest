package world

import "testing"

func TestSurfableWaterRejectsWarpMats(t *testing.T) {
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			37: {
				"2,7": collisionWater,
				"4,7": collisionWater,
			},
		},
	}
	warpManager := newPhaserWarpManager(nil)
	addPhaserWarpIndex(warpManager.byMap, 37, &phaserMapWarp{
		SourceMapID: 37,
		X:           2,
		Y:           7,
		WarpType:    "carpet",
	})
	wh := &WorldHandler{
		ActorManager: actorManager,
		phaserWarps:  warpManager,
	}

	if isSurfableWaterTile(wh, 37, 2, 7) {
		t.Fatalf("warp mat with water collision should not be surfable")
	}
	if !isSurfableWaterTile(wh, 37, 4, 7) {
		t.Fatalf("plain water collision tile should be surfable")
	}
}
