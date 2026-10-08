import {afterEach,expect,test,vi} from "vitest";
const network=vi.hoisted(()=>({listeners:new Set<(data:unknown)=>void>(),retire:new Set<()=>void>(),send:vi.fn()}));
vi.mock("@/net",()=>({WorldSocket:{sessionGeneration:0,subscribeSessionRetirement:(receive:()=>void)=>{network.retire.add(receive);return()=>network.retire.delete(receive)}},OpCodes:{}}));
vi.mock("@/phaser-game/services/PhaserNetworkService",()=>({isConnected:()=>true,onNameValidation:(receive:(data:unknown)=>void)=>{network.listeners.add(receive);return()=>network.listeners.delete(receive)},requestNameValidation:network.send}));
import {validateName} from "./authService";
const receive=(data:unknown)=>network.listeners.forEach(listener=>listener(data));
afterEach(()=>{network.send.mockReset();network.listeners.clear();network.retire.clear();vi.useRealTimers();});
test("concurrent names resolve only their own tagged replies",async()=>{
 const first=validateName("Alpha");const second=validateName("Bravo");
 const a=network.send.mock.calls[0][0],b=network.send.mock.calls[1][0];
 receive({success:true,requestId:b,name:"Bravo",valid:true,available:false,errorMessage:"taken"});
 receive({success:true,requestId:a,name:"Alpha",valid:true,available:true,errorMessage:""});
 expect(await first).toMatchObject({available:true});expect(await second).toMatchObject({available:false});expect(network.listeners.size).toBe(0);
});
test("name edit/unmount cancellation discards an old reply",async()=>{
 const controller=new AbortController();const run=validateName("Alpha",controller.signal);const id=network.send.mock.calls[0][0];controller.abort();
 receive({success:true,requestId:id,name:"Alpha",valid:true,available:true,errorMessage:""});expect(await run).toBeNull();expect(network.listeners.size).toBe(0);
});
test("transport retirement cancels validation and releases its listener",async()=>{
 const run=validateName("Alpha");network.retire.forEach(stop=>stop());expect(await run).toBeNull();expect(network.listeners.size).toBe(0);
});
