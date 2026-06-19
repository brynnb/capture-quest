package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

const testCollisionLand = 1

type warpIntegrityMap struct {
	ID          int
	Name        string
	IsOverworld bool
}

type warpIntegrityTile struct {
	X         int
	Y         int
	Collision int
}

type warpIntegrityRow struct {
	ID               int
	SourceMapID      int
	SourceMapName    string
	X                int
	Y                int
	DestinationMapID int
	DestinationMap   string
	HasDestination   bool
}

type warpIntegrityResolved struct {
	Warp warpIntegrityRow
	X    int
	Y    int
}

func TestAllWarpDestinationsLandInBoundsAndCanMove(t *testing.T) {
	db := openImportPokemonSQLiteForTest(t)
	defer db.Close()

	maps := loadWarpIntegrityMaps(t, db)
	tiles := loadWarpIntegrityTiles(t, db)
	offsets := loadOverworldMapOffsets(db)
	events := loadWarpIntegrityEvents(t, db, maps)
	warps := loadWarpIntegrityWarps(t, db)
	resolveWarpIntegrityLastMapDestinations(events, warps)

	runtimeWarps := make([]importedRuntimeWarp, 0, len(warps))
	for _, warp := range warps {
		if !warp.HasDestination {
			continue
		}
		runtimeWarps = append(runtimeWarps, importedRuntimeWarp{
			ID:               warp.ID,
			SourceMapID:      warp.SourceMapID,
			X:                warp.X,
			Y:                warp.Y,
			DestinationMapID: warp.DestinationMapID,
			HasDestination:   true,
		})
	}

	importedMaps := make(map[int]importedMapInfo, len(maps))
	for id, info := range maps {
		importedMaps[id] = importedMapInfo{Name: info.Name, IsOverworld: info.IsOverworld}
	}
	updates := resolveWarpDestinationUpdates(importedMaps, offsets, events, runtimeWarps)
	resolvedByWarpID := make(map[int]warpDestinationUpdate, len(updates))
	for _, update := range updates {
		resolvedByWarpID[update.WarpID] = update
	}
	sourceEvents := sourceEventsByCoordinate(importedMaps, offsets, events)

	var failures []string
	skippedSpecial := 0
	for _, warp := range warps {
		sourceEvent, hasSourceEvent := sourceEvents[warpCoordKey(warp.SourceMapID, warp.X, warp.Y)]
		if isSpecialWarpOutsideGenericValidation(warp, sourceEvent, hasSourceEvent) {
			skippedSpecial++
			continue
		}
		sourceTileMap := tiles[warp.SourceMapID]
		if _, ok := sourceTileMap[coordKey(warp.X, warp.Y)]; !ok {
			failures = append(failures, fmt.Sprintf("warp %d %s(%d,%d) source tile missing", warp.ID, warp.SourceMapName, warp.X, warp.Y))
			continue
		}
		if !hasWarpActivationTile(sourceTileMap, warp.X, warp.Y) {
			failures = append(failures, fmt.Sprintf("warp %d %s(%d,%d) has no source tile/neighbor a client can path to", warp.ID, warp.SourceMapName, warp.X, warp.Y))
		}

		if !warp.HasDestination {
			failures = append(failures, fmt.Sprintf("warp %d %s(%d,%d) unresolved destination %q", warp.ID, warp.SourceMapName, warp.X, warp.Y, warp.DestinationMap))
			continue
		}
		update, ok := resolvedByWarpID[warp.ID]
		if !ok {
			failures = append(failures, fmt.Sprintf("warp %d %s(%d,%d)->%s did not resolve destination coordinates", warp.ID, warp.SourceMapName, warp.X, warp.Y, warp.DestinationMap))
			continue
		}
		destinationTiles := tiles[warp.DestinationMapID]
		destinationTile, ok := destinationTiles[coordKey(update.X, update.Y)]
		if !ok {
			failures = append(failures, fmt.Sprintf("warp %d %s(%d,%d)->%s(%d,%d) lands off-map", warp.ID, warp.SourceMapName, warp.X, warp.Y, warp.DestinationMap, update.X, update.Y))
			continue
		}
		if !hasAdjacentLandTile(destinationTiles, update.X, update.Y) {
			failures = append(failures, fmt.Sprintf("warp %d %s(%d,%d)->%s(%d,%d collision=%d) leaves player unable to move one tile by keyboard/click", warp.ID, warp.SourceMapName, warp.X, warp.Y, warp.DestinationMap, update.X, update.Y, destinationTile.Collision))
		}
	}

	if len(failures) > 0 {
		sort.Strings(failures)
		const maxShown = 50
		if len(failures) > maxShown {
			t.Fatalf("%d warp integrity failures; first %d:\n%s", len(failures), maxShown, strings.Join(failures[:maxShown], "\n"))
		}
		t.Fatalf("%d warp integrity failures:\n%s", len(failures), strings.Join(failures, "\n"))
	}
	t.Logf("validated %d ordinary warps; skipped %d special/scripted/dynamic warps", len(warps)-skippedSpecial, skippedSpecial)
}

