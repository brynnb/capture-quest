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

type sqliteTileCollisionMap map[int]map[string]int

type warpActivationClassification struct {
	MapID       int
	MapName     string
	X           int
	Y           int
	IsOverworld bool
	WarpType    string
	Direction   string
}

const (
	warpTypeDoor     = "door"
	warpTypeCarpet   = "carpet"
	warpTypeInactive = "inactive"
)

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

var warpTileInFrontExtraCheckMaps = map[string]bool{
	"rocktunnel1f":     true,
	"rockethideoutb1f": true,
	"rockethideoutb2f": true,
	"rockethideoutb4f": true,
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
	inactiveCount := 0
	updatedRows := int64(0)
	for _, classification := range classifications {
		if classification.WarpType == warpTypeDoor {
			doorCount++
			continue
		}
		switch classification.WarpType {
		case warpTypeCarpet:
			carpetCount++
		case warpTypeInactive:
			inactiveCount++
		}
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
		updatedRows += affected
	}

	log.Printf("  -> Classified %d warps: %d door, %d carpet, %d inactive (%d runtime rows updated)",
		len(classifications), doorCount, carpetCount, inactiveCount, updatedRows)
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
	tileCollisionsByMapID, err := loadSQLiteTileCollisionsByMapID(sqlite)
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
			WarpType:    warpTypeDoor,
		}
		if !isDoorOrWarpFootTile(tilesetID, footTileID) {
			sourceWidthTiles := mapWidth * 2
			sourceHeightTiles := mapHeight * 2
			if usesEdgeExtraWarpCheck(mapName, tilesetID) && !isWarpPointOnMapEdge(x, y, sourceWidthTiles, sourceHeightTiles) {
				classification.WarpType = warpTypeInactive
			} else {
				classification.WarpType = warpTypeCarpet
				classification.Direction = inferWarpDirection(
					x,
					y,
					sourceWidthTiles,
					sourceHeightTiles,
					destMap,
					destWarpIndex,
					tileCollisionsByMapID[mapID],
					mapInfoByName,
					warpPointsByMapID,
				)
			}
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

func loadSQLiteTileCollisionsByMapID(sqlite *sql.DB) (sqliteTileCollisionMap, error) {
	rows, err := sqlite.Query(`
		SELECT map_id, COALESCE(local_x, x), COALESCE(local_y, y), collision_type
		FROM tiles
		WHERE map_id IS NOT NULL`)
	if err != nil {
		return nil, fmt.Errorf("load tile collision maps: %w", err)
	}
	defer rows.Close()

	result := make(sqliteTileCollisionMap)
	for rows.Next() {
		var mapID, x, y, collisionType int
		if err := rows.Scan(&mapID, &x, &y, &collisionType); err != nil {
			return nil, fmt.Errorf("scan tile collision map: %w", err)
		}
		if result[mapID] == nil {
			result[mapID] = make(map[string]int)
		}
		result[mapID][fmt.Sprintf("%d,%d", x, y)] = collisionType
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tile collision maps: %w", err)
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

func usesEdgeExtraWarpCheck(mapName string, tilesetID int) bool {
	normalizedMapName := normalizeMapName(mapName)
	if normalizedMapName == "ssanne3f" {
		return true
	}
	if warpTileInFrontExtraCheckMaps[normalizedMapName] {
		return false
	}
	switch tilesetID {
	case 0, 13, 14, 23:
		return false
	default:
		return true
	}
}

func isWarpPointOnMapEdge(x, y, widthTiles, heightTiles int) bool {
	return x <= 0 || y <= 0 || x >= widthTiles-1 || y >= heightTiles-1
}

func inferWarpDirection(
	sourceX,
	sourceY,
	sourceWidthTiles,
	sourceHeightTiles int,
	destMapName string,
	destWarpIndex int,
	sourceCollisions map[string]int,
	mapInfoByName map[string]sqliteMapDimensions,
	warpPointsByMapID map[int][]sqliteWarpPoint,
) string {
	if direction, ok := inferWarpDirectionFromSourceMapEdge(sourceX, sourceY, sourceWidthTiles, sourceHeightTiles); ok {
		return direction
	}
	if direction, ok := inferWarpDirectionFromSourceWalkability(sourceX, sourceY, sourceCollisions); ok {
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

func inferWarpDirectionFromSourceWalkability(sourceX, sourceY int, sourceCollisions map[string]int) (string, bool) {
	if len(sourceCollisions) == 0 {
		return "", false
	}
	sourceCollision, sourceExists := sourceCollisions[fmt.Sprintf("%d,%d", sourceX, sourceY)]
	if !sourceExists || sourceCollision == 1 {
		return "", false
	}

	candidates := []struct {
		x         int
		y         int
		direction string
	}{
		{x: sourceX, y: sourceY + 1, direction: "UP"},
		{x: sourceX, y: sourceY - 1, direction: "DOWN"},
		{x: sourceX - 1, y: sourceY, direction: "RIGHT"},
		{x: sourceX + 1, y: sourceY, direction: "LEFT"},
	}
	for _, candidate := range candidates {
		if sourceCollisions[fmt.Sprintf("%d,%d", candidate.x, candidate.y)] == 1 {
			return candidate.direction, true
		}
	}
	return "", false
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
