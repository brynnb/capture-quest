import { expect, test, type Browser, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors, type PageErrorCollector } from "./helpers/errors";
import { clickTile } from "./helpers/input";
import {
  getGameState,
  waitForActorAbsent,
  waitForActorVisible,
  waitForMapChange,
  waitForNoMapLoading,
  waitForWarps,
} from "./helpers/state";

async function newTestPage(browser: Browser) {
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = collectPageErrors(page);
  return { context, page, errors };
}

async function enterAsGuest(page: Page) {
  const name = await createGuestCharacterAndEnterWorld(page);
  await waitForNoMapLoading(page);
  const state = await getGameState(page);
  expect(state.player.internalId).not.toBeNull();
  return { id: state.player.internalId!, name };
}

function assertNoErrors(...collectors: PageErrorCollector[]) {
  for (const collector of collectors) {
    collector.assertNoSevereErrors();
  }
}

test("players are removed from old map visibility when another player warps", async ({
  browser,
}) => {
  test.setTimeout(180_000);

  const playerA = await newTestPage(browser);
  const playerB = await newTestPage(browser);

  try {
    const playerAInfo = await enterAsGuest(playerA.page);
    const playerBInfo = await enterAsGuest(playerB.page);

    await waitForActorVisible(
      playerA.page,
      (actor) => actor.name === playerBInfo.name,
    );
    await waitForActorVisible(
      playerB.page,
      (actor) => actor.name === playerAInfo.name,
    );

    const sharedMap = (await getGameState(playerA.page)).map.id;
    await waitForWarps(playerA.page);
    const warp = (await getGameState(playerA.page)).warps[0];
    await clickTile(playerA.page, warp.x, warp.y);

    await expect
      .poll(
        async () => {
          const state = await getGameState(playerA.page);
          return state.map.id;
        },
        { timeout: 30_000 },
      )
      .not.toBe(sharedMap);

    await waitForActorAbsent(
      playerB.page,
      (actor) => actor.name === playerAInfo.name,
    );

    await waitForWarps(playerA.page);
    const returnWarp = (await getGameState(playerA.page)).warps.find(
      (candidate) => candidate.destinationMapId === sharedMap,
    );
    expect(returnWarp, "return warp to Player B's map").toBeTruthy();
    const playerAInteriorMap = (await getGameState(playerA.page)).map.id;
    await clickTile(playerA.page, returnWarp!.x, returnWarp!.y);
    await waitForMapChange(playerA.page, playerAInteriorMap);

    await waitForActorVisible(
      playerB.page,
      (actor) => actor.name === playerAInfo.name,
    );

    await waitForWarps(playerB.page);
    const playerBWarp = (await getGameState(playerB.page)).warps[0];
    const playerBSharedMap = (await getGameState(playerB.page)).map.id;
    await clickTile(playerB.page, playerBWarp.x, playerBWarp.y);
    await waitForMapChange(playerB.page, playerBSharedMap);

    await waitForActorAbsent(
      playerA.page,
      (actor) => actor.name === playerBInfo.name,
    );

    await quitToCharacterSelect(playerA.page);
    await quitToCharacterSelect(playerB.page);
    assertNoErrors(playerA.errors, playerB.errors);
  } finally {
    await playerA.context.close();
    await playerB.context.close();
  }
});
