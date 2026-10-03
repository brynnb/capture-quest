import { expect, test } from "@playwright/test";
import { execFile } from "node:child_process";
import { readFile, realpath, rename, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { promisify } from "node:util";
import type { BattleCommandResponse, PokeBattleCloseRequest, PokeMoveLearnRequest, GameplayStateResponse, SafariBattleActionRequest, SafariBattleActionResponse } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { advanceBattleTextToPhase, waitForBattleOpen, catchSafariInRenderedUI } from "./helpers/battle";
import { collectPageErrors } from "./helpers/errors";
import { pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading } from "./helpers/state";

const run = promisify(execFile);

// Both encounter families use the shell-owned exact-process protocol. Database
// reads inspect the unchanged private cluster, never the application's .env.
async function isolatedCrashRuntime() {
  const runDir = resolve(process.env.E2E_ISOLATED_RUN_DIR ?? "");
  expect(runDir.startsWith("/var/tmp/capturequest-rendered.")).toBe(true);
  const databaseURL = process.env.E2E_ISOLATED_DATABASE_URL ?? "";
  const database = new URL(databaseURL);
  expect(database.hostname).toBe(""); expect(database.searchParams.get("host")).toBe(runDir);
  await readFile(join(runDir, "pg/postmaster.pid"));
  const serverPid = Number((await readFile(join(runDir, "current-server.pid"), "utf8")).trim());
  expect(Number.isSafeInteger(serverPid) && serverPid > 1).toBe(true);
  expect(await realpath(`/proc/${serverPid}/exe`)).toBe(join(runDir, "cq-server"));
  const sql = async (query: string) => (await run("psql", [databaseURL, "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-c", query])).stdout.trim();
  const crash = async () => {
    const serverPid = Number((await readFile(join(runDir, "current-server.pid"), "utf8")).trim());
    expect(Number.isSafeInteger(serverPid) && serverPid > 1).toBe(true);
    // The shell verifies this request against its own live child, sends SIGKILL,
    // reaps exit 137, and starts the same binary with this unchanged private DB.
    expect(await realpath(`/proc/${serverPid}/exe`)).toBe(join(runDir, "cq-server"));
    await writeFile(join(runDir, "crash-request.tmp"), `${serverPid}\n`);
    await rename(join(runDir, "crash-request.tmp"), join(runDir, "crash-request"));
    await expect.poll(async () => {
      try { return JSON.parse(await readFile(join(runDir, "restart-receipt.json"), "utf8")).oldPid; }
      catch (error) { if ((error as NodeJS.ErrnoException).code === "ENOENT") return 0; throw error; }
    }, { timeout: 60000 }).toBe(serverPid);
    const receipt = JSON.parse(await readFile(join(runDir, "restart-receipt.json"), "utf8"));
    expect(receipt.exitCode).toBe(137); expect(receipt.newPid).not.toBe(serverPid);
    expect(await realpath(`/proc/${receipt.newPid}/exe`)).toBe(join(runDir, "cq-server"));
    return receipt;
  };
  const record = async (entry: unknown) => {
    const evidencePath = join(runDir, "process-recovery-evidence.json");
    let evidence: unknown[] = [];
    try { evidence = JSON.parse(await readFile(evidencePath, "utf8")); } catch (error) { if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error; }
    evidence.push(entry);
    await writeFile(evidencePath, JSON.stringify(evidence, null, 2));
  };
  return { sql, crash, record };
}

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
