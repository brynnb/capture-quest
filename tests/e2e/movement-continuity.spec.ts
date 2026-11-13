import { expect, test, type Page } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createCharacter, enterWorld, loginAsGuest, quitToCharacterSelect, uniqueTrainerName } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile, waitForWarpMode } from "./helpers/state";

type PositionSample = { t: number; x: number; y: number };

async function startSampling(page: Page) {
  const playerId = (await getGameState(page)).player.id;
  await page.evaluate((id) => {
    const state = (window as any).__movementContinuity;
    const sprite = state.renderer?.getActorSprite(id);
    if (!sprite) throw new Error("The rendered player sprite is missing");
    state.sprite = sprite;
    state.samples = [];
    state.recording = true;
    const sample = (t: number) => {
      if (!state.recording) return;
      state.samples.push({ t, x: sprite.x, y: sprite.y });
      requestAnimationFrame(sample);
    };
    requestAnimationFrame(sample);
  }, playerId);
}

async function stopSampling(page: Page): Promise<PositionSample[]> {
  return page.evaluate(() => {
    const state = (window as any).__movementContinuity;
    state.recording = false;
    return state.samples;
  });
}

function longestPauseWhileTraveling(samples: PositionSample[]) {
  const changed = samples.filter((sample, index) => {
    if (index === 0) return false;
    const previous = samples[index - 1];
    return Math.abs(sample.x - previous.x) + Math.abs(sample.y - previous.y) > 0.01;
  });
  expect(changed.length, "The real sprite must animate across multiple tiles").toBeGreaterThan(15);
  let longest = 0;
  for (let index = 1; index < changed.length; index += 1) {
    longest = Math.max(longest, changed[index].t - changed[index - 1].t);
  }
  return longest;
}

test("click paths and held WASD walk continuously across server round trips", async ({ page }) => {
  test.setTimeout(120_000);
  const errors = collectPageErrors(page);
  const packets: Array<{ direction: string; opcode: number; data: any }> = [];
  const delayed = new Set<NodeJS.Timeout>();
  await page.routeWebSocket("**/ws", (socket) => {
    const server = socket.connectToServer();
    socket.onMessage((message) => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if ([OpCodes.PlayerStepRequest, OpCodes.PlayerStepCompleteRequest].includes(opcode)) {
          packets.push({ direction: "sent", opcode, data: JSON.parse(message.subarray(6).toString()) });
        }
      }
      server.send(message);
    });
    server.onMessage((message) => {
      const opcode = Buffer.isBuffer(message) && message.length >= 6 ? message.readUInt16LE(4) : -1;
      if ([OpCodes.PlayerStepResponse, OpCodes.PlayerStepCompleteResponse].includes(opcode)) {
        packets.push({ direction: "received", opcode, data: JSON.parse((message as Buffer).subarray(6).toString()) });
        // A 125 ms response journey makes a serialized two-round-trip pause
        // unmistakable, while remaining below the ordinary 200 ms walk duration.
        const timer = setTimeout(() => {
          delayed.delete(timer);
          socket.send(message);
        }, 125);
        delayed.add(timer);
      } else {
        socket.send(message);
      }
    });
    socket.onClose(() => {
      for (const timer of delayed) clearTimeout(timer);
      delayed.clear();
    });
  });
  await page.addInitScript(() => {
    (window as any).__movementContinuity = {};
  });
  await loginAsGuest(page);
  const characterName = uniqueTrainerName();
  await createCharacter(page, characterName);
  await page.evaluate(async () => {
    const path = "/src/phaser-game/renderers/MapRenderer.ts";
    const { MapRenderer } = await import(path);
    const render = MapRenderer.prototype.renderMap;
    MapRenderer.prototype.renderMap = function (...args: any[]) {
      (window as any).__movementContinuity.renderer = this;
      return render.apply(this, args as any);
    };
  });
  await enterWorld(page, characterName);
  await jumpToScenario(page, "debug_warp_reds_house_1f_exit_mat");
  await waitForMap(page, "REDS_HOUSE_1F");
  await waitForNoMapLoading(page);
  await page.getByRole("button", { name: "Instant Warp" }).click();
  await waitForWarpMode(page, true);
  await clickTile(page, 7, 7);
  await waitForWarpMode(page, false);
  await waitForPlayerTile(page, 7, 7);
  await waitForPlayerIdle(page);

  await startSampling(page);
  // Columns 2 and 3 are actual exit warps; stay on the ordinary floor.
  await clickTile(page, 4, 7);
  await waitForPlayerTile(page, 4, 7);
  await waitForPlayerIdle(page);
  const clickSamples = await stopSampling(page);
  const clickPause = longestPauseWhileTraveling(clickSamples);

  await startSampling(page);
  await page.evaluate(() => {
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
  });
  await page.keyboard.down("d");
  // Release during the final tile's animation, so a prepared extra step must
  // be retired without advancing the character or leaving movement locked.
  await page.waitForFunction(() => (window as any).__movementContinuity.sprite.x > 5 * 16 + 9);
  await page.keyboard.up("d");
  await waitForPlayerTile(page, 6, 7);
  await waitForPlayerIdle(page);
  const keyboardSamples = await stopSampling(page);
  const keyboardPause = longestPauseWhileTraveling(keyboardSamples);
  await page.waitForTimeout(350);
  await waitForPlayerTile(page, 6, 7);
  await expect(page.getByRole("button", { name: "Bag", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "Bag", exact: true }).click();
  await expect.poll(async () => (await getGameState(page)).ui.isInventoryOpen).toBe(true);
  await page.getByRole("button", { name: "Bag", exact: true }).click();

  const evidencePath = test.info().outputPath("movement-continuity.json");
  await writeFile(evidencePath, JSON.stringify({ responseDelayMs: 125, clickPause, keyboardPause, clickSamples, keyboardSamples, packets }, null, 2));
  await test.info().attach("movement-continuity.json", { path: evidencePath, contentType: "application/json" });
  console.log(JSON.stringify({ responseDelayMs: 125, clickPause, keyboardPause }));
  await page.screenshot({ path: test.info().outputPath("continuous-movement.png") });
  expect(clickPause, "Click navigation must not stand still at each intermediate tile").toBeLessThan(100);
  expect(keyboardPause, "Held D must not stand still at each intermediate tile").toBeLessThan(100);
  errors.assertNoSevereErrors();
  await quitToCharacterSelect(page);
});
