package main

import (
	"database/sql"
	"fmt"
	"log"

	"capturequest/internal/phaserdata"
)

type sqliteMapDimensions struct {
	ID          int
	WidthTiles  int
	HeightTiles int
}

type sqliteWarpPoint struct {
	X int
	Y int
}

type warpActivationClassification struct {
	MapID       int
	MapName     string
	X           int
	Y           int
	IsOverworld bool
	WarpType    string
	Direction   string
}

var doorTileIDsByTileset = map[int]map[int]bool{
	0:  {0x1B: true, 0x58: true},
	2:  {0x5E: true},
	3:  {0x3A: true},
	8:  {0x54: true},
	9:  {0x3B: true},
	10: {0x3B: true},
	12: {0x3B: true},
	13: {0x1E: true},
	18: {0x1C: true, 0x38: true, 0x1A: true},
	19: {0x1A: true, 0x1C: true, 0x53: true},
	20: {0x34: true},
	22: {0x43: true, 0x58: true, 0x1B: true},
	23: {0x3B: true, 0x1B: true},
}

var warpTileIDsByTileset = map[int]map[int]bool{
	0:  {0x1B: true, 0x58: true},
	1:  {0x3B: true, 0x1A: true, 0x1C: true},
	2:  {0x5E: true},
	3:  {0x5A: true, 0x5C: true, 0x3A: true},
	4:  {0x3B: true, 0x1A: true, 0x1C: true},
	5:  {0x4A: true},
	6:  {0x5E: true},
	7:  {0x4A: true},
	8:  {0x54: true, 0x5C: true, 0x32: true},
	9:  {0x3B: true, 0x1A: true, 0x1C: true},
	10: {0x3B: true, 0x1A: true, 0x1C: true},
	11: {0x13: true},
	12: {0x3B: true, 0x1A: true, 0x1C: true},
	13: {0x37: true, 0x39: true, 0x1E: true, 0x4A: true},
	15: {0x1B: true, 0x13: true},
	16: {0x15: true, 0x55: true, 0x04: true},
	17: {0x18: true, 0x1A: true, 0x22: true},
	18: {0x1A: true, 0x1C: true, 0x38: true},
	19: {0x1A: true, 0x1C: true, 0x53: true},
	20: {0x34: true},
	22: {0x43: true, 0x58: true, 0x20: true, 0x1B: true, 0x13: true},
	23: {0x1B: true, 0x3B: true},
}

func classifyWarpActivationsPostgres(sqlite, pg *sql.DB) error {
	log.Println("Classifying warp activation metadata...")
	classifications, err := classifyWarpActivations(sqlite)
	if err != nil {
		return err
	}

	if _, err := pg.Exec(`UPDATE phaser_warps SET warp_type = 'door', warp_direction = NULL`); err != nil {
		return fmt.Errorf("reset phaser_warps activation metadata: %w", err)
	}

	offsets := loadOverworldMapOffsets(sqlite)
	stmt, err := pg.Prepare(`
		UPDATE phaser_warps
		SET warp_type = $1, warp_direction = $2
		WHERE source_map_id = $3 AND x = $4 AND y = $5`)
	if err != nil {
		return fmt.Errorf("prepare warp activation update: %w", err)
	}
	defer stmt.Close()

	doorCount := 0
	carpetCount := 0
	updatedCarpets := int64(0)
	for _, classification := range classifications {
		if classification.WarpType == "door" {
			doorCount++
			continue
		}
		carpetCount++
		x, y := classification.X, classification.Y
		if classification.IsOverworld {
			if offset, ok := offsets[normalizeMapName(classification.MapName)]; ok {
				x += offset.X
				y += offset.Y
			}
		}
		result, err := stmt.Exec(classification.WarpType, classification.Direction, classification.MapID, x, y)
		if err != nil {
			return fmt.Errorf("update warp activation metadata for map %d (%d,%d): %w", classification.MapID, x, y, err)
		}
		affected, _ := result.RowsAffected()
		updatedCarpets += affected
	}

	log.Printf("  -> Classified %d warps: %d door, %d carpet (%d runtime rows updated)",
		len(classifications), doorCount, carpetCount, updatedCarpets)
	return nil
}

