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
} from "./helpers/state";

type CoinBalanceEvent = {
  success?: boolean;
  coins: number;
  money?: number;
  message?: string;
  error?: string;
};

type SlotResultEvent = {
  success: boolean;
  bet: number;
  coins: number;
  payout: number;
  reelPositions?: number[];
  error?: string;
};

type Prize = {
  id: number;
  type: string;
  name: string;
  coinCost: number;
};

type PrizeListEvent = {
  success: boolean;
  coins: number;
  prizes: Prize[];
  error?: string;
};

type PrizeBuyEvent = {
  success: boolean;
  coins: number;
  prizeName?: string;
  prizeLevel?: number;
  addedToParty?: boolean;
  pcBox?: number;
  pcSlot?: number;
  message?: string;
  error?: string;
};

async function withNextWindowEvent<T>(
  page: Page,
  eventName: string,
  trigger: () => Promise<void>,
  timeoutMs = 20_000,
): Promise<T> {
  const eventPromise = page.evaluate(
    ({ eventName, timeoutMs }) =>
      new Promise<unknown>((resolve, reject) => {
        const timeout = window.setTimeout(() => {
          window.removeEventListener(eventName, handler);
          reject(new Error(`Timed out waiting for ${eventName}`));
        }, timeoutMs);

        const handler = (event: Event) => {
          window.clearTimeout(timeout);
          window.removeEventListener(eventName, handler);
          resolve((event as CustomEvent).detail);
        };

        window.addEventListener(eventName, handler);
      }),
    { eventName, timeoutMs },
  ) as Promise<T>;

  await page.waitForTimeout(50);
  await trigger();
  return eventPromise;
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

  const coinPurchase = await withNextWindowEvent<CoinBalanceEvent>(
    page,
    "gameCornerCoinBalance",
    async () => {
      await page.evaluate(() => window.__capturequestTest?.buyGameCornerCoins());
    },
  );
  expect(coinPurchase).toMatchObject({
    success: true,
    coins: 60,
    money: 0,
    message: "Here are 50 coins!",
  });
  await dismissDialogue(page, 6);

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
  const slotResult = await withNextWindowEvent<SlotResultEvent>(
    page,
    "gameCornerSlotResult",
    async () => {
      await page.getByTestId("slot-machine-spin").click();
    },
  );
  expect(slotResult.success).toBe(true);
  expect(slotResult.bet).toBe(3);
  expect(slotResult.coins).toBeGreaterThanOrEqual(0);
  expect(slotResult.coins).toBeLessThanOrEqual(9999);
  expect(slotResult.payout).toBeGreaterThanOrEqual(0);

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

test("Game Corner prize exchange requires Coin Case and buys rewards", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);

  await jumpToScenario(page, "game_corner_prize_list_no_coin_case");
  await waitForMap(page, /Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);
  const missingCaseList = await withNextWindowEvent<PrizeListEvent>(
    page,
    "gameCornerPrizeList",
    async () => {
      await page.evaluate(() =>
        window.__capturequestTest?.requestGameCornerPrizeList(),
      );
    },
  );
  expect(missingCaseList).toMatchObject({
    success: false,
    coins: 500,
    error: "You need a COIN CASE!",
  });
  await dismissDialogue(page, 4);

  await jumpToScenario(page, "game_corner_prize_list_with_coin_case");
  await waitForMap(page, /Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);

  const prizeList = await withNextWindowEvent<PrizeListEvent>(
    page,
    "gameCornerPrizeList",
    async () => {
      await page.evaluate(() =>
        window.__capturequestTest?.requestGameCornerPrizeList(),
      );
    },
  );
  expect(prizeList.success).toBe(true);
  expect(prizeList.coins).toBe(9999);
  expect(prizeList.prizes.map((prize) => prize.name)).toEqual(
    expect.arrayContaining(["ABRA", "TM23 Dragon Rage"]),
  );

  const tm23 = prizeList.prizes.find(
    (prize) => prize.name === "TM23 Dragon Rage",
  );
  expect(tm23).toBeTruthy();

  const prizeBuy = await withNextWindowEvent<PrizeBuyEvent>(
    page,
    "gameCornerPrizeBuy",
    async () => {
      await page.evaluate(
        (prizeId) => window.__capturequestTest?.buyGameCornerPrize(prizeId),
        tm23!.id,
      );
    },
  );
  expect(prizeBuy).toMatchObject({
    success: true,
    coins: 6699,
    prizeName: "TM23 Dragon Rage",
    message: "Here you go!",
  });
  await waitForInventoryItem(page, /TM23/i);
  await dismissDialogue(page, 6);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Game Corner prize Pokemon go to party or PC when party is full", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);

  await jumpToScenario(page, "game_corner_prize_list_with_coin_case");
  await waitForMap(page, /Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);

  const openPartyPrizeList = await withNextWindowEvent<PrizeListEvent>(
    page,
    "gameCornerPrizeList",
    async () => {
      await page.evaluate(() =>
        window.__capturequestTest?.requestGameCornerPrizeList(),
      );
    },
  );
  const abra = openPartyPrizeList.prizes.find((prize) => prize.name === "ABRA");
  expect(abra).toBeTruthy();

  const partyPrizeBuy = await withNextWindowEvent<PrizeBuyEvent>(
    page,
    "gameCornerPrizeBuy",
    async () => {
      await page.evaluate(
        (prizeId) => window.__capturequestTest?.buyGameCornerPrize(prizeId),
        abra!.id,
      );
    },
  );
  expect(partyPrizeBuy).toMatchObject({
    success: true,
    coins: 9819,
    prizeName: "ABRA",
    prizeLevel: 9,
    addedToParty: true,
    pcBox: -1,
    pcSlot: -1,
    message: "Here you go!",
  });

  await page.evaluate(() => window.__capturequestTest?.requestPokemonParty());
  await waitForPartyPokemon(page, 63);
  await expect
    .poll(async () => {
      const state = await getGameState(page);
      return state.pokemon.party.map((pokemon) => ({
        id: pokemon.id,
        name: pokemon.name,
        level: pokemon.level,
      }));
    })
    .toContainEqual({ id: 63, name: "ABRA", level: 9 });
  await dismissDialogue(page, 6);

  await jumpToScenario(page, "debug_game_corner_prize_full_party_ready");
  await waitForMap(page, /Game Corner Prize Room|GAME_CORNER_PRIZE_ROOM/);

  const fullPartyPrizeList = await withNextWindowEvent<PrizeListEvent>(
    page,
    "gameCornerPrizeList",
    async () => {
      await page.evaluate(() =>
        window.__capturequestTest?.requestGameCornerPrizeList(),
      );
    },
  );
  const fullPartyAbra = fullPartyPrizeList.prizes.find(
    (prize) => prize.name === "ABRA",
  );
  expect(fullPartyAbra).toBeTruthy();

  const pcPrizeBuy = await withNextWindowEvent<PrizeBuyEvent>(
    page,
    "gameCornerPrizeBuy",
    async () => {
      await page.evaluate(
        (prizeId) => window.__capturequestTest?.buyGameCornerPrize(prizeId),
        fullPartyAbra!.id,
      );
    },
  );
  expect(pcPrizeBuy).toMatchObject({
    success: true,
    coins: 9819,
    prizeName: "ABRA",
    prizeLevel: 9,
    addedToParty: false,
    pcBox: 0,
    pcSlot: 1,
    message: "Here you go!",
  });

  await page.evaluate(() => window.__capturequestTest?.requestPokemonParty());
  await expect
    .poll(async () => {
      const state = await getGameState(page);
      return state.pokemon.party.map((pokemon) => pokemon.id);
    })
    .toEqual([1, 4, 7, 25, 16, 19]);

  await page.evaluate(() => window.__capturequestTest?.requestPokemonPC());
  await waitForPCPokemon(page, 63);
  await expect
    .poll(async () => {
      const state = await getGameState(page);
      return state.pokemon.pc.boxPokemon.map((pokemon) => ({
        id: pokemon.id,
        name: pokemon.name,
        level: pokemon.level,
        boxSlot: pokemon.boxSlot,
      }));
    })
    .toContainEqual({ id: 63, name: "ABRA", level: 9, boxSlot: 1 });
  await page.getByRole("button", { name: "LOG OFF" }).click();
  await expect
    .poll(async () => {
      const state = await getGameState(page);
      return state.pokemon.pc.isOpen;
    })
    .toBe(false);
  await dismissDialogue(page, 6);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
