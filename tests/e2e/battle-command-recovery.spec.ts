import { expect, test } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { advanceBattleTextToPhase, waitForBattleOpen, endWildBattleIfOpen } from "./helpers/battle";
import { collectPageErrors } from "./helpers/errors";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading } from "./helpers/state";
import { clickTile, pressSpace } from "./helpers/input";
import * as OpCodes from "../../src/net/generated/opcodes";

for (const partySize of [1, 6]) {
  test(`lost capture reply restores the summary and one caught row with party size ${partySize}`, async ({ page }) => {
    test.setTimeout(150000);
    const errors = collectPageErrors(page);
    let actions = 0, closes = 0;
    let release: (() => void) | undefined;
    await page.routeWebSocket("**/ws", socket => {
      const server = socket.connectToServer();
      socket.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          if (message.readUInt16LE(4) === OpCodes.PokeBattleActionRequest) actions++;
          if (message.readUInt16LE(4) === OpCodes.PokeBattleCloseRequest) closes++;
        }
        server.send(message);
      });
      server.onMessage(message => {
      // Consumption/party recovery must come from the correlated snapshot,
      // even if the independent state notifications were also lost.
      if (actions > 0 && Buffer.isBuffer(message) && message.length >= 6
        && [OpCodes.CQInventoryResponse, OpCodes.CharacterWallet, OpCodes.PokemonPartyResponse].includes(message.readUInt16LE(4))) return;
        if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PokeBattleActionResponse && !release) {
          release = () => socket.send(message); return;
        }
        socket.send(message);
      });
    });
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, `active_battle_fixture_capture_recovery_${partySize}`);
    await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
    await page.getByTestId("battle-action-item").click({ timeout: 10000 });
    await page.getByTestId("battle-item-0").click({ timeout: 10000 });
    await expect.poll(() => !!release).toBe(true);
    const summary = page.getByText(partySize === 6 ? /MAGIKARP was transferred to\s+Bill's PC \(BOX 1\)\./ : /MAGIKARP's data was added to the POKéDEX!/);
    await expect(summary).toBeVisible({ timeout: 20000 });
    expect(actions).toBe(1); expect(closes).toBe(0);
    expect((await getGameState(page)).inventory.items.filter(item => item.shortName === "MASTER_BALL")).toHaveLength(0);
    expect((await getGameState(page)).pokemon.party).toHaveLength(partySize === 6 ? 6 : 2);
    // Reentry must preserve the summary, not dismiss a catch as an ordinary win.
    await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
    await expect(summary).toBeVisible(); expect(closes).toBe(0);
    await pressSpace(page);
    await expect.poll(async () => (await getGameState(page)).battle.isOpen).toBe(false);
    expect(closes).toBe(1);
    const settled = (await getGameState(page)).pokemon.party;
    release!();
    await page.evaluate(async () => { const path = "/src/phaser-game/services/PlayerMovementService.ts"; const { readOwnedPlayerPosition } = await import(path); await readOwnedPlayerPosition(); });
    expect((await getGameState(page)).battle.isOpen).toBe(false);
    await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
    expect((await getGameState(page)).pokemon.party).toEqual(settled);
    expect((await getGameState(page)).inventory.items.filter(item => item.shortName === "MASTER_BALL")).toHaveLength(0);
    if (partySize === 6) {
      await clickTile(page, 13, 4);
      await expect(page.getByTestId("pokemon-pc-main-menu")).toBeVisible({ timeout: 15000 });
      await page.getByTestId("pokemon-pc-bills-pc").click();
      await expect.poll(async () => (await getGameState(page)).pokemon.pc.boxPokemon.filter(pokemon => pokemon.id === 129).length).toBe(1);
      await page.getByTestId("pokemon-pc-close").click({ timeout: 10000 });
    } else expect(settled.filter(pokemon => pokemon.id === 129)).toHaveLength(1);
    expect(actions).toBe(1); expect(closes).toBe(1);
    await quitToCharacterSelect(page); errors.assertNoSevereErrors();
  });
}