func classifyWarpActivations(sqlite *sql.DB) ([]warpActivationClassification, error) {
	mapInfoByName, err := loadSQLiteMapDimensionsByName(sqlite)
	if err != nil {
		return nil, err
	}
	warpPointsByMapID, err := loadSQLiteWarpPointsByMapID(sqlite)
	if err != nil {
		return nil, err
	}

	rows, err := sqlite.Query(`
		SELECT w.map_id, m.name, w.x, w.y, w.dest_map, w.dest_warp_index,
		       tr.tileset_id, tr.block_index, m.width, m.height, m.is_overworld
		FROM warp_events w
		JOIN maps m ON m.id = w.map_id
		JOIN tiles_raw tr ON tr.map_id = w.map_id
			AND tr.x = w.x / 2 AND tr.y = w.y / 2
		ORDER BY w.map_id, w.x, w.y`)
	if err != nil {
		return nil, fmt.Errorf("query warp activation source rows: %w", err)
	}
	defer rows.Close()

	blockDataCache := make(map[string][]byte)
	classifications := make([]warpActivationClassification, 0)
	for rows.Next() {
		var (
			mapID, x, y, destWarpIndex int
			tilesetID, blockIndex      int
			mapWidth, mapHeight        int
			isOverworldInt             int
			mapName, destMap           string
		)
		if err := rows.Scan(
			&mapID,
			&mapName,
			&x,
			&y,
			&destMap,
			&destWarpIndex,
			&tilesetID,
			&blockIndex,
			&mapWidth,
			&mapHeight,
			&isOverworldInt,
		); err != nil {
			return nil, fmt.Errorf("scan warp activation source row: %w", err)
		}

		blockData, ok, err := loadWarpBlockData(sqlite, blockDataCache, tilesetID, blockIndex)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		position := (x % 2) + 2*(y%2)
		footTileID, ok := phaserdata.RawFootTileIDFromBlockData(blockData, position)
		if !ok {
			continue
		}

		classification := warpActivationClassification{
			MapID:       mapID,
			MapName:     mapName,
			X:           x,
			Y:           y,
			IsOverworld: isOverworldInt != 0,
			WarpType:    "door",
		}
		if !isDoorOrWarpFootTile(tilesetID, footTileID) {
			classification.WarpType = "carpet"
			classification.Direction = inferWarpDirection(
				x,
				y,
				mapWidth*2,
				mapHeight*2,
				destMap,
				destWarpIndex,
				mapInfoByName,
				warpPointsByMapID,
			)
		}
		classifications = append(classifications, classification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read warp activation source rows: %w", err)
	}
	return classifications, nil
}

func loadSQLiteMapDimensionsByName(sqlite *sql.DB) (map[string]sqliteMapDimensions, error) {
	rows, err := sqlite.Query(`SELECT id, name, width, height FROM maps`)
	if err != nil {
		return nil, fmt.Errorf("load map dimensions: %w", err)
	}
	defer rows.Close()

	result := make(map[string]sqliteMapDimensions)
	for rows.Next() {
		var id, width, height int
		var name string
		if err := rows.Scan(&id, &name, &width, &height); err != nil {
			return nil, fmt.Errorf("scan map dimensions: %w", err)
		}
		result[normalizeMapName(name)] = sqliteMapDimensions{
			ID:          id,
			WidthTiles:  width * 2,
			HeightTiles: height * 2,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read map dimensions: %w", err)
	}
	return result, nil
}

func loadSQLiteWarpPointsByMapID(sqlite *sql.DB) (map[int][]sqliteWarpPoint, error) {
	rows, err := sqlite.Query(`SELECT map_id, x, y FROM warp_events WHERE map_id IS NOT NULL ORDER BY map_id, id`)
	if err != nil {
		return nil, fmt.Errorf("load warp points: %w", err)
	}
	defer rows.Close()

	result := make(map[int][]sqliteWarpPoint)
	for rows.Next() {
		var mapID, x, y int
		if err := rows.Scan(&mapID, &x, &y); err != nil {
			return nil, fmt.Errorf("scan warp point: %w", err)
		}
		result[mapID] = append(result[mapID], sqliteWarpPoint{X: x, Y: y})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read warp points: %w", err)
	}
	return result, nil
}

func loadWarpBlockData(sqlite *sql.DB, cache map[string][]byte, tilesetID, blockIndex int) ([]byte, bool, error) {
	lookupTilesetID := remapWarpTilesetForBlockData(tilesetID)
	key := fmt.Sprintf("%d:%d", lookupTilesetID, blockIndex)
	if data, ok := cache[key]; ok {
		return data, true, nil
	}

	var data []byte
	err := sqlite.QueryRow(
		`SELECT block_data FROM blocksets WHERE tileset_id = ? AND block_index = ?`,
		lookupTilesetID,
		blockIndex,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load block data for tileset %d block %d: %w", lookupTilesetID, blockIndex, err)
	}
	cache[key] = data
	return data, true, nil
}

func remapWarpTilesetForBlockData(tilesetID int) int {
	switch tilesetID {
	case 5:
		return 7
	case 2:
		return 6
	case 10, 9:
		return 12
	case 4:
		return 1
	default:
		return tilesetID
	}
}

func isDoorOrWarpFootTile(tilesetID, footTileID int) bool {
	if doorTileIDsByTileset[tilesetID][footTileID] {
		return true
	}
	return warpTileIDsByTileset[tilesetID][footTileID]
}

func inferWarpDirection(
	sourceX,
	sourceY,
	sourceWidthTiles,
	sourceHeightTiles int,
	destMapName string,
	destWarpIndex int,
	mapInfoByName map[string]sqliteMapDimensions,
	warpPointsByMapID map[int][]sqliteWarpPoint,
) string {
	if direction, ok := inferWarpDirectionFromSourceMapEdge(sourceX, sourceY, sourceWidthTiles, sourceHeightTiles); ok {
		return direction
	}
	destInfo, ok := mapInfoByName[normalizeMapName(destMapName)]
	if ok && destWarpIndex >= 1 {
		warps := warpPointsByMapID[destInfo.ID]
		if destWarpIndex <= len(warps) {
			point := warps[destWarpIndex-1]
			return inferWarpDirectionFromDestination(point.X, point.Y, destInfo.WidthTiles, destInfo.HeightTiles)
		}
	}
	return inferWarpDirectionFromSourceEdge(sourceX, sourceY, sourceWidthTiles, sourceHeightTiles)
}

func inferWarpDirectionFromSourceMapEdge(sourceX, sourceY, sourceWidthTiles, sourceHeightTiles int) (string, bool) {
	switch {
	case sourceY <= 0:
		return "UP", true
	case sourceY >= sourceHeightTiles-1:
		return "DOWN", true
	case sourceX <= 0:
		return "LEFT", true
	case sourceX >= sourceWidthTiles-1:
		return "RIGHT", true
	default:
		return "", false
	}
}

func inferWarpDirectionFromDestination(destX, destY, destWidthTiles, destHeightTiles int) string {
	distTop := destY
	distBottom := (destHeightTiles - 1) - destY
	distLeft := destX
	distRight := (destWidthTiles - 1) - destX
	minDist := minInt(distTop, distBottom, distLeft, distRight)

	switch minDist {
	case distTop:
		return "DOWN"
	case distBottom:
		return "UP"
	case distLeft:
		return "RIGHT"
	default:
		return "LEFT"
	}
}

func inferWarpDirectionFromSourceEdge(sourceX, sourceY, sourceWidthTiles, sourceHeightTiles int) string {
	if sourceY <= 0 {
		return "UP"
	}
	if sourceY >= sourceHeightTiles-1 {
		return "DOWN"
	}
	if sourceX <= 0 {
		return "LEFT"
	}
	if sourceX >= sourceWidthTiles-1 {
		return "RIGHT"
	}

	distTop := sourceY
	distBottom := (sourceHeightTiles - 1) - sourceY
	distLeft := sourceX
	distRight := (sourceWidthTiles - 1) - sourceX
	minDist := minInt(distTop, distBottom, distLeft, distRight)

	switch minDist {
	case distTop:
		return "UP"
	case distBottom:
		return "DOWN"
	case distLeft:
		return "LEFT"
	default:
		return "RIGHT"
	}
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}
