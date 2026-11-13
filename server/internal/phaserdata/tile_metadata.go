package phaserdata

// RawFootTileIDFromBlockData returns the lower 8x8 tile under the player's
// feet for one 16x16 quadrant of a Gen 1 block.
func RawFootTileIDFromBlockData(blockData []byte, position int) (int, bool) {
	feetIndices := map[int]int{
		0: 4,
		1: 6,
		2: 12,
		3: 14,
	}
	index, ok := feetIndices[position]
	if !ok || index >= len(blockData) {
		return 0, false
	}
	return int(blockData[index]), true
}

// BlocksetTilesetID resolves the extractor's shared graphics identities.
// This mirrors TILESET_BLOCKSET_ALIASES in the authoritative extractor config.
// Catalog rows retain the original identity while shared artwork/block data
// belongs to the source tileset (MART/DOJO/house/gate variants).
func BlocksetTilesetID(id int64) int64 {
	switch id {
	case 2:
		return 6
	case 5:
		return 7
	case 4:
		return 1
	case 9, 10:
		return 12
	default:
		return id
	}
}
