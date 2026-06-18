import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  waitForInventoryItem,
  waitForInventoryOpen,
  waitForMap,
  waitForMessage,
} from "./helpers/state";

test("party-use medicine attaches to the cursor from inventory", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_inventory_potion_ready");
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForInventoryItem(page, "POTION");

  await page.getByRole("button", { name: "Inventory" }).click();
  await waitForInventoryOpen(page, true);
  await page.getByTestId("inventory-item-potion").click();

  await expect(page.getByTestId("inventory-cursor-item")).toBeVisible();
  await expect(page.getByTestId("inventory-cursor-item")).toContainText("Potion", {
    ignoreCase: true,
  });
  await expect(page.locator("[data-cq-party-item-target='true']").first()).toBeVisible();

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Coin Case clicked from inventory reports current coins", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_game_corner_buy_coins_ready");
  await waitForMap(page, /Game Corner|GAME_CORNER/);
  await waitForInventoryItem(page, 69);

  await page.getByRole("button", { name: "Inventory" }).click();
  await waitForInventoryOpen(page, true);
  await page.getByTestId("inventory-item-coin-case").click();

  await expect(page.getByTestId("inventory-cursor-item")).toHaveCount(0);
  await waitForMessage(page, /You have 10 coins\./i);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("battle-only and non-usable items do not attach to the cursor outside battle", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_inventory_blocked_items_ready");
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForInventoryItem(page, "X_ATTACK");
  await waitForInventoryItem(page, "NUGGET");

  await page.getByRole("button", { name: "Inventory" }).click();
  await waitForInventoryOpen(page, true);

  await page.getByTestId("inventory-item-x-attack").click();
  await expect(page.getByTestId("inventory-cursor-item")).toHaveCount(0);
  await expect(page.locator("[data-cq-party-item-target='true']")).toHaveCount(0);
  await waitForMessage(page, /outside of battle/i);

  await page.getByTestId("inventory-item-nugget").click();
  await expect(page.getByTestId("inventory-cursor-item")).toHaveCount(0);
  await expect(page.locator("[data-cq-party-item-target='true']")).toHaveCount(0);
  await waitForMessage(page, /can't be used like that/i);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
