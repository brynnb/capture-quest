import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  getGameState,
  waitForInventoryItem,
  waitForInventoryOpen,
  waitForMap,
  waitForMessage,
  waitForPlayerTile,
  waitForWarps,
} from "./helpers/state";

test("Bicycle can be toggled from inventory and pauses indoors", async ({
  page,
}) => {
  test.setTimeout(180_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "bike_shop_already_has_bicycle");
  await waitForInventoryItem(page, 6);

  await page.getByRole("button", { name: "Warp Home" }).click();
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 9, 4, 30_000);
  await waitForWarps(page);

  await page.getByRole("button", { name: "Inventory" }).click();
  await waitForInventoryOpen(page, true);
  await page.getByTestId("inventory-item-bicycle").click();
  await waitForMessage(page, /Bicycle/i);

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.player.isCycling && state.audio.isBicycleActive;
      },
      { timeout: 15_000 },
    )
    .toBe(true);

  await page.getByTestId("inventory-item-bicycle").click();
  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.player.isCycling || next.audio.isBicycleActive;
      },
      { timeout: 15_000 },
    )
    .toBe(false);

  await page.getByTestId("inventory-item-bicycle").click();
  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.player.isCycling && next.audio.isBicycleActive;
      },
      { timeout: 15_000 },
    )
    .toBe(true);

  await page.getByRole("button", { name: "Done" }).click();
  await waitForInventoryOpen(page, false);

  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  await waitForMap(page, "REDS_HOUSE_1F");

  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.player.isCycling || next.audio.isBicycleActive;
      },
      { timeout: 15_000 },
    )
    .toBe(false);

  await page.getByRole("button", { name: "Warp Home" }).click();
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 9, 4, 30_000);

  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.player.isCycling && next.audio.isBicycleActive;
      },
      { timeout: 15_000 },
    )
    .toBe(true);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
