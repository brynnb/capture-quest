import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForMap,waitForPlayerTile} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import {observeResponseDelivery} from "./helpers/responseDelivery";
import * as OpCodes from "../../src/net/generated/opcodes";
import type {StateFieldResponse} from "../../src/net/generated/world_api";

for(const mode of ["timeout","reentry"] as const)test(`TELEPORT held duplicate ${mode} projects current owned position`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();const faults=await inventoryCommandFaults<StateFieldResponse>(page,OpCodes.FieldMoveUseRequest,OpCodes.FieldMoveUseResponse,true,request=>request.moveName==="TELEPORT");
 const character=await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"field_move_teleport_no_badge_required");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 const destination=await sql("SELECT id FROM phaser_maps WHERE name='REDS_HOUSE_2F'");expect(destination).toMatch(/^\d+$/);
 await sql(`INSERT INTO character_bind(id,slot,map_id,x,y,z,heading) VALUES(${id},0,${destination},3,6,0,0) ON CONFLICT(id,slot) DO UPDATE SET map_id=EXCLUDED.map_id,x=EXCLUDED.x,y=EXCLUDED.y`);
 let delivered=0;await observeResponseDelivery(page,[OpCodes.FieldMoveUseResponse],()=>{delivered++;});
 await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText("teleport",{exact:true}).click();await expect.poll(()=>faults.successes).toBe(2);expect(faults.replies[1]).toEqual(faults.replies[0]);
 if(mode==="reentry"){await quitToCharacterSelect(page);await enterWorld(page,character);await waitForNoMapLoading(page);}
 await waitForMap(page,"REDS_HOUSE_2F");await waitForPlayerTile(page,3,6);
 const before=(await getGameState(page)).player;faults.deliverReply(0);await expect.poll(()=>delivered).toBe(1);
 expect((await getGameState(page)).player).toEqual(before);expect((await getGameState(page)).dialogue.isOpen).toBe(false);expect(faults.requests).toBe(1);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("1");expect(await sql(`SELECT map_id || ',' || x::int || ',' || y::int FROM character_data WHERE id=${id}`)).toBe(`${destination},3,6`);
 await page.screenshot({path:info.outputPath(`travel-${mode}-owned.png`)});await record({family:"ability-travel",mode,id,requests:faults.requests,successes:faults.successes,delivered,position:before});await quitToCharacterSelect(page);
});
