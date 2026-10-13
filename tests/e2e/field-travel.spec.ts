import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerTile,waitForMap} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {dismissDialogue} from "./helpers/input";

for(const move of ["DIG","TELEPORT"] as const)test(`party ${move} obeys owned destination policy`,async({page},info)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const {sql,record}=await isolatedCrashRuntime();await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,move==="DIG"?"field_move_dig_no_badge_required":"field_move_teleport_no_badge_required");await waitForNoMapLoading(page);
 const id=(await getGameState(page)).player.internalId!;
 const destination=await sql("SELECT id FROM phaser_maps WHERE name='REDS_HOUSE_2F'");expect(destination).toMatch(/^\d+$/);
 if(move==="TELEPORT")await sql(`INSERT INTO character_bind(id,slot,map_id,x,y,z,heading) VALUES(${id},0,${destination},3,6,0,0) ON CONFLICT(id,slot) DO UPDATE SET map_id=EXCLUDED.map_id,x=EXCLUDED.x,y=EXCLUDED.y`);
 await page.locator("#party-pokemon-hud [data-cq-party-entry='true']").first().click();await page.getByText(move.toLowerCase(),{exact:true}).click();
 if(move==="TELEPORT"){await waitForMap(page,"REDS_HOUSE_2F");await waitForPlayerTile(page,3,6);}else{await expect.poll(async()=>(await getGameState(page)).dialogue.isOpen).toBe(true);expect((await getGameState(page)).player.x).toBe(10);expect((await getGameState(page)).player.y).toBe(10);await dismissDialogue(page);}
 const saved=await sql(`SELECT map_id || ',' || x::int || ',' || y::int FROM character_data WHERE id=${id}`);expect(saved).toBe(move==="TELEPORT"?`${destination},3,6`:"9999,10,10");
 await page.screenshot({path:info.outputPath(`field-travel-${move}.png`)});await record({family:"ability-travel-prerequisite",move,id,saved});await quitToCharacterSelect(page);
});
