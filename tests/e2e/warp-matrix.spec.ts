import { expect, test, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import {
  clickTile,
  dismissDialogue,
  pressMovement,
  type MovementDirection,
} from "./helpers/input";
import {
  getGameState,
  waitForMap,
  waitForNoMapLoading,
  waitForPlayerIdle,
  waitForPlayerTile,
  waitForWarps,
} from "./helpers/state";
import type { CaptureQuestWarpProbeCase } from "../../src/testing/capturequestTestBridge";

const DEFAULT_RANDOM_CASE_LIMIT = 20;

function envBool(name: string, fallback = false) {
  const value = process.env[name];
  if (!value) return fallback;
  return ["1", "true", "yes", "on"].includes(value.toLowerCase());
}

function envNumber(name: string, fallback: number) {
  const value = process.env[name];
  if (!value) return fallback;
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function seededScore(seed: number, value: number) {
  let next = (value ^ seed) >>> 0;
  next = Math.imul(next ^ (next >>> 16), 0x7feb352d);
  next = Math.imul(next ^ (next >>> 15), 0x846ca68b);
  return (next ^ (next >>> 16)) >>> 0;
}

function selectWarpCases(cases: CaptureQuestWarpProbeCase[]) {
  const fullSweep = envBool("WARP_MATRIX_FULL");
  const explicitLimit = process.env.WARP_MATRIX_LIMIT != null;
  if (fullSweep || explicitLimit) {
    return cases;
  }

  const limit = Math.min(
    envNumber("WARP_MATRIX_RANDOM_LIMIT", DEFAULT_RANDOM_CASE_LIMIT),
    cases.length,
  );
  const seed = envNumber("WARP_MATRIX_SEED", 1337);
  return [...cases]
    .sort((a, b) => seededScore(seed, a.id) - seededScore(seed, b.id))
    .slice(0, limit);
}

function movementDirection(direction: string): MovementDirection {
  switch (direction.toUpperCase()) {
    case "UP":
      return "up";
    case "DOWN":
      return "down";
    case "LEFT":
      return "left";
    case "RIGHT":
      return "right";
    default:
      throw new Error(`Unsupported movement direction: ${direction}`);
  }
}

function describeWarp(probe: CaptureQuestWarpProbeCase) {
  return [
    `warp ${probe.id}`,
    `${probe.sourceMapName}(${probe.sourceClientMapId})`,
    `${probe.x},${probe.y}`,
    "->",
    `${probe.destinationMapName}(${probe.destinationClientMapId})`,
    `${probe.expectedX},${probe.expectedY}`,
    `${probe.warpType}/${probe.warpDirection || "ANY"}`,
  ].join(" ");
}

async function requestWarpProbeCases(page: Page) {
  const limit = envNumber("WARP_MATRIX_LIMIT", 0);
  const offset = envNumber("WARP_MATRIX_OFFSET", 0);
  const sourceMapId = envNumber("WARP_MATRIX_SOURCE_MAP_ID", 0);

  const response = await page.evaluate(
    (options) => window.__capturequestTest?.requestWarpProbeCases(options),
    {
      limit,
      offset,
      sourceMapId,
    },
  );

  expect(response, "warp probe response").toBeTruthy();
  expect(response?.success, response?.error).toBe(true);
  return response!;
}

async function warpToTile(
  page: Page,
  mapId: number,
  x: number,
  y: number,
  direction = "DOWN",
) {
  await page.evaluate(
    ({ mapId: nextMapId, x: nextX, y: nextY, direction: nextDirection }) => {
      window.__capturequestTest?.warpToMap(nextMapId, nextX, nextY, nextDirection);
    },
    { mapId, x, y, direction },
  );
  await waitForNoMapLoading(page);
  await waitForMap(page, mapId);
  await waitForPlayerTile(page, x, y, 30_000);
  await settleAfterPostWarpMove(page);
}

async function waitForWarpOutcome(page: Page, probe: CaptureQuestWarpProbeCase) {
  await waitForNoMapLoading(page);
  await waitForMap(page, probe.destinationClientMapId);
  await waitForPlayerTile(page, probe.expectedX, probe.expectedY, 30_000);
  await settleAfterPostWarpMove(page);

  const state = await getGameState(page);
  expect(state.dialogue.isOpen, `${describeWarp(probe)} opened dialogue`).toBe(false);
  expect(state.worldInput.frozen, `${describeWarp(probe)} left input frozen`).toBe(false);
}

async function assertCanMoveAfterWarp(page: Page, probe: CaptureQuestWarpProbeCase) {
  await pressMovement(page, movementDirection(probe.postWarpMoveDirection));
  await waitForPostWarpMovement(page, probe);
  await settleAfterPostWarpMove(page);

  await warpToTile(
    page,
    probe.destinationClientMapId,
    probe.expectedX,
    probe.expectedY,
    probe.keyboardDirection,
  );

  const state = await getGameState(page);
  const postMoveTargetIsWarp = state.warps.some(
    (warp) => warp.x === probe.postWarpMoveX && warp.y === probe.postWarpMoveY,
  );
  if (postMoveTargetIsWarp) {
    return;
  }

  await clickTile(page, probe.postWarpMoveX, probe.postWarpMoveY);
  await waitForPostWarpMovement(page, probe);
  await settleAfterPostWarpMove(page);
}

async function waitForPostWarpMovement(page: Page, probe: CaptureQuestWarpProbeCase) {
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        const atExpectedTarget =
          state.player.x === probe.postWarpMoveX &&
          state.player.y === probe.postWarpMoveY;
        const leftArrivalTile =
          state.player.x !== probe.expectedX || state.player.y !== probe.expectedY;
        return atExpectedTarget || leftArrivalTile;
      },
      { timeout: 20_000 },
    )
    .toBe(true);
}

