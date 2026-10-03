import { expect, test } from "@playwright/test";
import { execFile } from "node:child_process";
import { readFile, realpath, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { promisify } from "node:util";
import { createGuestCharacterAndEnterWorld } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement } from "./helpers/input";
import { getGameState, waitForMap, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

const run = promisify(execFile);
const mode = process.env.CQ_E2E_SHUTDOWN_MODE;

test(`active player shutdown persists position and reports ${mode ?? "isolated"} final save`, async ({ page }) => {
  test.skip(!mode, "Requires the dedicated isolated runner shutdown mode");
  test.setTimeout(120_000);
  expect(["success", "failure"]).toContain(mode);
  const runDir = resolve(process.env.E2E_ISOLATED_RUN_DIR ?? "");
  expect(runDir.startsWith("/var/tmp/capturequest-rendered.")).toBe(true);
  const pid = Number(process.env.E2E_ISOLATED_SERVER_PID);
  expect(Number.isSafeInteger(pid) && pid > 1).toBe(true);
  const databaseURL = process.env.E2E_ISOLATED_DATABASE_URL ?? "";
  const database = new URL(databaseURL);
  expect(database.hostname).toBe("");
  expect(database.searchParams.get("host")).toBe(runDir);
  await readFile(join(runDir, "pg/postmaster.pid"));
  expect(await realpath(`/proc/${pid}/exe`)).toBe(join(runDir, "cq-server"));
  const sql = async (query: string) => (await run("psql", [databaseURL, "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-c", query])).stdout.trim();
  const sockets = new Set<import("@playwright/test").WebSocket>();
  let closedSockets = 0;
  page.on("websocket", (socket) => {
    if (new URL(socket.url()).pathname !== "/ws") return;
    sockets.add(socket);
    socket.on("close", () => { sockets.delete(socket); closedSockets++; });
  });
  const errors = collectPageErrors(page);
  await createGuestCharacterAndEnterWorld(page);
  await waitForMap(page, "REDS_HOUSE_2F");
  await waitForPlayerTile(page, 3, 6);
  await pressMovement(page, "right");
  await waitForPlayerTile(page, 4, 6);
  await waitForPlayerIdle(page);
  const state = await getGameState(page);
  const id = state.player.internalId;
  expect(Number.isSafeInteger(id) && id! > 0).toBe(true);
  expect(sockets.size).toBe(1);
  const positionQuery = `SELECT map_id || ',' || x || ',' || y FROM character_data WHERE id=${id}`;
  const expectedPosition = `${state.map.id},4,6`;
  await expect.poll(() => sql(positionQuery)).toBe(expectedPosition);
  await page.screenshot({ path: join(runDir, "active-player.png") });
  const playtimeQuery = `SELECT time_played FROM character_data WHERE id=${id}`;
  const before = Number(await sql(playtimeQuery));
  if (mode === "failure") {
    // Reject only this test character's actual playtime changes at commit.
    await sql(`CREATE FUNCTION reject_shutdown_playtime() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'isolated shutdown playtime failure'; END $$;
      CREATE CONSTRAINT TRIGGER reject_shutdown_playtime AFTER UPDATE ON character_data DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
      WHEN (OLD.id=${id} AND NEW.time_played IS DISTINCT FROM OLD.time_played) EXECUTE FUNCTION reject_shutdown_playtime();`);
  }
  // Accrue a whole active second; the final save must have an interval to write.
  await page.waitForTimeout(1100);
  errors.assertNoSevereErrors();
  expect(await realpath(`/proc/${pid}/exe`)).toBe(join(runDir, "cq-server"));
  process.kill(pid, "SIGTERM");
  await expect.poll(async () => {
    try {
      const stat = await readFile(`/proc/${pid}/stat`, "utf8");
      return stat.slice(stat.lastIndexOf(")") + 2).startsWith("Z ");
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ENOENT") return true;
      throw error;
    }
  }, { timeout: 15_000 }).toBe(true);
  await expect.poll(() => sockets.size, { timeout: 5_000 }).toBe(0);
  expect(closedSockets).toBeGreaterThan(0);
  expect(await sql(positionQuery)).toBe(expectedPosition);
  const after = Number(await sql(playtimeQuery));
  const log = await readFile(join(runDir, "server.log"), "utf8");
  if (mode === "failure") {
    expect(after).toBe(before);
    expect(log).toContain("final playtime");
    expect(log).toContain("Shutdown failed:");
  } else {
    expect(after).toBeGreaterThan(before);
    expect(log).not.toContain("Shutdown failed:");
  }
  await writeFile(join(runDir, "shutdown-evidence.json"), JSON.stringify({ mode, characterId: id, position: expectedPosition, playtimeBefore: before, playtimeAfter: after, closedSockets }, null, 2));
});
