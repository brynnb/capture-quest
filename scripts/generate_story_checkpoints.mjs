import fs from "node:fs";
import path from "node:path";

const repoRoot = process.cwd();
const scenariosDir = path.join(repoRoot, "server", "script_tests", "scenarios");
const outputPath = path.join(repoRoot, "server", "script_tests", "story_checkpoints.json");

const chapters = [
  { id: "01_intro_pallet", title: "Intro, Pallet Town, and Oak's Parcel", order: 100 },
  { id: "02_viridian_pewter", title: "Viridian Forest, Pewter City, and Brock", order: 200 },
  { id: "03_mt_moon_cerulean", title: "Mt. Moon, Cerulean City, Misty, and Bill", order: 300 },
  { id: "04_vermilion_ss_anne", title: "Vermilion City, S.S. Anne, Cut, and Lt. Surge", order: 400 },
  { id: "05_celadon_rocket_lavender", title: "Celadon City, Rocket Hideout, and Lavender Tower", order: 500 },
  { id: "06_fuchsia_safari", title: "Snorlax, Fuchsia City, Safari Zone, Surf, Strength, and Koga", order: 600 },
  { id: "07_saffron_silph", title: "Saffron City, Silph Co., and Sabrina", order: 700 },
  { id: "08_cinnabar_blaine", title: "Cinnabar Island, Pokemon Mansion, Fossils, and Blaine", order: 800 },
  { id: "09_viridian_victory_road", title: "Viridian Gym, Badge Gates, and Victory Road", order: 900 },
  { id: "10_elite_four_hall_of_fame", title: "Elite Four, Champion, and Hall of Fame", order: 1000 },
  { id: "80_side_content", title: "Side Content and Optional Rewards", order: 8000 },
  { id: "90_system_mechanics", title: "Reusable Mechanics and System Scripts", order: 9000 },
  { id: "99_debug_fixtures", title: "Debug Fixtures", order: 9900 },
];

const chapterById = Object.fromEntries(chapters.map((chapter) => [chapter.id, chapter]));

const mainlineOrder = [
  "pallet_town_oak_stops_player",
  "oak_lab_choose_starter_intro",
  "oak_starter_charmander_prompt",
  "oak_starter_charmander",
  "oak_first_rival_battle_charmander_win",
  "oak_lab_rival_exits_after_battle",
  "viridian_mart_oaks_parcel",
  "oak_lab_pokedex_delivery",
  "viridian_city_old_man_catch_demo_start",
  "viridian_city_old_man_catch_demo_win",
  "pallet_town_after_oak_pokeballs_flag",
  "pewter_city_youngster_gym_guide",
  "pewter_gym_brock_generated_pre_battle",
  "gym_brock_reward",
  "mt_moon_b2f_fossil_prompt",
  "mt_moon_b2f_dome_fossil",
  "cerulean_rival_charmander_start",
  "cerulean_rival_charmander_win",
  "bills_house_ss_ticket",
  "gym_misty_reward",
  "vermilion_city_ss_anne_guard_pass",
  "ss_anne_2f_rival_charmander_start",
  "ss_anne_2f_rival_charmander_win",
  "ss_anne_captain_hm01",
  "vermilion_city_ss_anne_departure",
  "gym_lt_surge_reward",
  "game_corner_poster_opens_rocket_hideout",
  "rocket_hideout_b4f_rocket3_lift_key_drop",
  "rocket_hideout_b4f_giovanni_start",
  "rocket_hideout_b4f_giovanni_win",
  "pokemon_tower_2f_rival_charmander_start",
  "pokemon_tower_2f_rival_charmander_right_side_win",
  "pokemon_tower_6f_marowak_start",
  "pokemon_tower_6f_marowak_win",
  "pokemon_tower_7f_mr_fuji_rescue",
  "mr_fujis_house_poke_flute",
  "route12_snorlax_start",
  "route12_snorlax_win",
  "safari_zone_gate_entry_offer_yes",
  "safari_secret_house_hm03",
  "wardens_house_hm04",
  "gym_koga_reward",
  "silph_card_key_5f_door1_open",
  "silph_co_7f_rival_charmander_start",
  "silph_co_7f_rival_charmander_upper_win",
  "silph_co_7f_lapras",
  "silph_co_11f_giovanni_start",
  "silph_co_11f_giovanni_win",
  "silph_co_11f_master_ball",
  "gym_sabrina_reward",
  "pokemon_mansion_1f_generated_switch_turn_on",
  "cinnabar_lab_submit_old_amber",
  "cinnabar_lab_receive_aerodactyl",
  "gym_blaine_reward",
  "viridian_city_gym_open_from_earth_badge",
  "gym_giovanni_reward",
  "route22_rival2_charmander_start",
  "route22_rival2_charmander_win",
  "route22_gate_boulder_badge_pass",
  "route23_cascade_badge_pass",
  "route23_thunder_badge_pass",
  "route23_rainbow_badge_pass",
  "route23_soul_badge_pass",
  "route23_marsh_badge_pass",
  "route23_volcano_badge_pass",
  "route23_earth_badge_pass",
  "victory_road_2f_moltres_start",
  "victory_road_2f_moltres_win",
  "loreleis_room_post_battle",
  "brunos_room_post_battle",
  "agathas_room_post_battle",
  "lances_room_lance_coord_battle_start",
  "lances_room_post_battle",
  "champions_room_rival_charmander_start",
  "champions_room_rival_charmander_win",
  "champions_room_victory_to_hall_of_fame",
  "hall_of_fame_oak_congratulations",
];

