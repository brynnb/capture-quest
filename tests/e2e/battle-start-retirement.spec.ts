import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,createCharacter,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {clickTile,pressMovement,pressSpace,dismissDialogue} from "./helpers/input";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle,waitForPlayerTile} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {observeResponseDelivery} from "./helpers/responseDelivery";

for(const mode of ["selector","other-character"] as const){
 test(`held committed battle start cannot revive mode=${mode}`,async({page,context},testInfo)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();let release:(()=>void)|undefined;let battleId="";let delivered=0;let readRequests=0;let readReplies=0;let starts=0;
 await context.routeWebSocket("**/ws",socket=>{
 const server=socket.connectToServer();socket.onMessage(message=>{
 if(Buffer.isBuffer(message) && message.length>=6){const opcode=message.readUInt16LE(4);if(opcode===OpCodes.GameplayStateRequest)readRequests++;if(opcode===OpCodes.TrainerBattleStartRequest)starts++;}
 server.send(message);
 });server.onMessage(message=>{
 if(!release && Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.PokeBattleStartResponse){const reply=JSON.parse(message.subarray(6).toString());if(reply.success && reply.battleId){battleId=reply.battleId;release=()=>socket.send(message);return;}}
 socket.send(message);
 });
 });
 const character=await createGuestCharacterAndEnterWorld(page);
 await jumpToScenario(page,"pewter_gym_brock_generated_post_tm_advice");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const id=(await getGameState(page)).player.internalId!;
 await clickTile(page,3,5);await waitForPlayerTile(page,3,5);await waitForPlayerIdle(page);
 await sql(`DELETE FROM character_event_flags WHERE character_id=${id} AND flag_name='EVENT_BEAT_BROCK'; UPDATE phaser_dialogue_text SET dialogue='Start owned battle.' WHERE label IN(SELECT battle_text_label FROM phaser_trainer_headers WHERE map_id=54)`);
 await observeResponseDelivery(page,[OpCodes.PokeBattleStartResponse,OpCodes.GameplayStateResponse],(opcode,identity)=>{if(opcode===OpCodes.PokeBattleStartResponse && identity===battleId)delivered++;if(opcode===OpCodes.GameplayStateResponse)readReplies++;});
 await pressMovement(page,"down");await pressSpace(page);await expect(page.getByTestId("pokemon-dialogue-box")).toContainText("Start owned battle.");await dismissDialogue(page);
 await expect.poll(()=>Boolean(release)).toBe(true);expect(starts).toBe(1);expect(await sql(`SELECT count(*) FROM character_battle_state WHERE character_id=${id}`)).toBe("1");
 await quitToCharacterSelect(page);await expect(page.getByTestId("battle-overlay")).toHaveCount(0);
 if(mode==="selector"){
 const before=readRequests;release!();await expect.poll(()=>delivered).toBe(1);await expect(page.getByTestId("battle-overlay")).toHaveCount(0);expect(readRequests).toBe(before);
 await enterWorld(page,character);await waitForNoMapLoading(page);await expect(page.getByTestId("battle-overlay")).toBeVisible();const restored=await page.evaluate(async()=>{const path="/src/stores/PokeBattleStore.ts";const {default:store}=await import(path) as typeof import("../../src/stores/PokeBattleStore");return store.getState().battleId;});expect(restored).toBe(battleId);
 await page.screenshot({path:testInfo.outputPath("owned-battle-restored.png")});await quitToCharacterSelect(page);
 }else{
 const other=`Q${Math.random().toString(36).slice(2,10).replace(/[0-9]/g,"a")}`;
 await createCharacter(page,other);await enterWorld(page,other);await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const next=(await getGameState(page)).player.internalId!;expect(next).not.toBe(id);
 const beforeRequests=readRequests,beforeReplies=readReplies;release!();await expect.poll(()=>delivered).toBe(1);await expect.poll(()=>readRequests).toBeGreaterThan(beforeRequests);await expect.poll(()=>readReplies).toBeGreaterThan(beforeReplies);
 await expect(page.getByTestId("battle-overlay")).toHaveCount(0);expect(await sql(`SELECT count(*) FROM character_battle_state WHERE character_id=${next}`)).toBe("0");
 await page.screenshot({path:testInfo.outputPath("foreign-hint-no-battle.png")});await quitToCharacterSelect(page);
 }
 await record({family:"battle-start-retirement",mode,id,battleId,delivered,starts,readRequests,readReplies});
 });
}
