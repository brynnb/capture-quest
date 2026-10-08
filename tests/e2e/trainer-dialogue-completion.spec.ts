import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {clickTile,pressMovement,pressSpace,dismissDialogue} from "./helpers/input";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerIdle,waitForPlayerTile} from "./helpers/state";
import {collectPageErrors} from "./helpers/errors";
import {isolatedCrashRuntime} from "./helpers/processRecovery";

for(const mode of ["live","retired-pointer","retired-keyboard"] as const){
 test(`trainer dialogue completion mode=${mode} respects its character owner`,async({page},testInfo)=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database runner");test.setTimeout(90000);
 const errors=collectPageErrors(page);const {sql,record}=await isolatedCrashRuntime();
 const character=await createGuestCharacterAndEnterWorld(page);
 await jumpToScenario(page,"pewter_gym_brock_generated_post_tm_advice");await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const state=await getGameState(page);const id=state.player.internalId!;
 const target=state.visibleActors.find(actor=>actor.text==="TEXT_PEWTERGYM_COOLTRAINER_M")!;expect(target.id).toBeGreaterThan(0);
 await clickTile(page,3,5);await waitForPlayerTile(page,3,5);await waitForPlayerIdle(page);
 // The fixture suppresses sight encounters through Brock's flag. Remove only
 // that flag after spawning; the direct read must consult durable current state.
 await sql(`DELETE FROM character_event_flags WHERE character_id=${id} AND flag_name='EVENT_BEAT_BROCK'; UPDATE phaser_dialogue_text SET dialogue='Ready trainer dialogue.' WHERE label IN(SELECT battle_text_label FROM phaser_trainer_headers WHERE map_id=54)`);
 await pressMovement(page,"down");await pressSpace(page);
 const box=page.getByTestId("pokemon-dialogue-box");await expect(box).toBeVisible();await expect(box).toContainText("Ready trainer dialogue.");
 if(mode==="live"){
 await dismissDialogue(page);await expect(page.getByTestId("battle-overlay")).toBeVisible();
 expect(errors.sentOpcodes.filter(opcode=>opcode===OpCodes.TrainerBattleStartRequest)).toHaveLength(1);
 expect(await sql(`SELECT count(*) FROM character_battle_state WHERE character_id=${id}`)).toBe("1");
 await page.screenshot({path:testInfo.outputPath("live-trainer-battle.png")});await quitToCharacterSelect(page);
 }else{
 await page.evaluate(async()=>{
 const path="/src/stores/PokemonDialogueStore.ts";
 const {default:store}=await import(path) as typeof import("../../src/stores/PokemonDialogueStore");
 const callback=store.getState().onClose;if(!callback)throw new Error("Trainer completion callback missing");
 (window as unknown as {retiredTrainerCallback:()=>void}).retiredTrainerCallback=callback;
 });
 expect(errors.sentOpcodes.filter(opcode=>opcode===OpCodes.TrainerBattleStartRequest)).toHaveLength(0);
 if(mode==="retired-keyboard"){
 await page.getByRole("button",{name:"Quit",exact:true}).focus();await page.keyboard.press("Enter");
 await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
 }else{await quitToCharacterSelect(page);}
 const retirement=await page.evaluate(async()=>{
 const path="/src/stores/PokeBattleStore.ts";
 const {default:store}=await import(path) as typeof import("../../src/stores/PokeBattleStore");
 const state=store.getState();return {isInBattle:state.isInBattle,battleId:state.battleId,phase:state.phase};
 });
 await record({family:"trainer-retirement-diagnostic",id,retirement,sent:errors.sentOpcodes.filter(opcode=>[OpCodes.TrainerBattleStartRequest,OpCodes.TrainerEncounterReady,OpCodes.GameplayStateRequest].includes(opcode)),durable:await sql(`SELECT count(*) FROM character_battle_state WHERE character_id=${id}`)});
 await expect(page.getByTestId("battle-overlay")).toHaveCount(0);
 await enterWorld(page,character);await waitForNoMapLoading(page);await waitForPlayerIdle(page);
 // Replay a real retired callback at the current browser lifecycle boundary;
 // this is application fencing, not a claimed delivery from a dead transport.
 await page.evaluate(()=>(window as unknown as {retiredTrainerCallback:()=>void}).retiredTrainerCallback());
 expect(errors.sentOpcodes.filter(opcode=>opcode===OpCodes.TrainerBattleStartRequest)).toHaveLength(0);
 await expect(page.getByTestId("battle-overlay")).toHaveCount(0);expect(await sql(`SELECT count(*) FROM character_battle_state WHERE character_id=${id}`)).toBe("0");
 await page.screenshot({path:testInfo.outputPath("retired-trainer-no-battle.png")});await quitToCharacterSelect(page);
 }
 await record({family:"trainer-dialogue-completion",mode,id,actorId:target.id,battleStarts:errors.sentOpcodes.filter(opcode=>opcode===OpCodes.TrainerBattleStartRequest).length});errors.assertNoSevereErrors();
 });
}
