import { expect, test, type Page } from "@playwright/test";
import {
  createGuestCharacterAndEnterWorld,
  quitToCharacterSelect,
} from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  centerTileInView,
  getGameState,
  waitForMap,
  waitForNoMapLoading,
  waitForPlayerIdle,
  waitForPlayerTile,
  waitForWarpMode,
} from "./helpers/state";

type PlayerPositionEvent = {
  x: number;
  y: number;
  mapId: number;
  direction: string;
  isMoving: boolean;
};

async function waitForNextPlayerPositionEvent(page: Page) {
  return page.evaluate(() =>
    window.__capturequestTest?.waitForEvent(
      "cq:playerPositionChanged",
      10_000,
    ),
  ) as Promise<PlayerPositionEvent>;
}

async function instantWarpTo(page: Page, x: number, y: number) {
  await page.getByRole("button", { name: "Instant Warp" }).click();
  await waitForWarpMode(page, true);
  await clickTile(page, x, y);
  await waitForWarpMode(page, false);
  await waitForPlayerTile(page, x, y);
}

test("instant warp updates the keyboard movement origin immediately", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForNoMapLoading(page);
  await waitForPlayerIdle(page);

  await instantWarpTo(page, 6, 7);

  const nextPosition = waitForNextPlayerPositionEvent(page);
  await pressMovement(page, "left");
  expect(await nextPosition).toMatchObject({ x: 5, y: 7 });
  await waitForPlayerTile(page, 5, 7);

  await page.waitForTimeout(750);
  const state = await getGameState(page);
  expect(state.player.x).toBe(5);
  expect(state.player.y).toBe(7);
  expect(state.map.name).toBe("REDS_HOUSE_1F");
  expect(state.worldInput.frozen).toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("instant warp updates the click-path origin immediately", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForNoMapLoading(page);
  await waitForPlayerIdle(page);

  await instantWarpTo(page, 6, 7);

  const nextPosition = waitForNextPlayerPositionEvent(page);
  await clickTile(page, 5, 7);
  expect(await nextPosition).toMatchObject({ x: 5, y: 7 });
  await waitForPlayerTile(page, 5, 7);

  await page.waitForTimeout(750);
  const state = await getGameState(page);
  expect(state.player.x).toBe(5);
  expect(state.player.y).toBe(7);
  expect(state.map.name).toBe("REDS_HOUSE_1F");
  expect(state.worldInput.frozen).toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("instant warp from map view to a far overworld tile keeps the new movement origin", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await page.setViewportSize({ width: 1900, height: 1000 });
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_field_move_surf_ready");
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForNoMapLoading(page);
  await waitForPlayerTile(page, 3, 17);
  await waitForPlayerIdle(page);

  await page.getByRole("button", { name: "View Map" }).click();
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.ui.isCameraFollowEnabled;
      },
      { timeout: 10_000 },
    )
    .toBe(false);
  await centerTileInView(page, 190, -81);

  await instantWarpTo(page, 190, -81);

  let nextPosition = waitForNextPlayerPositionEvent(page);
  await pressMovement(page, "left");
  expect(await nextPosition).toMatchObject({ x: 189, y: -81 });
  await waitForPlayerTile(page, 189, -81);

  await page.waitForTimeout(750);
  let state = await getGameState(page);
  expect(state.player.x).toBe(189);
  expect(state.player.y).toBe(-81);
  expect(state.player.x).not.toBe(3);
  expect(state.player.y).not.toBe(17);
  expect(state.worldInput.frozen).toBe(false);

  nextPosition = waitForNextPlayerPositionEvent(page);
  await clickTile(page, 190, -81);
  expect(await nextPosition).toMatchObject({ x: 190, y: -81 });
  await waitForPlayerTile(page, 190, -81);

  await page.waitForTimeout(750);
  state = await getGameState(page);
  expect(state.player.x).toBe(190);
  expect(state.player.y).toBe(-81);
  expect(state.map.name).toMatch(/Kanto|Unified Overworld/);
  expect(state.worldInput.frozen).toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
