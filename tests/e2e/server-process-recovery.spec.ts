import { expect, test } from "@playwright/test";
import type { BattleCommandResponse, PokeBattleCloseRequest, PokeMoveLearnRequest, GameplayStateResponse, SafariBattleActionRequest, SafariBattleActionResponse } from "../../src/net/generated/world_api";
import type { CutsceneEndRequest, CutsceneEndResponse, CutsceneStartNotify } from "../../src/net/generated/protocol";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { advanceBattleTextToPhase, waitForBattleOpen, catchSafariInRenderedUI } from "./helpers/battle";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement, pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

test("failed final playtime remains at its durable baseline after SIGKILL", async ({ page }) => {
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the owned isolated crash runner");
  test.setTimeout(120000);
  const { sql, crash, record } = await isolatedCrashRuntime();
  const cards: number[] = [];
  const entries: number[] = [];
  page.on("websocket", socket => socket.on("framereceived", frame => {
    const bytes = frame.payload;
    if (!Buffer.isBuffer(bytes) || bytes.length < 6) return;
    const opcode = bytes.readUInt16LE(4);
    if (opcode === OpCodes.TrainerCardResponse || opcode === OpCodes.CharacterData) {
      const payload = JSON.parse(bytes.subarray(6).toString());
      if (opcode === OpCodes.TrainerCardResponse && payload.success) cards.push(payload.timePlayed);
      if (opcode === OpCodes.CharacterData) entries.push(payload.timePlayed);
    }
  }));
  const character = await createGuestCharacterAndEnterWorld(page);
  // EnterWorld can render the scene before the character state stream arrives.
  await expect.poll(async () => (await getGameState(page)).player.internalId ?? 0).toBeGreaterThan(0);
  const id = (await getGameState(page)).player.internalId!;
  expect(Number.isSafeInteger(id) && id > 0).toBe(true);
  await quitToCharacterSelect(page);
  // Seed only the metric in the verified private fixture, before entry loads it.
  const baseline = 120;
  await sql(`UPDATE character_data SET time_played=${baseline} WHERE id=${id}`);
  await enterWorld(page, character);
  await expect.poll(() => entries.at(-1)).toBe(baseline);
  const positionQuery = `SELECT map_id || ',' || x || ',' || y FROM character_data WHERE id=${id}`;
  const position = await sql(positionQuery);
  await sql(`CREATE FUNCTION reject_crash_playtime() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated failed final playtime'; END $$;
    CREATE CONSTRAINT TRIGGER reject_crash_playtime AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
    WHEN (OLD.id=${id} AND NEW.time_played IS DISTINCT FROM OLD.time_played) EXECUTE FUNCTION reject_crash_playtime();`);
  // Observe a real whole active second through the rendered trainer-card request.
  await page.waitForTimeout(1100);
  const cardCount = cards.length;
  await page.getByRole("button", {name:"Trainer", exact:true}).click();
  await expect.poll(() => cards.length).toBeGreaterThan(cardCount);
  const active = cards.at(-1)!;
  expect(active).toBeGreaterThan(baseline);
  await page.keyboard.press("Escape");
  await quitToCharacterSelect(page); // freezes final playtime; commit is rejected
  expect(await sql(`SELECT time_played FROM character_data WHERE id=${id}`)).toBe(String(baseline));
  const receipt = await crash();
  await sql(`DROP TRIGGER reject_crash_playtime ON character_data`);
  await page.reload();
  await page.getByRole("button", {name:"PLAY AS GUEST"}).click();
  await expect(page.getByRole("heading", {name:"SELECT A CHARACTER"})).toBeVisible();
  const entryCount = entries.length;
  await enterWorld(page, character);
  expect(entries.length).toBeGreaterThan(entryCount);
  expect(entries.at(-1)).toBe(baseline);
  expect(await sql(positionQuery)).toBe(position);
  await record({family:"playtime", id, baseline, active, recoveredBaseline:entries.at(-1), position, receipt});
  await quitToCharacterSelect(page);
});

