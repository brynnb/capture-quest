import {expect,test,type Page} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {createGuestCharacterAndEnterWorld,enterWorld,quitToCharacterSelect} from "./helpers/auth";
import {isolatedCrashRuntime} from "./helpers/processRecovery";
import {getGameState,waitForNoMapLoading} from "./helpers/state";

// Routed sockets have no native frame events. Observe the production parser,
// preserving dispatch, so replay checks prove browser receipt before assertions.
async function observeReadDelivery(page:Page,receive:(opcode:number,requestId:string)=>void){
  await page.exposeFunction("observeReadDelivery",receive);
  await page.evaluate(async opcodes=>{
    const path="/src/net/index.ts";
    const {WorldSocket}=await import(path) as typeof import("../../src/net/index");
    const dispatch=WorldSocket.onJson;
    WorldSocket.onJson=(opcode,data)=>{
      if(opcodes.includes(opcode) && data && typeof(data as {requestId?:unknown}).requestId==="string")void(window as unknown as {observeReadDelivery:(opcode:number,id:string)=>Promise<void>}).observeReadDelivery(opcode,(data as {requestId:string}).requestId);
      dispatch?.(opcode,data);
    };
  },[OpCodes.TrainerCardResponse,OpCodes.PokedexListResponse,OpCodes.PokedexStatusResponse]);
}

test("trainer-card timeout retries with a fresh identity and ignores its late reply",async({page,context})=>{
  test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires the private database runner");
  test.setTimeout(60000);
  const {sql}=await isolatedCrashRuntime();
  const requests:string[]=[];const held:Array<{id:string;deliver:()=>void}>=[];let hold=true;let delivered=0;
  await context.routeWebSocket("**/ws",socket=>{
    const server=socket.connectToServer();
    socket.onMessage(message=>{
      if(Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.TrainerCardRequest)requests.push(JSON.parse(message.subarray(6).toString()).requestId);
      server.send(message);
    });
    server.onMessage(message=>{
      if(hold && Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.TrainerCardResponse){
        const reply=JSON.parse(message.subarray(6).toString());
        if(reply.success && reply.requestId){held.push({id:reply.requestId,deliver:()=>socket.send(message)});return;}
      }
      socket.send(message);
    });
  });
  await createGuestCharacterAndEnterWorld(page);
  await expect.poll(async()=>(await getGameState(page)).player.internalId??0).toBeGreaterThan(0);
  const id=(await getGameState(page)).player.internalId!;
  await observeReadDelivery(page,(opcode,requestId)=>{if(opcode===OpCodes.TrainerCardResponse && held.some(reply=>reply.id===requestId))delivered++;});
  await page.getByRole("button",{name:"Trainer",exact:true}).click();
  await expect.poll(()=>held.length).toBeGreaterThan(0);
  const error=page.getByText("Trainer information could not be refreshed. Close and reopen the view to retry.",{exact:true});
  await expect(error).toBeAttached({timeout:20000});
  await page.keyboard.press("Escape");await expect(error).toBeVisible();
  const beforeRetry=requests.length;hold=false;
  await sql(`INSERT INTO character_wallet(character_id,pokedollars) VALUES(${id},300) ON CONFLICT(character_id) DO UPDATE SET pokedollars=300`);
  await page.getByRole("button",{name:"Trainer",exact:true}).click();
  const money=page.getByText("Money",{exact:true}).locator("..").getByText("¥300",{exact:true});
  await expect(money).toBeVisible();expect(requests).toHaveLength(beforeRetry+1);expect(new Set(requests).size).toBe(requests.length);
  held.forEach(reply=>reply.deliver());await expect.poll(()=>delivered).toBe(held.length);await expect(money).toBeVisible();
  await page.keyboard.press("Escape");await quitToCharacterSelect(page);
});

