import {beforeEach,afterEach,expect,test,vi} from "vitest";
vi.mock("@/net/index",()=>({WorldSocket:{sessionGeneration:0,subscribeSessionRetirement:()=>()=>{}}}));
import {acceptOwnedSafariVisit,acceptSafariVisitNotice,bindSafariVisitView,claimSafariExit,isCurrentSafariExit,shouldRecoverSafariExit} from "./SafariVisitService";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import type {SafariRecoveryState} from "@/net/generated/world_api";
const visit=(id="visit",rev=1):SafariRecoveryState=>({visitId:id,visitRevision:rev,active:true,ballsLeft:30,stepsLeft:500,pokemon:undefined});
let cleanup:()=>void;let receive:ReturnType<typeof vi.fn>;
beforeEach(()=>{useGameScreenStore.setState({currentScreen:"characterSelect"});usePlayerCharacterStore.setState({characterProfile:{...usePlayerCharacterStore.getState().characterProfile,id:42}});useGameScreenStore.setState({currentScreen:"game"});receive=vi.fn();cleanup=bindSafariVisitView(receive);acceptOwnedSafariVisit(visit());receive.mockClear();});
afterEach(()=>cleanup());
test("step updates advance only the known visit and reject older or foreign state",()=>{
 expect(acceptSafariVisitNotice({...visit("visit",2),characterId:42,stepsLeft:499})).toBe("ignore");expect(receive.mock.calls.at(-1)![0].stepsLeft).toBe(499);
 receive.mockClear();acceptSafariVisitNotice({...visit(),characterId:42});acceptSafariVisitNotice({...visit("visit",3),characterId:99});expect(receive).not.toHaveBeenCalled();
 expect(acceptSafariVisitNotice({...visit("next",1),characterId:42})).toBe("refresh");expect(receive).not.toHaveBeenCalled();
});
test("owned terminal narrative is claimed once and retirement clears private visit state",()=>{
 const ended={...visit(),active:false,stepsLeft:0,exitMessage:"Source expiry message"};acceptOwnedSafariVisit(ended);
 expect(shouldRecoverSafariExit({...ended,characterId:42})).toBe(true);
 expect(claimSafariExit(ended)).toBe(true);expect(claimSafariExit(ended)).toBe(false);
 expect(shouldRecoverSafariExit({...ended,characterId:42})).toBe(false);
 useGameScreenStore.setState({currentScreen:"characterSelect"});expect(receive.mock.calls.at(-1)![0]).toBeNull();
 useGameScreenStore.setState({currentScreen:"game"});expect(claimSafariExit(ended)).toBe(true);
});

test("a new visit retires the old exit callback even with the same character",()=>{
 const ended={...visit(),active:false,stepsLeft:0,exitMessage:"Expiry"};acceptOwnedSafariVisit(ended);expect(isCurrentSafariExit(ended)).toBe(true);
 acceptOwnedSafariVisit(visit("next"));expect(isCurrentSafariExit(ended)).toBe(false);
});
