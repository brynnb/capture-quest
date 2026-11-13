import { expect, test, type Page } from "@playwright/test";
import type {
  GameCornerPrizeBuyRequest,
  GameCornerPrizeBuyResponse,
  GameCornerSlotPlayRequest,
  GameCornerSlotResultResponse,
} from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, pressMovement, pressSpace } from "./helpers/input";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { observeResponseDelivery } from "./helpers/responseDelivery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPCPokemon, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

type DurableState = {
  money: number;
  coins: number;
  revision: number;
  receipt: { result: unknown } | null;
  pokemon: Array<{ id: number; pokemon_id: number; box: number; box_slot: number }>;
};

async function readDurable(sql: (query: string) => Promise<string>, characterId: number): Promise<DurableState> {
  const encoded = await sql(`SELECT json_build_object(
    'money',COALESCE(w.pokedollars,0),'coins',COALESCE(c.coins,0),
    'revision',COALESCE((SELECT revision FROM character_shop_state WHERE character_id=${characterId}),0),
    'receipt',(SELECT result_json::json FROM character_shop_state WHERE character_id=${characterId}),
    'pokemon',(SELECT COALESCE(json_agg(p ORDER BY p.id),'[]'::json) FROM character_pokemon p WHERE character_id=${characterId})
  ) FROM character_data d
    LEFT JOIN character_wallet w ON w.character_id=d.id
    LEFT JOIN character_coins c ON c.character_id=d.id
    WHERE d.id=${characterId}`);
  // Wallet/coin rows are optional; the character itself must still exist.
  expect(encoded, `Missing durable fixture character ${characterId}`).not.toBe("");
  return JSON.parse(encoded);
}

async function openCabinet(page: Page, coins: number) {
  await waitForPlayerIdle(page);
  await clickTile(page, 18, 15);
  await expect(page.getByTestId("slot-machine")).toBeVisible();
  await expect(page.getByTestId("slot-machine-coins")).toHaveText(`COINS: ${coins}`, { timeout: 15_000 });
  await expect(page.getByTestId("slot-machine-spin")).toBeEnabled();
}

async function clickPrizeChoice(page: Page, text: string) {
  await expect.poll(() => page.evaluate(label => window.__capturequestTest?.overlayTextToViewport(label) ?? null, text)).not.toBeNull();
  const point = await page.evaluate(label => window.__capturequestTest!.overlayTextToViewport(label), text);
  expect(point).toBeTruthy();
  await page.mouse.click(point!.x, point!.y);
}

function requiresWebSocketCrashRunner() {
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true" || process.env.CQ_E2E_TRANSPORT === "native", "Requires the isolated WebSocket exact-process runner");
}