for (const outcome of ["run", "party", "pc"] as const) {
  test(`committed Safari ${outcome} survives SIGKILL and a fresh server/client`, async ({ page, context }) => {
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated runner crash recovery mode");
    test.setTimeout(180000);
    const { sql, crash, record } = await isolatedCrashRuntime();
    const errors = collectPageErrors(page);
    const commands: SafariBattleActionRequest[] = [];
    const snapshots: GameplayStateResponse[] = [];
    let held: SafariBattleActionResponse | undefined;
    let replies = 0;
    // Context routing follows the fresh page as well, but does not retain the
    // dead transport or frontend memory as the recovery authority.
    await context.routeWebSocket("**/ws", socket => {
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
            const response: SafariBattleActionResponse = JSON.parse(message.subarray(6).toString()); replies++;
            if (!held && !response.closed && (outcome === "run" ? response.isOver : response.caught)) { held = response; return; }
          }
        }
        socket.send(message);
      });
    });
    const character = await createGuestCharacterAndEnterWorld(page);
    if (outcome === "run") {
      await jumpToScenario(page, "safari_battle_run"); await waitForMap(page, "SAFARI_ZONE_CENTER"); await waitForNoMapLoading(page);
      await page.getByTestId("battle-action-run").click({ timeout: 10000 }); await expect.poll(() => !!held).toBe(true);
    } else await catchSafariInRenderedUI(page, outcome === "pc" ? 6 : 1, () => replies, () => !!held);
    expect(held).toMatchObject({ success: true, isOver: true });
    const committed = held!;
    const commandCount = commands.length;
    const characterId = (await getGameState(page)).player.internalId;
    expect(Number.isSafeInteger(characterId) && characterId! > 0).toBe(true);
    const readDurable = async () => JSON.parse(await sql(`SELECT json_build_object(
      'position',(SELECT json_build_object('mapId',map_id,'x',x,'y',y) FROM character_data WHERE id=${characterId}),
      'safari',(SELECT state_json::json FROM character_safari_state WHERE character_id=${characterId}),
      'pokemon',(SELECT json_agg(p ORDER BY p.id) FROM character_pokemon p WHERE character_id=${characterId}),
      'pokedex',(SELECT json_agg(p ORDER BY p.pokemon_id) FROM character_pokedex p WHERE character_id=${characterId}))`));
    const before = await readDurable();
    expect(before.safari.visit.battle).toMatchObject({ battleId: committed.battleId, revision: committed.revision, Phase: 1, Caught: outcome !== "run" });
    expect(before.safari.visit.ballsLeft).toBe(committed.ballsLeft);
    const caughtRows = before.pokemon.filter((pokemon: { pokemon_id: number }) => pokemon.pokemon_id === 129);
    if (outcome !== "run") {
      expect(caughtRows).toHaveLength(1);
      expect(before.safari.visit.capture.sentToPC).toBe(outcome === "pc");
    }
    errors.assertNoSevereErrors();
    const receipt = await crash();
    expect(await readDurable()).toEqual(before);
    await page.close();
    const fresh = await context.newPage();
    const freshErrors = collectPageErrors(fresh);
    await fresh.goto("/"); await fresh.getByRole("button", { name: "PLAY AS GUEST" }).click();
    await expect(fresh.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30000 });
    await enterWorld(fresh, character); await waitForMap(fresh, "SAFARI_ZONE_CENTER"); await waitForNoMapLoading(fresh);
    const summary = fresh.getByText(outcome === "run" ? /^Got away safely!\s*▼?$/ : outcome === "pc" ? /MAGIKARP was transferred to\s+Bill's PC \(BOX 1\)\./ : /MAGIKARP's data was added to the POKéDEX!/);
    await expect(summary).toBeVisible();
    expect((await getGameState(fresh)).player.internalId).toBe(characterId);
    const recovered = [...snapshots].reverse().find(snapshot => snapshot.safari?.battleId === committed.battleId)?.safari;
    expect(recovered).toMatchObject({ battleId: committed.battleId, revision: committed.revision, isOver: true, ballsLeft: committed.ballsLeft });
    expect(commands).toHaveLength(commandCount);
    expect(await readDurable()).toEqual(before);
    await fresh.screenshot({ path: test.info().outputPath(`process-recovered-${outcome}.png`) });
    await pressSpace(fresh); await expect.poll(async () => (await getGameState(fresh)).battle.isOpen).toBe(false);
    expect(commands).toHaveLength(commandCount + 1);
    expect(commands.at(-1)).toMatchObject({ action: "close", battle: { battleId: committed.battleId, revision: committed.revision } });
    const after = await readDurable();
    expect(after.safari.visit.battle).toBeUndefined(); expect(after.pokemon).toEqual(before.pokemon); expect(after.pokedex).toEqual(before.pokedex);
    freshErrors.assertNoSevereErrors(); await quitToCharacterSelect(fresh); await fresh.close();
    await record({ outcome, characterId, receipt, committed, before, after, commandCount });
  });
}

