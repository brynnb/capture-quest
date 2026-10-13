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
