import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle} from "./helpers/state";
import {clickTile,pressSpace,pressMovement,dismissDialogue} from "./helpers/input";
import {endWildBattleIfOpen} from "./helpers/battle";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {observeResponseDelivery} from "./helpers/responseDelivery";

for(const outcome of ["encounter","blackout"] as const) for(const order of ["ack-first","hint-first"] as const){
 test(`SURF ${outcome} ${order} retains owned recovery and releases movement`,async({page,context},testInfo)=>{
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires private exact-process runner to reload fixture catalog");test.setTimeout(120000);
  const {sql,crash,record}=await isolatedCrashRuntime();
  let armed=false,firstReleased=false,commands=0,hints=0;
  let ack:(()=>void)|undefined,hint:(()=>void)|undefined,read:(()=>void)|undefined;
  const hintOpcode=outcome==="encounter"?OpCodes.PokeBattleStartResponse:OpCodes.PokeBattleEndNotify;
  await context.routeWebSocket("**/ws",socket=>{
   const server=socket.connectToServer();
   socket.onMessage(message=>{if(armed && Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.PokeSurfingRequest)commands++;server.send(message);});
   server.onMessage(message=>{
    if(armed && Buffer.isBuffer(message) && message.length>=6){
     const opcode=message.readUInt16LE(4);
     if(opcode===OpCodes.PokeSurfingResponse && !ack){ack=()=>socket.send(message);return;}
     if(opcode===hintOpcode && !hint){hint=()=>socket.send(message);return;}
     if(firstReleased && opcode===OpCodes.GameplayStateResponse && !read){read=()=>socket.send(message);return;}
    }
    socket.send(message);
   });
  });
  const character=await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"debug_field_move_surf_ready");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
  const id=(await getGameState(page)).player.internalId!;
  const area=await sql("SELECT COALESCE(max(id),0)+1 FROM phaser_encounter_areas");expect(area).toMatch(/^\d+$/);
  // Private deterministic fixture: all 256 possible 8-bit rolls select the
  // encounter. Production rates and source assets are unchanged.
  await sql(`INSERT INTO phaser_encounter_areas(id,name,encounter_rate,encounter_type) VALUES(${area},'SURF_ORDER_FIXTURE',256,'water'); INSERT INTO phaser_encounter_area_slots(encounter_area_id,slot_index,pokemon_id,level,probability) VALUES(${area},0,129,5,100); UPDATE phaser_tiles SET encounter_area_id=${area} WHERE map_id IS NULL AND x=4 AND y=17`);
  if(outcome==="blackout")await sql(`UPDATE character_pokemon SET cur_hp=0 WHERE character_id=${id}; UPDATE character_data SET options='{"lastPokeCenterMapId":9999,"lastPokeCenterX":3,"lastPokeCenterY":17}' WHERE id=${id}`);
  // Catalog caches are loaded at startup. Reload through the exact owned process.
  const restart=await crash();await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible({timeout:20000});await page.getByRole("button",{name:"PLAY AS GUEST"}).click();await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();await enterWorld(page,character);await waitForNoMapLoading(page);await waitForPlayerIdle(page);
  await observeResponseDelivery(page,[hintOpcode],()=>{hints++;});armed=true;
  await clickTile(page,4,17);await expect.poll(async()=>(await getGameState(page)).dialogue.isChoicePending).toBe(true);await pressSpace(page);
  await expect.poll(()=>Boolean(ack && hint)).toBe(true);
  firstReleased=true;if(order==="ack-first")ack!();else hint!();
  await expect.poll(()=>Boolean(read)).toBe(true);
  if(order==="ack-first")hint!();else ack!();
  await expect.poll(()=>hints).toBe(1);read!();
  const pose=async()=>{const state=await getGameState(page);return{x:state.player.x,y:state.player.y,surfing:state.player.isSurfing};};
  await expect.poll(pose).toEqual(outcome==="encounter"?{x:4,y:17,surfing:true}:{x:3,y:17,surfing:false});
  if(outcome==="encounter"){await expect(page.getByTestId("battle-overlay")).toBeVisible();await endWildBattleIfOpen(page);}
  else await expect(page.getByTestId("battle-overlay")).toHaveCount(0);
  await dismissDialogue(page);await waitForPlayerIdle(page);await pressMovement(page,"left");
  await expect.poll(pose).toEqual(outcome==="encounter"?{x:3,y:17,surfing:false}:{x:2,y:17,surfing:false});
  expect(commands).toBe(1);expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='surf'`)).toBe("1");
  await page.screenshot({path:testInfo.outputPath(`surf-${outcome}-${order}.png`)});await record({family:"surf-publication-order",outcome,order,id,commands,hints,restart});await quitToCharacterSelect(page);
 });
}
