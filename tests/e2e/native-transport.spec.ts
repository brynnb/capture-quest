import {expect,test,type Page} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {pressMovement} from "./helpers/input";
import {waitForMap,waitForPlayerTile,waitForPlayerIdle} from "./helpers/state";

async function assertNative(page:Page){
 const state=await page.evaluate(async()=>{
  const path="/src/net/index.ts";
  const {WorldSocket}=await import(path) as typeof import("../../src/net/index");
  const socket=WorldSocket as unknown as {useWebSocket:boolean;webtransport:unknown};
  return {connected:WorldSocket.isConnected,websocket:socket.useWebSocket,native:Boolean(socket.webtransport)};
 });
 expect(state).toEqual({connected:true,websocket:false,native:true});return state;
}

test("native WebTransport serves login, movement and informational reads",async({page},testInfo)=>{
  test.skip(process.env.CQ_E2E_TRANSPORT!=="native","Requires native isolated transport mode");
  test.setTimeout(90000);
  const character=await createGuestCharacterAndEnterWorld(page);
  await waitForMap(page,"REDS_HOUSE_2F");await waitForPlayerTile(page,3,6);
  await assertNative(page);
  await pressMovement(page,"right");await waitForPlayerTile(page,4,6);await waitForPlayerIdle(page);
  await page.getByRole("button",{name:"Trainer",exact:true}).click();
  await expect(page.getByText("Play Time",{exact:true})).toBeVisible();
  await page.screenshot({path:testInfo.outputPath("native-trainer-card.png")});
  await page.keyboard.press("Escape");await quitToCharacterSelect(page);
  await enterWorld(page,character);await waitForMap(page,"REDS_HOUSE_2F");await waitForPlayerTile(page,4,6);
  await assertNative(page);
  await quitToCharacterSelect(page);
});

test("native WebTransport retires across SIGKILL and reenters without WebSocket fallback",async({page},testInfo)=>{
 test.skip(process.env.CQ_E2E_TRANSPORT!=="native" || process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires native exact-process crash mode");
 test.setTimeout(120000);const {crash,record}=await isolatedCrashRuntime();
 const character=await createGuestCharacterAndEnterWorld(page);await waitForMap(page,"REDS_HOUSE_2F");await waitForPlayerTile(page,3,6);
 const before=await assertNative(page);const receipt=await crash();
 await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible({timeout:20000});
 await page.getByRole("button",{name:"PLAY AS GUEST"}).click();await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
 await enterWorld(page,character);await waitForMap(page,"REDS_HOUSE_2F");await waitForPlayerTile(page,3,6);const after=await assertNative(page);
 await page.getByRole("button",{name:"Trainer",exact:true}).click();await expect(page.getByText("Play Time",{exact:true})).toBeVisible();
 await page.screenshot({path:testInfo.outputPath("native-after-process-replacement.png")});
 await record({family:"native-transport",before,after,receipt});
 await page.keyboard.press("Escape");await quitToCharacterSelect(page);
});

test("native fishing commits one receipt and restores its battle after process replacement",async({page},testInfo)=>{
 test.skip(process.env.CQ_E2E_TRANSPORT!=="native" || process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires native exact-process runner");test.setTimeout(120000);
 const {crash,sql,record}=await isolatedCrashRuntime();const character=await createGuestCharacterAndEnterWorld(page);
 const scenarios=await import("./helpers/scenarioDebugger");await scenarios.jumpToScenario(page,"debug_inventory_old_rod_water_ready");
 const state=await import("./helpers/state");await state.waitForNoMapLoading(page);await state.waitForPlayerIdle(page);const before=await assertNative(page);
 const id=(await state.getGameState(page)).player.internalId!;await page.getByRole("button",{name:"Bag",exact:true}).click();await page.getByTestId("inventory-item-old-rod").click();await expect(page.getByTestId("battle-overlay")).toBeVisible();
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='fishing'`)).toBe("1");
 const receipt=await crash();await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible({timeout:20000});await page.getByRole("button",{name:"PLAY AS GUEST"}).click();await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
 await enterWorld(page,character);await state.waitForNoMapLoading(page);await expect(page.getByTestId("battle-overlay")).toBeVisible();const after=await assertNative(page);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='fishing'`)).toBe("1");expect(await sql(`SELECT count(*) FROM character_battle_state WHERE character_id=${id}`)).toBe("1");
 await page.screenshot({path:testInfo.outputPath("native-fishing-restored.png")});await record({family:"native-fishing",id,before,after,receipt,revision:1});await quitToCharacterSelect(page);
});

