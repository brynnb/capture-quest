import { expect, test, type Page } from "@playwright/test";
import { waitForBattleOpen } from "./helpers/battle";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { dismissDialogue, pressMovement, pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  getGameState,
  waitForActorVisible,
  waitForInventoryItem,
  waitForMap,
  waitForPlayerIdle,
  waitForPlayerTile,
} from "./helpers/state";

async function pressSpaceUntilChoice(page: Page, maxPresses = 16) {
  if (await advanceDialogueTowardChoice(page, maxPresses)) return;

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.dialogue.isOpen && state.dialogue.isChoicePending;
      },
      { timeout: 5_000 },
    )
    .toBe(true);
}

async function advanceDialogueTowardChoice(page: Page, maxPresses = 16) {
  for (let i = 0; i < maxPresses; i += 1) {
    const state = await getGameState(page);
    if (state.dialogue.isOpen && state.dialogue.isChoicePending) return true;

    if (!state.dialogue.isOpen) {
      await pressSpace(page);
      await page.waitForTimeout(300);
      continue;
    }

    if (state.dialogue.isOpen && state.dialogue.isTyping) {
      await expect
        .poll(
          async () => {
            const next = await getGameState(page);
            return (
              !next.dialogue.isOpen ||
              next.dialogue.isChoicePending ||
              !next.dialogue.isTyping
            );
          },
          { timeout: 6_000 },
        )
        .toBe(true);
      continue;
    }

    await pressSpace(page);
    await page.waitForTimeout(250);
  }

  return false;
}

async function clickDialogueUntilChoice(page: Page, maxClicks = 8) {
  for (let i = 0; i < maxClicks; i += 1) {
    const state = await getGameState(page);
    if (state.dialogue.isOpen && state.dialogue.isChoicePending) return;

    if (state.dialogue.isOpen && state.dialogue.isTyping) {
      await expect
        .poll(
          async () => {
            const next = await getGameState(page);
            return !next.dialogue.isTyping;
          },
          { timeout: 6_000 },
        )
        .toBe(true);
    }

    await page.getByTestId("pokemon-dialogue-box").click();
    await page.waitForTimeout(250);
  }

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.dialogue.isOpen && state.dialogue.isChoicePending;
      },
      { timeout: 5_000 },
    )
    .toBe(true);
}

test("Space talks to Bike Shop clerk and arrows change Yes/No choice", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "bike_shop_no_voucher_yes");
  await waitForMap(page, "BIKE_SHOP");
  await waitForPlayerTile(page, 6, 4);

  await pressMovement(page, "up");
  await waitForPlayerTile(page, 6, 4);
  await waitForPlayerIdle(page);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.player.direction;
      },
      { timeout: 5_000 },
    )
    .toBe("UP");
  await page.waitForTimeout(250);
  await pressSpace(page);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.dialogue.isOpen && /BIKE SHOP/i.test(state.dialogue.text);
      },
      { timeout: 10_000 },
    )
    .toBe(true);

  await clickDialogueUntilChoice(page);
  let state = await getGameState(page);
  expect(state.dialogue.selectedChoice).toBe("yes");

  await page.keyboard.press("ArrowRight");
  state = await getGameState(page);
  expect(state.dialogue.selectedChoice).toBe("no");

  await pressSpace(page);
  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.dialogue.isOpen && !next.dialogue.isChoicePending;
      },
      { timeout: 15_000 },
    )
    .toBe(true);
  expect((await getGameState(page)).inventory.items.some((item) => item.itemId === 6)).toBe(
    false,
  );
  await dismissDialogue(page, 8);

  await jumpToScenario(page, "bike_shop_no_voucher_yes");
  await waitForMap(page, "BIKE_SHOP");
  await waitForPlayerTile(page, 6, 4);
  await pressMovement(page, "up");
  await waitForPlayerTile(page, 6, 4);
  await waitForPlayerIdle(page);
  await page.waitForTimeout(250);
  await pressSpaceUntilChoice(page);
  await pressSpace(page);
  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.dialogue.isOpen || next.messages.some((message) => message.text.length > 0);
      },
      { timeout: 15_000 },
    )
    .toBe(true);
  await dismissDialogue(page, 8);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Old Man tutorial starts an item-only guaranteed catch battle", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "viridian_city_old_man_catch_demo_start");
  await waitForMap(page, /Viridian City|VIRIDIAN_CITY|Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 8, -67);
  await waitForPlayerIdle(page);
  await waitForActorVisible(
    page,
    (actor) => actor.text === "TEXT_VIRIDIANCITY_OLD_MAN",
  );
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.player.direction;
      },
      { timeout: 5_000 },
    )
    .toBe("LEFT");

  await pressSpace(page);
  await pressSpaceUntilChoice(page, 20);
  await page.keyboard.press("ArrowRight");
  expect((await getGameState(page)).dialogue.selectedChoice).toBe("no");
  await pressSpace(page);

  for (let i = 0; i < 20; i += 1) {
    const state = await getGameState(page);
    if (state.battle.isOpen) break;
    await pressSpace(page);
    await page.waitForTimeout(150);
  }
  await waitForBattleOpen(page);
  await waitForInventoryItem(page, 4);

  const state = await getGameState(page);
  expect(state.battle.battleType).toBe("wild");
  expect(state.battle.enemyPokemonName?.toUpperCase()).toContain("WEEDLE");
  expect(state.battle.allowedActions).toEqual(["item"]);
  expect(state.battle.guaranteedCatch).toBe(true);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Game Corner prize-room background vendors route to the prize menu", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await page.evaluate(() => {
    window.addEventListener("gameCornerPrizeList", (event) => {
      (window as unknown as Record<string, unknown>).__lastGameCornerPrizeList =
        (event as CustomEvent).detail;
    });
  });

  await jumpToScenario(page, "game_corner_prize_list_with_coin_case");
  await waitForMap(page, /Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);
  await page.evaluate(() => {
    window.__capturequestTest?.requestGameCornerPrizeList();
  });

  await expect
    .poll(
      async () =>
        page.evaluate(
          () =>
            (window as unknown as {
              __lastGameCornerPrizeList?: { prizes?: unknown[]; success?: boolean };
            }).__lastGameCornerPrizeList,
        ),
      { timeout: 30_000 },
    )
    .toMatchObject({
      success: true,
    });

  const prizeCount = await page.evaluate(
    () =>
      (window as unknown as {
        __lastGameCornerPrizeList?: { prizes?: unknown[] };
      }).__lastGameCornerPrizeList?.prizes?.length ?? 0,
  );
  expect(prizeCount).toBeGreaterThan(0);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
