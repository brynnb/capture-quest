import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {clickTile} from "./helpers/input";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle,waitForMessage} from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";
import type {ItemPickupResponse} from "../../src/net/generated/world_api";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import {observeResponseDelivery} from "./helpers/responseDelivery";
import {isolatedCrashRuntime} from "./helpers/processRecovery";

for(const mode of ["normal","timeout","reentry","process"] as const){
test(`owned item pickup ${mode} commits once and reconciles actors`,async({page},testInfo)=>{
 test.skip(process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires private catalog fixture and exact-process reload");test.setTimeout(90000);
 const {sql,crash,record}=await isolatedCrashRuntime();
 const map=await sql("SELECT id FROM phaser_maps WHERE name='REDS_HOUSE_2F'");
 const existing=await sql("SELECT id FROM phaser_objects WHERE name='CQ_PICKUP_FIXTURE' LIMIT 1");
 const object=existing || await sql("SELECT COALESCE(max(id),0)+1 FROM phaser_objects");
 expect(map).toMatch(/^\d+$/);expect(object).toMatch(/^\d+$/);
 // Reuse a real catalog item/sprite; this additional actor exists only in the
 // isolated PostgreSQL fixture and does not alter generated source assets.
 if(!existing)await sql(`INSERT INTO phaser_objects(id,map_id,x,y,local_x,local_y,object_type,name,item_id,sprite_name,action_direction,movement_type) SELECT ${object},${map},4,6,4,6,'item','CQ_PICKUP_FIXTURE',item_id,sprite_name,'DOWN','LAND' FROM phaser_objects WHERE object_type='item' AND item_id IS NOT NULL AND sprite_name IS NOT NULL ORDER BY id LIMIT 1`);
 expect(await sql(`SELECT count(*) FROM phaser_objects WHERE id=${object}`)).toBe("1");
 const item=await sql(`SELECT item_id FROM phaser_objects WHERE id=${object}`);expect(item).toMatch(/^\d+$/);
 const restart=await crash();
 const faults=await inventoryCommandFaults<ItemPickupResponse>(page,OpCodes.ItemPickupRequest,OpCodes.ItemPickupResponse,mode!=="normal");
 let delivered=0;
 const character=await createGuestCharacterAndEnterWorld(page);await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 await observeResponseDelivery(page,[OpCodes.ItemPickupResponse],()=>{delivered++;});
 const id=(await getGameState(page)).player.internalId!;
 const quantity=()=>sql(`SELECT COALESCE(sum(ii.quantity),0) FROM cq_character_inventory ci JOIN cq_item_instances ii ON ii.id=ci.item_instance_id WHERE ci.character_id=${id} AND ii.item_id=${item}`);
 const before=Number(await quantity());
 await expect.poll(async()=>(await getGameState(page)).visibleActors.some(actor=>actor.name==='CQ_PICKUP_FIXTURE')).toBe(true);
 await clickTile(page,4,6);await expect.poll(()=>faults.successes).toBe(1);await expect.poll(()=>faults.rejections).toBe(1);
 await expect.poll(quantity).toBe(String(before+1));
 let committedRestart:unknown;
 if(mode==="normal")await waitForMessage(page,/Picked up /i);
 if(mode==="timeout")await waitForMessage(page,/Item state refreshed/i);
 if(mode==="reentry"){await quitToCharacterSelect(page);await enterWorld(page,character);await waitForNoMapLoading(page);}
 if(mode==="process"){
  committedRestart=await crash();await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible({timeout:20000});await page.getByRole("button",{name:"PLAY AS GUEST"}).click();await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();await enterWorld(page,character);await waitForNoMapLoading(page);
 }
 await expect.poll(async()=>(await getGameState(page)).visibleActors.some(actor=>actor.name==='CQ_PICKUP_FIXTURE')).toBe(false);
 expect(await sql(`SELECT count(*) FROM character_collected_items WHERE character_id=${id} AND object_id=${object}`)).toBe("1");
 expect(await sql(`SELECT revision FROM character_shop_state WHERE character_id=${id}`)).toBe("1");
 expect(faults.requests).toBe(1);
 const bagRevision=()=>page.evaluate(async()=>{const path="/src/stores/CQInventoryStore.ts";return (await import(path)).default.getState().commandRevision;});
 await expect.poll(bagRevision).toBe(1);
 const beforeDelivery=delivered;
 if(mode!=="normal"){
  if(mode==="process")await page.evaluate(async reply=>{const path="/src/net/index.ts";const {WorldSocket}=await import(path);WorldSocket.onJson?.(107,reply);},faults.replies[0]);
  else faults.deliverReply(0);
  await expect.poll(()=>delivered).toBe(beforeDelivery+1);
 }
 expect(await quantity()).toBe(String(before+1));expect(await bagRevision()).toBe(1);
 expect((await getGameState(page)).visibleActors.some(actor=>actor.name==='CQ_PICKUP_FIXTURE')).toBe(false);
 await page.screenshot({path:testInfo.outputPath(`owned-item-pickup-${mode}.png`)});await record({family:"item-pickup-command",mode,id,object,item,before,quantity:before+1,requests:faults.requests,successes:faults.successes,rejections:faults.rejections,delivered,restart,committedRestart});await quitToCharacterSelect(page);
});

}
