import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForMap,waitForPlayerTile,waitForNoMapLoading,waitForMessage} from "./helpers/state";
import {pressMovement} from "./helpers/input";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {observeResponseDelivery} from "./helpers/responseDelivery";
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


for(const mode of ["pending-action","reentry"] as const)test(`Warp Home held reset ${mode} retires old battle and reset replies`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 let actionReplies=0;let deliverAction:(()=>void)|undefined;
 const faults=await inventoryCommandFaults<WarpHomeResponse>(page,OpCodes.WarpHomeRequest,OpCodes.WarpHomeResponse,true,request=>request.current!==true,(opcode,message,deliver)=>{
  if(mode==="pending-action" && opcode===OpCodes.SafariBattleActionResponse && Buffer.isBuffer(message)){
   const reply=JSON.parse(message.subarray(6).toString());if(reply.success){actionReplies++;deliverAction=deliver;return true;}
  }return false;
 });
 const {sql,record}=await isolatedCrashRuntime();const character=await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"safari_battle_run");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 const delivered:number[]=[];await observeResponseDelivery(page,[OpCodes.WarpHomeResponse,OpCodes.SafariBattleActionResponse],opcode=>{delivered.push(opcode)});
 await expect(page.getByTestId("battle-action-safari-ball")).toBeVisible();
 if(mode==="pending-action"){await page.getByTestId("battle-action-run").click();await expect.poll(()=>actionReplies).toBe(1);expect((await getGameState(page)).battle.isOpen).toBe(true);}
 await page.getByRole("button",{name:"Warp Home",exact:true}).click();await expect.poll(()=>faults.successes).toBe(2);
 if(mode==="reentry"){await quitToCharacterSelect(page);await enterWorld(page,character);await waitForNoMapLoading(page);}
 await waitForMap(page,9999);await waitForPlayerTile(page,9,4);await expect.poll(async()=>(await getGameState(page)).battle.isOpen).toBe(false);
 const before=(await getGameState(page)).player;const count=delivered.length;
 if(deliverAction)deliverAction();faults.deliverReply(0);await expect.poll(()=>delivered.length).toBe(count+(deliverAction?2:1));
 expect((await getGameState(page)).player).toEqual(before);expect((await getGameState(page)).battle.isOpen).toBe(false);expect((await getGameState(page)).dialogue.isOpen).toBe(false);
 expect(faults.requests).toBe(1);expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("1");expect(await sql(`SELECT count(*) FROM character_safari_state WHERE character_id=${id}`)).toBe("0");
 await page.screenshot({path:info.outputPath(`warp-home-retired-${mode}.png`)});await record({family:"warp-home-held",mode,id,actionReplies,requests:faults.requests,delivered,position:before});await quitToCharacterSelect(page);
});


test("Warp Home late commit rejection retains Safari battle and source",async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const replies:Array<{success:boolean;current?:boolean;error?:string}>=[];
 page.on("websocket",socket=>socket.on("framereceived",frame=>{const packet=frame.payload;if(Buffer.isBuffer(packet)&&packet.length>=6&&packet.readUInt16LE(4)===OpCodes.WarpHomeResponse)replies.push(JSON.parse(packet.subarray(6).toString()));}));
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"safari_battle_run");await waitForNoMapLoading(page);await expect(page.getByTestId("battle-action-safari-ball")).toBeVisible();const id=(await getGameState(page)).player.internalId!;
 const original=await sql(`SELECT state_json FROM character_safari_state WHERE character_id=${id}`);
 await sql(`CREATE FUNCTION reject_home_browser() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late home rejection';END $$;CREATE CONSTRAINT TRIGGER reject_home_browser AFTER DELETE ON character_safari_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW WHEN(OLD.character_id=${id}) EXECUTE FUNCTION reject_home_browser()`);
 await page.getByRole("button",{name:"Warp Home",exact:true}).click();await expect.poll(()=>replies.some(reply=>reply.success===false)).toBe(true);
 await expect.poll(async()=>(await getGameState(page)).dialogue.isOpen).toBe(true);await expect(page.getByTestId("battle-action-safari-ball")).toBeVisible();
 await waitForMap(page,"SAFARI_ZONE_CENTER");await waitForPlayerTile(page,14,24);expect(await sql(`SELECT state_json FROM character_safari_state WHERE character_id=${id}`)).toBe(original);
 expect(await sql(`SELECT count(*) FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("0");
 await page.screenshot({path:info.outputPath("warp-home-rejected-retained.png")});await record({family:"warp-home-rejection",id,replies,position:(await getGameState(page)).player});await sql("DROP TRIGGER reject_home_browser ON character_safari_state");await quitToCharacterSelect(page);
});

test("Warp Home failed owned read locks ordinary movement but permits explicit emergency retry",async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 let dropReads=false;let armFailure=true;let drops=0;let steps=0;
 const faults=await inventoryCommandFaults<WarpHomeResponse>(page,OpCodes.WarpHomeRequest,OpCodes.WarpHomeResponse,false,request=>request.current!==true,(opcode,message)=>{
  if(opcode===OpCodes.WarpHomeResponse&&Buffer.isBuffer(message)){const reply=JSON.parse(message.subarray(6).toString());if(reply.success&&!reply.current&&armFailure)dropReads=true;}
  if(opcode===OpCodes.GameplayStateResponse&&dropReads){drops++;return true;}return false;
 },opcode=>{if(opcode===OpCodes.PlayerStepRequest)steps++;});
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await waitForMap(page,"REDS_HOUSE_2F");await waitForPlayerTile(page,3,6);const id=(await getGameState(page)).player.internalId!;
 await page.getByRole("button",{name:"Warp Home",exact:true}).click();await expect.poll(()=>faults.successes).toBe(2);await waitForMessage(page,"Current state could not be recovered.",20000);
 expect(drops).toBeGreaterThan(0);const old=(await getGameState(page)).player;const beforeSteps=steps;await pressMovement(page,"right");expect(steps).toBe(beforeSteps);expect((await getGameState(page)).player).toEqual(old);
 expect(faults.requests).toBe(1);expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("1");
 await page.screenshot({path:info.outputPath("warp-home-read-failed-locked.png")});
 // The next click is a new explicit recovery action, never an automatic resend.
 armFailure=false;dropReads=false;
 await page.getByRole("button",{name:"Warp Home",exact:true}).click();
 await expect.poll(()=>faults.requests).toBe(2);dropReads=false;await waitForMap(page,9999);await waitForPlayerTile(page,9,4);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("2");
 await page.screenshot({path:info.outputPath("warp-home-explicit-retry.png")});await record({family:"warp-home-read-failure",id,drops,steps,requests:faults.requests,revision:2});await quitToCharacterSelect(page);
});
