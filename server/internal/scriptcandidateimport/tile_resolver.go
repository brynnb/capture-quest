package scriptcandidateimport

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"capturequest/internal/phaserdata"
	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

type sourceMapMeta struct {
	ID        int
	Name      string
	TilesetID int
	Overworld bool
}

type coordinateOffset struct {
	X int
	Y int
}

type coordinateResolver struct {
	maps    map[string]sourceMapMeta
	offsets map[string]coordinateOffset
}

func newCoordinateResolver(ctx context.Context, db *sql.DB) (*coordinateResolver, error) {
	exists, err := sqliteTableExists(ctx, db, "maps")
	if err != nil {
		return nil, fmt.Errorf("check maps table: %w", err)
	}
	if !exists {
		return &coordinateResolver{
			maps:    map[string]sourceMapMeta{},
			offsets: map[string]coordinateOffset{},
		}, nil
	}
	maps, err := loadSourceMaps(ctx, db)
	if err != nil {
		return nil, err
	}
	offsets, err := loadOverworldMapOffsets(ctx, db)
	if err != nil {
		return nil, err
	}
	return &coordinateResolver{maps: maps, offsets: offsets}, nil
}

func (resolver *coordinateResolver) Normalize(coords []scriptedevents.EventCoordinate, mapName string) []scriptedevents.EventCoordinate {
	if len(coords) == 0 {
		return nil
	}
	normalized := make([]scriptedevents.EventCoordinate, len(coords))
	for i, coord := range coords {
		normalized[i] = coord
		if normalized[i].MapName == "" {
			normalized[i].MapName = mapName
		}
		coordMapName := mapNameToUpperSnake(normalized[i].MapName)
		if coordMapName == "" {
			coordMapName = mapName
		}
		normalized[i].MapName = coordMapName
		if resolver == nil {
			continue
		}
		meta, ok := resolver.maps[coordMapName]
		if !ok || !meta.Overworld {
			if normalized[i].MapID == 0 && ok {
				normalized[i].MapID = meta.ID
			}
			continue
		}
		if offset, ok := resolver.offsets[coordMapName]; ok {
			normalized[i].X += offset.X
			normalized[i].Y += offset.Y
			normalized[i].MapID = 9999
			normalized[i].MapName = coordMapName
		}
	}
	return normalized
}

type tileOverrideResolver struct {
	maps                   map[string]sourceMapMeta
	blocksets              map[int]map[int][]byte
	tilesetTiles           map[int]map[int][]byte
	collisionTiles         map[int]map[int]bool
	tileImageIDBySignature map[string]int
}

func newTileOverrideResolver(ctx context.Context, db *sql.DB) (*tileOverrideResolver, error) {
	for _, table := range []string{"maps", "blocksets", "tile_images", "tileset_tiles"} {
		exists, err := sqliteTableExists(ctx, db, table)
		if err != nil {
			return nil, fmt.Errorf("check %s table: %w", table, err)
		}
		if !exists {
			return nil, fmt.Errorf("missing required %s table", table)
		}
	}

	resolver := &tileOverrideResolver{}
	var err error
	if resolver.maps, err = loadSourceMaps(ctx, db); err != nil {
		return nil, err
	}
	if resolver.blocksets, err = loadSourceBlocksets(ctx, db); err != nil {
		return nil, err
	}
	if resolver.tilesetTiles, err = loadSourceTilesetTiles(ctx, db); err != nil {
		return nil, err
	}
	if resolver.collisionTiles, err = loadSourceCollisionTiles(ctx, db); err != nil {
		return nil, err
	}
	if resolver.tileImageIDBySignature, err = loadTileImageSignatures(ctx, db, resolver.tilesetTiles); err != nil {
		return nil, err
	}
	return resolver, nil
}

