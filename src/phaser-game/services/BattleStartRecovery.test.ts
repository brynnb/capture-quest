import {afterEach,beforeEach,expect,test,vi} from "vitest";
const mock=vi.hoisted(()=>({read:vi.fn(),retire:new Set<()=>void>(),generation:0}));
vi.mock("@/net",()=>({WorldSocket:{get sessionGeneration(){return mock.generation},subscribeSessionRetirement:(cb:()=>void)=>{mock.retire.add(cb);return()=>mock.retire.delete(cb)}},OpCodes:{}}));
vi.mock("@/net/index",()=>({WorldSocket:{get sessionGeneration(){return mock.generation},subscribeSessionRetirement:(cb:()=>void)=>{mock.retire.add(cb);return()=>mock.retire.delete(cb)}}}));
vi.mock("./GameplayRecoveryService",()=>({readCurrentGameplayState:mock.read,applyGameplaySnapshot:vi.fn(),applyGameplayResourceSnapshot:vi.fn()}));
vi.mock("@/services/audio/AudioManager",()=>({default:{playSFX:vi.fn(),playMusic:vi.fn()}}));
import AudioManager from "@/services/audio/AudioManager";
import {bindBattleScene,recoverBattlePublication,recoverFieldPublication} from "./BattleCommandService";
import usePokeBattleStore from "@/stores/PokeBattleStore";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import type {GameplayStateResponse,GameplayBattleState} from "@/net/generated/world_api";
const pokemon={id:1,name:"PIKACHU",level:5,hp:20,maxHp:20,moves:[]} as unknown as GameplayBattleState["playerPokemon"];
const battle=(id="owned",revision=1):GameplayBattleState=>({battleId:id,revision,phase:"action_select",turnNumber:0,playerPokemon:pokemon,enemyPokemon:pokemon,playerParty:[pokemon],playerActive:0,battleType:"trainer",allowedActions:[],guaranteedCatch:false,trainerClass:"YOUNGSTER",trainerName:"Trainer"});
const snapshot=(value:GameplayBattleState|null)=>({battle:value,safari:null}) as GameplayStateResponse;
function deferred(){let resolve!:(v:GameplayStateResponse)=>void;const promise=new Promise<GameplayStateResponse>(r=>resolve=r);return{promise,resolve};}
let cleanup:()=>void;
beforeEach(()=>{mock.generation=0;mock.read.mockReset();usePlayerCharacterStore.setState({characterProfile:{...usePlayerCharacterStore.getState().characterProfile,id:42}});useGameScreenStore.setState({currentScreen:"game"});usePokeBattleStore.getState().retireBattle();cleanup=bindBattleScene(vi.fn(async()=>{}));});
afterEach(()=>{cleanup();mock.retire.clear();vi.restoreAllMocks();});
test("a historical hint cannot supply battle state or retire a newer queue",async()=>{
 mock.read.mockResolvedValue(snapshot(null));await recoverBattlePublication({success:true,battleId:"old",revision:1,playerPokemon:pokemon});expect(usePokeBattleStore.getState().isInBattle).toBe(false);
 mock.read.mockResolvedValue(snapshot(battle("current",3)));await recoverBattlePublication({success:true,battleId:"old",revision:1,events:[{type:"message",message:"Old",targetHp:0,targetMaxHp:0}]});expect(usePokeBattleStore.getState()).toMatchObject({battleId:"current",revision:3,eventQueue:[]});
 const generation=usePokeBattleStore.getState().presentationGeneration;await recoverBattlePublication({success:true,battleId:"current",revision:3});expect(usePokeBattleStore.getState().presentationGeneration).toBe(generation);
});
test("matching intro events survive the authoritative initial read",async()=>{
 const events=[{type:"message",message:"Trainer intro",targetHp:0,targetMaxHp:0}];mock.read.mockResolvedValue(snapshot(battle()));await recoverBattlePublication({success:true,battleId:"owned",revision:1,events});expect(usePokeBattleStore.getState().eventQueue).toEqual(events);
});
test("scene retirement and character replacement fence held reads",async()=>{
 for(const retire of [()=>cleanup(),()=>usePlayerCharacterStore.setState({characterProfile:{...usePlayerCharacterStore.getState().characterProfile,id:43}}),()=>useGameScreenStore.setState({currentScreen:"characterSelect"}),()=>mock.retire.forEach(cb=>cb())]){
 cleanup=bindBattleScene(vi.fn(async()=>{}));usePlayerCharacterStore.setState({characterProfile:{...usePlayerCharacterStore.getState().characterProfile,id:42}});useGameScreenStore.setState({currentScreen:"game"});
 const held=deferred();mock.read.mockReturnValueOnce(held.promise);const run=recoverBattlePublication({success:true});retire();held.resolve(snapshot(battle()));await run;expect(usePokeBattleStore.getState().isInBattle).toBe(false);
 }
});
test("a newer presentation or a pending command cannot be overwritten",async()=>{
 const held=deferred();mock.read.mockReturnValueOnce(held.promise);const run=recoverBattlePublication({success:true});usePokeBattleStore.getState().startBattle({...battle("new",5),events:[]});held.resolve(snapshot(battle("old",1)));await run;expect(usePokeBattleStore.getState().battleId).toBe("new");
 usePokeBattleStore.setState({battleCommandPending:true});const before=mock.read.mock.calls.length;await recoverBattlePublication({success:true});expect(mock.read).toHaveBeenCalledTimes(before);
});
test("failure permits a fresh hint without resending a start command",async()=>{
 mock.read.mockRejectedValueOnce(new Error("timeout"));await recoverBattlePublication({success:true});mock.read.mockResolvedValueOnce(snapshot(battle()));await recoverBattlePublication({success:true});expect(usePokeBattleStore.getState().battleId).toBe("owned");
});

