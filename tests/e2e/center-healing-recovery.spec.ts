import { expect, test, type Page } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile, pressSpace } from "./helpers/input";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getEngineSnapshot, getGameState, waitForNoMapLoading } from "./helpers/state";

const label = "ViridianPokecenterNurseText";

async function advanceDialogue(page: Page, untilChoice = false) {
  for (let i = 0; i < 50; i++) {
    const state = await getGameState(page);
    if (untilChoice && state.dialogue.isChoicePending) return;
    if (!untilChoice && !state.worldInput.frozen && !state.dialogue.isOpen) return;
    if (state.dialogue.isOpen && !state.dialogue.isChoicePending) await pressSpace(page);
    else await page.waitForTimeout(150);
  }
  throw new Error(`Nurse dialogue did not reach ${untilChoice ? "choice" : "completion"}`);
}

for (const loseDelivery of [false, true]) {
  test(`center nurse Yes/No and durable recovery, lost delivery=${loseDelivery}`, async ({ page }, testInfo) => {
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated database and exact-process crash runner");
    test.setTimeout(180000);
    const { sql, crash, record } = await isolatedCrashRuntime();
    let errors = collectPageErrors(page);
    const starts: Array<{ scriptLabel: string; completionToken: string }> = [];
    const commands: Array<{ scriptLabel: string; completionToken: string; requestId: string; cancel?: boolean }> = [];
    const replies: Array<{ requestId: string; success: boolean; completed: boolean; replayed: boolean }> = [];
    const interactions: Array<{ success: boolean; started: boolean; error?: string }> = [];
    let droppedStart = false, droppedCompletion = false, legacyRequests = 0;
    await page.routeWebSocket("**/ws", socket => {
      const server = socket.connectToServer();
      socket.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          const opcode = message.readUInt16LE(4);
          if (opcode === OpCodes.PokeCenterHealRequest) legacyRequests++;
          if (opcode === OpCodes.CutsceneEndRequest) {
            const command = JSON.parse(message.subarray(6).toString());
            if (command.scriptLabel === label) commands.push(command);
          }
        }
        server.send(message);
      });
      server.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          const opcode = message.readUInt16LE(4);
          if (opcode !== OpCodes.CutsceneStartNotify && opcode !== OpCodes.CutsceneEndResponse && opcode !== OpCodes.ScriptedEventInteractResponse) { socket.send(message); return; }
          const payload = JSON.parse(message.subarray(6).toString());
          if (opcode === OpCodes.ScriptedEventInteractResponse) interactions.push(payload);
          if (opcode === OpCodes.CutsceneStartNotify && payload.scriptLabel === label) {
            starts.push(payload);
            if (loseDelivery && !droppedStart) { droppedStart = true; return; }
          }
          if (opcode === OpCodes.CutsceneEndResponse && commands.some(command => command.requestId === payload.requestId)) {
            replies.push(payload);
            if (loseDelivery && payload.success && payload.completed && !droppedCompletion) { droppedCompletion = true; return; }
          }
        }
        socket.send(message);
      });
    });
    const character = await createGuestCharacterAndEnterWorld(page);
    if (!loseDelivery) {
      await jumpToScenario(page, "debug_pokemon_center_pc_ready");
      const nurse = (await getEngineSnapshot(page, { bounds: { radius: 20 } })).actors.find(actor => actor.text === "TEXT_VIRIDIANPOKECENTER_NURSE");
      expect(nurse).toBeTruthy();
      const started = await page.evaluate(async actorId => {
        const path = "/src/phaser-game/services/PhaserNetworkService.ts";
        const net = await import(path);
        return net.tryScriptedEventInteraction(actorId);
      }, nurse!.id);
      expect(started).toBe(false);
      expect(interactions.at(-1)).toMatchObject({ success: false, started: false, error: "actor unavailable or out of reach" });
      expect(starts).toHaveLength(0);
    }
    await jumpToScenario(page, "debug_pokemon_center_nurse_ready");
    const characterId = (await getGameState(page)).player.internalId!;
    expect(Number.isSafeInteger(characterId) && characterId > 0).toBe(true);
    // The browser fixture seeds healthy Pokemon. Modify only the verified private
    // database while its character has no active session, then exercise real UI.
    await quitToCharacterSelect(page);
    await sql(`UPDATE character_pokemon SET cur_hp=1,status=8,move1_pp=0 WHERE character_id=${characterId}; INSERT INTO character_defeated_trainers(character_id,trainer_object_id) VALUES(${characterId},1109); UPDATE character_data SET options=COALESCE(options,'{}'::jsonb)-'lastPokeCenterMapId' WHERE id=${characterId}`);
    await enterWorld(page, character); await waitForNoMapLoading(page);
    expect((await getGameState(page)).pokemon.party[0].curHp).toBe(1);
    await page.screenshot({ path: testInfo.outputPath("source-nurse.png") });
    await clickTile(page, 3, 1);
    if (loseDelivery) {
      await expect.poll(() => droppedStart).toBe(true);
      await quitToCharacterSelect(page); await enterWorld(page, character);
      await waitForNoMapLoading(page);
      await expect.poll(() => starts.length).toBe(2);
      expect(starts[1].completionToken).toBe(starts[0].completionToken);
    }
    await advanceDialogue(page, true);
    await page.screenshot({ path: testInfo.outputPath("nurse-choice.png") });
    await page.getByTestId("dialogue-choice-no").click();
    await advanceDialogue(page);
    expect(replies.at(-1)).toMatchObject({ success: true, completed: false });
    expect(commands.at(-1)?.cancel).toBe(true);
    expect((await getGameState(page)).pokemon.party[0].curHp).toBe(1);
    expect(await sql(`SELECT count(*) FROM character_defeated_trainers WHERE character_id=${characterId}`)).toBe("1");
    expect(await sql(`SELECT COALESCE(options->>'lastPokeCenterMapId','unset') FROM character_data WHERE id=${characterId}`)).toBe("unset");
    await clickTile(page, 3, 1); await advanceDialogue(page, true);
    await page.getByTestId("dialogue-choice-yes").click();
    if (loseDelivery) {
      // Keep driving presentation until the server commits, then kill the exact
      // owned process before its missing acknowledgement can be recovered.
      for (let i = 0; i < 40 && !droppedCompletion; i++) { await pressSpace(page); await page.waitForTimeout(100); }
      await expect.poll(() => droppedCompletion).toBe(true);
      errors.assertNoSevereErrors();
      const receipt = await crash();
      // Active clients may reconnect during the deliberate server outage. Keep
      // every unrelated error failing instead of relaxing the global collector.
      expect(errors.pageErrors).toEqual([]); expect(errors.networkErrors).toEqual([]);
      expect(errors.retiredPositionPackets).toEqual([]);
      for (const error of errors.consoleErrors) {
        expect(error === "[CaptureQuestSocket] WebSocket error: Event" || /^WebSocket connection to 'ws:\/\/localhost:\d+\/ws' failed: Error in connection establishment: net::ERR_CONNECTION_REFUSED$/.test(error)).toBe(true);
      }
      await record({ family: "center-healing", characterId, receipt, completionToken: commands.at(-1)?.completionToken, outageErrors: errors.consoleErrors });
      await page.reload(); await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
      await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
      errors = collectPageErrors(page);
      await enterWorld(page, character); await waitForNoMapLoading(page);
    } else {
      await advanceDialogue(page);
    }
    const healed = (await getGameState(page)).pokemon.party[0];
    expect(healed.curHp).toBe(healed.maxHp);
    expect(await sql(`SELECT count(*) FROM character_pokemon WHERE character_id=${characterId} AND (cur_hp<>max_hp OR status<>0)`)).toBe("0");
    expect(await sql(`SELECT count(*) FROM character_defeated_trainers WHERE character_id=${characterId}`)).toBe("0");
    expect(await sql(`SELECT options->>'lastPokeCenterMapId' FROM character_data WHERE id=${characterId}`)).toBe("41");
    expect(legacyRequests).toBe(0);
    await page.screenshot({ path: testInfo.outputPath("nurse-healed.png") });
    await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
    expect((await getGameState(page)).pokemon.party[0].curHp).toBe(healed.maxHp);
    if (!loseDelivery) {
      const nurse = (await getEngineSnapshot(page)).actors.find(actor => actor.text === "TEXT_VIRIDIANPOKECENTER_NURSE");
      expect(nurse).toBeTruthy();
      const sourceObjectId = Number(await sql(`SELECT id FROM phaser_objects WHERE map_id=41 AND text='TEXT_VIRIDIANPOKECENTER_NURSE'`));
      expect(Number.isSafeInteger(sourceObjectId) && sourceObjectId > 0).toBe(true);
      await quitToCharacterSelect(page);
      await sql(`INSERT INTO character_object_visibility_overrides(character_id,object_id,visible,source) VALUES(${characterId},${sourceObjectId},false,'test')`);
      await enterWorld(page, character); await waitForNoMapLoading(page);
      expect((await getEngineSnapshot(page)).actors.some(actor => actor.text === "TEXT_VIRIDIANPOKECENTER_NURSE")).toBe(false);
      await page.screenshot({ path: testInfo.outputPath("nurse-hidden.png") });
      const startCount = starts.length;
      const started = await page.evaluate(async actorId => {
        const path = "/src/phaser-game/services/PhaserNetworkService.ts";
        const net = await import(path);
        return net.tryScriptedEventInteraction(actorId);
      }, nurse!.id);
      expect(started).toBe(false);
      expect(interactions.at(-1)).toMatchObject({ success: false, started: false, error: "actor unavailable or out of reach" });
      expect(starts).toHaveLength(startCount);
    }
    await quitToCharacterSelect(page); errors.assertNoSevereErrors();
  });
}