for (const choice of ["learn", "skip"] as const) {
  test(`pending move ${choice} survives two SIGKILLs without repeating EXP or choice`, async ({ page, context }) => {
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated runner crash recovery mode");
    test.setTimeout(240000);
    const { sql, crash, record } = await isolatedCrashRuntime();
    const errors = collectPageErrors(page);
    const learning: PokeMoveLearnRequest[] = [];
    const closes: PokeBattleCloseRequest[] = [];
    let actions = 0, replies = 0;
    let pending: BattleCommandResponse | undefined;
    let settledReply: BattleCommandResponse | undefined;
    await context.routeWebSocket("**/ws", socket => {
      const server = socket.connectToServer();
      socket.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          const opcode = message.readUInt16LE(4);
          if (opcode === OpCodes.PokeBattleActionRequest) actions++;
          if (opcode === OpCodes.PokeMoveLearnRequest) learning.push(JSON.parse(message.subarray(6).toString()));
          if (opcode === OpCodes.PokeBattleCloseRequest) closes.push(JSON.parse(message.subarray(6).toString()));
        }
        server.send(message);
      });
      server.onMessage(message => {
        if (Buffer.isBuffer(message) && message.length >= 6) {
          const opcode = message.readUInt16LE(4);
          if (opcode === OpCodes.PokeBattleActionResponse) {
            const response: BattleCommandResponse = JSON.parse(message.subarray(6).toString()); replies++;
            if (response.success && response.battle?.pendingMove) { pending = response; return; }
          }
          if (opcode === OpCodes.PokeMoveLearnResponse) {
            settledReply = JSON.parse(message.subarray(6).toString()); return;
          }
        }
        socket.send(message);
      });
    });
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "active_battle_fixture_learning_recovery");
    await waitForBattleOpen(page); await advanceBattleTextToPhase(page, "action_select");
    const initial = (await getGameState(page)).pokemon.party[0];
    expect(initial.level).toBe(6); expect(initial.moves).toHaveLength(4);
    const characterId = (await getGameState(page)).player.internalId;
    expect(Number.isSafeInteger(characterId) && characterId! > 0).toBe(true);
    const readDurable = async () => JSON.parse(await sql(`SELECT json_build_object(
      'battle',(SELECT battle_json::json FROM character_battle_state WHERE character_id=${characterId}),
      'pokemon',(SELECT json_agg(p ORDER BY p.id) FROM character_pokemon p WHERE character_id=${characterId}))`));
    const original = await readDurable();
    for (let turn = 0; turn < 8 && !pending; turn++) {
      const before = replies;
      await page.getByTestId("battle-action-fight").click(); await page.getByTestId("battle-move-0").click();
      await expect.poll(() => replies > before).toBe(true);
      if (!pending) {
        await expect(page.getByText("Waiting for the battle…", { exact: true })).toBeHidden();
        await advanceBattleTextToPhase(page, "action_select");
      }
    }
    expect(pending?.success).toBe(true);
    const identity = { battleId: pending!.battle!.battleId, revision: pending!.battle!.revision };
    const before = await readDurable();
    expect(before.battle).toMatchObject({ ...identity, pendingMove: { moveId: 73, moveName: "LEECH_SEED", pokemonIndex: 0 } });
    expect(before.pokemon.map((p: { id: number }) => p.id)).toEqual(original.pokemon.map((p: { id: number }) => p.id));
    expect(before.pokemon[0]).toMatchObject({ level: 7, exp: initial.exp + 72 });
    expect(learning).toHaveLength(0); expect(closes).toHaveLength(0);
    const actionCount = actions;
    errors.assertNoSevereErrors();
    const pendingCrash = await crash();
    expect(await readDurable()).toEqual(before);
    await page.close();
    const fresh = await context.newPage();
    const freshErrors = collectPageErrors(fresh);
    await fresh.goto("/"); await fresh.getByRole("button", { name: "PLAY AS GUEST" }).click();
    await expect(fresh.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30000 });
    await enterWorld(fresh, character); await waitForNoMapLoading(fresh);
    await expect(fresh.getByText("Forget a move?", { exact: true })).toBeVisible();
    await expect(fresh.getByText("Trying to learn LEECH_SEED", { exact: true })).toBeVisible();
    expect((await getGameState(fresh)).player.internalId).toBe(characterId);
    expect((await getGameState(fresh)).pokemon.party[0]).toMatchObject({ level: 7, exp: initial.exp + 72 });
    expect(await readDurable()).toEqual(before);
    expect(actions).toBe(actionCount); expect(learning).toHaveLength(0); expect(closes).toHaveLength(0);
    await fresh.screenshot({ path: test.info().outputPath(`process-pending-${choice}.png`) });
    if (choice === "learn") await fresh.getByRole("button", { name: /^> TACKLE PP / }).click({ timeout: 10000 });
    else await fresh.getByRole("button", { name: "> Don't learn LEECH_SEED", exact: true }).click({ timeout: 10000 });
    await expect.poll(() => !!settledReply).toBe(true);
    expect(settledReply!.success).toBe(true);
    expect(learning).toHaveLength(1); expect(learning[0].battle).toEqual(identity);
    const settled = await readDurable();
    expect(settled.battle).toMatchObject({ battleId: identity.battleId, revision: identity.revision + 1 });
    expect(settled.battle.pendingMove).toBeUndefined();
    expect(settled.pokemon.map((p: { id: number }) => p.id)).toEqual(before.pokemon.map((p: { id: number }) => p.id));
    expect(settled.pokemon[0].exp).toBe(before.pokemon[0].exp);
    const moveIds = choice === "learn" ? [75, 73, 45, 22] : [75, 33, 45, 22];
    expect([1, 2, 3, 4].map(slot => settled.pokemon[0][`move${slot}_id`])).toEqual(moveIds);
    expect(closes).toHaveLength(0); freshErrors.assertNoSevereErrors();
    const settledCrash = await crash();
    expect(await readDurable()).toEqual(settled);
    await fresh.close();
    const final = await context.newPage();
    const finalErrors = collectPageErrors(final);
    await final.goto("/"); await final.getByRole("button", { name: "PLAY AS GUEST" }).click();
    await expect(final.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30000 });
    await enterWorld(final, character); await waitForNoMapLoading(final);
    await expect.poll(() => closes.length, { timeout: 20000 }).toBe(1);
    await expect.poll(async () => (await getGameState(final)).battle.isOpen).toBe(false);
    expect(closes[0].battle).toEqual({ battleId: identity.battleId, revision: identity.revision + 1 });
    expect(actions).toBe(actionCount); expect(learning).toHaveLength(1);
    const after = await readDurable();
    expect(after.battle).toBeNull(); expect(after.pokemon).toEqual(settled.pokemon);
    const party = (await getGameState(final)).pokemon.party;
    expect(party[0]).toMatchObject({ level: 7, exp: initial.exp + 72 });
    expect(party[0].moves.map(move => move.id)).toEqual(moveIds);
    await expect(final.getByText("Forget a move?", { exact: true })).toBeHidden();
    await final.screenshot({ path: test.info().outputPath(`process-settled-${choice}.png`) });
    await quitToCharacterSelect(final); await enterWorld(final, character); await waitForNoMapLoading(final);
    expect((await getGameState(final)).pokemon.party).toEqual(party);
    expect(await readDurable()).toEqual(after);
    expect(actions).toBe(actionCount); expect(learning).toHaveLength(1); expect(closes).toHaveLength(1);
    finalErrors.assertNoSevereErrors(); await quitToCharacterSelect(final); await final.close();
    await record({ outcome: `move-${choice}`, characterId, pendingCrash, settledCrash, identity, original, before, settled, after, actionCount });
  });
}

