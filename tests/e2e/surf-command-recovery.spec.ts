import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle} from "./helpers/state";
import {clickTile,pressSpace} from "./helpers/input";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {observeResponseDelivery} from "./helpers/responseDelivery";

for (const mode of ["timeout","reentry","process"] as const) {
  test(`committed SURF ${mode} recovers without another entry`,async({page,context},testInfo)=>{
    test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");
    test.skip(mode==="process" && process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires exact-process runner");
    test.setTimeout(90000);
    const {sql,record,crash}=await isolatedCrashRuntime();
    const recoveryIds=new Set<string>();
    let requests=0, recoveryReads=0, recoveryReplies=0, oldId="", delivered=0;
    let release:(()=>void)|undefined;
    let historical:unknown, receipt:unknown;
    await context.routeWebSocket("**/ws",socket=>{
      const server=socket.connectToServer();
      socket.onMessage(message=>{
        if(Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.PokeSurfingRequest){
          const request=JSON.parse(message.subarray(6).toString());
          expect(request.requestId).toBeTruthy(); expect(request.characterId).toBeGreaterThan(0);
          expect(request.fromX).toBe(3); expect(request.fromY).toBe(17); requests++;
        }
        if (release && Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.GameplayStateRequest) {
          recoveryReads++;
          recoveryIds.add(JSON.parse(message.subarray(6).toString()).requestId);
        }
        server.send(message);
      });
      server.onMessage(message=>{
        if(Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.PokeSurfingResponse){
          const reply=JSON.parse(message.subarray(6).toString());
          if(reply.success && !release){oldId=reply.requestId; historical=reply; release=()=>socket.send(message); return;}
        }
        if (Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.GameplayStateResponse) {
          const reply=JSON.parse(message.subarray(6).toString());
          if (reply.success && recoveryIds.has(reply.requestId)) recoveryReplies++;
        }
        socket.send(message);
      });
    });
    const character=await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page,"debug_field_move_surf_ready");
    await waitForNoMapLoading(page); await waitForPlayerIdle(page);
    const id=(await getGameState(page)).player.internalId!;
    await observeResponseDelivery(page,[OpCodes.PokeSurfingResponse],(_opcode,requestId)=>{if(requestId===oldId)delivered++;});
    await clickTile(page,4,17);
    await expect.poll(async()=>(await getGameState(page)).dialogue.isChoicePending).toBe(true);
    await pressSpace(page);
    await expect.poll(()=>Boolean(release)).toBe(true);
    expect(await sql(`SELECT x::int || ',' || y::int FROM character_data WHERE id=${id}`)).toBe("4,17");
    if(mode==="reentry"){
      await quitToCharacterSelect(page); await enterWorld(page,character); await waitForNoMapLoading(page);
    }
    if(mode==="process"){
      receipt=await crash();
      await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible({timeout:20000});
      await page.getByRole("button",{name:"PLAY AS GUEST"}).click();
      await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
      await enterWorld(page,character); await waitForNoMapLoading(page);
    }
    // Movement broadcasts can display the committed tile before the held ack
    // times out. Require the actual owned recovery request while it is held.
    if (mode === "timeout") {
      await expect.poll(()=>recoveryReads,{timeout:20000}).toBeGreaterThan(0);
      await expect.poll(()=>recoveryReplies).toBeGreaterThan(0);
    }
    const pose=async()=>{const state=await getGameState(page);return{x:state.player.x,y:state.player.y,surfing:state.player.isSurfing};};
    await expect.poll(pose,{timeout:20000}).toEqual({x:4,y:17,surfing:true});
    expect(requests).toBe(1);
    if(mode==="process"){
      await page.evaluate(async reply=>{const path="/src/net/index.ts";const {WorldSocket}=await import(path);WorldSocket.onJson?.(105,reply);},historical);
    }else release!();
    await expect.poll(()=>delivered).toBe(1);
    expect(await pose()).toEqual({x:4,y:17,surfing:true}); expect(requests).toBe(1);
    await page.screenshot({path:testInfo.outputPath(`surf-${mode}-recovered.png`)});
    await record({family:"surf-command",mode,id,requests,recoveryReads,recoveryReplies,oldId,delivered,receipt});
    await quitToCharacterSelect(page);
  });
}
