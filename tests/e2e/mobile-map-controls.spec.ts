import {
  devices,
  expect,
  test,
  type CDPSession,
  type Page,
} from "@playwright/test";
import {
  createGuestCharacterAndEnterWorld,
} from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  centerTileInView,
  getGameState,
  tileToViewport,
  waitForMap,
  waitForNoMapLoading,
  waitForPlayerTile,
  waitForWarpMode,
} from "./helpers/state";

test.use({ ...devices["Pixel 5"] });

type TouchPoint = { id: number; x: number; y: number };

async function dispatchTouch(
  client: CDPSession,
  type: "touchStart" | "touchMove" | "touchEnd",
  points: TouchPoint[],
) {
  await client.send("Input.dispatchTouchEvent", {
    type,
    touchPoints: points.map((point) => ({
      id: point.id,
      x: point.x,
      y: point.y,
      radiusX: 2,
      radiusY: 2,
      force: 1,
    })),
  });
}

async function clickMobileMenuAction(page: Page, name: string) {
  const action = page.getByRole("button", { name });
  if (!(await action.isVisible())) {
    await page.getByRole("button", { name: "Open game menu" }).click();
  }
  await expect(action).toBeVisible();
  await action.click();
}

test("mobile map view supports touch pan, pinch zoom, and confirm-to-warp", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  // Enter the game at desktop dimensions to keep this test independent of the
  // character-select layout. The browser remains touch-capable throughout.
  await page.setViewportSize({ width: 1280, height: 900 });
  await createGuestCharacterAndEnterWorld(page, { forceClicks: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await jumpToScenario(page, "debug_field_move_surf_ready");
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForNoMapLoading(page);
  await waitForPlayerTile(page, 3, 17);

  await clickMobileMenuAction(page, "View Map");
  await expect
    .poll(async () => (await getGameState(page)).ui.isCameraFollowEnabled)
    .toBe(false);

  const client = await page.context().newCDPSession(page);
  const canvas = page.locator("#phaser-game-container canvas");
  const box = await canvas.boundingBox();
  expect(box).not.toBeNull();
  const center = {
    x: box!.x + box!.width / 2,
    y: box!.y + box!.height / 2,
  };

  const beforePan = await getGameState(page);
  await dispatchTouch(client, "touchStart", [{ id: 1, ...center }]);
  for (const amount of [20, 40, 60, 80]) {
    await dispatchTouch(client, "touchMove", [
      { id: 1, x: center.x + amount, y: center.y + amount * 0.6 },
    ]);
  }
  await dispatchTouch(client, "touchEnd", []);
  await expect
    .poll(async () => {
      const state = await getGameState(page);
      return Math.hypot(
        (state.ui.cameraScrollX ?? 0) - (beforePan.ui.cameraScrollX ?? 0),
        (state.ui.cameraScrollY ?? 0) - (beforePan.ui.cameraScrollY ?? 0),
      );
    })
    .toBeGreaterThan(40);

  const zoomBeforePinch = (await getGameState(page)).ui.cameraZoom ?? 0;
  await dispatchTouch(client, "touchStart", [
    { id: 1, x: center.x - 34, y: center.y },
    { id: 2, x: center.x + 34, y: center.y },
  ]);
  for (const distance of [50, 68, 92]) {
    await dispatchTouch(client, "touchMove", [
      { id: 1, x: center.x - distance, y: center.y },
      { id: 2, x: center.x + distance, y: center.y },
    ]);
  }
  await dispatchTouch(client, "touchEnd", []);
  await expect
    .poll(async () => (await getGameState(page)).ui.cameraZoom ?? 0)
    .toBeGreaterThan(zoomBeforePinch * 1.4);

  await page.mouse.move(center.x, center.y);
  for (let index = 0; index < 16; index += 1) {
    await page.mouse.wheel(0, 100);
  }
  await expect
    .poll(async () => (await getGameState(page)).ui.cameraZoom)
    .toBe(0.1);

  await clickMobileMenuAction(page, "Instant Warp");
  await waitForWarpMode(page, true);
  await centerTileInView(page, 190, -81);
  const targetPoint = await tileToViewport(page, 190, -81);
  await page.touchscreen.tap(targetPoint.x, targetPoint.y);

  await expect
    .poll(async () => (await getGameState(page)).ui.pendingInstantWarpTarget)
    .toEqual({ mapId: 9999, x: 190, y: -81 });
  let state = await getGameState(page);
  expect(state.player).toMatchObject({ x: 3, y: 17 });
  expect(state.ui.instantWarpTargetVisible).toBe(true);
  expect(state.ui.cameraZoom).toBe(0.1);
  await expect(page.getByTestId("instant-warp-selection")).toContainText(
    "190, -81",
  );

  await page.getByRole("button", { name: "Confirm Warp" }).click();
  await waitForWarpMode(page, false);
  await waitForPlayerTile(page, 190, -81);
  state = await getGameState(page);
  expect(state.ui.pendingInstantWarpTarget).toBeNull();
  expect(state.ui.instantWarpTargetVisible).toBe(false);
  expect(state.ui.isCameraFollowEnabled).toBe(true);

  await clickMobileMenuAction(page, "Quit");
  await expect(
    page.getByRole("heading", { name: "SELECT A CHARACTER" }),
  ).toBeVisible();
  errors.assertNoSevereErrors();
});
