package main

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strings"
)

type merchantSeed struct {
	ID      int
	Name    string
	MapName string
}

type merchantItemSeed struct {
	MerchantID   int
	ItemID       int
	DisplayOrder int
}

type gameCornerPrizeSeed struct {
	ID        int
	Type      string
	PokemonID *int
	TMMoveID  *int
	ItemID    *int
	Name      string
	CoinCost  int
	SortOrder int
}

type elevatorFloorSeed struct {
	ElevatorMapName string
	FloorMapName    string
	FloorLabel      string
	DestX           int
	DestY           int
	SortOrder       int
	RequiresFlag    string
	RequiresItemID  *int
}

type inGameTradeSeed struct {
	TradeKey             string
	TextConstant         string
	MapName              string
	SourceFile           string
	ScriptLabel          string
	RequestedPokemonName string
	OfferedPokemonName   string
	OfferedNickname      string
	DialogueSet          string
	OriginalTradeIndex   int
}

type boulderTargetSeed struct {
	TargetFamily              string
	MapName                   string
	SourceLabel               string
	X                         int
	Y                         int
	Flag                      string
	DropsThroughHole          bool
	SourceMissableObject      string
	SourceObjectName          string
	DestinationMapName        string
	DestinationMissableObject string
	DestinationObjectName     string
	SourceFile                string
}

type pokeClassSeed struct {
	ID        int
	Name      string
	ClassType string
	Lore      string
}

type pokeFactionSeed struct {
	ID         int
	Name       string
	ShortName  string
	Lore       string
	IsPlayable bool
	IsStarting bool
}

type pokeStartCitySeed struct {
	ID          int
	MapID       int
	Name        string
	SpawnX      int
	SpawnY      int
	Description string
	SortOrder   int
}

type dialogueTextSeed struct {
	Label      string
	SourceFile string
	Dialogue   string
}

type textPointerSeed struct {
	MapName       string
	TextConstant  string
	LocalLabel    string
	DialogueLabel string
	PointerIndex  int
	IsTrainer     bool
}

type phaserObjectSeed struct {
	MapID        int
	ObjectType   string
	SpriteName   string
	Name         string
	LocalX       int
	LocalY       int
	MovementType string
	Text         string
}

type branchingDialogueSeed struct {
	MapName            string
	PromptTextConstant string
	PromptText         string
	YesDialogue        string
	NoDialogue         string
	YesActions         string
	NoActions          string
}

type disallowedWordSeed struct {
	Word     string
	Category string
}

func seedCaptureQuestRuntimeDataPostgres(pg *sql.DB, sqlite *sql.DB) error {
	log.Println("Seeding CaptureQuest runtime data from imported static data...")
	tx, err := pg.Begin()
	if err != nil {
		return fmt.Errorf("begin runtime seed transaction: %w", err)
	}
	defer tx.Rollback()

	if err := seedCharacterCreationDataPostgres(tx); err != nil {
		return err
	}
	if err := seedCQItemsPostgres(tx); err != nil {
		return err
	}
	if err := seedCQMerchantsPostgres(tx); err != nil {
		return err
	}
	if err := seedCQMerchantItemsPostgres(tx); err != nil {
		return err
	}
	if err := seedGameCornerPrizesPostgres(tx); err != nil {
		return err
	}
	if err := seedElevatorFloorsPostgres(tx); err != nil {
		return err
	}
	if err := seedCinnabarGymQuizPostgres(tx); err != nil {
		return err
	}
	if err := seedInGameTradesPostgres(tx, sqlite); err != nil {
		return err
	}
	if err := seedBoulderTargetsPostgres(tx, sqlite); err != nil {
		return err
	}
	if err := seedDisallowedWordsPostgres(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit runtime seed data: %w", err)
	}
	log.Println("  -> Seeded character creation data, CQ item templates, marts, Game Corner prizes, elevators, Cinnabar quiz data, in-game trades, boulder targets, and chat filter words")
	return nil
}

func seedCharacterCreationDataPostgres(tx *sql.Tx) error {
	if err := seedPokeClassesPostgres(tx); err != nil {
		return err
	}
	if err := seedPokeFactionsPostgres(tx); err != nil {
		return err
	}
	if err := seedPokeStartCitiesPostgres(tx); err != nil {
		return err
	}
	return nil
}

func seedPokeClassesPostgres(tx *sql.Tx) error {
	stmt, err := tx.Prepare(`
		INSERT INTO poke_classes (id, name, class_type, lore)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			class_type = EXCLUDED.class_type,
			lore = EXCLUDED.lore`)
	if err != nil {
		return fmt.Errorf("prepare Poke class seed: %w", err)
	}
	defer stmt.Close()

	for _, class := range pokeClassSeeds {
		if _, err := stmt.Exec(class.ID, class.Name, class.ClassType, class.Lore); err != nil {
			return fmt.Errorf("seed Poke class %d: %w", class.ID, err)
		}
	}
	return nil
}

func seedPokeFactionsPostgres(tx *sql.Tx) error {
	stmt, err := tx.Prepare(`
		INSERT INTO poke_factions (
			id, name, short_name, lore, is_playable, is_starting
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			short_name = EXCLUDED.short_name,
			lore = EXCLUDED.lore,
			is_playable = EXCLUDED.is_playable,
			is_starting = EXCLUDED.is_starting`)
	if err != nil {
		return fmt.Errorf("prepare Poke faction seed: %w", err)
	}
	defer stmt.Close()

	for _, faction := range pokeFactionSeeds {
		if _, err := stmt.Exec(
			faction.ID,
			faction.Name,
			faction.ShortName,
			faction.Lore,
			boolAsSmallInt(faction.IsPlayable),
			boolAsSmallInt(faction.IsStarting),
		); err != nil {
			return fmt.Errorf("seed Poke faction %d: %w", faction.ID, err)
		}
	}
	return nil
}

func seedPokeStartCitiesPostgres(tx *sql.Tx) error {
	stmt, err := tx.Prepare(`
		INSERT INTO poke_start_cities (
			id, map_id, name, spawn_x, spawn_y, description, sort_order
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			map_id = EXCLUDED.map_id,
			name = EXCLUDED.name,
			spawn_x = EXCLUDED.spawn_x,
			spawn_y = EXCLUDED.spawn_y,
			description = EXCLUDED.description,
			sort_order = EXCLUDED.sort_order`)
	if err != nil {
		return fmt.Errorf("prepare Poke start city seed: %w", err)
	}
	defer stmt.Close()

	for _, city := range pokeStartCitySeeds {
		if _, err := stmt.Exec(city.ID, city.MapID, city.Name, city.SpawnX, city.SpawnY, city.Description, city.SortOrder); err != nil {
			return fmt.Errorf("seed Poke start city %d: %w", city.ID, err)
		}
	}
	return nil
}

func boolAsSmallInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func seedDisallowedWordsPostgres(tx *sql.Tx) error {
	stmt, err := tx.Prepare(`
		INSERT INTO disallowed_words (word, category)
		VALUES ($1, $2)
		ON CONFLICT (word) DO UPDATE SET
			category = EXCLUDED.category`)
	if err != nil {
		return fmt.Errorf("prepare disallowed word seed: %w", err)
	}
	defer stmt.Close()

	for _, seed := range disallowedWordSeeds {
		if _, err := stmt.Exec(seed.Word, seed.Category); err != nil {
			return fmt.Errorf("seed disallowed word %q: %w", seed.Word, err)
		}
	}
	return nil
}

