import {beforeEach,afterEach,expect,test,vi} from "vitest";
const network=vi.hoisted(()=>({listeners:new Set<(data:any)=>void>(),retire:new Set<()=>void>(),send:vi.fn(),generation:0}));
vi.mock("@/net/index",()=>({WorldSocket:{get sessionGeneration(){return network.generation},subscribeSessionRetirement:(cb:()=>void)=>{network.retire.add(cb);return()=>network.retire.delete(cb)}}}));
vi.mock("./PhaserNetworkService",()=>({isConnected:()=>true,onDialogueRead:(cb:(data:any)=>void)=>{network.listeners.add(cb);return()=>network.listeners.delete(cb)},requestDialogueRead:network.send}));
import {fetchDialogueWithBranching} from "./DialogueService";
import useGameScreenStore from "@/stores/GameScreenStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
beforeEach(()=>{usePlayerCharacterStore.setState({characterProfile:{...usePlayerCharacterStore.getState().characterProfile,id:42}});useGameScreenStore.setState({currentScreen:"game"});});
const receive=(requestId:string,textConstant:string,dialogue:string,extra={})=>network.listeners.forEach(cb=>cb({success:true,requestId,textConstant,characterId:usePlayerCharacterStore.getState().characterProfile.id,dialogueEntries:[{dialogue,label:"Label",sourceFile:"fixture",isTrainer:0,mapName:null}],hasBranching:false,branchingPrompt:null,...extra}));
afterEach(()=>{network.listeners.clear();network.retire.clear();network.send.mockReset();vi.useRealTimers();});
test("out-of-order and untagged replies cannot satisfy another dialogue",async()=>{
 const first=fetchDialogueWithBranching("FIRST"), second=fetchDialogueWithBranching("SECOND");
 const a=network.send.mock.calls[0][0],b=network.send.mock.calls[1][0];
 receive("","FIRST","Wrong");receive(b,"SECOND","Second");receive(a,"FIRST","First");
 expect((await first).lines).toEqual(["First"]);expect((await second).lines).toEqual(["Second"]);expect(network.listeners.size).toBe(0);
});
test("timeout and retry ignore the old response",async()=>{
 vi.useFakeTimers();const first=fetchDialogueWithBranching("TEXT");const failure=expect(first).rejects.toThrow("Timeout");const old=network.send.mock.calls[0][0];
 await vi.advanceTimersByTimeAsync(5000);await failure;
 const retry=fetchDialogueWithBranching("TEXT");receive(old,"TEXT","Old");receive(network.send.mock.calls[1][0],"TEXT","Fresh");expect((await retry).lines).toEqual(["Fresh"]);
});
test("cancellation and retirement release owned subscriptions",async()=>{
 const controller=new AbortController();const first=fetchDialogueWithBranching("TEXT",controller.signal);const failure=expect(first).rejects.toMatchObject({name:"AbortError"});controller.abort();await failure;
 const second=fetchDialogueWithBranching("TEXT");const retired=expect(second).rejects.toMatchObject({name:"AbortError"});network.retire.forEach(cb=>cb());await retired;expect(network.listeners.size).toBe(0);expect(network.retire.size).toBe(0);
});
test("wrong identity and malformed response reject instead of supplying fallback text",async()=>{
 const first=fetchDialogueWithBranching("TEXT");const failure=expect(first).rejects.toThrow("identity");receive(network.send.mock.calls[0][0],"OTHER","Wrong");await failure;
 const second=fetchDialogueWithBranching("TEXT");const invalid=expect(second).rejects.toThrow("Invalid");receive(network.send.mock.calls[1][0],"TEXT","Wrong",{hasBranching:true,branchingPrompt:null});await invalid;
});
test("character replacement cancels pending reads",async()=>{
 const before=usePlayerCharacterStore.getState().characterProfile;
 const first=fetchDialogueWithBranching("TEXT");const failure=expect(first).rejects.toMatchObject({name:"AbortError"});
 usePlayerCharacterStore.setState({characterProfile:{...before,id:(before.id??0)+1}});await failure;
 usePlayerCharacterStore.setState({characterProfile:before});expect(network.listeners.size).toBe(0);
});
test("send failure releases the listener and permits retry",async()=>{
 network.send.mockRejectedValueOnce(new Error("write failed"));await expect(fetchDialogueWithBranching("TEXT")).rejects.toThrow("write failed");expect(network.listeners.size).toBe(0);
 const next=fetchDialogueWithBranching("TEXT");receive(network.send.mock.calls[1][0],"TEXT","Retry");expect((await next).lines).toEqual(["Retry"]);
});

test("leaving the game screen cancels pending dialogue",async()=>{
 const run=fetchDialogueWithBranching("TEXT");const failure=expect(run).rejects.toMatchObject({name:"AbortError"});useGameScreenStore.setState({currentScreen:"characterSelect"});await failure;expect(network.listeners.size).toBe(0);
});