async function settleAfterPostWarpMove(page: Page) {
  try {
    await waitForPlayerIdle(page, 5_000);
    return;
  } catch (err) {
    const state = await getGameState(page);
    if (!state.dialogue.isOpen) {
      throw err;
    }
  }

  await dismissDialogue(page, 6);
  await waitForPlayerIdle(page);
}

async function activateByKeyboard(page: Page, probe: CaptureQuestWarpProbeCase) {
  await warpToTile(
    page,
    probe.sourceClientMapId,
    probe.keyboardSetupX,
    probe.keyboardSetupY,
    probe.keyboardDirection,
  );
  await pressMovement(page, movementDirection(probe.keyboardDirection));
  await waitForWarpOutcome(page, probe);
  await assertCanMoveAfterWarp(page, probe);
}

async function activateByClick(page: Page, probe: CaptureQuestWarpProbeCase) {
  await warpToTile(
    page,
    probe.sourceClientMapId,
    probe.clickSetupX,
    probe.clickSetupY,
    probe.keyboardDirection,
  );
  await clickTile(page, probe.x, probe.y);
  await waitForWarpOutcome(page, probe);
  await assertCanMoveAfterWarp(page, probe);
}

test("configured warp sample works by keyboard and click", async ({ page }) => {
  const fullSweep = envBool("WARP_MATRIX_FULL");
  const configuredLimit = fullSweep
    ? 0
    : envNumber(
        "WARP_MATRIX_LIMIT",
        envNumber("WARP_MATRIX_RANDOM_LIMIT", DEFAULT_RANDOM_CASE_LIMIT),
      );
  test.setTimeout(
    configuredLimit > 0
      ? Math.max(300_000, configuredLimit * 20_000)
      : 7_200_000,
  );
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await waitForNoMapLoading(page);
  await waitForWarps(page);

  const response = await requestWarpProbeCases(page);
  const cases = selectWarpCases(response.cases);
  const skipped = response.skippedCases ?? [];
  const strictSkippedCases =
    fullSweep ||
    process.env.WARP_MATRIX_LIMIT != null ||
    process.env.WARP_MATRIX_SOURCE_MAP_ID != null;
  const range =
    fullSweep || cases.length === response.totalCaseCount
      ? `${cases.length}`
      : `${cases.length}/${response.totalCaseCount}`;

  if (strictSkippedCases) {
    expect(
      skipped.map(
        (item) =>
          `warp ${item.id} ${item.sourceMapName}(${item.sourceMapId}) ${item.x},${item.y}: ${item.reason}`,
      ),
      `skipped warp probe cases in requested range (${range})`,
    ).toEqual([]);
  }
  expect(cases.length, "warp probe cases").toBeGreaterThan(0);

  for (const [index, probe] of cases.entries()) {
    const label = `[${index + 1}/${cases.length}] ${describeWarp(probe)}`;
    console.log(`[warp-matrix] ${label}`);
    await test.step(`${label} keyboard`, async () => {
      await activateByKeyboard(page, probe);
    });
    await test.step(`${label} click`, async () => {
      await activateByClick(page, probe);
    });
  }

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
