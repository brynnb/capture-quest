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
