import * as OpCodes from "../../src/net/generated/opcodes";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
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

  await page.getByRole("button", { name: "Bag", exact: true }).click();
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

  // The Bag toggle remains accessible when the item list overlaps Done.
  await page.getByRole("button", { name: "Bag", exact: true }).click();
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


test("Bicycle duplicate and lost replies reconcile without a toggle retry or historical rewind", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const faults = await inventoryCommandFaults(page, OpCodes.BicycleStateRequest,
    OpCodes.BicycleStateResponse, true, request => "wantsRiding" in request);
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "bike_shop_already_has_bicycle");
  await waitForInventoryItem(page, 6);
  await page.getByRole("button", { name: "Warp Home" }).click();
  await waitForPlayerTile(page, 9, 4, 30_000);
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await waitForInventoryOpen(page, true);
  const bicycle = page.getByTestId("inventory-item-bicycle");
  const riding = async () => {
    const state = await getGameState(page);
    return state.player.isCycling && state.audio.isBicycleActive;
  };
  await bicycle.click();
  await expect.poll(riding, { timeout: 20_000 }).toBe(true);
  expect(faults.requests).toBe(1);
  expect(faults.successes).toBe(1);
  expect(faults.rejections).toBe(1);
  await bicycle.click();
  await expect.poll(riding, { timeout: 20_000 }).toBe(false);
  expect(faults.requests).toBe(2);
  expect(faults.successes).toBe(2);
  expect(faults.rejections).toBe(2);
  faults.deliverReply(0);
  await expect.poll(riding).toBe(false);

  await bicycle.click();
  await expect.poll(riding, { timeout: 20_000 }).toBe(true);
  expect(faults.requests).toBe(3);
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await quitToCharacterSelect(page);
  await enterWorld(page, character);
  faults.deliverReply(2);
  await expect.poll(riding).toBe(false);
  errors.assertNoSevereErrors();
});
