package main

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestPokeStartCitySeedsIncludeKantoHomeTowns(t *testing.T) {
	want := []struct {
		name  string
		mapID int
	}{
		{"Pallet Town", 0},
		{"Viridian City", 1},
		{"Pewter City", 2},
		{"Cerulean City", 3},
		{"Vermilion City", 5},
		{"Lavender Town", 4},
		{"Celadon City", 6},
		{"Fuchsia City", 7},
		{"Saffron City", 10},
		{"Cinnabar Island", 8},
	}

	if len(pokeStartCitySeeds) != len(want) {
		t.Fatalf("pokeStartCitySeeds has %d entries, want %d", len(pokeStartCitySeeds), len(want))
	}

	seenMapIDs := map[int]string{}
	for i, city := range pokeStartCitySeeds {
		if city.ID != i+1 {
			t.Fatalf("city %q has id %d, want %d", city.Name, city.ID, i+1)
		}
		if city.SortOrder != i+1 {
			t.Fatalf("city %q has sort order %d, want %d", city.Name, city.SortOrder, i+1)
		}
		if city.Name != want[i].name {
			t.Fatalf("city %d name = %q, want %q", i, city.Name, want[i].name)
		}
		if city.MapID != want[i].mapID {
			t.Fatalf("city %q map id = %d, want %d", city.Name, city.MapID, want[i].mapID)
		}
		if city.Description == "" {
			t.Fatalf("city %q has empty description", city.Name)
		}
		if previous, ok := seenMapIDs[city.MapID]; ok {
			t.Fatalf("cities %q and %q share map id %d; selector dedupes by map id", previous, city.Name, city.MapID)
		}
		seenMapIDs[city.MapID] = city.Name
	}
}

func TestDisallowedWordSeedsAreUniqueAndCategorized(t *testing.T) {
	if len(disallowedWordSeeds) < 30 {
		t.Fatalf("disallowedWordSeeds has %d entries, want seeded profanity filter list", len(disallowedWordSeeds))
	}

	seen := map[string]bool{}
	categories := map[string]bool{}
	for _, seed := range disallowedWordSeeds {
		if seed.Word == "" {
			t.Fatal("disallowed word seed has empty word")
		}
		if seed.Category == "" {
			t.Fatalf("disallowed word %q has empty category", seed.Word)
		}
		if seen[seed.Word] {
			t.Fatalf("duplicate disallowed word seed %q", seed.Word)
		}
		seen[seed.Word] = true
		categories[seed.Category] = true
	}

	for _, category := range []string{"racial", "homophobic", "transphobic"} {
		if !categories[category] {
			t.Fatalf("disallowed word seeds missing %s category", category)
		}
	}
}

func TestSeedCQItemsMarksEvolutionStonesUsableAndPartyTargeted(t *testing.T) {
	raw := openCQItemSeedSQLite(t)
	defer raw.Close()

	if _, err := raw.Exec(`
		INSERT INTO phaser_items (
			id, name, short_name, price, is_usable, uses_party_menu,
			vending_price, move_id, is_guard_drink, is_key_item
		) VALUES
			(10, 'MOON STONE', 'MOON_STONE', 2100, 0, 0, NULL, NULL, 0, 0)`); err != nil {
		t.Fatalf("insert phaser item: %v", err)
	}

	tx, err := raw.Begin()
	if err != nil {
		t.Fatalf("begin seed tx: %v", err)
	}
	if err := seedCQItemsPostgres(tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("seed CQ items: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed tx: %v", err)
	}

	var itemType int
	var isUsable, usesPartyMenu, stackable bool
	if err := raw.QueryRow(`
		SELECT item_type, is_usable, uses_party_menu, stackable
		FROM cq_items
		WHERE short_name = 'MOON_STONE'`,
	).Scan(&itemType, &isUsable, &usesPartyMenu, &stackable); err != nil {
		t.Fatalf("query seeded Moon Stone: %v", err)
	}
	if itemType != 9 {
		t.Fatalf("Moon Stone item_type = %d, want 9", itemType)
	}
	if !isUsable {
		t.Fatal("Moon Stone is_usable = false, want true")
	}
	if !usesPartyMenu {
		t.Fatal("Moon Stone uses_party_menu = false, want true")
	}
	if !stackable {
		t.Fatal("Moon Stone stackable = false, want true")
	}
}

