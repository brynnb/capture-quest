package scriptcandidateimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

func TestMapCandidateSupportsMultiFlagConditions(t *testing.T) {
	candidate := safariCandidate("ViridianCityGamblerGymClosed", "", "")
	candidate.Conditions = candidateCondition{
		RequiresEventsAbsent: []string{"EVENT_BEAT_VIRIDIAN_GYM_GIOVANNI"},
		RequiresBadgesAbsent: []string{"EARTHBADGE"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresFlag != "" || event.RequiresFlagAbsent != "" {
		t.Fatalf("scalar flags = %q/%q, want array-only absent conditions", event.RequiresFlag, event.RequiresFlagAbsent)
	}
	wantAbsent := []string{"EVENT_BEAT_VIRIDIAN_GYM_GIOVANNI", "EVENT_GOT_EARTHBADGE"}
	if len(event.RequiresFlagsAbsent) != len(wantAbsent) {
		t.Fatalf("requiresFlagsAbsent = %#v, want %#v", event.RequiresFlagsAbsent, wantAbsent)
	}
	for i := range wantAbsent {
		if event.RequiresFlagsAbsent[i] != wantAbsent[i] {
			t.Fatalf("requiresFlagsAbsent = %#v, want %#v", event.RequiresFlagsAbsent, wantAbsent)
		}
	}
}

func TestMapCandidateMapsWarpActionToEventWarp(t *testing.T) {
	candidate := safariCandidate("PokemonTower7FMrFujiRescue", "EVENT_RESCUED_MR_FUJI", "")
	candidate.MapName = "PokemonTower7F"
	candidate.Trigger = candidateTrigger{
		Type:  "npc_click",
		Label: "TEXT_POKEMONTOWER7F_MR_FUJI",
	}
	candidate.Actions = []candidateAction{
		{Type: "lockInput"},
		{Type: "dialogue", Speaker: "MR. FUJI", Lines: []string{"Follow me to my home."}},
		{Type: "setEvent", Event: "EVENT_RESCUED_MR_FUJI"},
		{Type: "warp", MapID: 149, X: 3, Y: 7, Direction: "UP"},
		{Type: "unlockInput"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if event.Warp == nil {
		t.Fatal("event.Warp is nil")
	}
	if *event.Warp != (scriptedevents.EventWarp{MapID: 149, X: 3, Y: 7}) {
		t.Fatalf("warp = %+v", *event.Warp)
	}
	if len(event.Actions) != 4 {
		t.Fatalf("actions len = %d, want 4", len(event.Actions))
	}
	for _, raw := range event.Actions {
		var action map[string]any
		if err := json.Unmarshal(raw, &action); err != nil {
			t.Fatal(err)
		}
		if action["type"] == "warp" {
			t.Fatalf("warp action should not remain in actions: %s", string(raw))
		}
	}
}

func TestMapActionsPreservesAudioActions(t *testing.T) {
	actions, err := mapActions([]candidateAction{
		{Type: "playSFX", SFXConstant: "SFX_GET_ITEM_1", Volume: 0.6},
		{Type: "playCry", PokemonName: "PIKACHU", SFXConstant: "SFX_CRY_0F"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 {
		t.Fatalf("actions len = %d, want 2", len(actions))
	}

	var sfx map[string]any
	if err := json.Unmarshal(actions[0], &sfx); err != nil {
		t.Fatal(err)
	}
	if got, want := sfx["type"], "playSFX"; got != want {
		t.Fatalf("sfx type = %v, want %q", got, want)
	}
	if got, want := sfx["sfxConstant"], "SFX_GET_ITEM_1"; got != want {
		t.Fatalf("sfx constant = %v, want %q", got, want)
	}
	if got, want := sfx["volume"], 0.6; got != want {
		t.Fatalf("sfx volume = %v, want %v", got, want)
	}

	var cry map[string]any
	if err := json.Unmarshal(actions[1], &cry); err != nil {
		t.Fatal(err)
	}
	if got, want := cry["type"], "playCry"; got != want {
		t.Fatalf("cry type = %v, want %q", got, want)
	}
	if got, want := cry["pokemonName"], "PIKACHU"; got != want {
		t.Fatalf("cry pokemonName = %v, want %q", got, want)
	}
	if got, want := cry["sfxConstant"], "SFX_CRY_0F"; got != want {
		t.Fatalf("cry fallback = %v, want %q", got, want)
	}
}

func TestMapActionsPreservesGameCornerPrizeVendorWindow(t *testing.T) {
	actions, err := mapActions([]candidateAction{
		{
			Type:         "gameCornerPrizeVendor",
			TextConstant: "TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_2",
			PrizeWindow:  2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("actions len = %d, want 1", len(actions))
	}

	var action map[string]any
	if err := json.Unmarshal(actions[0], &action); err != nil {
		t.Fatal(err)
	}
	if got, want := action["type"], "gameCornerPrizeVendor"; got != want {
		t.Fatalf("type = %v, want %q", got, want)
	}
	if got, want := action["textConstant"], "TEXT_GAMECORNERPRIZEROOM_PRIZE_VENDOR_2"; got != want {
		t.Fatalf("textConstant = %v, want %q", got, want)
	}
	if got, want := action["prizeWindow"], float64(2); got != want {
		t.Fatalf("prizeWindow = %v, want %v", got, want)
	}
}

func TestCanShareMapSetFlagWithExistingExtractorBattle(t *testing.T) {
	event := scriptedevents.EventFile{
		ScriptLabel: "RocketHideoutB4FGiovanniEncounter",
		MapName:     "ROCKET_HIDEOUT_B4F",
		Trigger: scriptedevents.EventTrigger{
			Type:   "npc_click",
			Source: extractorSource,
			Label:  "TEXT_ROCKETHIDEOUTB4F_GIOVANNI",
		},
		Actions: []json.RawMessage{
			rawAction(map[string]any{
				"type":         "startTrainerBattle",
				"trainerClass": "GIOVANNI",
				"partyIndex":   1,
				"winFlag":      "EVENT_BEAT_ROCKET_HIDEOUT_GIOVANNI",
				"postWinActions": []json.RawMessage{
					rawAction(map[string]any{"type": "setFlag", "flag": "EVENT_ROCKET_HIDEOUT_GIOVANNI_LEFT"}),
				},
			}),
		},
	}

	if !canShareMapSetFlagWithExistingExtractorBattle(event, existingScript{ScriptLabel: "RocketHideoutB4FGiovanniDefeated"}) {
		t.Fatal("extractor battle should share a post-win map flag with a legacy non-CaptureQuest owner")
	}
	if canShareMapSetFlagWithExistingExtractorBattle(event, existingScript{ScriptLabel: "ManualOverride", Source: capturequestSource}) {
		t.Fatal("CaptureQuest-owned scripts must still block generated map flag sharing")
	}
}

func TestMapCandidateSupportsItemNameConditions(t *testing.T) {
	event, err := mapCandidate(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "VermilionOldRodHouse",
		ScriptLabel: "VermilionOldRodHouseFishingGuruGift",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_VERMILIONOLDRODHOUSE_FISHING_GURU",
		},
		Conditions: candidateCondition{
			RequiresItem:       "OLD_ROD",
			RequiresItemAbsent: "GOOD_ROD",
		},
		Actions: []candidateAction{{Type: "dialogue", Lines: []string{"How are the fish biting?"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresItemName != "OLD_ROD" {
		t.Fatalf("requiresItemName = %q, want OLD_ROD", event.RequiresItemName)
	}
	if event.RequiresItemAbsentName != "GOOD_ROD" {
		t.Fatalf("requiresItemAbsentName = %q, want GOOD_ROD", event.RequiresItemAbsentName)
	}
}

func TestMapCandidateNormalizesOverworldCoordinates(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE maps (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			tileset_id INTEGER NOT NULL,
			is_overworld INTEGER NOT NULL
		);
		CREATE TABLE overworld_map_positions (
			id INTEGER PRIMARY KEY,
			map_name TEXT NOT NULL,
			x_offset INTEGER NOT NULL,
			y_offset INTEGER NOT NULL
		);
		INSERT INTO maps (id, name, tileset_id, is_overworld) VALUES (5, 'VERMILION_CITY', 0, 1);
		INSERT INTO overworld_map_positions (id, map_name, x_offset, y_offset) VALUES (5, 'VERMILION_CITY', 170, -54);
	`); err != nil {
		t.Fatal(err)
	}

	resolver, err := newCoordinateResolver(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	event, err := mapCandidateWithResolver(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "VermilionCity",
		ScriptLabel: "VermilionCitySSAnneGuardNoTicketBlocked",
		Trigger: candidateTrigger{
			Type:        "coord",
			Label:       "SSAnneTicketCheckCoords",
			Coordinates: []scriptedevents.EventCoordinate{{MapName: "VermilionCity", X: 18, Y: 30}},
		},
		Actions: []candidateAction{{Type: "dialogue", Lines: []string{"You need a ticket."}}},
	}, resolver)
	if err != nil {
		t.Fatal(err)
	}

	if len(event.Trigger.Coordinates) != 1 {
		t.Fatalf("coordinates = %#v, want one coordinate", event.Trigger.Coordinates)
	}
	got := event.Trigger.Coordinates[0]
	if got.MapName != "VERMILION_CITY" || got.MapID != 9999 || got.X != 188 || got.Y != -24 {
		t.Fatalf("coordinate = %#v, want VERMILION_CITY map 9999 at (188,-24)", got)
	}
}

func TestMapCandidateMapsKnownEventAliases(t *testing.T) {
	event, err := mapCandidate(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "MtMoonB2F",
		ScriptLabel: "MtMoonB2FDomeFossilChoice",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_MTMOONB2F_DOME_FOSSIL",
		},
		Conditions: candidateCondition{
			RequiresEvent: "EVENT_BEAT_MT_MOON_EXIT_SUPER_NERD",
		},
		Actions: []candidateAction{
			{Type: "setEvent", Event: "EVENT_BEAT_MT_MOON_EXIT_SUPER_NERD"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresFlag != "EVENT_BEAT_MT_MOON_SUPER_NERD" {
		t.Fatalf("requiresFlag = %q, want runtime alias", event.RequiresFlag)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[0], "setFlag", "flag", "EVENT_BEAT_MT_MOON_SUPER_NERD")
}

func TestMapCandidateSupportsBadgeConditionsAndRoute23Aliases(t *testing.T) {
	event, err := mapCandidate(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route23",
		ScriptLabel: "Route23BadgeCheck2Pass",
		Trigger: candidateTrigger{
			Type:  "coord",
			Label: "Route23BadgeCheckCascadeCoords",
			Coordinates: []scriptedevents.EventCoordinate{
				{MapName: "Route23", X: 8, Y: 136},
			},
		},
		Conditions: candidateCondition{
			RequiresBadge:       "CASCADEBADGE",
			RequiresEventAbsent: "EVENT_PASSED_CASCADEBADGE_CHECK",
		},
		Actions: []candidateAction{
			{Type: "setEvent", Event: "EVENT_PASSED_CASCADEBADGE_CHECK"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresFlag != "EVENT_GOT_CASCADEBADGE" {
		t.Fatalf("requiresFlag = %q, want badge flag", event.RequiresFlag)
	}
	if event.RequiresFlagAbsent != "EVENT_ROUTE23_BADGE2_CHECKED" {
		t.Fatalf("requiresFlagAbsent = %q, want route23 alias", event.RequiresFlagAbsent)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[0], "setFlag", "flag", "EVENT_ROUTE23_BADGE2_CHECKED")
}

func TestMapCandidatePreservesChoiceTextConstant(t *testing.T) {
	event, err := mapCandidate(scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "MtMoonB2F",
		ScriptLabel: "MtMoonB2FHelixFossilChoice",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_MTMOONB2F_HELIX_FOSSIL",
		},
		Actions: []candidateAction{
			{
				Type:         "choice",
				Prompt:       "You want the HELIX FOSSIL?",
				TextConstant: "TEXT_MTMOONB2F_HELIX_FOSSIL",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[0], "choice", "textConstant", "TEXT_MTMOONB2F_HELIX_FOSSIL")
	assertMappedAction(t, actions[0], "choice", "prompt", "You want the HELIX FOSSIL?")
}

func TestMapCandidateRejectsUnsupportedAction(t *testing.T) {
	candidate := safariCandidate("UnsupportedScript", "EVENT_TEST", "")
	candidate.Actions = append(candidate.Actions, candidateAction{Type: "unknownAction"})
	if _, err := mapCandidate(candidate); err == nil {
		t.Fatal("mapCandidate returned nil error for unsupported action")
	}
}

func TestMapCandidateSupportsNeutralActionFamilies(t *testing.T) {
	candidate := safariCandidate("BroadNeutralScript", "EVENT_TEST", "")
	candidate.Actions = []candidateAction{
		{Type: "setEvent", Event: "EVENT_GOT_TEST_ITEM"},
		{Type: "resetEvent", Event: "EVENT_TEST_RESET"},
		{Type: "giveItem", ItemConstant: "POTION", Quantity: 2},
		{Type: "takeItem", ItemID: 42},
		{Type: "givePokemon", SpeciesID: 25, Level: 12, Message: "Received PIKACHU!"},
		{Type: "hideObject", ObjectKey: "TestMap_NPC_1"},
		{Type: "showObject", TriggerLabel: "TestTrigger"},
		{Type: "movePlayer", Movements: []string{"UP", "LEFT"}},
		{Type: "startWildBattle", PokemonID: 150, Level: 70, WinFlag: "EVENT_BEAT_MEWTWO", AllowedActions: []string{"item"}, GuaranteedCatch: true},
		{
			Type:              "startTrainerBattle",
			TrainerClass:      "RIVAL1",
			TrainerPartyIndex: 1,
			WinFlag:           "EVENT_BEAT_RIVAL",
			PostWinActions: []candidateAction{
				{Type: "setEvent", Event: "EVENT_RIVAL_LEFT"},
			},
		},
		{Type: "healParty"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	actions := decodeMappedActions(t, event.Actions)
	if len(actions) != len(candidate.Actions) {
		t.Fatalf("mapped %d actions, want %d: %#v", len(actions), len(candidate.Actions), actions)
	}
	assertMappedAction(t, actions[0], "setFlag", "flag", "EVENT_GOT_TEST_ITEM")
	assertMappedAction(t, actions[1], "resetFlag", "flag", "EVENT_TEST_RESET")
	assertMappedAction(t, actions[2], "giveItem", "itemName", "POTION")
	assertMappedNumber(t, actions[2], "quantity", 2)
	assertMappedAction(t, actions[3], "takeItem", "type", "takeItem")
	assertMappedNumber(t, actions[3], "itemId", 42)
	assertMappedAction(t, actions[4], "givePokemon", "message", "Received PIKACHU!")
	assertMappedNumber(t, actions[4], "pokemonId", 25)
	assertMappedAction(t, actions[5], "hideObject", "objectKey", "TestMap_NPC_1")
	assertMappedAction(t, actions[6], "showObject", "triggerLabel", "TestTrigger")
	assertMappedAction(t, actions[7], "movePlayer", "type", "movePlayer")
	assertMappedAction(t, actions[8], "startWildBattle", "winFlag", "EVENT_BEAT_MEWTWO")
	assertMappedBool(t, actions[8], "guaranteedCatch", true)
	assertMappedStringList(t, actions[8], "allowedActions", []string{"item"})
	assertMappedAction(t, actions[9], "startTrainerBattle", "trainerClass", "RIVAL1")
	assertMappedAction(t, actions[10], "healParty", "type", "healParty")

	postWin, ok := actions[9]["postWinActions"].([]any)
	if !ok || len(postWin) != 1 {
		t.Fatalf("postWinActions = %#v, want one nested action", actions[9]["postWinActions"])
	}
	nested, ok := postWin[0].(map[string]any)
	if !ok {
		t.Fatalf("nested post-win action = %#v, want object", postWin[0])
	}
	assertMappedAction(t, nested, "setFlag", "flag", "EVENT_RIVAL_LEFT")
}

func TestMapCandidateSupportsPokemonNameActions(t *testing.T) {
	candidate := safariCandidate("PokemonNameScript", "EVENT_TEST", "")
	candidate.Actions = []candidateAction{
		{Type: "givePokemon", PokemonConstant: "LAPRAS", Level: 15},
		{Type: "startWildBattle", PokemonName: "SNORLAX", Level: 30, WinFlag: "EVENT_BEAT_SNORLAX"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	actions := decodeMappedActions(t, event.Actions)
	if len(actions) != 2 {
		t.Fatalf("mapped %d actions, want 2: %#v", len(actions), actions)
	}
	assertMappedAction(t, actions[0], "givePokemon", "pokemonName", "LAPRAS")
	assertMappedNumber(t, actions[0], "level", 15)
	assertMappedAction(t, actions[1], "startWildBattle", "pokemonName", "SNORLAX")
	assertMappedNumber(t, actions[1], "level", 30)
	assertMappedAction(t, actions[1], "startWildBattle", "winFlag", "EVENT_BEAT_SNORLAX")
}

func TestMapCandidateSupportsPokedexCaughtCondition(t *testing.T) {
	candidate := safariCandidate("Route2GateOaksAideHM05Reward", "EVENT_GOT_HM05", "")
	candidate.MapName = "Route2Gate"
	candidate.Trigger = candidateTrigger{
		Type:  "npc_click",
		Label: "TEXT_ROUTE2GATE_OAKS_AIDE",
	}
	candidate.Conditions.RequiresPokedexCaught = 10
	candidate.Actions = []candidateAction{{Type: "giveItem", ItemConstant: "HM_FLASH"}}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresPokedexCaught == nil || *event.RequiresPokedexCaught != 10 {
		t.Fatalf("requiresPokedexCaught = %#v, want 10", event.RequiresPokedexCaught)
	}
}

func TestMapCandidateSupportsCoinConditionsAndActions(t *testing.T) {
	candidate := safariCandidate("GameCornerClerk2CoinsGift", "", "EVENT_GOT_20_COINS_2")
	candidate.MapName = "GameCorner"
	candidate.Trigger = candidateTrigger{
		Type:  "npc_click",
		Label: "TEXT_GAMECORNER_CLERK2",
	}
	candidate.Conditions.RequiresItem = "COIN_CASE"
	candidate.Conditions.RequiresCoinsBelow = 9990
	candidate.Actions = []candidateAction{
		{Type: "giveCoins", Coins: 20},
		{Type: "setEvent", Event: "EVENT_GOT_20_COINS_2"},
	}

	event, err := mapCandidate(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if event.RequiresItemName != "COIN_CASE" {
		t.Fatalf("requiresItemName = %q, want COIN_CASE", event.RequiresItemName)
	}
	if event.RequiresCoinsBelow == nil || *event.RequiresCoinsBelow != 9990 {
		t.Fatalf("requiresCoinsBelow = %#v, want 9990", event.RequiresCoinsBelow)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[0], "giveCoins", "type", "giveCoins")
	assertMappedNumber(t, actions[0], "coins", 20)
	assertMappedAction(t, actions[1], "setFlag", "flag", "EVENT_GOT_20_COINS_2")
}

func insertExtractorDiagnostic(t *testing.T, dbPath string, diagnostic extractorDiagnostic) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS script_event_candidate_diagnostics (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			map_name TEXT NOT NULL,
			script_label TEXT NOT NULL,
			status TEXT NOT NULL,
			reason TEXT NOT NULL,
			details_json TEXT NOT NULL
		)`); err != nil {
		t.Fatal(err)
	}
	details := string(diagnostic.Details)
	if details == "" {
		details = "{}"
	}
	if _, err := db.Exec(
		`INSERT INTO script_event_candidate_diagnostics
			(map_name, script_label, status, reason, details_json)
		 VALUES (?, ?, ?, ?, ?)`,
		diagnostic.MapName,
		diagnostic.ScriptLabel,
		diagnostic.Status,
		diagnostic.Reason,
		details,
	); err != nil {
		t.Fatal(err)
	}
}

func decodeMappedActions(t *testing.T, rawActions []json.RawMessage) []map[string]any {
	t.Helper()
	actions := make([]map[string]any, 0, len(rawActions))
	for i, raw := range rawActions {
		var action map[string]any
		if err := json.Unmarshal(raw, &action); err != nil {
			t.Fatalf("decode action %d: %v", i, err)
		}
		actions = append(actions, action)
	}
	return actions
}

func assertMappedAction(t *testing.T, action map[string]any, wantType, key, want string) {
	t.Helper()
	if got, ok := action["type"].(string); !ok || got != wantType {
		t.Fatalf("action type = %#v, want %q in %#v", action["type"], wantType, action)
	}
	if got, ok := action[key].(string); !ok || got != want {
		t.Fatalf("action[%s] = %#v, want %q in %#v", key, action[key], want, action)
	}
}

func assertMappedNumber(t *testing.T, action map[string]any, key string, want float64) {
	t.Helper()
	if got, ok := action[key].(float64); !ok || got != want {
		t.Fatalf("action[%s] = %#v, want %.0f in %#v", key, action[key], want, action)
	}
}

func assertMappedBool(t *testing.T, action map[string]any, key string, want bool) {
	t.Helper()
	if got, ok := action[key].(bool); !ok || got != want {
		t.Fatalf("action[%s] = %#v, want %t in %#v", key, action[key], want, action)
	}
}

func assertMappedStringList(t *testing.T, action map[string]any, key string, want []string) {
	t.Helper()
	values, ok := action[key].([]any)
	if !ok || len(values) != len(want) {
		t.Fatalf("action[%s] = %#v, want %#v in %#v", key, action[key], want, action)
	}
	for i, expected := range want {
		if got, ok := values[i].(string); !ok || got != expected {
			t.Fatalf("action[%s][%d] = %#v, want %q in %#v", key, i, values[i], expected, action)
		}
	}
}

func TestNameConversions(t *testing.T) {
	if got := camelToUpperSnake("SafariZoneGate"); got != "SAFARI_ZONE_GATE" {
		t.Fatalf("camelToUpperSnake SafariZoneGate = %q", got)
	}
	if got := mapNameToUpperSnake("Route12Gate2F"); got != "ROUTE_12_GATE_2F" {
		t.Fatalf("mapNameToUpperSnake Route12Gate2F = %q", got)
	}
	if got := mapNameToUpperSnake("SilphCo11F"); got != "SILPH_CO_11F" {
		t.Fatalf("mapNameToUpperSnake SilphCo11F = %q", got)
	}
	if got := mapNameToUpperSnake("CeruleanCaveB1F"); got != "CERULEAN_CAVE_B1F" {
		t.Fatalf("mapNameToUpperSnake CeruleanCaveB1F = %q", got)
	}
	if got := camelToSnake("PokemonTower2FRivalEncounter"); got != "pokemon_tower_2_f_rival_encounter" {
		t.Fatalf("camelToSnake PokemonTower2FRivalEncounter = %q", got)
	}
}
