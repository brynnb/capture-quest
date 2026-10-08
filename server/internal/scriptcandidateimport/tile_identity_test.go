package scriptcandidateimport

import "testing"

func TestNativeIdentitySurvivesCatalogRenumberingAndRejectsWrongSource(t *testing.T) {
	data := make([]byte, 16)
	resolver := &tileOverrideResolver{maps: map[string]sourceMapMeta{"ROOM": {TilesetID: 0}}, blocksets: map[int]map[int][]byte{0: {59: data}}, tilesetTiles: map[int]map[int][]byte{0: {0: make([]byte, 16)}}}
	signature, err := renderTileQuadrantSignature(data, 0, 0, resolver.tilesetTiles)
	if err != nil {
		t.Fatal(err)
	}
	resolver.tileImageIDBySignature = map[string]int{signature: 261}
	first, err := resolver.resolveTile("ROOM", 59, 0)
	if err != nil || first != 261 {
		t.Fatalf("first catalog: %d %v", first, err)
	}
	resolver.tileImageIDBySignature[signature] = 50
	second, err := resolver.resolveTile("ROOM", 59, 0)
	if err != nil || second != 50 {
		t.Fatalf("renumbered catalog: %d %v", second, err)
	}
	for _, identity := range []struct {
		mapName    string
		block, pos int
	}{{"MISSING", 59, 0}, {"ROOM", 58, 0}, {"ROOM", 59, 4}} {
		if _, err := resolver.resolveTile(identity.mapName, identity.block, identity.pos); err == nil {
			t.Fatalf("bad native identity accepted: %+v", identity)
		}
	}
	external := &TileIdentityResolver{catalog: map[int][3]int{50: {0, 59, 0}}}
	if err := external.VerifyCatalogIdentity(50, 0, 59, 0); err != nil {
		t.Fatal(err)
	}
	if err := external.VerifyCatalogIdentity(50, 0, 58, 0); err == nil {
		t.Fatal("mismatched runtime catalog accepted")
	}
}
