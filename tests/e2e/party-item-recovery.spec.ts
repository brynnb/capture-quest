import { expect, test } from "@playwright/test";
import type { CQPartyItemUseResponse } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForInventoryItem, waitForInventoryOpen, waitForNoMapLoading } from "./helpers/state";

for (const loseReply of [false, true]) {
 test(`Potion duplicate and lost reply=${loseReply} recover once, allow another use and survive reentry`, async ({page}, testInfo) => {
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires a disposable isolated database and owned crash runner");
  test.setTimeout(120000);
  const {sql,crash,record}=await isolatedCrashRuntime();
  let errors=collectPageErrors(page);
  const faults=await inventoryCommandFaults<CQPartyItemUseResponse>(page,OpCodes.CQItemUseRequest,OpCodes.CQItemUseResponse,loseReply);
  const character=await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page,"debug_inventory_potion_ready");
  await waitForInventoryItem(page,"POTION");
  const characterId=(await getGameState(page)).player.internalId;
  expect(Number.isSafeInteger(characterId) && characterId!>0).toBe(true);
  // The browser debugger seeds healthy Pokemon (its fixture type has no curHp).
  // Injure only the verified private test DB while the character is offline.
  await quitToCharacterSelect(page);
  await sql(`UPDATE character_pokemon SET cur_hp=1 WHERE character_id=${characterId} AND party_slot=0 AND box_slot=0`);
  await enterWorld(page,character);await waitForNoMapLoading(page);
  const before=await getGameState(page);
  const pokemon=before.pokemon.party[0];
  expect(pokemon.curHp).toBe(1);expect(pokemon.maxHp).toBeGreaterThan(41);
  expect(pokemon.rowId).toBeGreaterThan(0);
  await page.getByRole("button",{name:"Bag",exact:true}).click();await waitForInventoryOpen(page,true);
  const usePotion=async()=>{
   await page.getByTestId("inventory-item-potion").click();
   await page.locator("[data-cq-party-item-target='true']").first().click();
  };
  for (let uses=1; uses<=2; uses++) {
   await usePotion();
   await expect.poll(()=>faults.replies.length).toBe(uses);
   const reply=faults.replies[uses-1];
   expect(reply.inventory.commandRevision).toBe(uses);
   expect(reply.party[0]).toMatchObject({rowId:pokemon.rowId,curHp:1+20*uses});
   await expect.poll(async()=>(await getGameState(page)).pokemon.party[0].curHp,{timeout:20000}).toBe(1+20*uses);
   await expect.poll(async()=>(await getGameState(page)).inventory.items.find(item=>item.shortName==="POTION")?.quantity??0).toBe(2-uses);
   expect((await getGameState(page)).inventory.money).toBe(before.inventory.money);
   await expect(page.locator("[data-cq-party-entry='true']").first()).toContainText(`${1+20*uses}/${pokemon.maxHp}`);
  }
  // After a distinct second use, the first historical result must stay inert.
  await page.evaluate(async()=>{
   const path="/src/phaser-game/services/PhaserNetworkService.ts";const net=await import(path);
   const target=window as typeof window & {latePartyReply?:boolean};
   const stop=net.onInventoryCommand(101,()=>{target.latePartyReply=true;stop()});
  });
  faults.deliverReply(0);
  await expect.poll(()=>page.evaluate(()=>(window as typeof window & {latePartyReply?:boolean}).latePartyReply)).toBe(true);
  expect((await getGameState(page)).pokemon.party[0].curHp).toBe(41);
  expect((await getGameState(page)).inventory.items).toHaveLength(0);
  await page.screenshot({path:testInfo.outputPath("party-item-recovered.png")});
  await quitToCharacterSelect(page);
  if(loseReply) {
   errors.assertNoSevereErrors();
   const receipt=await crash();
   await record({family:"party-item",characterId,receipt,uses:faults.successes});
   await page.reload();await page.getByRole("button",{name:"PLAY AS GUEST"}).click();
   await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
   errors=collectPageErrors(page);
  }
  await enterWorld(page,character);await waitForNoMapLoading(page);
  expect((await getGameState(page)).pokemon.party[0]).toMatchObject({rowId:pokemon.rowId,curHp:41});
  expect((await getGameState(page)).inventory.items).toHaveLength(0);
  expect(faults.requests).toBe(2);expect(faults.successes).toBe(2);expect(faults.rejections).toBe(2);
  await quitToCharacterSelect(page);errors.assertNoSevereErrors();
 });
}
