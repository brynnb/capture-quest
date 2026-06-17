import { expect, type Page } from "@playwright/test";
import type { CaptureQuestTestState } from "../../../src/testing/capturequestTestBridge";
import { clickTile, pressMovement, type MovementDirection } from "./input";
import {
  getGameState,
  waitForMapChange,
  waitForNoMapLoading,
  waitForPlayerIdle,
  waitForPlayerTile,
  waitForWarps,
} from "./state";

type TestWarp = CaptureQuestTestState["warps"][number];

export function requireWarp(
  state: CaptureQuestTestState,
  predicate: (warp: TestWarp) => boolean,
  description: string,
) {
  const warp = state.warps.find(predicate);
  expect(warp, description).toBeTruthy();
  return warp!;
}

export function tileBeforeWarp(warp: TestWarp, direction: MovementDirection) {
  switch (direction) {
    case "up":
      return { x: warp.x, y: warp.y + 1 };
    case "down":
      return { x: warp.x, y: warp.y - 1 };
    case "left":
      return { x: warp.x + 1, y: warp.y };
    case "right":
      return { x: warp.x - 1, y: warp.y };
  }
}

export function tileBeyondWarp(warp: TestWarp, direction: MovementDirection) {
  switch (direction) {
    case "up":
      return { x: warp.x, y: warp.y - 1 };
    case "down":
      return { x: warp.x, y: warp.y + 1 };
    case "left":
      return { x: warp.x - 1, y: warp.y };
    case "right":
      return { x: warp.x + 1, y: warp.y };
  }
}

export async function moveToTileAndWait(page: Page, x: number, y: number) {
  const state = await getGameState(page);
  if (state.player.x === x && state.player.y === y) {
    await waitForPlayerIdle(page);
    return;
  }

  await clickTile(page, x, y);
  await waitForPlayerTile(page, x, y, 30_000);
  await waitForPlayerIdle(page);
}

export async function moveToTileByKeyboardAndWait(
  page: Page,
  x: number,
  y: number,
  maxSteps = 30,
) {
  for (let step = 0; step < maxSteps; step += 1) {
    const state = await getGameState(page);
    if (state.player.x === x && state.player.y === y) {
      await waitForPlayerIdle(page);
      return;
    }

    const dx = x - (state.player.x ?? x);
    const dy = y - (state.player.y ?? y);
    const direction: MovementDirection =
      dx < 0 ? "left" : dx > 0 ? "right" : dy < 0 ? "up" : "down";

    await pressMovement(page, direction);
    await waitForPlayerIdle(page);
  }

  await waitForPlayerTile(page, x, y, 5_000);
}

export async function activateDoorWarpWithKeyboard(
  page: Page,
  warp: TestWarp,
  direction: MovementDirection,
) {
  const before = tileBeforeWarp(warp, direction);
  await moveToTileAndWait(page, before.x, before.y);

  const startingMapId = (await getGameState(page)).map.id;
  await pressMovement(page, direction);
  await waitForMapChange(page, startingMapId);
  await waitForWarps(page);
  await waitForPlayerIdle(page);
}

export async function activateDirectionalWarpWithKeyboard(
  page: Page,
  warp: TestWarp,
  direction: MovementDirection,
) {
  await moveToTileAndWait(page, warp.x, warp.y);

  const startingMapId = (await getGameState(page)).map.id;
  await pressMovement(page, direction);
  await waitForMapChange(page, startingMapId);
  await waitForWarps(page);
  await waitForPlayerIdle(page);
}

export async function activateWarpWithClick(
  page: Page,
  warp: TestWarp,
  setupTile?: { x: number; y: number },
) {
  if (setupTile) {
    await moveToTileAndWait(page, setupTile.x, setupTile.y);
  } else {
    await waitForPlayerIdle(page);
  }

  const startingMapId = (await getGameState(page)).map.id;
  await clickTile(page, warp.x, warp.y);
  await waitForMapChange(page, startingMapId);
  await waitForNoMapLoading(page);
  await waitForWarps(page);
  await waitForPlayerIdle(page);
}

export async function activateCurrentWarpByClickingBeyond(
  page: Page,
  warp: TestWarp,
  direction: MovementDirection,
) {
  await moveToTileAndWait(page, warp.x, warp.y);

  const target = tileBeyondWarp(warp, direction);
  const startingMapId = (await getGameState(page)).map.id;
  await clickTile(page, target.x, target.y);
  await waitForMapChange(page, startingMapId);
  await waitForNoMapLoading(page);
  await waitForWarps(page);
  await waitForPlayerIdle(page);
}
