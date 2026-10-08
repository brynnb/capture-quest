import {afterEach,expect,test,vi} from "vitest";
const network=vi.hoisted(()=>({listeners:new Set<(data:unknown)=>void>(),send:vi.fn()}));
vi.mock("./PhaserNetworkService",()=>({isConnected:()=>true,onWarps:(receive:(data:unknown)=>void)=>{network.listeners.add(receive);return()=>network.listeners.delete(receive)},requestWarps:network.send}));
vi.mock("@/stores/PlayerCharacterStore",()=>({default:{getState:()=>({characterProfile:{id:42}})}}));
import {MapDataService} from "./MapDataService";
const response=(requestId:string,mapId:number)=>({success:true,requestId,characterId:42,mapId,warps:[]});
const receive=(data:unknown)=>network.listeners.forEach(fn=>fn(data));
afterEach(()=>{vi.useRealTimers();network.send.mockReset();network.listeners.clear()});
test("warp replies cannot settle another map's overlapping read",async()=>{
 const service=new MapDataService();const a=service.fetchWarps(33);const b=service.fetchWarps(34);
 const [first,second]=network.send.mock.calls.map(call=>call[0]);receive(response(second.requestId,34));await b;expect(network.listeners.size).toBe(1);
 receive(response(first.requestId,33));await a;expect(network.listeners.size).toBe(0);
});
test("warp cancellation retires listeners and a late reply cannot settle the next read",async()=>{
 const service=new MapDataService();const controller=new AbortController();const old=service.fetchWarps(33,controller.signal);const abort=expect(old).rejects.toMatchObject({name:"AbortError"});
 const first=network.send.mock.calls[0][0];controller.abort();await abort;
 const current=service.fetchWarps(34);const second=network.send.mock.calls[1][0];
 receive(response(first.requestId,33));expect(network.listeners.size).toBe(1);receive(response(second.requestId,34));await current;expect(network.listeners.size).toBe(0);
});
test("warp read timeout rejects rather than fabricating an empty catalog",async()=>{
 vi.useFakeTimers();const run=new MapDataService().fetchWarps(33);const timeout=expect(run).rejects.toThrow("Timeout");await vi.advanceTimersByTimeAsync(10000);await timeout;expect(network.listeners.size).toBe(0);
});
