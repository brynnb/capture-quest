import { expect, test } from "@playwright/test";
import type { PokemonPCResponse } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { clickTile } from "./helpers/input";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading, waitForPlayerTile } from "./helpers/state";

for (const mode of ["normal","timeout","crash"] as const) {
  const loseReply=mode!=="normal";
  test(`PC deposit duplicate and mode=${mode} recover stable membership at Indigo`,async({page,context},testInfo)=>{
    test.skip(process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires the private database and exact-process crash runner");
    test.setTimeout(120000);
    const {sql,crash,record}=await isolatedCrashRuntime();
    let errors=collectPageErrors(page);
    const faults=await inventoryCommandFaults<PokemonPCResponse>(page,OpCodes.PokemonPCDepositRequest,OpCodes.PokemonPCDepositResponse,loseReply);
    const character=await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page,"debug_pc_indigo_ready");
    await waitForPlayerTile(page,14,8);
    const before=await getGameState(page);
    const charId=before.player.internalId!;
    expect(Number.isSafeInteger(charId) && charId>0).toBe(true);
    const target=before.pokemon.party[0].rowId;
    const remaining=before.pokemon.party[1].rowId;
    expect(target).toBeGreaterThan(0);
    await clickTile(page,15,8); await waitForPlayerTile(page,15,8);
    await expect(page.getByTestId("pokemon-pc-main-menu")).toBeVisible();
    await page.getByTestId("pokemon-pc-bills-pc").click();
    await page.getByTestId("pokemon-pc-party-slot-0").click();
    await page.getByTestId("pokemon-pc-deposit").click();
    await expect.poll(()=>faults.replies.length).toBe(1);
    expect(faults.replies[0].pc.box.map(p=>p.rowId)).toEqual([target]);
    expect(faults.replies[0].party.map(p=>p.rowId)).toEqual([remaining]);
    expect(faults.replies[0].inventory.commandRevision).toBe(1);
    expect(await sql(`SELECT box FROM character_pokemon WHERE id=${target} AND character_id=${charId}`)).toBe("0");
    await expect.poll(()=>faults.rejections).toBe(1);
    let expectedParty=[remaining], expectedBox=[target], expectedRevision=1;
    if(mode==="crash") {
      // Close the pending listener before its timeout recovery, then kill the
      // exact server after commit and before the withheld acknowledgement.
      errors.assertNoSevereErrors(); await page.close();
      const receipt=await crash(); await record({family:"pc-storage",charId,target,remaining,receipt,revision:1});
      page=await context.newPage(); await page.goto("/");
      await page.getByRole("button",{name:"PLAY AS GUEST"}).click();
      await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
      errors=collectPageErrors(page);
      await enterWorld(page,character); await waitForNoMapLoading(page);
    }
    await expect.poll(async()=>(await getGameState(page)).pokemon.party.map(p=>p.rowId),{timeout:20000}).toEqual(expectedParty);
    await expect.poll(async()=>(await getGameState(page)).pokemon.pc.boxPokemon.map(p=>p.rowId)).toEqual(expectedBox);
    expect(faults.requests).toBe(1); expect(faults.successes).toBe(1);
    await expect.poll(()=>faults.rejections).toBe(1);
    if(mode!=="crash") {
      await page.screenshot({path:testInfo.outputPath("indigo-pc-recovered.png")});
      await page.getByTestId("pokemon-pc-box-slot-0").click();
      await page.getByTestId("pokemon-pc-withdraw").click();
      expectedParty=[remaining,target]; expectedBox=[]; expectedRevision=2;
      await expect.poll(async()=>(await getGameState(page)).pokemon.party.map(p=>p.rowId)).toEqual(expectedParty);
      await expect.poll(async()=>(await getGameState(page)).pokemon.pc.boxPokemon.length).toBe(0);
      await page.evaluate(async()=>{
        const path="/src/phaser-game/services/PhaserNetworkService.ts"; const net=await import(path);
        const target=window as typeof window & {latePCReply?:boolean};
        const stop=net.onInventoryCommand(111,()=>{target.latePCReply=true;stop()});
      });
      faults.deliverReply(0);
      await expect.poll(()=>page.evaluate(()=>(window as typeof window & {latePCReply?:boolean}).latePCReply)).toBe(true);
      expect((await getGameState(page)).pokemon.pc.boxPokemon).toHaveLength(0);
      expect((await getGameState(page)).pokemon.party.map(p=>p.rowId)).toEqual(expectedParty);
      await page.getByTestId("pokemon-pc-close").click();
    }
    await quitToCharacterSelect(page); errors.assertNoSevereErrors();
    await enterWorld(page,character); await waitForNoMapLoading(page);
    expect((await getGameState(page)).pokemon.party.map(p=>p.rowId)).toEqual(expectedParty);
    expect((await getGameState(page)).pokemon.pc.boxPokemon.map(p=>p.rowId)).toEqual(expectedBox);
    expect(await sql(`SELECT revision FROM character_shop_state WHERE character_id=${charId}`)).toBe(String(expectedRevision));
    await quitToCharacterSelect(page); errors.assertNoSevereErrors();
  });
}