func TestElevatorFloorSeedsCoverRedBlueElevators(t *testing.T) {
	if len(elevatorFloorSeeds) != 19 {
		t.Fatalf("elevatorFloorSeeds has %d entries, want 19", len(elevatorFloorSeeds))
	}

	counts := map[string]int{}
	rocketRequiresLiftKey := 0
	seen := map[string]bool{}
	for _, seed := range elevatorFloorSeeds {
		if seed.ElevatorMapName == "" || seed.FloorMapName == "" || seed.FloorLabel == "" {
			t.Fatalf("incomplete elevator seed: %#v", seed)
		}
		key := seed.ElevatorMapName + "->" + seed.FloorMapName
		if seen[key] {
			t.Fatalf("duplicate elevator seed %s", key)
		}
		seen[key] = true
		counts[seed.ElevatorMapName]++

		if seed.ElevatorMapName == "ROCKET_HIDEOUT_ELEVATOR" {
			if seed.RequiresItemID == nil || *seed.RequiresItemID != 74 {
				t.Fatalf("Rocket elevator seed %#v should require Lift Key item 74", seed)
			}
			rocketRequiresLiftKey++
			continue
		}
		if seed.RequiresItemID != nil {
			t.Fatalf("%s should not require an item", key)
		}
	}

	wantCounts := map[string]int{
		"CELADON_MART_ELEVATOR":   5,
		"ROCKET_HIDEOUT_ELEVATOR": 3,
		"SILPH_CO_ELEVATOR":       11,
	}
	for mapName, want := range wantCounts {
		if counts[mapName] != want {
			t.Fatalf("%s has %d floors, want %d", mapName, counts[mapName], want)
		}
	}
	if rocketRequiresLiftKey != 3 {
		t.Fatalf("Rocket elevator Lift Key-gated floors = %d, want 3", rocketRequiresLiftKey)
	}
}

func TestSeedElevatorFloorsPostgres(t *testing.T) {
	raw := openElevatorSeedSQLite(t)
	defer raw.Close()

	tx, err := raw.Begin()
	if err != nil {
		t.Fatalf("begin seed tx: %v", err)
	}
	if err := seedElevatorFloorsPostgres(tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("seed elevator floors: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed tx: %v", err)
	}

	var count int
	if err := raw.QueryRow(`SELECT COUNT(*) FROM phaser_elevator_floors`).Scan(&count); err != nil {
		t.Fatalf("count seeded elevator floors: %v", err)
	}
	if count != len(elevatorFloorSeeds) {
		t.Fatalf("seeded elevator floors = %d, want %d", count, len(elevatorFloorSeeds))
	}

	var celadon5FMap, celadonDestX, celadonDestY int
	if err := raw.QueryRow(`
		SELECT floor_map_id, dest_x, dest_y
		FROM phaser_elevator_floors
		WHERE elevator_map_id = 127 AND floor_label = '5F'`,
	).Scan(&celadon5FMap, &celadonDestX, &celadonDestY); err != nil {
		t.Fatalf("query Celadon 5F elevator seed: %v", err)
	}
	if celadon5FMap != 136 || celadonDestX != 1 || celadonDestY != 1 {
		t.Fatalf("Celadon 5F seed = map %d (%d,%d), want map 136 (1,1)", celadon5FMap, celadonDestX, celadonDestY)
	}

	var rocketRows int
	if err := raw.QueryRow(`
		SELECT COUNT(*)
		FROM phaser_elevator_floors
		WHERE elevator_map_id = 203 AND requires_item_id = 74`,
	).Scan(&rocketRows); err != nil {
		t.Fatalf("query Rocket elevator Lift Key rows: %v", err)
	}
	if rocketRows != 3 {
		t.Fatalf("Rocket Lift Key-gated rows = %d, want 3", rocketRows)
	}
}

