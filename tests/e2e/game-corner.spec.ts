import { expect, test, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import {
  clickTile,
  dismissDialogue,
  pressMovement,
  pressSpace,
  waitForDialogueText,
} from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  getGameState,
  waitForInventoryItem,
  waitForMap,
  waitForPartyPokemon,
  waitForPCPokemon,
  waitForPlayerTile,
  waitForPlayerIdle,
} from "./helpers/state";

async function advanceCoinClerkDialogue(page: Page, untilChoice = false) {
  for (let i = 0; i < 50; i++) {
    const state = await getGameState(page);
    if (untilChoice && state.dialogue.isChoicePending) return;
    if (!untilChoice && !state.worldInput.frozen && !state.dialogue.isOpen) return;
    if (state.dialogue.isOpen && !state.dialogue.isChoicePending) await pressSpace(page);
    else await page.waitForTimeout(150);
  }
  throw new Error(`Coin clerk dialogue did not reach ${untilChoice ? "choice" : "completion"}`);
}

async function clickPrizeChoice(page: Page, text: string) {
  await expect.poll(() => page.evaluate(label => window.__capturequestTest?.overlayTextToViewport(label) ?? null, text)).not.toBeNull();
  const point = await page.evaluate(label => window.__capturequestTest!.overlayTextToViewport(label), text);
  expect(point).toBeTruthy();
  await page.mouse.click(point!.x, point!.y);
}

async function pressSpaceUntilInventoryItem(
  page: Page,
  matcher: string | number | RegExp,
  maxPresses = 20,
) {
  for (let i = 0; i < maxPresses; i += 1) {
    const state = await getGameState(page);
    const hasItem = state.inventory.items.some((item) => {
      if (typeof matcher === "number") return item.itemId === matcher;
      if (typeof matcher === "string") {
        return item.name === matcher || item.shortName === matcher;
      }
      return matcher.test(item.name) || matcher.test(item.shortName);
    });
    if (hasItem) return;

    await pressSpace(page);
    await page.waitForTimeout(250);
  }

  await waitForInventoryItem(page, matcher);
}

// Source bg-events are at y=2; the floor immediately below is y=3.
// Wait for each actual tile step so taps are not coalesced by movement.
async function standAtPrizeWindow(page: Page, windowX: number) {
  await dismissDialogue(page, 6);
  await waitForPlayerIdle(page);
  await waitForPlayerTile(page, 2, 4);
  for (let x = 3; x <= windowX; x += 1) {
    await pressMovement(page, "right");
    await waitForPlayerTile(page, x, 4);
    await waitForPlayerIdle(page);
  }
  await pressMovement(page, "up");
  await waitForPlayerTile(page, windowX, 3);
  await waitForPlayerIdle(page);
}

