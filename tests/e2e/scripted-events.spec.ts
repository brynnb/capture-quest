import { expect, test, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { dismissDialogue, pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForPlayerTile } from "./helpers/state";

async function pressSpaceUntilVisible(
  page: Page,
  text: string,
  maxPresses = 12,
) {
  const target = page.getByText(text);
  for (let i = 0; i < maxPresses; i += 1) {
    if (await target.isVisible().catch(() => false)) return;
    await pressSpace(page);
    await page.waitForTimeout(150);
  }
  await expect(target).toBeVisible({ timeout: 5_000 });
}

test("scripted NPC events can be triggered from a browser fixture", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "viridian_mart_oaks_parcel");

  await waitForMap(page, /Viridian Mart|VIRIDIAN_MART/);
  await waitForPlayerTile(page, 3, 6);

  await pressSpaceUntilVisible(page, "Received OAK's PARCEL!");
  await dismissDialogue(page, 6);

  const state = await getGameState(page);
  expect(state.dialogue.isOpen).toBe(false);
  expect(state.battle.isOpen).toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
