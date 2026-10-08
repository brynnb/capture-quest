import { expect, test } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { pressMovement } from "./helpers/input";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerTile } from "./helpers/state";

for (const mode of ["newer-stream", "retirement"] as const) {
  test(`late actor snapshot mode=${mode} cannot rewind its scene`, async ({ page }) => {
    test.setTimeout(120_000);
    const errors = collectPageErrors(page);
    let hold = false;
    let release: (() => void) | undefined;
    await inventoryCommandFaults(page, OpCodes.PlayerFacingRequest, OpCodes.PlayerFacingResponse, true, () => true,
      (opcode, message, deliver) => {
        if (opcode === OpCodes.ServerPlayerMovementNotify) return true;
        if (hold && !release && opcode === OpCodes.PhaserActorsResponse && Buffer.isBuffer(message)) {
          const reply = JSON.parse(message.subarray(6).toString());
          if (reply.success) { release = deliver; return true; }
        }
        return false;
      });
    const character = await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page, "seafoam_1f_runtime_boulder_push_facing_update");
    await waitForNoMapLoading(page); await waitForPlayerTile(page, 18, 11);
    hold = true;
    await pressMovement(page, "up");
    await expect.poll(() => Boolean(release), { timeout: 20_000 }).toBe(true);
    if (mode === "newer-stream") {
      // The first actor read contains the boulder at y=9. A second real push
      // changes it to y=8 while that response is held in the transport.
      const mapId = (await getGameState(page)).map.id!;
      await page.evaluate(async (mapId) => {
        const bridgePath = "/src/net/NetworkBridge.ts";
        const opcodePath = "/src/net/generated/opcodes.ts";
        const { NetworkBridge } = await import(bridgePath);
        const opcodes = await import(opcodePath);
        await NetworkBridge.send({ requestId: "second-owned-push", mapId, fromX: 18, fromY: 10, direction: "UP" }, opcodes.PlayerFacingRequest);
      }, mapId);
      await expect.poll(async () => {
        const boulder = (await getGameState(page)).visibleActors.find(a => a.name === "SeafoamIslands1F_NPC_1");
        return boulder?.y;
      }).toBe(8);
      release!();
      await waitForPlayerTile(page, 18, 9);
      expect((await getGameState(page)).visibleActors.find(a => a.name === "SeafoamIslands1F_NPC_1")?.y).toBe(8);
    } else {
      await quitToCharacterSelect(page);
      release!();
      hold = false;
      await enterWorld(page, character); await waitForNoMapLoading(page);
      await waitForPlayerTile(page, 18, 10);
      expect((await getGameState(page)).visibleActors.find(a => a.name === "SeafoamIslands1F_NPC_1")?.y).toBe(9);
    }
    expect(errors.sentOpcodes.filter(o => o === OpCodes.PlayerStepCompleteRequest)).toEqual([]);
    errors.assertNoSevereErrors();
  });
}
