import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { advanceBattleTextToPhase, waitForBattleOpen, endWildBattleIfOpen } from "./helpers/battle";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading } from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";

test("a lost committed turn reply recovers without resending; its late reply cannot revive a reentered panel", async ({ page }) => {
  test.setTimeout(120000);
  const errors = collectPageErrors(page);
  let actions = 0, closes = 0, endNotifications = 0, recoveredAbsent = false;
  let release: (() => void) | undefined;
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
        if (opcode === OpCodes.PokeBattleActionResponse && !release) { release = () => socket.send(message); return; }
        if (opcode === OpCodes.PokeBattleEndNotify) endNotifications++;
        if (opcode === OpCodes.GameplayStateResponse && actions > 0) {
          const snapshot = JSON.parse(message.subarray(6).toString());
          recoveredAbsent ||= snapshot.success && snapshot.battle === null;
        }
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "active_battle_fixture_wild"); await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
  await page.getByTestId("battle-action-fight").click(); await page.getByTestId("battle-move-0").click();
  await expect.poll(() => !!release).toBe(true);
  await expect.poll(() => recoveredAbsent, { timeout: 20000 }).toBe(true);
  await expect.poll(async () => (await getGameState(page)).battle.isOpen).toBe(false);
  expect(actions).toBe(1); expect(closes).toBe(0); expect(endNotifications).toBe(0);
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
