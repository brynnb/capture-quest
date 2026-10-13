import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForMap,waitForPlayerTile,waitForPlayerIdle} from "./helpers/state";
import {clickTile} from "./helpers/input";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import * as OpCodes from "../../src/net/generated/opcodes";
import type {ElevatorResponse} from "../../src/net/generated/world_api";

for(const loseReply of [false,true])test(`elevator panel duplicate lostReply=${loseReply} uses current travel recovery`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const faults=await inventoryCommandFaults<ElevatorResponse>(page,OpCodes.ElevatorSelectRequest,OpCodes.ElevatorFloorsResponse,loseReply);
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"celadon_mart_elevator_floors");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 await clickTile(page,3,1);await waitForPlayerTile(page,3,1);await waitForPlayerIdle(page);await clickTile(page,3,0);
 const point=()=>page.evaluate(()=>window.__capturequestTest!.overlayTextToViewport("5F"));await expect.poll(point).not.toBeNull();
 await page.screenshot({path:info.outputPath("elevator-source-menu.png")});const target=await point();await page.mouse.click(target!.x,target!.y);
 await expect.poll(()=>faults.successes).toBe(2);expect(faults.replies[1]).toEqual(faults.replies[0]);
 await waitForMap(page,"CELADON_MART_5F");await waitForPlayerTile(page,1,1);
 expect(faults.requests).toBe(1);expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("1");
 const map=(await getGameState(page)).map.id;expect(await sql(`SELECT map_id || ',' || x::int || ',' || y::int FROM character_data WHERE id=${id}`)).toBe(`${map},1,1`);
 await page.screenshot({path:info.outputPath("elevator-current-pose.png")});await record({family:"elevator",loseReply,id,requests:faults.requests,successes:faults.successes,map});await quitToCharacterSelect(page);
});
