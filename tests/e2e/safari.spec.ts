import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement, pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForPlayerTile, waitForPlayerIdle } from "./helpers/state";

test("Safari gate entry commits the visit and shows the zone", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_zone_gate_entry_offer_yes");
  await waitForMap(page, "SAFARI_ZONE_GATE");
  await waitForPlayerTile(page, 4, 2);
  // A coordinate script triggers on stepping onto the tile, not fixture placement.
  await pressMovement(page, "down");
  await waitForPlayerTile(page, 4, 3);
  await waitForPlayerIdle(page);
  await pressMovement(page, "up");
  for (let i = 0; i < 30; i += 1) {
    const state = await getGameState(page);
    if (state.map.id === 220 && !state.dialogue.isOpen) break;
    await pressSpace(page);
    await page.waitForTimeout(150);
  }
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  await waitForPlayerTile(page, 14, 25);
  await page.screenshot({ path: test.info().outputPath("safari-entry.png") });
  errors.assertNoSevereErrors();
});

test("Safari battle run resolves through the real action menu", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_battle_run");
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  await expect(page.getByTestId("battle-action-safari-ball")).toBeVisible();
  await page.screenshot({ path: test.info().outputPath("safari-battle.png") });
  await page.getByTestId("battle-action-run").click();
  for (let i = 0; i < 20; i += 1) {
    if (!(await getGameState(page)).battle.isOpen) break;
    await pressSpace(page);
    await page.waitForTimeout(150);
  }
  await expect.poll(async () => (await getGameState(page)).battle.isOpen).toBe(false);
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  errors.assertNoSevereErrors();
});

test("Safari step exhaustion returns the player to the gate", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_step_expiry");
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  await waitForPlayerTile(page, 14, 24);
  await pressMovement(page, "up");
  await expect.poll(async () => (await getGameState(page)).dialogue.text, { timeout: 15_000 }).toMatch(/SAFARI GAME is over/);
  await page.screenshot({ path: test.info().outputPath("safari-expiry.png") });
  await pressSpace(page, 4);
  await waitForMap(page, "SAFARI_ZONE_GATE");
  await waitForPlayerTile(page, 3, 4);
  errors.assertNoSevereErrors();
});