test("an issued cutscene and its committed completion survive SIGKILL without replaying effects", async ({ page, context }) => {
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated runner crash recovery mode");
  test.setTimeout(240000);
  const { sql, crash, record } = await isolatedCrashRuntime();
  const errors = collectPageErrors(page);
  const label = "OaksLabChooseStarterIntro";
  const starts: CutsceneStartNotify[] = [];
  const completions: CutsceneEndRequest[] = [];
  const replies: CutsceneEndResponse[] = [];
  let held: CutsceneEndResponse | undefined;
  await context.routeWebSocket("**/ws", socket => {
    const server = socket.connectToServer();
    socket.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.CutsceneEndRequest) {
        const request: CutsceneEndRequest = JSON.parse(message.subarray(6).toString());
        if (request.scriptLabel === label) completions.push(request);
      }
      server.send(message);
    });
    server.onMessage(message => {
      if (Buffer.isBuffer(message) && message.length >= 6) {
        const opcode = message.readUInt16LE(4);
        if (opcode === OpCodes.CutsceneStartNotify) {
          const start: CutsceneStartNotify = JSON.parse(message.subarray(6).toString());
          if (start.scriptLabel === label) {
            starts.push(start);
            if (starts.length === 1) return;
          }
        }
        if (opcode === OpCodes.CutsceneEndResponse) {
          const reply: CutsceneEndResponse = JSON.parse(message.subarray(6).toString());
          if (completions.some(request => request.requestId === reply.requestId)) {
            if (reply.success && !held) { held = reply; return; }
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
  await expect.poll(() => starts.length).toBe(1);
  const characterId = (await getGameState(page)).player.internalId;
  expect(Number.isSafeInteger(characterId) && characterId! > 0).toBe(true);
  const readDurable = async () => JSON.parse(await sql(`SELECT json_build_object(
    'position',(SELECT json_build_object('mapId',map_id,'x',x,'y',y,'heading',heading) FROM character_data WHERE id=${characterId}),
    'plans',(SELECT json_agg(p ORDER BY p.sequence) FROM character_cutscene_plans p WHERE character_id=${characterId}),
    'flags',(SELECT json_agg(f ORDER BY f.flag_name) FROM character_event_flags f WHERE character_id=${characterId}),
    'visibility',(SELECT json_agg(v ORDER BY v.object_id) FROM character_object_visibility_overrides v WHERE character_id=${characterId}),
    'pokemon',(SELECT json_agg(p ORDER BY p.id) FROM character_pokemon p WHERE character_id=${characterId}))`));
  const before = await readDurable();
  const token = starts[0].completionToken;
  const plan = before.plans.find((p: { completion_token: string }) => p.completion_token === token);
  expect(plan).toMatchObject({ script_label: label, resolution: "pending", completed: false, x: 5, y: 11 });
  expect(JSON.parse(plan.script_json)).toMatchObject({ ScriptLabel: label });
  expect(before.position).toMatchObject({ x: 5, y: 11 }); expect(completions).toHaveLength(0);
  errors.assertNoSevereErrors();
  const issuedCrash = await crash();
  expect(await readDurable()).toEqual(before);
  await page.close();
  const fresh = await context.newPage();
  const freshErrors = collectPageErrors(fresh);
  await fresh.goto("/"); await fresh.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect(fresh.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30000 });
  await enterWorld(fresh, character); await waitForMap(fresh, "OAKS_LAB"); await waitForNoMapLoading(fresh);
  await expect.poll(() => starts.length).toBeGreaterThanOrEqual(2);
  expect(starts[1]).toEqual(starts[0]);
  // Stop at the actual committed response, before the lost-ack timeout can
  // replay completion or unlock the original page. The next process is authority.
  for (let i = 0; i < 90 && !held; i++) {
    const state = await getGameState(fresh);
    if (state.dialogue.isOpen || state.dialogue.isChoicePending) await pressSpace(fresh);
    else await fresh.waitForTimeout(200);
  }
  await expect.poll(() => !!held).toBe(true);
  expect(held).toMatchObject({ success: true, completed: true, replayed: false, x: 5, y: 3 });
  expect(completions).toHaveLength(1); expect(completions[0]).toMatchObject({ scriptLabel: label, completionToken: token });
  const settled = await readDurable();
  expect(settled.position).toMatchObject({ x: 5, y: 3 });
  expect(settled.plans.find((p: { completion_token: string }) => p.completion_token === token)).toMatchObject({ resolution: "resolved", completed: true });
  const flags = settled.flags.map((f: { flag_name: string }) => f.flag_name);
  for (const flag of ["EVENT_FOLLOWED_OAK_INTO_LAB", "EVENT_FOLLOWED_OAK_INTO_LAB_2", "EVENT_OAK_ASKED_TO_CHOOSE_MON"]) expect(flags).toContain(flag);
  expect(settled.pokemon).toEqual(before.pokemon); freshErrors.assertNoSevereErrors();
  const completedCrash = await crash();
  expect(await readDurable()).toEqual(settled);
  await fresh.close();
  const final = await context.newPage();
  const finalErrors = collectPageErrors(final);
  const startCount = starts.length;
  await final.goto("/"); await final.getByRole("button", { name: "PLAY AS GUEST" }).click();
  await expect(final.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible({ timeout: 30000 });
  await enterWorld(final, character); await waitForMap(final, "OAKS_LAB"); await waitForNoMapLoading(final);
  await waitForPlayerIdle(final); await waitForPlayerTile(final, 5, 3);
  expect(starts).toHaveLength(startCount); expect(completions).toHaveLength(1);
  expect(await readDurable()).toEqual(settled);
  await pressMovement(final, "down"); await waitForPlayerIdle(final); await waitForPlayerTile(final, 5, 4);
  const moved = await readDurable();
  await final.evaluate(async ({ completionToken, scriptLabel }) => {
    const bridgePath = "/src/net/NetworkBridge.ts", opcodePath = "/src/net/generated/opcodes.ts";
    const { NetworkBridge } = await import(bridgePath);
    const opcodes = await import(opcodePath);
    NetworkBridge.send({ completionToken, scriptLabel, requestId: "after-process-death" }, opcodes.CutsceneEndRequest);
  }, { completionToken: token, scriptLabel: label });
  await expect.poll(() => replies.find(reply => reply.requestId === "after-process-death")?.replayed).toBe(true);
  expect(replies.find(reply => reply.requestId === "after-process-death")).toMatchObject({ completed: true, x: 5, y: 4 });
  await waitForPlayerTile(final, 5, 4); expect(await readDurable()).toEqual(moved);
  expect(completions).toHaveLength(2); expect(starts).toHaveLength(startCount);
  await final.screenshot({ path: test.info().outputPath("process-cutscene-settled.png") });
  finalErrors.assertNoSevereErrors(); await quitToCharacterSelect(final); await final.close();
  await record({ outcome: "issued-cutscene", characterId, token, issuedCrash, completedCrash, before, settled, moved, startCount, completions });
});
