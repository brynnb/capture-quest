import { catchSafariInRenderedUI } from "./helpers/battle";
import { expect, test } from "@playwright/test";
import type { GameplayStateResponse, SafariBattleActionResponse, SafariBattleActionRequest } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement, pressSpace, clickTile } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForPlayerTile, waitForPlayerIdle, waitForNoMapLoading } from "./helpers/state";

test("Safari gate entry commits the visit and shows the zone", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_zone_gate_entry_offer_yes");
  await waitForMap(page, "SAFARI_ZONE_GATE");
  await waitForPlayerTile(page, 4, 2);
  // A coordinate script triggers on stepping onto the tile, not fixture placement.
  await pressMovement(page, "down");
  await waitForPlayerTile(page, 4, 3);
  await waitForPlayerIdle(page);
  await pressMovement(page, "up");
  for (let i = 0; i < 30; i += 1) {
    const state = await getGameState(page);
    if (state.map.id === 220 && !state.dialogue.isOpen) break;
    await pressSpace(page);
    await page.waitForTimeout(150);
  }
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  await waitForPlayerTile(page, 14, 25);
  await page.screenshot({ path: test.info().outputPath("safari-entry.png") });
  errors.assertNoSevereErrors();
});

test("Safari battle run resolves through the real action menu", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_battle_run");
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  await expect(page.getByTestId("battle-action-safari-ball")).toBeVisible();
  await page.screenshot({ path: test.info().outputPath("safari-battle.png") });
  await page.getByTestId("battle-action-run").click();
  for (let i = 0; i < 20; i += 1) {
    if (!(await getGameState(page)).battle.isOpen) break;
    await pressSpace(page);
    await page.waitForTimeout(150);
  }
  await expect.poll(async () => (await getGameState(page)).battle.isOpen).toBe(false);
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  errors.assertNoSevereErrors();
});

test("Safari step exhaustion returns the player to the gate", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "safari_step_expiry");
  await waitForMap(page, "SAFARI_ZONE_CENTER");
  await waitForPlayerTile(page, 14, 24);
  await pressMovement(page, "up");
  await expect.poll(async () => (await getGameState(page)).dialogue.text, { timeout: 15_000 }).toMatch(/SAFARI GAME is over/);
  await page.screenshot({ path: test.info().outputPath("safari-expiry.png") });
  await pressSpace(page, 4);
  await waitForMap(page, "SAFARI_ZONE_GATE");
  await waitForPlayerTile(page, 3, 4);
  errors.assertNoSevereErrors();
});

for (const partySize of [1, 6]) {
  test(`lost Safari capture reply recovers party/PC placement and one catch with party size ${partySize}`, async ({ page }) => {
    test.setTimeout(180000);
    const errors = collectPageErrors(page);
    const commands: SafariBattleActionRequest[] = [];
    const snapshots: GameplayStateResponse[] = [];
    let replies = 0;
    let captured: SafariBattleActionResponse | undefined;
    let release: (() => void) | undefined;
    await page.routeWebSocket("**/ws", socket => {
      const server = socket.connectToServer();
      socket.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.SafariBattleActionRequest) commands.push(JSON.parse(message.subarray(6).toString()));
        server.send(message);
      });
      server.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          const opcode = message.readUInt16LE(4);
          if (opcode === OpCodes.GameplayStateResponse) snapshots.push(JSON.parse(message.subarray(6).toString()));
          if (opcode === OpCodes.SafariBattleActionResponse) {
            const response: SafariBattleActionResponse = JSON.parse(message.subarray(6).toString());
            replies++;
            if (response.caught && !response.closed && !captured) { captured = response; release = () => socket.send(message); return; }
          }
        }
        socket.send(message);
      });
    });
    const character = await createGuestCharacterAndEnterWorld(page);
    await catchSafariInRenderedUI(page, partySize, () => replies, () => !!captured);
    expect(captured).toBeDefined();
    const settledCount = commands.length;
    const captureCommand = commands.at(-1)!;
    expect(captured!.battleId).toBe(captureCommand.battle.battleId);
    expect(captured!.revision).toBe(captureCommand.battle.revision + 1);
    const summary = page.getByText(partySize === 6 ? /MAGIKARP was transferred to\s+Bill's PC \(BOX 1\)\./ : /MAGIKARP's data was added to the POKéDEX!/);
    await expect(summary).toBeVisible({ timeout: 20000 });
    expect(commands).toHaveLength(settledCount);
    const recovered = [...snapshots].reverse().find(snapshot => snapshot.safari?.caught)?.safari;
    expect(recovered).toMatchObject({ battleId: captured!.battleId, revision: captured!.revision, caught: true, ballsLeft: captured!.ballsLeft });
    expect(Boolean(recovered?.sentToPC)).toBe(partySize === 6);
    if (partySize === 6) expect(recovered?.pcBox).toBe(1);
    const party = (await getGameState(page)).pokemon.party;
    expect(party).toHaveLength(partySize === 6 ? 6 : 2);
    await page.screenshot({ path: test.info().outputPath(`safari-capture-${partySize}.png`) });
    await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
    await expect(summary).toBeVisible(); expect(commands).toHaveLength(settledCount);
    expect((await getGameState(page)).pokemon.party).toEqual(party);
    await pressSpace(page); await expect.poll(async () => (await getGameState(page)).battle.isOpen).toBe(false);
    expect(commands).toHaveLength(settledCount + 1); expect(commands.at(-1)?.action).toBe("close");
    release!();
    await page.evaluate(async () => { const path = "/src/phaser-game/services/PlayerMovementService.ts"; const { readOwnedPlayerPosition } = await import(path); await readOwnedPlayerPosition(); });
    expect((await getGameState(page)).battle.isOpen).toBe(false);
    await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
    expect((await getGameState(page)).pokemon.party).toEqual(party);
    if (partySize === 6) {
      // Matched catalog: map 41 is VIRIDIAN_POKECENTER; this explicit test warp
      // preserves earned rows and ends the visit, unlike resetting a fixture.
      await page.evaluate(async () => { const path = "/src/testing/capturequestTestBridge.ts"; const { warpToMap } = await import(path); await warpToMap(41, 12, 4, "RIGHT"); });
      await waitForMap(page, "VIRIDIAN_POKECENTER"); await waitForNoMapLoading(page);
      await clickTile(page, 13, 4); await expect(page.getByTestId("pokemon-pc-main-menu")).toBeVisible({ timeout: 15000 });
      await page.getByTestId("pokemon-pc-bills-pc").click();
      await expect.poll(async () => (await getGameState(page)).pokemon.pc.boxPokemon.filter(pokemon => pokemon.id === 129).length).toBe(1);
      await page.screenshot({ path: test.info().outputPath("safari-pc-caught-row.png") });
      await page.getByTestId("pokemon-pc-close").click({ timeout: 10000 });
    } else expect(party.filter(pokemon => pokemon.id === 129)).toHaveLength(1);
    expect(commands).toHaveLength(settledCount + 1);
    await quitToCharacterSelect(page); errors.assertNoSevereErrors();
  });
}
