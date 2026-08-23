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
	ID                int
	SourceMapID       int
	SourceMapName     string
	SourceWarpIndex   int
	X                 int
	Y                 int
	DestinationMapID  int
	DestinationMap    string
	DestinationKind   string
	DestinationWarpID int
	HasDestination    bool
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
	var failures []string
	skippedSpecial := 0
	deterministicLastMap := 0
	for _, warp := range warps {
		if warp.DestinationKind == "last-map" && !warp.HasDestination {
			skippedSpecial++
			continue
		}
		if warp.DestinationKind == "last-map" {
			deterministicLastMap++
		}
		sourceX, sourceY := warp.X, warp.Y
		if info := maps[warp.SourceMapID]; info.IsOverworld {
			if offset, ok := offsets[normalizeMapName(info.Name)]; ok {
				sourceX += offset.X
				sourceY += offset.Y
			}
		}
		if isSpecialWarpOutsideGenericValidation(warp) {
			skippedSpecial++
			continue
		}
		sourceTileMap := tiles[warp.SourceMapID]
		if _, ok := sourceTileMap[coordKey(sourceX, sourceY)]; !ok {
			failures = append(failures, fmt.Sprintf("warp %d %s(%d,%d) source tile missing", warp.ID, warp.SourceMapName, warp.X, warp.Y))
			continue
		}
		if !hasWarpActivationTile(sourceTileMap, sourceX, sourceY) {
			if hasReachableEquivalentWarp(warp, warps, maps, offsets, tiles) {
				skippedSpecial++
				continue
			}
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
	if deterministicLastMap == 0 {
		t.Fatal("expected at least one deterministic LAST_MAP exit to be validated")
	}
	t.Logf(
		"validated %d warps including %d deterministic LAST_MAP exits; skipped %d special/scripted/dynamic warps",
		len(warps)-skippedSpecial,
		deterministicLastMap,
		skippedSpecial,
	)
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
		SELECT id, source_map_id, source_map, source_warp_index, x, y, destination_map_id,
		       destination_map, destination_kind, destination_warp_id
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
			&warp.SourceWarpIndex,
			&warp.X,
			&warp.Y,
			&destinationMapID,
			&warp.DestinationMap,
			&warp.DestinationKind,
			&warp.DestinationWarpID,
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
	sourceMapIDsByName := make(map[string]int)
	targetEntries := make(map[string]warpIntegrityRow, len(warps))
	mapNamesByID := make(map[int]string)
	for _, warp := range warps {
		sourceMapIDsByName[normalizeMapName(warp.SourceMapName)] = warp.SourceMapID
		targetEntries[fmt.Sprintf("%d:%d", warp.SourceMapID, warp.SourceWarpIndex)] = warp
		mapNamesByID[warp.SourceMapID] = warp.SourceMapName
	}

	candidateOrigins := make(map[int]map[int]bool)
	for _, event := range events {
		sourceMapID, ok := sourceMapIDsByName[normalizeMapName(event.DestMap)]
		if !ok {
			continue
		}
		targetEntry, ok := targetEntries[fmt.Sprintf("%d:%d", sourceMapID, event.DestWarpIndex)]
		if !ok || targetEntry.DestinationKind != "last-map" {
			continue
		}
		if candidateOrigins[sourceMapID] == nil {
			candidateOrigins[sourceMapID] = make(map[int]bool)
		}
		candidateOrigins[sourceMapID][event.MapID] = true
		if mapNamesByID[event.MapID] == "" {
			mapNamesByID[event.MapID] = event.MapName
		}
	}

	for i := range warps {
		warp := &warps[i]
		if warp.HasDestination || strings.ToUpper(warp.DestinationMap) != "LAST_MAP" {
			continue
		}
		uniqueMapIDs := sortedMapKeys(candidateOrigins[warp.SourceMapID])
		if len(uniqueMapIDs) == 1 {
			warp.HasDestination = true
			warp.DestinationMapID = uniqueMapIDs[0]
			warp.DestinationMap = mapNamesByID[uniqueMapIDs[0]]
		}
	}
}

func sortedMapKeys(values map[int]bool) []int {
	ids := make([]int, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func isSpecialWarpOutsideGenericValidation(warp warpIntegrityRow) bool {
	sourceName := normalizeMapName(warp.SourceMapName)
	destinationName := normalizeMapName(warp.DestinationMap)
	// This retained, unreachable duplicate has source data but no map tiles.
	if sourceName == "undergroundpathroute7copy" {
		return true
	}
	if strings.Contains(destinationName, "unusedmap") {
		return true
	}
	if strings.Contains(sourceName, "elevator") || strings.Contains(destinationName, "elevator") {
		return true
	}
	if isSeafoamBoulderHoleWarp(warp) {
		return true
	}
	return false
}

func hasReachableEquivalentWarp(
	warp warpIntegrityRow,
	warps []warpIntegrityRow,
	maps map[int]warpIntegrityMap,
	offsets map[string]coordinateOffset,
	tiles map[int]map[string]warpIntegrityTile,
) bool {
	for _, candidate := range warps {
		if candidate.ID == warp.ID ||
			candidate.SourceMapID != warp.SourceMapID ||
			candidate.DestinationKind != warp.DestinationKind ||
			candidate.DestinationMapID != warp.DestinationMapID ||
			candidate.DestinationWarpID != warp.DestinationWarpID {
			continue
		}
		x, y := candidate.X, candidate.Y
		if info := maps[candidate.SourceMapID]; info.IsOverworld {
			if offset, ok := offsets[normalizeMapName(info.Name)]; ok {
				x += offset.X
				y += offset.Y
			}
		}
		if hasWarpActivationTile(tiles[candidate.SourceMapID], x, y) {
			return true
		}
	}
	return false
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
