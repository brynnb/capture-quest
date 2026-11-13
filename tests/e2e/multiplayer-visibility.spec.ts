import { expect, test, type Browser, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors, type PageErrorCollector } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { clickTile, pressMovement } from "./helpers/input";
import { activateWarpWithClick, tileBeforeWarp } from "./helpers/warps";
import {
  getGameState,
  waitForActorAbsent,
  waitForActorVisible,
  waitForMapChange,
  waitForMap,
  waitForNoMapLoading,
  waitForPlayerIdle,
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
    if (sharedMap === null) throw new Error("Players have no shared map");
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
    await waitForNoMapLoading(playerA.page);
    await waitForPlayerIdle(playerA.page);
    // Arrival is on this door warp. Click onto the adjacent floor first,
    // then enter the stair without holding a key across the map transition.
    await activateWarpWithClick(
      playerA.page,
      returnWarp!,
      tileBeforeWarp(returnWarp!, "up"),
    );
    await waitForMap(playerA.page, sharedMap);

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


test("character-private boulder movement does not change another viewer's puzzle", async ({ browser }) => {
  test.setTimeout(120_000);
  const a = await newTestPage(browser), b = await newTestPage(browser);
  try {
    await enterAsGuest(a.page); await enterAsGuest(b.page);
    for (const participant of [a, b]) {
      await jumpToScenario(participant.page, "seafoam_1f_runtime_boulder_push_facing_update");
      await waitForNoMapLoading(participant.page);
    }
    const position = async (page: Page) => {
      const actor = (await getGameState(page)).visibleActors.find(actor => actor.name === "SeafoamIslands1F_NPC_1");
      return actor && [actor.x, actor.y];
    };
    await expect.poll(() => position(a.page)).toEqual([18, 10]);
    await expect.poll(() => position(b.page)).toEqual([18, 10]);
    await pressMovement(a.page, "up");
    await expect.poll(() => position(a.page)).toEqual([18, 9]);
    // Allow the same streamed publication to be processed by both contexts.
    await b.page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
    expect(await position(b.page)).toEqual([18, 10]);
    assertNoErrors(a.errors, b.errors);
  } finally { await a.context.close(); await b.context.close(); }
});
