import {expect,test} from "@playwright/test";
import {loginAsGuest,createCharacter,enterWorld,uniqueTrainerName} from "./helpers/auth";
import {waitForNoMapLoading,getGameState} from "./helpers/state";
import {collectPageErrors} from "./helpers/errors";

test("another-map update and delayed older texture cannot overwrite the current tile view",async({page})=>{
 test.setTimeout(90000);const errors=collectPageErrors(page);await loginAsGuest(page);
 const name=uniqueTrainerName();await createCharacter(page,name);
 await page.evaluate(async()=>{
  const path="/src/phaser-game/renderers/MapRenderer.ts";const {MapRenderer}=await import(path);
  const state:any={};(window as any).__tileOrdering=state;
  const render=MapRenderer.prototype.renderMap;
  MapRenderer.prototype.renderMap=function(...args:any[]){state.renderer=this;state.tile=args[0][0];return render.apply(this,args as any);};
 });
 await enterWorld(page,name);await waitForNoMapLoading(page);const game=await getGameState(page);
 const result=await page.evaluate(async mapId=>{
  const state=(window as any).__tileOrdering;const renderer=state.renderer;const tile=state.tile;
  if(!renderer || !tile)throw new Error("rendered map tile not captured");
  const key=`${tile.x},${tile.y}`;const before=renderer.tileDataMap.get(key);
  window.dispatchEvent(new CustomEvent("worldTileUpdate",{detail:{mapId:mapId+1,tiles:[{x:tile.x,y:tile.y,tileImageId:0,collisionType:0,erased:true}]}}));
  const otherMapIgnored=renderer.tileDataMap.get(key)===before;
  const original=renderer.loadTileTextureIfNeeded;let release!:()=>void;
  renderer.loadTileTextureIfNeeded=()=>new Promise<void>(resolve=>{release=resolve});
  try{
   window.dispatchEvent(new CustomEvent("worldTileUpdate",{detail:{mapId,tiles:[{x:tile.x,y:tile.y,tileImageId:tile.tileImageId,collisionType:0}]}}));
   window.dispatchEvent(new CustomEvent("worldTileUpdate",{detail:{mapId,tiles:[{x:tile.x,y:tile.y,tileImageId:0,collisionType:0,erased:true}]}}));
   release();await Promise.resolve();await Promise.resolve();
   return{otherMapIgnored,removed:!renderer.tileDataMap.has(key)};
  }finally{renderer.loadTileTextureIfNeeded=original;}
 },game.map.id!);
 expect(result).toEqual({otherMapIgnored:true,removed:true});errors.assertNoSevereErrors();
});
