package world

import "testing"

func TestCutTileAllowsOnlyThatCharacterToPathThroughClearedTree(t *testing.T) {
	wh := &WorldHandler{CutTiles: NewCutTileManager()}
	actorManager := NewPhaserActorManager(wh)
	wh.ActorManager = actorManager
	actorManager.collisionMap[1] = map[string]int{
		"0,0": collisionLand,
		"1,0": collisionBlocked,
		"2,0": collisionLand,
	}

	if path := actorManager.FindPathForCharacterWithOptions(7, 1, 0, 0, 2, 0, nil, pathfindOptions{}); len(path) != 0 {
		t.Fatalf("path before cut = %#v, want blocked", path)
	}

	wh.CutTiles.MarkCut(7, 1, 1, 0)

	path := actorManager.FindPathForCharacterWithOptions(7, 1, 0, 0, 2, 0, nil, pathfindOptions{})
	if len(path) != 2 || path[0] != (PathNode{X: 1, Y: 0}) || path[1] != (PathNode{X: 2, Y: 0}) {
		t.Fatalf("path after cut = %#v, want path through cut tile", path)
	}

	if path := actorManager.FindPathForCharacterWithOptions(8, 1, 0, 0, 2, 0, nil, pathfindOptions{}); len(path) != 0 {
		t.Fatalf("other character path = %#v, want blocked", path)
	}
}

func TestCutTileClearMapRestoresBlockedPath(t *testing.T) {
	wh := &WorldHandler{CutTiles: NewCutTileManager()}
	actorManager := NewPhaserActorManager(wh)
	wh.ActorManager = actorManager
	actorManager.collisionMap[1] = map[string]int{
		"0,0": collisionLand,
		"1,0": collisionBlocked,
		"2,0": collisionLand,
	}

	wh.CutTiles.MarkCut(7, 1, 1, 0)
	if path := actorManager.FindPathForCharacterWithOptions(7, 1, 0, 0, 2, 0, nil, pathfindOptions{}); len(path) == 0 {
		t.Fatal("path after cut was blocked, want path through cut tile")
	}

	wh.CutTiles.ClearMap(7, 1)
	if path := actorManager.FindPathForCharacterWithOptions(7, 1, 0, 0, 2, 0, nil, pathfindOptions{}); len(path) != 0 {
		t.Fatalf("path after map clear = %#v, want blocked", path)
	}
}