func TestInGameTradeSeedsMatchPokeRedActiveTrades(t *testing.T) {
	want := []inGameTradeSeed{
		{"TRADE_FOR_TERRY", "TEXT_ROUTE11GATE2F_YOUNGSTER", "ROUTE_11_GATE_2F", "scripts/Route11Gate2F.asm", "Route11Gate2FYoungsterText", "NIDORINO", "NIDORINA", "TERRY", "CASUAL", 0},
		{"TRADE_FOR_MARCEL", "TEXT_ROUTE2TRADEHOUSE_GAMEBOY_KID", "ROUTE_2_TRADE_HOUSE", "scripts/Route2TradeHouse.asm", "Route2TradeHouseGameboyKidText", "ABRA", "MR_MIME", "MARCEL", "CASUAL", 1},
		{"TRADE_FOR_SAILOR", "TEXT_CINNABARLABFOSSILROOM_SCIENTIST2", "CINNABAR_LAB_FOSSIL_ROOM", "scripts/CinnabarLabFossilRoom.asm", "CinnabarLabFossilRoomScientist2Text", "PONYTA", "SEEL", "SAILOR", "CASUAL", 3},
		{"TRADE_FOR_DUX", "TEXT_VERMILIONTRADEHOUSE_LITTLE_GIRL", "VERMILION_TRADE_HOUSE", "scripts/VermilionTradeHouse.asm", "VermilionTradeHouseLittleGirlText", "SPEAROW", "FARFETCHD", "DUX", "HAPPY", 4},
		{"TRADE_FOR_MARC", "TEXT_ROUTE18GATE2F_YOUNGSTER", "ROUTE_18_GATE_2F", "scripts/Route18Gate2F.asm", "Route18Gate2FYoungsterText", "SLOWBRO", "LICKITUNG", "MARC", "CASUAL", 5},
		{"TRADE_FOR_LOLA", "TEXT_CERULEANTRADEHOUSE_GAMBLER", "CERULEAN_TRADE_HOUSE", "scripts/CeruleanTradeHouse.asm", "CeruleanTradeHouseGamblerText", "POLIWHIRL", "JYNX", "LOLA", "EVOLUTION", 6},
		{"TRADE_FOR_DORIS", "TEXT_CINNABARLABTRADEROOM_GRAMPS", "CINNABAR_LAB_TRADE_ROOM", "scripts/CinnabarLabTradeRoom.asm", "CinnabarLabTradeRoomGrampsText", "RAICHU", "ELECTRODE", "DORIS", "EVOLUTION", 7},
		{"TRADE_FOR_CRINKLES", "TEXT_CINNABARLABTRADEROOM_BEAUTY", "CINNABAR_LAB_TRADE_ROOM", "scripts/CinnabarLabTradeRoom.asm", "CinnabarLabTradeRoomBeautyText", "VENONAT", "TANGELA", "CRINKLES", "HAPPY", 8},
		{"TRADE_FOR_SPOT", "TEXT_UNDERGROUNDPATHROUTE5_LITTLE_GIRL", "UNDERGROUND_PATH_ROUTE_5", "scripts/UndergroundPathRoute5.asm", "UndergroundPathRoute5LittleGirlText", "NIDORAN_M", "NIDORAN_F", "SPOT", "HAPPY", 9},
	}

	if len(inGameTradeSeeds) != len(want) {
		t.Fatalf("inGameTradeSeeds has %d entries, want %d", len(inGameTradeSeeds), len(want))
	}
	seenTextConstants := map[string]string{}
	for i, got := range inGameTradeSeeds {
		if got != want[i] {
			t.Fatalf("trade seed %d = %#v, want %#v", i, got, want[i])
		}
		if got.TradeKey == "TRADE_FOR_CHIKUCHIKU" {
			t.Fatal("unused pokered TRADE_FOR_CHIKUCHIKU should not be seeded as an active NPC trade")
		}
		if previous, ok := seenTextConstants[got.TextConstant]; ok {
			t.Fatalf("trades %s and %s share text constant %s", previous, got.TradeKey, got.TextConstant)
		}
		seenTextConstants[got.TextConstant] = got.TradeKey
	}
}

