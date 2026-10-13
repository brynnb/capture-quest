import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import * as OpCodes from "../../src/net/generated/opcodes";
import type {StateFieldResponse} from "../../src/net/generated/world_api";

for(const loseReply of [false,true])test(`party STRENGTH duplicate lost reply=${loseReply} recovers owned state`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();
 const faults=await inventoryCommandFaults<StateFieldResponse>(page,OpCodes.FieldMoveUseRequest,OpCodes.FieldMoveUseResponse,loseReply,request=>request.moveName==="STRENGTH");
 await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"seafoam_1f_runtime_boulder_push_facing_update");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const id=(await getGameState(page)).player.internalId!;
 await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText("strength",{exact:true}).click();
 await expect.poll(()=>faults.successes).toBe(2);expect(faults.rejections).toBe(0);expect(faults.replies[1]).toEqual(faults.replies[0]);
 await expect.poll(async()=>(await getGameState(page)).dialogue.isOpen,{timeout:20000}).toBe(true);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='field-state'`)).toBe("1");
 expect(await sql(`SELECT active FROM character_field_move_state WHERE character_id=${id} AND move_name='STRENGTH'`)).toBe("1");expect(faults.requests).toBe(1);
 await page.screenshot({path:info.outputPath("state-field-owned.png")});await record({family:"state-field",loseReply,id,requests:faults.requests,successes:faults.successes,rejections:faults.rejections});await quitToCharacterSelect(page);
});
