import {afterEach,beforeEach,expect,test,vi} from "vitest";
const mock=vi.hoisted(()=>({read:vi.fn(),retire:new Set<()=>void>(),generation:0}));
vi.mock("@/net",()=>({WorldSocket:{get sessionGeneration(){return mock.generation}},OpCodes:{}}));
vi.mock("@/net/index",()=>({WorldSocket:{get sessionGeneration(){return mock.generation},subscribeSessionRetirement:(cb:()=>void)=>{mock.retire.add(cb);return()=>mock.retire.delete(cb)}}}));
vi.mock("./GameplayRecoveryService",()=>({readCurrentGameplayState:mock.read,applyGameplaySnapshot:vi.fn()}));
vi.mock("@/services/audio/AudioManager",()=>({default:{playSFX:vi.fn(),playMusic:vi.fn()}}));
import {bindBattleScene,recoverBattleStartNotice} from "./BattleCommandService";
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
 mock.read.mockResolvedValue(snapshot(null));await recoverBattleStartNotice({success:true,battleId:"old",revision:1,playerPokemon:pokemon});expect(usePokeBattleStore.getState().isInBattle).toBe(false);
 mock.read.mockResolvedValue(snapshot(battle("current",3)));await recoverBattleStartNotice({success:true,battleId:"old",revision:1,events:[{type:"message",message:"Old",targetHp:0,targetMaxHp:0}]});expect(usePokeBattleStore.getState()).toMatchObject({battleId:"current",revision:3,eventQueue:[]});
 const generation=usePokeBattleStore.getState().presentationGeneration;await recoverBattleStartNotice({success:true,battleId:"current",revision:3});expect(usePokeBattleStore.getState().presentationGeneration).toBe(generation);
});
test("matching intro events survive the authoritative initial read",async()=>{
 const events=[{type:"message",message:"Trainer intro",targetHp:0,targetMaxHp:0}];mock.read.mockResolvedValue(snapshot(battle()));await recoverBattleStartNotice({success:true,battleId:"owned",revision:1,events});expect(usePokeBattleStore.getState().eventQueue).toEqual(events);
});
test("scene retirement and character replacement fence held reads",async()=>{
 for(const retire of [()=>cleanup(),()=>usePlayerCharacterStore.setState({characterProfile:{...usePlayerCharacterStore.getState().characterProfile,id:43}}),()=>useGameScreenStore.setState({currentScreen:"characterSelect"}),()=>mock.retire.forEach(cb=>cb())]){
 cleanup=bindBattleScene(vi.fn(async()=>{}));usePlayerCharacterStore.setState({characterProfile:{...usePlayerCharacterStore.getState().characterProfile,id:42}});useGameScreenStore.setState({currentScreen:"game"});
 const held=deferred();mock.read.mockReturnValueOnce(held.promise);const run=recoverBattleStartNotice({success:true});retire();held.resolve(snapshot(battle()));await run;expect(usePokeBattleStore.getState().isInBattle).toBe(false);
 }
});
test("a newer presentation or a pending command cannot be overwritten",async()=>{
 const held=deferred();mock.read.mockReturnValueOnce(held.promise);const run=recoverBattleStartNotice({success:true});usePokeBattleStore.getState().startBattle({...battle("new",5),events:[]});held.resolve(snapshot(battle("old",1)));await run;expect(usePokeBattleStore.getState().battleId).toBe("new");
 usePokeBattleStore.setState({battleCommandPending:true});const before=mock.read.mock.calls.length;await recoverBattleStartNotice({success:true});expect(mock.read).toHaveBeenCalledTimes(before);
});
test("failure permits a fresh hint without resending a start command",async()=>{
 mock.read.mockRejectedValueOnce(new Error("timeout"));await recoverBattleStartNotice({success:true});mock.read.mockResolvedValueOnce(snapshot(battle()));await recoverBattleStartNotice({success:true});expect(usePokeBattleStore.getState().battleId).toBe("owned");
});
