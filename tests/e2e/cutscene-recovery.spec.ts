import { expect, test, type Page } from "@playwright/test";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement, pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";

async function finishDialogue(page: Page) {
  for (let i = 0; i < 60; i++) {
    const state = await getGameState(page);
    if (!state.worldInput.frozen && !state.dialogue.isOpen && !state.dialogue.isChoicePending) return;
    if (state.dialogue.isOpen || state.dialogue.isChoicePending) await pressSpace(page);
    else await page.waitForTimeout(200);
  }
  await expect.poll(async () => (await getGameState(page)).worldInput.frozen, { timeout: 20000 }).toBe(false);
}

test("lost cutscene delivery resumes after re-entry; lost completion retries without rewinding a later position", async ({ page }) => {
  test.setTimeout(180000);
  const errors = collectPageErrors(page);
  const label = "OaksLabChooseStarterIntro";
  const starts: Array<{ scriptLabel: string; completionToken: string }> = [];
  const completions: Array<{ scriptLabel: string; completionToken: string; requestId: string }> = [];
  const replies: Array<{ requestId: string; success: boolean; completed: boolean; replayed: boolean; x: number; y: number }> = [];
  let droppedStart = false, droppedReply = false;
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    socket.onMessage((message) => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.CutsceneEndRequest) {
        const request = JSON.parse(message.subarray(6).toString());
        if (request.scriptLabel === label) completions.push(request);
      }
      server.send(message);
    });
    server.onMessage((message) => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.CutsceneStartNotify) {
          const start = JSON.parse(message.subarray(6).toString());
          if (start.scriptLabel === label) {
            starts.push(start);
            if (!droppedStart) { droppedStart = true; return; }
          }
        }
        if (opcode === OpCodes.CutsceneEndResponse) {
          const reply = JSON.parse(message.subarray(6).toString());
          if (completions.some(request => request.requestId === reply.requestId)) {
            if (reply.success && !droppedReply) { droppedReply = true; return; }
            replies.push(reply);
          }
        }
      }
      socket.send(message);
    });
  });
  const character = await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page, "oak_lab_choose_starter_intro");
  await waitForMap(page, "OAKS_LAB"); await waitForNoMapLoading(page);
  await expect.poll(() => droppedStart).toBe(true);
  await waitForPlayerTile(page, 5, 11);
  await quitToCharacterSelect(page); await enterWorld(page, character);
  await waitForNoMapLoading(page);
  await expect.poll(() => starts.length).toBeGreaterThanOrEqual(2);
  expect(starts[1].completionToken).toBe(starts[0].completionToken);
  await finishDialogue(page);
  await expect.poll(() => completions.length, { timeout: 25000 }).toBe(2);
  expect(droppedReply).toBe(true);
  expect(completions[1].completionToken).toBe(completions[0].completionToken);
  expect(completions[1].requestId).not.toBe(completions[0].requestId);
  await expect.poll(() => replies[0]?.replayed).toBe(true);
  expect(replies[0].completed).toBe(true);
  await waitForPlayerIdle(page); await waitForPlayerTile(page, 5, 3);
  await pressMovement(page, "down"); await waitForPlayerTile(page, 5, 4);
  await quitToCharacterSelect(page); await enterWorld(page, character);
  await waitForNoMapLoading(page); await waitForPlayerIdle(page);
  await page.evaluate(async ({ completionToken, scriptLabel }) => {
    const bridgePath = "/src/net/NetworkBridge.ts", opcodePath = "/src/net/generated/opcodes.ts";
    const { NetworkBridge } = await import(bridgePath);
    const opcodes = await import(opcodePath);
    NetworkBridge.send({ completionToken, scriptLabel, requestId: "after-reentry" }, opcodes.CutsceneEndRequest);
  }, { completionToken: starts[0].completionToken, scriptLabel: label });
  await expect.poll(() => replies.find(reply => reply.requestId === "after-reentry")?.replayed).toBe(true);
  expect(replies.find(reply => reply.requestId === "after-reentry")).toMatchObject({ completed: true, x: 5, y: 4 });
  await waitForPlayerTile(page, 5, 4);
  await quitToCharacterSelect(page);
  errors.assertNoSevereErrors();
});
