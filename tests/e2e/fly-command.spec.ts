import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";

test("party FLY selection uses the served catalog destination",async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"field_move_fly_with_thunder_badge");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 const choice=JSON.parse(await sql(`SELECT json_build_object('name',c.name,'mapId',c.map_id,'x',c.spawn_x,'y',c.spawn_y,'overworld',m.is_overworld) FROM poke_start_cities c JOIN phaser_maps m ON m.id=c.map_id ORDER BY c.id LIMIT 1`));
 await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText("fly",{exact:true}).click();await page.getByText(choice.name,{exact:true}).click();
 const map=choice.overworld?9999:choice.mapId;
 await expect.poll(async()=>{const state=await getGameState(page);return{x:state.player.x,y:state.player.y};}).toEqual({x:choice.x,y:choice.y});await waitForNoMapLoading(page);
 expect(await sql(`SELECT map_id || ',' || x::int || ',' || y::int FROM character_data WHERE id=${id}`)).toBe(`${map},${choice.x},${choice.y}`);
 await page.screenshot({path:info.outputPath("fly-catalog-owned.png")});await record({family:"fly-source-prerequisite",id,choice,map});await quitToCharacterSelect(page);
});

for(const loseReply of [false,true])test(`FLY duplicate lost reply=${loseReply} recovers current catalog travel`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const faultsModule=await import("./helpers/inventoryCommandFaults");const ops=await import("../../src/net/generated/opcodes");const {sql,record}=await isolatedCrashRuntime();
 const faults=await faultsModule.inventoryCommandFaults(page,ops.FieldMoveUseRequest,ops.FieldMoveUseResponse,loseReply,request=>request.moveName==="FLY");
 await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"field_move_fly_with_thunder_badge");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 const choice=JSON.parse(await sql(`SELECT json_build_object('name',c.name,'mapId',c.map_id,'x',c.spawn_x,'y',c.spawn_y,'overworld',m.is_overworld) FROM poke_start_cities c JOIN phaser_maps m ON m.id=c.map_id ORDER BY c.id LIMIT 1`));
 await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText("fly",{exact:true}).click();await page.getByText(choice.name,{exact:true}).click();
 await expect.poll(()=>faults.successes).toBe(2);expect(faults.replies[1]).toEqual(faults.replies[0]);expect(faults.requests).toBe(1);
 await expect.poll(async()=>{const value=await getGameState(page);return{x:value.player.x,y:value.player.y};},{timeout:20000}).toEqual({x:choice.x,y:choice.y});await waitForNoMapLoading(page);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("1");await page.screenshot({path:info.outputPath("fly-correlated.png")});await record({family:"fly-command",loseReply,id,requests:faults.requests,successes:faults.successes,choice});await quitToCharacterSelect(page);
});

test("FLY selector retires on source scene replacement without sending travel",async({page})=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);const {sql,record}=await isolatedCrashRuntime();
 await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"field_move_fly_with_thunder_badge");await waitForNoMapLoading(page);const id=(await getGameState(page)).player.internalId!;
 const choice=await sql("SELECT name FROM poke_start_cities ORDER BY id LIMIT 1");await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText("fly",{exact:true}).click();await expect(page.getByText(choice,{exact:true})).toBeVisible();
 await page.evaluate(async()=>{
  const path="/src/net/index.ts";
  const {WorldSocket,OpCodes}=await import(path) as typeof import("../../src/net/index");
  WorldSocket.sendJsonMessage(OpCodes.DebugSceneJumpRequest,{scenarioName:"debug_escape_rope_ready"});
 });await expect.poll(async()=>{const state=await getGameState(page);return state.map.id;}).not.toBe(9999);await waitForNoMapLoading(page);await expect(page.getByText(choice,{exact:true})).toBeHidden();expect(await sql(`SELECT count(*) FROM character_field_command_state WHERE character_id=${id} AND domain='travel'`)).toBe("0");await record({family:"fly-selector-retirement",id,choice});await quitToCharacterSelect(page);
});