func seedCQItemsPostgres(tx *sql.Tx) error {
	rows, err := tx.Query(`
		SELECT id, name, short_name, price, vending_price, is_usable,
			uses_party_menu, is_key_item, is_guard_drink, move_id
		FROM phaser_items
		ORDER BY id`)
	if err != nil {
		return fmt.Errorf("load phaser_items for cq_items seed: %w", err)
	}

	var sources []phaserItemSeedRow
	for rows.Next() {
		var source phaserItemSeedRow
		if err := rows.Scan(
			&source.ID,
			&source.Name,
			&source.ShortName,
			&source.Price,
			&source.VendingPrice,
			&source.IsUsable,
			&source.UsesPartyMenu,
			&source.IsKeyItem,
			&source.IsGuardDrink,
			&source.MoveID,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan phaser item for cq_items seed: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("iterate phaser items for cq_items seed: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close phaser_items rows for cq_items seed: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO cq_items (
			id, name, short_name, price, vending_price, item_type,
			is_usable, uses_party_menu, is_key_item, is_guard_drink, move_id,
			stackable, stack_size, heal_amount, status_cure, pp_restore,
			revive_percent, ball_modifier, bonus_attack, bonus_defense,
			bonus_speed, bonus_special, bonus_accuracy, bonus_encounter_rate,
			bonus_crit, bonus_flee, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16,
			$17, $18, $19, $20,
			$21, $22, $23, $24,
			$25, $26, CURRENT_TIMESTAMP
		)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			short_name = EXCLUDED.short_name,
			price = EXCLUDED.price,
			vending_price = EXCLUDED.vending_price,
			item_type = EXCLUDED.item_type,
			is_usable = EXCLUDED.is_usable,
			uses_party_menu = EXCLUDED.uses_party_menu,
			is_key_item = EXCLUDED.is_key_item,
			is_guard_drink = EXCLUDED.is_guard_drink,
			move_id = EXCLUDED.move_id,
			stackable = EXCLUDED.stackable,
			stack_size = EXCLUDED.stack_size,
			heal_amount = EXCLUDED.heal_amount,
			status_cure = EXCLUDED.status_cure,
			pp_restore = EXCLUDED.pp_restore,
			revive_percent = EXCLUDED.revive_percent,
			ball_modifier = EXCLUDED.ball_modifier,
			bonus_attack = EXCLUDED.bonus_attack,
			bonus_defense = EXCLUDED.bonus_defense,
			bonus_speed = EXCLUDED.bonus_speed,
			bonus_special = EXCLUDED.bonus_special,
			bonus_accuracy = EXCLUDED.bonus_accuracy,
			bonus_encounter_rate = EXCLUDED.bonus_encounter_rate,
			bonus_crit = EXCLUDED.bonus_crit,
			bonus_flee = EXCLUDED.bonus_flee,
			updated_at = CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("prepare cq_items seed: %w", err)
	}
	defer stmt.Close()

	for _, source := range sources {
		item := buildCQItemSeed(source)
		if _, err := stmt.Exec(
			item.ID,
			item.Name,
			item.ShortName,
			item.Price,
			nullableInt64Value(item.VendingPrice),
			item.ItemType,
			item.IsUsable,
			item.UsesPartyMenu,
			item.IsKeyItem,
			item.IsGuardDrink,
			nullableInt64Value(item.MoveID),
			item.Stackable,
			item.StackSize,
			item.HealAmount,
			nullableStringValue(item.StatusCure),
			item.PPRestore,
			item.RevivePercent,
			item.BallModifier,
			item.BonusAttack,
			item.BonusDefense,
			item.BonusSpeed,
			item.BonusSpecial,
			item.BonusAccuracy,
			item.BonusEncounterRate,
			item.BonusCrit,
			item.BonusFlee,
		); err != nil {
			return fmt.Errorf("seed cq item %s: %w", item.ShortName, err)
		}
	}
	return nil
}

type phaserItemSeedRow struct {
	ID            int
	Name          string
	ShortName     string
	Price         sql.NullInt64
	VendingPrice  sql.NullInt64
	IsUsable      int
	UsesPartyMenu int
	IsKeyItem     int
	IsGuardDrink  int
	MoveID        sql.NullInt64
}

type cqItemSeedRow struct {
	ID                 int
	Name               string
	ShortName          string
	Price              int
	VendingPrice       sql.NullInt64
	ItemType           int
	IsUsable           bool
	UsesPartyMenu      bool
	IsKeyItem          bool
	IsGuardDrink       bool
	MoveID             sql.NullInt64
	Stackable          bool
	StackSize          int
	HealAmount         int
	StatusCure         sql.NullString
	PPRestore          int
	RevivePercent      int
	BallModifier       float64
	BonusAttack        int
	BonusDefense       int
	BonusSpeed         int
	BonusSpecial       int
	BonusAccuracy      int
	BonusEncounterRate int
	BonusCrit          int
	BonusFlee          int
}

var (
	evolutionStoneShortNames = map[string]bool{
		"MOON_STONE":    true,
		"FIRE_STONE":    true,
		"THUNDER_STONE": true,
		"WATER_STONE":   true,
		"LEAF_STONE":    true,
	}
	pokeBallShortNames = map[string]bool{
		"MASTER_BALL": true,
		"ULTRA_BALL":  true,
		"GREAT_BALL":  true,
		"POKE_BALL":   true,
		"SAFARI_BALL": true,
	}
	battleItemShortNames = map[string]bool{
		"X_ACCURACY": true,
		"GUARD_SPEC": true,
		"DIRE_HIT":   true,
		"X_ATTACK":   true,
		"X_DEFEND":   true,
		"X_SPEED":    true,
		"X_SPECIAL":  true,
	}
	fieldItemShortNames = map[string]bool{
		"BICYCLE":     true,
		"TOWN_MAP":    true,
		"ITEMFINDER":  true,
		"POKEDEX":     true,
		"EXP_ALL":     true,
		"OLD_ROD":     true,
		"GOOD_ROD":    true,
		"SUPER_ROD":   true,
		"REPEL":       true,
		"SUPER_REPEL": true,
		"MAX_REPEL":   true,
		"ESCAPE_ROPE": true,
		"POKE_FLUTE":  true,
		"POKE_DOLL":   true,
	}
	directUseFieldItemShortNames = map[string]bool{
		"BICYCLE":     true,
		"TOWN_MAP":    true,
		"ITEMFINDER":  true,
		"POKEDEX":     true,
		"EXP_ALL":     true,
		"OLD_ROD":     true,
		"GOOD_ROD":    true,
		"SUPER_ROD":   true,
		"REPEL":       true,
		"SUPER_REPEL": true,
		"MAX_REPEL":   true,
		"ESCAPE_ROPE": true,
		"POKE_FLUTE":  true,
	}
)

func buildCQItemSeed(source phaserItemSeedRow) cqItemSeedRow {
	shortName := normalizedCQItemShortName(source.ShortName)
	item := cqItemSeedRow{
		ID:            source.ID,
		Name:          normalizedCQItemName(source.Name, source.ShortName),
		ShortName:     shortName,
		Price:         0,
		VendingPrice:  source.VendingPrice,
		ItemType:      cqItemSeedType(source.ShortName, source.UsesPartyMenu != 0),
		IsUsable:      source.IsUsable != 0 || evolutionStoneShortNames[source.ShortName] || directUseFieldItemShortNames[source.ShortName],
		UsesPartyMenu: source.UsesPartyMenu != 0 || evolutionStoneShortNames[source.ShortName],
		IsKeyItem:     source.IsKeyItem != 0,
		IsGuardDrink:  source.IsGuardDrink != 0 || source.ShortName == "GUARD_SPEC",
		MoveID:        source.MoveID,
		Stackable:     source.IsKeyItem == 0 && !strings.HasPrefix(source.ShortName, "HM_"),
		StackSize:     99,
	}
	if source.Price.Valid {
		item.Price = int(source.Price.Int64)
	}
	item.HealAmount = cqItemSeedHealAmount(source.ShortName)
	item.StatusCure = cqItemSeedStatusCure(source.ShortName)
	item.PPRestore = cqItemSeedPPRestore(source.ShortName)
	item.RevivePercent = cqItemSeedRevivePercent(source.ShortName)
	item.BallModifier = cqItemSeedBallModifier(source.ShortName)
	item.BonusAttack = boolAsSmallInt(source.ShortName == "X_ATTACK")
	item.BonusDefense = boolAsSmallInt(source.ShortName == "X_DEFEND")
	item.BonusSpeed = boolAsSmallInt(source.ShortName == "X_SPEED")
	item.BonusSpecial = boolAsSmallInt(source.ShortName == "X_SPECIAL")
	item.BonusAccuracy = boolAsSmallInt(source.ShortName == "X_ACCURACY")
	item.BonusEncounterRate = cqItemSeedEncounterRate(source.ShortName)
	item.BonusCrit = boolAsSmallInt(source.ShortName == "DIRE_HIT")
	item.BonusFlee = boolAsSmallInt(source.ShortName == "POKE_DOLL")
	return item
}

func normalizedCQItemName(name, shortName string) string {
	switch shortName {
	case "ELIXER":
		return "ELIXIR"
	case "MAX_ELIXER":
		return "MAX ELIXIR"
	default:
		return name
	}
}

func normalizedCQItemShortName(shortName string) string {
	switch shortName {
	case "ELIXER":
		return "ELIXIR"
	case "MAX_ELIXER":
		return "MAX_ELIXIR"
	default:
		return shortName
	}
}

func cqItemSeedType(shortName string, usesPartyMenu bool) int {
	switch {
	case strings.HasPrefix(shortName, "TM_"):
		return 5
	case strings.HasPrefix(shortName, "HM_"):
		return 6
	case pokeBallShortNames[shortName]:
		return 1
	case evolutionStoneShortNames[shortName]:
		return 9
	case battleItemShortNames[shortName]:
		return 3
	case fieldItemShortNames[shortName]:
		return 4
	case usesPartyMenu:
		return 2
	default:
		return 0
	}
}

func cqItemSeedHealAmount(shortName string) int {
	switch shortName {
	case "POTION":
		return 20
	case "SUPER_POTION":
		return 50
	case "HYPER_POTION":
		return 200
	case "MAX_POTION", "FULL_RESTORE":
		return 999
	case "FRESH_WATER":
		return 50
	case "SODA_POP":
		return 60
	case "LEMONADE":
		return 80
	default:
		return 0
	}
}

func cqItemSeedStatusCure(shortName string) sql.NullString {
	switch shortName {
	case "ANTIDOTE":
		return sql.NullString{String: "poison", Valid: true}
	case "BURN_HEAL":
		return sql.NullString{String: "burn", Valid: true}
	case "ICE_HEAL":
		return sql.NullString{String: "freeze", Valid: true}
	case "AWAKENING":
		return sql.NullString{String: "sleep", Valid: true}
	case "PARLYZ_HEAL":
		return sql.NullString{String: "paralyze", Valid: true}
	case "FULL_RESTORE", "FULL_HEAL":
		return sql.NullString{String: "all", Valid: true}
	default:
		return sql.NullString{}
	}
}

func cqItemSeedPPRestore(shortName string) int {
	switch shortName {
	case "ETHER", "ELIXER":
		return 10
	case "MAX_ETHER", "MAX_ELIXER":
		return 999
	default:
		return 0
	}
}

func cqItemSeedRevivePercent(shortName string) int {
	switch shortName {
	case "REVIVE":
		return 50
	case "MAX_REVIVE":
		return 100
	default:
		return 0
	}
}

func cqItemSeedBallModifier(shortName string) float64 {
	switch shortName {
	case "MASTER_BALL":
		return 255
	case "ULTRA_BALL":
		return 2.0
	case "GREAT_BALL", "SAFARI_BALL":
		return 1.5
	case "POKE_BALL":
		return 1.0
	default:
		return 0
	}
}

func cqItemSeedEncounterRate(shortName string) int {
	switch shortName {
	case "REPEL":
		return -100
	case "SUPER_REPEL":
		return -200
	case "MAX_REPEL":
		return -250
	default:
		return 0
	}
}

func nullableInt64Value(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func nullableStringValue(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}

func seedCQMerchantsPostgres(tx *sql.Tx) error {
	stmt, err := tx.Prepare(`
		INSERT INTO cq_merchants (id, name, map_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			map_name = EXCLUDED.map_name`)
	if err != nil {
		return fmt.Errorf("prepare CQ merchant seed: %w", err)
	}
	defer stmt.Close()

	for _, merchant := range cqMerchantSeeds {
		if _, err := stmt.Exec(merchant.ID, merchant.Name, merchant.MapName); err != nil {
			return fmt.Errorf("seed CQ merchant %d: %w", merchant.ID, err)
		}
	}
	if _, err := tx.Exec(`
		UPDATE cq_merchants AS cm
		SET map_id = pm.id,
			map_name = pm.name
		FROM phaser_maps AS pm
		WHERE regexp_replace(lower(COALESCE(cm.map_name, '')), '[^a-z0-9]', '', 'g')
			= regexp_replace(lower(pm.name), '[^a-z0-9]', '', 'g')`); err != nil {
		return fmt.Errorf("link CQ merchants to phaser maps: %w", err)
	}
	return nil
}

func seedCQMerchantItemsPostgres(tx *sql.Tx) error {
	if _, err := tx.Exec(`DELETE FROM cq_merchant_items WHERE merchant_id BETWEEN 1 AND 14`); err != nil {
		return fmt.Errorf("clear CQ merchant item seed rows: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO cq_merchant_items (merchant_id, item_id, display_order)
		VALUES ($1, $2, $3)
		ON CONFLICT (merchant_id, item_id) DO UPDATE SET
			display_order = EXCLUDED.display_order`)
	if err != nil {
		return fmt.Errorf("prepare CQ merchant item seed: %w", err)
	}
	defer stmt.Close()

	for _, item := range cqMerchantItemSeeds {
		if _, err := stmt.Exec(item.MerchantID, item.ItemID, item.DisplayOrder); err != nil {
			return fmt.Errorf("seed CQ merchant %d item %d: %w", item.MerchantID, item.ItemID, err)
		}
	}
	return nil
}

func seedGameCornerPrizesPostgres(tx *sql.Tx) error {
	if _, err := tx.Exec(`DELETE FROM phaser_game_corner_prizes`); err != nil {
		return fmt.Errorf("clear Game Corner prize rows: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO phaser_game_corner_prizes
			(id, prize_type, pokemon_id, tm_move_id, item_id, prize_name, coin_cost, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			prize_type = EXCLUDED.prize_type,
			pokemon_id = EXCLUDED.pokemon_id,
			tm_move_id = EXCLUDED.tm_move_id,
			item_id = EXCLUDED.item_id,
			prize_name = EXCLUDED.prize_name,
			coin_cost = EXCLUDED.coin_cost,
			sort_order = EXCLUDED.sort_order`)
	if err != nil {
		return fmt.Errorf("prepare Game Corner prize seed: %w", err)
	}
	defer stmt.Close()

	for _, prize := range gameCornerPrizeSeeds {
		if _, err := stmt.Exec(prize.ID, prize.Type, prize.PokemonID, prize.TMMoveID, prize.ItemID, prize.Name, prize.CoinCost, prize.SortOrder); err != nil {
			return fmt.Errorf("seed Game Corner prize %d: %w", prize.ID, err)
		}
	}
	return nil
}

func seedElevatorFloorsPostgres(tx *sql.Tx) error {
	mapIDs, err := loadPhaserMapIDs(tx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM phaser_elevator_floors`); err != nil {
		return fmt.Errorf("clear elevator floor rows: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO phaser_elevator_floors (
			elevator_map_id, floor_map_id, floor_label, dest_x, dest_y,
			sort_order, requires_flag, requires_item_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (elevator_map_id, floor_map_id) DO UPDATE SET
			floor_label = EXCLUDED.floor_label,
			dest_x = EXCLUDED.dest_x,
			dest_y = EXCLUDED.dest_y,
			sort_order = EXCLUDED.sort_order,
			requires_flag = EXCLUDED.requires_flag,
			requires_item_id = EXCLUDED.requires_item_id`)
	if err != nil {
		return fmt.Errorf("prepare elevator floor seed: %w", err)
	}
	defer stmt.Close()

	for _, floor := range elevatorFloorSeeds {
		elevatorMapID, ok := mapIDs[floor.ElevatorMapName]
		if !ok {
			return fmt.Errorf("seed elevator floor %s -> %s: missing elevator map", floor.ElevatorMapName, floor.FloorMapName)
		}
		floorMapID, ok := mapIDs[floor.FloorMapName]
		if !ok {
			return fmt.Errorf("seed elevator floor %s -> %s: missing destination map", floor.ElevatorMapName, floor.FloorMapName)
		}
		var requiresFlag any
		if floor.RequiresFlag != "" {
			requiresFlag = floor.RequiresFlag
		}
		if _, err := stmt.Exec(
			elevatorMapID,
			floorMapID,
			floor.FloorLabel,
			floor.DestX,
			floor.DestY,
			floor.SortOrder,
			requiresFlag,
			floor.RequiresItemID,
		); err != nil {
			return fmt.Errorf("seed elevator floor %s -> %s: %w", floor.ElevatorMapName, floor.FloorMapName, err)
		}
	}
	return nil
}

func loadPhaserMapIDs(tx *sql.Tx) (map[string]int, error) {
	rows, err := tx.Query(`SELECT name, id FROM phaser_maps`)
	if err != nil {
		return nil, fmt.Errorf("load phaser map ids: %w", err)
	}
	defer rows.Close()

	ids := map[string]int{}
	for rows.Next() {
		var name string
		var id int
		if err := rows.Scan(&name, &id); err != nil {
			return nil, fmt.Errorf("scan phaser map id: %w", err)
		}
		ids[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate phaser map ids: %w", err)
	}
	return ids, nil
}

func seedCinnabarGymQuizPostgres(tx *sql.Tx) error {
	if err := seedDialogueTextPostgres(tx, cinnabarGymQuizDialogueTextSeeds); err != nil {
		return fmt.Errorf("seed Cinnabar Gym quiz dialogue text: %w", err)
	}
	if err := seedTextPointersPostgres(tx, cinnabarGymQuizTextPointerSeeds); err != nil {
		return fmt.Errorf("seed Cinnabar Gym quiz text pointers: %w", err)
	}
	if err := seedPhaserObjectsPostgres(tx, cinnabarGymQuizObjectSeeds); err != nil {
		return fmt.Errorf("seed Cinnabar Gym quiz signs: %w", err)
	}
	if err := seedBranchingDialoguePostgres(tx, cinnabarGymQuizBranchingDialogueSeeds); err != nil {
		return fmt.Errorf("seed Cinnabar Gym quiz branching dialogue: %w", err)
	}
	return nil
}

func seedDialogueTextPostgres(tx *sql.Tx, seeds []dialogueTextSeed) error {
	stmt, err := tx.Prepare(`
		INSERT INTO phaser_dialogue_text (label, source_file, dialogue)
		VALUES ($1, $2, $3)
		ON CONFLICT (label) DO UPDATE SET
			source_file = EXCLUDED.source_file,
			dialogue = EXCLUDED.dialogue`)
	if err != nil {
		return fmt.Errorf("prepare dialogue text seed: %w", err)
	}
	defer stmt.Close()

	for _, seed := range seeds {
		if _, err := stmt.Exec(seed.Label, seed.SourceFile, seed.Dialogue); err != nil {
			return fmt.Errorf("seed dialogue text %s: %w", seed.Label, err)
		}
	}
	return nil
}

func seedTextPointersPostgres(tx *sql.Tx, seeds []textPointerSeed) error {
	stmt, err := tx.Prepare(`
		INSERT INTO phaser_text_pointers (
			map_name, text_constant, local_label, dialogue_label, pointer_index, is_trainer
		)
		VALUES ($1, $2, $3, $4, $5, $6)`)
	if err != nil {
		return fmt.Errorf("prepare text pointer seed: %w", err)
	}
	defer stmt.Close()

	for _, seed := range seeds {
		if _, err := tx.Exec(`DELETE FROM phaser_text_pointers WHERE text_constant = $1`, seed.TextConstant); err != nil {
			return fmt.Errorf("clear text pointer %s: %w", seed.TextConstant, err)
		}
		if _, err := stmt.Exec(
			seed.MapName,
			seed.TextConstant,
			seed.LocalLabel,
			seed.DialogueLabel,
			seed.PointerIndex,
			boolAsSmallInt(seed.IsTrainer),
		); err != nil {
			return fmt.Errorf("seed text pointer %s: %w", seed.TextConstant, err)
		}
	}
	return nil
}

func seedPhaserObjectsPostgres(tx *sql.Tx, seeds []phaserObjectSeed) error {
	stmt, err := tx.Prepare(`
		INSERT INTO phaser_objects (
			map_id, object_type, sprite_name, name, local_x, local_y, movement_type, text
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`)
	if err != nil {
		return fmt.Errorf("prepare phaser object seed: %w", err)
	}
	defer stmt.Close()

	for _, seed := range seeds {
		if _, err := tx.Exec(`DELETE FROM phaser_objects WHERE map_id = $1 AND text = $2`, seed.MapID, seed.Text); err != nil {
			return fmt.Errorf("clear phaser object %s: %w", seed.Text, err)
		}
		if _, err := stmt.Exec(
			seed.MapID,
			seed.ObjectType,
			seed.SpriteName,
			seed.Name,
			seed.LocalX,
			seed.LocalY,
			seed.MovementType,
			seed.Text,
		); err != nil {
			return fmt.Errorf("seed phaser object %s: %w", seed.Text, err)
		}
	}
	return nil
}

func seedBranchingDialoguePostgres(tx *sql.Tx, seeds []branchingDialogueSeed) error {
	stmt, err := tx.Prepare(`
		INSERT INTO phaser_branching_dialogue (
			map_name, prompt_text_constant, prompt_text, yes_dialogue, no_dialogue, yes_actions, no_actions
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb)
		ON CONFLICT (prompt_text_constant) DO UPDATE SET
			map_name = EXCLUDED.map_name,
			prompt_text = EXCLUDED.prompt_text,
			yes_dialogue = EXCLUDED.yes_dialogue,
			no_dialogue = EXCLUDED.no_dialogue,
			yes_actions = EXCLUDED.yes_actions,
			no_actions = EXCLUDED.no_actions`)
	if err != nil {
		return fmt.Errorf("prepare branching dialogue seed: %w", err)
	}
	defer stmt.Close()

	for _, seed := range seeds {
		if _, err := stmt.Exec(
			seed.MapName,
			seed.PromptTextConstant,
			seed.PromptText,
			seed.YesDialogue,
			seed.NoDialogue,
			seed.YesActions,
			seed.NoActions,
		); err != nil {
			return fmt.Errorf("seed branching dialogue %s: %w", seed.PromptTextConstant, err)
		}
	}
	return nil
}

func seedInGameTradesPostgres(tx *sql.Tx, sqlite *sql.DB) error {
	trades, err := inGameTradeSeedsForImport(sqlite)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM phaser_in_game_trades`); err != nil {
		return fmt.Errorf("clear in-game trade rows: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO phaser_in_game_trades (
			trade_key, text_constant, map_name, source_file, script_label,
			requested_pokemon_id, requested_pokemon_name,
			offered_pokemon_id, offered_pokemon_name, offered_nickname,
			dialogue_set, original_trade_index
		)
		SELECT
			$1, $2, $3, $4, $5,
			requested.id, requested.name,
			offered.id, offered.name, $8,
			$9, $10
		FROM phaser_pokemon requested
		CROSS JOIN phaser_pokemon offered
		WHERE requested.name = $6 AND offered.name = $7
		ON CONFLICT (trade_key) DO UPDATE SET
			text_constant = EXCLUDED.text_constant,
			map_name = EXCLUDED.map_name,
			source_file = EXCLUDED.source_file,
			script_label = EXCLUDED.script_label,
			requested_pokemon_id = EXCLUDED.requested_pokemon_id,
			requested_pokemon_name = EXCLUDED.requested_pokemon_name,
			offered_pokemon_id = EXCLUDED.offered_pokemon_id,
			offered_pokemon_name = EXCLUDED.offered_pokemon_name,
			offered_nickname = EXCLUDED.offered_nickname,
			dialogue_set = EXCLUDED.dialogue_set,
			original_trade_index = EXCLUDED.original_trade_index`)
	if err != nil {
		return fmt.Errorf("prepare in-game trade seed: %w", err)
	}
	defer stmt.Close()

	for _, trade := range trades {
		result, err := stmt.Exec(
			trade.TradeKey,
			trade.TextConstant,
			trade.MapName,
			trade.SourceFile,
			trade.ScriptLabel,
			trade.RequestedPokemonName,
			trade.OfferedPokemonName,
			trade.OfferedNickname,
			trade.DialogueSet,
			trade.OriginalTradeIndex,
		)
		if err != nil {
			return fmt.Errorf("seed in-game trade %s: %w", trade.TradeKey, err)
		}
		if rows, _ := result.RowsAffected(); rows == 0 {
			return fmt.Errorf("seed in-game trade %s: missing pokemon %s or %s",
				trade.TradeKey, trade.RequestedPokemonName, trade.OfferedPokemonName)
		}
	}
	return nil
}

func inGameTradeSeedsForImport(sqlite *sql.DB) ([]inGameTradeSeed, error) {
	trades, found, err := loadInGameTradeSeedsFromSQLite(sqlite)
	if err != nil {
		return nil, err
	}
	if found {
		if len(trades) == 0 {
			return nil, fmt.Errorf("script_event_in_game_trades exists but has no active trade rows")
		}
		log.Printf("  -> Loaded %d active in-game trades from extractor data", len(trades))
		return trades, nil
	}
	log.Printf("  -> Extractor in-game trade table missing; using built-in fallback seeds")
	return inGameTradeSeeds, nil
}

func loadInGameTradeSeedsFromSQLite(sqlite *sql.DB) ([]inGameTradeSeed, bool, error) {
	if sqlite == nil {
		return nil, false, nil
	}
	exists, err := sqliteTableExists(sqlite, "script_event_in_game_trades")
	if err != nil {
		return nil, false, fmt.Errorf("check script_event_in_game_trades: %w", err)
	}
	if !exists {
		return nil, false, nil
	}

	rows, err := sqlite.Query(`
		SELECT trade_key, text_constant, map_name, source_file, script_label,
		       requested_pokemon, offered_pokemon, offered_nickname,
		       dialogue_set, original_trade_index
		FROM script_event_in_game_trades
		WHERE active = 1
		ORDER BY original_trade_index`)
	if err != nil {
		return nil, false, fmt.Errorf("query script_event_in_game_trades: %w", err)
	}
	defer rows.Close()

	trades := []inGameTradeSeed{}
	for rows.Next() {
		var trade inGameTradeSeed
		var mapName string
		if err := rows.Scan(
			&trade.TradeKey,
			&trade.TextConstant,
			&mapName,
			&trade.SourceFile,
			&trade.ScriptLabel,
			&trade.RequestedPokemonName,
			&trade.OfferedPokemonName,
			&trade.OfferedNickname,
			&trade.DialogueSet,
			&trade.OriginalTradeIndex,
		); err != nil {
			return nil, true, err
		}
		trade.MapName = mapNameToUpperSnake(mapName)
		trade.SourceFile = strings.TrimPrefix(trade.SourceFile, "pokemon-game-data/")
		trades = append(trades, trade)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return trades, true, nil
}

func seedBoulderTargetsPostgres(tx *sql.Tx, sqlite *sql.DB) error {
	targets, err := boulderTargetSeedsForImport(sqlite)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM phaser_boulder_targets`); err != nil {
		return fmt.Errorf("clear boulder target rows: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO phaser_boulder_targets (
			target_family, map_name, source_label, x, y, flag, drops_through_hole,
			source_missable_object, source_object_name,
			destination_map_name, destination_missable_object, destination_object_name,
			source_file
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (target_family, map_name, x, y) DO UPDATE SET
			source_label = EXCLUDED.source_label,
			flag = EXCLUDED.flag,
			drops_through_hole = EXCLUDED.drops_through_hole,
			source_missable_object = EXCLUDED.source_missable_object,
			source_object_name = EXCLUDED.source_object_name,
			destination_map_name = EXCLUDED.destination_map_name,
			destination_missable_object = EXCLUDED.destination_missable_object,
			destination_object_name = EXCLUDED.destination_object_name,
			source_file = EXCLUDED.source_file`)
	if err != nil {
		return fmt.Errorf("prepare boulder target seed: %w", err)
	}
	defer stmt.Close()

	for _, target := range targets {
		if target.SourceObjectName == "" && target.SourceMissableObject != "" {
			objectName, err := resolveMissableObjectName(tx, target.SourceMissableObject)
			if err != nil {
				return fmt.Errorf("resolve source missable object %s: %w", target.SourceMissableObject, err)
			}
			target.SourceObjectName = objectName
		}
		if target.DestinationObjectName == "" && target.DestinationMissableObject != "" {
			objectName, err := resolveMissableObjectName(tx, target.DestinationMissableObject)
			if err != nil {
				return fmt.Errorf("resolve destination missable object %s: %w", target.DestinationMissableObject, err)
			}
			target.DestinationObjectName = objectName
		}
		if _, err := stmt.Exec(
			target.TargetFamily,
			target.MapName,
			target.SourceLabel,
			target.X,
			target.Y,
			target.Flag,
			target.DropsThroughHole,
			target.SourceMissableObject,
			target.SourceObjectName,
			target.DestinationMapName,
			target.DestinationMissableObject,
			target.DestinationObjectName,
			target.SourceFile,
		); err != nil {
			return fmt.Errorf("seed boulder target %s %s (%d,%d): %w",
				target.TargetFamily, target.MapName, target.X, target.Y, err)
		}
	}
	return nil
}

func boulderTargetSeedsForImport(sqlite *sql.DB) ([]boulderTargetSeed, error) {
	targets, found, err := loadBoulderTargetSeedsFromSQLite(sqlite)
	if err != nil {
		return nil, err
	}
	if found {
		if len(targets) == 0 {
			return nil, fmt.Errorf("script_event_boulder_targets exists but has no target rows")
		}
		log.Printf("  -> Loaded %d boulder targets from extractor data", len(targets))
		return targets, nil
	}
	log.Printf("  -> Extractor boulder target table missing; using built-in fallback seeds")
	return boulderTargetSeeds, nil
}

func loadBoulderTargetSeedsFromSQLite(sqlite *sql.DB) ([]boulderTargetSeed, bool, error) {
	if sqlite == nil {
		return nil, false, nil
	}
	exists, err := sqliteTableExists(sqlite, "script_event_boulder_targets")
	if err != nil {
		return nil, false, fmt.Errorf("check script_event_boulder_targets: %w", err)
	}
	if !exists {
		return nil, false, nil
	}

	rows, err := sqlite.Query(`
		SELECT target_family, map_name, source_label, x, y, flag,
		       drops_through_hole, source_missable_object,
		       destination_map_name, destination_missable_object, source_file
		FROM script_event_boulder_targets
		ORDER BY target_family, map_name, x, y`)
	if err != nil {
		return nil, false, fmt.Errorf("query script_event_boulder_targets: %w", err)
	}
	defer rows.Close()

	targets := []boulderTargetSeed{}
	for rows.Next() {
		var target boulderTargetSeed
		var mapName, destinationMapName string
		if err := rows.Scan(
			&target.TargetFamily,
			&mapName,
			&target.SourceLabel,
			&target.X,
			&target.Y,
			&target.Flag,
			&target.DropsThroughHole,
			&target.SourceMissableObject,
			&destinationMapName,
			&target.DestinationMissableObject,
			&target.SourceFile,
		); err != nil {
			return nil, true, err
		}
		target.MapName = mapNameToUpperSnake(mapName)
		target.DestinationMapName = mapNameToUpperSnake(destinationMapName)
		target.SourceFile = strings.TrimPrefix(target.SourceFile, "pokemon-game-data/")
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return targets, true, nil
}

func resolveMissableObjectName(tx *sql.Tx, hsConstant string) (string, error) {
	var objectName string
	err := tx.QueryRow(
		`SELECT object_name FROM phaser_missable_objects WHERE hs_constant = $1 AND object_name IS NOT NULL`,
		hsConstant,
	).Scan(&objectName)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("missable object %s not found", hsConstant)
	}
	return objectName, err
}

func sqliteTableExists(sqlite *sql.DB, table string) (bool, error) {
	var name string
	err := sqlite.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return name == table, nil
}

func mapNameToUpperSnake(value string) string {
	converted := strings.ToUpper(camelToSnake(value))
	converted = floorSuffixPattern.ReplaceAllString(converted, "_${1}F")
	return basementFloorPattern.ReplaceAllString(converted, "_B${1}F")
}

var acronymBoundaryPattern = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
var lowerUpperBoundaryPattern = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var alphaDigitBoundaryPattern = regexp.MustCompile(`([A-Za-z])([0-9])`)
var nonIdentifierPattern = regexp.MustCompile(`[^A-Za-z0-9]+`)
var floorSuffixPattern = regexp.MustCompile(`_((?:B)?\d+)_F\b`)
var basementFloorPattern = regexp.MustCompile(`_B_(\d+)F\b`)

func camelToSnake(value string) string {
	value = strings.TrimSpace(value)
	value = nonIdentifierPattern.ReplaceAllString(value, "_")
	value = acronymBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	value = lowerUpperBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	value = alphaDigitBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	return strings.Trim(strings.ToLower(value), "_")
}

func intPtr(value int) *int {
	return &value
}

var pokeClassSeeds = []pokeClassSeed{
	{1, "Bug Catcher", "Bug", "A hobbyist who spends their time scouring forests for various insect Pokemon."},
	{2, "Hiker", "Rock", "A sturdy traveler who feels at home among the jagged peaks and rocky tunnels of the mountains."},
	{3, "Fisher", "Water", "A patient soul who waits by the water's edge for a bite from the depths."},
	{4, "Biker", "Poison", "A rebel of the roads who favors tough Pokemon that can handle the smog and city streets."},
	{5, "Burglar", "Fire", "A sly character who knows their way around locked doors and fiery companions."},
	{6, "Juggler", "Psychic", "A performer who uses psychic abilities and quick hands to keep their Pokemon in a perfect rhythm."},
	{7, "Bird Keeper", "Flying", "A lover of the skies who trains Pokemon that soar above the clouds."},
	{8, "Sailor", "Water", "A hardy mariner who has weathered many storms alongside their aquatic partners."},
	{9, "Black Belt", "Fighting", "A dedicated martial artist who trains alongside Fighting-type Pokemon to reach physical perfection."},
	{10, "Mystic", "Psychic", "A student of the arcane and spiritual, guiding Pokemon with ancient powers."},
	{11, "Adventurer", "Normal", "A versatile explorer prepared for any eventuality in the wild."},
	{12, "Miner", "Electric", "A worker of the depths who uses Electric-type Pokemon to light the way and shatter rocks."},
}

var pokeFactionSeeds = []pokeFactionSeed{
	{
		ID:         1,
		Name:       "Pallet Pioneers",
		ShortName:  "PAL",
		Lore:       "Pallet Pioneers are trainers shaped by quiet roads, tall grass, and Professor Oak's lab at the edge of town.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         2,
		Name:       "Viridian Rangers",
		ShortName:  "VIR",
		Lore:       "Viridian Rangers grow up around forest trails, bug catchers, and the first true stretch of wilderness north of Pallet.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         3,
		Name:       "Pewter Pathfinders",
		ShortName:  "PEW",
		Lore:       "Pewter Pathfinders live beside stone museums, rocky gyms, and the cave paths that lead toward Mt. Moon.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         4,
		Name:       "Cerulean Surfers",
		ShortName:  "CER",
		Lore:       "Cerulean Surfers call bridge crossings, blue water, and Misty's gym their familiar backyard.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         5,
		Name:       "Vermilion Voyagers",
		ShortName:  "VER",
		Lore:       "Vermilion Voyagers are dockside trainers raised near ships, trade routes, and Lt. Surge's electric gym.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         6,
		Name:       "Lavender Lookouts",
		ShortName:  "LAV",
		Lore:       "Lavender Lookouts come from a quiet town watched over by Pokemon Tower and its ghost stories.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         7,
		Name:       "Celadon Scouts",
		ShortName:  "CEL",
		Lore:       "Celadon Scouts know department stores, city gardens, and the bright distractions of Kanto's biggest shopping district.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         8,
		Name:       "Saffron Sentinels",
		ShortName:  "SAF",
		Lore:       "Saffron Sentinels come from Kanto's crossroads city, home to Silph Co., fighting dojos, and psychic challengers.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         10,
		Name:       "Fuchsia Fighters",
		ShortName:  "FUC",
		Lore:       "Fuchsia Fighters grow up near the Safari Zone, winding fences, and Koga's hidden gym tricks.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         11,
		Name:       "Cinnabar Scholars",
		ShortName:  "CIN",
		Lore:       "Cinnabar Scholars are island trainers shaped by volcano heat, old mansion rumors, and Blaine's research-minded gym.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         12,
		Name:       "Route Runners",
		ShortName:  "RTE",
		Lore:       "Route Runners are restless travelers from the long roads, bike paths, and detours between Kanto's towns.",
		IsPlayable: true,
		IsStarting: true,
	},
	{
		ID:         13,
		Name:       "Victory Vanguard",
		ShortName:  "VIC",
		Lore:       "Victory Vanguard trainers dream of Indigo Plateau, Victory Road, and the final climb toward the Pokemon League.",
		IsPlayable: true,
		IsStarting: false,
	},
}

var pokeStartCitySeeds = []pokeStartCitySeed{
	{
		ID:          1,
		MapID:       0,
		Name:        "Pallet Town",
		SpawnX:      8,
		SpawnY:      6,
		Description: "A quiet town where new trainers begin their journey.",
		SortOrder:   1,
	},
	{
		ID:          2,
		MapID:       1,
		Name:        "Viridian City",
		SpawnX:      12,
		SpawnY:      10,
		Description: "A forest-edge city watched over by the mysterious final Gym.",
		SortOrder:   2,
	},
	{
		ID:          3,
		MapID:       2,
		Name:        "Pewter City",
		SpawnX:      10,
		SpawnY:      10,
		Description: "A rugged northern city known for Brock's Gym and its museum.",
		SortOrder:   3,
	},
	{
		ID:          4,
		MapID:       3,
		Name:        "Cerulean City",
		SpawnX:      10,
		SpawnY:      10,
		Description: "A bright waterside city near Nugget Bridge and Misty's Gym.",
		SortOrder:   4,
	},
	{
		ID:          5,
		MapID:       5,
		Name:        "Vermilion City",
		SpawnX:      10,
		SpawnY:      10,
		Description: "A busy harbor city where ships, sailors, and Electric-type battles gather.",
		SortOrder:   5,
	},
	{
		ID:          6,
		MapID:       4,
		Name:        "Lavender Town",
		SpawnX:      5,
		SpawnY:      5,
		Description: "A quiet town below Pokemon Tower, wrapped in ghost stories and memory.",
		SortOrder:   6,
	},
	{
		ID:          7,
		MapID:       6,
		Name:        "Celadon City",
		SpawnX:      12,
		SpawnY:      10,
		Description: "Kanto's department-store city, packed with gardens, prizes, and rumors.",
		SortOrder:   7,
	},
	{
		ID:          8,
		MapID:       7,
		Name:        "Fuchsia City",
		SpawnX:      10,
		SpawnY:      10,
		Description: "A southern city beside the Safari Zone and Koga's ninja Gym.",
		SortOrder:   8,
	},
	{
		ID:          9,
		MapID:       10,
		Name:        "Saffron City",
		SpawnX:      10,
		SpawnY:      10,
		Description: "Kanto's golden crossroads, home to Silph Co. and Sabrina's Gym.",
		SortOrder:   9,
	},
	{
		ID:          10,
		MapID:       8,
		Name:        "Cinnabar Island",
		SpawnX:      5,
		SpawnY:      5,
		Description: "A volcanic island of laboratories, fossils, and Blaine's fiery quiz Gym.",
		SortOrder:   10,
	},
}

var disallowedWordSeeds = []disallowedWordSeed{
	{Word: "nigger", Category: "racial"},
	{Word: "nigga", Category: "racial"},
	{Word: "niggas", Category: "racial"},
	{Word: "coon", Category: "racial"},
	{Word: "porch monkey", Category: "racial"},
	{Word: "porchmonkey", Category: "racial"},
	{Word: "jigaboo", Category: "racial"},
	{Word: "jiggaboo", Category: "racial"},
	{Word: "jig", Category: "racial"},
	{Word: "spook", Category: "racial"},
	{Word: "tarbaby", Category: "racial"},
	{Word: "tar baby", Category: "racial"},
	{Word: "darky", Category: "racial"},
	{Word: "pickaninny", Category: "racial"},
	{Word: "chink", Category: "racial"},
	{Word: "gook", Category: "racial"},
	{Word: "zipperhead", Category: "racial"},
	{Word: "spic", Category: "racial"},
	{Word: "wetback", Category: "racial"},
	{Word: "beaner", Category: "racial"},
	{Word: "kike", Category: "racial"},
	{Word: "heeb", Category: "racial"},
	{Word: "raghead", Category: "racial"},
	{Word: "sandnigger", Category: "racial"},
	{Word: "sandnigga", Category: "racial"},
	{Word: "towelhead", Category: "racial"},
	{Word: "faggot", Category: "homophobic"},
	{Word: "faggots", Category: "homophobic"},
	{Word: "fag", Category: "homophobic"},
	{Word: "fags", Category: "homophobic"},
	{Word: "tranny", Category: "transphobic"},
	{Word: "tranney", Category: "transphobic"},
	{Word: "trannies", Category: "transphobic"},
	{Word: "trannys", Category: "transphobic"},
	{Word: "dyke", Category: "homophobic"},
}

var cqMerchantSeeds = []merchantSeed{
	{1, "Viridian City Poke Mart", "VIRIDIAN_MART"},
	{2, "Pewter City Poke Mart", "PEWTER_MART"},
	{3, "Cerulean City Poke Mart", "CERULEAN_MART"},
	{4, "Vermilion City Poke Mart", "VERMILION_MART"},
	{5, "Lavender Town Poke Mart", "LAVENDER_MART"},
	{6, "Celadon Dept. Store 2F (Items)", "CELADON_MART_2F"},
	{7, "Celadon Dept. Store 2F (TMs)", "CELADON_MART_2F"},
	{8, "Celadon Dept. Store 4F", "CELADON_MART_4F"},
	{9, "Celadon Dept. Store 5F (Battle)", "CELADON_MART_5F"},
	{10, "Celadon Dept. Store 5F (Vitamins)", "CELADON_MART_5F"},
	{11, "Fuchsia City Poke Mart", "FUCHSIA_MART"},
	{12, "Cinnabar Island Poke Mart", "CINNABAR_MART"},
	{13, "Saffron City Poke Mart", "SAFFRON_MART"},
	{14, "Indigo Plateau Poke Mart", "INDIGO_PLATEAU_LOBBY"},
}

var cqMerchantItemSeeds = []merchantItemSeed{
	{1, 4, 1}, {1, 11, 2}, {1, 15, 3}, {1, 12, 4},
	{2, 4, 1}, {2, 20, 2}, {2, 29, 3}, {2, 11, 4}, {2, 12, 5}, {2, 14, 6}, {2, 15, 7},
	{3, 4, 1}, {3, 20, 2}, {3, 30, 3}, {3, 11, 4}, {3, 12, 5}, {3, 14, 6}, {3, 15, 7},
	{4, 4, 1}, {4, 19, 2}, {4, 13, 3}, {4, 14, 4}, {4, 15, 5}, {4, 30, 6},
	{5, 3, 1}, {5, 19, 2}, {5, 53, 3}, {5, 29, 4}, {5, 56, 5}, {5, 11, 6}, {5, 12, 7}, {5, 13, 8}, {5, 15, 9},
	{6, 3, 1}, {6, 19, 2}, {6, 53, 3}, {6, 56, 4}, {6, 11, 5}, {6, 12, 6}, {6, 13, 7}, {6, 14, 8}, {6, 15, 9},
	{7, 120, 1}, {7, 121, 2}, {7, 90, 3}, {7, 95, 4}, {7, 125, 5}, {7, 89, 6}, {7, 93, 7}, {7, 97, 8}, {7, 105, 9},
	{8, 51, 1}, {8, 32, 2}, {8, 33, 3}, {8, 34, 4}, {8, 47, 5},
	{9, 46, 1}, {9, 55, 2}, {9, 58, 3}, {9, 65, 4}, {9, 66, 5}, {9, 67, 6}, {9, 68, 7},
	{10, 35, 1}, {10, 36, 2}, {10, 37, 3}, {10, 38, 4}, {10, 39, 5},
	{11, 2, 1}, {11, 3, 2}, {11, 19, 3}, {11, 53, 4}, {11, 52, 5}, {11, 56, 6},
	{12, 2, 1}, {12, 3, 2}, {12, 18, 3}, {12, 57, 4}, {12, 29, 5}, {12, 52, 6}, {12, 53, 7},
	{13, 3, 1}, {13, 18, 2}, {13, 57, 3}, {13, 29, 4}, {13, 52, 5}, {13, 53, 6},
	{14, 2, 1}, {14, 3, 2}, {14, 16, 3}, {14, 17, 4}, {14, 52, 5}, {14, 53, 6}, {14, 57, 7},
}

var gameCornerPrizeSeeds = []gameCornerPrizeSeed{
	{1, "pokemon", intPtr(63), nil, nil, "ABRA", 180, 1},
	{2, "pokemon", intPtr(35), nil, nil, "CLEFAIRY", 500, 2},
	{3, "pokemon", intPtr(30), nil, nil, "NIDORINA", 1200, 3},
	{4, "pokemon", intPtr(147), nil, nil, "DRATINI", 2800, 4},
	{5, "pokemon", intPtr(123), nil, nil, "SCYTHER", 5500, 5},
	{6, "pokemon", intPtr(137), nil, nil, "PORYGON", 9999, 6},
	{7, "tm", nil, intPtr(82), intPtr(111), "TM23 Dragon Rage", 3300, 7},
	{8, "tm", nil, intPtr(63), intPtr(103), "TM15 Hyper Beam", 5500, 8},
	{9, "tm", nil, intPtr(164), intPtr(138), "TM50 Substitute", 7700, 9},
}

var elevatorFloorSeeds = []elevatorFloorSeed{
	{"CELADON_MART_ELEVATOR", "CELADON_MART_1F", "1F", 1, 1, 1, "", nil},
	{"CELADON_MART_ELEVATOR", "CELADON_MART_2F", "2F", 1, 1, 2, "", nil},
	{"CELADON_MART_ELEVATOR", "CELADON_MART_3F", "3F", 1, 1, 3, "", nil},
	{"CELADON_MART_ELEVATOR", "CELADON_MART_4F", "4F", 1, 1, 4, "", nil},
	{"CELADON_MART_ELEVATOR", "CELADON_MART_5F", "5F", 1, 1, 5, "", nil},

	{"ROCKET_HIDEOUT_ELEVATOR", "ROCKET_HIDEOUT_B1F", "B1F", 24, 19, 1, "", intPtr(74)},
	{"ROCKET_HIDEOUT_ELEVATOR", "ROCKET_HIDEOUT_B2F", "B2F", 24, 19, 2, "", intPtr(74)},
	{"ROCKET_HIDEOUT_ELEVATOR", "ROCKET_HIDEOUT_B4F", "B4F", 24, 15, 3, "", intPtr(74)},

	{"SILPH_CO_ELEVATOR", "SILPH_CO_1F", "1F", 20, 0, 1, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_2F", "2F", 20, 0, 2, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_3F", "3F", 20, 0, 3, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_4F", "4F", 20, 0, 4, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_5F", "5F", 20, 0, 5, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_6F", "6F", 18, 0, 6, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_7F", "7F", 18, 0, 7, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_8F", "8F", 18, 0, 8, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_9F", "9F", 18, 0, 9, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_10F", "10F", 12, 0, 10, "", nil},
	{"SILPH_CO_ELEVATOR", "SILPH_CO_11F", "11F", 13, 0, 11, "", nil},
}

var cinnabarGymQuizDialogueTextSeeds = []dialogueTextSeed{
	{
		Label:      "_CinnabarGymQuizIntroText",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "POK\u00e9MON Quiz!\n\nGet it right and\nthe door opens to\nthe next room!\n\nGet it wrong and\nface a trainer!\n\nIf you want to\nconserve your\nPOK\u00e9MON for the\nGYM LEADER...\n\nThen get it right!\nHere we go!",
	},
	{
		Label:      "_CinnabarGymQuizCorrectText",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "You're absolutely\ncorrect!\n\nGo on through!",
	},
	{
		Label:      "_CinnabarGymQuizIncorrectText",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "Sorry! Bad call!",
	},
	{
		Label:      "_CinnabarQuizQuestionsText1",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "CATERPIE evolves\ninto BUTTERFREE?",
	},
	{
		Label:      "_CinnabarQuizQuestionsText2",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "There are 9\ncertified POK\u00e9MON\nLEAGUE BADGEs?",
	},
	{
		Label:      "_CinnabarQuizQuestionsText3",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "POLIWAG evolves 3\ntimes?",
	},
	{
		Label:      "_CinnabarQuizQuestionsText4",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "Are thunder moves\neffective against\nground element-\ntype POK\u00e9MON?",
	},
	{
		Label:      "_CinnabarQuizQuestionsText5",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "POK\u00e9MON of the\nsame kind and\nlevel are not\nidentical?",
	},
	{
		Label:      "_CinnabarQuizQuestionsText6",
		SourceFile: "data/text/text_2.asm",
		Dialogue:   "TM28 contains\nTOMBSTONER?",
	},
}

var cinnabarGymQuizTextPointerSeeds = []textPointerSeed{
	{"CinnabarGym", "TEXT_CINNABARGYM_QUIZ_1", "CinnabarGymQuizText1", "_CinnabarGymQuizIntroText", 20, false},
	{"CinnabarGym", "TEXT_CINNABARGYM_QUIZ_2", "CinnabarGymQuizText2", "_CinnabarGymQuizIntroText", 21, false},
	{"CinnabarGym", "TEXT_CINNABARGYM_QUIZ_3", "CinnabarGymQuizText3", "_CinnabarGymQuizIntroText", 22, false},
	{"CinnabarGym", "TEXT_CINNABARGYM_QUIZ_4", "CinnabarGymQuizText4", "_CinnabarGymQuizIntroText", 23, false},
	{"CinnabarGym", "TEXT_CINNABARGYM_QUIZ_5", "CinnabarGymQuizText5", "_CinnabarGymQuizIntroText", 24, false},
	{"CinnabarGym", "TEXT_CINNABARGYM_QUIZ_6", "CinnabarGymQuizText6", "_CinnabarGymQuizIntroText", 25, false},
}

var cinnabarGymQuizObjectSeeds = []phaserObjectSeed{
	{166, "sign", "SPRITE_SIGN", "CinnabarGym_SIGN_Quiz1", 15, 7, "LAND", "TEXT_CINNABARGYM_QUIZ_1"},
	{166, "sign", "SPRITE_SIGN", "CinnabarGym_SIGN_Quiz2", 10, 1, "LAND", "TEXT_CINNABARGYM_QUIZ_2"},
	{166, "sign", "SPRITE_SIGN", "CinnabarGym_SIGN_Quiz3", 9, 7, "LAND", "TEXT_CINNABARGYM_QUIZ_3"},
	{166, "sign", "SPRITE_SIGN", "CinnabarGym_SIGN_Quiz4", 9, 13, "LAND", "TEXT_CINNABARGYM_QUIZ_4"},
	{166, "sign", "SPRITE_SIGN", "CinnabarGym_SIGN_Quiz5", 1, 13, "LAND", "TEXT_CINNABARGYM_QUIZ_5"},
	{166, "sign", "SPRITE_SIGN", "CinnabarGym_SIGN_Quiz6", 1, 7, "LAND", "TEXT_CINNABARGYM_QUIZ_6"},
}

var cinnabarGymQuizBranchingDialogueSeeds = []branchingDialogueSeed{
	{
		MapName:            "CINNABAR_GYM",
		PromptTextConstant: "TEXT_CINNABARGYM_QUIZ_1",
		PromptText:         "CATERPIE evolves\ninto BUTTERFREE?",
		YesDialogue:        "You're absolutely\ncorrect!\n\nGo on through!",
		NoDialogue:         "Sorry! Bad call!",
		YesActions:         `[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE1_UNLOCKED"}]`,
		NoActions:          `[{"type":"startTrainerBattle","trainerClass":"BURGLAR","partyIndex":4,"trainerName":"BURGLAR","winFlag":"EVENT_BEAT_CINNABAR_GYM_TRAINER_1","postWinActions":[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE1_UNLOCKED"}]}]`,
	},
	{
		MapName:            "CINNABAR_GYM",
		PromptTextConstant: "TEXT_CINNABARGYM_QUIZ_2",
		PromptText:         "There are 9\ncertified POK\u00e9MON\nLEAGUE BADGEs?",
		YesDialogue:        "Sorry! Bad call!",
		NoDialogue:         "You're absolutely\ncorrect!\n\nGo on through!",
		YesActions:         `[{"type":"startTrainerBattle","trainerClass":"SUPER_NERD","partyIndex":10,"trainerName":"SUPER NERD","winFlag":"EVENT_BEAT_CINNABAR_GYM_TRAINER_2","postWinActions":[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE2_UNLOCKED"}]}]`,
		NoActions:          `[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE2_UNLOCKED"}]`,
	},
	{
		MapName:            "CINNABAR_GYM",
		PromptTextConstant: "TEXT_CINNABARGYM_QUIZ_3",
		PromptText:         "POLIWAG evolves 3\ntimes?",
		YesDialogue:        "Sorry! Bad call!",
		NoDialogue:         "You're absolutely\ncorrect!\n\nGo on through!",
		YesActions:         `[{"type":"startTrainerBattle","trainerClass":"BURGLAR","partyIndex":5,"trainerName":"BURGLAR","winFlag":"EVENT_BEAT_CINNABAR_GYM_TRAINER_3","postWinActions":[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE3_UNLOCKED"}]}]`,
		NoActions:          `[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE3_UNLOCKED"}]`,
	},
	{
		MapName:            "CINNABAR_GYM",
		PromptTextConstant: "TEXT_CINNABARGYM_QUIZ_4",
		PromptText:         "Are thunder moves\neffective against\nground element-\ntype POK\u00e9MON?",
		YesDialogue:        "Sorry! Bad call!",
		NoDialogue:         "You're absolutely\ncorrect!\n\nGo on through!",
		YesActions:         `[{"type":"startTrainerBattle","trainerClass":"SUPER_NERD","partyIndex":11,"trainerName":"SUPER NERD","winFlag":"EVENT_BEAT_CINNABAR_GYM_TRAINER_4","postWinActions":[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE4_UNLOCKED"}]}]`,
		NoActions:          `[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE4_UNLOCKED"}]`,
	},
	{
		MapName:            "CINNABAR_GYM",
		PromptTextConstant: "TEXT_CINNABARGYM_QUIZ_5",
		PromptText:         "POK\u00e9MON of the\nsame kind and\nlevel are not\nidentical?",
		YesDialogue:        "You're absolutely\ncorrect!\n\nGo on through!",
		NoDialogue:         "Sorry! Bad call!",
		YesActions:         `[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE5_UNLOCKED"}]`,
		NoActions:          `[{"type":"startTrainerBattle","trainerClass":"BURGLAR","partyIndex":6,"trainerName":"BURGLAR","winFlag":"EVENT_BEAT_CINNABAR_GYM_TRAINER_5","postWinActions":[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE5_UNLOCKED"}]}]`,
	},
	{
		MapName:            "CINNABAR_GYM",
		PromptTextConstant: "TEXT_CINNABARGYM_QUIZ_6",
		PromptText:         "TM28 contains\nTOMBSTONER?",
		YesDialogue:        "Sorry! Bad call!",
		NoDialogue:         "You're absolutely\ncorrect!\n\nGo on through!",
		YesActions:         `[{"type":"startTrainerBattle","trainerClass":"SUPER_NERD","partyIndex":12,"trainerName":"SUPER NERD","winFlag":"EVENT_BEAT_CINNABAR_GYM_TRAINER_6","postWinActions":[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE6_UNLOCKED"}]}]`,
		NoActions:          `[{"type":"setFlag","flag":"EVENT_CINNABAR_GYM_GATE6_UNLOCKED"}]`,
	},
}

var inGameTradeSeeds = []inGameTradeSeed{
	{
		TradeKey:             "TRADE_FOR_TERRY",
		TextConstant:         "TEXT_ROUTE11GATE2F_YOUNGSTER",
		MapName:              "ROUTE_11_GATE_2F",
		SourceFile:           "scripts/Route11Gate2F.asm",
		ScriptLabel:          "Route11Gate2FYoungsterText",
		RequestedPokemonName: "NIDORINO",
		OfferedPokemonName:   "NIDORINA",
		OfferedNickname:      "TERRY",
		DialogueSet:          "CASUAL",
		OriginalTradeIndex:   0,
	},
	{
		TradeKey:             "TRADE_FOR_MARCEL",
		TextConstant:         "TEXT_ROUTE2TRADEHOUSE_GAMEBOY_KID",
		MapName:              "ROUTE_2_TRADE_HOUSE",
		SourceFile:           "scripts/Route2TradeHouse.asm",
		ScriptLabel:          "Route2TradeHouseGameboyKidText",
		RequestedPokemonName: "ABRA",
		OfferedPokemonName:   "MR_MIME",
		OfferedNickname:      "MARCEL",
		DialogueSet:          "CASUAL",
		OriginalTradeIndex:   1,
	},
	{
		TradeKey:             "TRADE_FOR_SAILOR",
		TextConstant:         "TEXT_CINNABARLABFOSSILROOM_SCIENTIST2",
		MapName:              "CINNABAR_LAB_FOSSIL_ROOM",
		SourceFile:           "scripts/CinnabarLabFossilRoom.asm",
		ScriptLabel:          "CinnabarLabFossilRoomScientist2Text",
		RequestedPokemonName: "PONYTA",
		OfferedPokemonName:   "SEEL",
		OfferedNickname:      "SAILOR",
		DialogueSet:          "CASUAL",
		OriginalTradeIndex:   3,
	},
	{
		TradeKey:             "TRADE_FOR_DUX",
		TextConstant:         "TEXT_VERMILIONTRADEHOUSE_LITTLE_GIRL",
		MapName:              "VERMILION_TRADE_HOUSE",
		SourceFile:           "scripts/VermilionTradeHouse.asm",
		ScriptLabel:          "VermilionTradeHouseLittleGirlText",
		RequestedPokemonName: "SPEAROW",
		OfferedPokemonName:   "FARFETCHD",
		OfferedNickname:      "DUX",
		DialogueSet:          "HAPPY",
		OriginalTradeIndex:   4,
	},
	{
		TradeKey:             "TRADE_FOR_MARC",
		TextConstant:         "TEXT_ROUTE18GATE2F_YOUNGSTER",
		MapName:              "ROUTE_18_GATE_2F",
		SourceFile:           "scripts/Route18Gate2F.asm",
		ScriptLabel:          "Route18Gate2FYoungsterText",
		RequestedPokemonName: "SLOWBRO",
		OfferedPokemonName:   "LICKITUNG",
		OfferedNickname:      "MARC",
		DialogueSet:          "CASUAL",
		OriginalTradeIndex:   5,
	},
	{
		TradeKey:             "TRADE_FOR_LOLA",
		TextConstant:         "TEXT_CERULEANTRADEHOUSE_GAMBLER",
		MapName:              "CERULEAN_TRADE_HOUSE",
		SourceFile:           "scripts/CeruleanTradeHouse.asm",
		ScriptLabel:          "CeruleanTradeHouseGamblerText",
		RequestedPokemonName: "POLIWHIRL",
		OfferedPokemonName:   "JYNX",
		OfferedNickname:      "LOLA",
		DialogueSet:          "EVOLUTION",
		OriginalTradeIndex:   6,
	},
	{
		TradeKey:             "TRADE_FOR_DORIS",
		TextConstant:         "TEXT_CINNABARLABTRADEROOM_GRAMPS",
		MapName:              "CINNABAR_LAB_TRADE_ROOM",
		SourceFile:           "scripts/CinnabarLabTradeRoom.asm",
		ScriptLabel:          "CinnabarLabTradeRoomGrampsText",
		RequestedPokemonName: "RAICHU",
		OfferedPokemonName:   "ELECTRODE",
		OfferedNickname:      "DORIS",
		DialogueSet:          "EVOLUTION",
		OriginalTradeIndex:   7,
	},
	{
		TradeKey:             "TRADE_FOR_CRINKLES",
		TextConstant:         "TEXT_CINNABARLABTRADEROOM_BEAUTY",
		MapName:              "CINNABAR_LAB_TRADE_ROOM",
		SourceFile:           "scripts/CinnabarLabTradeRoom.asm",
		ScriptLabel:          "CinnabarLabTradeRoomBeautyText",
		RequestedPokemonName: "VENONAT",
		OfferedPokemonName:   "TANGELA",
		OfferedNickname:      "CRINKLES",
		DialogueSet:          "HAPPY",
		OriginalTradeIndex:   8,
	},
	{
		TradeKey:             "TRADE_FOR_SPOT",
		TextConstant:         "TEXT_UNDERGROUNDPATHROUTE5_LITTLE_GIRL",
		MapName:              "UNDERGROUND_PATH_ROUTE_5",
		SourceFile:           "scripts/UndergroundPathRoute5.asm",
		ScriptLabel:          "UndergroundPathRoute5LittleGirlText",
		RequestedPokemonName: "NIDORAN_M",
		OfferedPokemonName:   "NIDORAN_F",
		OfferedNickname:      "SPOT",
		DialogueSet:          "HAPPY",
		OriginalTradeIndex:   9,
	},
}

var boulderTargetSeeds = []boulderTargetSeed{
	{
		TargetFamily: "victory_road",
		MapName:      "VICTORY_ROAD_1F",
		SourceLabel:  "VictoryRoad1FDefaultScript",
		X:            17,
		Y:            13,
		Flag:         "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH",
		SourceFile:   "scripts/VictoryRoad1F.asm",
	},
	{
		TargetFamily: "victory_road",
		MapName:      "VICTORY_ROAD_2F",
		SourceLabel:  "VictoryRoad2FDefaultScript",
		X:            1,
		Y:            16,
		Flag:         "EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH1",
		SourceFile:   "scripts/VictoryRoad2F.asm",
	},
	{
		TargetFamily: "victory_road",
		MapName:      "VICTORY_ROAD_2F",
		SourceLabel:  "VictoryRoad2FDefaultScript",
		X:            9,
		Y:            16,
		Flag:         "EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH2",
		SourceFile:   "scripts/VictoryRoad2F.asm",
	},
	{
		TargetFamily: "victory_road",
		MapName:      "VICTORY_ROAD_3F",
		SourceLabel:  "VictoryRoad3FDefaultScript",
		X:            3,
		Y:            5,
		Flag:         "EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH1",
		SourceFile:   "scripts/VictoryRoad3F.asm",
	},
	{
		TargetFamily:              "victory_road",
		MapName:                   "VICTORY_ROAD_3F",
		SourceLabel:               "VictoryRoad3FDefaultScript",
		X:                         23,
		Y:                         15,
		Flag:                      "EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH2",
		DropsThroughHole:          true,
		SourceMissableObject:      "HS_VICTORY_ROAD_3F_BOULDER",
		DestinationMapName:        "VICTORY_ROAD_2F",
		DestinationMissableObject: "HS_VICTORY_ROAD_2F_BOULDER",
		SourceFile:                "scripts/VictoryRoad3F.asm",
	},
}
