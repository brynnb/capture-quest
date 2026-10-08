import {expect,test} from "@playwright/test";
import {loginAsGuest,createCharacter,enterWorld,uniqueTrainerName} from "./helpers/auth";
import {collectPageErrors} from "./helpers/errors";
import {waitForNoMapLoading} from "./helpers/state";
import * as OpCodes from "../../src/net/generated/opcodes";
test("catalog timeout shows explicit retry and ignores the older reply",async({page})=>{
 test.setTimeout(90000);
 const errors=collectPageErrors(page);let reads=0;let held:Buffer|undefined;
 await page.routeWebSocket("**/ws",socket=>{
  const server=socket.connectToServer();
  socket.onMessage(message=>{
   if(Buffer.isBuffer(message)&&message.length>=6&&message.readUInt16LE(4)===OpCodes.StaticDataRequest){
    reads++;
    if(reads===2&&held) socket.send(held);
   }
   server.send(message);
  });
  server.onMessage(message=>{
   if(!held&&Buffer.isBuffer(message)&&message.length>=6&&message.readUInt16LE(4)===OpCodes.StaticDataResponse){
    const data=JSON.parse(message.subarray(6).toString());
    if(data.success){
     if(data.classes.length) data.classes[0].name="STALE CATALOG";
     const body=Buffer.from(JSON.stringify(data));held=Buffer.alloc(body.length+6);
     held.writeUInt32LE(body.length+2,0);held.writeUInt16LE(OpCodes.StaticDataResponse,4);body.copy(held,6);return;
    }
   }
   socket.send(message);
  });
 });
 const login=loginAsGuest(page);
 await expect(page.getByRole("button",{name:"Retry",exact:true})).toBeVisible({timeout:20000});
 expect(reads).toBe(1);expect(held).toBeDefined();
 await page.getByRole("button",{name:"Retry",exact:true}).click();await login;
 expect(reads).toBe(2);
 expect(await page.evaluate(async()=>{
  const path="/src/stores/StaticDataStore.ts";const {default:store}=await import(path);
  return store.getState().classes.some((item:{name:string})=>item.name==="STALE CATALOG");
 })).toBe(false);
 const name=uniqueTrainerName();await createCharacter(page,name);await enterWorld(page,name);await waitForNoMapLoading(page);
 errors.assertNoSevereErrors();
});