func openImportPokemonSQLiteForTest(t *testing.T) *sql.DB {
	t.Helper()
	for _, root := range candidateRepoRoots(t) {
		candidate := filepath.Join(root, "public", "phaser", "pokemon.db")
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		db, err := sql.Open("sqlite", candidate)
		if err != nil {
			continue
		}
		if err := db.Ping(); err == nil && sqliteColumnExists(db, "warp_events", "map_id") {
			return db
		}
		db.Close()
	}
	t.Fatal("could not open public/phaser/pokemon.db")
	return nil
}

func candidateRepoRoots(t *testing.T) []string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	roots := []string{wd}
	for dir := wd; ; {
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		roots = append(roots, parent)
		if _, err := os.Stat(filepath.Join(parent, "package.json")); err == nil {
			break
		}
		dir = parent
	}
	return roots
}

func sqliteColumnExists(db *sql.DB, tableName, columnName string) bool {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false
		}
		if name == columnName {
			return true
		}
	}
	return false
}

func loadWarpIntegrityMaps(t *testing.T, db *sql.DB) map[int]warpIntegrityMap {
	t.Helper()
	rows, err := db.Query(`SELECT id, name, is_overworld FROM maps`)
	if err != nil {
		t.Fatalf("query maps: %v", err)
	}
	defer rows.Close()

	maps := make(map[int]warpIntegrityMap)
	for rows.Next() {
		var info warpIntegrityMap
		var isOverworld int
		if err := rows.Scan(&info.ID, &info.Name, &isOverworld); err != nil {
			t.Fatalf("scan map: %v", err)
		}
		info.IsOverworld = isOverworld != 0
		maps[info.ID] = info
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read maps: %v", err)
	}
	return maps
}

func loadWarpIntegrityTiles(t *testing.T, db *sql.DB) map[int]map[string]warpIntegrityTile {
	t.Helper()
	rows, err := db.Query(`SELECT map_id, x, y, collision_type FROM tiles`)
	if err != nil {
		t.Fatalf("query tiles: %v", err)
	}
	defer rows.Close()

	tiles := make(map[int]map[string]warpIntegrityTile)
	for rows.Next() {
		var mapID int
		var tile warpIntegrityTile
		if err := rows.Scan(&mapID, &tile.X, &tile.Y, &tile.Collision); err != nil {
			t.Fatalf("scan tile: %v", err)
		}
		if tiles[mapID] == nil {
			tiles[mapID] = make(map[string]warpIntegrityTile)
		}
		tiles[mapID][coordKey(tile.X, tile.Y)] = tile
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read tiles: %v", err)
	}
	return tiles
}

