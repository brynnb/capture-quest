import { expect, test, type Page } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, pressSpace } from "./helpers/input";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerTile } from "./helpers/state";

const clerkScript = "GameCornerClerk1BuyCoins";
type Completion = { scriptLabel: string; completionToken: string; requestId: string; cancel?: boolean };
type CompletionReply = { requestId: string; success: boolean; completed: boolean; replayed: boolean };

async function advanceClerk(page: Page, untilChoice = false) {
  for (let attempt = 0; attempt < 50; attempt++) {
    const state = await getGameState(page);
    if (untilChoice && state.dialogue.isChoicePending) return;
    if (!untilChoice && !state.worldInput.frozen && !state.dialogue.isOpen) return;
    if (state.dialogue.isOpen && !state.dialogue.isChoicePending) await pressSpace(page);
    else await page.waitForTimeout(100);
  }
  throw new Error(`Coin clerk did not reach ${untilChoice ? "choice" : "completion"}`);
}

async function displayedBalances(page: Page) {
  const { inventory } = await getGameState(page);
  return { money: inventory.money, coins: inventory.coins };
}

for (const loseReply of [false, true]) {
  test(`coin clerk uses its scripted receipt once, lost reply=${loseReply}`, async ({ page }, testInfo) => {
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true" || process.env.CQ_E2E_TRANSPORT === "native", "Requires the isolated WebSocket crash runner");
    test.setTimeout(150_000);
    const { sql, crash, record } = await isolatedCrashRuntime();
    let errors = collectPageErrors(page);
    const commands: Completion[] = [];
    const replies: CompletionReply[] = [];
    let legacyPurchases = 0;
    let heldSuccess = false;
    await page.routeWebSocket("**/ws", socket => {
      const server = socket.connectToServer();
      socket.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          const opcode = message.readUInt16LE(4);
          if (opcode === OpCodes.GameCornerBuyCoinsRequest) legacyPurchases++;
          if (opcode === OpCodes.CutsceneEndRequest) {
            const command = JSON.parse(message.subarray(6).toString()) as Completion;
            if (command.scriptLabel === clerkScript) {
              commands.push(command);
              if (!command.cancel) server.send(message);
            }
          }
        }
        server.send(message);
      });
      server.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.CutsceneEndResponse) {
          const reply = JSON.parse(message.subarray(6).toString()) as CompletionReply;
          if (commands.some(command => command.requestId === reply.requestId)) {
            replies.push(reply);
            if (loseReply && reply.success && reply.completed) { heldSuccess = true; return; }
          }
        }
        socket.send(message);
      });
    });
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "debug_game_corner_buy_coins_ready");
    await waitForPlayerTile(page, 5, 8);
    const characterId = (await getGameState(page)).player.internalId!;
    expect(Number.isSafeInteger(characterId) && characterId > 0).toBe(true);
    const savedBalances = async () => JSON.parse(await sql(`SELECT json_build_object('money',w.pokedollars,'coins',c.coins) FROM character_wallet w JOIN character_coins c ON c.character_id=w.character_id WHERE w.character_id=${characterId}`));
    await expect.poll(() => displayedBalances(page)).toEqual({ money: 1000, coins: 10 });

    await clickTile(page, 5, 6); await advanceClerk(page, true);
    await page.getByTestId("dialogue-choice-no").click(); await advanceClerk(page);
    expect(await savedBalances()).toEqual({ money: 1000, coins: 10 });
    expect(replies.at(-1)).toMatchObject({ success: true, completed: false });

    await clickTile(page, 5, 6); await advanceClerk(page, true);
    await page.screenshot({ path: testInfo.outputPath("coin-clerk-choice.png") });
    await page.getByTestId("dialogue-choice-yes").click();
    if (loseReply) {
      for (let attempt = 0; attempt < 40 && !heldSuccess; attempt++) {
        const state = await getGameState(page);
        if (state.dialogue.isOpen && !state.dialogue.isChoicePending) await pressSpace(page);
        else await page.waitForTimeout(100);
      }
      await expect.poll(() => heldSuccess).toBe(true);
      expect(await savedBalances()).toEqual({ money: 0, coins: 60 });
      errors.assertNoSevereErrors();
      const receipt = await crash();
      await record({ family: "coin-clerk", characterId, receipt, completionToken: commands.at(-1)?.completionToken });
      await page.reload();
      await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
      await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
      errors = collectPageErrors(page);
      await enterWorld(page, character); await waitForNoMapLoading(page);
    } else {
      await advanceClerk(page);
      await expect.poll(() => replies.some(reply => reply.success && reply.completed && reply.replayed)).toBe(true);
    }
    await expect.poll(() => displayedBalances(page)).toEqual({ money: 0, coins: 60 });
    expect(await savedBalances()).toEqual({ money: 0, coins: 60 });
    expect(legacyPurchases).toBe(0);
    const purchase = commands.find(command => !command.cancel)!;
    expect(purchase.completionToken).toBeTruthy();
    await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
    await page.evaluate(({ completionToken, scriptLabel, opcode }) => {
      const path = "/src/net/NetworkBridge.ts";
      return import(path).then(({ NetworkBridge }) => NetworkBridge.send({ completionToken, scriptLabel, requestId: "coin-replay-after-reentry" }, opcode));
    }, { completionToken: purchase.completionToken, scriptLabel: clerkScript, opcode: OpCodes.CutsceneEndRequest });
    await expect.poll(() => replies.find(reply => reply.requestId === "coin-replay-after-reentry")?.replayed).toBe(true);
    expect(await savedBalances()).toEqual({ money: 0, coins: 60 });
    await expect.poll(() => displayedBalances(page)).toEqual({ money: 0, coins: 60 });
    await page.getByRole("button", { name: "Bag", exact: true }).click();
    await page.getByTestId("inventory-item-coin-case").click();
    await expect(page.getByText("You have 60 coins.", { exact: true }).last()).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("coin-clerk-recovered.png") });
    await page.getByRole("button", { name: "Bag", exact: true }).click();
    await quitToCharacterSelect(page);
    errors.assertNoSevereErrors();
  });
}

