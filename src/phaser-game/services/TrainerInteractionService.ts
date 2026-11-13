import {correlatedRequest,readForCurrentCharacter} from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";
import type {TrainerInteractResponse} from "@/net/generated/protocol";

export function readTrainerInteraction(actorId:number,signal?:AbortSignal):Promise<TrainerInteractResponse>{
 return readForCurrentCharacter(async(characterId,ownedSignal)=>{
 const reply=await correlatedRequest<TrainerInteractResponse>(PhaserNet.onTrainerInteraction,id=>PhaserNet.requestTrainerInteraction(id,actorId),ownedSignal,5000);
 if(reply.characterId!==characterId || reply.trainerActorId!==actorId)throw new Error("Trainer response identity mismatch");
 if(typeof reply.dialogue!=="string" || typeof reply.trainerName!=="string" || typeof reply.trainerClass!=="string" || typeof reply.shouldBattle!=="boolean" || typeof reply.defeated!=="boolean")throw new Error("Invalid trainer response");
 return reply;
 },signal);
}
