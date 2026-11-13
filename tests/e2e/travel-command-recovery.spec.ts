import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForMap,waitForPlayerTile} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import {observeResponseDelivery} from "./helpers/responseDelivery";
import * as OpCodes from "../../src/net/generated/opcodes";
import type {StateFieldResponse} from "../../src/net/generated/world_api";

for(const move of ["TELEPORT","FLY"] as const)for(const mode of ["timeout","reentry"] as const)test(`${move} held duplicate ${mode} projects current owned position`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();const faults=await inventoryCommandFaults<StateFieldResponse>(page,OpCodes.FieldMoveUseRequest,OpCodes.FieldMoveUseResponse,true,request=>request.moveName===move);
 const character=await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,move==="FLY"?"field_move_fly_with_thunder_badge":"field_move_teleport_no_badge_required");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 const choice=move==="FLY"?JSON.parse(await sql("SELECT json_build_object('name',c.name,'mapId',CASE WHEN m.is_overworld=1 THEN 9999 ELSE c.map_id END,'x',c.spawn_x,'y',c.spawn_y) FROM poke_start_cities c JOIN phaser_maps m ON m.id=c.map_id ORDER BY c.id LIMIT 1")):{name:"REDS_HOUSE_2F",mapId:Number(await sql("SELECT id FROM phaser_maps WHERE name='REDS_HOUSE_2F'")),x:3,y:6};
 if(move==="TELEPORT")await sql(`INSERT INTO character_bind(id,slot,map_id,x,y,z,heading) VALUES(${id},0,${choice.mapId},3,6,0,0) ON CONFLICT(id,slot) DO UPDATE SET map_id=EXCLUDED.map_id,x=EXCLUDED.x,y=EXCLUDED.y`);
 let delivered=0;await observeResponseDelivery(page,[OpCodes.FieldMoveUseResponse],()=>{delivered++;});
 await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText(move.toLowerCase(),{exact:true}).click();if(move==="FLY")await page.getByText(choice.name,{exact:true}).click();await expect.poll(()=>faults.successes).toBe(2);expect(faults.replies[1]).toEqual(faults.replies[0]);
 if(mode==="reentry"){await quitToCharacterSelect(page);await enterWorld(page,character);await waitForNoMapLoading(page);}
 await waitForMap(page,choice.mapId);await waitForPlayerTile(page,choice.x,choice.y);
 const before=(await getGameState(page)).player;faults.deliverReply(0);await expect.poll(()=>delivered).toBe(1);
 expect((await getGameState(page)).player).toEqual(before);expect((await getGameState(page)).dialogue.isOpen).toBe(false);expect(faults.requests).toBe(1);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("1");expect(await sql(`SELECT map_id || ',' || x::int || ',' || y::int FROM character_data WHERE id=${id}`)).toBe(`${choice.mapId},${choice.x},${choice.y}`);
 await page.screenshot({path:info.outputPath(`travel-${move}-${mode}-owned.png`)});await record({family:"ability-travel",move,mode,id,requests:faults.requests,successes:faults.successes,delivered,position:before});await quitToCharacterSelect(page);
});
