import {expect,test} from "@playwright/test";
import {loginAsGuest,createCharacter,enterWorld,uniqueTrainerName} from "./helpers/auth";
import {waitForNoMapLoading} from "./helpers/state";
import {collectPageErrors} from "./helpers/errors";
import {isolatedCrashRuntime} from "./helpers/processRecovery";

test("initial map publication rereads a tile erased during image preparation",async({page})=>{
 test.setTimeout(90000);const errors=collectPageErrors(page);const {sql}=await isolatedCrashRuntime();
 await loginAsGuest(page);const name=uniqueTrainerName();await createCharacter(page,name);
 await page.evaluate(async()=>{
  const dataPath="/src/phaser-game/services/MapDataService.ts",tilePath="/src/phaser-game/managers/TileManager.ts",renderPath="/src/phaser-game/renderers/MapRenderer.ts";
  const {MapDataService}=await import(dataPath);const {TileManager}=await import(tilePath);const {MapRenderer}=await import(renderPath);
  const state:any={reads:0};(window as any).__initialTilePublication=state;
  const fetch=MapDataService.prototype.fetchTiles;
  MapDataService.prototype.fetchTiles=async function(...args:any[]){const tiles=await fetch.apply(this,args as any);state.reads++;if(!state.tile)state.tile=tiles[0];return tiles;};
  const load=TileManager.prototype.loadTileImages;let first=true;
  TileManager.prototype.loadTileImages=async function(...args:any[]){if(first){first=false;state.held=true;await new Promise<void>(resolve=>state.release=resolve);}return load.apply(this,args as any);};
  const render=MapRenderer.prototype.renderMap;
  MapRenderer.prototype.renderMap=function(...args:any[]){state.renderer=this;return render.apply(this,args as any);};
 });
 const entry=enterWorld(page,name);
 await expect.poll(()=>page.evaluate(()=>Boolean((window as any).__initialTilePublication?.held)),{timeout:20000}).toBe(true);
 const tile=await page.evaluate(()=>(window as any).__initialTilePublication.tile) as {id:number;mapId:number;x:number;y:number};
 expect(Number.isSafeInteger(tile.id)).toBe(true);expect(tile.id).toBeGreaterThan(0);
 await sql(`UPDATE phaser_tiles SET is_tile_erased=1,has_tile_edit=1 WHERE id=${tile.id}`);
 await page.evaluate(tile=>{window.dispatchEvent(new CustomEvent("worldTileUpdate",{detail:{mapId:tile.mapId,tiles:[{x:tile.x,y:tile.y,tileImageId:0,collisionType:0,erased:true}]}}));(window as any).__initialTilePublication.release();},tile);
 await entry;await waitForNoMapLoading(page);
 const result=await page.evaluate(tile=>{const state=(window as any).__initialTilePublication;return{reads:state.reads,erased:!state.renderer.tileDataMap.has(`${tile.x},${tile.y}`)};},tile);
 expect(result.reads).toBeGreaterThanOrEqual(2);expect(result.erased).toBe(true);errors.assertNoSevereErrors();
});
