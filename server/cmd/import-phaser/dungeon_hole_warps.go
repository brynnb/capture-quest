package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type dungeonWarpLanding struct {
	DestinationMap string
	WarpIndex      int
	X              int
	Y              int
}

type dungeonWarpSourceTrigger struct {
	SourceMap      string
	DestinationMap string
	WarpIndex      int
	X              int
	Y              int
	SourceFile     string
}

type dungeonHoleWarpSeed struct {
	SourceMap      string
	X              int
	Y              int
	DestinationMap string
	DestinationX   int
	DestinationY   int
	SourceFile     string
}

type asmPoint struct {
	X int
	Y int
}

type asmCoordinateBlock struct {
	Label  string
	Line   int
	Points []asmPoint
}

func seedDungeonHoleWarpsPostgres(pg *sql.DB) error {
	seeds, err := loadDungeonHoleWarpSeeds()
	if err != nil {
		return err
	}
	if len(seeds) == 0 {
		return fmt.Errorf("no dungeon hole warp seeds generated from source asm")
	}
	if err := resetPostgresIdentitySequence(pg, "phaser_warps"); err != nil {
		return err
	}
	return upsertDungeonHoleWarpsPostgres(pg, seeds)
}

func loadDungeonHoleWarpSeeds() ([]dungeonHoleWarpSeed, error) {
	specialWarpsPath, err := findPokemonSourcePath(filepath.Join("data", "maps", "special_warps.asm"))
	if err != nil {
		return nil, err
	}
	rawSpecialWarps, err := os.ReadFile(specialWarpsPath)
	if err != nil {
		return nil, fmt.Errorf("read special dungeon warps %s: %w", specialWarpsPath, err)
	}
	landings, err := parseDungeonWarpLandingsASM(string(rawSpecialWarps))
	if err != nil {
		return nil, fmt.Errorf("parse special dungeon warps %s: %w", specialWarpsPath, err)
	}

	scriptsDir, err := findPokemonSourcePath("scripts")
	if err != nil {
		return nil, err
	}
	triggers, err := parseDungeonWarpSourceTriggersDir(scriptsDir)
	if err != nil {
		return nil, err
	}

	landingsByKey := make(map[string]dungeonWarpLanding)
	for _, landing := range landings {
		landingsByKey[dungeonWarpKey(landing.DestinationMap, landing.WarpIndex)] = landing
	}

	seeds := make([]dungeonHoleWarpSeed, 0, len(triggers))
	seen := make(map[string]bool)
	for _, trigger := range triggers {
		landing, ok := landingsByKey[dungeonWarpKey(trigger.DestinationMap, trigger.WarpIndex)]
		if !ok {
			continue
		}
		key := strings.Join([]string{
			normalizeMapName(trigger.SourceMap),
			strconv.Itoa(trigger.X),
			strconv.Itoa(trigger.Y),
		}, ":")
		if seen[key] {
			continue
		}
		seen[key] = true
		seeds = append(seeds, dungeonHoleWarpSeed{
			SourceMap:      trigger.SourceMap,
			X:              trigger.X,
			Y:              trigger.Y,
			DestinationMap: landing.DestinationMap,
			DestinationX:   landing.X,
			DestinationY:   landing.Y,
			SourceFile:     trigger.SourceFile,
		})
	}

	sort.Slice(seeds, func(i, j int) bool {
		if seeds[i].SourceMap != seeds[j].SourceMap {
			return seeds[i].SourceMap < seeds[j].SourceMap
		}
		if seeds[i].Y != seeds[j].Y {
			return seeds[i].Y < seeds[j].Y
		}
		return seeds[i].X < seeds[j].X
	})
	return seeds, nil
}

