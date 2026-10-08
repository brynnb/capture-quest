import { afterEach, expect, test, vi } from "vitest";
const net=vi.hoisted(()=>({listeners:new Set<(data:unknown)=>void>(),send:vi.fn()}));
vi.mock("@/phaser-game/services/PhaserNetworkService",()=>({isConnected:()=>true,onStaticContent:(receive:(data:unknown)=>void)=>{net.listeners.add(receive);return()=>net.listeners.delete(receive)},requestStaticContent:net.send}));
import { getStaticData, getCharCreateData } from "./characterService";
const response=(requestId:string)=>({success:true,requestId,classes:[],maps:[],factions:[],startCities:[]});
const receive=(data:unknown)=>net.listeners.forEach(fn=>fn(data));
afterEach(()=>{net.send.mockReset();net.listeners.clear();vi.useRealTimers();});
test("overlapping catalog endpoints settle only their matching request",async()=>{
 const first=getStaticData();const second=getCharCreateData();
 const [a,b]=net.send.mock.calls.map(call=>call[0]);
 receive(response(b));await second;expect(net.listeners.size).toBe(1);
 receive(response(a));await first;expect(net.listeners.size).toBe(0);
});
test("timeout and cancellation remove listeners; old packets cannot settle retry",async()=>{
 vi.useFakeTimers();const first=getStaticData();const timed=expect(first).rejects.toThrow("Timeout");
 const old=net.send.mock.calls[0][0];await vi.advanceTimersByTimeAsync(10000);await timed;
 const controller=new AbortController();const retry=getStaticData(controller.signal);const cancelled=expect(retry).rejects.toMatchObject({name:"AbortError"});
 receive(response(old));expect(net.listeners.size).toBe(1);controller.abort();await cancelled;expect(net.listeners.size).toBe(0);
});
