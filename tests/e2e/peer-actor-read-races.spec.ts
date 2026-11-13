import { expect, test } from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import { loginAsGuest, createCharacter, uniqueTrainerName, createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { collectPageErrors } from "./helpers/errors";
import { pressMovement } from "./helpers/input";
import { getGameState, waitForNoMapLoading, waitForPlayerTile } from "./helpers/state";

for (const mode of ["quit", "replacement"] as const) {
  test(`late peer snapshot cannot resurrect or rewind ${mode}`, async ({ page, browser, baseURL }) => {
    test.setTimeout(120_000);
    const peerContext = await browser.newContext({ baseURL });
    const peer = await peerContext.newPage();
    const errors = collectPageErrors(page);
    const peerErrors = collectPageErrors(peer);
    let hold = false;
    let release: (() => void) | undefined;
    let peerId: number | undefined;
    let pendingRead: Promise<string> | undefined;
    await inventoryCommandFaults(page, OpCodes.PlayerFacingRequest, OpCodes.PlayerFacingResponse, false, () => false,
      (opcode, message, deliver) => {
        if (hold && !release && opcode === OpCodes.PhaserActorsResponse && Buffer.isBuffer(message)) {
          const reply = JSON.parse(message.subarray(6).toString());
          if (reply.success && reply.actors.some((actor: { internalId: number; objectType: string }) => actor.objectType === "player" && actor.internalId === peerId)) {
            release = deliver;
            return true;
          }
        }
        return false;
      });
    try {
      await loginAsGuest(page);
      const observer = uniqueTrainerName();
      await createCharacter(page, observer);
      await page.evaluate(async () => {
        const path = "/src/phaser-game/renderers/MapRenderer.ts";
        const { MapRenderer } = await import(path);
        const render = MapRenderer.prototype.renderMap;
        MapRenderer.prototype.renderMap = function (...args: any[]) {
          (window as any).__peerReadScene = this.scene;
          return render.apply(this, args as any);
        };
      });
      await enterWorld(page, observer);
      await waitForNoMapLoading(page);
      const character = await createGuestCharacterAndEnterWorld(peer);
      await waitForNoMapLoading(peer);
      peerId = (await getGameState(peer)).player.internalId!;
      expect(Number.isSafeInteger(peerId)).toBe(true);
      const observed = async () => (await getGameState(page)).visibleActors.filter(actor => actor.type === "player" && actor.internalId === peerId);
      await expect.poll(async () => (await observed()).length).toBe(1);
      expect((await getGameState(page)).map.id).toBe(38);
      await waitForPlayerTile(peer, 3, 6);
      hold = true;
      pendingRead = page.evaluate(async () => {
        await (window as any).__peerReadScene.reconcileActors(new AbortController().signal);
      }).then(() => "applied", error => String(error));
      await expect.poll(() => Boolean(release)).toBe(true);
      await quitToCharacterSelect(peer);
      await expect.poll(async () => (await observed()).length).toBe(0);
      if (mode === "replacement") {
        await peer.reload();
        await peer.getByRole("button", { name: "PLAY AS GUEST" }).click();
        await expect(peer.getByRole("heading", { name: "SELECT A CHARACTER" })).toBeVisible();
        await enterWorld(peer, character);
        await waitForNoMapLoading(peer);
        await expect.poll(async () => (await observed()).length).toBe(1);
        // Canonical map 38 has walkable (4,6); (3,5) is a blocked bed.
        await pressMovement(peer, "right");
        await waitForPlayerTile(peer, 4, 6);
        await expect.poll(async () => (await observed())[0]?.x).toBe(4);
      }
      const deliver = release!;
      release = undefined;
      hold = false;
      deliver();
      expect(await pendingRead).toBe("applied");
      if (mode === "quit") expect(await observed()).toEqual([]);
      else {
        const actors = await observed();
        expect(actors).toHaveLength(1);
        expect(actors[0]).toMatchObject({ x: 4, y: 6 });
      }
      errors.assertNoSevereErrors();
      peerErrors.assertNoSevereErrors();
    } finally {
      // Release a held read before disposing only this test's peer context.
      release?.();
      await pendingRead;
      await peerContext.close();
    }
  });
}
