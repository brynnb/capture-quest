import { expect, test } from "@playwright/test";
import { waitForBattleOpen } from "./helpers/battle";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, dismissDialogue, pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import {
  getGameState,
  waitForAudioRequest,
  waitForInventoryItem,
  waitForLastSFX,
  waitForMap,
  waitForPlayerTile,
  waitForWarps,
} from "./helpers/state";
import { activateDirectionalWarpWithKeyboard, requireWarp } from "./helpers/warps";

test("audio requests fire for map music, doors, items, battle, surf, and bike", async ({
  page,
}) => {
  test.setTimeout(180_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);

  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForWarps(page);
  let state = await getGameState(page);
  const exitWarp = requireWarp(
    state,
    (warp) =>
      warp.destinationMapId === 0 &&
      warp.warpDirection === "DOWN" &&
      warp.x === 2 &&
      warp.y === 7,
    "Red's House 1F exit",
  );
  await activateDirectionalWarpWithKeyboard(page, exitWarp, "down");
  await waitForLastSFX(page, /go_outside/i);
  await waitForMap(page, /Kanto|Unified Overworld/);

  await jumpToScenario(page, "viridian_mart_oaks_parcel");
  await waitForMap(page, /Viridian Mart|VIRIDIAN_MART/);
  await dismissDialogue(page, 8);
  await waitForInventoryItem(page, 70);
  await waitForLastSFX(page, /get_key_item|get_item_1/i);

  await jumpToScenario(page, "active_battle_fixture_wild");
  await waitForBattleOpen(page);
  await waitForAudioRequest(page, /wild_battle/i);

  await jumpToScenario(page, "debug_field_move_surf_ready");
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 3, 17);
  await clickTile(page, 4, 17);
  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.dialogue.isOpen && next.dialogue.isChoicePending;
      },
      { timeout: 30_000 },
    )
    .toBe(true);
  await pressSpace(page);
  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.player.isSurfing && next.audio.isSurfing;
      },
      { timeout: 30_000 },
    )
    .toBe(true);

  await jumpToScenario(page, "bike_shop_already_has_bicycle");
  await page.getByRole("button", { name: "Warp Home" }).click();
  await waitForMap(page, /Kanto|Unified Overworld/);
  await page.getByRole("button", { name: "Inventory" }).click();
  await page.getByTestId("inventory-item-bicycle").click();
  await expect
    .poll(
      async () => {
        const next = await getGameState(page);
        return next.audio.isBicycleActive;
      },
      { timeout: 20_000 },
    )
    .toBe(true);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