// Keep the real native QUIC transport, withholding only the decoded completion
// from application delivery. The private runner then kills the committed server
// before the client can learn the outcome through its missing acknowledgement.
test("native coin clerk recovers an unacknowledged purchase after process death", async ({ page }, testInfo) => {
  test.skip(process.env.CQ_E2E_TRANSPORT !== "native" || process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the native exact-process runner");
  test.setTimeout(150_000);
  const { sql, crash, record } = await isolatedCrashRuntime();
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "debug_game_corner_buy_coins_ready");
  await waitForPlayerTile(page, 5, 8);
  const characterId = (await getGameState(page)).player.internalId!;
  expect(Number.isSafeInteger(characterId) && characterId > 0).toBe(true);
  const before = await page.evaluate(async ({ requestOpcode, responseOpcode, script }) => {
    const netPath = "/src/net/index.ts", bridgePath = "/src/net/NetworkBridge.ts";
    const { WorldSocket } = await import(netPath);
    const { NetworkBridge } = await import(bridgePath);
    const socket = WorldSocket as { isConnected: boolean; useWebSocket: boolean; webtransport: unknown; onJson: ((opcode: number, payload: unknown) => void) | null };
    const original = socket.onJson;
    if (typeof original !== "function") throw new Error("Native application dispatcher missing");
    const evidence = { held: 0, requests: 0, completionToken: "" };
    (window as unknown as { __nativeCoinFault: typeof evidence }).__nativeCoinFault = evidence;
    const ids = new Set<string>();
    const send = NetworkBridge.send.bind(NetworkBridge);
    NetworkBridge.send = (data: Record<string, unknown>, opcode: number) => {
      if (opcode === requestOpcode && data.scriptLabel === script && !data.cancel) {
        ids.add(String(data.requestId)); evidence.requests++; evidence.completionToken = String(data.completionToken);
      }
      return send(data, opcode);
    };
    socket.onJson = (opcode, payload) => {
      if (opcode === responseOpcode) {
        const reply = payload as CompletionReply;
        if (ids.has(reply.requestId) && reply.success && reply.completed) { evidence.held++; return; }
      }
      original(opcode, payload);
    };
    return { connected: socket.isConnected, websocket: socket.useWebSocket, native: Boolean(socket.webtransport) };
  }, { requestOpcode: OpCodes.CutsceneEndRequest, responseOpcode: OpCodes.CutsceneEndResponse, script: clerkScript });
  expect(before).toEqual({ connected: true, websocket: false, native: true });
  const fault = () => page.evaluate(() => (window as unknown as { __nativeCoinFault: { held: number; requests: number; completionToken: string } }).__nativeCoinFault);
  await clickTile(page, 5, 6); await advanceClerk(page, true);
  await page.getByTestId("dialogue-choice-yes").click();
  for (let attempt = 0; attempt < 40 && (await fault()).held === 0; attempt++) {
    const state = await getGameState(page);
    if (state.dialogue.isOpen && !state.dialogue.isChoicePending) await pressSpace(page);
    else await page.waitForTimeout(100);
  }
  await expect.poll(async () => (await fault()).held).toBeGreaterThan(0);
  const evidence = await fault();
  expect(await sql(`SELECT w.pokedollars::text||','||c.coins::text FROM character_wallet w JOIN character_coins c ON c.character_id=w.character_id WHERE w.character_id=${characterId}`)).toBe("0,60");
  const receipt = await crash();
  await page.reload(); await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
  await enterWorld(page, character); await waitForNoMapLoading(page);
  const after = await page.evaluate(async () => {
    const path = "/src/net/index.ts"; const { WorldSocket } = await import(path);
    const socket = WorldSocket as { isConnected: boolean; useWebSocket: boolean; webtransport: unknown };
    return { connected: socket.isConnected, websocket: socket.useWebSocket, native: Boolean(socket.webtransport) };
  });
  expect(after).toEqual({ connected: true, websocket: false, native: true });
  await expect.poll(() => displayedBalances(page)).toEqual({ money: 0, coins: 60 });
  expect(await sql(`SELECT w.pokedollars::text||','||c.coins::text FROM character_wallet w JOIN character_coins c ON c.character_id=w.character_id WHERE w.character_id=${characterId}`)).toBe("0,60");
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await page.getByTestId("inventory-item-coin-case").click();
  await expect(page.getByText("You have 60 coins.", { exact: true }).last()).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("native-coin-clerk-recovered.png") });
  await record({ family: "native-coin-clerk-unacknowledged", characterId, before, after, evidence, receipt });
  await page.getByRole("button", { name: "Bag", exact: true }).click(); await quitToCharacterSelect(page);
});
