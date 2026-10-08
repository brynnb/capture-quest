import {expect,test} from "@playwright/test";
import {randomUUID} from "node:crypto";
import {loginAsGuest,createCharacter,enterWorld,quitToCharacterSelect,uniqueTrainerName} from "./helpers/auth";
import {collectPageErrors} from "./helpers/errors";
import {waitForNoMapLoading} from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";

for(const mode of ["account","connection"] as const) test(`pending catalog survives ${mode} replacement without applying old data`,async({page})=>{
 test.setTimeout(120000);
 const errors=collectPageErrors(page);let held:Buffer|undefined;let reads=0;let connections=0;
 await page.routeWebSocket("**/ws",socket=>{
  connections++;const server=socket.connectToServer();
  socket.onMessage(message=>{
   if(Buffer.isBuffer(message)&&message.length>=6&&message.readUInt16LE(4)===OpCodes.StaticDataRequest){
    reads++;if(reads===2&&held) socket.send(held);
   }
   server.send(message);
  });
  server.onMessage(message=>{
   if(!held&&Buffer.isBuffer(message)&&message.length>=6&&message.readUInt16LE(4)===OpCodes.StaticDataResponse){
    const data=JSON.parse(message.subarray(6).toString());
    if(data.success){
     if(data.classes.length)data.classes[0].name="RETIRED CATALOG";
     const body=Buffer.from(JSON.stringify(data));held=Buffer.alloc(body.length+6);
     held.writeUInt32LE(body.length+2,0);held.writeUInt16LE(OpCodes.StaticDataResponse,4);body.copy(held,6);
     if(mode==="connection"){socket.close({code:1012,reason:"isolated read retirement"});server.close();}
     return;
    }
   }
   socket.send(message);
  });
 });
 const initialToken=randomUUID();const login=loginAsGuest(page,initialToken);
 await expect.poll(()=>!!held,{timeout:20000}).toBe(true);
 if(mode==="account"){
  const status=await page.evaluate(async token=>{const path="/src/services/authService.ts";const auth=await import(path);return(await auth.loginGuest(token)).status;},randomUUID());
  expect(status).toBeGreaterThan(0);expect(connections).toBe(1);
 }else{
  await expect(page.getByRole("button",{name:"PLAY AS GUEST"})).toBeVisible();
  await expect.poll(()=>connections,{timeout:15000}).toBeGreaterThan(1);
  await expect.poll(()=>page.evaluate(async()=>{const path="/src/net/index.ts";return(await import(path)).WorldSocket.isConnected;})).toBe(true);
  await page.getByRole("button",{name:"PLAY AS GUEST"}).click();
 }
 await login;
 expect(reads).toBe(2);
 expect(await page.evaluate(async()=>{const path="/src/stores/StaticDataStore.ts";const store=(await import(path)).default;return store.getState().classes.some((row:{name:string})=>row.name==="RETIRED CATALOG");})).toBe(false);
 const name=uniqueTrainerName();await createCharacter(page,name);await enterWorld(page,name);await waitForNoMapLoading(page);await quitToCharacterSelect(page);
 if(mode==="account"){
  await page.evaluate(async token=>{const path="/src/services/authService.ts";await(await import(path)).loginGuest(token);},initialToken);
  await expect(page.getByRole("button",{name,exact:true})).toHaveCount(0);
 }
 errors.assertNoSevereErrors();
});
