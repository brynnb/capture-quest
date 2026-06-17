import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld } from "./helpers/auth";
import { waitForBattleOpen } from "./helpers/battle";
import { collectPageErrors } from "./helpers/errors";
import { pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  getGameState,
  waitForMap,
  waitForNoMapLoading,
  waitForTestBridge,
  waitForWarpMode,
} from "./helpers/state";

async function reloadAndWaitForBridge(page: import("@playwright/test").Page) {
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitForTestBridge(page);
}

test("reload clears stale map and warp UI modes", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await waitForNoMapLoading(page);

  await page.getByRole("button", { name: "Instant Warp" }).click();
  await waitForWarpMode(page, true);

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

  await reloadAndWaitForBridge(page);
  const state = await getGameState(page);
  expect(state.ui.isWarpMode).toBe(false);
  expect(state.ui.isCameraFollowEnabled).toBe(true);

  errors.assertNoSevereErrors();
});

test("reload does not preserve stale dialogue mode", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "viridian_mart_oaks_parcel");
  await waitForMap(page, /Viridian Mart|VIRIDIAN_MART/);

  await pressSpace(page);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.dialogue.isOpen;
      },
      { timeout: 30_000 },
    )
    .toBe(true);

  await reloadAndWaitForBridge(page);
  expect((await getGameState(page)).dialogue.isOpen).toBe(false);

  errors.assertNoSevereErrors();
});

test("reload does not preserve stale battle or cutscene mode", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "active_battle_fixture_wild");
  await waitForBattleOpen(page);

  await reloadAndWaitForBridge(page);
  const state = await getGameState(page);
  expect(state.battle.isOpen).toBe(false);
  expect(state.debug.isCutscenePlaying).toBe(false);
  errors.assertNoSevereErrors();
});