func TestLoadInGameTradeSeedsFromSQLite(t *testing.T) {
	sqlite := openTradeSeedSQLite(t)
	defer sqlite.Close()

	trades, found, err := loadInGameTradeSeedsFromSQLite(sqlite)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("loadInGameTradeSeedsFromSQLite found=false, want true")
	}
	if len(trades) != 2 {
		t.Fatalf("loaded %d active trades, want 2", len(trades))
	}

	want := []inGameTradeSeed{
		{"TRADE_FOR_MARCEL", "TEXT_ROUTE2TRADEHOUSE_GAMEBOY_KID", "ROUTE_2_TRADE_HOUSE", "scripts/Route2TradeHouse.asm", "Route2TradeHouseGameboyKidText", "ABRA", "MR_MIME", "MARCEL", "CASUAL", 1},
		{"TRADE_FOR_DORIS", "TEXT_CINNABARLABTRADEROOM_GRAMPS", "CINNABAR_LAB_TRADE_ROOM", "scripts/CinnabarLabTradeRoom.asm", "CinnabarLabTradeRoomGrampsText", "RAICHU", "ELECTRODE", "DORIS", "EVOLUTION", 7},
	}
	for i, got := range trades {
		if got != want[i] {
			t.Fatalf("trade %d = %#v, want %#v", i, got, want[i])
		}
	}
}

