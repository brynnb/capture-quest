import fs from "node:fs";
import path from "node:path";
import { describe, expect, test } from "vitest";
import manifest from "../../server/script_tests/story_checkpoints.json";

type StoryKind = "mainline" | "side" | "system" | "debug";
type E2EMode = "interactive" | "stateOnly" | "scriptOnly";

interface StoryCheckpointEntry {
  chapter: string;
  storyOrder: number;
  kind: StoryKind;
  e2eMode: E2EMode;
  driver: string;
}

const scenarioDir = path.resolve(process.cwd(), "server", "script_tests", "scenarios");

const allowedKinds = new Set<StoryKind>(["mainline", "side", "system", "debug"]);
const allowedE2EModes = new Set<E2EMode>(["interactive", "stateOnly", "scriptOnly"]);
const allowedDrivers = new Set([
  "actorNoScript",
  "boulderPush",
  "clickNoScript",
  "coordNoScript",
  "coordinate",
  "daycareDeposit",
  "daycareStep",
  "daycareWithdraw",
  "dialogueChoice",
  "directScript",
  "elevatorFloors",
  "elevatorSelect",
  "fieldMovePermission",
  "gameCornerBuyCoins",
  "gameCornerHiddenCoin",
  "gameCornerPrizeBuy",
  "gameCornerPrizeList",
  "gameCornerSlotPlay",
  "mapScript",
  "npcClick",
  "objectClick",
  "repelStep",
  "repelUse",
  "resolveActiveBattle",
  "runtimeBoulderPush",
  "safariBattleAction",
  "safariEnter",
  "safariStep",
  "seafoamBoulderHole",
  "seafoamCurrent",
  "silphCardKey",
  "stateOnly",
  "vermilionGymTrash",
]);

function scenarioNamesFromDisk() {
  return fs
    .readdirSync(scenarioDir)
    .filter((file) => file.endsWith(".json"))
    .map((file) => file.replace(/\.json$/, ""))
    .sort();
}

function manifestEntries() {
  return manifest.scenarios as Record<string, StoryCheckpointEntry>;
}

describe("story checkpoint manifest", () => {
  test("covers every script simulator scenario exactly once", () => {
    expect(Object.keys(manifestEntries()).sort()).toEqual(scenarioNamesFromDisk());
  });

  test("uses valid chapters, kinds, e2e modes, drivers, and unique story order values", () => {
    const chapterIds = new Set(manifest.chapters.map((chapter) => chapter.id));
    const seenOrders = new Map<number, string>();

    for (const [name, entry] of Object.entries(manifestEntries())) {
      expect(chapterIds.has(entry.chapter), `${name} chapter`).toBe(true);
      expect(Number.isInteger(entry.storyOrder), `${name} storyOrder`).toBe(true);
      expect(entry.storyOrder, `${name} storyOrder`).toBeGreaterThan(0);
      expect(allowedKinds.has(entry.kind), `${name} kind`).toBe(true);
      expect(allowedE2EModes.has(entry.e2eMode), `${name} e2eMode`).toBe(true);
      expect(allowedDrivers.has(entry.driver), `${name} driver`).toBe(true);

      const previous = seenOrders.get(entry.storyOrder);
      expect(previous, `${name} shares storyOrder ${entry.storyOrder}`).toBeUndefined();
      seenOrders.set(entry.storyOrder, name);
    }
  });

  test("keeps the core browser storyline visible to Playwright", () => {
    const entries = manifestEntries();
    expect(entries.pallet_town_oak_stops_player.kind).toBe("mainline");
    expect(entries.oak_lab_choose_starter_intro.e2eMode).not.toBe("scriptOnly");
    expect(entries.viridian_mart_oaks_parcel.e2eMode).toBe("interactive");
    expect(entries.gym_giovanni_reward.kind).toBe("mainline");
    expect(entries.champions_room_victory_to_hall_of_fame.e2eMode).not.toBe("scriptOnly");
    expect(entries.debug_field_move_surf_ready.kind).toBe("debug");
  });
});