func upsertDungeonHoleWarpsPostgres(pg *sql.DB, seeds []dungeonHoleWarpSeed) error {
	mapIDs, err := loadPostgresMapIDsByName(pg)
	if err != nil {
		return err
	}

	updateStmt, err := pg.Prepare(`
		UPDATE phaser_warps
		SET destination_map_id = $1,
		    destination_map = $2,
		    destination_x = $3,
		    destination_y = $4,
		    warp_type = 'carpet',
		    warp_direction = NULL
		WHERE source_map_id = $5
		  AND x = $6
		  AND y = $7`)
	if err != nil {
		return fmt.Errorf("prepare dungeon hole warp update: %w", err)
	}
	defer updateStmt.Close()

	insertStmt, err := pg.Prepare(`
		INSERT INTO phaser_warps (
			source_map_id, x, y, destination_map_id, destination_map,
			destination_x, destination_y, warp_type, warp_direction
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'carpet', NULL)`)
	if err != nil {
		return fmt.Errorf("prepare dungeon hole warp insert: %w", err)
	}
	defer insertStmt.Close()

	inserted := int64(0)
	updated := int64(0)
	for _, seed := range seeds {
		sourceMapID, ok := mapIDs[normalizeMapName(seed.SourceMap)]
		if !ok {
			return fmt.Errorf("source map %s for dungeon hole warp %s (%d,%d) not found", seed.SourceMap, seed.SourceFile, seed.X, seed.Y)
		}
		destinationMapID, ok := mapIDs[normalizeMapName(seed.DestinationMap)]
		if !ok {
			return fmt.Errorf("destination map %s for dungeon hole warp %s (%d,%d) not found", seed.DestinationMap, seed.SourceFile, seed.X, seed.Y)
		}

		result, err := updateStmt.Exec(
			destinationMapID,
			seed.DestinationMap,
			seed.DestinationX,
			seed.DestinationY,
			sourceMapID,
			seed.X,
			seed.Y,
		)
		if err != nil {
			return fmt.Errorf("update dungeon hole warp %s (%d,%d): %w", seed.SourceMap, seed.X, seed.Y, err)
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			updated += affected
			continue
		}

		if _, err := insertStmt.Exec(
			sourceMapID,
			seed.X,
			seed.Y,
			destinationMapID,
			seed.DestinationMap,
			seed.DestinationX,
			seed.DestinationY,
		); err != nil {
			return fmt.Errorf("insert dungeon hole warp %s (%d,%d): %w", seed.SourceMap, seed.X, seed.Y, err)
		}
		inserted++
	}

	log.Printf("  -> Seeded %d dungeon hole warps (%d inserted, %d updated)", len(seeds), inserted, updated)
	return nil
}

func loadPostgresMapIDsByName(pg *sql.DB) (map[string]int, error) {
	rows, err := pg.Query(`SELECT id, name FROM phaser_maps`)
	if err != nil {
		return nil, fmt.Errorf("load map ids for dungeon hole warps: %w", err)
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan map id for dungeon hole warps: %w", err)
		}
		result[normalizeMapName(name)] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read map ids for dungeon hole warps: %w", err)
	}
	return result, nil
}

func parseDungeonWarpLandingsASM(raw string) ([]dungeonWarpLanding, error) {
	type listEntry struct {
		destinationMap string
		warpIndex      int
	}
	type dataEntry struct {
		destinationMap string
		x              int
		y              int
	}

	var list []listEntry
	var data []dataEntry
	section := ""
	for _, rawLine := range strings.Split(raw, "\n") {
		line := cleanASMLine(rawLine)
		switch line {
		case "DungeonWarpList:":
			section = "list"
			continue
		case "DungeonWarpData:":
			section = "data"
			continue
		}
		if line == "" {
			continue
		}

		switch section {
		case "list":
			if strings.HasPrefix(line, "db -1") {
				section = ""
				continue
			}
			args, ok := parseASMCall(line, "db")
			if !ok || len(args) != 2 {
				continue
			}
			warpIndex, err := parseASMInt(args[1])
			if err != nil {
				return nil, err
			}
			list = append(list, listEntry{
				destinationMap: mapNameToUpperSnake(args[0]),
				warpIndex:      warpIndex,
			})
		case "data":
			if len(list) > 0 && len(data) == len(list) {
				section = ""
				continue
			}
			args, ok := parseASMCall(line, "fly_warp")
			if !ok || len(args) != 3 {
				continue
			}
			x, err := parseASMInt(args[1])
			if err != nil {
				return nil, err
			}
			y, err := parseASMInt(args[2])
			if err != nil {
				return nil, err
			}
			data = append(data, dataEntry{
				destinationMap: mapNameToUpperSnake(args[0]),
				x:              x,
				y:              y,
			})
		}
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("DungeonWarpList not found")
	}
	if len(data) != len(list) {
		return nil, fmt.Errorf("DungeonWarpData entries=%d, want %d", len(data), len(list))
	}

	landings := make([]dungeonWarpLanding, 0, len(list))
	for i := range list {
		if normalizeMapName(list[i].destinationMap) != normalizeMapName(data[i].destinationMap) {
			return nil, fmt.Errorf("dungeon warp list/data map mismatch at %d: %s vs %s", i, list[i].destinationMap, data[i].destinationMap)
		}
		landings = append(landings, dungeonWarpLanding{
			DestinationMap: list[i].destinationMap,
			WarpIndex:      list[i].warpIndex,
			X:              data[i].x,
			Y:              data[i].y,
		})
	}
	return landings, nil
}

