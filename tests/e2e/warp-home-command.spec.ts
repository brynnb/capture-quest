import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForMap,waitForPlayerTile,waitForNoMapLoading} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import type {WarpHomeResponse} from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";

for(const loseReply of [false,true])test(`Warp Home resets Safari battle once with lostReply=${loseReply}`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const faults=await inventoryCommandFaults<WarpHomeResponse>(page,OpCodes.WarpHomeRequest,OpCodes.WarpHomeResponse,loseReply,request=>request.current!==true);
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"safari_battle_run");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 await expect(page.getByTestId("battle-action-safari-ball")).toBeVisible();expect(await sql(`SELECT count(*) FROM character_safari_state WHERE character_id=${id}`)).toBe("1");
 await page.getByRole("button",{name:"Warp Home",exact:true}).click();await expect.poll(()=>faults.successes).toBe(2);expect(faults.replies[1]).toEqual(faults.replies[0]);
 await waitForMap(page,9999);await waitForPlayerTile(page,9,4);await expect.poll(async()=>(await getGameState(page)).battle.isOpen).toBe(false);
 expect(await sql(`SELECT map_id || ',' || x::int || ',' || y::int FROM character_data WHERE id=${id}`)).toBe("9999,9,4");
 expect(faults.requests).toBe(1);expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("1");
 expect(await sql(`SELECT count(*) FROM character_safari_state WHERE character_id=${id}`)).toBe("0");expect(await sql(`SELECT count(*) FROM character_battle_state WHERE character_id=${id}`)).toBe("0");
 await page.screenshot({path:info.outputPath("warp-home-current.png")});await record({family:"warp-home",loseReply,id,requests:faults.requests,successes:faults.successes,revision:1});await quitToCharacterSelect(page);
});