test("Game Corner clerk can be reached through the counter", async ({ page }) => {
  test.setTimeout(90_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_game_corner_buy_coins_ready");
  await waitForMap(page, /Game Corner|GAME_CORNER/);
  await waitForPlayerTile(page, 5, 8);

  await pressSpace(page);
  await waitForDialogueText(page, /ROCKET\s+GAME CORNER|game coins/i, 4);
  await dismissDialogue(page, 12);

  await pressMovement(page, "down");
  await waitForPlayerTile(page, 5, 9);

  await clickTile(page, 5, 6);
  await waitForPlayerTile(page, 5, 8);
  await waitForDialogueText(page, /ROCKET\s+GAME CORNER|game coins/i, 4);
  await dismissDialogue(page, 12);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Coin Case acquisition enables buying coins and playing Game Corner slots", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);

  await jumpToScenario(page, "debug_celadon_diner_coin_case_ready");
  await waitForMap(page, /Celadon Diner|CELADON_DINER/);
  await waitForPlayerTile(page, 1, 1);
  await pressSpaceUntilInventoryItem(page, 69);
  await dismissDialogue(page, 10);

  await jumpToScenario(page, "debug_game_corner_buy_coins_ready");
  await waitForMap(page, /Game Corner|GAME_CORNER/);

  await waitForPlayerIdle(page);
  await pressSpace(page);
  await advanceCoinClerkDialogue(page, true);
  await page.getByTestId("dialogue-choice-no").click();
  await advanceCoinClerkDialogue(page);
  await expect.poll(async () => (await getGameState(page)).inventory).toMatchObject({money:1000,coins:10});

  await pressSpace(page);
  await advanceCoinClerkDialogue(page, true);
  await page.getByTestId("dialogue-choice-yes").click();
  await advanceCoinClerkDialogue(page);
  await expect.poll(async () => (await getGameState(page)).inventory).toMatchObject({money:0,coins:60});

  await jumpToScenario(page, "debug_game_corner_slots_ready");
  await waitForMap(page, /Game Corner|GAME_CORNER/);
  await waitForPlayerTile(page, 17, 15);
  await dismissDialogue(page, 6);
  await clickTile(page, 18, 15);
  await expect(page.getByTestId("slot-machine")).toBeVisible({
    timeout: 15_000,
  });
  await expect(page.getByTestId("slot-machine-coins")).toContainText(
    "COINS: 10",
    { timeout: 15_000 },
  );

  await page.getByTestId("slot-machine-bet-3").click();
  await page.getByTestId("slot-machine-spin").click();
  await expect(page.getByTestId("slot-machine-spin")).toBeDisabled();
  await expect(page.getByTestId("slot-machine-spin")).toBeEnabled({
    timeout: 10_000,
  });
  await page.getByTestId("slot-machine-quit").click();
  await expect(page.getByTestId("slot-machine")).toBeHidden({
    timeout: 10_000,
  });

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Game Corner prize exchange requires Coin Case and buys rewards", async ({page}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page,"game_corner_prize_list_no_coin_case");
  await waitForMap(page,/Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);
  await standAtPrizeWindow(page,2);
  await pressSpace(page);
  await expect.poll(async()=>(await getGameState(page)).messages.map(message=>message.text).join(" ")).toContain("COIN CASE");
  await expect.poll(async()=>(await getGameState(page)).worldInput.frozen).toBe(false);

  await jumpToScenario(page,"game_corner_prize_list_with_coin_case");
  await waitForMap(page,/Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);
  await standAtPrizeWindow(page,6);
  await pressSpace(page);
  await clickPrizeChoice(page,"CANCEL");
  await expect.poll(async()=>(await getGameState(page)).worldInput.frozen).toBe(false);
  expect((await getGameState(page)).inventory.coins).toBe(9999);
  await pressSpace(page);
  await clickPrizeChoice(page,"TM23 Dragon Rage - 3300C");
  await waitForInventoryItem(page,/TM23/i);
  await expect.poll(async()=>(await getGameState(page)).inventory.coins).toBe(6699);
  await waitForDialogueText(page,/Here you go/i);
  await dismissDialogue(page,6);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Game Corner prize Pokemon go to party or PC when party is full",async({page})=>{
  test.setTimeout(150_000);
  const errors=collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page,"game_corner_prize_list_with_coin_case");
  await waitForMap(page,/Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);
  await standAtPrizeWindow(page,2);
  await pressSpace(page);
  await clickPrizeChoice(page,"ABRA - 180C");
  await waitForPartyPokemon(page,63);
  await expect.poll(async()=>(await getGameState(page)).inventory.coins).toBe(9819);
  expect((await getGameState(page)).pokemon.party).toEqual(expect.arrayContaining([expect.objectContaining({id:63,name:"ABRA",level:9})]));
  await dismissDialogue(page,6);

  await jumpToScenario(page,"debug_game_corner_prize_full_party_ready");
  await waitForMap(page,/Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);
  await standAtPrizeWindow(page,2);
  await pressSpace(page);
  await clickPrizeChoice(page,"ABRA - 180C");
  await waitForPCPokemon(page,63);
  await expect.poll(async()=>(await getGameState(page)).inventory.coins).toBe(9819);
  expect((await getGameState(page)).pokemon.party.map(pokemon=>pokemon.id)).toEqual([1,4,7,25,16,19]);
  expect((await getGameState(page)).pokemon.pc.boxPokemon).toEqual(expect.arrayContaining([expect.objectContaining({id:63,name:"ABRA",level:9,boxSlot:1})]));
  expect((await getGameState(page)).pokemon.pc.isOpen).toBe(false);
  await dismissDialogue(page,6);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
