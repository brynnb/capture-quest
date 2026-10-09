import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {pressMovement,pressSpace} from "./helpers/input";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle,waitForMap,waitForPlayerTile} from "./helpers/state";
import {isolatedCrashRuntime} from "./helpers/processRecovery";

for(const mode of ["reentry","process"] as const){
 test(`unfinished Safari expiry narrative recovers after ${mode}`,async({page},testInfo)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");
 test.skip(mode==="process" && process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires exact-process crash runner");test.setTimeout(120000);
 const {sql,crash,record}=await isolatedCrashRuntime();
 const character=await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"safari_step_expiry");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const id=(await getGameState(page)).player.internalId!;await pressMovement(page,"up");
 const box=page.getByTestId("pokemon-dialogue-box");await expect(box).toContainText("Your SAFARI GAME is over!");
 const visit=await sql(`SELECT state_json::json->'visit'->>'visitId' FROM character_safari_state WHERE character_id=${id}`);expect(visit).toBeTruthy();
 await page.evaluate(async()=>{const path="/src/stores/PokemonDialogueStore.ts";const {default:store}=await import(path) as typeof import("../../src/stores/PokemonDialogueStore");const callback=store.getState().onClose;if(!callback)throw new Error("Missing owned expiry completion");(window as unknown as {oldSafariExit:()=>void}).oldSafariExit=callback;});
 let receipt:unknown;
 if(mode==="process"){
 receipt=await crash();await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible({timeout:20000});await page.getByRole("button",{name:"PLAY AS GUEST"}).click();await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
 }else await quitToCharacterSelect(page);
 await enterWorld(page,character);await waitForNoMapLoading(page);await expect(box).toContainText("Your SAFARI GAME is over!");
 await page.evaluate(()=>(window as unknown as {oldSafariExit:()=>void}).oldSafariExit());await expect(box).toBeVisible();
 expect(await sql(`SELECT state_json::json->'visit'->>'visitId' FROM character_safari_state WHERE character_id=${id}`)).toBe(visit);
 await page.screenshot({path:testInfo.outputPath(`recovered-safari-exit-${mode}.png`)});
 await pressSpace(page,4);await waitForMap(page,"SAFARI_ZONE_GATE");await waitForPlayerTile(page,3,4);
 await record({family:"safari-exit-recovery",mode,id,visit,receipt});await quitToCharacterSelect(page);
 });
}
