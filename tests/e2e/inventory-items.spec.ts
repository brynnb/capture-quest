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
} from "./helpers/state";

test("Repel inventory use consumes once and survives leaving the world", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const characterName = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "repel_use_consumes_item");
  await waitForInventoryItem(page, 30);
  const repelInstanceId = (await getGameState(page)).inventory.items.find(item => item.itemId === 30)!.instanceId;
  // Local fixtures top inventory up on re-entry. Track the original stack so
  // another legitimate stack cannot mask identity or consumption regressions.
  const repelRow = page.locator(`[data-testid="inventory-item-repel"][data-instance-id="${repelInstanceId}"]`);
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await waitForInventoryOpen(page, true);
  await repelRow.click();
  await waitForMessage(page, /REPEL's effect started!/i);
  const quantity = async () =>
    (await getGameState(page)).inventory.items.find(item => item.instanceId === repelInstanceId)?.quantity;
  await expect.poll(quantity).toBe(1);
  await repelRow.click();
  await waitForMessage(page, /A repel is already active!/i);
  expect(await quantity()).toBe(1);
  await quitToCharacterSelect(page);
  await enterWorld(page, characterName);
  const bag = page.getByRole("button", { name: "Bag", exact: true });
  if (!(await bag.isVisible())) {
    await page.getByRole("button", { name: "Open game menu" }).click();
  }
  await bag.click();
  await waitForInventoryOpen(page, true);
  const activeMessages = async () =>
    (await getGameState(page)).messages.filter(message => message.text.includes("A repel is already active!")).length;
  const previousActiveMessages = await activeMessages();
  await repelRow.click();
  await expect.poll(activeMessages).toBeGreaterThan(previousActiveMessages);
  expect(await quantity()).toBe(1);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("party-use medicine attaches to the cursor from inventory", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_inventory_potion_ready");
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForInventoryItem(page, "POTION");

  await page.getByRole("button", { name: "Bag", exact: true }).click();
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

  await page.getByRole("button", { name: "Bag", exact: true }).click();
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

  await page.getByRole("button", { name: "Bag", exact: true }).click();
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
