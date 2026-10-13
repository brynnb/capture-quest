import {expect,test} from "@playwright/test";
import {createGuestCharacterAndEnterWorld,quitToCharacterSelect} from "./helpers/auth";
import {jumpToScenario} from "./helpers/scenarioDebugger";
import {pressSpace,dismissDialogue,pressMovement} from "./helpers/input";
import {getGameState,waitForNoMapLoading,waitForPlayerTile} from "./helpers/state";

test("CUT owned tile reread retains replacement art and walkability",async({page},info)=>{
 test.setTimeout(90000);
 await createGuestCharacterAndEnterWorld(page);await jumpToScenario(page,"debug_field_move_cut_ready");await waitForNoMapLoading(page);await waitForPlayerTile(page,189,-169);
 const read=()=>page.evaluate(async()=>{
  const path="/src/phaser-game/services/MapDataService.ts";const {MapDataService}=await import(path);
  const tiles=await new MapDataService().fetchTilesInBounds(9999,{minX:189,maxX:189,minY:-170,maxY:-170});
  if(tiles.length!==1)throw new Error("Expected one tree square");return tiles[0];
 });
 const scope=()=>page.evaluate(async()=>{const path="/src/phaser-game/services/GameplayRecoveryService.ts";return (await (await import(path)).readCurrentGameplayState()).cutScope;});
 const originalScope=await scope();expect(originalScope).toBeTruthy();
 const before=await read();expect(before.rawFootTileId).toBe(0x3d);
 await pressSpace(page);await expect.poll(async()=>(await getGameState(page)).dialogue.isOpen).toBe(true);await dismissDialogue(page);
 const after=await read();expect(after.tileImageId).not.toBe(before.tileImageId);expect(after.rawFootTileId).toBe(0x52);expect(after.collisionType).toBe(1);expect(after.contentOrigin).toBe("cut");expect(after.coordinateOrigin).toBe(before.coordinateOrigin);expect(after.isNativeGameData).toBe(before.isNativeGameData);
 // Reading the source again must not revert the server's transient view.
 expect(await read()).toEqual(after);expect(await scope()).toBe(originalScope);
 await pressMovement(page,"up");await waitForPlayerTile(page,189,-170);
 await page.screenshot({path:info.outputPath("cut-projected-reread.png")});
 await jumpToScenario(page,"debug_escape_rope_ready");await waitForNoMapLoading(page);
 await jumpToScenario(page,"debug_field_move_cut_ready");await waitForNoMapLoading(page);
 expect(await scope()).not.toBe(originalScope);const reset=await read();expect(reset.tileImageId).toBe(before.tileImageId);expect(reset.rawFootTileId).toBe(0x3d);
 await quitToCharacterSelect(page);
});
