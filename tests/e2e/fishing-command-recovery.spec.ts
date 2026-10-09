import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle,waitForInventoryOpen} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {observeResponseDelivery} from "./helpers/responseDelivery";

for(const mode of ["timeout","reentry"] as const){
 test(`committed fishing ${mode} recovers without another rod command`,async({page,context},testInfo)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();let requests=0;let release:(()=>void)|undefined;let oldId="";let delivered=0;
 await context.routeWebSocket("**/ws",socket=>{
 const server=socket.connectToServer();socket.onMessage(message=>{if(Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.PokeFishingRequest){const req=JSON.parse(message.subarray(6).toString());expect(req.requestId).toBeTruthy();expect(req.instanceId).toBeGreaterThan(0);requests++;}server.send(message);});
 server.onMessage(message=>{if(Buffer.isBuffer(message) && message.length>=6){const opcode=message.readUInt16LE(4);if(opcode===OpCodes.PokeFishingResponse){const reply=JSON.parse(message.subarray(6).toString());if(reply.success && !release){oldId=reply.requestId;release=()=>socket.send(message);}return;}if(opcode===OpCodes.PokeBattleStartResponse)return;}socket.send(message);});
 });
 const character=await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"debug_inventory_old_rod_water_ready");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const id=(await getGameState(page)).player.internalId!;await observeResponseDelivery(page,[OpCodes.PokeFishingResponse],(_opcode,requestId)=>{if(requestId===oldId)delivered++;});
 await page.getByRole("button",{name:"Bag",exact:true}).click();await waitForInventoryOpen(page,true);await page.getByTestId("inventory-item-old-rod").click();
 await expect.poll(()=>Boolean(release)).toBe(true);expect(await sql(`SELECT count(*) FROM character_battle_state WHERE character_id=${id}`)).toBe("1");
 if(mode==="reentry"){await quitToCharacterSelect(page);await enterWorld(page,character);await waitForNoMapLoading(page);}
 await expect(page.getByTestId("battle-overlay")).toBeVisible({timeout:20000});expect(requests).toBe(1);
 const before=await page.evaluate(async()=>{const path="/src/stores/PokeBattleStore.ts";const {default:store}=await import(path) as typeof import("../../src/stores/PokeBattleStore");const state=store.getState();return {id:state.battleId,revision:state.revision,generation:state.presentationGeneration};});
 release!();await expect.poll(()=>delivered).toBe(1);const after=await page.evaluate(async()=>{const path="/src/stores/PokeBattleStore.ts";const {default:store}=await import(path) as typeof import("../../src/stores/PokeBattleStore");const state=store.getState();return {id:state.battleId,revision:state.revision,generation:state.presentationGeneration};});expect(after).toEqual(before);
 await page.screenshot({path:testInfo.outputPath(`fishing-${mode}-recovered.png`)});await record({family:"fishing-command",mode,id,requests,oldId,delivered,battle:after});await quitToCharacterSelect(page);
 });
}
