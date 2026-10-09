import {WorldSocket} from "@/net/index";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";
import useGameScreenStore from "@/stores/GameScreenStore";
import {SafariZoneMaxBalls,SafariZoneMaxSteps,type SafariRecoveryState} from "@/net/generated/world_api";
import type {SafariVisitState} from "@/net/generated/protocol";

type View=Pick<SafariRecoveryState,"visitId"|"visitRevision"|"active"|"ballsLeft"|"stepsLeft">;
let known:View|null=null;
let owner:number|undefined;
let generation=-1;
const listeners=new Set<(view:View|null)=>void>();
let shownExit:string|null=null;
let watching=false;
function retire(){known=null;shownExit=null;publish();}
function watch(){if(watching)return;watching=true;
 WorldSocket.subscribeSessionRetirement(retire);
 usePlayerCharacterStore.subscribe((state,before)=>{if(state.characterProfile.id!==before.characterProfile.id)retire();});
 useGameScreenStore.subscribe(state=>{if(state.currentScreen!=="game")retire();});
}
function scope():boolean{
 watch();
 const id=usePlayerCharacterStore.getState().characterProfile?.id;
 if(owner!==id || generation!==WorldSocket.sessionGeneration || useGameScreenStore.getState().currentScreen!=="game"){
 owner=id;generation=WorldSocket.sessionGeneration;known=null;shownExit=null;
 }
 return !!id && useGameScreenStore.getState().currentScreen==="game";
}
function valid(view:View):boolean{return typeof view.visitId==="string" && !!view.visitId && Number.isSafeInteger(view.visitRevision) && view.visitRevision>0 && typeof view.active==="boolean" && Number.isSafeInteger(view.ballsLeft) && view.ballsLeft>=0 && view.ballsLeft<=SafariZoneMaxBalls && Number.isSafeInteger(view.stepsLeft) && view.stepsLeft>=0 && view.stepsLeft<=SafariZoneMaxSteps;}
function publish(){listeners.forEach(receive=>receive(known));}
export function acceptOwnedSafariVisit(view:SafariRecoveryState|null|undefined):void{
 if(!scope())return;
 if(view && !valid(view))throw new Error("Invalid owned Safari visit identity/counters");
 if(view && known?.visitId===view.visitId && view.visitRevision<known.visitRevision)return;
 known=view??null;publish();
}
export function acceptSafariVisitNotice(data:unknown):"ignore"|"refresh"{
 if(!scope())return "ignore";
 const view=data as SafariVisitState & {refresh?:boolean};
 if(view?.characterId===owner && view.refresh===true)return "refresh";
 if(!view || view.characterId!==owner || !valid(view))return "ignore";
 if(!known || view.visitId!==known.visitId)return "refresh";
 if(view.visitRevision<=known.visitRevision)return "ignore";
 known=view;publish();return "ignore";
}
export function bindSafariVisitView(receive:(view:View|null)=>void):()=>void{
 scope();listeners.add(receive);receive(known);
 return()=>{listeners.delete(receive);if(listeners.size===0){known=null;}};
}
export function claimSafariExit(view:SafariRecoveryState):boolean{
 if(!scope() || !view.visitId || view.active || view.pokemon || !view.exitMessage || shownExit===view.visitId)return false;
 shownExit=view.visitId;return true;
}

export function shouldRecoverSafariExit(data:unknown):boolean{
 if(!scope())return false;
 const view=data as SafariVisitState;
 return !!view && view.characterId===owner && valid(view) && !view.active && shownExit!==view.visitId && (!known || (view.visitId===known.visitId && view.visitRevision>=known.visitRevision));
}

export function isCurrentSafariExit(view:SafariRecoveryState):boolean{
 return scope() && known?.visitId===view.visitId && known.visitRevision===view.visitRevision && !known.active;
}
