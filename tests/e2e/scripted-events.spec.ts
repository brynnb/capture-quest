import { expect, test, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import {
  dismissDialogue,
  pressMovement,
  pressSpace,
} from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  getGameState,
  waitForMap,
  waitForNoMapLoading,
  waitForPlayerIdle,
  waitForPlayerTile,
} from "./helpers/state";

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

async function finishCutsceneDialogue(page: Page, maxPresses = 40) {
  for (let i = 0; i < maxPresses; i += 1) {
    const state = await getGameState(page);
    if (
      !state.worldInput.frozen &&
      !state.dialogue.isOpen &&
      !state.dialogue.isChoicePending
    ) {
      return;
    }

    if (state.dialogue.isOpen || state.dialogue.isChoicePending) {
      await pressSpace(page);
    } else {
      await page.waitForTimeout(200);
    }
  }

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.worldInput.frozen || state.dialogue.isOpen;
      },
      { timeout: 10_000 },
    )
    .toBe(false);
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

test("scripted player movement updates the real player tile after Oak Lab intro", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "oak_lab_choose_starter_intro");
  await waitForMap(page, "OAKS_LAB");
  await waitForNoMapLoading(page);

  await finishCutsceneDialogue(page);
  await waitForPlayerIdle(page);
  await waitForPlayerTile(page, 5, 3);

  const beforeMove = await getGameState(page);
  await pressMovement(page, "down");
  await waitForPlayerTile(page, 5, 4);

  const afterMove = await getGameState(page);
  expect(afterMove.map.id).toBe(beforeMove.map.id);
  expect(afterMove.map.name).toBe("OAKS_LAB");
  expect(afterMove.worldInput.frozen).toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