func TestLoadInGameTradeSeedsFromSQLiteMissingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pokemon.db")
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	if _, err := sqlite.Exec(`CREATE TABLE placeholder (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	trades, found, err := loadInGameTradeSeedsFromSQLite(sqlite)
	if err != nil {
		t.Fatal(err)
	}
	if found || len(trades) != 0 {
		t.Fatalf("found=%t trades=%#v, want missing table", found, trades)
	}
}

func TestBoulderTargetSeedsMatchPokeRedVictoryRoadTargets(t *testing.T) {
	want := []boulderTargetSeed{
		{TargetFamily: "victory_road", MapName: "VICTORY_ROAD_1F", SourceLabel: "VictoryRoad1FDefaultScript", X: 17, Y: 13, Flag: "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH", SourceFile: "scripts/VictoryRoad1F.asm"},
		{TargetFamily: "victory_road", MapName: "VICTORY_ROAD_2F", SourceLabel: "VictoryRoad2FDefaultScript", X: 1, Y: 16, Flag: "EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH1", SourceFile: "scripts/VictoryRoad2F.asm"},
		{TargetFamily: "victory_road", MapName: "VICTORY_ROAD_2F", SourceLabel: "VictoryRoad2FDefaultScript", X: 9, Y: 16, Flag: "EVENT_VICTORY_ROAD_2_BOULDER_ON_SWITCH2", SourceFile: "scripts/VictoryRoad2F.asm"},
		{TargetFamily: "victory_road", MapName: "VICTORY_ROAD_3F", SourceLabel: "VictoryRoad3FDefaultScript", X: 3, Y: 5, Flag: "EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH1", SourceFile: "scripts/VictoryRoad3F.asm"},
		{TargetFamily: "victory_road", MapName: "VICTORY_ROAD_3F", SourceLabel: "VictoryRoad3FDefaultScript", X: 23, Y: 15, Flag: "EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH2", DropsThroughHole: true, SourceMissableObject: "HS_VICTORY_ROAD_3F_BOULDER", DestinationMapName: "VICTORY_ROAD_2F", DestinationMissableObject: "HS_VICTORY_ROAD_2F_BOULDER", SourceFile: "scripts/VictoryRoad3F.asm"},
	}

	if len(boulderTargetSeeds) != len(want) {
		t.Fatalf("boulderTargetSeeds has %d entries, want %d", len(boulderTargetSeeds), len(want))
	}
	for i, got := range boulderTargetSeeds {
		if got != want[i] {
			t.Fatalf("boulder target seed %d = %#v, want %#v", i, got, want[i])
		}
	}
}

func TestLoadBoulderTargetSeedsFromSQLite(t *testing.T) {
	sqlite := openBoulderTargetSeedSQLite(t)
	defer sqlite.Close()

	targets, found, err := loadBoulderTargetSeedsFromSQLite(sqlite)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("loadBoulderTargetSeedsFromSQLite found=false, want true")
	}
	if len(targets) != 2 {
		t.Fatalf("loaded %d boulder targets, want 2", len(targets))
	}

	want := []boulderTargetSeed{
		{TargetFamily: "victory_road", MapName: "VICTORY_ROAD_1F", SourceLabel: "VictoryRoad1FDefaultScript", X: 17, Y: 13, Flag: "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH", SourceFile: "scripts/VictoryRoad1F.asm"},
		{TargetFamily: "victory_road", MapName: "VICTORY_ROAD_3F", SourceLabel: "VictoryRoad3FDefaultScript", X: 23, Y: 15, Flag: "EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH2", DropsThroughHole: true, SourceMissableObject: "HS_VICTORY_ROAD_3F_BOULDER", DestinationMapName: "VICTORY_ROAD_2F", DestinationMissableObject: "HS_VICTORY_ROAD_2F_BOULDER", SourceFile: "scripts/VictoryRoad3F.asm"},
	}
	for i, got := range targets {
		if got != want[i] {
			t.Fatalf("target %d = %#v, want %#v", i, got, want[i])
		}
	}
}

func TestLoadBoulderTargetSeedsFromSQLiteMissingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pokemon.db")
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlite.Close()
	if _, err := sqlite.Exec(`CREATE TABLE placeholder (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	targets, found, err := loadBoulderTargetSeedsFromSQLite(sqlite)
	if err != nil {
		t.Fatal(err)
	}
	if found || len(targets) != 0 {
		t.Fatalf("found=%t targets=%#v, want missing table", found, targets)
	}
}

func TestCinnabarGymQuizSeedsCoverAllSixQuestions(t *testing.T) {
	if len(cinnabarGymQuizBranchingDialogueSeeds) != 6 {
		t.Fatalf("branching dialogue seeds = %d, want 6", len(cinnabarGymQuizBranchingDialogueSeeds))
	}
	if len(cinnabarGymQuizObjectSeeds) != 6 {
		t.Fatalf("object seeds = %d, want 6", len(cinnabarGymQuizObjectSeeds))
	}
	if len(cinnabarGymQuizTextPointerSeeds) != 6 {
		t.Fatalf("text pointer seeds = %d, want 6", len(cinnabarGymQuizTextPointerSeeds))
	}

	wantCoords := map[string][2]int{
		"TEXT_CINNABARGYM_QUIZ_1": {15, 7},
		"TEXT_CINNABARGYM_QUIZ_2": {10, 1},
		"TEXT_CINNABARGYM_QUIZ_3": {9, 7},
		"TEXT_CINNABARGYM_QUIZ_4": {9, 13},
		"TEXT_CINNABARGYM_QUIZ_5": {1, 13},
		"TEXT_CINNABARGYM_QUIZ_6": {1, 7},
	}
	seenObjects := map[string]bool{}
	for _, seed := range cinnabarGymQuizObjectSeeds {
		if seed.MapID != 166 {
			t.Fatalf("%s map id = %d, want Cinnabar Gym map id 166", seed.Text, seed.MapID)
		}
		if seed.ObjectType != "sign" || seed.SpriteName != "SPRITE_SIGN" {
			t.Fatalf("%s object type/sprite = %s/%s, want sign/SPRITE_SIGN", seed.Text, seed.ObjectType, seed.SpriteName)
		}
		want, ok := wantCoords[seed.Text]
		if !ok {
			t.Fatalf("unexpected quiz object text constant %s", seed.Text)
		}
		if seed.LocalX != want[0] || seed.LocalY != want[1] {
			t.Fatalf("%s coords = (%d,%d), want (%d,%d)", seed.Text, seed.LocalX, seed.LocalY, want[0], want[1])
		}
		seenObjects[seed.Text] = true
	}

	for _, seed := range cinnabarGymQuizBranchingDialogueSeeds {
		if seed.MapName != "CINNABAR_GYM" {
			t.Fatalf("%s map name = %s, want CINNABAR_GYM", seed.PromptTextConstant, seed.MapName)
		}
		if !seenObjects[seed.PromptTextConstant] {
			t.Fatalf("branching dialogue %s has no matching sign object", seed.PromptTextConstant)
		}
		assertValidActionJSON(t, seed.PromptTextConstant, "yes", seed.YesActions)
		assertValidActionJSON(t, seed.PromptTextConstant, "no", seed.NoActions)
	}
}

func TestCinnabarGymQuizBranchingActionsMatchSourceAnswers(t *testing.T) {
	want := map[string]struct {
		yesType        string
		noType         string
		battleClass    string
		battleParty    float64
		unlockedGate   string
		trainerWinFlag string
	}{
		"TEXT_CINNABARGYM_QUIZ_1": {"setFlag", "startTrainerBattle", "BURGLAR", 4, "EVENT_CINNABAR_GYM_GATE1_UNLOCKED", "EVENT_BEAT_CINNABAR_GYM_TRAINER_1"},
		"TEXT_CINNABARGYM_QUIZ_2": {"startTrainerBattle", "setFlag", "SUPER_NERD", 10, "EVENT_CINNABAR_GYM_GATE2_UNLOCKED", "EVENT_BEAT_CINNABAR_GYM_TRAINER_2"},
		"TEXT_CINNABARGYM_QUIZ_3": {"startTrainerBattle", "setFlag", "BURGLAR", 5, "EVENT_CINNABAR_GYM_GATE3_UNLOCKED", "EVENT_BEAT_CINNABAR_GYM_TRAINER_3"},
		"TEXT_CINNABARGYM_QUIZ_4": {"startTrainerBattle", "setFlag", "SUPER_NERD", 11, "EVENT_CINNABAR_GYM_GATE4_UNLOCKED", "EVENT_BEAT_CINNABAR_GYM_TRAINER_4"},
		"TEXT_CINNABARGYM_QUIZ_5": {"setFlag", "startTrainerBattle", "BURGLAR", 6, "EVENT_CINNABAR_GYM_GATE5_UNLOCKED", "EVENT_BEAT_CINNABAR_GYM_TRAINER_5"},
		"TEXT_CINNABARGYM_QUIZ_6": {"startTrainerBattle", "setFlag", "SUPER_NERD", 12, "EVENT_CINNABAR_GYM_GATE6_UNLOCKED", "EVENT_BEAT_CINNABAR_GYM_TRAINER_6"},
	}

	for _, seed := range cinnabarGymQuizBranchingDialogueSeeds {
		expect, ok := want[seed.PromptTextConstant]
		if !ok {
			t.Fatalf("unexpected branching seed %s", seed.PromptTextConstant)
		}
		yesActions := decodeActions(t, seed.YesActions)
		noActions := decodeActions(t, seed.NoActions)
		if yesActions[0]["type"] != expect.yesType {
			t.Fatalf("%s yes action type = %v, want %s", seed.PromptTextConstant, yesActions[0]["type"], expect.yesType)
		}
		if noActions[0]["type"] != expect.noType {
			t.Fatalf("%s no action type = %v, want %s", seed.PromptTextConstant, noActions[0]["type"], expect.noType)
		}

		var battle map[string]any
		if expect.yesType == "startTrainerBattle" {
			battle = yesActions[0]
		} else {
			battle = noActions[0]
		}
		if battle["trainerClass"] != expect.battleClass {
			t.Fatalf("%s trainer class = %v, want %s", seed.PromptTextConstant, battle["trainerClass"], expect.battleClass)
		}
		if battle["partyIndex"] != expect.battleParty {
			t.Fatalf("%s party index = %v, want %.0f", seed.PromptTextConstant, battle["partyIndex"], expect.battleParty)
		}
		if battle["winFlag"] != expect.trainerWinFlag {
			t.Fatalf("%s win flag = %v, want %s", seed.PromptTextConstant, battle["winFlag"], expect.trainerWinFlag)
		}
		postWin, ok := battle["postWinActions"].([]any)
		if !ok || len(postWin) != 1 {
			t.Fatalf("%s postWinActions = %#v, want one setFlag", seed.PromptTextConstant, battle["postWinActions"])
		}
		postWinSetFlag, ok := postWin[0].(map[string]any)
		if !ok || postWinSetFlag["flag"] != expect.unlockedGate {
			t.Fatalf("%s postWin flag = %#v, want %s", seed.PromptTextConstant, postWin[0], expect.unlockedGate)
		}
	}
}

func assertValidActionJSON(t *testing.T, textConstant, branch, raw string) {
	t.Helper()
	actions := decodeActions(t, raw)
	if len(actions) == 0 {
		t.Fatalf("%s %s actions empty", textConstant, branch)
	}
}

func decodeActions(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var actions []map[string]any
	if err := json.Unmarshal([]byte(raw), &actions); err != nil {
		t.Fatalf("invalid action JSON %q: %v", raw, err)
	}
	return actions
}

func openCQItemSeedSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.Exec(`
		CREATE TABLE phaser_items (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			short_name TEXT NOT NULL,
			price INTEGER DEFAULT NULL,
			is_usable INTEGER DEFAULT 0,
			uses_party_menu INTEGER DEFAULT 0,
			vending_price INTEGER DEFAULT NULL,
			move_id INTEGER DEFAULT NULL,
			is_guard_drink INTEGER DEFAULT 0,
			is_key_item INTEGER DEFAULT 0
		);
		CREATE TABLE cq_items (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			short_name TEXT NOT NULL,
			price INTEGER DEFAULT 0,
			vending_price INTEGER DEFAULT NULL,
			item_type INTEGER DEFAULT 0,
			is_usable BOOLEAN DEFAULT FALSE,
			uses_party_menu BOOLEAN DEFAULT FALSE,
			is_key_item BOOLEAN DEFAULT FALSE,
			is_guard_drink BOOLEAN DEFAULT FALSE,
			move_id INTEGER DEFAULT NULL,
			stackable BOOLEAN DEFAULT TRUE,
			stack_size INTEGER DEFAULT 99,
			heal_amount INTEGER DEFAULT 0,
			status_cure TEXT DEFAULT NULL,
			pp_restore INTEGER DEFAULT 0,
			revive_percent INTEGER DEFAULT 0,
			ball_modifier REAL DEFAULT 0,
			bonus_attack INTEGER DEFAULT 0,
			bonus_defense INTEGER DEFAULT 0,
			bonus_speed INTEGER DEFAULT 0,
			bonus_special INTEGER DEFAULT 0,
			bonus_accuracy INTEGER DEFAULT 0,
			bonus_encounter_rate INTEGER DEFAULT 0,
			bonus_crit INTEGER DEFAULT 0,
			bonus_flee INTEGER DEFAULT 0,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP
		)`); err != nil {
		t.Fatal(err)
	}
	return sqlite
}

func openElevatorSeedSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.Exec(`
		CREATE TABLE phaser_maps (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		);
		CREATE TABLE phaser_elevator_floors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			elevator_map_id INTEGER NOT NULL,
			floor_map_id INTEGER NOT NULL,
			floor_label TEXT NOT NULL,
			dest_x INTEGER NOT NULL,
			dest_y INTEGER NOT NULL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			requires_flag TEXT DEFAULT NULL,
			requires_item_id INTEGER DEFAULT NULL,
			UNIQUE (elevator_map_id, floor_map_id)
		)`); err != nil {
		t.Fatal(err)
	}

	mapIDs := map[string]int{
		"CELADON_MART_1F":         122,
		"CELADON_MART_2F":         123,
		"CELADON_MART_3F":         124,
		"CELADON_MART_4F":         125,
		"CELADON_MART_ELEVATOR":   127,
		"CELADON_MART_5F":         136,
		"SILPH_CO_1F":             181,
		"ROCKET_HIDEOUT_B1F":      199,
		"ROCKET_HIDEOUT_B2F":      200,
		"ROCKET_HIDEOUT_B4F":      202,
		"ROCKET_HIDEOUT_ELEVATOR": 203,
		"SILPH_CO_2F":             207,
		"SILPH_CO_3F":             208,
		"SILPH_CO_4F":             209,
		"SILPH_CO_5F":             210,
		"SILPH_CO_6F":             211,
		"SILPH_CO_7F":             212,
		"SILPH_CO_8F":             213,
		"SILPH_CO_9F":             233,
		"SILPH_CO_10F":            234,
		"SILPH_CO_11F":            235,
		"SILPH_CO_ELEVATOR":       236,
	}
	for name, id := range mapIDs {
		if _, err := sqlite.Exec(`INSERT INTO phaser_maps (id, name) VALUES (?, ?)`, id, name); err != nil {
			t.Fatal(err)
		}
	}
	return sqlite
}

func openTradeSeedSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.Exec(`
		CREATE TABLE script_event_in_game_trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			trade_key TEXT NOT NULL UNIQUE,
			map_name TEXT,
			script_label TEXT,
			text_constant TEXT,
			requested_pokemon TEXT NOT NULL,
			offered_pokemon TEXT NOT NULL,
			offered_nickname TEXT NOT NULL,
			dialogue_set TEXT NOT NULL,
			original_trade_index INTEGER NOT NULL,
			active INTEGER NOT NULL,
			source_file TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		key, mapName, label, text, requested, offered, nickname, set, source string
		index, active                                                        int
	}{
		{"TRADE_FOR_CHIKUCHIKU", "", "", "", "BUTTERFREE", "BEEDRILL", "CHIKUCHIKU", "HAPPY", "pokemon-game-data/data/events/trades.asm", 2, 0},
		{"TRADE_FOR_MARCEL", "Route2TradeHouse", "Route2TradeHouseGameboyKidText", "TEXT_ROUTE2TRADEHOUSE_GAMEBOY_KID", "ABRA", "MR_MIME", "MARCEL", "CASUAL", "pokemon-game-data/scripts/Route2TradeHouse.asm", 1, 1},
		{"TRADE_FOR_DORIS", "CinnabarLabTradeRoom", "CinnabarLabTradeRoomGrampsText", "TEXT_CINNABARLABTRADEROOM_GRAMPS", "RAICHU", "ELECTRODE", "DORIS", "EVOLUTION", "pokemon-game-data/scripts/CinnabarLabTradeRoom.asm", 7, 1},
	}
	for _, row := range rows {
		if _, err := sqlite.Exec(`
			INSERT INTO script_event_in_game_trades (
				trade_key, map_name, script_label, text_constant,
				requested_pokemon, offered_pokemon, offered_nickname,
				dialogue_set, original_trade_index, active, source_file
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row.key, row.mapName, row.label, row.text, row.requested, row.offered,
			row.nickname, row.set, row.index, row.active, row.source,
		); err != nil {
			t.Fatal(err)
		}
	}
	return sqlite
}

