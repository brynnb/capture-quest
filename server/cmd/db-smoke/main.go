// Command db-smoke verifies that a freshly bootstrapped Postgres database has
// the core CaptureQuest schema and deterministic seed/import data.
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"capturequest/internal/config"

	_ "github.com/jackc/pgx/v4/stdlib"
)

var tableNamePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
	log.Println("Postgres bootstrap smoke checks passed.")
}

func run() error {
	target, err := config.GetDatabaseTarget()
	if err != nil {
		return fmt.Errorf("load database target: %w", err)
	}
	if target.Dialect != config.DatabaseDialectPostgres {
		return fmt.Errorf("db-smoke is Postgres-only; got DB dialect %q from %s", target.Dialect, target.Source)
	}

	pg, err := sql.Open(target.DriverName, target.DSN)
	if err != nil {
		return fmt.Errorf("open Postgres: %w", err)
	}
	defer pg.Close()
	if err := pg.Ping(); err != nil {
		return fmt.Errorf("ping Postgres: %w", err)
	}

	if err := requireTables(pg, coreTables); err != nil {
		return err
	}
	if err := requireMinimumCounts(pg, minimumCounts); err != nil {
		return err
	}
	if err := requireScriptedEvents(pg); err != nil {
		return err
	}
	if err := requireMtMoonYoungster(pg); err != nil {
		return err
	}
	if err := requireOakLabExitWarp(pg); err != nil {
		return err
	}
	if err := requireRoute6GateExitTiles(pg); err != nil {
		return err
	}
	if err := requireAgathaTopExitWarps(pg); err != nil {
		return err
	}
	if err := requireSilphElevatorRuntimeConfiguration(pg); err != nil {
		return err
	}
	if err := requireNoIncompletePlayableWarps(pg); err != nil {
		return err
	}
	if err := requireCeruleanMartTalkOverCounter(pg); err != nil {
		return err
	}
	if err := requireBikeShopTalkOverCounter(pg); err != nil {
		return err
	}
	if err := requireMartMerchants(pg); err != nil {
		return err
	}
	if err := requireInGameTrades(pg); err != nil {
		return err
	}
	return nil
}

var coreTables = []string{
	"account",
	"account_ip",
	"character_data",
	"character_bind",
	"character_battle_state",
	"character_wallet",
	"character_in_game_trades",
	"character_pokemon",
	"character_pc_state",
	"character_pokedex",
	"cq_items",
	"cq_item_instances",
	"cq_character_inventory",
	"cq_merchants",
	"cq_merchant_items",
	"phaser_maps",
	"phaser_tiles",
	"phaser_objects",
	"phaser_trainer_headers",
	"phaser_in_game_trades",
	"phaser_cutscene_scripts",
	"poke_classes",
	"poke_factions",
	"poke_start_cities",
}

var minimumCounts = map[string]int{
	"phaser_pokemon":         151,
	"phaser_moves":           1,
	"phaser_maps":            1,
	"phaser_tiles":           1,
	"phaser_objects":         1,
	"phaser_trainer_classes": 1,
	"phaser_trainer_parties": 1,
	"phaser_trainer_headers": 1,
	"phaser_in_game_trades":  9,
	"cq_items":               1,
	"cq_merchants":           1,
	"cq_merchant_items":      1,
	"poke_classes":           12,
	"poke_factions":          12,
	"poke_start_cities":      10,
}

func requireTables(pg *sql.DB, tables []string) error {
	missing := []string{}
	for _, table := range tables {
		exists, err := tableExists(pg, table)
		if err != nil {
			return fmt.Errorf("check table %s: %w", table, err)
		}
		if !exists {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required tables: %v", missing)
	}
	log.Printf("ok: %d required tables exist", len(tables))
	return nil
}

func tableExists(pg *sql.DB, table string) (bool, error) {
	var regclass sql.NullString
	if err := pg.QueryRow(`SELECT to_regclass($1)`, "public."+table).Scan(&regclass); err != nil {
		return false, err
	}
	return regclass.Valid && regclass.String != "", nil
}

func requireMinimumCounts(pg *sql.DB, expectations map[string]int) error {
	for table, minimum := range expectations {
		count, err := countRows(pg, table)
		if err != nil {
			return err
		}
		if count < minimum {
			return fmt.Errorf("%s has %d rows, want at least %d", table, count, minimum)
		}
		log.Printf("ok: %s has %d rows", table, count)
	}
	return nil
}

func countRows(pg *sql.DB, table string) (int, error) {
	if !tableNamePattern.MatchString(table) {
		return 0, fmt.Errorf("unsafe table name %q", table)
	}
	var count int
	if err := pg.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&count); err != nil {
		return 0, fmt.Errorf("count %s: %w", table, err)
	}
	return count, nil
}