for (const loseReply of [false, true]) {
  test(`slot duplicate freezes one roll and current recovery never resends, lost reply=${loseReply}`, async ({ page }, testInfo) => {
    requiresWebSocketCrashRunner();
    test.setTimeout(120_000);
    const { sql, record } = await isolatedCrashRuntime();
    const errors = collectPageErrors(page);
    const commands: GameCornerSlotPlayRequest[] = [];
    const recoveryReads: string[] = [];
    const faults = await inventoryCommandFaults<GameCornerSlotResultResponse>(
      page, OpCodes.GameCornerSlotPlayRequest, OpCodes.GameCornerSlotResultResponse, loseReply,
      undefined, undefined,
      (opcode, message) => {
        if (!Buffer.isBuffer(message) || message.length < 6) return;
        const request = JSON.parse(message.subarray(6).toString());
        if (opcode === OpCodes.GameCornerSlotPlayRequest) commands.push(request);
        if (opcode === OpCodes.GameplayStateRequest && request.current && commands.length > 0) recoveryReads.push(request.requestId);
      },
    );
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "debug_game_corner_slots_ready");
    await waitForPlayerTile(page, 17, 15);
    const characterId = (await getGameState(page)).player.internalId!;
    expect(Number.isSafeInteger(characterId) && characterId > 0).toBe(true);
    await openCabinet(page, 10);
    const before = await readDurable(sql, characterId);
    await page.getByTestId("slot-machine-bet-3").click();
    await page.getByTestId("slot-machine-spin").click();
    await expect(page.getByTestId("slot-machine-spin")).toBeDisabled();
    await expect.poll(() => faults.replies.length).toBe(2);
    const result = faults.replies[0];
    const request = commands[0];
    expect(faults.replies[1]).toEqual(result);
    expect(result).toMatchObject({
      success: true, version: 1, kind: "slot", requestId: request.requestId, characterId,
      commandRevision: before.revision + 1, machineX: 18, machineY: 15, bet: 3,
    });
    expect(result).not.toHaveProperty("coins");
    expect(result).not.toHaveProperty("money");
    expect(request).toMatchObject({ command: { characterId, revision: before.revision }, fromX: 17, fromY: 15, machineX: 18, machineY: 15, bet: 3 });
    const expectedCoins = before.coins - 3 + result.payout;
    await expect(page.getByTestId("slot-machine-spin")).toBeEnabled({ timeout: 20_000 });
    await expect(page.getByTestId("slot-machine-coins")).toHaveText(`COINS: ${expectedCoins}`);
    await expect.poll(async () => (await getGameState(page)).inventory.coins).toBe(expectedCoins);
    if (loseReply) {
      await expect(page.getByTestId("slot-machine")).toContainText("The spin will not be repeated.");
      await expect(page.getByTestId("slot-machine-payout")).toHaveText("PAYOUT: 0");
    } else {
      await expect(page.getByTestId("slot-machine-payout")).toHaveText(`PAYOUT: ${result.payout}`);
    }
    const committed = await readDurable(sql, characterId);
    expect(committed).toMatchObject({ money: before.money, coins: expectedCoins, revision: before.revision + 1 });
    expect(committed.pokemon).toEqual(before.pokemon);
    expect(committed.receipt?.result).toEqual(result);
    expect(commands).toHaveLength(1);
    expect(faults.requests).toBe(1);
    expect(faults.successes).toBe(2);
    expect(faults.rejections).toBe(0);
    expect(recoveryReads.length).toBeGreaterThan(0);

    // Observe actual delivery of the old packet after a fresh cabinet read.
    // It must not revive the old animation or replace the current balance.
    await page.getByTestId("slot-machine-quit").click();
    await expect(page.getByTestId("slot-machine")).toBeHidden();
    await openCabinet(page, expectedCoins);
    let delivered = 0;
    await observeResponseDelivery(page, [OpCodes.GameCornerSlotResultResponse], (_opcode, id) => { if (id === result.requestId) delivered++; });
    faults.deliverReply(0);
    await expect.poll(() => delivered).toBe(1);
    await expect(page.getByTestId("slot-machine-spin")).toBeEnabled();
    await expect(page.getByTestId("slot-machine-payout")).toHaveText("PAYOUT: 0");
    await expect(page.getByTestId("slot-machine-coins")).toHaveText(`COINS: ${expectedCoins}`);
    expect(await readDurable(sql, characterId)).toEqual(committed);
    expect(commands).toHaveLength(1);
    await page.screenshot({ path: testInfo.outputPath(`slot-owned-recovery-${loseReply}.png`) });
    await page.getByTestId("slot-machine-quit").click();
    await quitToCharacterSelect(page);
    await enterWorld(page, character);
    await waitForNoMapLoading(page);
    await expect.poll(async () => (await getGameState(page)).inventory.coins).toBe(expectedCoins);
    expect(await readDurable(sql, characterId)).toEqual(committed);
    expect(commands).toHaveLength(1);
    await record({ family: "gamecorner-slot", loseReply, characterId, request, result, recoveryReads, committed });
    await quitToCharacterSelect(page);
    errors.assertNoSevereErrors();
  });
}

