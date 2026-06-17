package world

import "testing"

func TestFindPathOnCollisionMapBlocksWaterWhenNotSurfing(t *testing.T) {
	collisionMap := map[string]int{
		"0,0": collisionLand,
		"1,0": collisionWater,
		"2,0": collisionLand,
	}

	if got := findPathOnCollisionMap(collisionMap, nil, 0, 0, 2, 0); len(got) != 0 {
		t.Fatalf("path over water without surfing = %#v, want blocked", got)
	}
}

func TestFindPathOnCollisionMapAllowsWaterWhenSurfing(t *testing.T) {
	collisionMap := map[string]int{
		"0,0": collisionLand,
		"1,0": collisionWater,
		"2,0": collisionLand,
	}

	got := findPathOnCollisionMapWithOptions(collisionMap, nil, 0, 0, 2, 0, pathfindOptions{AllowWater: true})
	want := []PathNode{{X: 1, Y: 0}, {X: 2, Y: 0}}
	if len(got) != len(want) {
		t.Fatalf("path length = %d (%#v), want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("path[%d] = %#v, want %#v (full path %#v)", i, got[i], want[i], got)
		}
	}
}
