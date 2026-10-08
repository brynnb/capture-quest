import {afterEach,expect,test,vi} from "vitest";
const net=vi.hoisted(()=>({listeners:new Set<(data:unknown)=>void>(),send:vi.fn()}));
vi.mock("./PhaserNetworkService",()=>({isConnected:()=>true,onTiles:(fn:(data:unknown)=>void)=>{net.listeners.add(fn);return()=>net.listeners.delete(fn)},requestTiles:net.send}));
vi.mock("./RuntimeAssetCompatibility",()=>({ensureRuntimeTileCatalogCurrent:vi.fn(async()=>{})}));
vi.mock("@/stores/PlayerCharacterStore",()=>({default:{getState:()=>({characterProfile:{id:42}})}}));
import {MapDataService} from "./MapDataService";
const receive=(data:unknown)=>net.listeners.forEach(fn=>fn(data));
const reply=(requestId:string,mapId:number)=>({success:true,requestId,characterId:42,mapId,tiles:[],nextAfterId:0,hasMore:false});
afterEach(()=>{net.send.mockReset();net.listeners.clear();vi.useRealTimers()});
test("different tile service instances cannot collide or settle each other's reads",async()=>{
 const a=new MapDataService().fetchTiles(33);const b=new MapDataService().fetchTiles(33);await Promise.resolve();await Promise.resolve();
 const [first,second]=net.send.mock.calls.map(call=>call[0]);expect(first.requestId).not.toBe(second.requestId);
 receive(reply(second.requestId,33));await b;expect(net.listeners.size).toBe(1);
 receive(reply(first.requestId,33));await a;expect(net.listeners.size).toBe(0);
});
test("abort removes tile listener and late read cannot settle replacement",async()=>{
 const service=new MapDataService();const controller=new AbortController();const old=service.fetchTiles(33,controller.signal);const cancelled=expect(old).rejects.toMatchObject({name:"AbortError"});await Promise.resolve();await Promise.resolve();
 const first=net.send.mock.calls[0][0];controller.abort();await cancelled;
 const current=service.fetchTiles(34);await Promise.resolve();await Promise.resolve();const second=net.send.mock.calls[1][0];
 receive(reply(first.requestId,33));expect(net.listeners.size).toBe(1);receive(reply(second.requestId,34));await current;expect(net.listeners.size).toBe(0);
});
test("send failure cleans up immediately rather than retaining a tile timeout",async()=>{
 net.send.mockRejectedValueOnce(new Error("transport failed"));await expect(new MapDataService().fetchTiles(33)).rejects.toThrow("transport failed");expect(net.listeners.size).toBe(0);
});
test("overworld keeps its 30-second timeout through the shared settlement primitive",async()=>{
 vi.useFakeTimers();const run=new MapDataService().fetchTiles(9999);const timed=expect(run).rejects.toThrow("Timeout");await Promise.resolve();await Promise.resolve();
 await vi.advanceTimersByTimeAsync(29999);expect(net.listeners.size).toBe(1);
 await vi.advanceTimersByTimeAsync(1);await timed;expect(net.listeners.size).toBe(0);
});

test("committed update invalidates map snapshots and retries a read overtaken by the stream",async()=>{
 const service=new MapDataService();service.setSnapshot(33,{tiles:[]} as never);
 const run=service.fetchTiles(33);await Promise.resolve();await Promise.resolve();const first=net.send.mock.calls[0][0];
 service.recordCommittedTileUpdate();expect(service.getSnapshot(33)).toBeUndefined();
 receive(reply(first.requestId,33));await Promise.resolve();await Promise.resolve();
 expect(net.send).toHaveBeenCalledTimes(2);const second=net.send.mock.calls[1][0];
 expect(second.requestId).not.toBe(first.requestId);receive(reply(second.requestId,33));await run;expect(net.listeners.size).toBe(0);
});
