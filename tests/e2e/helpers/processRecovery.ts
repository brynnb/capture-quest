import { expect } from "@playwright/test";
import { execFile } from "node:child_process";
import { readFile, realpath, rename, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { promisify } from "node:util";

const run = promisify(execFile);

// Every crash-recovery family uses the shell-owned exact-process protocol. Database
// reads inspect the unchanged private cluster, never the application's .env.
export async function isolatedCrashRuntime() {
  const runDir = resolve(process.env.E2E_ISOLATED_RUN_DIR ?? "");
  expect(runDir.startsWith("/var/tmp/capturequest-rendered.")).toBe(true);
  const databaseURL = process.env.E2E_ISOLATED_DATABASE_URL ?? "";
  const database = new URL(databaseURL);
  expect(database.hostname).toBe(""); expect(database.searchParams.get("host")).toBe(runDir);
  await readFile(join(runDir, "pg/postmaster.pid"));
  const ownedServerPid = async () => {
    const serverPid = Number((await readFile(join(runDir, "current-server.pid"), "utf8")).trim());
    expect(Number.isSafeInteger(serverPid) && serverPid > 1).toBe(true);
    expect(await realpath(`/proc/${serverPid}/exe`)).toBe(join(runDir, "cq-server"));
    return serverPid;
  };
  await ownedServerPid();
  // Pause only this runner's verified child at an observed commit boundary.
  // The crash protocol kills/reaps that same stopped child and restarts it.
  const freeze = async () => { const pid = await ownedServerPid(); process.kill(pid, "SIGSTOP"); return pid; };
  const sql = async (query: string) => (await run("psql", [databaseURL, "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-c", query])).stdout.trim();
  const crash = async () => {
    const serverPid = await ownedServerPid();
    // The shell verifies its child, sends SIGKILL and reaps exit 137.
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
  return { sql, crash, freeze, record };
}