test("retired trainer-card response cannot rewind same-character reentry",async({page,context},testInfo)=>{
  test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires the private database runner");
  test.setTimeout(90000);
  const {sql}=await isolatedCrashRuntime();
  let oldId="";
  let releaseOld:(()=>void)|undefined;
  let delivered=0;
  let oldListId="";let repeatList:(()=>void)|undefined;let listDelivered=0;
  await context.routeWebSocket("**/ws",socket=>{
    const server=socket.connectToServer();
    socket.onMessage(message=>server.send(message));
    server.onMessage(message=>{
      if(!releaseOld && Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.TrainerCardResponse){
        const reply=JSON.parse(message.subarray(6).toString());
        if(reply.success && reply.requestId){oldId=reply.requestId;releaseOld=()=>socket.send(message);return;}
      }
      if(!repeatList && Buffer.isBuffer(message) && message.length>=6 && message.readUInt16LE(4)===OpCodes.PokedexListResponse){
        const reply=JSON.parse(message.subarray(6).toString());
        if(reply.success && reply.requestId){oldListId=reply.requestId;repeatList=()=>socket.send(message);}
      }
      socket.send(message);
    });
  });
  const character=await createGuestCharacterAndEnterWorld(page);
  await expect.poll(async()=>(await getGameState(page)).player.internalId??0).toBeGreaterThan(0);
  const id=(await getGameState(page)).player.internalId!;
  await observeReadDelivery(page,(opcode,requestId)=>{if(opcode===OpCodes.TrainerCardResponse && requestId===oldId)delivered++;if(opcode===OpCodes.PokedexListResponse && requestId===oldListId)listDelivered++;});
  await page.getByRole("button",{name:"Trainer",exact:true}).click();
  await expect.poll(()=>Boolean(releaseOld)).toBe(true);
  await page.keyboard.press("Escape");
  await quitToCharacterSelect(page);
  await sql(`INSERT INTO character_wallet(character_id,pokedollars) VALUES(${id},200) ON CONFLICT(character_id) DO UPDATE SET pokedollars=200`);
  await enterWorld(page,character);await waitForNoMapLoading(page);
  await page.getByRole("button",{name:"Trainer",exact:true}).click();
  await expect(page.getByText("Money",{exact:true}).locator("..").getByText("¥200",{exact:true})).toBeVisible();
  releaseOld!();
  await expect.poll(()=>delivered).toBe(1);
  await expect(page.getByText("Money",{exact:true}).locator("..").getByText("¥200",{exact:true})).toBeVisible();
  await page.screenshot({path:testInfo.outputPath("trainer-after-retired-reply.png")});
  await page.keyboard.press("Escape");
  const counts=(await sql(`SELECT COALESCE(SUM(seen),0)||','||COALESCE(SUM(caught),0) FROM character_pokedex WHERE character_id=${id}`)).split(',').map(Number);
  await page.getByRole("button",{name:"Pokédex",exact:true}).click();
  await expect(page.getByText(`Seen ${counts[0]} / Caught ${counts[1]}`,{exact:true})).toBeVisible();
  await expect.poll(()=>listDelivered).toBe(1);
  await page.keyboard.press("Escape");
  const unseen=await sql(`SELECT id FROM phaser_pokemon WHERE id BETWEEN 1 AND 151 AND id NOT IN(SELECT pokemon_id FROM character_pokedex WHERE character_id=${id}) ORDER BY id LIMIT 1`);
  expect(Number(unseen)).toBeGreaterThan(0);
  await sql(`INSERT INTO character_pokedex(character_id,pokemon_id,seen,caught,first_seen_at) VALUES(${id},${unseen},1,0,CURRENT_TIMESTAMP)`);
  await page.getByRole("button",{name:"Pokédex",exact:true}).click();
  const currentCounts=`Seen ${counts[0]+1} / Caught ${counts[1]}`;
  await expect(page.getByText(currentCounts,{exact:true})).toBeVisible();
  repeatList!();await expect.poll(()=>listDelivered).toBe(2);
  await expect(page.getByText(currentCounts,{exact:true})).toBeVisible();
  await page.screenshot({path:testInfo.outputPath("pokedex-after-duplicate-list.png")});
  await page.keyboard.press("Escape");
  await quitToCharacterSelect(page);
});
