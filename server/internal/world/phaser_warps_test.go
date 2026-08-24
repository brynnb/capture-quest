package world

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestPhaserMapWarpActivationRules(t *testing.T) {
	door := &phaserMapWarp{
		SourceMapID: 40,
		X:           4,
		Y:           11,
		WarpType:    "door",
	}
	if !door.canActivateByClick(40, 4, 10, nil) {
		t.Fatalf("door warp should activate from an adjacent clicked destination")
	}
	if door.canActivateByClick(40, 4, 9, nil) {
		t.Fatalf("door warp should not activate from two tiles away")
	}

	carpet := &phaserMapWarp{
		SourceMapID:   40,
		X:             4,
		Y:             11,
		WarpType:      "carpet",
		WarpDirection: "DOWN",
	}
	if !carpet.canActivateByClick(40, 4, 11, nil) {
		t.Fatalf("clicked carpet warp should activate from the mat tile")
	}
	if carpet.canActivateByClick(40, 4, 10, nil) {
		t.Fatalf("clicked carpet warp should not activate from an adjacent tile")
	}
	if carpet.canActivateByDirection(40, 4, 11, "UP", nil) {
		t.Fatalf("carpet warp should not activate from the wrong direction")
	}
	if !carpet.canActivateByDirection(40, 4, 11, "DOWN", nil) {
		t.Fatalf("carpet warp should activate when standing on the mat and pressing its direction")
	}

	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			40: {
				"4,10": collisionLand,
				"4,11": collisionBlocked,
			},
		},
	}
	if !carpet.canActivateByClick(40, 4, 10, actorManager) {
		t.Fatalf("blocked directional carpet should activate by click from the walkable tile before it")
	}
	if !carpet.canActivateByDirection(40, 4, 10, "DOWN", actorManager) {
		t.Fatalf("blocked directional carpet should activate by pressing its direction from the walkable tile before it")
	}
	if carpet.canActivateByDirection(40, 4, 10, "UP", actorManager) {
		t.Fatalf("blocked directional carpet should not activate from the wrong direction")
	}
}

