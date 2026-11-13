import {expect,test} from "@playwright/test";
import * as OpCodes from "../../src/net/generated/opcodes";
import {loginAsGuest,uniqueTrainerName} from "./helpers/auth";
import {isolatedCrashRuntime} from "./helpers/processRecovery";

test("a delayed earlier name check cannot replace the current unavailable result",async({page,context})=>{
 test.skip(process.env.CQ_E2E_DATABASE_FIXTURE!=="true","Requires private database fixtures");
 const {sql}=await isolatedCrashRuntime();const first=uniqueTrainerName(),second=uniqueTrainerName();
 await sql(`INSERT INTO character_data(name,deleted_at) VALUES('${second}',CURRENT_TIMESTAMP)`);
 let release:(()=>void)|undefined;let delivered=0;
 await context.routeWebSocket("**/ws",socket=>{
  const server=socket.connectToServer();socket.onMessage(message=>server.send(message));
  server.onMessage(message=>{
   if(Buffer.isBuffer(message)&&message.length>=6&&message.readUInt16LE(4)===OpCodes.ValidateNameResponse){
    const reply=JSON.parse(message.subarray(6).toString());
    if(reply.name===first && reply.success){release=()=>socket.send(message);return;}
   }
   socket.send(message);
  });
 });
 await loginAsGuest(page);await page.getByRole("button",{name:"CREATE NEW CHARACTER"}).first().click();
 await page.exposeFunction("observeNameDelivery",(name:string)=>{if(name===first)delivered++;});
 await page.evaluate(async opcode=>{
  const path="/src/net/index.ts";
  const {WorldSocket}=await import(path) as typeof import("../../src/net/index");const dispatch=WorldSocket.onJson;
  WorldSocket.onJson=(op,data)=>{if(op===opcode)void(window as unknown as {observeNameDelivery:(name:string)=>Promise<void>}).observeNameDelivery((data as {name:string}).name);dispatch?.(op,data);};
 },OpCodes.ValidateNameResponse);
 const input=page.getByPlaceholder("Enter character name");await input.fill(first);await expect.poll(()=>Boolean(release)).toBe(true);
 await input.fill(second);await expect(page.getByText("Name is already taken.",{exact:true})).toBeVisible();
 release!();await expect.poll(()=>delivered).toBe(1);await expect(page.getByText("Name is already taken.",{exact:true})).toBeVisible();
});