func requireScriptedEvents(pg *sql.DB) error {
	expected, root, err := countScriptEventFiles()
	if err != nil {
		return err
	}
	if expected == 0 {
		return fmt.Errorf("no scripted event JSON files found under %s", root)
	}
	count, err := countRows(pg, "phaser_cutscene_scripts")
	if err != nil {
		return err
	}
	if count < expected {
		return fmt.Errorf("phaser_cutscene_scripts has %d rows, want at least %d scripted event files from %s", count, expected, root)
	}
	log.Printf("ok: phaser_cutscene_scripts has %d rows for %d scripted event files", count, expected)
	return nil
}

func countScriptEventFiles() (int, string, error) {
	root, err := findRepoRoot()
	if err != nil {
		return 0, "", err
	}
	scriptDirs := []string{
		filepath.Join(root, "server", "scripted_events", "scripts"),
		filepath.Join(root, "server", "scripted_events", "manual_scripts"),
	}
	labels := make(map[string]struct{})
	foundDir := false
	for _, scriptsDir := range scriptDirs {
		entries, err := os.ReadDir(scriptsDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return 0, scriptsDir, fmt.Errorf("read scripted event scripts dir: %w", err)
		}
		foundDir = true
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(scriptsDir, entry.Name()))
			if err != nil {
				return 0, scriptsDir, fmt.Errorf("read scripted event file %s: %w", entry.Name(), err)
			}
			var event struct {
				ScriptLabel string `json:"scriptLabel"`
			}
			if err := json.Unmarshal(raw, &event); err != nil {
				return 0, scriptsDir, fmt.Errorf("parse scripted event file %s: %w", entry.Name(), err)
			}
			if event.ScriptLabel == "" {
				return 0, scriptsDir, fmt.Errorf("scripted event file %s missing scriptLabel", entry.Name())
			}
			labels[event.ScriptLabel] = struct{}{}
		}
	}
	if !foundDir {
		return 0, strings.Join(scriptDirs, ", "), fmt.Errorf("no scripted event scripts dirs found")
	}
	return len(labels), strings.Join(scriptDirs, ", "), nil
}

func findRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "server", "schema", "postgres_runtime_schema.sql")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	return "", fmt.Errorf("repo root not found from %s", cwd)
}

func requireMtMoonYoungster(pg *sql.DB) error {
	var row struct {
		ObjectID          int
		MapID             int
		X                 int
		Y                 int
		Direction         string
		TrainerClass      string
		PartyIndex        int
		Text              string
		PointerIndex      int
		HeaderIndex       int
		SightRange        int
		BattleTextLabel   string
		EndBattleText     string
		AfterBattleText   string
		TrainerClassLabel string
	}

	err := pg.QueryRow(`
		SELECT
			po.id,
			po.map_id,
			COALESCE(po.local_x, po.x),
			COALESCE(po.local_y, po.y),
			po.action_direction,
			po.trainer_class,
			po.trainer_party_index,
			po.text,
			tp.pointer_index,
			th.header_index,
			th.sight_range,
			th.battle_text_label,
			th.end_battle_text_label,
			th.after_battle_text_label,
			tc.constant_name
		FROM phaser_objects po
		LEFT JOIN phaser_maps pm
			ON pm.id = po.map_id
		JOIN phaser_text_pointers tp
			ON tp.text_constant = po.text
			AND tp.is_trainer = 1
		JOIN phaser_trainer_headers th
			ON th.header_index = tp.pointer_index - 1
			AND (
				th.map_id = po.map_id
				OR LOWER(REPLACE(th.map_name, '_', '')) = LOWER(REPLACE(pm.name, '_', ''))
			)
		LEFT JOIN phaser_trainer_classes tc
			ON tc.constant_name = po.trainer_class
		WHERE po.name = $1
		LIMIT 1`, "MtMoon1F_NPC_2").Scan(
		&row.ObjectID,
		&row.MapID,
		&row.X,
		&row.Y,
		&row.Direction,
		&row.TrainerClass,
		&row.PartyIndex,
		&row.Text,
		&row.PointerIndex,
		&row.HeaderIndex,
		&row.SightRange,
		&row.BattleTextLabel,
		&row.EndBattleText,
		&row.AfterBattleText,
		&row.TrainerClassLabel,
	)
	if err != nil {
		return fmt.Errorf("load MtMoon1F_NPC_2 trainer smoke row: %w", err)
	}

	expectations := map[string]bool{
		"position":             row.X == 12 && row.Y == 16,
		"facing":               row.Direction == "RIGHT",
		"trainer class":        row.TrainerClass == "YOUNGSTER" && row.TrainerClassLabel == "YOUNGSTER",
		"party index":          row.PartyIndex == 3,
		"text constant":        row.Text == "TEXT_MTMOON1F_YOUNGSTER1",
		"trainer pointer join": row.PointerIndex == row.HeaderIndex+1,
		"sight range":          row.SightRange == 3,
		"battle text":          row.BattleTextLabel == "_MtMoon1FYoungster1BattleText",
		"after battle text":    row.AfterBattleText == "_MtMoon1FYoungster1AfterBattleText",
	}
	for label, ok := range expectations {
		if !ok {
			return fmt.Errorf("MtMoon1F_NPC_2 %s failed: %+v", label, row)
		}
	}
	log.Printf("ok: MtMoon1F_NPC_2 trainer header has sight range %d at (%d,%d)", row.SightRange, row.X, row.Y)
	return nil
}

