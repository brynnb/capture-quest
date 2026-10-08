import {beforeEach,afterEach,describe,expect,test,vi} from "vitest";
const network=vi.hoisted(()=>({listeners:new Set<(data:any)=>void>(),retire:new Set<()=>void>(),send:vi.fn(),generation:0}));
vi.mock("@/net/index",()=>({WorldSocket:{get sessionGeneration(){return network.generation},subscribeSessionRetirement:(cb:()=>void)=>{network.retire.add(cb);return()=>network.retire.delete(cb)}}}));
vi.mock("./PhaserNetworkService",()=>({isConnected:()=>true,onDialogueRead:(cb:(data:any)=>void)=>{network.listeners.add(cb);return()=>network.listeners.delete(cb)},requestDialogueRead:network.send,onTrainerInteraction:(cb:(data:any)=>void)=>{network.listeners.add(cb);return()=>network.listeners.delete(cb)},requestTrainerInteraction:network.send}));
import {fetchDialogueWithBranching} from "./DialogueService";
import {readTrainerInteraction} from "./TrainerInteractionService";
import useGameScreenStore from "@/stores/GameScreenStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
beforeEach(()=>{network.generation=0;usePlayerCharacterStore.setState({characterProfile:{...usePlayerCharacterStore.getState().characterProfile,id:42}});useGameScreenStore.setState({currentScreen:"game"});});
afterEach(()=>{network.listeners.clear();network.retire.clear();network.send.mockReset();vi.useRealTimers();});
for(const kind of ["dialogue","trainer"] as const){
 describe(`${kind} character read`,()=>{
 const read=async(source:number,signal?:AbortSignal)=>kind==="dialogue" ? (await fetchDialogueWithBranching(String(source),signal)).lines.join(" ") : (await readTrainerInteraction(source,signal)).dialogue;
 const receive=(requestId:string,source:number,text:string,extra={})=>network.listeners.forEach(cb=>cb({success:true,requestId,characterId:42,...(kind==="dialogue" ? {textConstant:String(source),dialogueEntries:[{dialogue:text,label:"Label",sourceFile:"fixture",isTrainer:0,mapName:null}],hasBranching:false,branchingPrompt:null} : {trainerActorId:source,trainerName:"Trainer",trainerClass:"YOUNGSTER",dialogue:text,shouldBattle:true,defeated:false}),...extra}));
 test("out-of-order and untagged replies cannot satisfy another source",async()=>{
 const first=read(1),second=read(2);const a=network.send.mock.calls[0][0],b=network.send.mock.calls[1][0];
 receive("",1,"Wrong");receive(b,2,"Second");receive(a,1,"First");expect(await first).toBe("First");expect(await second).toBe("Second");expect(network.listeners.size).toBe(0);
 });
 test("timeout and retry ignore the old response",async()=>{
 vi.useFakeTimers();const first=read(1);const failure=expect(first).rejects.toThrow("Timeout");const old=network.send.mock.calls[0][0];await vi.advanceTimersByTimeAsync(5000);await failure;
 const retry=read(1);receive(old,1,"Old");receive(network.send.mock.calls[1][0],1,"Fresh");expect(await retry).toBe("Fresh");
 });
 test("cancellation and retirement release owned subscriptions",async()=>{
 const controller=new AbortController();const first=read(1,controller.signal);const failed=expect(first).rejects.toMatchObject({name:"AbortError"});controller.abort();await failed;
 const second=read(1);const retired=expect(second).rejects.toMatchObject({name:"AbortError"});network.retire.forEach(cb=>cb());await retired;expect(network.listeners.size).toBe(0);expect(network.retire.size).toBe(0);
 });
 test("wrong source and malformed reply reject instead of supplying fallback text",async()=>{
 const first=read(1);const failed=expect(first).rejects.toThrow("identity");receive(network.send.mock.calls[0][0],2,"Wrong");await failed;
 const next=read(1);const invalid=expect(next).rejects.toThrow("Invalid");receive(network.send.mock.calls[1][0],1,"Wrong",kind==="dialogue" ? {hasBranching:true,branchingPrompt:null} : {shouldBattle:"yes"});await invalid;
 });
 test("character replacement cancels pending reads",async()=>{
 const first=read(1);const failed=expect(first).rejects.toMatchObject({name:"AbortError"});const before=usePlayerCharacterStore.getState().characterProfile;
 usePlayerCharacterStore.setState({characterProfile:{...before,id:43}});await failed;expect(network.listeners.size).toBe(0);
 });
 test("screen replacement cancels pending reads",async()=>{
 const first=read(1);const failed=expect(first).rejects.toMatchObject({name:"AbortError"});useGameScreenStore.setState({currentScreen:"characterSelect"});await failed;expect(network.listeners.size).toBe(0);
 });
 test("write failure releases the listener and permits retry",async()=>{
 network.send.mockRejectedValueOnce(new Error("write failed"));await expect(read(1)).rejects.toThrow("write failed");expect(network.listeners.size).toBe(0);
 const next=read(1);receive(network.send.mock.calls[1][0],1,"Retry");expect(await next).toBe("Retry");
 });
 test("another character and server rejection fail closed",async()=>{
 const first=read(1);const failed=expect(first).rejects.toThrow("identity");receive(network.send.mock.calls[0][0],1,"Wrong",{characterId:99});await failed;
 const second=read(1);const rejected=expect(second).rejects.toThrow("unavailable");network.listeners.forEach(cb=>cb({requestId:network.send.mock.calls[1][0],success:false,error:"unavailable"}));await rejected;
 });
 });
}