func parseDungeonWarpSourceTriggersDir(scriptsDir string) ([]dungeonWarpSourceTrigger, error) {
	entries, err := os.ReadDir(scriptsDir)
	if err != nil {
		return nil, fmt.Errorf("read scripts dir %s: %w", scriptsDir, err)
	}

	var triggers []dungeonWarpSourceTrigger
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".asm" {
			continue
		}
		path := filepath.Join(scriptsDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read script %s: %w", path, err)
		}
		sourceMap := mapNameToUpperSnake(strings.TrimSuffix(entry.Name(), ".asm"))
		fileTriggers, err := parseDungeonWarpSourceTriggersASM(sourceMap, entry.Name(), string(raw))
		if err != nil {
			return nil, fmt.Errorf("parse dungeon warp source triggers %s: %w", path, err)
		}
		triggers = append(triggers, fileTriggers...)
	}

	sort.Slice(triggers, func(i, j int) bool {
		if triggers[i].SourceMap != triggers[j].SourceMap {
			return triggers[i].SourceMap < triggers[j].SourceMap
		}
		if triggers[i].Y != triggers[j].Y {
			return triggers[i].Y < triggers[j].Y
		}
		return triggers[i].X < triggers[j].X
	})
	return triggers, nil
}

func parseDungeonWarpSourceTriggersASM(sourceMap, sourceFile, raw string) ([]dungeonWarpSourceTrigger, error) {
	lines := cleanASMLines(raw)
	coordinateBlocks := parseASMCoordinateBlocks(lines)
	triggers := parseDirectDungeonWarpTriggers(sourceMap, sourceFile, lines, coordinateBlocks)
	conditional, err := parseConditionalDungeonWarpTriggers(sourceMap, sourceFile, lines, coordinateBlocks)
	if err != nil {
		return nil, err
	}
	triggers = append(triggers, conditional...)
	return triggers, nil
}

func parseDirectDungeonWarpTriggers(sourceMap, sourceFile string, lines []string, coordinateBlocks []asmCoordinateBlock) []dungeonWarpSourceTrigger {
	var triggers []dungeonWarpSourceTrigger
	for i, line := range lines {
		if !strings.Contains(line, "IsPlayerOnDungeonWarp") {
			continue
		}
		label, ok := findPreviousLDHL(lines, i, 8)
		if !ok {
			continue
		}
		destinationMap, ok := findPreviousDungeonDestinationMap(lines, i, 12)
		if !ok {
			continue
		}
		block, ok := findASMCoordinateBlock(coordinateBlocks, label, i)
		if !ok {
			continue
		}
		for index, point := range block.Points {
			triggers = append(triggers, dungeonWarpSourceTrigger{
				SourceMap:      sourceMap,
				DestinationMap: destinationMap,
				WarpIndex:      index + 1,
				X:              point.X,
				Y:              point.Y,
				SourceFile:     sourceFile,
			})
		}
	}
	return triggers
}

func parseConditionalDungeonWarpTriggers(sourceMap, sourceFile string, lines []string, coordinateBlocks []asmCoordinateBlock) ([]dungeonWarpSourceTrigger, error) {
	var triggers []dungeonWarpSourceTrigger
	for i, line := range lines {
		if !strings.HasPrefix(line, "call ") || !strings.Contains(line, "isPlayerFallingDownHole") {
			continue
		}
		label, ok := findPreviousLDHL(lines, i, 4)
		if !ok {
			continue
		}
		block, ok := findASMCoordinateBlock(coordinateBlocks, label, i)
		if !ok {
			continue
		}

		compareIndex := 0
		defaultMap := ""
		branchMap := ""
		afterBranch := false
		for j := i + 1; j < len(lines) && j <= i+20; j++ {
			current := lines[j]
			if strings.HasPrefix(current, "cp ") && compareIndex == 0 {
				value, err := parseASMInt(strings.TrimSpace(strings.TrimPrefix(current, "cp ")))
				if err != nil {
					return nil, err
				}
				compareIndex = value
				continue
			}
			if strings.HasPrefix(current, "jr nz,") && defaultMap != "" {
				afterBranch = true
				continue
			}
			if operand, ok := parseASMLoadAIdentifier(current); ok {
				if afterBranch {
					branchMap = mapNameToUpperSnake(operand)
				} else if compareIndex > 0 && defaultMap == "" {
					defaultMap = mapNameToUpperSnake(operand)
				}
				continue
			}
			if current == "ld [wDungeonWarpDestinationMap], a" {
				break
			}
		}
		if compareIndex == 0 || defaultMap == "" || branchMap == "" {
			continue
		}
		for index, point := range block.Points {
			destinationMap := defaultMap
			if index+1 == compareIndex {
				destinationMap = branchMap
			}
			triggers = append(triggers, dungeonWarpSourceTrigger{
				SourceMap:      sourceMap,
				DestinationMap: destinationMap,
				WarpIndex:      index + 1,
				X:              point.X,
				Y:              point.Y,
				SourceFile:     sourceFile,
			})
		}
	}
	return triggers, nil
}

func cleanASMLines(raw string) []string {
	rawLines := strings.Split(raw, "\n")
	lines := make([]string, len(rawLines))
	for i, line := range rawLines {
		lines[i] = cleanASMLine(line)
	}
	return lines
}

