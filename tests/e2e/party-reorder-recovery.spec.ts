import { expect, test, type Page } from "@playwright/test";
import type { PokemonPartyReorderResponse } from "../../src/net/generated/world_api";
import * as OpCodes from "../../src/net/generated/opcodes";
import { createGuestCharacterAndEnterWorld, enterWorld, quitToCharacterSelect } from "./helpers/auth";
import { collectPageErrors } from "./helpers/errors";
import { inventoryCommandFaults } from "./helpers/inventoryCommandFaults";
import { isolatedCrashRuntime } from "./helpers/processRecovery";
import { jumpToScenario } from "./helpers/scenarioDebugger";
import { getGameState, waitForNoMapLoading } from "./helpers/state";

async function dragFirstToSecond(page: Page, beforeRelease?: () => Promise<void>) {
  const rows=page.locator("#party-pokemon-hud [data-cq-party-entry='true']");
  const from=await rows.first().getByTestId("party-reorder-handle").boundingBox();
  const to=await rows.nth(1).boundingBox();
  expect(from).not.toBeNull(); expect(to).not.toBeNull();
  await page.mouse.move(from!.x+from!.width/2,from!.y+from!.height/2);
  await page.mouse.down();
  await page.mouse.move(to!.x+to!.width/2,to!.y+to!.height/2,{steps:5});
  await beforeRelease?.();
  await page.mouse.up();
}

for (const loseReply of [false,true]) {
 test(`Party reorder duplicate and lost reply=${loseReply} preserve IDs through drag and reentry`,async({page},testInfo)=>{
  test.skip(process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires the disposable database and owned crash runner");
  test.setTimeout(120000);
  const {sql,crash,record}=await isolatedCrashRuntime();
  let errors=collectPageErrors(page);
  const faults=await inventoryCommandFaults<PokemonPartyReorderResponse>(page,OpCodes.PokemonPartyReorderRequest,OpCodes.PokemonPartyReorderResponse,loseReply);
  const character=await createGuestCharacterAndEnterWorld(page);
  await jumpToScenario(page,"debug_pokemon_center_pc_ready");
  const before=await getGameState(page);
  const characterId=before.player.internalId!;
  expect(Number.isSafeInteger(characterId) && characterId>0).toBe(true);
  const original=before.pokemon.party.map(p=>p.rowId);
  expect(original).toHaveLength(2);
  const reversed=[...original].reverse();
  const order=async()=>(await getGameState(page)).pokemon.party.map(p=>p.rowId);
  const renderedOrder=()=>page.locator("#party-pokemon-hud [data-cq-party-entry='true']").evaluateAll(rows=>rows.map(row=>Number(row.getAttribute("data-pokemon-row-id"))));
  // A real party read can replace the source view while the mouse is down.
  // Cancellation must remove the preview without sending a stale permutation.
  await dragFirstToSecond(page,async()=>{
    await expect.poll(renderedOrder).toEqual(reversed);
    await page.evaluate(async()=>{
      const path="/src/phaser-game/services/PhaserNetworkService.ts"; const net=await import(path);
      net.sendPokemonPartyRequest();
    });
    await expect.poll(renderedOrder).toEqual(original);
  });
  expect(faults.requests).toBe(0); expect(await order()).toEqual(original);
  await dragFirstToSecond(page);
  await expect.poll(()=>faults.replies.length).toBe(1);
  expect(faults.replies[0].party.map(p=>p.rowId)).toEqual(reversed);
  expect(faults.replies[0].inventory.commandRevision).toBe(1);
  if(loseReply) {
    // Dropping the reply must not leave an optimistic order in the shared store.
    expect(await order()).toEqual(original);
    await dragFirstToSecond(page);
    expect(faults.requests).toBe(1);
  }
  await expect.poll(order,{timeout:20000}).toEqual(reversed);
  await expect.poll(renderedOrder).toEqual(reversed);
  await page.screenshot({path:testInfo.outputPath("party-reordered.png")});
  await dragFirstToSecond(page);
  await expect.poll(()=>faults.replies.length).toBe(2);
  expect(faults.replies[1].party.map(p=>p.rowId)).toEqual(original);
  expect(faults.replies[1].inventory.commandRevision).toBe(2);
  await expect.poll(()=>faults.rejections).toBe(2);
  if(loseReply) {
    expect(await order()).toEqual(reversed);
    errors.assertNoSevereErrors();
    const receipt=await crash(); await record({family:"party-reorder",characterId,receipt,commands:faults.successes});
    await page.reload(); await page.getByRole("button",{name:"PLAY AS GUEST"}).click();
    await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
    errors=collectPageErrors(page);
  } else {
    await expect.poll(order).toEqual(original);
    await quitToCharacterSelect(page);
  }
  await enterWorld(page,character); await waitForNoMapLoading(page);
  await expect.poll(order).toEqual(original); await expect.poll(renderedOrder).toEqual(original);
  expect(await sql(`SELECT revision FROM character_shop_state WHERE character_id=${characterId}`)).toBe("2");
  expect((await getGameState(page)).inventory.money).toBe(before.inventory.money);
  expect((await getGameState(page)).pokemon.party).toEqual(before.pokemon.party);
  // Observe delivery through the real listener dispatch before asserting that
  // the earlier committed reply cannot rewind this scene's recovered party.
  // A killed connection cannot deliver its old packet; the normal-reentry case
  // exercises actual late delivery, while unit checks cover retired connections.
  if(!loseReply) {
    await page.evaluate(async()=>{
      const path="/src/phaser-game/services/PhaserNetworkService.ts"; const net=await import(path);
      const target=window as typeof window & {lateReorderReply?:boolean};
      const stop=net.onInventoryCommand(91,()=>{target.lateReorderReply=true;stop()});
    });
    faults.deliverReply(0);
    await expect.poll(()=>page.evaluate(()=>(window as typeof window & {lateReorderReply?:boolean}).lateReorderReply)).toBe(true);
    expect(await order()).toEqual(original);
  }
  expect(faults.requests).toBe(2); expect(faults.successes).toBe(2);
  await quitToCharacterSelect(page); errors.assertNoSevereErrors();
 });
}