func TestPhaserWarpManagerLoadSkipsInactiveAndElevatorRows(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE phaser_warps (
			id INTEGER PRIMARY KEY,
			source_map_id INTEGER,
			x INTEGER,
			y INTEGER,
			destination_map_id INTEGER,
			destination_x INTEGER,
			destination_y INTEGER,
			warp_type TEXT,
			warp_direction TEXT
		);
		INSERT INTO phaser_warps (
			id, source_map_id, x, y, destination_map_id, destination_x, destination_y, warp_type, warp_direction
		) VALUES
			(1, 181, 10, 17, 10, 18, 22, 'door', NULL),
			(2, 181, 16, 10, 208, 27, 3, 'inactive', NULL),
			(3, 236, 1, 3, 237, 1, 1, 'elevator', NULL)`); err != nil {
		t.Fatalf("seed db: %v", err)
	}

	manager := newPhaserWarpManager(db)
	manager.load()

	if got := manager.warpAt(181, 10, 17); got == nil || got.ID != 1 {
		t.Fatalf("loaded playable warp = %#v, want id 1", got)
	}
	if got := manager.warpAt(181, 16, 10); got != nil {
		t.Fatalf("inactive complete warp loaded = %#v, want nil", got)
	}
	if got := manager.warpAt(236, 1, 3); got != nil {
		t.Fatalf("elevator metadata warp loaded = %#v, want nil", got)
	}
}

func TestPhaserMapWarpBlockedEntryRequiresBlockedWarpTile(t *testing.T) {
	carpet := &phaserMapWarp{
		SourceMapID:   40,
		X:             4,
		Y:             11,
		WarpType:      "carpet",
		WarpDirection: "DOWN",
	}
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			40: {
				"4,10": collisionLand,
				"4,11": collisionLand,
			},
		},
	}
	if carpet.canActivateByClick(40, 4, 10, actorManager) {
		t.Fatalf("walkable carpet mat should still require standing on the mat")
	}
}

func TestRequestedWarpActivationRequiresCurrentOrWarpDestination(t *testing.T) {
	warp := &phaserMapWarp{
		SourceMapID: 1,
		X:           2,
		Y:           0,
		WarpType:    "door",
	}

	if !warp.canActivateForRequestedDestination(1, 1, 0, 1, 0, nil) {
		t.Fatalf("requested warp should allow explicit activation from the current tile")
	}
	if !warp.canActivateForRequestedDestination(1, 1, 0, 2, 0, nil) {
		t.Fatalf("requested warp should allow movement whose destination is the warp tile")
	}
	if warp.canActivateForRequestedDestination(1, 1, 0, 1, -1, nil) {
		t.Fatalf("requested warp should reject unrelated movement destinations")
	}
}

func TestWarpActivationFacingDirectionPreservesOverworldEntryDirection(t *testing.T) {
	route6GateEntry := &phaserMapWarp{
		SourceMapID:   UnifiedOverworldMapID,
		X:             190,
		Y:             -89,
		DestMapID:     73,
		DestX:         3,
		DestY:         0,
		WarpType:      "carpet",
		WarpDirection: "UP",
	}

	if got := route6GateEntry.activationFacingDirection(UnifiedOverworldMapID, 190, -89, "DOWN", nil); got != "DOWN" {
		t.Fatalf("overworld gate entry direction = %q, want DOWN", got)
	}
}

func TestWarpActivationFacingDirectionKeepsInteriorMatDirection(t *testing.T) {
	interiorExit := &phaserMapWarp{
		SourceMapID:   73,
		X:             3,
		Y:             0,
		WarpType:      "carpet",
		WarpDirection: "UP",
	}

	if got := interiorExit.activationFacingDirection(73, 3, 0, "DOWN", nil); got != "UP" {
		t.Fatalf("interior mat direction = %q, want UP", got)
	}
}

func TestDirectionalWarpForFacingAttemptChecksBlockedFrontTile(t *testing.T) {
	warp := &phaserMapWarp{
		ID:            384,
		SourceMapID:   71,
		X:             3,
		Y:             7,
		WarpType:      "carpet",
		WarpDirection: "DOWN",
	}
	manager := newPhaserWarpManager(nil)
	addPhaserWarpIndex(manager.byMap, warp.SourceMapID, warp)
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			71: {
				"3,6": collisionLand,
				"3,7": collisionBlocked,
			},
		},
	}

	if got := manager.directionalWarpForFacingAttempt(71, 3, 6, "DOWN", actorManager); got != warp {
		t.Fatalf("facing attempt warp = %#v, want blocked front-tile warp", got)
	}
	if got := manager.directionalWarpForFacingAttempt(71, 3, 6, "UP", actorManager); got != nil {
		t.Fatalf("wrong-direction facing attempt warp = %#v, want nil", got)
	}
}

func TestDirectionalWarpForFacingAttemptAllowsBlockedStairsWithDestinationDirection(t *testing.T) {
	warp := &phaserMapWarp{
		ID:            719,
		SourceMapID:   119,
		X:             5,
		Y:             4,
		WarpType:      "carpet",
		WarpDirection: "UP",
	}
	manager := newPhaserWarpManager(nil)
	addPhaserWarpIndex(manager.byMap, warp.SourceMapID, warp)
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			119: {
				"4,4": collisionLand,
				"5,4": collisionWater,
			},
		},
	}

	if !warp.canActivateByClick(119, 4, 4, actorManager) {
		t.Fatalf("blocked stair warp should activate by click from an adjacent walkable tile")
	}
	if got := manager.directionalWarpForFacingAttempt(119, 4, 4, "RIGHT", actorManager); got != warp {
		t.Fatalf("facing attempt warp = %#v, want blocked stair warp", got)
	}
	if got := manager.directionalWarpForFacingAttempt(119, 4, 4, "UP", actorManager); got != nil {
		t.Fatalf("non-facing direction warp = %#v, want nil", got)
	}
}

func TestDirectionalWarpForFacingAttemptAllowsBlockedDoorStairs(t *testing.T) {
	warp := &phaserMapWarp{
		ID:          719,
		SourceMapID: 119,
		X:           5,
		Y:           4,
		WarpType:    "door",
	}
	manager := newPhaserWarpManager(nil)
	addPhaserWarpIndex(manager.byMap, warp.SourceMapID, warp)
	actorManager := &PhaserActorManager{
		collisionMap: map[int]map[string]int{
			119: {
				"4,4": collisionLand,
				"5,4": collisionWater,
			},
		},
	}

	if got := manager.directionalWarpForFacingAttempt(119, 4, 4, "RIGHT", actorManager); got != warp {
		t.Fatalf("facing attempt door warp = %#v, want blocked stair warp", got)
	}
	if got := manager.directionalWarpForFacingAttempt(119, 4, 4, "UP", actorManager); got != nil {
		t.Fatalf("non-facing direction door warp = %#v, want nil", got)
	}
}

func TestPhaserMapWarpPathDestinationActivationRules(t *testing.T) {
	actorManager := &PhaserActorManager{overworldMapIds: map[int]bool{29: true}}

	door := &phaserMapWarp{
		SourceMapID: 40,
		WarpType:    "door",
	}
	if !door.canActivateOnPathDestination(40, actorManager) {
		t.Fatalf("door warp should activate on final path destination")
	}

	overworldCarpet := &phaserMapWarp{
		SourceMapID:   29,
		WarpType:      "carpet",
		WarpDirection: "UP",
	}
	if !overworldCarpet.canActivateOnPathDestination(UnifiedOverworldMapID, actorManager) {
		t.Fatalf("overworld carpet gate should activate on final path destination")
	}

	interiorCarpet := &phaserMapWarp{
		SourceMapID:   190,
		WarpType:      "carpet",
		WarpDirection: "DOWN",
	}
	if interiorCarpet.canActivateOnPathDestination(190, actorManager) {
		t.Fatalf("interior carpet mat should require explicit click or directional activation")
	}
}
