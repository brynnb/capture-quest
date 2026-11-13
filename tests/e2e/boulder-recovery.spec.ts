import { expect, test } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { pressMovement } from "./helpers/input";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerIdle, waitForPlayerTile } from "./helpers/state";

for (const mode of ["normal", "lost", "lost-all-movement", "lost-all-world", "crash"] as const) {
  test(`boulder duplicate and mode=${mode} recover one object move and follow-up`, async ({ page, context }) => {
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY !== "true", "Requires the isolated database and recorded server");
    test.setTimeout(120_000);
    const { sql, crash, freeze, record } = await isolatedCrashRuntime();
    let errors = collectPageErrors(page);
    let frozen: Promise<number> | undefined;
    let facingRequests = 0;
    const faults = mode === "crash" ? undefined : await inventoryCommandFaults(page,
      OpCodes.PlayerFacingRequest, OpCodes.PlayerFacingResponse, mode !== "normal", () => true,
      opcode => (mode === "lost-all-movement" || mode === "lost-all-world") && (opcode === OpCodes.ServerPlayerMovementNotify
        || mode === "lost-all-world" && (opcode === OpCodes.PhaserActorPositionUpdate || opcode === OpCodes.PhaserActorDespawn)));
    if (mode === "crash") {
      await page.routeWebSocket("**/ws", socket => {
        const server = socket.connectToServer();
        socket.onMessage(message => {
          if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PlayerFacingRequest) {
            facingRequests++;
            server.send(message);
          }
          server.send(message);
        });
        server.onMessage(message => {
          if (Buffer.isBuffer(message) && message.length >= 6 && message.readUInt16LE(4) === OpCodes.PlayerFacingResponse) {
            const reply = JSON.parse(message.subarray(6).toString());
            if (!frozen && reply.success && reply.serverMovementPending) { frozen = freeze(); return; }
          }
          socket.send(message);
        });
      });
    }
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "seafoam_1f_runtime_boulder_push_facing_update");
    await waitForNoMapLoading(page);
    await waitForPlayerTile(page, 18, 11);
    const before = await getGameState(page);
    const charId = before.player.internalId!;
    expect(Number.isSafeInteger(charId) && charId > 0).toBe(true);
    const objectQuery = `SELECT p.x,p.y FROM character_object_positions p JOIN phaser_objects o ON o.id=p.object_id WHERE p.character_id=${charId} AND o.name='SeafoamIslands1F_NPC_1'`;
    await pressMovement(page, "up");
    if (mode === "crash") {
      await expect.poll(() => Boolean(frozen)).toBe(true);
      const pid = await frozen!;
      let receipt: Awaited<ReturnType<typeof crash>>;
      try {
        expect(await sql(objectQuery)).toBe("18|9");
        expect(await sql(`SELECT x,y,json_array_length(path_json::json->'path') FROM character_movement_routes WHERE character_id=${charId}`)).toBe("18|11|1");
        expect(facingRequests).toBe(1);
      } finally { await page.close(); receipt = await crash(); }
      expect(receipt.oldPid).toBe(pid);
      await record({ family: "boulder", charId, object: "SeafoamIslands1F_NPC_1", receipt });
      page = await context.newPage(); await page.goto("/");
      await page.getByRole("button", { name: "PLAY AS GUEST" }).click();
      await expect(page.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
      errors = collectPageErrors(page);
      await enterWorld(page, character);
      await waitForNoMapLoading(page);
    } else {
      await expect.poll(() => faults!.successes).toBe(1);
      await expect.poll(() => faults!.rejections).toBe(1);
      expect(faults!.requests).toBe(1);
    }
    await waitForPlayerTile(page, 18, 10); await waitForPlayerIdle(page);
    await expect.poll(async () => {
      const boulder = (await getGameState(page)).visibleActors.find(a => a.name === "SeafoamIslands1F_NPC_1");
      return boulder && [boulder.x, boulder.y];
    }).toEqual([18, 9]);
    expect(await sql(objectQuery)).toBe("18|9");
    expect(await sql(`SELECT COUNT(*) FROM character_movement_routes WHERE character_id=${charId}`)).toBe("0");
    if (mode !== "crash") {
      faults!.deliverReply(0);
      await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
      await waitForPlayerTile(page, 18, 10);
      await quitToCharacterSelect(page); await enterWorld(page, character); await waitForNoMapLoading(page);
      await waitForPlayerTile(page, 18, 10);
      expect(await sql(objectQuery)).toBe("18|9");
    }
    expect(errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepCompleteRequest)).toEqual([]);
    errors.assertNoSevereErrors();
  });
}
