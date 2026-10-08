import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,createCharacter,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {observeResponseDelivery} from "./helpers/responseDelivery";

for(const family of ["safari","blackout"] as const){
 test(`late ${family} publication reads only the replacement character`,async({page,context},testInfo)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {record}=await isolatedCrashRuntime();const opcode=family==="safari" ? OpCodes.SafariBattleStartNotify : OpCodes.PokeBattleEndNotify;
 let release:(()=>void)|undefined;let delivered=0;let reads=0;let replies=0;let historical:Record<string,unknown>|undefined;
 await context.routeWebSocket("**/ws",socket=>{
 const server=socket.connectToServer();socket.onMessage(message=>{if(Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.GameplayStateRequest)reads++;server.send(message);});
 server.onMessage(message=>{if(!release && Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===opcode){historical=JSON.parse(message.subarray(6).toString());release=()=>socket.send(message);return;}socket.send(message);});
 });
 await createGuestCharacterAndEnterWorld(page);
 await observeResponseDelivery(page,[opcode,OpCodes.GameplayStateResponse],received=>{if(received===opcode)delivered++;else replies++;});
 await jumpToScenario(page,family==="safari" ? "safari_battle_run" : "debug_blackout_empty_party");await waitForNoMapLoading(page);
 if(family==="blackout")await page.evaluate(async()=>{const path="/src/net/NetworkBridge.ts";const {NetworkBridge}=await import(path);const opPath="/src/net/generated/opcodes.ts";const ops=await import(opPath);await NetworkBridge.send({mapId:51},ops.PokeBattleStartRequest);});
 await expect.poll(()=>Boolean(release)).toBe(true);await quitToCharacterSelect(page);
 const other=`Q${Math.random().toString(36).slice(2,10).replace(/[0-9]/g,"a")}`;await createCharacter(page,other);await enterWorld(page,other);await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const before=(await getGameState(page)).player;const beforeReads=reads,beforeReplies=replies;
 release!();await expect.poll(()=>delivered).toBe(1);await expect.poll(()=>reads).toBeGreaterThan(beforeReads);await expect.poll(()=>replies).toBeGreaterThan(beforeReplies);
 await expect(page.getByTestId("battle-overlay")).toHaveCount(0);
 await expect.poll(async()=>{const state=await getGameState(page);return {map:state.map.id,x:state.player.x,y:state.player.y};}).toEqual({map:38,x:before.x,y:before.y});
 await page.screenshot({path:testInfo.outputPath(`foreign-${family}-publication.png`)});await record({family:`${family}-publication`,historical,delivered,reads,replies,owner:before.internalId,position:{map:38,x:before.x,y:before.y}});await quitToCharacterSelect(page);
 });
}