func cleanASMLine(line string) string {
	if idx := strings.Index(line, ";"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func parseASMCoordinateBlocks(lines []string) []asmCoordinateBlock {
	var blocks []asmCoordinateBlock
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "dbmapcoord ") {
			continue
		}
		label, ok := previousASMLabel(lines, i)
		if !ok {
			continue
		}
		block := asmCoordinateBlock{Label: label, Line: i}
		for ; i < len(lines); i++ {
			line := lines[i]
			if strings.HasPrefix(line, "db -1") {
				break
			}
			args, ok := parseASMCall(line, "dbmapcoord")
			if !ok || len(args) != 2 {
				continue
			}
			x, err := parseASMInt(args[0])
			if err != nil {
				break
			}
			y, err := parseASMInt(args[1])
			if err != nil {
				break
			}
			block.Points = append(block.Points, asmPoint{X: x, Y: y})
		}
		if len(block.Points) > 0 {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func previousASMLabel(lines []string, before int) (string, bool) {
	for i := before - 1; i >= 0; i-- {
		line := lines[i]
		if line == "" {
			continue
		}
		if strings.ContainsAny(line, " \t") {
			return "", false
		}
		return strings.TrimSuffix(line, ":"), true
	}
	return "", false
}

func findASMCoordinateBlock(blocks []asmCoordinateBlock, label string, referenceLine int) (asmCoordinateBlock, bool) {
	label = strings.TrimSuffix(label, ":")
	bestIndex := -1
	bestDistance := 0
	for i, block := range blocks {
		if block.Label != label {
			continue
		}
		distance := block.Line - referenceLine
		if distance < 0 {
			distance = -distance
		}
		if bestIndex == -1 || distance < bestDistance {
			bestIndex = i
			bestDistance = distance
		}
	}
	if bestIndex == -1 {
		return asmCoordinateBlock{}, false
	}
	return blocks[bestIndex], true
}

func findPreviousLDHL(lines []string, before, maxDistance int) (string, bool) {
	for i := before - 1; i >= 0 && before-i <= maxDistance; i-- {
		if strings.HasPrefix(lines[i], "ld hl,") {
			return strings.TrimSpace(strings.TrimPrefix(lines[i], "ld hl,")), true
		}
	}
	return "", false
}

func findPreviousDungeonDestinationMap(lines []string, before, maxDistance int) (string, bool) {
	for i := before - 1; i >= 0 && before-i <= maxDistance; i-- {
		if lines[i] != "ld [wDungeonWarpDestinationMap], a" {
			continue
		}
		for j := i - 1; j >= 0 && before-j <= maxDistance; j-- {
			if operand, ok := parseASMLoadAIdentifier(lines[j]); ok {
				return mapNameToUpperSnake(operand), true
			}
		}
	}
	return "", false
}

func parseASMLoadAIdentifier(line string) (string, bool) {
	if !strings.HasPrefix(line, "ld a,") {
		return "", false
	}
	operand := strings.TrimSpace(strings.TrimPrefix(line, "ld a,"))
	if operand == "" ||
		strings.ContainsAny(operand, "[]$") ||
		strings.EqualFold(operand, "a") ||
		strings.Contains(operand, " ") {
		return "", false
	}
	if _, err := parseASMInt(operand); err == nil {
		return "", false
	}
	return operand, true
}

func parseASMCall(line, name string) ([]string, bool) {
	line = strings.TrimSpace(line)
	if line != name && !strings.HasPrefix(line, name+" ") && !strings.HasPrefix(line, name+"\t") {
		return nil, false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, name))
	if rest == "" {
		return nil, true
	}
	parts := strings.Split(rest, ",")
	args := make([]string, 0, len(parts))
	for _, part := range parts {
		args = append(args, strings.TrimSpace(part))
	}
	return args, true
}

func parseASMInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "()")
	if value == "" {
		return 0, fmt.Errorf("empty asm int")
	}
	if strings.HasPrefix(value, "$") {
		parsed, err := strconv.ParseInt(strings.TrimPrefix(value, "$"), 16, 64)
		return int(parsed), err
	}
	parsed, err := strconv.Atoi(value)
	return parsed, err
}

func dungeonWarpKey(destinationMap string, warpIndex int) string {
	return normalizeMapName(destinationMap) + ":" + strconv.Itoa(warpIndex)
}

func findPokemonSourcePath(relative string) (string, error) {
	base := filepath.Join("tools", "pokemon-gameboy-extractor-tool", "pokemon-game-data")
	candidates := []string{
		filepath.Join(base, relative),
		filepath.Join("..", base, relative),
		filepath.Join("..", "..", base, relative),
		filepath.Join("..", "..", "..", base, relative),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("pokemon source path %s not found", relative)
}
