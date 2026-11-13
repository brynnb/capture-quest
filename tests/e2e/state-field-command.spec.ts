import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle} from "./helpers/state";
import {observeResponseDelivery} from "./helpers/responseDelivery";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import * as OpCodes from "../../src/net/generated/opcodes";
import type {StateFieldResponse} from "../../src/net/generated/world_api";

for(const move of ["STRENGTH","FLASH"] as const)for(const loseReply of [false,true])test(`party ${move} duplicate lost reply=${loseReply} recovers owned state`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();
 const faults=await inventoryCommandFaults<StateFieldResponse>(page,OpCodes.FieldMoveUseRequest,OpCodes.FieldMoveUseResponse,loseReply,request=>request.moveName===move);
 await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,move==="STRENGTH"?"seafoam_1f_runtime_boulder_push_facing_update":"field_move_flash_with_boulder_badge");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const id=(await getGameState(page)).player.internalId!;
 await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText(move.toLowerCase(),{exact:true}).click();
 await expect.poll(()=>faults.successes).toBe(2);expect(faults.rejections).toBe(0);expect(faults.replies[1]).toEqual(faults.replies[0]);
 await expect.poll(async()=>(await getGameState(page)).dialogue.isOpen,{timeout:20000}).toBe(true);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='field-state'`)).toBe("1");
 expect(await sql(`SELECT active FROM character_field_move_state WHERE character_id=${id} AND move_name='${move}'`)).toBe("1");expect(faults.requests).toBe(1);
 await page.screenshot({path:info.outputPath("state-field-owned.png")});await record({family:"state-field",move,loseReply,id,requests:faults.requests,successes:faults.successes,rejections:faults.rejections});await quitToCharacterSelect(page);
});


for(const mode of ["reentry","source"] as const)test(`held FLASH reply stays retired after ${mode}`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();
 const faults=await inventoryCommandFaults<StateFieldResponse>(page,OpCodes.FieldMoveUseRequest,OpCodes.FieldMoveUseResponse,true,request=>request.moveName==="FLASH");
 const character=await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"field_move_flash_with_boulder_badge");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 let delivered=0;await observeResponseDelivery(page,[OpCodes.FieldMoveUseResponse],()=>{delivered++;});
 const id=(await getGameState(page)).player.internalId!;
 await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText("flash",{exact:true}).click();await expect.poll(()=>faults.successes).toBe(2);
 if(mode==="reentry"){await quitToCharacterSelect(page);await enterWorld(page,character);await waitForNoMapLoading(page);}
 else {await jumpToScenario(page,"debug_field_move_surf_ready");await waitForNoMapLoading(page);}
 await waitForPlayerIdle(page);const before=(await getGameState(page)).player;
 expect((await getGameState(page)).dialogue.isOpen).toBe(false);
 faults.deliverReply(0);await expect.poll(()=>delivered).toBe(1);
 expect((await getGameState(page)).dialogue.isOpen).toBe(false);expect((await getGameState(page)).player).toEqual(before);expect(faults.requests).toBe(1);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='field-state'`)).toBe("1");
 if(mode==="source")expect(await sql(`SELECT count(*) FROM character_field_move_state WHERE character_id=${id}`)).toBe("0");
 else expect(await sql(`SELECT active FROM character_field_move_state WHERE character_id=${id} AND move_name='FLASH'`)).toBe("1");
 await page.screenshot({path:info.outputPath(`held-flash-${mode}.png`)});await record({family:"state-field-retirement",mode,id,delivered,requests:faults.requests,position:before});await quitToCharacterSelect(page);
});
