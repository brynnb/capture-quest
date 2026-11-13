package world

import (
	"capturequest/internal/testdb"
	"context"
	"testing"
)

func TestCutTileAllowsOnlyThatCharacterToPathThroughClearedTree(t *testing.T) {
	wh := &WorldHandler{database: testdb.Postgres(t), CutTiles: NewCutTileManager()}
	actorManager := NewPhaserActorManager(wh)
	wh.ActorManager = actorManager
	actorManager.collisionMap[1] = map[string]int{
		"0,0": collisionLand,
		"1,0": collisionBlocked,
		"2,0": collisionLand,
	}

	if path, err := actorManager.FindPathForCharacterWithOptions(context.Background(), wh.database, 7, 1, 0, 0, 2, 0, nil, pathfindOptions{}); err != nil || len(path) != 0 {
		t.Fatalf("path before cut = %#v, want blocked", path)
	}

	raw := cutTreeReplacementRawFoot
	wh.CutTiles.MarkCut(7, 1, wh.CutTiles.Scope(7, 1), TileEdit{X: 1, Y: 0, TileImageID: 100, CollisionType: collisionLand, RawFootTileID: &raw})

	path, err := actorManager.FindPathForCharacterWithOptions(context.Background(), wh.database, 7, 1, 0, 0, 2, 0, nil, pathfindOptions{})
	if err != nil || len(path) != 2 || path[0] != (PathNode{X: 1, Y: 0}) || path[1] != (PathNode{X: 2, Y: 0}) {
		t.Fatalf("path after cut = %#v, want path through cut tile", path)
	}

	if path, err := actorManager.FindPathForCharacterWithOptions(context.Background(), wh.database, 8, 1, 0, 0, 2, 0, nil, pathfindOptions{}); err != nil || len(path) != 0 {
		t.Fatalf("other character path = %#v, want blocked", path)
	}
}

func TestCutTileClearMapRestoresBlockedPath(t *testing.T) {
	wh := &WorldHandler{database: testdb.Postgres(t), CutTiles: NewCutTileManager()}
	actorManager := NewPhaserActorManager(wh)
	wh.ActorManager = actorManager
	actorManager.collisionMap[1] = map[string]int{
		"0,0": collisionLand,
		"1,0": collisionBlocked,
		"2,0": collisionLand,
	}

	raw := cutTreeReplacementRawFoot
	wh.CutTiles.MarkCut(7, 1, wh.CutTiles.Scope(7, 1), TileEdit{X: 1, Y: 0, TileImageID: 100, CollisionType: collisionLand, RawFootTileID: &raw})
	if path, err := actorManager.FindPathForCharacterWithOptions(context.Background(), wh.database, 7, 1, 0, 0, 2, 0, nil, pathfindOptions{}); err != nil || len(path) == 0 {
		t.Fatal("path after cut was blocked, want path through cut tile")
	}

	wh.CutTiles.ClearMap(7, 1)
	if path, err := actorManager.FindPathForCharacterWithOptions(context.Background(), wh.database, 7, 1, 0, 0, 2, 0, nil, pathfindOptions{}); err != nil || len(path) != 0 {
		t.Fatalf("path after map clear = %#v, want blocked", path)
	}
}

func TestCutScopeCannotRecreateStateAfterResetOrOwnerReplacement(t *testing.T) {
	manager := NewCutTileManager()
	raw := cutTreeReplacementRawFoot
	tile := TileEdit{X: 1, Y: 0, TileImageID: 200, CollisionType: collisionLand, RawFootTileID: &raw}
	original := manager.Scope(42, 50)
	if original == "" || manager.Scope(42, 50) != original {
		t.Fatal("scope is missing or unstable within visit")
	}
	if manager.MarkCut(42, 50, "", tile) || manager.MarkCut(43, 50, original, tile) || manager.MarkCut(42, 51, original, tile) {
		t.Fatal("scope accepted an absent/foreign owner")
	}
	if !manager.MarkCut(42, 50, original, tile) {
		t.Fatal("current owner could not mark")
	}
	manager.ClearMap(42, 50)
	if manager.MarkCut(42, 50, original, tile) || len(manager.CollisionOverrides(42, 50)) != 0 {
		t.Fatal("old scope recreated a cleared cut")
	}
	current := manager.Scope(42, 50)
	if current == original || !manager.MarkCut(42, 50, current, tile) {
		t.Fatal("map reset did not replace scope")
	}
	manager.ClearCharacter(42)
	if manager.MarkCut(42, 50, current, tile) {
		t.Fatal("retired character scope reactivated")
	}
	replacement := NewCutTileManager()
	if replacement.Scope(42, 50) == current || replacement.MarkCut(42, 50, current, tile) {
		t.Fatal("old process scope accepted")
	}
	if len(manager.scopes) != 0 || len(manager.tiles) != 0 {
		t.Fatal("character reset retained unbounded visit data")
	}
}