test("actual prize menu purchase survives a lost acknowledgement and process death without a second reward", async ({ page }, testInfo) => {
  requiresWebSocketCrashRunner();
  test.setTimeout(150_000);
  const { sql, crash, record } = await isolatedCrashRuntime();
  let errors = collectPageErrors(page);
  const commands: GameCornerPrizeBuyRequest[] = [];
  const recoveryReads: string[] = [];
  const faults = await inventoryCommandFaults<GameCornerPrizeBuyResponse>(
    page, OpCodes.GameCornerPrizeBuyRequest, OpCodes.GameCornerPrizeBuyResponse, true,
    undefined, undefined,
    (opcode, message) => {
      if (!Buffer.isBuffer(message) || message.length < 6) return;
      const request = JSON.parse(message.subarray(6).toString());
      if (opcode === OpCodes.GameCornerPrizeBuyRequest) commands.push(request);
      if (opcode === OpCodes.GameplayStateRequest && request.current && commands.length > 0) recoveryReads.push(request.requestId);
    },
  );
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_game_corner_prize_full_party_ready");
  await waitForPlayerTile(page, 2, 4);
  await waitForPlayerIdle(page);
  const characterId = (await getGameState(page)).player.internalId!;
  expect(Number.isSafeInteger(characterId) && characterId > 0).toBe(true);
  await expect.poll(async () => (await getGameState(page)).inventory.coins).toBe(9999);
  await pressMovement(page, "up");
  await waitForPlayerTile(page, 2, 3);
  await waitForPlayerIdle(page);
  await pressSpace(page);
  await expect.poll(() => page.evaluate(() => window.__capturequestTest?.overlayTextToViewport("ABRA - 180C") ?? null)).not.toBeNull();
  const before = await readDurable(sql, characterId);
  expect(before.pokemon.filter(pokemon => pokemon.box === -1)).toHaveLength(6);
  expect(before.pokemon.filter(pokemon => pokemon.box >= 0)).toHaveLength(1);
  expect(before.pokemon.filter(pokemon => pokemon.pokemon_id === 63)).toHaveLength(0);
  await page.screenshot({ path: testInfo.outputPath("prize-owned-choice.png") });
  await clickPrizeChoice(page, "ABRA - 180C");
  await expect.poll(() => faults.replies.length).toBe(2);
  const result = faults.replies[0];
  const request = commands[0];
  expect(faults.replies[1]).toEqual(result);
  expect(result).toMatchObject({
    success: true, version: 1, kind: "prize", requestId: request.requestId, characterId,
    commandRevision: before.revision + 1, prizeId: request.prizeId,
    prizeName: "ABRA", prizeLevel: 9, addedToParty: false, pcBox: 0, pcSlot: 1,
  });
  expect(result).not.toHaveProperty("coins");
  expect(request).toMatchObject({ command: { characterId, revision: before.revision }, fromX: 2, fromY: 3 });
  const committed = await readDurable(sql, characterId);
  expect(committed).toMatchObject({ money: before.money, coins: 9819, revision: before.revision + 1 });
  expect(committed.pokemon).toHaveLength(before.pokemon.length + 1);
  expect(committed.pokemon.filter(pokemon => before.pokemon.some(prior => prior.id === pokemon.id))).toEqual(before.pokemon);
  expect(committed.pokemon.filter(pokemon => pokemon.pokemon_id === 63)).toHaveLength(1);
  expect(committed.receipt?.result).toEqual(result);
  // Both successful acknowledgements are withheld. Kill the exact owned process
  // before the ten-second fallback read can teach this view the committed result.
  expect(recoveryReads).toHaveLength(0);
  expect((await getGameState(page)).inventory.coins).toBe(9999);
  expect(commands).toHaveLength(1);
  errors.assertNoSevereErrors();
  const receipt = await crash();
  await page.reload();
  errors = collectPageErrors(page);
  await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
  await enterWorld(page, character);
  await waitForNoMapLoading(page);
  await waitForPCPokemon(page, 63);
  await expect.poll(async () => (await getGameState(page)).inventory.coins).toBe(9819);
  expect((await getGameState(page)).pokemon.party.map(pokemon => pokemon.id)).toEqual([1, 4, 7, 25, 16, 19]);
  expect((await getGameState(page)).pokemon.pc.isOpen).toBe(false);
  expect(await readDurable(sql, characterId)).toEqual(committed);
  expect(commands).toHaveLength(1);

  // Explicitly retry the captured wire command after restart. This is a test
  // injection, distinct from the UI, which must never resend on its own.
  await page.evaluate(async ({ request, opcode }) => {
    const path = "/src/net/NetworkBridge.ts";
    const { NetworkBridge } = await import(path);
    await NetworkBridge.send(request, opcode);
  }, { request, opcode: OpCodes.GameCornerPrizeBuyRequest });
  await expect.poll(() => faults.replies.length).toBe(4);
  expect(faults.replies[2]).toEqual(result);
  expect(faults.replies[3]).toEqual(result);
  expect(commands).toHaveLength(2);
  expect(faults.requests).toBe(2);
  expect(faults.rejections).toBe(0);
  expect(await readDurable(sql, characterId)).toEqual(committed);

  let delivered = 0;
  await observeResponseDelivery(page, [OpCodes.GameCornerPrizeBuyResponse], (_opcode, id) => { if (id === request.requestId) delivered++; });
  faults.deliverReply(2);
  await expect.poll(() => delivered).toBe(1);
  expect((await getGameState(page)).dialogue.isOpen).toBe(false);
  expect((await getGameState(page)).worldInput.frozen).toBe(false);
  expect(await page.evaluate(() => window.__capturequestTest?.overlayTextToViewport("ABRA - 180C") ?? null)).toBeNull();
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await page.getByTestId("inventory-item-coin-case").click();
  await expect(page.getByText("You have 9819 coins.", { exact: true }).last()).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("prize-recovered-after-process-death.png") });
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  expect(await readDurable(sql, characterId)).toEqual(committed);
  await record({ family: "gamecorner-prize-unacknowledged", characterId, receipt, request, result, committed, uiCommandsBeforeReplay: 1 });
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
