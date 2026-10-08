import { expect, test } from "@playwright/test";
import { loginAsGuest, createCharacter, enterWorld, uniqueTrainerName } from "./helpers/auth";
import { getGameState, waitForMap, waitForNoMapLoading, waitForPlayerIdle } from "./helpers/state";
import { collectPageErrors } from "./helpers/errors";
import { isolatedCrashRuntime } from "./helpers/processRecovery";

for (const view of ["interior", "unified"] as const) {
  test(`${view} resident tile reconciliation recovers an omitted update`, async ({ page }) => {
    test.setTimeout(90_000);
    const errors = collectPageErrors(page);
    const { sql } = await isolatedCrashRuntime();
    await loginAsGuest(page);
    const name = uniqueTrainerName();
    await createCharacter(page, name);
    await page.evaluate(async () => {
      const path = "/src/phaser-game/renderers/MapRenderer.ts";
      const { MapRenderer, MAP_TILE_CHUNK_SIZE } = await import(path);
      const state: any = { chunks: new Map(), chunkSize: MAP_TILE_CHUNK_SIZE };
      (window as any).__residentRecovery = state;
      const render = MapRenderer.prototype.renderMap;
      MapRenderer.prototype.renderMap = function (...args: any[]) {
        state.renderer = this;
        state.tile = args[0][0];
        return render.apply(this, args as any);
      };
      const upsert = MapRenderer.prototype.upsertTileChunk;
      MapRenderer.prototype.upsertTileChunk = function (...args: any[]) {
        state.renderer = this;
        state.chunks.set(args[0], args[3]);
        return upsert.apply(this, args as any);
      };
    });
    await enterWorld(page, name);
    await waitForNoMapLoading(page);
    if (view === "unified") {
      await page.getByRole("button", { name: "Warp Home" }).click();
      await waitForMap(page, /Kanto|Unified Overworld/);
      await waitForNoMapLoading(page);
      await waitForPlayerIdle(page);
      expect((await getGameState(page)).map.id).toBe(9999);
    }
    const player = (await getGameState(page)).player;
    const tile = await page.evaluate(({ view, player }) => {
      const state = (window as any).__residentRecovery;
      if (view === "interior") return state.tile;
      // Pick an actual resident tile in the player's required chunk, not a
      // speculative camera chunk that recovery may legitimately leave untouched.
      return [...state.chunks.values()].flat().find((tile: any) =>
        Math.floor(tile.x / state.chunkSize) === Math.floor(player.x! / state.chunkSize) &&
        Math.floor(tile.y / state.chunkSize) === Math.floor(player.y! / state.chunkSize));
    }, { view, player }) as { id: number; x: number; y: number };
    expect(Number.isSafeInteger(tile?.id)).toBe(true);
    expect(tile.id).toBeGreaterThan(0);
    await sql(`UPDATE phaser_tiles SET is_tile_erased=1,has_tile_edit=1 WHERE id=${tile.id}`);
    const before = await page.evaluate(tile => {
      const renderer = (window as any).__residentRecovery.renderer;
      return { render: renderer.tileDataMap.has(`${tile.x},${tile.y}`),
        collision: renderer.scene.playerMovementController.collisionMap.has(`${tile.x},${tile.y}`) };
    }, tile);
    expect(before).toEqual({ render: true, collision: true });
    const after = await page.evaluate(async tile => {
      const renderer = (window as any).__residentRecovery.renderer;
      await renderer.scene.reconcileResidentTiles(new AbortController().signal);
      return { render: renderer.tileDataMap.has(`${tile.x},${tile.y}`),
        collision: renderer.scene.playerMovementController.collisionMap.has(`${tile.x},${tile.y}`) };
    }, tile);
    expect(after).toEqual({ render: false, collision: false });
    errors.assertNoSevereErrors();
  });
}
