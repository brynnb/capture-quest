import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, dismissDialogue, pressMovement, pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForPlayerTile, waitForPlayerIdle } from "./helpers/state";

test("surf can be started from a scenario fixture using player input", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_field_move_surf_ready");

  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 3, 17);

  await clickTile(page, 4, 17);

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.dialogue.isOpen && state.dialogue.isChoicePending;
      },
      { timeout: 30_000 },
    )
    .toBe(true);

  await pressSpace(page);

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return {
          x: state.player.x,
          y: state.player.y,
          surfing: state.player.isSurfing,
        };
      },
      { timeout: 30_000 },
    )
    .toEqual({ x: 4, y: 17, surfing: true });

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("surf requires deliberate Space input when using WASD", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_field_move_surf_ready");

  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 3, 17);

  await pressMovement(page, "right");
  await page.waitForTimeout(300);

  let state = await getGameState(page);
  expect(state.player.x).toBe(3);
  expect(state.player.y).toBe(17);
  expect(state.player.isSurfing).toBe(false);
  expect(state.dialogue.isOpen).toBe(false);

  await pressSpace(page);
  await expect
    .poll(
      async () => {
        state = await getGameState(page);
        return state.dialogue.isOpen && state.dialogue.isChoicePending;
      },
      { timeout: 30_000 },
    )
    .toBe(true);
  expect((await getGameState(page)).dialogue.selectedChoice).toBe("yes");

  await pressSpace(page);
  await expect
    .poll(
      async () => {
        state = await getGameState(page);
        return {
          x: state.player.x,
          y: state.player.y,
          surfing: state.player.isSurfing,
        };
      },
      { timeout: 30_000 },
    )
    .toEqual({ x: 4, y: 17, surfing: true });

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("cut can be used from a scenario fixture and then walked through", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_field_move_cut_ready");

  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 189, -169);

  await pressSpace(page);

  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.dialogue.isOpen;
      },
      { timeout: 30_000 },
    )
    .toBe(true);

  await dismissDialogue(page);

  await pressMovement(page, "up");
  await waitForPlayerTile(page, 189, -170);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("clicking a cut bush triggers cut before the player can walk through it", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_field_move_cut_ready");

  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 189, -169);

  await clickTile(page, 189, -170);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.dialogue.isOpen;
      },
      { timeout: 30_000 },
    )
    .toBe(true);

  await dismissDialogue(page);
  await pressMovement(page, "up");
  await waitForPlayerTile(page, 189, -170);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});


test("facing an adjacent Strength boulder preserves its server-controlled follow-up step", async ({ page }, testInfo) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "seafoam_1f_runtime_boulder_push_facing_update");
  await waitForMap(page, "SEAFOAM_ISLANDS_1F");
  await waitForPlayerTile(page, 18, 11);
  await waitForPlayerIdle(page);
  await expect.poll(async () => {
    const boulder = (await getGameState(page)).visibleActors.find((actor) => actor.name === "SeafoamIslands1F_NPC_1");
    return boulder && { x: boulder.x, y: boulder.y };
  }).toEqual({ x: 18, y: 10 });
  await page.screenshot({ path: testInfo.outputPath("strength-before.png") });
  await pressMovement(page, "up");
  await waitForPlayerTile(page, 18, 10);
  await waitForPlayerIdle(page);
  await expect.poll(async () => {
    const boulder = (await getGameState(page)).visibleActors.find((actor) => actor.name === "SeafoamIslands1F_NPC_1");
    return boulder && { x: boulder.x, y: boulder.y };
  }).toEqual({ x: 18, y: 9 });
  await page.screenshot({ path: testInfo.outputPath("strength-after.png") });
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