func openBoulderTargetSeedSQLite(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pokemon.db")
	sqlite, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.Exec(`
		CREATE TABLE script_event_boulder_targets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			target_family TEXT NOT NULL,
			map_name TEXT NOT NULL,
			source_label TEXT NOT NULL,
			x INTEGER NOT NULL,
			y INTEGER NOT NULL,
			flag TEXT NOT NULL,
			drops_through_hole INTEGER NOT NULL,
			source_missable_object TEXT NOT NULL,
			destination_map_name TEXT NOT NULL,
			destination_missable_object TEXT NOT NULL,
			source_file TEXT NOT NULL,
			target_json TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	rows := []struct {
		targetFamily, mapName, sourceLabel, flag, sourceMissable, destinationMap, destinationMissable, sourceFile string
		x, y, dropsThroughHole                                                                                    int
	}{
		{"victory_road", "VictoryRoad1F", "VictoryRoad1FDefaultScript", "EVENT_VICTORY_ROAD_1_BOULDER_ON_SWITCH", "", "", "", "pokemon-game-data/scripts/VictoryRoad1F.asm", 17, 13, 0},
		{"victory_road", "VictoryRoad3F", "VictoryRoad3FDefaultScript", "EVENT_VICTORY_ROAD_3_BOULDER_ON_SWITCH2", "HS_VICTORY_ROAD_3F_BOULDER", "VictoryRoad2F", "HS_VICTORY_ROAD_2F_BOULDER", "pokemon-game-data/scripts/VictoryRoad3F.asm", 23, 15, 1},
	}
	for _, row := range rows {
		if _, err := sqlite.Exec(`
			INSERT INTO script_event_boulder_targets (
				target_family, map_name, source_label, x, y, flag,
				drops_through_hole, source_missable_object,
				destination_map_name, destination_missable_object, source_file, target_json
			)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			row.targetFamily, row.mapName, row.sourceLabel, row.x, row.y, row.flag,
			row.dropsThroughHole, row.sourceMissable, row.destinationMap,
			row.destinationMissable, row.sourceFile, "{}",
		); err != nil {
			t.Fatal(err)
		}
	}
	return sqlite
}
