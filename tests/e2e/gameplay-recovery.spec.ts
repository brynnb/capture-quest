import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerTile } from "./helpers/state";
import type { GameplayStateResponse } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";

test("lost sight-trainer notification survives re-entry and a lost battle start recovers current saved battle", async ({ page }) => {
  test.setTimeout(150000);
  const errors = collectPageErrors(page);
  const snapshots: GameplayStateResponse[] = [];
  const ready: Array<{ encounterToken: string; trainerActorId: number }> = [];
  let token = "", droppedBattle = false;
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.TrainerEncounterReady) ready.push(JSON.parse(message.subarray(6).toString()));
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.TrainerEncounterNotify) { token = JSON.parse(message.subarray(6).toString()).encounterToken; return; }
        if (opcode === OpCodes.PokeBattleStartResponse) { droppedBattle = true; return; }
        if (opcode === OpCodes.GameplayStateResponse) snapshots.push(JSON.parse(message.subarray(6).toString()));
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  // Matched extractor catalog: MT_MOON_1F Youngster object 313 is at (12,16),
  // faces RIGHT with sight range 3. (15,17) and (15,16) are walkable tiles.
  await page.evaluate(async () => {
    const path = "/src/testing/capturequestTestBridge.ts";
    const { warpToMap } = await import(path);
    await warpToMap(59, 15, 17, "UP");
  });
  await waitForMap(page, "MT_MOON_1F"); await waitForNoMapLoading(page); await waitForPlayerTile(page, 15, 17);
  await pressMovement(page, "up"); await waitForPlayerTile(page, 15, 16);
  await expect.poll(() => token).not.toBe("");
  expect(ready).toHaveLength(0);
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  await expect.poll(() => snapshots.some(snapshot => snapshot.trainer?.encounterToken === token)).toBe(true);
  await expect.poll(() => droppedBattle).toBe(true);
  expect(ready).toHaveLength(1); expect(ready[0].encounterToken).toBe(token);
  expect((await getGameState(page)).battle.isOpen).toBe(false);
  await page.evaluate(async () => {
    const path = "/src/phaser-game/services/GameplayRecoveryService.ts";
    const { recoverGameplayState } = await import(path);
    await recoverGameplayState(59);
  });
  await expect(page.getByTestId("battle-action-fight")).toBeVisible();
  const saved = [...snapshots].reverse().find(snapshot => snapshot.battle)?.battle;
  expect(saved?.trainerClass).toBe("YOUNGSTER"); expect(saved?.revision).toBe(1);
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  await expect(page.getByTestId("battle-action-fight")).toBeVisible();
  expect([...snapshots].reverse().find(snapshot => snapshot.battle)?.battle?.battleId).toBe(saved?.battleId);
  expect(ready).toHaveLength(1);
  await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});

test("Safari battle and counters recover through the current-state read when legacy notifications are lost", async ({ page }) => {
  test.setTimeout(150000);
  const errors = collectPageErrors(page);
  const snapshots: GameplayStateResponse[] = [];
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => server.send(message));
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if ([OpCodes.SafariZoneEnterResponse, OpCodes.SafariZoneStepUpdate, OpCodes.SafariBattleStartNotify].includes(opcode)) return;
        if (opcode === OpCodes.GameplayStateResponse) snapshots.push(JSON.parse(message.subarray(6).toString()));
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_battle_run"); await waitForMap(page, "SAFARI_ZONE_CENTER"); await waitForNoMapLoading(page);
  await expect(page.getByTestId("battle-action-safari-ball")).toBeVisible();
  expect([...snapshots].reverse().find(snapshot => snapshot.safari)?.safari).toMatchObject({ active: true, ballsLeft: 30, stepsLeft: 499, pokemon: { id: 111 } });
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  await expect(page.getByTestId("battle-action-safari-ball")).toBeVisible();
  expect([...snapshots].reverse().find(snapshot => snapshot.safari)?.safari).toMatchObject({ ballsLeft: 30, stepsLeft: 499, pokemon: { id: 111 } });
  await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});
