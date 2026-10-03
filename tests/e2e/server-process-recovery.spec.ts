import { expect, test } from "@playwright/test";
import { execFile } from "node:child_process";
import { readFile, realpath, rename, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { promisify } from "node:util";
import type { GameplayStateResponse, SafariBattleActionRequest, SafariBattleActionResponse } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { catchSafariInRenderedUI } from "./helpers/battle";
import { collectPageErrors } from "./helpers/errors";
import { pressSpace } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForMap, waitForNoMapLoading } from "./helpers/state";

const run = promisify(execFile);

for (const outcome of ["run", "party", "pc"] as const) {
  test(`committed Safari ${outcome} survives SIGKILL and a fresh server/client`, async ({ page, context }) => {
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated runner crash recovery mode");
    test.setTimeout(180000);
    const runDir = resolve(process.env.E2E_ISOLATED_RUN_DIR ?? "");
    expect(runDir.startsWith("/var/tmp/capturequest-rendered.")).toBe(true);
    const databaseURL = process.env.E2E_ISOLATED_DATABASE_URL ?? "";
    const database = new URL(databaseURL);
    expect(database.hostname).toBe(""); expect(database.searchParams.get("host")).toBe(runDir);
    await readFile(join(runDir, "pg/postmaster.pid"));
    const serverPid = Number((await readFile(join(runDir, "current-server.pid"), "utf8")).trim());
    expect(Number.isSafeInteger(serverPid) && serverPid > 1).toBe(true);
    expect(await realpath(`/proc/${serverPid}/exe`)).toBe(join(runDir, "cq-server"));
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
    const sql = async (query: string) => (await run("psql", [databaseURL, "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-c", query])).stdout.trim();
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
    const evidencePath = join(runDir, "process-recovery-evidence.json");
    let evidence: unknown[] = [];
    try { evidence = JSON.parse(await readFile(evidencePath, "utf8")); } catch (error) { if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error; }
    evidence.push({ outcome, characterId, receipt, committed, before, after, commandCount });
    await writeFile(evidencePath, JSON.stringify(evidence, null, 2));
  });
}
