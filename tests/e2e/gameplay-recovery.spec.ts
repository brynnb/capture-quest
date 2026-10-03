import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerTile } from "./helpers/state";
import type { GameplayStateResponse } from "../../src/net/generated/world_api";
import type { SafariBattleActionResponse } from "../../src/net/generated/world_api";
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

test("lost Safari run and dismissal replies recover without resending or reviving a closed encounter", async ({ page }) => {
  test.setTimeout(150000);
  const errors = collectPageErrors(page);
  const commands: Array<{ action: string; battle: { battleId: string; revision: number } }> = [];
  const snapshots: GameplayStateResponse[] = [];
  const releases: Array<() => void> = [];
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.SafariBattleActionRequest) commands.push(JSON.parse(message.subarray(6).toString()));
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.SafariBattleActionResponse) { releases.push(() => socket.send(message)); return; }
        if (opcode === OpCodes.GameplayStateResponse) snapshots.push(JSON.parse(message.subarray(6).toString()));
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_battle_run"); await waitForMap(page, "SAFARI_ZONE_CENTER"); await waitForNoMapLoading(page);
  await page.getByTestId("battle-action-run").click({ timeout: 10000 });
  await expect(page.getByText(/^Got away safely!\s*▼?$/)).toBeVisible({ timeout: 20000 });
  expect(commands.map(command => command.action)).toEqual(["run"]);
  const terminal = [...snapshots].reverse().find(snapshot => snapshot.safari?.isOver)?.safari;
  expect(terminal).toMatchObject({ active: true, ballsLeft: 30, stepsLeft: 499, revision: 2 });
  // These optional JSON flags omit false; Run is terminal without a wild flee.
  expect(Boolean(terminal?.caught)).toBe(false); expect(Boolean(terminal?.fled)).toBe(false);
  expect(terminal?.battleId).toBe(commands[0].battle.battleId);
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  await expect(page.getByText(/^Got away safely!\s*▼?$/)).toBeVisible();
  await page.keyboard.press("Space");
  await expect.poll(async () => (await getGameState(page)).battle.isOpen, { timeout: 20000 }).toBe(false);
  expect(commands.map(command => command.action)).toEqual(["run", "close"]);
  expect(commands[1].battle).toEqual({ battleId: terminal?.battleId, revision: 2 });
  for (const release of releases) release();
  await page.evaluate(async () => { const path = "/src/phaser-game/services/PlayerMovementService.ts"; const { readOwnedPlayerPosition } = await import(path); await readOwnedPlayerPosition(); });
  expect((await getGameState(page)).battle.isOpen).toBe(false);
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  expect((await getGameState(page)).battle.isOpen).toBe(false);
  expect(commands).toHaveLength(2);
  await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});

test("lost last-ball and close replies preserve the terminal encounter at the gate and present expiry", async ({ page }) => {
  test.setTimeout(150000);
  const errors = collectPageErrors(page);
  const commands: string[] = [];
  let outcome: SafariBattleActionResponse | undefined;
  const releases: Array<() => void> = [];
  await page.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.SafariBattleActionRequest) commands.push(JSON.parse(message.subarray(6).toString()).action);
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.SafariBattleActionResponse) {
        const response = JSON.parse(message.subarray(6).toString());
        if (!response.closed) outcome = response;
        releases.push(() => socket.send(message)); return;
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_last_ball_recovery"); await waitForMap(page, "SAFARI_ZONE_CENTER"); await waitForNoMapLoading(page);
  await page.getByTestId("battle-action-safari-ball").click({ timeout: 10000 });
  await expect.poll(() => !!outcome).toBe(true);
  expect(outcome).toMatchObject({ ballsLeft: 0, isOver: true, safariOver: true, position: { mapId: 156, x: 3, y: 4 } });
  await expect.poll(async () => (await getGameState(page)).map.id, { timeout: 25000 }).toBe(156); await waitForNoMapLoading(page);
  const summary = page.getByText(outcome!.caught ? /RHYHORN's data was added to the POKéDEX!/ : /^Got away safely!\s*▼?$/);
  await expect(summary).toBeVisible(); expect(commands).toEqual(["ball"]);
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  await expect(summary).toBeVisible(); await waitForPlayerTile(page, 3, 4);
  await page.keyboard.press("Space");
  await expect(page.getByText(/PA: Ding-dong! Your SAFARI GAME is over!/)).toBeVisible({ timeout: 20000 });
  expect(commands).toEqual(["ball", "close"]);
  for (const release of releases) release();
  await page.evaluate(async () => { const path = "/src/phaser-game/services/PlayerMovementService.ts"; const { readOwnedPlayerPosition } = await import(path); await readOwnedPlayerPosition(); });
  expect((await getGameState(page)).battle.isOpen).toBe(false); await waitForPlayerTile(page, 3, 4);
  await page.keyboard.press("Space"); await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});