const mainlineOrderMap = new Map(mainlineOrder.map((name, index) => [name, index]));

const interactiveScenarios = new Set([
  "active_battle_fixture_wild",
  "bike_shop_no_voucher_yes",
  "debug_celadon_diner_coin_case_ready",
  "debug_field_move_cut_ready",
  "debug_field_move_surf_ready",
  "debug_game_corner_buy_coins_ready",
  "debug_game_corner_prize_full_party_ready",
  "debug_game_corner_slots_ready",
  "debug_inventory_blocked_items_ready",
  "debug_inventory_old_rod_land_ready",
  "debug_inventory_old_rod_water_ready",
  "debug_inventory_potion_ready",
  "debug_pokemon_center_pc_ready",
  "game_corner_prize_list_with_coin_case",
  "oak_lab_choose_starter_intro",
  "viridian_city_old_man_catch_demo_start",
  "viridian_mart_oaks_parcel",
]);

const extraStateOnlyScenarios = new Set([
  "bike_shop_already_has_bicycle",
  "game_corner_prize_list_no_coin_case",
  "debug_warp_cerulean_trashed_house_upper_exit",
  "debug_warp_cinnabar_lab_fossil_door",
  "debug_warp_cinnabar_lab_lobby_exit",
  "debug_warp_cinnabar_lab_metronome_door",
  "debug_warp_cinnabar_lab_trade_door",
  "debug_warp_oaks_lab_exit_mat",
  "debug_warp_reds_house_1f_exit_mat",
  "debug_warp_route11_gate_1f_center",
  "debug_warp_underground_path_route6_exit_mat",
  "debug_warp_viridian_mart_exit_mat",
]);

