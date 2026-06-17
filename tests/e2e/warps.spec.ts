import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  getGameState,
  waitForMap,
  waitForMapChange,
  waitForNoMapLoading,
  waitForPlayerIdle,
  waitForPlayerTile,
  waitForWarps,
} from "./helpers/state";
import {
  activateDirectionalWarpWithKeyboard,
  activateDoorWarpWithKeyboard,
  activateCurrentWarpByClickingBeyond,
  activateWarpWithClick,
  moveToTileByKeyboardAndWait,
  requireWarp,
  tileBeforeWarp,
} from "./helpers/warps";

test("player can use real tile clicks to move through starting house warps", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await waitForNoMapLoading(page);
  await waitForWarps(page);

  const upstairsState = await getGameState(page);
  const upstairsMapId = upstairsState.map.id;
  const stairsWarp = upstairsState.warps[0];
  expect(stairsWarp).toBeTruthy();

  await clickTile(page, stairsWarp.x, stairsWarp.y);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.map.id;
      },
      { timeout: 30_000 },
    )
    .not.toBe(upstairsMapId);

  await waitForNoMapLoading(page);
  await waitForWarps(page);

  const downstairsState = await getGameState(page);
  const downstairsMapId = downstairsState.map.id;
  const exitWarp =
    downstairsState.warps.find(
      (warp) => warp.destinationMapId !== upstairsMapId,
    ) ?? downstairsState.warps[0];
  expect(exitWarp).toBeTruthy();

  await clickTile(page, exitWarp.x, exitWarp.y);
  await expect
    .poll(
      async () => {
        const state = await getGameState(page);
        return state.map.id;
      },
      { timeout: 30_000 },
    )
    .not.toBe(downstairsMapId);
  await waitForNoMapLoading(page);
  await waitForPlayerIdle(page);

  const outsideState = await getGameState(page);
  expect(outsideState.worldInput.frozen).toBe(false);
  expect(outsideState.player.x).not.toBeNull();
  expect(outsideState.player.y).not.toBeNull();

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Red's House door warps work first try by keyboard and click", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForWarps(page);

  let state = await getGameState(page);
  let exitWarp = requireWarp(
    state,
    (warp) =>
      warp.destinationMapId === 0 &&
      warp.warpDirection === "DOWN" &&
      warp.x === 2,
    "Red's House 1F down exit mat",
  );

  await activateDirectionalWarpWithKeyboard(page, exitWarp, "down");
  state = await getGameState(page);
  expect(state.map.id).not.toBe(37);
  expect(state.worldInput.frozen).toBe(false);

  state = await getGameState(page);
  const entryWarp = requireWarp(
    state,
    (warp) => warp.destinationMapId === 37 && warp.x === 5 && warp.y === 5,
    "Pallet Town door into Red's House",
  );
  await activateDoorWarpWithKeyboard(page, entryWarp, "up");
  await waitForMap(page, "REDS_HOUSE_1F");

  state = await getGameState(page);
  exitWarp = requireWarp(
    state,
    (warp) =>
      warp.destinationMapId === 0 &&
      warp.warpDirection === "DOWN" &&
      warp.x === 2,
    "Red's House 1F down exit mat for click",
  );
  await activateWarpWithClick(page, exitWarp, tileBeforeWarp(exitWarp, "down"));
  state = await getGameState(page);
  expect(state.map.id).not.toBe(37);
  expect(state.worldInput.frozen).toBe(false);

  state = await getGameState(page);
  const clickEntryWarp = requireWarp(
    state,
    (warp) => warp.destinationMapId === 37 && warp.x === 5 && warp.y === 5,
    "Pallet Town door into Red's House for click",
  );
  await activateWarpWithClick(page, clickEntryWarp, tileBeforeWarp(clickEntryWarp, "up"));
  await waitForMap(page, "REDS_HOUSE_1F");

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Route 11 gate left and right exits work first try", async ({ page }) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_route11_gate_1f_center");
  await waitForMap(page, "ROUTE_11_GATE_1F");
  await waitForWarps(page);

  let state = await getGameState(page);
  let leftExit = requireWarp(
    state,
    (warp) => warp.destinationMapId === 22 && warp.x === 0 && warp.y === 4,
    "Route 11 Gate left exit",
  );
  await activateDoorWarpWithKeyboard(page, leftExit, "left");
  state = await getGameState(page);
  expect(state.map.id).not.toBe(84);

  await jumpToScenario(page, "debug_warp_route11_gate_1f_center");
  await waitForMap(page, "ROUTE_11_GATE_1F");
  state = await getGameState(page);
  leftExit = requireWarp(
    state,
    (warp) => warp.destinationMapId === 22 && warp.x === 0 && warp.y === 4,
    "Route 11 Gate left exit for click",
  );
  const leftClickSetup = tileBeforeWarp(leftExit, "left");
  await moveToTileByKeyboardAndWait(page, leftClickSetup.x, leftClickSetup.y);
  await activateWarpWithClick(page, leftExit);
  state = await getGameState(page);
  expect(state.map.id).not.toBe(84);

  await jumpToScenario(page, "debug_warp_route11_gate_1f_center");
  await waitForMap(page, "ROUTE_11_GATE_1F");
  state = await getGameState(page);
  let rightExit = requireWarp(
    state,
    (warp) => warp.destinationMapId === 22 && warp.x === 7 && warp.y === 4,
    "Route 11 Gate right exit",
  );
  await activateDoorWarpWithKeyboard(page, rightExit, "right");
  state = await getGameState(page);
  expect(state.map.id).not.toBe(84);

  await jumpToScenario(page, "debug_warp_route11_gate_1f_center");
  await waitForMap(page, "ROUTE_11_GATE_1F");
  state = await getGameState(page);
  rightExit = requireWarp(
    state,
    (warp) => warp.destinationMapId === 22 && warp.x === 7 && warp.y === 4,
    "Route 11 Gate right exit for click",
  );
  const rightClickSetup = tileBeforeWarp(rightExit, "right");
  await moveToTileByKeyboardAndWait(page, rightClickSetup.x, rightClickSetup.y);
  await activateWarpWithClick(page, rightExit);
  state = await getGameState(page);
  expect(state.map.id).not.toBe(84);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Cerulean Trashed House upper exit works first try by keyboard and click", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_cerulean_trashed_house_upper_exit");
  await waitForMap(page, "CERULEAN_TRASHED_HOUSE");
  await waitForWarps(page);

  let state = await getGameState(page);
  let upperExit = requireWarp(
    state,
    (warp) => warp.destinationMapId === 3 && warp.x === 3 && warp.y === 0,
    "Cerulean Trashed House upper exit",
  );
  const houseMapId = state.map.id;
  await waitForPlayerIdle(page);
  await pressMovement(page, "up");
  await waitForMapChange(page, houseMapId);
  state = await getGameState(page);
  expect(state.map.id).not.toBe(62);

  await jumpToScenario(page, "debug_warp_cerulean_trashed_house_upper_exit");
  await waitForMap(page, "CERULEAN_TRASHED_HOUSE");

  state = await getGameState(page);
  upperExit = requireWarp(
    state,
    (warp) => warp.destinationMapId === 3 && warp.x === 3 && warp.y === 0,
    "Cerulean Trashed House upper exit for click",
  );
  await activateCurrentWarpByClickingBeyond(page, upperExit, "up");
  state = await getGameState(page);
  expect(state.map.id).not.toBe(62);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("normal mart exit works first try by keyboard and click", async ({
  page,
}) => {
  test.setTimeout(150_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_viridian_mart_exit_mat");
  await waitForMap(page, /Viridian Mart|VIRIDIAN_MART/);
  await waitForWarps(page);

  let state = await getGameState(page);
  let exitWarp = requireWarp(
    state,
    (warp) =>
      warp.destinationMapId !== state.map.id &&
      warp.warpDirection === "DOWN" &&
      warp.x === 3 &&
      warp.y === 7,
    "Viridian Mart left downward exit",
  );
  const startingMapId = state.map.id;
  await pressMovement(page, "down");
  await waitForMapChange(page, startingMapId);
  state = await getGameState(page);
  expect(state.map.id).not.toBe(exitWarp.sourceMapId);
  expect(state.worldInput.frozen).toBe(false);

  await jumpToScenario(page, "debug_warp_viridian_mart_exit_mat");
  await waitForMap(page, /Viridian Mart|VIRIDIAN_MART/);
  state = await getGameState(page);
  exitWarp = requireWarp(
    state,
    (warp) =>
      warp.destinationMapId !== state.map.id &&
      warp.warpDirection === "DOWN" &&
      warp.x === 3 &&
      warp.y === 7,
    "Viridian Mart left downward exit for click",
  );
  const clickStartingMapId = state.map.id;
  await clickTile(page, exitWarp.x, exitWarp.y);
  await waitForMapChange(page, clickStartingMapId);
  state = await getGameState(page);
  expect(state.map.id).not.toBe(exitWarp.sourceMapId);
  expect(state.worldInput.frozen).toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("Warp Home lands in Kanto at the safe Pallet Town tile", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_route11_gate_1f_center");
  await waitForMap(page, "ROUTE_11_GATE_1F");

  await page.getByRole("button", { name: "Warp Home" }).click();
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 9, 4, 30_000);

  const state = await getGameState(page);
  expect(state.worldInput.frozen).toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
