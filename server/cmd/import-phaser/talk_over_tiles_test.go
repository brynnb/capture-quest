package main

import (
	"database/sql"
	"testing"
)

func TestIsTalkOverTileUsesOriginalTilesetCounters(t *testing.T) {
	tests := []struct {
		name     string
		tileset  int64
		rawFoot  sql.NullInt64
		expected bool
	}{
		{
			name:     "mart counter",
			tileset:  2,
			rawFoot:  sql.NullInt64{Int64: 0x1e, Valid: true},
			expected: true,
		},
		{
			name:     "same raw foot is not global",
			tileset:  0,
			rawFoot:  sql.NullInt64{Int64: 0x1e, Valid: true},
			expected: false,
		},
		{
			name:     "ordinary mart floor",
			tileset:  2,
			rawFoot:  sql.NullInt64{Int64: 0x1b, Valid: true},
			expected: false,
		},
		{
			name:     "unknown raw foot",
			tileset:  2,
			rawFoot:  sql.NullInt64{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTalkOverTile(tt.tileset, tt.rawFoot); got != tt.expected {
				t.Fatalf("isTalkOverTile(%d, %#x valid=%v) = %v, want %v",
					tt.tileset, tt.rawFoot.Int64, tt.rawFoot.Valid, got, tt.expected)
			}
		})
	}
}

func TestRawFootTileIDForPlacedTileUsesMapBlockPosition(t *testing.T) {
	// This mirrors the Bike Shop left-side counter: the placed CLUB block has
	// raw foot tile $07 at local tile (5,2), even if its rendered 16x16 image
	// is de-duplicated with a non-counter tile image elsewhere.
	blockData := []byte{
		0x0f, 0x1f, 0x07, 0x08,
		0x1f, 0x0f, 0x07, 0x08,
		0x0f, 0x1f, 0x07, 0x08,
		0x1f, 0x0f, 0x17, 0x18,
	}
	mapMeta := sqliteMapBlockMetadata{
		Width:     4,
		Height:    4,
		TilesetID: 21,
		BlkData: []byte{
			0x04, 0x05, 0x02, 0x03,
			0x08, 0x08, 0x06, 0x07,
			0x0b, 0x0a, 0x0a, 0x0a,
			0x0a, 0x01, 0x0a, 0x09,
		},
	}
	blocksets := map[int64]map[int64][]byte{
		21: {
			0x06: blockData,
		},
	}

	raw, ok := rawFootTileIDForPlacedTile(mapMeta, blocksets, 5, 2)
	if !ok {
		t.Fatal("rawFootTileIDForPlacedTile returned ok=false")
	}
	if raw != 0x07 {
		t.Fatalf("rawFootTileIDForPlacedTile Bike Shop (5,2) = %#x, want 0x07", raw)
	}
	if !isTalkOverTile(21, sql.NullInt64{Int64: int64(raw), Valid: true}) {
		t.Fatalf("Bike Shop (5,2) raw foot %#x should be a CLUB talk-over tile", raw)
	}
}

func TestBlocksetTilesetIDRemapsSharedGraphicsTilesets(t *testing.T) {
	tests := []struct {
		tilesetID int64
		want      int64
	}{
		{tilesetID: 2, want: 6},
		{tilesetID: 5, want: 7},
		{tilesetID: 21, want: 21},
	}

	for _, tt := range tests {
		if got := blocksetTilesetID(tt.tilesetID); got != tt.want {
			t.Fatalf("blocksetTilesetID(%d) = %d, want %d", tt.tilesetID, got, tt.want)
		}
	}
}