const directDriverByTrigger = {
  active_battle_state: "stateOnly",
  boulder_push: "boulderPush",
  click_no_script: "clickNoScript",
  coord: "coordinate",
  coord_no_script: "coordNoScript",
  daycare_deposit: "daycareDeposit",
  daycare_step: "daycareStep",
  daycare_withdraw: "daycareWithdraw",
  dialogue_choice: "dialogueChoice",
  direct: "directScript",
  elevator_floors: "elevatorFloors",
  elevator_select: "elevatorSelect",
  event_object_state: "stateOnly",
  field_move_permission: "fieldMovePermission",
  fixture_state: "stateOnly",
  gamecorner_buy_coins: "gameCornerBuyCoins",
  gamecorner_hidden_coin: "gameCornerHiddenCoin",
  gamecorner_prize_buy: "gameCornerPrizeBuy",
  gamecorner_prize_list: "gameCornerPrizeList",
  gamecorner_slot_play: "gameCornerSlotPlay",
  map_load: "stateOnly",
  map_script: "mapScript",
  npc_click: "npcClick",
  object_click: "objectClick",
  pathfind: "stateOnly",
  repel_step: "repelStep",
  repel_use: "repelUse",
  resolve_active_battle: "resolveActiveBattle",
  runtime_boulder_push: "runtimeBoulderPush",
  safari_battle_action: "safariBattleAction",
  safari_enter: "safariEnter",
  safari_step: "safariStep",
  seafoam_boulder_hole: "seafoamBoulderHole",
  seafoam_current: "seafoamCurrent",
  seafoam_surf_check: "stateOnly",
  silph_card_key: "silphCardKey",
  tile_state: "stateOnly",
  vermilion_gym_trash: "vermilionGymTrash",
};

function loadScenarios() {
  return fs
    .readdirSync(scenariosDir)
    .filter((file) => file.endsWith(".json"))
    .sort()
    .map((file) => {
      const scenario = JSON.parse(fs.readFileSync(path.join(scenariosDir, file), "utf8"));
      return {
        file,
        name: scenario.name || file.replace(/\.json$/, ""),
        scenario,
      };
    });
}

function chapterForScenario(name, scenario) {
  const mainlineIndex = mainlineOrderMap.get(name);
  if (mainlineIndex !== undefined) {
    return chapterForMainlineIndex(mainlineIndex);
  }
  const text = `${name} ${scenario.fixture?.mapName || ""} ${scenario.trigger?.mapName || ""}`.toLowerCase();
  if (name.startsWith("debug_") || name.startsWith("fixture_") || name.startsWith("active_battle_fixture")) {
    return "99_debug_fixtures";
  }
  if (text.includes("pallet") || text.includes("oak") || text.includes("starter") || text.includes("viridian_mart")) {
    return "01_intro_pallet";
  }
  if (text.includes("viridian_city") || text.includes("route_1") || /\broute2(_|$)/.test(text) || text.includes("pewter")) {
    return "02_viridian_pewter";
  }
  if (text.includes("mt_moon") || text.includes("cerulean") || text.includes("bill") || text.includes("route25")) {
    return "03_mt_moon_cerulean";
  }
  if (text.includes("vermilion") || text.includes("ss_anne") || text.includes("pokemon_fan_club") || text.includes("route11")) {
    return "04_vermilion_ss_anne";
  }
  if (text.includes("celadon") || text.includes("rocket_hideout") || text.includes("pokemon_tower") || text.includes("lavender") || text.includes("mr_fuji")) {
    return "05_celadon_rocket_lavender";
  }
  if (text.includes("fuchsia") || text.includes("safari") || text.includes("snorlax") || text.includes("route12") || text.includes("route15") || text.includes("route16") || text.includes("route18")) {
    return "06_fuchsia_safari";
  }
  if (text.includes("saffron") || text.includes("silph") || text.includes("copycat") || text.includes("fighting_dojo")) {
    return "07_saffron_silph";
  }
  if (text.includes("cinnabar") || text.includes("pokemon_mansion") || text.includes("mansion")) {
    return "08_cinnabar_blaine";
  }
  if (text.includes("viridian_gym") || text.includes("route22") || text.includes("route23") || text.includes("victory_road")) {
    return "09_viridian_victory_road";
  }
  if (text.includes("lorelei") || text.includes("bruno") || text.includes("agatha") || text.includes("lance") || text.includes("champion") || text.includes("hall_of_fame") || text.includes("indigo")) {
    return "10_elite_four_hall_of_fame";
  }
  if (
    text.includes("trade") ||
    text.includes("daycare") ||
    text.includes("museum") ||
    text.includes("game_corner") ||
    text.includes("fossil") ||
    text.includes("power_plant") ||
    text.includes("mewtwo") ||
    text.includes("zapdos") ||
    text.includes("articuno") ||
    text.includes("moltres") ||
    text.includes("old_rod") ||
    text.includes("good_rod") ||
    text.includes("super_rod") ||
    text.includes("route5_gate") ||
    text.includes("underground")
  ) {
    return "80_side_content";
  }
  return "90_system_mechanics";
}

