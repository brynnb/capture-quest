package scriptcandidateimport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

func TestRunPreservesCaptureQuestOverride(t *testing.T) {
	candidate := safariCandidate("SafariZoneGateExit", "", "EVENT_IN_SAFARI_ZONE")
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	path := filepath.Join(outputDir, "safari_zone_gate_exit.json")
	override := []byte(`{
  "scriptLabel": "SafariZoneGateExit",
  "mapName": "SAFARI_ZONE_GATE",
  "trigger": {
    "type": "coord",
    "source": "capturequest",
    "label": "SafariZoneGateExit",
    "coordinates": [{"mapName": "SafariZoneGate", "mapId": 156, "x": 3, "y": 2}]
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(path, override, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want one skipped override", stats)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(override) {
		t.Fatalf("override was modified:\n%s", after)
	}
}

func TestRunPreservesCaptureQuestTriggerOverride(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "BikeShop",
		ScriptLabel: "GeneratedBikeShopClerk",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_BIKESHOP_CLERK",
		},
		Actions:    []candidateAction{{Type: "lockInput"}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	overridePath := filepath.Join(outputDir, "bike_shop_exchange_voucher.json")
	override := []byte(`{
  "scriptLabel": "BikeShopExchangeVoucher",
  "mapName": "BIKE_SHOP",
  "trigger": {
    "type": "npc_click",
    "source": "capturequest",
    "label": "TEXT_BIKESHOP_CLERK"
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(overridePath, override, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want trigger override skipped", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_bike_shop_clerk.json")); !os.IsNotExist(err) {
		t.Fatalf("generated trigger duplicate exists or stat failed: %v", err)
	}
	after, err := os.ReadFile(overridePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(override) {
		t.Fatalf("override was modified:\n%s", after)
	}
}

func TestRunSkipsExistingExtractorTriggerOwner(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route12Gate2F",
		ScriptLabel: "GeneratedRoute12Gift",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE12GATE2F_BRUNETTE_GIRL",
		},
		Actions:    []candidateAction{{Type: "giveItem", ItemConstant: "TM_SWIFT"}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route12_gate2_ftm39_swift.json")
	existing := []byte(`{
  "scriptLabel": "Route12Gate2FTM39Swift",
  "mapName": "ROUTE12_GATE2_F",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_ROUTE12GATE2F_BRUNETTE_GIRL"
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want existing trigger skipped", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_route12_gift.json")); !os.IsNotExist(err) {
		t.Fatalf("generated trigger duplicate exists or stat failed: %v", err)
	}
}

func TestRunMergesSourceAudioIntoExistingExtractorReward(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route12Gate2F",
		ScriptLabel: "GeneratedRoute12Gift",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE12GATE2F_BRUNETTE_GIRL",
		},
		Conditions: candidateCondition{
			RequiresEventAbsent: "EVENT_GOT_TM39",
		},
		Actions: []candidateAction{
			{Type: "dialogue", Lines: []string{"TM39 is SWIFT!"}},
			{Type: "giveItem", ItemConstant: "TM_SWIFT"},
			{Type: "playSFX", SFXConstant: "SFX_GET_ITEM_1"},
			{Type: "setEvent", Event: "EVENT_GOT_TM39"},
			{Type: "unlockInput"},
		},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route12_gate2_ftm39_swift.json")
	existing := []byte(`{
  "scriptLabel": "Route12Gate2FTM39Swift",
  "mapName": "ROUTE12_GATE2_F",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_ROUTE12GATE2F_BRUNETTE_GIRL"
  },
  "setsFlags": ["EVENT_GOT_TM39"],
  "actions": [
    {"type": "lockInput"},
    {"type": "dialogue", "lines": ["TM39 is SWIFT!"]},
    {"type": "setFlag", "flag": "EVENT_GOT_TM39"},
    {"type": "unlockInput"},
    {"type": "giveItem", "itemName": "TM_SWIFT", "quantity": 1}
  ]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.SkippedOverrides != 0 {
		t.Fatalf("stats = %+v, want existing extractor reward enriched in place", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_route12_gift.json")); !os.IsNotExist(err) {
		t.Fatalf("generated duplicate exists or stat failed: %v", err)
	}

	raw, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	var event scriptedevents.EventFile
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	actions := decodeMappedActions(t, event.Actions)
	assertMappedAction(t, actions[3], "playSFX", "sfxConstant", "SFX_GET_ITEM_1")
	assertMappedAction(t, actions[4], "unlockInput", "type", "unlockInput")
	assertMappedAction(t, actions[5], "giveItem", "itemName", "TM_SWIFT")
}

func TestRunAllowsConditionedExtractorTriggerBranch(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "FightingDojo",
		ScriptLabel: "FightingDojoHitmonleeAlreadyGot",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_FIGHTINGDOJO_HITMONLEE_POKE_BALL",
		},
		Conditions: candidateCondition{
			RequiresEvent: "EVENT_GOT_FIGHTING_DOJO_POKEMON",
		},
		Actions:    []candidateAction{{Type: "dialogue", Lines: []string{"Better not get greedy..."}}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "fighting_dojo_hitmonlee_choice.json")
	existing := []byte(`{
  "scriptLabel": "FightingDojoHitmonleeChoice",
  "mapName": "FIGHTING_DOJO",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_FIGHTINGDOJO_HITMONLEE_POKE_BALL"
  },
  "requiresFlagAbsent": "EVENT_GOT_FIGHTING_DOJO_POKEMON",
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.SkippedOverrides != 0 {
		t.Fatalf("stats = %+v, want conditioned branch written", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "fighting_dojo_hitmonlee_already_got.json")); err != nil {
		t.Fatalf("conditioned branch was not written: %v", err)
	}
}

func TestRunAllowsFacingConditionedExtractorTriggerBranch(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route11Gate2F",
		ScriptLabel: "Route11Gate2FLeftBinocularsSnorlax",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE11GATE2F_LEFT_BINOCULARS",
		},
		Conditions: candidateCondition{
			RequiresPlayerFacing: "up",
		},
		Actions:    []candidateAction{{Type: "dialogue", Lines: []string{"A big POKEMON is asleep on a road!"}}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route11_gate2f_left_binoculars_view.json")
	existing := []byte(`{
  "scriptLabel": "Route11Gate2FLeftBinocularsView",
  "mapName": "ROUTE_11_GATE_2F",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_ROUTE11GATE2F_LEFT_BINOCULARS"
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.SkippedOverrides != 0 {
		t.Fatalf("stats = %+v, want facing-conditioned branch written", stats)
	}

	generatedPath := filepath.Join(outputDir, "route_11_gate_2_f_left_binoculars_snorlax.json")
	raw, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated branch: %v", err)
	}
	var event scriptedevents.EventFile
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("decode generated event: %v", err)
	}
	if event.RequiresPlayerFacing != "UP" {
		t.Fatalf("requiresPlayerFacing = %q, want UP", event.RequiresPlayerFacing)
	}
}

func TestRunSkipsExistingMapFlagOwner(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "MrFujisHouse",
		ScriptLabel: "GeneratedMrFujiPokeFlute",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_MRFUJISHOUSE_MR_FUJI",
		},
		Actions: []candidateAction{
			{Type: "giveItem", ItemConstant: "POKE_FLUTE"},
			{Type: "setEvent", Event: "EVENT_GOT_POKE_FLUTE"},
		},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "mr_fujis_house_poke_flute.json")
	existing := []byte(`{
  "scriptLabel": "MrFujisHousePokeFlute",
  "mapName": "MR_FUJIS_HOUSE",
  "trigger": {
    "type": "map_script",
    "source": "extractor"
  },
  "setsFlags": ["EVENT_GOT_POKE_FLUTE"],
  "actions": [{"type": "setFlag", "flag": "EVENT_GOT_POKE_FLUTE"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want existing map flag owner skipped", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_mr_fuji_poke_flute.json")); !os.IsNotExist(err) {
		t.Fatalf("generated map-flag duplicate exists or stat failed: %v", err)
	}
}

func TestRunSkipsExistingMapFlagOwnerForBattleWinFlag(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route24",
		ScriptLabel: "GeneratedRoute24RocketBattle",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE24_COOLTRAINER_M1",
		},
		Actions: []candidateAction{
			{
				Type:              "startTrainerBattle",
				TrainerClass:      "ROCKET",
				TrainerPartyIndex: 6,
				WinFlag:           "EVENT_BEAT_ROUTE24_ROCKET",
			},
		},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route24_nugget_bridge_rocket.json")
	existing := []byte(`{
  "scriptLabel": "Route24NuggetBridgeRocket",
  "mapName": "ROUTE_24",
  "trigger": {
    "type": "coord",
    "source": "capturequest",
    "label": "Route24NuggetBridgeRocketCoords",
    "coordinates": [{"mapName": "Route24", "mapId": 9999, "x": 190, "y": -219}]
  },
  "setsFlags": ["EVENT_BEAT_ROUTE24_ROCKET"],
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.SkippedOverrides != 1 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want existing battle win-flag owner skipped", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "generated_route24_rocket_battle.json")); !os.IsNotExist(err) {
		t.Fatalf("generated battle duplicate exists or stat failed: %v", err)
	}
}

func TestRunUpdatesExistingExtractorLabelInPlace(t *testing.T) {
	candidate := scriptCandidate{
		Version:     1,
		Kind:        "scriptEventCandidate",
		MapName:     "Route2Gate",
		ScriptLabel: "Route2GateOaksAideHM05Reward",
		Trigger: candidateTrigger{
			Type:  "npc_click",
			Label: "TEXT_ROUTE2GATE_OAKS_AIDE",
		},
		Conditions: candidateCondition{
			RequiresEventAbsent:   "EVENT_GOT_HM05",
			RequiresPokedexCaught: 10,
		},
		Actions:    []candidateAction{{Type: "giveItem", ItemConstant: "HM_FLASH"}},
		Confidence: "adapter",
	}
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	existingPath := filepath.Join(outputDir, "route2_gate_oaks_aide_hm05_reward.json")
	existing := []byte(`{
  "scriptLabel": "Route2GateOaksAideHM05Reward",
  "mapName": "ROUTE_2_GATE",
  "trigger": {
    "type": "npc_click",
    "source": "extractor",
    "label": "TEXT_ROUTE2GATE_OAKS_AIDE"
  },
  "actions": [{"type": "lockInput"}]
}
`)
	if err := os.WriteFile(existingPath, existing, 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.SkippedOverrides != 0 {
		t.Fatalf("stats = %+v, want one in-place write", stats)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "route_2_gate_oaks_aide_hm_05_reward.json")); !os.IsNotExist(err) {
		t.Fatalf("canonical duplicate exists or stat failed: %v", err)
	}
	raw, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	var event scriptedevents.EventFile
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	if event.RequiresPokedexCaught == nil || *event.RequiresPokedexCaught != 10 {
		t.Fatalf("requiresPokedexCaught = %#v, want 10", event.RequiresPokedexCaught)
	}
}