func loadWarpIntegrityEvents(t *testing.T, db *sql.DB, maps map[int]warpIntegrityMap) []importedWarpEvent {
	t.Helper()
	mapIDByName := make(map[string]int, len(maps))
	for id, info := range maps {
		mapIDByName[normalizeMapName(info.Name)] = id
	}

	rows, err := db.Query(`
		SELECT map_id, map_name, x, y, dest_map, dest_warp_index
		FROM warp_events
		ORDER BY map_id, id`)
	if err != nil {
		t.Fatalf("query warp events: %v", err)
	}
	defer rows.Close()

	var events []importedWarpEvent
	for rows.Next() {
		var event importedWarpEvent
		var mapID sql.NullInt64
		if err := rows.Scan(&mapID, &event.MapName, &event.X, &event.Y, &event.DestMap, &event.DestWarpIndex); err != nil {
			t.Fatalf("scan warp event: %v", err)
		}
		if mapID.Valid {
			event.MapID = int(mapID.Int64)
		} else {
			resolvedMapID, ok := mapIDByName[normalizeMapName(event.MapName)]
			if !ok {
				t.Fatalf("warp event map %q has no map_id and no matching maps row", event.MapName)
			}
			event.MapID = resolvedMapID
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read warp events: %v", err)
	}
	return events
}

func loadWarpIntegrityWarps(t *testing.T, db *sql.DB) []warpIntegrityRow {
	t.Helper()
	rows, err := db.Query(`
		SELECT id, source_map_id, source_map, x, y, destination_map_id, destination_map
		FROM warps
		ORDER BY id`)
	if err != nil {
		t.Fatalf("query warps: %v", err)
	}
	defer rows.Close()

	var warps []warpIntegrityRow
	for rows.Next() {
		var warp warpIntegrityRow
		var destinationMapID sql.NullInt64
		if err := rows.Scan(
			&warp.ID,
			&warp.SourceMapID,
			&warp.SourceMapName,
			&warp.X,
			&warp.Y,
			&destinationMapID,
			&warp.DestinationMap,
		); err != nil {
			t.Fatalf("scan warp: %v", err)
		}
		if destinationMapID.Valid {
			warp.HasDestination = true
			warp.DestinationMapID = int(destinationMapID.Int64)
		}
		warps = append(warps, warp)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read warps: %v", err)
	}
	return warps
}

func resolveWarpIntegrityLastMapDestinations(events []importedWarpEvent, warps []warpIntegrityRow) {
	eventsBySource := make(map[string]importedWarpEvent, len(events))
	eventsByDestinationName := make(map[string][]importedWarpEvent)
	for _, event := range events {
		eventsBySource[warpCoordKey(event.MapID, event.X, event.Y)] = event
		destinationName := normalizeMapName(event.DestMap)
		eventsByDestinationName[destinationName] = append(eventsByDestinationName[destinationName], event)
	}

	mapNamesByID := make(map[int]string)
	for _, event := range events {
		if mapNamesByID[event.MapID] == "" {
			mapNamesByID[event.MapID] = event.MapName
		}
	}

	for i := range warps {
		warp := &warps[i]
		if warp.HasDestination || strings.ToUpper(warp.DestinationMap) != "LAST_MAP" {
			continue
		}
		sourceEvent, ok := eventsBySource[warpCoordKey(warp.SourceMapID, warp.X, warp.Y)]
		if !ok {
			continue
		}
		sourceName := normalizeMapName(warp.SourceMapName)
		incoming := eventsByDestinationName[sourceName]
		if len(incoming) == 0 {
			continue
		}

		exactMapIDs := distinctIncomingMapIDs(incoming, sourceEvent.DestWarpIndex)
		if len(exactMapIDs) == 1 {
			warp.HasDestination = true
			warp.DestinationMapID = exactMapIDs[0]
			warp.DestinationMap = mapNamesByID[exactMapIDs[0]]
			continue
		}
		uniqueMapIDs := distinctIncomingMapIDs(incoming, 0)
		if len(uniqueMapIDs) == 1 {
			warp.HasDestination = true
			warp.DestinationMapID = uniqueMapIDs[0]
			warp.DestinationMap = mapNamesByID[uniqueMapIDs[0]]
		}
	}
}

func distinctIncomingMapIDs(events []importedWarpEvent, destWarpIndex int) []int {
	seen := make(map[int]bool)
	for _, event := range events {
		if destWarpIndex > 0 && event.DestWarpIndex != destWarpIndex {
			continue
		}
		seen[event.MapID] = true
	}
	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func sourceEventsByCoordinate(
	maps map[int]importedMapInfo,
	offsets map[string]coordinateOffset,
	events []importedWarpEvent,
) map[string]importedWarpEvent {
	sourceEvents := make(map[string]importedWarpEvent, len(events)*2)
	for _, event := range events {
		sourceEvents[warpCoordKey(event.MapID, event.X, event.Y)] = event
		globalX, globalY := globalWarpEventCoordinates(maps, offsets, event.MapID, event.MapName, event.X, event.Y)
		sourceEvents[warpCoordKey(event.MapID, globalX, globalY)] = event
	}
	return sourceEvents
}

func isSpecialWarpOutsideGenericValidation(warp warpIntegrityRow, sourceEvent importedWarpEvent, hasSourceEvent bool) bool {
	sourceName := normalizeMapName(warp.SourceMapName)
	destinationName := normalizeMapName(warp.DestinationMap)
	if strings.Contains(destinationName, "unusedmap") {
		return true
	}
	if strings.Contains(sourceName, "elevator") || strings.Contains(destinationName, "elevator") {
		return true
	}
	if isSeafoamBoulderHoleWarp(warp) {
		return true
	}
	if hasSourceEvent && strings.EqualFold(sourceEvent.DestMap, "LAST_MAP") && isDynamicLastMapSource(sourceName) {
		return true
	}
	return false
}

func isDynamicLastMapSource(normalizedSourceName string) bool {
	switch normalizedSourceName {
	case
		"diglettscaveroute11",
		"mtmoon1f",
		"rocktunnel1f",
		"route22gate",
		"seafoamislands1f",
		"silphco11f",
		"undergroundpathroute5",
		"undergroundpathroute6",
		"undergroundpathroute7",
		"undergroundpathroute7copy",
		"undergroundpathroute8",
		"viridianforestsouthgate":
		return true
	default:
		return false
	}
}

func isSeafoamBoulderHoleWarp(warp warpIntegrityRow) bool {
	sourceName := normalizeMapName(warp.SourceMapName)
	destinationName := normalizeMapName(warp.DestinationMap)
	if !strings.HasPrefix(sourceName, "seafoamislands") || !strings.HasPrefix(destinationName, "seafoamislands") {
		return false
	}
	return (warp.X == 20 || warp.X == 21) && warp.Y == 17
}

func hasWarpActivationTile(tiles map[string]warpIntegrityTile, x, y int) bool {
	if isLand(tiles[coordKey(x, y)]) {
		return true
	}
	return hasAdjacentLandTile(tiles, x, y)
}

func hasAdjacentLandTile(tiles map[string]warpIntegrityTile, x, y int) bool {
	for _, delta := range cardinalDeltas() {
		if isLand(tiles[coordKey(x+delta.x, y+delta.y)]) {
			return true
		}
	}
	return false
}

func isLand(tile warpIntegrityTile) bool {
	return tile.Collision == testCollisionLand
}

func cardinalDeltas() []struct{ x, y int } {
	return []struct{ x, y int }{
		{x: 0, y: -1},
		{x: 0, y: 1},
		{x: -1, y: 0},
		{x: 1, y: 0},
	}
}

func coordKey(x, y int) string {
	return fmt.Sprintf("%d,%d", x, y)
}
