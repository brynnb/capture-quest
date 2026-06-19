import fs from "node:fs";
import path from "node:path";
import { expect, type Page } from "@playwright/test";
import { pressSpace } from "./input";
import { jumpToScenario } from "./scenarioDebugger";
import { getGameState, waitForPlayerTile } from "./state";

export type StoryKind = "mainline" | "side" | "system" | "debug";
export type E2EMode = "interactive" | "stateOnly" | "scriptOnly";

export interface StoryCheckpointEntry {
  chapter: string;
  storyOrder: number;
  kind: StoryKind;
  e2eMode: E2EMode;
  driver: string;
}

export interface StoryCheckpointScenario {
  name: string;
  description?: string;
  fixture?: {
    activeBattle?: {
      type?: string;
    };
    mapName?: string;
    x?: number;
    y?: number;
    direction?: string;
  };
  trigger?: {
    type?: string;
    bet?: number;
    isLucky?: boolean;
    mapName?: string;
    textConstant?: string;
    scriptLabel?: string;
  };
  expect?: {
    scriptLabel?: string;
    finalX?: number;
    finalY?: number;
    activeBattleAbsent?: boolean;
    activeBattleType?: string;
    inventoryContains?: Array<{ itemId?: number; itemName?: string; quantity?: number }>;
    partySpecies?: number[];
    pcContains?: Array<{ speciesId: number }>;
  };
}

export interface StoryCheckpoint {
  name: string;
  entry: StoryCheckpointEntry;
  scenario: StoryCheckpointScenario;
}

interface StoryManifest {
  scenarios: Record<string, StoryCheckpointEntry>;
}

const scenarioDir = path.resolve(process.cwd(), "server", "script_tests", "scenarios");
const manifestPath = path.resolve(process.cwd(), "server", "script_tests", "story_checkpoints.json");

function readJSON<T>(filePath: string): T {
  return JSON.parse(fs.readFileSync(filePath, "utf8")) as T;
}

export function loadStoryManifest(): StoryManifest {
  return readJSON<StoryManifest>(manifestPath);
}

export function loadStoryScenario(name: string): StoryCheckpointScenario {
  return readJSON<StoryCheckpointScenario>(path.join(scenarioDir, `${name}.json`));
}

export function orderedStoryCheckpoints(
  predicate: (checkpoint: StoryCheckpoint) => boolean,
): StoryCheckpoint[] {
  const manifest = loadStoryManifest();
  return Object.entries(manifest.scenarios)
    .map(([name, entry]) => ({
      name,
      entry,
      scenario: loadStoryScenario(name),
    }))
    .filter(predicate)
    .sort((a, b) => {
      const orderDiff = a.entry.storyOrder - b.entry.storyOrder;
      if (orderDiff !== 0) return orderDiff;
      return a.name.localeCompare(b.name);
    });
}

export async function jumpToStoryCheckpoint(page: Page, checkpoint: StoryCheckpoint) {
  await jumpToScenario(page, checkpoint.name);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.debug.lastAppliedScenario?.scenarioName ?? null;
      },
      { timeout: 15_000 },
    )
    .toBe(checkpoint.name);
}

export async function assertStoryFixtureApplied(page: Page, checkpoint: StoryCheckpoint) {
  const fixture = checkpoint.scenario.fixture;
  if (!fixture) return;

  if (
    typeof fixture.x === "number" &&
    typeof fixture.y === "number" &&
    !isOverworldFixtureMap(fixture.mapName)
  ) {
    await waitForPlayerTile(page, fixture.x, fixture.y, 30_000);
  }

}

function isOverworldFixtureMap(mapName?: string) {
  if (!mapName) return false;
  return (
    mapName === "PALLET_TOWN" ||
    mapName.endsWith("_CITY") ||
    mapName.endsWith("_TOWN") ||
    mapName.endsWith("_ISLAND") ||
    mapName.startsWith("ROUTE")
  );
}

export async function assertStoryCheckpointState(page: Page, checkpoint: StoryCheckpoint) {
  if (checkpoint.entry.e2eMode === "stateOnly") {
    await assertStoryFixtureApplied(page, checkpoint);
  }

  const expectedBattleType = checkpoint.scenario.fixture?.activeBattle?.type;
  if (expectedBattleType) {
    await expect
      .poll(
        async () => {
          const state = await getGameState(page);
          return state.battle.battleType;
        },
        { timeout: 15_000 },
      )
      .toBe(expectedBattleType);
  }
}

export async function performScenarioTrigger(page: Page, checkpoint: StoryCheckpoint) {
  switch (checkpoint.entry.driver) {
    case "gameCornerBuyCoins":
      await page.evaluate(() => window.__capturequestTest?.buyGameCornerCoins());
      return;
    case "gameCornerPrizeList":
      await page.evaluate(() => window.__capturequestTest?.requestGameCornerPrizeList());
      return;
    case "gameCornerSlotPlay":
      await page.evaluate(
        ([bet, isLucky]) =>
          window.__capturequestTest?.playGameCornerSlot(
            Number(bet) || 1,
            Boolean(isLucky),
          ),
        [checkpoint.scenario.trigger?.bet, checkpoint.scenario.trigger?.isLucky],
      );
      return;
    case "npcClick":
    case "objectClick":
    case "dialogueChoice":
      await pressSpace(page);
      return;
    case "stateOnly":
      return;
    default:
      throw new Error(
        `Story checkpoint ${checkpoint.name} does not have a browser trigger driver for ${checkpoint.entry.driver}`,
      );
  }
}

export async function assertScenarioOutcome(page: Page, checkpoint: StoryCheckpoint) {
  await assertStoryCheckpointState(page, checkpoint);
}
