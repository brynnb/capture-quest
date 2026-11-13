import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { NetworkBridge } from "./NetworkBridge";
import { WorldSocket } from "./index";
import * as OpCodes from "./generated/opcodes";
import * as PhaserNet from "@/phaser-game/services/PhaserNetworkService";
import { deleteCharacter, readCharacterList } from "@/services/authService";
import { CorrelatedResponseError } from "@/phaser-game/services/CorrelatedRequest";
import useCharacterSelectStore from "@/stores/CharacterSelectStore";
import useGameScreenStore from "@/stores/GameScreenStore";

const character = {id:42,name:"Currenttrainer",factionId:1,class:1,mapId:1,gender:0,lastLogin:1};

beforeEach(() => {
  NetworkBridge.initialize();
  useCharacterSelectStore.getState().reset();
  useCharacterSelectStore.getState().setCharacters([character]);
  useGameScreenStore.getState().setScreen("characterSelect");
  vi.spyOn(PhaserNet,"isConnected").mockReturnValue(true);
});
afterEach(() => vi.restoreAllMocks());

test.each([OpCodes.CharacterListResponse,OpCodes.CharacterDeleteResponse])("character reply %d only reaches account subscribers and survives scene cleanup",async opcode=>{
  const receive=vi.fn();const stop=PhaserNet.onCharacterCommand(opcode,receive);
  try {
    WorldSocket.onJson?.(opcode,{success:true,accountId:7,characters:[]});
    expect(receive).not.toHaveBeenCalled();
    PhaserNet.clearAllHandlers();
    const reply={success:false,requestId:"owned-account-request",error:"Unavailable"};
    WorldSocket.onJson?.(opcode,reply);
    await vi.waitFor(()=>expect(receive).toHaveBeenCalledWith(reply));
    expect(receive).toHaveBeenCalledOnce();
    expect(useCharacterSelectStore.getState().characters).toEqual([character]);
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");
  } finally {stop();}
});

test.each(["title","login","register","characterSelect"] as const)("unowned legacy roster cannot change characters or navigate %s",screen=>{
  useGameScreenStore.getState().setScreen(screen);
  WorldSocket.onJson?.(OpCodes.SendCharInfo,{success:true,requestId:"",accountId:99,characters:[]});
  WorldSocket.onJson?.(OpCodes.SendCharInfo,[]);
  expect(useCharacterSelectStore.getState().characters).toEqual([character]);
  expect(useGameScreenStore.getState().currentScreen).toBe(screen);
});

test("current roster adapter waits for its request ID through actual network routing",async()=>{
  let requestId="";
  const send=vi.spyOn(WorldSocket,"sendStreamJsonMessage").mockImplementation(async(opcode,payload)=>{
    expect(opcode).toBe(OpCodes.CharacterListRequest);
    requestId=(payload as {requestId:string}).requestId;
  });
  const observe=vi.fn();const stop=PhaserNet.onCharacterCommand(OpCodes.CharacterListResponse,observe);
  const controller=new AbortController();
  try {
    let settled=false;
    const pending=readCharacterList(controller.signal).then(reply=>{settled=true;return reply;});
    expect(requestId).not.toBe("");
    const wrong={success:true,requestId:"old-request",accountId:7,characters:[]};
    WorldSocket.onJson?.(OpCodes.CharacterListResponse,wrong);
    await vi.waitFor(()=>expect(observe).toHaveBeenCalledWith(wrong));
    expect(settled).toBe(false);
    PhaserNet.clearAllHandlers();
    const current={success:true,requestId,accountId:7,characters:[character]};
    WorldSocket.onJson?.(OpCodes.CharacterListResponse,current);
    await expect(pending).resolves.toEqual(current);
    expect(send).toHaveBeenCalledOnce();
    expect(useCharacterSelectStore.getState().characters).toEqual([character]);
  } finally {controller.abort();stop();}
});

test("deletion sends a stable ID and resolves only its correlated final roster",async()=>{
  let requestId="";
  const send=vi.spyOn(WorldSocket,"sendStreamJsonMessage").mockImplementation(async(opcode,payload)=>{
    expect(opcode).toBe(OpCodes.DeleteCharacter);
    expect(payload).toEqual({requestId:expect.any(String),characterId:character.id});
    requestId=(payload as {requestId:string}).requestId;
  });
  const controller=new AbortController();
  try {
    const pending=deleteCharacter(character.id,controller.signal);
    expect(useCharacterSelectStore.getState().characters).toEqual([character]);
    const reply={success:true,requestId,accountId:7,characterId:character.id,characters:[]};
    WorldSocket.onJson?.(OpCodes.CharacterDeleteResponse,reply);
    await expect(pending).resolves.toEqual(reply);
    expect(send).toHaveBeenCalledOnce();
    // The owning store decides whether this result still belongs to its screen.
    expect(useCharacterSelectStore.getState().characters).toEqual([character]);
  } finally {controller.abort();}
});

test("explicit rejection is distinguishable from an uncertain deletion without a resend",async()=>{
  let requestId="";
  const send=vi.spyOn(WorldSocket,"sendStreamJsonMessage").mockImplementation(async(_opcode,payload)=>{
    requestId=(payload as {requestId:string}).requestId;
  });
  const pending=deleteCharacter(character.id);
  const rejected=expect(pending).rejects.toBeInstanceOf(CorrelatedResponseError);
  WorldSocket.onJson?.(OpCodes.CharacterDeleteResponse,{success:false,requestId,error:"Character unavailable."});
  await rejected;
  expect(send).toHaveBeenCalledOnce();
  expect(useCharacterSelectStore.getState().characters).toEqual([character]);
});

test("an aborted current read cannot consume a replacement read or apply its late roster",async()=>{
  const requestIds:string[]=[];
  vi.spyOn(WorldSocket,"sendStreamJsonMessage").mockImplementation(async(_opcode,payload)=>{
    requestIds.push((payload as {requestId:string}).requestId);
  });
  const controller=new AbortController();
  const old=readCharacterList(controller.signal);
  const cancelled=expect(old).rejects.toMatchObject({name:"AbortError"});
  controller.abort();await cancelled;
  const current=readCharacterList();
  expect(requestIds).toHaveLength(2);
  expect(requestIds[0]).not.toBe(requestIds[1]);
  WorldSocket.onJson?.(OpCodes.CharacterListResponse,{success:true,requestId:requestIds[0],accountId:99,characters:[]});
  const reply={success:true,requestId:requestIds[1],accountId:7,characters:[character]};
  WorldSocket.onJson?.(OpCodes.CharacterListResponse,reply);
  await expect(current).resolves.toEqual(reply);
  expect(useCharacterSelectStore.getState().characters).toEqual([character]);
});