func (resolver *tileOverrideResolver) MapCandidate(candidate tileOverrideCandidate) ([]scriptedevents.EventTileOverrideRule, error) {
	if candidate.Kind != "" && candidate.Kind != "eventTileOverrideCandidate" {
		return nil, fmt.Errorf("unsupported tile override kind %q", candidate.Kind)
	}
	if candidate.MapName == "" || candidate.ScriptLabel == "" {
		return nil, fmt.Errorf("tile override candidate missing mapName or scriptLabel")
	}
	if len(candidate.Replacements) == 0 {
		return nil, fmt.Errorf("tile override candidate has no replacements")
	}

	mapName := mapNameToUpperSnake(candidate.MapName)
	meta, ok := resolver.maps[mapName]
	if !ok {
		return nil, fmt.Errorf("unknown map %s", mapName)
	}
	blocksetID := sourceBlocksetTilesetID(meta.TilesetID)
	rules := []scriptedevents.EventTileOverrideRule{}
	for _, replacement := range candidate.Replacements {
		if replacement.LabelPrefix == "" {
			return nil, fmt.Errorf("%s replacement missing labelPrefix", candidate.ScriptLabel)
		}
		requiresFlag := mapEventName(replacement.RequiresEvent)
		requiresFlagAbsent := mapEventName(replacement.RequiresEventAbsent)
		if requiresFlag != "" && requiresFlagAbsent != "" {
			return nil, fmt.Errorf("%s replacement %s has both requiresEvent and requiresEventAbsent", candidate.ScriptLabel, replacement.LabelPrefix)
		}
		blockData := resolver.blocksets[blocksetID][replacement.BlockID]
		if len(blockData) == 0 {
			return nil, fmt.Errorf("%s replacement %s references missing block %d for tileset %d", candidate.ScriptLabel, replacement.LabelPrefix, replacement.BlockID, blocksetID)
		}
		for position := 0; position < 4; position++ {
			signature, err := renderTileQuadrantSignature(blockData, position, blocksetID, resolver.tilesetTiles)
			if err != nil {
				return nil, fmt.Errorf("%s replacement %s position %d: %w", candidate.ScriptLabel, replacement.LabelPrefix, position, err)
			}
			tileImageID := resolver.tileImageIDBySignature[signature]
			if tileImageID == 0 {
				return nil, fmt.Errorf("%s replacement %s position %d has no matching tile image", candidate.ScriptLabel, replacement.LabelPrefix, position)
			}
			dx := position % 2
			dy := position / 2
			rules = append(rules, scriptedevents.EventTileOverrideRule{
				MapID:              meta.ID,
				MapName:            meta.Name,
				X:                  replacement.BlockX*2 + dx,
				Y:                  replacement.BlockY*2 + dy,
				TileImageID:        tileImageID,
				CollisionType:      resolver.collisionType(blockData, position, meta.TilesetID),
				RequiresFlag:       requiresFlag,
				RequiresFlagAbsent: requiresFlagAbsent,
				Label:              fmt.Sprintf("%s_%d_%d", replacement.LabelPrefix, dx, dy),
			})
		}
	}
	return rules, nil
}

func loadSourceMaps(ctx context.Context, db *sql.DB) (map[string]sourceMapMeta, error) {
	hasOverworld, err := sqliteColumnExists(ctx, db, "maps", "is_overworld")
	if err != nil {
		return nil, fmt.Errorf("check maps.is_overworld: %w", err)
	}
	query := `SELECT id, name, tileset_id, 0 FROM maps`
	if hasOverworld {
		query = `SELECT id, name, tileset_id, is_overworld FROM maps`
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query maps: %w", err)
	}
	defer rows.Close()

	result := map[string]sourceMapMeta{}
	for rows.Next() {
		var meta sourceMapMeta
		var tilesetID sql.NullInt64
		var isOverworld sql.NullInt64
		if err := rows.Scan(&meta.ID, &meta.Name, &tilesetID, &isOverworld); err != nil {
			return nil, fmt.Errorf("scan map: %w", err)
		}
		if tilesetID.Valid {
			meta.TilesetID = int(tilesetID.Int64)
		}
		meta.Overworld = isOverworld.Valid && isOverworld.Int64 != 0
		result[mapNameToUpperSnake(meta.Name)] = meta
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read maps: %w", err)
	}
	return result, nil
}

