import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {pressSpace,dismissDialogue,pressMovement} from "./helpers/input";
import {getGameState,waitForNoMapLoading,waitForPlayerTile} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {inventoryCommandFaults} from "./helpers/inventoryCommandFaults";
import {observeResponseDelivery} from "./helpers/responseDelivery";
import * as OpCodes from "../../src/net/generated/opcodes";
import type {CutFieldResponse} from "../../src/net/generated/world_api";

for(const mode of ["timeout","reset"] as const)test(`CUT held duplicate ${mode} reconciles only its current visit`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();const faults=await inventoryCommandFaults<CutFieldResponse>(page,OpCodes.FieldMoveUseRequest,OpCodes.FieldMoveUseResponse,true,request=>request.moveName==="CUT");
 await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"debug_field_move_cut_ready");await waitForNoMapLoading(page);await waitForPlayerTile(page,189,-169);
 let delivered=0;await observeResponseDelivery(page,[OpCodes.FieldMoveUseResponse],()=>{delivered++;});const id=(await getGameState(page)).player.internalId!;
 await pressSpace(page);await expect.poll(()=>faults.successes).toBe(2);expect(faults.replies[1]).toEqual(faults.replies[0]);
 const read=()=>page.evaluate(async()=>{const path="/src/phaser-game/services/MapDataService.ts";const {MapDataService}=await import(path);return (await new MapDataService().fetchTilesInBounds(9999,{minX:189,maxX:189,minY:-170,maxY:-170}))[0];});
 if(mode==="timeout"){
  await expect.poll(async()=>(await getGameState(page)).dialogue.isOpen,{timeout:20000}).toBe(true);await dismissDialogue(page);await pressMovement(page,"up");await waitForPlayerTile(page,189,-170);expect((await read()).rawFootTileId).toBe(0x52);
 }else{
  await jumpToScenario(page,"debug_escape_rope_ready");await waitForNoMapLoading(page);await jumpToScenario(page,"debug_field_move_cut_ready");await waitForNoMapLoading(page);expect((await read()).rawFootTileId).toBe(0x3d);
 }
 const before=(await getGameState(page)).player;faults.deliverReply(0);await expect.poll(()=>delivered).toBe(1);
 expect((await getGameState(page)).player).toEqual(before);expect((await getGameState(page)).dialogue.isOpen).toBe(false);expect((await read()).rawFootTileId).toBe(mode==="timeout"?0x52:0x3d);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='cut'`)).toBe("1");expect(faults.requests).toBe(1);
 await page.screenshot({path:info.outputPath(`cut-${mode}-recovered.png`)});await record({family:"cut-command",mode,id,requests:faults.requests,successes:faults.successes,delivered,scope:faults.replies[0].cutScope});await quitToCharacterSelect(page);
});
