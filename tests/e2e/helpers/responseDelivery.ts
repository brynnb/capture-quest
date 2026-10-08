import type {Page} from "@playwright/test";

// Routed sockets do not expose native frame events. Preserve the production
// dispatcher while observing browser receipt before making stale-reply claims.
export async function observeResponseDelivery(page:Page,opcodes:number[],receive:(opcode:number,requestId:string)=>void){
 await page.exposeFunction("observeResponseDelivery",receive);
 await page.evaluate(async opcodes=>{
 const path="/src/net/index.ts";
 const {WorldSocket}=await import(path) as typeof import("../../../src/net/index");
 const dispatch=WorldSocket.onJson;
 WorldSocket.onJson=(opcode,data)=>{
 if(opcodes.includes(opcode) && data && typeof(data as {requestId?:unknown}).requestId==="string")void(window as unknown as {observeResponseDelivery:(opcode:number,id:string)=>Promise<void>}).observeResponseDelivery(opcode,(data as {requestId:string}).requestId);
 dispatch?.(opcode,data);
 };
 },opcodes);
}
