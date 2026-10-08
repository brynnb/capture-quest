import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {clickTile,pressMovement,pressSpace,dismissDialogue} from "./helpers/input";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {waitForNoMapLoading,waitForPlayerTile,waitForPlayerIdle,getGameState} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {observeResponseDelivery} from "./helpers/responseDelivery";

for(const family of ["ordinary","trainer"] as const){
for(const mode of ["timeout","reentry"] as const){
 test(`${family} actor dialogue rejects a held reply after ${mode}`,async({page,context},testInfo)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires the private database runner");
 test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();
 const requestOpcode=family==="ordinary" ? OpCodes.PhaserDialogueRequest : OpCodes.TrainerInteractRequest;
 const responseOpcode=family==="ordinary" ? OpCodes.PhaserDialogueResponse : OpCodes.TrainerInteractResponse;
 const textConstant=family==="ordinary" ? "TEXT_GAMECORNER_BEAUTY1" : "TEXT_PEWTERGYM_COOLTRAINER_M";
 const target=family==="ordinary" ? {x:2,y:6} : {x:3,y:6};
 let actorId=0;
 const requests:string[]=[];let oldId="";let release:(()=>void)|undefined;let delivered=0;let failures=0;
 page.on("console",message=>{if(message.text().includes("Interaction could not complete"))failures++;});
 await context.routeWebSocket("**/ws",socket=>{
 const server=socket.connectToServer();
 socket.onMessage(message=>{
 if(Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===requestOpcode){
 const req=JSON.parse(message.subarray(6).toString());if(family==="ordinary" ? req.textConstant===textConstant : req.actorId===actorId)requests.push(req.requestId);
 }
 server.send(message);
 });
 server.onMessage(message=>{
 if(!release && Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===responseOpcode){
 const reply=JSON.parse(message.subarray(6).toString());
 if(reply.success && (family==="ordinary" ? reply.textConstant===textConstant : reply.trainerActorId===actorId)){expect(reply.requestId).toBeTruthy();oldId=reply.requestId;release=()=>socket.send(message);return;}
 }
 socket.send(message);
 });
 });
 const character=await createGuestCharacterAndEnterWorld(page);
 await jumpToScenario(page,family==="ordinary" ? "debug_game_corner_buy_coins_ready" : "pewter_gym_brock_generated_post_tm_advice");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 actorId=(await getGameState(page)).visibleActors.find(actor=>actor.text===textConstant)!.id;
 expect(actorId).toBeGreaterThan(0);
 const labels=family==="ordinary" ? `SELECT dialogue_label FROM phaser_text_pointers WHERE text_constant='${textConstant}'` : `SELECT battle_text_label FROM phaser_trainer_headers WHERE map_id=54 UNION SELECT after_battle_text_label FROM phaser_trainer_headers WHERE map_id=54`;
 const update=async(text:string)=>expect(await sql(`UPDATE phaser_dialogue_text SET dialogue='${text}' WHERE label IN(${labels}) RETURNING label`)).not.toBe("");
 await update("Historical dialogue.");
 await observeResponseDelivery(page,[responseOpcode],(_opcode,id)=>{if(id===oldId)delivered++;});
 if(family==="ordinary"){for(const x of [4,3,2]){await pressMovement(page,"left");await waitForPlayerTile(page,x,8);await waitForPlayerIdle(page);}await pressMovement(page,"up");await pressSpace(page);}else{await clickTile(page,3,5);await waitForPlayerTile(page,3,5);await waitForPlayerIdle(page);await pressMovement(page,"down");await pressSpace(page);}
 await expect.poll(()=>Boolean(release),{timeout:20000}).toBe(true);
 await expect(page.getByTestId("pokemon-dialogue-box")).toHaveCount(0);
 if(mode==="timeout")await expect.poll(()=>failures,{timeout:10000}).toBe(1);
 else {await quitToCharacterSelect(page);await enterWorld(page,character);await waitForNoMapLoading(page);await waitForPlayerIdle(page);}
 const beforeRetry=(await getGameState(page)).player;
 if(family==="ordinary")await waitForPlayerTile(page,2,8);
 await update("Current dialogue.");
 if(family==="ordinary")await clickTile(page,target.x,target.y);else{await pressMovement(page,"down");await pressSpace(page);}
 const box=page.getByTestId("pokemon-dialogue-box");await expect(box).toBeVisible();await expect(box).toContainText("Current dialogue.",{timeout:10000});
 expect(requests).toHaveLength(2);expect(new Set(requests).size).toBe(2);
 release!();await expect.poll(()=>delivered).toBe(1);await expect(box).toContainText("Current dialogue.");
 await page.screenshot({path:testInfo.outputPath(`${family}-dialogue-after-${mode}.png`)});
 await record({family:`${family}-dialogue-read`,mode,beforeRetry,requests,historicalRequestId:oldId,delivered,currentText:"Current dialogue."});
 await dismissDialogue(page);await quitToCharacterSelect(page);
 });
}

}