func loadOverworldMapOffsets(ctx context.Context, db *sql.DB) (map[string]coordinateOffset, error) {
	exists, err := sqliteTableExists(ctx, db, "overworld_map_positions")
	if err != nil {
		return nil, fmt.Errorf("check overworld_map_positions table: %w", err)
	}
	if !exists {
		return map[string]coordinateOffset{}, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT map_name, x_offset, y_offset FROM overworld_map_positions`)
	if err != nil {
		return nil, fmt.Errorf("query overworld map offsets: %w", err)
	}
	defer rows.Close()

	result := map[string]coordinateOffset{}
	for rows.Next() {
		var mapName string
		var x, y int
		if err := rows.Scan(&mapName, &x, &y); err != nil {
			return nil, fmt.Errorf("scan overworld map offset: %w", err)
		}
		result[mapNameToUpperSnake(mapName)] = coordinateOffset{X: x, Y: y}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read overworld map offsets: %w", err)
	}
	return result, nil
}

func loadSourceBlocksets(ctx context.Context, db *sql.DB) (map[int]map[int][]byte, error) {
	rows, err := db.QueryContext(ctx, `SELECT tileset_id, block_index, block_data FROM blocksets`)
	if err != nil {
		return nil, fmt.Errorf("query blocksets: %w", err)
	}
	defer rows.Close()

	result := map[int]map[int][]byte{}
	for rows.Next() {
		var tilesetID, blockIndex int
		var blockData []byte
		if err := rows.Scan(&tilesetID, &blockIndex, &blockData); err != nil {
			return nil, fmt.Errorf("scan blockset: %w", err)
		}
		if result[tilesetID] == nil {
			result[tilesetID] = map[int][]byte{}
		}
		result[tilesetID][blockIndex] = blockData
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read blocksets: %w", err)
	}
	return result, nil
}

func loadSourceTilesetTiles(ctx context.Context, db *sql.DB) (map[int]map[int][]byte, error) {
	rows, err := db.QueryContext(ctx, `SELECT tileset_id, tile_index, tile_data FROM tileset_tiles`)
	if err != nil {
		return nil, fmt.Errorf("query tileset_tiles: %w", err)
	}
	defer rows.Close()

	result := map[int]map[int][]byte{}
	for rows.Next() {
		var tilesetID, tileIndex int
		var tileData []byte
		if err := rows.Scan(&tilesetID, &tileIndex, &tileData); err != nil {
			return nil, fmt.Errorf("scan tileset tile: %w", err)
		}
		if result[tilesetID] == nil {
			result[tilesetID] = map[int][]byte{}
		}
		result[tilesetID][tileIndex] = tileData
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tileset tiles: %w", err)
	}
	return result, nil
}

func loadSourceCollisionTiles(ctx context.Context, db *sql.DB) (map[int]map[int]bool, error) {
	exists, err := sqliteTableExists(ctx, db, "collision_tiles")
	if err != nil {
		return nil, fmt.Errorf("check collision_tiles table: %w", err)
	}
	result := map[int]map[int]bool{}
	if !exists {
		return result, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT tileset_id, tile_id FROM collision_tiles`)
	if err != nil {
		return nil, fmt.Errorf("query collision_tiles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tilesetID, tileID int
		if err := rows.Scan(&tilesetID, &tileID); err != nil {
			return nil, fmt.Errorf("scan collision tile: %w", err)
		}
		if result[tilesetID] == nil {
			result[tilesetID] = map[int]bool{}
		}
		result[tilesetID][tileID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read collision tiles: %w", err)
	}
	return result, nil
}

func loadTileImageSignatures(ctx context.Context, db *sql.DB, tilesetTiles map[int]map[int][]byte) (map[string]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT ti.id, ti.tileset_id, ti.position, bs.block_data
		FROM tile_images ti
		JOIN blocksets bs
		  ON bs.tileset_id = ti.tileset_id AND bs.block_index = ti.block_index
		ORDER BY ti.id`)
	if err != nil {
		return nil, fmt.Errorf("query tile image signatures: %w", err)
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var id, tilesetID, position int
		var blockData []byte
		if err := rows.Scan(&id, &tilesetID, &position, &blockData); err != nil {
			return nil, fmt.Errorf("scan tile image signature: %w", err)
		}
		signature, err := renderTileQuadrantSignature(blockData, position, tilesetID, tilesetTiles)
		if err != nil {
			return nil, fmt.Errorf("tile image %d: %w", id, err)
		}
		if result[signature] == 0 {
			result[signature] = id
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tile image signatures: %w", err)
	}
	return result, nil
}

func (resolver *tileOverrideResolver) collisionType(blockData []byte, position int, rawTilesetID int) int {
	for _, index := range quadrantSubTileIndices(position) {
		if index < len(blockData) && (blockData[index] == 0x14 || blockData[index] == 0x48) {
			return 2
		}
	}
	footTileID, ok := phaserdata.RawFootTileIDFromBlockData(blockData, position)
	if !ok {
		return 0
	}
	if resolver.collisionTiles[rawTilesetID][footTileID] {
		return 1
	}
	return 0
}

func renderTileQuadrantSignature(blockData []byte, position int, tilesetID int, tilesetTiles map[int]map[int][]byte) (string, error) {
	indices := quadrantSubTileIndices(position)
	if len(indices) == 0 {
		return "", fmt.Errorf("invalid block quadrant position %d", position)
	}
	pixels := make([]byte, 16*16)
	for i, blockIndex := range indices {
		if blockIndex >= len(blockData) {
			return "", fmt.Errorf("block data has %d bytes, need index %d", len(blockData), blockIndex)
		}
		tileID := int(blockData[blockIndex])
		tileData := tilesetTiles[tilesetID][tileID]
		if len(tileData) < 16 {
			return "", fmt.Errorf("missing 2bpp tile data for tileset %d tile %#x", tilesetID, tileID)
		}
		offsetX := (i % 2) * 8
		offsetY := (i / 2) * 8
		writeDecodedTilePixels(pixels, offsetX, offsetY, tileData)
	}
	return string(pixels), nil
}

func writeDecodedTilePixels(dest []byte, offsetX, offsetY int, tileData []byte) {
	for row := 0; row < 8; row++ {
		lo := tileData[row*2]
		hi := tileData[row*2+1]
		for bit := 0; bit < 8; bit++ {
			shift := 7 - bit
			value := ((hi >> shift) & 1) << 1
			value |= (lo >> shift) & 1
			dest[(offsetY+row)*16+offsetX+bit] = value
		}
	}
}

func quadrantSubTileIndices(position int) []int {
	switch position {
	case 0:
		return []int{0, 1, 4, 5}
	case 1:
		return []int{2, 3, 6, 7}
	case 2:
		return []int{8, 9, 12, 13}
	case 3:
		return []int{10, 11, 14, 15}
	default:
		return nil
	}
}

func sourceBlocksetTilesetID(tilesetID int) int {
	switch tilesetID {
	case 2:
		return 6
	case 5:
		return 7
	default:
		return tilesetID
	}
}

func sortEventTileRulesForImport(rules []scriptedevents.EventTileOverrideRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].MapID != rules[j].MapID {
			return rules[i].MapID < rules[j].MapID
		}
		if rules[i].MapName != rules[j].MapName {
			return rules[i].MapName < rules[j].MapName
		}
		if rules[i].X != rules[j].X {
			return rules[i].X < rules[j].X
		}
		if rules[i].Y != rules[j].Y {
			return rules[i].Y < rules[j].Y
		}
		if rules[i].RequiresFlag != rules[j].RequiresFlag {
			return rules[i].RequiresFlag < rules[j].RequiresFlag
		}
		if rules[i].RequiresFlagAbsent != rules[j].RequiresFlagAbsent {
			return rules[i].RequiresFlagAbsent < rules[j].RequiresFlagAbsent
		}
		return rules[i].Label < rules[j].Label
	})
}

func eventTileRuleKeyForImport(rule scriptedevents.EventTileOverrideRule) string {
	return fmt.Sprintf("%d|%s|%d|%d|%s|%s", rule.MapID, rule.MapName, rule.X, rule.Y, rule.RequiresFlag, rule.RequiresFlagAbsent)
}
