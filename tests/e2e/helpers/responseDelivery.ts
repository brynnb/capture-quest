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
 const envelope=data as {requestId?:unknown;battleId?:unknown}|null;
 const id=typeof envelope?.requestId==="string" ? envelope.requestId : envelope?.battleId;
 if(opcodes.includes(opcode) && data)void(window as unknown as {observeResponseDelivery:(opcode:number,id:string)=>Promise<void>}).observeResponseDelivery(opcode,typeof id==="string" ? id : "");
 dispatch?.(opcode,data);
 };
 },opcodes);
}