test("Safari publication uses current identity and leaves a duplicate queue intact",async()=>{
 const value={...snapshot(null),safari:{visitId:"visit",visitRevision:1,active:true,battleId:"safari",revision:2,ballsLeft:25,stepsLeft:400,pokemon:{id:25,name:"PIKACHU",level:5,hp:20,maxHp:20},playerParty:[pokemon]}} as GameplayStateResponse;
 mock.read.mockResolvedValue(value);await recoverBattlePublication({battleId:"old",revision:1},"safari-start");expect(usePokeBattleStore.getState()).toMatchObject({battleId:"safari",revision:2,isSafari:true});
 const generation=usePokeBattleStore.getState().presentationGeneration;await recoverBattlePublication({},"safari-start");expect(usePokeBattleStore.getState().presentationGeneration).toBe(generation);
});
test("standalone end projects only the owned pose and is fenced by movement generation",async()=>{
 cleanup();let epoch=1;const project=vi.fn(async()=>{});cleanup=bindBattleScene(project,()=>epoch);
 const position={success:true,requestId:"owned",mapId:50,x:7,y:8,direction:"UP",serverMovementPending:false};mock.read.mockResolvedValue({...snapshot(null),position});
 await recoverBattlePublication({blackoutMapId:999,blackoutX:99,blackoutY:99},"standalone-end");expect(project).toHaveBeenCalledWith(position);
 const held=deferred();mock.read.mockReturnValueOnce(held.promise);const run=recoverBattlePublication({},"standalone-end");epoch++;held.resolve({...snapshot(null),position} as GameplayStateResponse);await run;expect(project).toHaveBeenCalledTimes(1);
});

test("leaving the game retires a live battle without waiting for scene destruction",()=>{
 useGameScreenStore.setState({currentScreen:"game"});usePokeBattleStore.getState().startBattle({...battle("live"),events:[]});
 useGameScreenStore.setState({currentScreen:"characterSelect"});expect(usePokeBattleStore.getState()).toMatchObject({isInBattle:false,battleId:"",phase:"none"});
});

test.each(["ordinary-start","standalone-end"] as const)("a %s hint cannot fail an awaited field position recovery",async(kind)=>{
 const held=deferred(); const position={success:true,requestId:"owned",mapId:50,x:8,y:8,direction:"RIGHT",serverMovementPending:false};
 mock.read.mockReturnValueOnce(held.promise).mockResolvedValue({...snapshot(battle()),position});
 const field=recoverFieldPublication(new AbortController().signal);
 const hint=recoverBattlePublication({success:true,battleId:"owned",revision:1},kind);
 held.resolve({...snapshot(battle()),position} as GameplayStateResponse);
 expect(await field).toBe(true); await hint;
});


test("retired field publication releases a new scene and queued hints stay retired",async()=>{
 const held=deferred();mock.read.mockReturnValueOnce(held.promise);
 const old=recoverFieldPublication(new AbortController().signal);
 const hint=recoverBattlePublication({success:true});
 cleanup(); const project=vi.fn(async()=>{}); cleanup=bindBattleScene(project);
 const position={success:true,requestId:"new",mapId:50,x:10,y:8,direction:"UP",serverMovementPending:false};
 mock.read.mockResolvedValueOnce({...snapshot(null),position});
 expect(await recoverFieldPublication(new AbortController().signal)).toBe(true);
 held.resolve(snapshot(battle("retired")));
 expect(await old).toBe(false);expect(await hint).toBe(false);
 expect(project).toHaveBeenCalledOnce();expect(usePokeBattleStore.getState().isInBattle).toBe(false);
});


test("field recovery rereads a settled movement view before releasing admission",async()=>{
 cleanup();let view=1;const project=vi.fn(async()=>{});cleanup=bindBattleScene(project,()=>view);
 const held=deferred();const oldPosition={success:true,requestId:"old",mapId:50,x:7,y:8,direction:"UP",serverMovementPending:false};
 const currentPosition={...oldPosition,requestId:"current",x:8};
 mock.read.mockReturnValueOnce(held.promise).mockResolvedValueOnce({...snapshot(null),position:currentPosition});
 const run=recoverFieldPublication(new AbortController().signal);
 view++;held.resolve({...snapshot(null),position:oldPosition} as GameplayStateResponse);
 expect(await run).toBe(true);expect(mock.read).toHaveBeenCalledTimes(2);expect(project).toHaveBeenCalledWith(currentPosition);
});


test("field-owned battle preserves one initial cry across a duplicate start hint",async()=>{
 const play=vi.mocked(AudioManager.playSFX);play.mockClear();
 const position={success:true,requestId:"owned",mapId:50,x:8,y:8,direction:"RIGHT",serverMovementPending:false};
 mock.read.mockResolvedValue({...snapshot(battle()),position});
 expect(await recoverFieldPublication(new AbortController().signal)).toBe(true);
 expect(play).toHaveBeenCalledOnce();
 await recoverBattlePublication({success:true,battleId:"owned",revision:1});
 expect(play).toHaveBeenCalledOnce();
});
