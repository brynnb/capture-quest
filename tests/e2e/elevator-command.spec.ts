import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForMap,waitForPlayerTile} from "./helpers/state";
import {pressMovement} from "./helpers/input";
import {openCeladonElevatorPanel,overlayTextPoint,clickOverlayText} from "./helpers/elevator";
import {observeResponseDelivery} from "./helpers/responseDelivery";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import * as OpCodes from "../../src/net/generated/opcodes";
import type {ElevatorResponse} from "../../src/net/generated/world_api";

for(const loseReply of [false,true])test(`elevator panel duplicate lostReply=${loseReply} uses current travel recovery`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const faults=await inventoryCommandFaults<ElevatorResponse>(page,OpCodes.ElevatorSelectRequest,OpCodes.ElevatorFloorsResponse,loseReply);
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"celadon_mart_elevator_floors");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 await openCeladonElevatorPanel(page);
 const point=()=>overlayTextPoint(page,"5F");await expect.poll(point).not.toBeNull();
 await page.screenshot({path:info.outputPath("elevator-source-menu.png")});const target=await point();await page.mouse.click(target!.x,target!.y);
 await expect.poll(()=>faults.successes).toBe(2);expect(faults.replies[1]).toEqual(faults.replies[0]);
 await waitForMap(page,"CELADON_MART_5F");await waitForPlayerTile(page,1,1);
 expect(faults.requests).toBe(1);expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("1");
 const map=(await getGameState(page)).map.id;expect(await sql(`SELECT map_id || ',' || x::int || ',' || y::int FROM character_data WHERE id=${id}`)).toBe(`${map},1,1`);
 await page.screenshot({path:info.outputPath("elevator-current-pose.png")});await record({family:"elevator",loseReply,id,requests:faults.requests,successes:faults.successes,map});await quitToCharacterSelect(page);
});


for(const retirement of ["cancel","blocked-move","scene"] as const)test(`elevator menu ${retirement} retires without travel`,async({page})=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"celadon_mart_elevator_floors");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 await openCeladonElevatorPanel(page);await expect.poll(()=>overlayTextPoint(page,"5F")).not.toBeNull();
 if(retirement==="cancel")await clickOverlayText(page,"CANCEL");
 else if(retirement==="blocked-move"){
  await pressMovement(page,"left");await waitForPlayerTile(page,3,1);expect(await overlayTextPoint(page,"5F")).not.toBeNull();
  await clickOverlayText(page,"CANCEL");await pressMovement(page,"left");await waitForPlayerTile(page,2,1);
 }
 else await jumpToScenario(page,"celadon_mart_elevator_floors");
 await expect.poll(()=>overlayTextPoint(page,"5F")).toBeNull();
 expect(await sql(`SELECT count(*) FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("0");
 await record({family:"elevator-menu-retirement",retirement,id});await quitToCharacterSelect(page);
});

for(const kind of ["list","select"] as const)test(`elevator held ${kind} reply retires through character reentry`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const request=kind==="list"?OpCodes.ElevatorFloorsRequest:OpCodes.ElevatorSelectRequest;
 const faults=await inventoryCommandFaults<ElevatorResponse>(page,request,OpCodes.ElevatorFloorsResponse,true);
 const {sql,record}=await isolatedCrashRuntime();const character=await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"celadon_mart_elevator_floors");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 let delivered=0;await observeResponseDelivery(page,[OpCodes.ElevatorFloorsResponse],()=>delivered++);
 await openCeladonElevatorPanel(page);if(kind==="select")await clickOverlayText(page,"5F");
 await expect.poll(()=>faults.successes).toBe(2);expect(faults.requests).toBe(1);
 await quitToCharacterSelect(page);await enterWorld(page,character);await waitForNoMapLoading(page);
 await waitForMap(page,kind==="select"?"CELADON_MART_5F":"CELADON_MART_ELEVATOR");await waitForPlayerTile(page,kind==="select"?1:3,1);
 const before=(await getGameState(page)).player;const count=delivered;faults.deliverReply(0);await expect.poll(()=>delivered).toBe(count+1);
 expect((await getGameState(page)).player).toEqual(before);expect((await getGameState(page)).dialogue.isOpen).toBe(false);expect(await overlayTextPoint(page,"5F")).toBeNull();
 expect(await sql(`SELECT COALESCE((SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'),0)`)).toBe(kind==="select"?"1":"0");
 await page.screenshot({path:info.outputPath(`elevator-retired-${kind}.png`)});await record({family:"elevator-held-reentry",kind,id,delivered,requests:faults.requests,position:before});await quitToCharacterSelect(page);
});

test("held elevator list cannot reopen after owned source warp and return",async({page})=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const faults=await inventoryCommandFaults<ElevatorResponse>(page,OpCodes.ElevatorFloorsRequest,OpCodes.ElevatorFloorsResponse,true);
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"celadon_mart_elevator_floors");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 let delivered=0;await observeResponseDelivery(page,[OpCodes.ElevatorFloorsResponse],()=>delivered++);
 await openCeladonElevatorPanel(page);await expect.poll(()=>faults.successes).toBe(2);
 const map=(await getGameState(page)).map.id!;
 await page.evaluate(map=>window.__capturequestTest!.warpToMap(map,2,1),map);await waitForPlayerTile(page,2,1);
 await page.evaluate(map=>window.__capturequestTest!.warpToMap(map,3,1),map);await waitForPlayerTile(page,3,1);
 faults.deliverReply(0);await expect.poll(()=>delivered).toBe(1);expect(await overlayTextPoint(page,"5F")).toBeNull();expect((await getGameState(page)).dialogue.isOpen).toBe(false);
 expect(await sql(`SELECT count(*) FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("0");await record({family:"elevator-list-source-retirement",id,delivered});await quitToCharacterSelect(page);
});