for (const choice of ["learn", "skip"] as const) {
  test(`a pending level-up survives reentry and a lost ${choice} reply settles only once`, async ({ page }) => {
    test.setTimeout(180000);
    const errors = collectPageErrors(page);
    let actions = 0, learning = 0, closes = 0, replies = 0;
    let pending = false;
    const releases: (() => void)[] = [];
    await page.routeWebSocket("**/ws", socket => {
      const server = socket.connectToServer();
      socket.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          const opcode = message.readUInt16LE(4);
          if (opcode === OpCodes.PokeBattleActionRequest) actions++;
          if (opcode === OpCodes.PokeMoveLearnRequest) learning++;
          if (opcode === OpCodes.PokeBattleCloseRequest) closes++;
        }
        server.send(message);
      });
      server.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          const opcode = message.readUInt16LE(4);
          if (opcode === OpCodes.PokeBattleActionResponse) {
            const response = JSON.parse(message.subarray(6).toString()); replies++;
            if (response.success && response.battle?.pendingMove) {
              pending = true; releases.push(() => socket.send(message)); return;
            }
          }
          if (opcode === OpCodes.PokeMoveLearnResponse) { releases.push(() => socket.send(message)); return; }
        }
        socket.send(message);
      });
    });
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "active_battle_fixture_learning_recovery");
    await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
    const initial = (await getGameState(page)).pokemon.party[0];
    expect(initial.level).toBe(6); expect(initial.moves).toHaveLength(4);
    for (let turn = 0; turn < 8 && !pending; turn++) {
      const before = replies;
      await page.getByTestId("battle-action-fight").click(); await page.getByTestId("battle-move-0").click();
      await expect.poll(() => replies > before).toBe(true);
      if (!pending) {
        await expect(page.getByText("Waiting for the battle…", { exact: true })).toBeHidden();
        await advanceBattleTextToPhase(page, "action_select");
      }
    }
    expect(pending).toBe(true);
    await expect(page.getByText("Forget a move?", { exact: true })).toBeVisible({ timeout: 20000 });
    await expect(page.getByText("Trying to learn LEECH_SEED", { exact: true })).toBeVisible();
    expect(closes).toBe(0); expect(learning).toBe(0);
    const pendingParty = (await getGameState(page)).pokemon.party;
    expect(pendingParty[0].level).toBe(7);
    expect(pendingParty[0].exp).toBe(initial.exp + 72);
    const actionCount = actions;
    await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
    await expect(page.getByText("Forget a move?", { exact: true })).toBeVisible();
    expect((await getGameState(page)).pokemon.party).toEqual(pendingParty);
    expect(actions).toBe(actionCount); expect(closes).toBe(0);
    if (choice === "learn") await page.getByRole("button", { name: /^> TACKLE PP / }).click({ timeout: 10000 });
    else await page.getByRole("button", { name: "> Don't learn LEECH_SEED", exact: true }).click({ timeout: 10000 });
    await expect.poll(() => learning).toBe(1);
    await expect.poll(async () => (await getGameState(page)).battle.isOpen, { timeout: 20000 }).toBe(false);
    expect(closes).toBe(1); expect(actions).toBe(actionCount);
    const settled = (await getGameState(page)).pokemon.party;
    expect(settled[0].exp).toBe(pendingParty[0].exp);
    expect(settled[0].moves.map(move => move.id)).toEqual(choice === "learn" ? [75, 73, 45, 22] : [75, 33, 45, 22]);
    for (const release of releases) release();
    await page.evaluate(async () => { const path = "/src/phaser-game/services/PlayerMovementService.ts"; const { readOwnedPlayerPosition } = await import(path); await readOwnedPlayerPosition(); });
    expect((await getGameState(page)).battle.isOpen).toBe(false);
    await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
    expect((await getGameState(page)).pokemon.party).toEqual(settled);
    expect(learning).toBe(1); expect(closes).toBe(1); expect(actions).toBe(actionCount);
    await quitToCharacterSelect(page); errors.assertNoSevereErrors();
  });
}

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

test("a lost blackout reply changes maps, heals once and dismisses through the destination scene", async ({ page }) => {
  test.setTimeout(240000);
  const errors = collectPageErrors(page);
  let actions = 0, closes = 0, replies = 0;
  let blackout: { mapId: number; x: number; y: number; money: number } | undefined;
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
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PokeBattleActionResponse) {
        const response = JSON.parse(message.subarray(6).toString()); replies++;
        if (response.success && response.end?.blackout) {
          blackout = { mapId: response.position.mapId, x: response.position.x, y: response.position.y, money: response.end.money };
          release = () => socket.send(message); return;
        }
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "active_battle_fixture_blackout_recovery");
  const sourceMap = (await getGameState(page)).map.id;
  await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
  for (let turn = 0; turn < 8 && !blackout; turn++) {
    const before = replies;
    await page.getByTestId("battle-action-fight").click(); await page.getByTestId("battle-move-0").click();
    await expect.poll(() => replies > before).toBe(true);
    if (!blackout) {
      await expect(page.getByText("Waiting for the battle…", { exact: true })).toBeHidden();
      await advanceBattleTextToPhase(page, "action_select");
    }
  }
  expect(blackout).toBeDefined(); expect(blackout!.mapId).not.toBe(sourceMap); expect(blackout!.money).toBe(499);
  const actionCount = actions;
  await expect.poll(async () => (await getGameState(page)).map.id, { timeout: 30000 }).toBe(blackout!.mapId);
  await waitForNoMapLoading(page);
  await expect.poll(async () => (await getGameState(page)).battle.isOpen, { timeout: 20000 }).toBe(false);
  expect(closes).toBe(1); expect(actions).toBe(actionCount);
  const settled = await getGameState(page);
  expect(settled.player).toMatchObject({ x: blackout!.x, y: blackout!.y });
  expect(settled.inventory.money).toBe(499);
  expect(settled.pokemon.party.every(pokemon => pokemon.curHp === pokemon.maxHp)).toBe(true);
  release!();
  await page.evaluate(async () => { const path = "/src/phaser-game/services/PlayerMovementService.ts"; const { readOwnedPlayerPosition } = await import(path); await readOwnedPlayerPosition(); });
  expect((await getGameState(page)).battle.isOpen).toBe(false);
  await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
  const reentered = await getGameState(page);
  expect(reentered.inventory.money).toBe(499); expect(reentered.pokemon.party).toEqual(settled.pokemon.party);
  expect(reentered.map.id).toBe(blackout!.mapId); expect(actions).toBe(actionCount); expect(closes).toBe(1);
  await quitToCharacterSelect(page); errors.assertNoSevereErrors();
});
