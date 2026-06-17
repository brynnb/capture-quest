package world

import "testing"

func TestFindPathOnCollisionMapAllowsSouthLedgeJump(t *testing.T) {
	collisionMap := map[string]int{
		tileKey(10, -46): 1,
		tileKey(10, -45): 0,
		tileKey(10, -44): 1,
	}
	rawFootTileMap := map[string]int{
		tileKey(10, -46): 0x2c,
		tileKey(10, -45): 0x37,
	}

	got := findPathOnCollisionMap(collisionMap, rawFootTileMap, 10, -46, 10, -44)
	want := []PathNode{{X: 10, Y: -44}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("findPathOnCollisionMap() = %v, want %v", got, want)
	}
}

func TestFindPathOnCollisionMapAllowsViridianSouthLedgePair(t *testing.T) {
	collisionMap := map[string]int{
		tileKey(8, -46): 1,
		tileKey(8, -45): 0,
		tileKey(8, -44): 1,
	}
	rawFootTileMap := map[string]int{
		tileKey(8, -46): 0x2c,
		tileKey(8, -45): 0x37,
	}

	got := findPathOnCollisionMap(collisionMap, rawFootTileMap, 8, -46, 8, -44)
	want := []PathNode{{X: 8, Y: -44}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("findPathOnCollisionMap() = %v, want %v", got, want)
	}
}

func TestFindPathOnCollisionMapBlocksReverseSouthLedgeJump(t *testing.T) {
	collisionMap := map[string]int{
		tileKey(10, -46): 1,
		tileKey(10, -45): 0,
		tileKey(10, -44): 1,
	}
	rawFootTileMap := map[string]int{
		tileKey(10, -44): 0x2c,
		tileKey(10, -45): 0x37,
	}

	if got := findPathOnCollisionMap(collisionMap, rawFootTileMap, 10, -44, 10, -46); len(got) != 0 {
		t.Fatalf("reverse ledge path = %v, want no path", got)
	}
}

func TestFindPathOnCollisionMapRequiresLedgeLandingTile(t *testing.T) {
	collisionMap := map[string]int{
		tileKey(10, -46): 1,
		tileKey(10, -45): 0,
		tileKey(10, -44): 0,
	}
	rawFootTileMap := map[string]int{
		tileKey(10, -46): 0x2c,
		tileKey(10, -45): 0x37,
	}

	if got := findPathOnCollisionMap(collisionMap, rawFootTileMap, 10, -46, 10, -44); len(got) != 0 {
		t.Fatalf("ledge path onto blocked landing = %v, want no path", got)
	}
}

func TestFindPathOnCollisionMapDoesNotJumpFuchsiaDecorativeRockEdges(t *testing.T) {
	collisionMap := map[string]int{
		tileKey(131, 40): 1,
		tileKey(131, 41): 0,
		tileKey(131, 42): 1,
		tileKey(132, 40): 1,
		tileKey(133, 40): 0,
		tileKey(134, 40): 1,
	}
	rawFootTileMap := map[string]int{
		tileKey(131, 40): 0x39,
		tileKey(131, 41): 0x55,
		tileKey(132, 40): 0x2c,
		tileKey(133, 40): 0x50,
	}

	if got := findPathOnCollisionMap(collisionMap, rawFootTileMap, 131, 40, 131, 42); len(got) != 0 {
		t.Fatalf("Fuchsia decorative down edge path = %v, want no path", got)
	}
	if got := findPathOnCollisionMap(collisionMap, rawFootTileMap, 132, 40, 134, 40); len(got) != 0 {
		t.Fatalf("Fuchsia decorative right edge path = %v, want no path", got)
	}
}

func TestCanJumpLedgeIncludesOriginalLeftAndRightPairs(t *testing.T) {
	if !canJumpLedge("LEFT", 0x2c, 0x27) {
		t.Fatal("expected original left ledge pair to be jumpable")
	}
	if !canJumpLedge("RIGHT", 0x2c, 0x1d) {
		t.Fatal("expected original right ledge pair to be jumpable")
	}
	if canJumpLedge("UP", 0x2c, 0x37) {
		t.Fatal("upward ledge jump should not be allowed")
	}
}
