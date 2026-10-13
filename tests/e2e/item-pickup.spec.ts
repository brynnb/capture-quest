import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,quitToCharacterSelect} from "./helpers/auth";
import {clickTile} from "./helpers/input";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle,waitForMessage} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";

test("adjacent item click commits one owned grant and collection marker",async({page},testInfo)=>{
 test.skip(process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires private catalog fixture and exact-process reload");test.setTimeout(90000);
 const {sql,crash,record}=await isolatedCrashRuntime();
 const map=await sql("SELECT id FROM phaser_maps WHERE name='REDS_HOUSE_2F'");
 const object=await sql("SELECT COALESCE(max(id),0)+1 FROM phaser_objects");
 expect(map).toMatch(/^\d+$/);expect(object).toMatch(/^\d+$/);
 // Reuse a real catalog item/sprite; this additional actor exists only in the
 // isolated PostgreSQL fixture and does not alter generated source assets.
 await sql(`INSERT INTO phaser_objects(id,map_id,x,y,local_x,local_y,object_type,name,item_id,sprite_name,action_direction,movement_type) SELECT ${object},${map},4,6,4,6,'item','CQ_PICKUP_FIXTURE',item_id,sprite_name,'DOWN','LAND' FROM phaser_objects WHERE object_type='item' AND item_id IS NOT NULL AND sprite_name IS NOT NULL ORDER BY id LIMIT 1`);
 expect(await sql(`SELECT count(*) FROM phaser_objects WHERE id=${object}`)).toBe("1");
 const item=await sql(`SELECT item_id FROM phaser_objects WHERE id=${object}`);expect(item).toMatch(/^\d+$/);
 const restart=await crash();await createGuestCharacterAndEnterWorld(page);await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const id=(await getGameState(page)).player.internalId!;
 const quantity=()=>sql(`SELECT COALESCE(sum(ii.quantity),0) FROM cq_character_inventory ci JOIN cq_item_instances ii ON ii.id=ci.item_instance_id WHERE ci.character_id=${id} AND ii.item_id=${item}`);
 const before=Number(await quantity());
 await expect.poll(async()=>(await getGameState(page)).visibleActors.some(actor=>actor.name==='CQ_PICKUP_FIXTURE')).toBe(true);
 await clickTile(page,4,6);await waitForMessage(page,/Picked up /i);
 await expect.poll(quantity).toBe(String(before+1));
 expect(await sql(`SELECT count(*) FROM character_collected_items WHERE character_id=${id} AND object_id=${object}`)).toBe("1");
 await page.screenshot({path:testInfo.outputPath("owned-item-pickup.png")});await record({family:"item-pickup-source",id,object,item,before,quantity:before+1,restart});await quitToCharacterSelect(page);
});