test("native SURF commits one receipt and restores water entry after process replacement",async({page},testInfo)=>{
 test.skip(process.env.CQ_E2E_TRANSPORT!=="native" || process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires native exact-process runner");test.setTimeout(120000);
 const {crash,sql,record}=await isolatedCrashRuntime();const character=await createGuestCharacterAndEnterWorld(page);
 const scenarios=await import("./helpers/scenarioDebugger");const state=await import("./helpers/state");const input=await import("./helpers/input");
 await scenarios.jumpToScenario(page,"debug_field_move_surf_ready");await state.waitForNoMapLoading(page);await waitForPlayerIdle(page);
 const before=await assertNative(page);const id=(await state.getGameState(page)).player.internalId!;
 await input.clickTile(page,4,17);await expect.poll(async()=>(await state.getGameState(page)).dialogue.isChoicePending).toBe(true);await input.pressSpace(page);
 const pose=async()=>{const value=await state.getGameState(page);return{x:value.player.x,y:value.player.y,surfing:value.player.isSurfing};};
 await expect.poll(pose).toEqual({x:4,y:17,surfing:true});
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='surf'`)).toBe("1");
 const receipt=await crash();await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible({timeout:20000});await page.getByRole("button",{name:"PLAY AS GUEST"}).click();await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
 await enterWorld(page,character);await state.waitForNoMapLoading(page);await expect.poll(pose).toEqual({x:4,y:17,surfing:true});const after=await assertNative(page);
 expect(await sql(`SELECT revision FROM character_field_command_state WHERE character_id=${id} AND domain='surf'`)).toBe("1");
 await page.screenshot({path:testInfo.outputPath("native-surf-restored.png")});await record({family:"native-surf",id,before,after,receipt,revision:1});await quitToCharacterSelect(page);
});


test("native pickup keeps its grant, revision and removed actor after process replacement",async({page},testInfo)=>{
 test.skip(process.env.CQ_E2E_TRANSPORT!=="native" || process.env.CQ_E2E_CRASH_RECOVERY!=="true","Requires native exact-process runner");test.setTimeout(120000);
 const runtime=await isolatedCrashRuntime();const {sql,crash,record}=runtime;const fixture=await runtime.pickupFixture();
 const state=await import("./helpers/state");const input=await import("./helpers/input");
 const character=await createGuestCharacterAndEnterWorld(page);await state.waitForNoMapLoading(page);await waitForPlayerIdle(page);const before=await assertNative(page);
 const id=(await state.getGameState(page)).player.internalId!;
 const quantity=()=>sql(`SELECT COALESCE(sum(ii.quantity),0) FROM cq_character_inventory ci JOIN cq_item_instances ii ON ii.id=ci.item_instance_id WHERE ci.character_id=${id} AND ii.item_id=${fixture.item}`);
 const initial=Number(await quantity());await input.clickTile(page,4,6);await state.waitForMessage(page,/Picked up /i);
 const present=async()=>(await state.getGameState(page)).visibleActors.some(actor=>actor.name==='CQ_PICKUP_FIXTURE');
 await expect.poll(present).toBe(false);expect(await quantity()).toBe(String(initial+1));
 const receipt=await crash();await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible({timeout:20000});await page.getByRole("button",{name:"PLAY AS GUEST"}).click();await expect(page.getByRole("heading",{name:"SELECT A CHARACTER"})).toBeVisible();
 await enterWorld(page,character);await state.waitForNoMapLoading(page);await expect.poll(present).toBe(false);const after=await assertNative(page);
 expect(await quantity()).toBe(String(initial+1));expect(await sql(`SELECT revision FROM character_shop_state WHERE character_id=${id}`)).toBe("1");expect(await sql(`SELECT count(*) FROM character_collected_items WHERE character_id=${id} AND object_id=${fixture.object}`)).toBe("1");
 await page.screenshot({path:testInfo.outputPath("native-pickup-restored.png")});await record({family:"native-pickup",id,before,after,fixture,receipt,quantity:initial+1,revision:1});await quitToCharacterSelect(page);
});
