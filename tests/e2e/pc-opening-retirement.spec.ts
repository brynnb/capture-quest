import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {collectPageErrors} from "./helpers/errors";
import {clickTile,pressMovement} from "./helpers/input";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {getGameState,waitForNoMapLoading,waitForPlayerTile} from "./helpers/state";

for(const mode of ["move","reentry"] as const) {
  test(`delayed PC opening cannot reopen after ${mode}`,async({page})=>{
    test.setTimeout(90000);
    const errors=collectPageErrors(page);
    let release:(()=>void)|undefined;
    let starts=0;
    await page.routeWebSocket("**/ws",socket=>{
      const server=socket.connectToServer();
      socket.onMessage(message=>server.send(message));
      server.onMessage(message=>{
        if(Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.PokemonPCOpenResponse) {
          const reply=JSON.parse(message.subarray(6).toString());
          if(reply.success && !release) { starts++; release=()=>socket.send(message); return; }
        }
        socket.send(message);
      });
    });
    const character=await createGuestCharacterAndEnterWorld(page);
    await jumpToScenario(page,"debug_pokemon_center_pc_ready");
    await clickTile(page,13,4); await waitForPlayerTile(page,13,4);
    await expect.poll(()=>starts).toBe(1);
    expect((await getGameState(page)).pokemon.pc.isOpen).toBe(false);
    if(mode==="move") {
      await pressMovement(page,"left"); await waitForPlayerTile(page,12,4);
    } else {
      await quitToCharacterSelect(page); await enterWorld(page,character); await waitForNoMapLoading(page);
    }
    await page.evaluate(async()=>{
      const path="/src/phaser-game/services/PhaserNetworkService.ts";const net=await import(path);
      const target=window as typeof window & {retiredPCOpeningDelivered?:boolean};
      const stop=net.onInventoryCommand(109,()=>{target.retiredPCOpeningDelivered=true;stop()});
    });
    release!();
    await expect.poll(()=>page.evaluate(()=>(window as typeof window & {retiredPCOpeningDelivered?:boolean}).retiredPCOpeningDelivered)).toBe(true);
    expect((await getGameState(page)).pokemon.pc.isOpen).toBe(false);
    await expect(page.getByTestId("pokemon-pc-main-menu")).toBeHidden();
    await quitToCharacterSelect(page); errors.assertNoSevereErrors();
  });
}
