import { expect, test, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, quitToCharacterSelect } from "./helpers/auth";
import { advanceBattleTextToPhase, endWildBattleIfOpen, waitForBattleOpen } from "./helpers/battle";
import { collectPageErrors } from "./helpers/errors";
import {
  expectBattle,
  expectDialogue,
  expectInventory,
  expectPlayer,
  expectTile,
  runProbeSequence,
  sendProbeCommand,
  waitForProbeIdle,
} from "./helpers/probe";
import {
  getEngineSnapshot,
  getGameState,
  waitForMap,
  waitForPlayerTile,
} from "./helpers/state";

async function advanceProbeDialogueToChoice(page: Page) {
  for (let i = 0; i < 30; i += 1) {
    const snapshot = await getEngineSnapshot(page);
    if (snapshot.dialogue.isOpen && snapshot.dialogue.isChoicePending) return snapshot;
    await sendProbeCommand(page, { press: "SPACE" });
    await page.waitForTimeout(snapshot.dialogue.isTyping ? 250 : 150);
  }
  return getEngineSnapshot(page);
}

test("engine probe returns a bounded live Phaser snapshot", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await sendProbeCommand(page, { scenario: { name: "debug_field_move_cut_ready" } });
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 189, -169);

  const state = await getGameState(page);
  const snapshot = await getEngineSnapshot(page);
  const maxRadiusTileCount = (snapshot.bounds.radius * 2 + 1) ** 2;

  expectPlayer(snapshot, { x: state.player.x, y: state.player.y });
  expect(snapshot.bounds).toMatchObject({
    mode: "radius",
    radius: 8,
    center: { x: 189, y: -169 },
  });
  expect(snapshot.tiles.length).toBeGreaterThan(0);
  expect(snapshot.tiles.length).toBeLessThanOrEqual(maxRadiusTileCount);
  expect(
    snapshot.tiles.every(
      (tile) =>
        tile.x >= snapshot.bounds.minX &&
        tile.x <= snapshot.bounds.maxX &&
        tile.y >= snapshot.bounds.minY &&
        tile.y <= snapshot.bounds.maxY,
    ),
  ).toBe(true);
  expectTile(snapshot, 189, -170, { isCutTree: true });

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("engine probe commands can drive scenario jumps and inventory item use", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await sendProbeCommand(page, { scenario: { name: "debug_inventory_potion_ready" } });
  await waitForMap(page, /Kanto|Unified Overworld/);

  let snapshot = await getEngineSnapshot(page);
  expectInventory(snapshot, { contains: "POTION" });

  const itemResult = await sendProbeCommand(page, { useItem: { itemName: "POTION" } });
  expect(itemResult.ok).toBe(true);
  await expect(page.getByTestId("inventory-cursor-item")).toBeVisible();
  snapshot = await getEngineSnapshot(page);
  expectInventory(snapshot, { contains: "POTION" });
  expect(
    snapshot.lastEvents.some((event) => event.type === "itemUsed"),
  ).toBe(true);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("engine probe commands can select dialogue choices", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await sendProbeCommand(page, { scenario: { name: "debug_field_move_surf_ready" } });
  await waitForMap(page, /Kanto|Unified Overworld/);
  await waitForPlayerTile(page, 3, 17);
  await waitForProbeIdle(page);

  await sendProbeCommand(page, { clickTile: { x: 4, y: 17 } });
  let snapshot = await advanceProbeDialogueToChoice(page);

  expectDialogue(snapshot, { isOpen: true, isChoicePending: true });
  await sendProbeCommand(page, { dialogueChoice: { choice: "no" } });
  snapshot = await getEngineSnapshot(page);
  expectDialogue(snapshot, { isChoicePending: false });
  expectPlayer(snapshot, { x: 3, y: 17, isSurfing: false });

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("engine probe commands can drive battle action and move selection", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await sendProbeCommand(page, { scenario: { name: "active_battle_fixture_wild" } });
  await waitForBattleOpen(page);
  await advanceBattleTextToPhase(page, "action_select");

  let snapshot = await getEngineSnapshot(page);
  expectBattle(snapshot, { isOpen: true, phase: "action_select", selectedAction: "fight" });
  await sendProbeCommand(page, { battleAction: { action: "fight" } });
  await expect
    .poll(
      async () => (await getEngineSnapshot(page)).battle.phase,
      { timeout: 15_000 },
    )
    .toBe("move_select");

  snapshot = await getEngineSnapshot(page);
  expectBattle(snapshot, { phase: "move_select", selectedMoveIndex: 0 });
  const beforeTurn = snapshot.battle.turnNumber;
  await sendProbeCommand(page, { battleMove: { index: 0 } });
  await expect
    .poll(
      async () => (await getEngineSnapshot(page)).battle.turnNumber,
      { timeout: 30_000 },
    )
    .toBeGreaterThan(beforeTurn);

  await endWildBattleIfOpen(page);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});

test("runProbeSequence returns a useful live after-snapshot", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);

  await createGuestCharacterAndEnterWorld(page);
  await waitForMap(page, "REDS_HOUSE_2F");
  await waitForPlayerTile(page, 3, 6);
  const snapshot = await runProbeSequence(page, [
    { face: "LEFT" },
  ]);

  expectPlayer(snapshot, { x: 3, y: 6, direction: "LEFT" });
  expect(snapshot.worldInput.frozen).toBe(false);

  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
