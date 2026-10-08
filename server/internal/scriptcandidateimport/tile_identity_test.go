package scriptcandidateimport

import (
	"context"
	"database/sql"
	"testing"
)

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

func TestCatalogSignaturesIncludeSharedGraphicsAliasRows(t *testing.T) {
	source, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	if _, err = source.Exec(`CREATE TABLE tile_images(id INTEGER,tileset_id INTEGER,block_index INTEGER,position INTEGER); INSERT INTO tile_images VALUES(77,5,49,0)`); err != nil {
		t.Fatal(err)
	}
	blocks := map[int]map[int][]byte{7: {49: make([]byte, 16)}}
	tiles := map[int]map[int][]byte{7: {0: make([]byte, 16)}}
	catalog, err := loadTileImageSignatures(context.Background(), source, blocks, tiles)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := renderTileQuadrantSignature(blocks[7][49], 0, 7, tiles)
	if err != nil {
		t.Fatal(err)
	}
	if catalog[signature] != 77 {
		t.Fatal("DOJO catalog row dropped because its data belongs to GYM")
	}
}

func TestSourceCoordinateResolverSharesTranslationAndRejectsMissingOffsets(t *testing.T) {
	resolver := &TileIdentityResolver{coordinates: &coordinateResolver{maps: map[string]sourceMapMeta{"ROUTE_23": {ID: 34, Overworld: true}, "ROOM": {ID: 38}}, offsets: map[string]coordinateOffset{"ROUTE_23": {X: -50, Y: -208}}}}
	x, y, err := resolver.ResolveCoordinate("ROUTE_23", 8, 136)
	if err != nil || x != -42 || y != -72 {
		t.Fatalf("source trigger translation: %d,%d %v", x, y, err)
	}
	x, y, err = resolver.ResolveCoordinate("ROOM", 3, 6)
	if err != nil || x != 3 || y != 6 {
		t.Fatal("interior coordinate changed")
	}
	delete(resolver.coordinates.offsets, "ROUTE_23")
	if _, _, err = resolver.ResolveCoordinate("ROUTE_23", 8, 136); err == nil {
		t.Fatal("missing native offset guessed")
	}
	if _, _, err = resolver.ResolveCoordinate("UNKNOWN", 0, 0); err == nil {
		t.Fatal("unknown source map accepted")
	}
}