function chapterForMainlineIndex(index) {
  if (index <= 7) return "01_intro_pallet";
  if (index <= 13) return "02_viridian_pewter";
  if (index <= 19) return "03_mt_moon_cerulean";
  if (index <= 25) return "04_vermilion_ss_anne";
  if (index <= 35) return "05_celadon_rocket_lavender";
  if (index <= 41) return "06_fuchsia_safari";
  if (index <= 49) return "07_saffron_silph";
  if (index <= 53) return "08_cinnabar_blaine";
  if (index <= 65) return "09_viridian_victory_road";
  return "10_elite_four_hall_of_fame";
}

function kindForScenario(name, scenario) {
  if (name.startsWith("debug_") || name.startsWith("fixture_") || name.startsWith("active_battle_fixture")) {
    return "debug";
  }
  if (mainlineOrderMap.has(name)) {
    return "mainline";
  }
  const trigger = scenario.trigger?.type || "";
  if (
    trigger === "field_move_permission" ||
    trigger === "pathfind" ||
    trigger === "tile_state" ||
    trigger === "event_object_state" ||
    trigger === "map_load" ||
    trigger === "repel_use" ||
    trigger === "repel_step"
  ) {
    return "system";
  }
  return "side";
}

function e2eModeForScenario(name, kind) {
  if (interactiveScenarios.has(name)) return "interactive";
  if (extraStateOnlyScenarios.has(name) || kind === "mainline") return "stateOnly";
  return "scriptOnly";
}

function storyOrderForScenario(name, scenario, chapterId, indexInChapter) {
  const chapter = chapterById[chapterId];
  const mainlineIndex = mainlineOrderMap.get(name);
  if (mainlineIndex !== undefined) {
    return chapter.order * 10000 + mainlineIndex * 10;
  }
  const triggerRank = triggerRankForScenario(scenario.trigger?.type || "");
  return chapter.order * 10000 + 5000 + triggerRank * 100 + indexInChapter;
}

function triggerRankForScenario(triggerType) {
  const ranks = {
    fixture_state: 1,
    map_load: 2,
    map_script: 3,
    coord: 4,
    npc_click: 5,
    object_click: 6,
    dialogue_choice: 7,
    direct: 8,
  };
  return ranks[triggerType] ?? 9;
}

const scenarios = loadScenarios();
const chapterCounters = new Map();
const manifestScenarios = {};

for (const { name, scenario } of scenarios) {
  const chapter = chapterForScenario(name, scenario);
  const counter = (chapterCounters.get(chapter) || 0) + 1;
  chapterCounters.set(chapter, counter);
  const kind = kindForScenario(name, scenario);
  const e2eMode = e2eModeForScenario(name, kind);
  const triggerType = scenario.trigger?.type || "";
  const driver = directDriverByTrigger[triggerType] || "unknown";

  manifestScenarios[name] = {
    chapter,
    storyOrder: storyOrderForScenario(name, scenario, chapter, counter),
    kind,
    e2eMode,
    driver,
  };
}

const manifest = {
  version: 1,
  description:
    "Story ordering and browser coverage metadata for script simulator scenarios.",
  sources: [
    "https://bulbapedia.bulbagarden.net/wiki/Walkthrough:Pok%C3%A9mon_Red_and_Blue",
    "https://bulbapedia.bulbagarden.net/wiki/Badge_sequence",
  ],
  chapters,
  scenarios: manifestScenarios,
};

fs.writeFileSync(outputPath, `${JSON.stringify(manifest, null, 2)}\n`);
console.log(`Wrote ${Object.keys(manifestScenarios).length} story checkpoints to ${outputPath}`);
