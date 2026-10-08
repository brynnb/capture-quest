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
