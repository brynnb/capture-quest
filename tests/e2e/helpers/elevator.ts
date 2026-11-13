import {expect,type Page} from "@playwright/test";
import {clickTile} from "./input";
import {waitForPlayerIdle,waitForPlayerTile} from "./state";

// Observe and click actual Phaser overlay text, never manufacture a choice.
export async function overlayTextPoint(page:Page,text:string){
 return page.evaluate(text=>window.__capturequestTest!.overlayTextToViewport(text),text);
}
export async function clickOverlayText(page:Page,text:string){
 await expect.poll(()=>overlayTextPoint(page,text)).not.toBeNull();
 const point=await overlayTextPoint(page,text);await page.mouse.click(point!.x,point!.y);
}
export async function openCeladonElevatorPanel(page:Page){
 await clickTile(page,3,1);await waitForPlayerTile(page,3,1);await waitForPlayerIdle(page);await clickTile(page,3,0);
}