func requireOakLabExitWarp(pg *sql.DB) error {
	rows, err := pg.Query(`
		SELECT x, y, destination_x, destination_y
		FROM phaser_warps
		WHERE source_map_id = 40
		  AND x IN (4, 5)
		  AND y = 11
		  AND destination_map_id = 0
		ORDER BY x`)
	if err != nil {
		return fmt.Errorf("query Oak's Lab exit warps: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var x, y, destX, destY int
		if err := rows.Scan(&x, &y, &destX, &destY); err != nil {
			return fmt.Errorf("scan Oak's Lab exit warp: %w", err)
		}
		count++
		if destX != 12 || destY != 11 {
			return fmt.Errorf("Oak's Lab exit warp (%d,%d) destination = (%d,%d), want (12,11)", x, y, destX, destY)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read Oak's Lab exit warps: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("Oak's Lab exit warp count = %d, want 2", count)
	}
	log.Println("ok: Oak's Lab exit warps land outside the lab door at Pallet Town (12,11)")
	return nil
}

func requireRoute6GateExitTiles(pg *sql.DB) error {
	expected := []struct {
		X int
		Y int
	}{
		{190, -83},
		{190, -82},
		{190, -81},
	}
	for _, tile := range expected {
		var collisionType int
		if err := pg.QueryRow(`
			SELECT collision_type
			FROM phaser_tiles
			WHERE map_id IS NULL AND x = $1 AND y = $2`,
			tile.X, tile.Y,
		).Scan(&collisionType); err != nil {
			return fmt.Errorf("Route 6 Gate exit tile (%d,%d): %w", tile.X, tile.Y, err)
		}
		if collisionType != 1 {
			return fmt.Errorf("Route 6 Gate exit tile (%d,%d) collision = %d, want land", tile.X, tile.Y, collisionType)
		}
	}
	log.Println("ok: Route 6 Gate south exit has connected overworld land tiles")
	return nil
}

func requireAgathaTopExitWarps(pg *sql.DB) error {
	rows, err := pg.Query(`
		SELECT pw.x, pw.y, pw.warp_type, COALESCE(pw.warp_direction, '')
		FROM phaser_warps pw
		JOIN phaser_maps source_map ON source_map.id = pw.source_map_id
		JOIN phaser_maps destination_map ON destination_map.id = pw.destination_map_id
		WHERE source_map.name = 'AGATHAS_ROOM'
		  AND destination_map.name = 'LANCES_ROOM'
		  AND pw.x IN (4, 5)
		  AND pw.y = 0
		ORDER BY pw.x`)
	if err != nil {
		return fmt.Errorf("query Agatha's Room top exit warps: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var x, y int
		var warpType, direction string
		if err := rows.Scan(&x, &y, &warpType, &direction); err != nil {
			return fmt.Errorf("scan Agatha's Room top exit warp: %w", err)
		}
		count++
		if warpType != "carpet" || direction != "UP" {
			return fmt.Errorf("Agatha's Room top exit warp (%d,%d) = type %q direction %q, want carpet UP", x, y, warpType, direction)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read Agatha's Room top exit warps: %w", err)
	}
	if count != 2 {
		return fmt.Errorf("Agatha's Room top exit warp count = %d, want 2", count)
	}
	log.Println("ok: Agatha's Room top exits to Lance activate when pressing up")
	return nil
}

func requireSilphElevatorRuntimeConfiguration(pg *sql.DB) error {
	var placeholderCount int
	if err := pg.QueryRow(`
		SELECT COUNT(*)
		FROM phaser_warps pw
		JOIN phaser_maps source_map ON source_map.id = pw.source_map_id
		JOIN phaser_maps destination_map ON destination_map.id = pw.destination_map_id
		WHERE source_map.name = 'SILPH_CO_ELEVATOR'
		  AND destination_map.name = 'UNUSED_MAP_ED'
		  AND pw.destination_x IS NULL
		  AND pw.destination_y IS NULL
		  AND pw.warp_type = 'elevator'`).Scan(&placeholderCount); err != nil {
		return fmt.Errorf("query Silph elevator dynamic warp placeholders: %w", err)
	}
	if placeholderCount != 2 {
		return fmt.Errorf("Silph elevator dynamic placeholder count = %d, want 2", placeholderCount)
	}

	var floorCount int
	if err := pg.QueryRow(`
		SELECT COUNT(*)
		FROM phaser_elevator_floors ef
		JOIN phaser_maps elevator_map ON elevator_map.id = ef.elevator_map_id
		WHERE elevator_map.name = 'SILPH_CO_ELEVATOR'`).Scan(&floorCount); err != nil {
		return fmt.Errorf("query Silph elevator floor rows: %w", err)
	}
	if floorCount != 11 {
		return fmt.Errorf("Silph elevator floor count = %d, want 11", floorCount)
	}

	var signCount int
	if err := pg.QueryRow(`
		SELECT COUNT(*)
		FROM phaser_objects po
		JOIN phaser_maps pm ON pm.id = po.map_id
		WHERE pm.name = 'SILPH_CO_ELEVATOR'
		  AND po.object_type = 'sign'`).Scan(&signCount); err != nil {
		return fmt.Errorf("query Silph elevator panel sign: %w", err)
	}
	if signCount < 1 {
		return fmt.Errorf("Silph elevator has no clickable sign actor")
	}

	log.Println("ok: Silph elevator has dynamic placeholders, 11 selectable floors, and a clickable panel")
	return nil
}

func requireNoIncompletePlayableWarps(pg *sql.DB) error {
	rows, err := pg.Query(`
		SELECT source_map.name, pw.id, pw.x, pw.y, COALESCE(destination_map.name, ''), COALESCE(pw.warp_type, 'door')
		FROM phaser_warps pw
		JOIN phaser_maps source_map ON source_map.id = pw.source_map_id
		LEFT JOIN phaser_maps destination_map ON destination_map.id = pw.destination_map_id
		WHERE (pw.destination_map_id IS NULL OR pw.destination_x IS NULL OR pw.destination_y IS NULL)
		  AND COALESCE(pw.warp_type, 'door') NOT IN ('elevator', 'inactive')
		ORDER BY source_map.name, pw.id
		LIMIT 10`)
	if err != nil {
		return fmt.Errorf("query incomplete playable warps: %w", err)
	}
	defer rows.Close()

	var incomplete []string
	for rows.Next() {
		var sourceMap, destinationMap, warpType string
		var id, x, y int
		if err := rows.Scan(&sourceMap, &id, &x, &y, &destinationMap, &warpType); err != nil {
			return fmt.Errorf("scan incomplete playable warp: %w", err)
		}
		if destinationMap == "" {
			destinationMap = "<null>"
		}
		incomplete = append(incomplete, fmt.Sprintf("%s #%d (%d,%d) -> %s [%s]", sourceMap, id, x, y, destinationMap, warpType))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read incomplete playable warps: %w", err)
	}
	if len(incomplete) > 0 {
		return fmt.Errorf("incomplete playable warps: %s", strings.Join(incomplete, "; "))
	}
	log.Println("ok: all playable warps have complete destination map and coordinates")
	return nil
}

func requireCeruleanMartTalkOverCounter(pg *sql.DB) error {
	var counterTalkOver bool
	if err := pg.QueryRow(`
		SELECT pt.talk_over_tile
		FROM phaser_tiles pt
		JOIN phaser_maps pm ON pm.id = pt.map_id
		WHERE pm.name = 'CERULEAN_MART'
		  AND pt.x = 1
		  AND pt.y = 5`).Scan(&counterTalkOver); err != nil {
		return fmt.Errorf("load Cerulean Mart counter tile: %w", err)
	}
	if !counterTalkOver {
		return fmt.Errorf("Cerulean Mart counter tile (1,5) talk_over_tile = false, want true")
	}

	var floorTalkOver bool
	if err := pg.QueryRow(`
		SELECT pt.talk_over_tile
		FROM phaser_tiles pt
		JOIN phaser_maps pm ON pm.id = pt.map_id
		WHERE pm.name = 'CERULEAN_MART'
		  AND pt.x = 2
		  AND pt.y = 5`).Scan(&floorTalkOver); err != nil {
		return fmt.Errorf("load Cerulean Mart floor tile: %w", err)
	}
	if floorTalkOver {
		return fmt.Errorf("Cerulean Mart floor tile (2,5) talk_over_tile = true, want false")
	}

	log.Println("ok: Cerulean Mart clerk counter extends talk range")
	return nil
}

func requireBikeShopTalkOverCounter(pg *sql.DB) error {
	expectedTalkOverTiles := [][2]int{
		{5, 2},
		{6, 3},
	}
	for _, tile := range expectedTalkOverTiles {
		var counterTalkOver bool
		if err := pg.QueryRow(`
		SELECT pt.talk_over_tile
		FROM phaser_tiles pt
		JOIN phaser_maps pm ON pm.id = pt.map_id
		WHERE pm.name = 'BIKE_SHOP'
		  AND pt.x = $1
		  AND pt.y = $2`, tile[0], tile[1]).Scan(&counterTalkOver); err != nil {
			return fmt.Errorf("load Bike Shop counter tile (%d,%d): %w", tile[0], tile[1], err)
		}
		if !counterTalkOver {
			return fmt.Errorf("Bike Shop counter tile (%d,%d) talk_over_tile = false, want true", tile[0], tile[1])
		}
	}

	var clerkCount int
	if err := pg.QueryRow(`
		SELECT COUNT(*)
		FROM phaser_objects po
		JOIN phaser_maps pm ON pm.id = po.map_id
		WHERE pm.name = 'BIKE_SHOP'
		  AND po.name = 'BikeShop_NPC_1'
		  AND po.sprite_name = 'SPRITE_BIKE_SHOP_CLERK'
		  AND COALESCE(po.x, po.local_x) = 6
		  AND COALESCE(po.y, po.local_y) = 2
		  AND po.text = 'TEXT_BIKESHOP_CLERK'`).Scan(&clerkCount); err != nil {
		return fmt.Errorf("count Bike Shop clerk: %w", err)
	}
	if clerkCount != 1 {
		return fmt.Errorf("Bike Shop clerk count = %d, want 1", clerkCount)
	}

	log.Println("ok: Bike Shop clerk counter extends talk range")
	return nil
}

func requireMartMerchants(pg *sql.DB) error {
	rows, err := pg.Query(`
		SELECT id, name, COALESCE(map_name, ''), COALESCE(map_id::text, '')
		FROM cq_merchants cm
		WHERE map_id IS NULL
		   OR NOT EXISTS (SELECT 1 FROM phaser_maps pm WHERE pm.id = cm.map_id)
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("query linked mart merchants: %w", err)
	}
	defer rows.Close()

	var unlinked []string
	for rows.Next() {
		var id int
		var name, mapName, mapID string
		if err := rows.Scan(&id, &name, &mapName, &mapID); err != nil {
			return fmt.Errorf("scan linked mart merchant: %w", err)
		}
		unlinked = append(unlinked, fmt.Sprintf("%d %q map_name=%q map_id=%q", id, name, mapName, mapID))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read linked mart merchants: %w", err)
	}
	if len(unlinked) > 0 {
		return fmt.Errorf("unlinked CQ merchants: %v", unlinked)
	}

	var totalMerchants int
	if err := pg.QueryRow(`SELECT COUNT(*) FROM cq_merchants`).Scan(&totalMerchants); err != nil {
		return fmt.Errorf("count CQ merchants: %w", err)
	}

	var ceruleanMerchants int
	if err := pg.QueryRow(`
		SELECT COUNT(*)
		FROM cq_merchants cm
		JOIN phaser_maps pm ON pm.id = cm.map_id
		WHERE pm.name = 'CERULEAN_MART'`).Scan(&ceruleanMerchants); err != nil {
		return fmt.Errorf("count Cerulean Mart merchants: %w", err)
	}
	if ceruleanMerchants != 1 {
		return fmt.Errorf("Cerulean Mart merchant count = %d, want 1", ceruleanMerchants)
	}

	var ceruleanItems int
	if err := pg.QueryRow(`
		SELECT COUNT(*)
		FROM cq_merchant_items cmi
		JOIN cq_merchants cm ON cm.id = cmi.merchant_id
		JOIN phaser_maps pm ON pm.id = cm.map_id
		WHERE pm.name = 'CERULEAN_MART'`).Scan(&ceruleanItems); err != nil {
		return fmt.Errorf("count Cerulean Mart merchant items: %w", err)
	}
	if ceruleanItems == 0 {
		return fmt.Errorf("Cerulean Mart merchant has no items")
	}

	log.Printf("ok: %d CQ merchants are linked to maps; Cerulean Mart has %d shop items", totalMerchants, ceruleanItems)
	return nil
}

func requireInGameTrades(pg *sql.DB) error {
	expected := map[string]struct {
		TextConstant       string
		RequestedPokemon   string
		OfferedPokemon     string
		OfferedNickname    string
		OriginalTradeIndex int
	}{
		"TRADE_FOR_TERRY":    {"TEXT_ROUTE11GATE2F_YOUNGSTER", "NIDORINO", "NIDORINA", "TERRY", 0},
		"TRADE_FOR_MARCEL":   {"TEXT_ROUTE2TRADEHOUSE_GAMEBOY_KID", "ABRA", "MR_MIME", "MARCEL", 1},
		"TRADE_FOR_SAILOR":   {"TEXT_CINNABARLABFOSSILROOM_SCIENTIST2", "PONYTA", "SEEL", "SAILOR", 3},
		"TRADE_FOR_DUX":      {"TEXT_VERMILIONTRADEHOUSE_LITTLE_GIRL", "SPEAROW", "FARFETCHD", "DUX", 4},
		"TRADE_FOR_MARC":     {"TEXT_ROUTE18GATE2F_YOUNGSTER", "SLOWBRO", "LICKITUNG", "MARC", 5},
		"TRADE_FOR_LOLA":     {"TEXT_CERULEANTRADEHOUSE_GAMBLER", "POLIWHIRL", "JYNX", "LOLA", 6},
		"TRADE_FOR_DORIS":    {"TEXT_CINNABARLABTRADEROOM_GRAMPS", "RAICHU", "ELECTRODE", "DORIS", 7},
		"TRADE_FOR_CRINKLES": {"TEXT_CINNABARLABTRADEROOM_BEAUTY", "VENONAT", "TANGELA", "CRINKLES", 8},
		"TRADE_FOR_SPOT":     {"TEXT_UNDERGROUNDPATHROUTE5_LITTLE_GIRL", "NIDORAN_M", "NIDORAN_F", "SPOT", 9},
	}

	for tradeKey, want := range expected {
		var row struct {
			TextConstant       string
			RequestedPokemon   string
			OfferedPokemon     string
			OfferedNickname    string
			OriginalTradeIndex int
		}
		err := pg.QueryRow(`
			SELECT text_constant, requested_pokemon_name,
			       offered_pokemon_name, offered_nickname, original_trade_index
			FROM phaser_in_game_trades
			WHERE trade_key = $1`, tradeKey).Scan(
			&row.TextConstant,
			&row.RequestedPokemon,
			&row.OfferedPokemon,
			&row.OfferedNickname,
			&row.OriginalTradeIndex,
		)
		if err != nil {
			return fmt.Errorf("load in-game trade smoke row %s: %w", tradeKey, err)
		}
		if row.TextConstant != want.TextConstant ||
			row.RequestedPokemon != want.RequestedPokemon ||
			row.OfferedPokemon != want.OfferedPokemon ||
			row.OfferedNickname != want.OfferedNickname ||
			row.OriginalTradeIndex != want.OriginalTradeIndex {
			return fmt.Errorf("in-game trade %s mismatch: got %+v want %+v", tradeKey, row, want)
		}
	}
	log.Printf("ok: %d active Red/Blue NPC in-game trades are seeded", len(expected))
	return nil
}
