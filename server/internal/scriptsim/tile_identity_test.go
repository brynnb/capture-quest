package scriptsim

import "testing"

func TestResolvedNativeExpectationStillRequiresImageCollisionAndLabel(t *testing.T) {
	expected := TileState{X: 0, Y: 4, TileImageID: 50, CollisionType: 0, Label: "Closed"}
	for _, state := range []TileState{{X: 0, Y: 4, TileImageID: 261, CollisionType: 0, Label: "Closed"}, {X: 0, Y: 4, TileImageID: 50, CollisionType: 1, Label: "Closed"}, {X: 0, Y: 4, TileImageID: 50, CollisionType: 0, Label: "Open"}} {
		if hasTileState([]TileState{state}, expected) {
			t.Fatalf("weakened tile assertion: %+v", state)
		}
	}
	if !hasTileState([]TileState{expected}, expected) {
		t.Fatal("resolved native expectation rejected")
	}
}
