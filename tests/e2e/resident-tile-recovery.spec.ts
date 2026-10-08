import {expect,test} from "@playwright/test";
import {loginAsGuest,createCharacter,enterWorld,uniqueTrainerName} from "./helpers/auth";
import {waitForNoMapLoading} from "./helpers/state";
import {collectPageErrors} from "./helpers/errors";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
test("owned resident tile reconciliation recovers an omitted update",async({page})=>{
 test.setTimeout(90000);const errors=collectPageErrors(page);const{sql}=await isolatedCrashRuntime();await loginAsGuest(page);const name=uniqueTrainerName();await createCharacter(page,name);
 await page.evaluate(async()=>{
  const path="/src/phaser-game/renderers/MapRenderer.ts";const{MapRenderer}=await import(path);const state:any={};(window as any).__residentRecovery=state;
  const render=MapRenderer.prototype.renderMap;
  MapRenderer.prototype.renderMap=function(...args:any[]){state.renderer=this;state.tile=args[0][0];return render.apply(this,args as any);};
 });
 await enterWorld(page,name);await waitForNoMapLoading(page);
 const tile=await page.evaluate(()=>(window as any).__residentRecovery.tile) as{id:number;x:number;y:number};expect(Number.isSafeInteger(tile.id)).toBe(true);expect(tile.id).toBeGreaterThan(0);
 await sql(`UPDATE phaser_tiles SET is_tile_erased=1,has_tile_edit=1 WHERE id=${tile.id}`);
 const before=await page.evaluate(tile=>(window as any).__residentRecovery.renderer.tileDataMap.has(`${tile.x},${tile.y}`),tile);expect(before).toBe(true);
 const after=await page.evaluate(async tile=>{
  const renderer=(window as any).__residentRecovery.renderer;
  await renderer.scene.reconcileResidentTiles(new AbortController().signal);
  return renderer.tileDataMap.has(`${tile.x},${tile.y}`);
 },tile);
 expect(after).toBe(false);errors.assertNoSevereErrors();
});
