import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { advanceBattleTextToPhase, waitForBattleOpen, endWildBattleIfOpen } from "./helpers/battle";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading } from "./helpers/state";
import { pressSpace } from "./helpers/input";
import * as OpCodes from "../../src/net/generated/opcodes";

test("a lost committed turn reply recovers without resending; its late reply cannot revive a reentered panel", async ({ page }) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  let actions = 0, closes = 0, endNotifications = 0, recoveredTerminal = false;
  const recoveryRequests: { current?: boolean; mapId?: number; requestId: string }[] = [];
  let release: (() => void) | undefined;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.PokeBattleActionRequest) actions++;
        if (opcode === OpCodes.PokeBattleCloseRequest) closes++;
        if (opcode === OpCodes.GameplayStateRequest && actions > 0) recoveryRequests.push(JSON.parse(message.subarray(6).toString()));
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.PokeBattleActionResponse && !release) { release = () => socket.send(message); return; }
        if (opcode === OpCodes.PokeBattleEndNotify) endNotifications++;
        if (opcode === OpCodes.GameplayStateResponse && actions > 0) {
          const snapshot = JSON.parse(message.subarray(6).toString());
          recoveredTerminal ||= snapshot.success && snapshot.battle?.needsDismissal === true;
        }
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "active_battle_fixture_wild"); await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
  await page.getByTestId("battle-action-fight").click(); await page.getByTestId("battle-move-0").click();
  await expect.poll(() => !!release).toBe(true);
  await expect.poll(() => recoveredTerminal, { timeout: 20000 }).toBe(true);
  await expect.poll(async () => (await getGameState(page)).battle.isOpen).toBe(false);
  expect(actions).toBe(1); expect(closes).toBe(1); expect(endNotifications).toBe(0);
  expect(recoveryRequests).toEqual([{ current: true, requestId: expect.any(String) }]);
  const partyAfterCommit = (await getGameState(page)).pokemon.party;
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  release!();
  // A second correlated round trip is a delivery barrier, without applying a
  // snapshot that could hide resurrection by an incorrectly global late reply.
  await page.evaluate(async () => { const path = "/src/phaser-game/services/PlayerMovementService.ts"; const { readOwnedPlayerPosition } = await import(path); await readOwnedPlayerPosition(); });
  expect((await getGameState(page)).battle.isOpen).toBe(false);
  expect((await getGameState(page)).pokemon.party).toEqual(partyAfterCommit);
  expect(actions).toBe(1);
  await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});

test("a lost close acknowledgement restores absence without sending a second close", async ({ page }) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  let closes = 0, droppedClose = false;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PokeBattleCloseRequest) closes++;
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PokeBattleCloseResponse) { droppedClose = true; return; }
      socket.send(message);
    });
  });
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "active_battle_fixture_wild"); await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
  await page.getByTestId("battle-action-fight").click(); await page.getByTestId("battle-move-0").click();
  await endWildBattleIfOpen(page);
  await expect.poll(() => droppedClose, { timeout: 20000 }).toBe(true);
  await expect.poll(async () => (await getGameState(page)).battle.isOpen, { timeout: 20000 }).toBe(false);
  expect(closes).toBe(1);
  await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});

test("a lost terminal trainer reply dismisses once and continues the original Brock reward script", async ({ page }) => {
  test.setTimeout(180000);
  const errors = collectPageErrors(page);
  let actions = 0, closes = 0, turnReplies = 0, droppedTerminal = false, rewardToken = "";
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.PokeBattleActionRequest) actions++;
        if (opcode === OpCodes.PokeBattleCloseRequest) closes++;
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.PokeBattleActionResponse) {
          const response = JSON.parse(message.subarray(6).toString());
          turnReplies++;
          if (response.success && response.battle?.needsDismissal) { droppedTerminal = true; return; }
        }
        if (opcode === OpCodes.CutsceneStartNotify) {
          const plan = JSON.parse(message.subarray(6).toString());
          if (plan.scriptLabel === "PewterGymBrockPostBattle") rewardToken = plan.completionToken;
        }
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "active_battle_fixture_brock_recovery");
  await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
  for (let turn = 0; turn < 4 && !droppedTerminal; turn++) {
    const before = turnReplies;
    await page.getByTestId("battle-action-fight").click(); await page.getByTestId("battle-move-0").click();
    await expect.poll(() => turnReplies > before).toBe(true);
    if (!droppedTerminal) {
      await expect(page.getByText("Waiting for the battle…", { exact: true })).toBeHidden();
      await advanceBattleTextToPhase(page, "action_select");
    }
  }
  expect(droppedTerminal).toBe(true);
  await expect.poll(() => rewardToken, { timeout: 20000 }).not.toBe("");
  expect(closes).toBe(1);
  const actionCount = actions;
  await expect.poll(async () => (await getGameState(page)).dialogue.isOpen).toBe(true);
  for (let i = 0; i < 40; i++) {
    const state = await getGameState(page);
    if (!state.worldInput.frozen && !state.dialogue.isOpen) break;
    if (state.dialogue.isOpen) await pressSpace(page);
    else await page.waitForTimeout(200);
  }
  await expect.poll(async () => (await getGameState(page)).inventory.items.filter(item => item.itemId === 122).reduce((sum, item) => sum + item.quantity, 0)).toBe(1);
  const settled = await getGameState(page);
  expect(settled.battle.isOpen).toBe(false);
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  await expect.poll(async () => (await getGameState(page)).inventory.items.filter(item => item.itemId === 122).reduce((sum, item) => sum + item.quantity, 0)).toBe(1);
  expect((await getGameState(page)).pokemon.party).toEqual(settled.pokemon.party);
  expect(actions).toBe(actionCount); expect(closes).toBe(1);
  await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});
