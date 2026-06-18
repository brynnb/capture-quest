import { expect, test } from "@playwright/test";
import { waitForBattleOpen } from "./helpers/battle";
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
} from "./helpers/state";

test("Old Rod used from inventory while facing water starts a fishing battle", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_inventory_old_rod_water_ready");
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 3, 17);
  await waitForInventoryItem(page, "OLD_ROD");

  await page.getByRole("button", { name: "Inventory" }).click();
  await waitForInventoryOpen(page, true);
  await page.getByTestId("inventory-item-old-rod").click();

  await waitForMessage(page, /Oh! A bite!/i);
  await waitForBattleOpen(page);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Old Rod used from inventory while facing land does not start a battle", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_inventory_old_rod_land_ready");
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 9, 4);
  await waitForInventoryItem(page, "OLD_ROD");

  await page.getByRole("button", { name: "Inventory" }).click();
  await waitForInventoryOpen(page, true);
  await page.getByTestId("inventory-item-old-rod").click();

  await waitForMessage(page, /can't fish here/i);
  await expect(page.getByTestId("inventory-cursor-item")).toHaveCount(0);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.battle.isOpen;
      },
      { timeout: 5_000 },
    )
    .toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
