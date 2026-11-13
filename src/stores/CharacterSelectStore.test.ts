import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  enter: vi.fn(),
  connect: vi.fn(), login: vi.fn(), guest: vi.fn(), register: vi.fn(), roster: vi.fn(), create: vi.fn(), delete: vi.fn(),
  cancelCutscene: vi.fn(),
  addMessage: vi.fn(),
  clearMessages: vi.fn(),
  resetPanels: vi.fn(),
  listeners: new Set<() => void>(),
  socket: { isConnected: true, sessionGeneration: 1, close: vi.fn(), subscribeSessionRetirement: vi.fn() },
}));
vi.mock("@/net", () => ({ WorldSocket: mocks.socket }));
vi.mock("@/services/authService", () => ({
  enterWorld: mocks.enter, deleteCharacter: mocks.delete, createCharacter: mocks.create,
  connectForAuthentication: mocks.connect, login: mocks.login, loginGuest: mocks.guest,
  registerAccount: mocks.register, readCharacterList: mocks.roster,
}));
vi.mock("@/phaser-game/services/CutsceneService", () => ({ cancelActiveCutscene: mocks.cancelCutscene }));
vi.mock("./ChatStore", () => ({
  default: { getState: () => ({ addMessage: mocks.addMessage, clearMessages: mocks.clearMessages }) },
  MessageType: { SYSTEM: "system", ZONE_ENTRY: "zone" },
}));
vi.mock("./GameStatusStore", async () => {
  const { create } = await import("zustand");
  return { default: create<{ isMapLoading: boolean; currentMap: number | null; setIsMapLoading: (value: boolean) => void; getMapNameById: (id: number) => string; resetPanelStates: () => void }>(set => ({
    isMapLoading: false, currentMap: 1,
    setIsMapLoading: value => set({ isMapLoading: value }),
    getMapNameById: id => `Map ${id}`, resetPanelStates: mocks.resetPanels,
  })) };
});

import useCharacterSelectStore, { type CharacterSelectEntry } from "./CharacterSelectStore";
import useGameScreenStore from "./GameScreenStore";
import useGameStatusStore from "./GameStatusStore";
import { CorrelatedResponseError } from "@/phaser-game/services/CorrelatedRequest";

const character: CharacterSelectEntry = { id: 42, name: "Entrytester", factionId: 1, class: 1, mapId: 1, gender: 0, lastLogin: 2 };
const other: CharacterSelectEntry = { ...character, id: 43, name: "Otherentry", lastLogin: 1 };
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function retireTransport() {
  mocks.socket.isConnected = false;
  mocks.socket.sessionGeneration++;
  for (const receive of [...mocks.listeners]) receive();
}

beforeEach(() => {
  useCharacterSelectStore.getState().reset();
  mocks.listeners.clear();
  vi.clearAllMocks();
  for (const fn of [mocks.connect,mocks.login,mocks.guest,mocks.register,mocks.roster,mocks.create,mocks.delete]) fn.mockReset();
  mocks.socket.isConnected = true;
  mocks.socket.sessionGeneration = 1;
  mocks.socket.close.mockImplementation(retireTransport);
  mocks.socket.subscribeSessionRetirement.mockImplementation((receive: () => void) => {
    mocks.listeners.add(receive);
    return () => mocks.listeners.delete(receive);
  });
  useGameScreenStore.getState().setScreen("characterSelect");
  useGameStatusStore.setState({ isMapLoading: false, currentMap: 1 });
  useCharacterSelectStore.getState().setCharacters([character, other]);
  mocks.connect.mockImplementation(async () => { retireTransport(); mocks.socket.isConnected = true; return true; });
  const authenticated = async () => { mocks.socket.sessionGeneration++; for (const receive of [...mocks.listeners]) receive(); return {status:1}; };
  mocks.login.mockImplementation(authenticated); mocks.guest.mockImplementation(authenticated); mocks.register.mockImplementation(authenticated);
  mocks.roster.mockResolvedValue(roster());
});

describe("owned character entry", () => {
  it("coalesces duplicate entry and switches screens only after confirmed success", async () => {
    const reply = deferred<boolean>(); mocks.enter.mockReturnValue(reply.promise);
    const first = useCharacterSelectStore.getState().enterWorld(character.name);
    const second = useCharacterSelectStore.getState().enterWorld(character.name);
    expect(second).toBe(first);
    expect(useCharacterSelectStore.getState().isEnteringWorld).toBe(true);
    expect(useGameStatusStore.getState().isMapLoading).toBe(true);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(1));
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");
    reply.resolve(true);
    await expect(first).resolves.toBe(true);
    expect(useGameScreenStore.getState().currentScreen).toBe("game");
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(useCharacterSelectStore.getState().isEnteringWorld).toBe(false);
    expect(mocks.socket.close).not.toHaveBeenCalled();
    expect(mocks.clearMessages).toHaveBeenCalledTimes(1);
    expect(mocks.listeners.size).toBe(0);
  });

  it("settles an explicit rejection, clears loading and permits an explicit retry", async () => {
    mocks.enter.mockResolvedValueOnce(false);
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({ isEnteringWorld: false, entryError: "Could not enter world. Please try again." });
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");
    expect(mocks.clearMessages).not.toHaveBeenCalled();
    mocks.enter.mockResolvedValueOnce(true);
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(true);
    expect(useCharacterSelectStore.getState().entryError).toBeNull();
  });

  it("retires an unacknowledged entry before a retry can reuse its transport", async () => {
    mocks.enter.mockRejectedValueOnce(new Error("Request timed out"));
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(false);
    expect(mocks.socket.close).toHaveBeenCalledOnce();
    expect(mocks.socket.close).toHaveBeenCalledWith(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({ isEnteringWorld: false, entryError: null });
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(mocks.clearMessages).not.toHaveBeenCalled();
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(false);
    expect(mocks.enter).toHaveBeenCalledOnce();
    mocks.socket.isConnected = true;
    mocks.enter.mockResolvedValueOnce(true);
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(true);
  });

  it("does not dispatch when the selection is abandoned before asynchronous preparation finishes", async () => {
    const result = useCharacterSelectStore.getState().enterWorld(character.name);
    useGameScreenStore.getState().setScreen("title");
    await expect(result).resolves.toBe(false);
    expect(mocks.enter).not.toHaveBeenCalled();
    expect(mocks.socket.close).not.toHaveBeenCalled();
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(useGameScreenStore.getState().currentScreen).toBe("title");
  });

  it.each(["screen", "selection", "reset"] as const)("retires a sent entry on %s replacement without publishing its late success", async cause => {
    const reply = deferred<boolean>(); mocks.enter.mockReturnValue(reply.promise);
    const result = useCharacterSelectStore.getState().enterWorld(character.name);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(1));
    if (cause === "screen") useGameScreenStore.getState().setScreen("register");
    else if (cause === "selection") useCharacterSelectStore.getState().setSelectedCharacter(other);
    else useCharacterSelectStore.getState().reset();
    expect(mocks.socket.close).toHaveBeenCalledOnce();
    expect(mocks.socket.close).toHaveBeenCalledWith(false);
    reply.resolve(true);
    await expect(result).resolves.toBe(false);
    expect(useGameScreenStore.getState().currentScreen).not.toBe("game");
    expect(mocks.clearMessages).not.toHaveBeenCalled();
    expect(useCharacterSelectStore.getState().entryError).toBeNull();
    expect(useGameStatusStore.getState().isMapLoading).toBe(false);
    expect(mocks.listeners.size).toBe(0);
  });

  it("an old failure cannot clear a replacement entry or close its new transport", async () => {
    const old = deferred<boolean>(); const next = deferred<boolean>();
    mocks.enter.mockReturnValueOnce(old.promise).mockReturnValueOnce(next.promise);
    const first = useCharacterSelectStore.getState().enterWorld(character.name);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(1));
    retireTransport();
    mocks.socket.isConnected = true;
    const second = useCharacterSelectStore.getState().enterWorld(character.name);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(2));
    old.reject(new Error("Old connection failed"));
    await expect(first).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({ isEnteringWorld: true, entryError: null });
    expect(useGameStatusStore.getState().isMapLoading).toBe(true);
    expect(mocks.socket.close).not.toHaveBeenCalled();
    next.resolve(true); await expect(second).resolves.toBe(true);
    expect(useGameScreenStore.getState().currentScreen).toBe("game");
  });

  it("rejects another character while the selected entry is pending without a second command", async () => {
    const reply = deferred<boolean>(); mocks.enter.mockReturnValue(reply.promise);
    const first = useCharacterSelectStore.getState().enterWorld(character.name);
    await expect(useCharacterSelectStore.getState().enterWorld(other.name)).resolves.toBe(false);
    await vi.waitFor(() => expect(mocks.enter).toHaveBeenCalledTimes(1));
    reply.resolve(true); await first;
  });
});


function roster(characters: CharacterSelectEntry[] = [character, other], accountId = 7) {
  return {success: true as const, requestId: "roster", accountId, characters};
}
function ownAccount() {
  useCharacterSelectStore.setState({accountId: 7, accountGeneration: mocks.socket.sessionGeneration, rosterReady: true});
}
const createPayload = {name:"Newtrainer",rivalName:"Blue",factionId:1,charClass:1,homeTownMapId:1,gender:0,tutorial:0};

describe("shared account admission and owned roster", () => {
  it("waits for the owned current roster after JWT success before navigating",async()=>{
    useGameScreenStore.getState().setScreen("title");
    const reply=deferred<ReturnType<typeof roster>>();mocks.roster.mockReturnValueOnce(reply.promise);
    const run=useCharacterSelectStore.getState().authenticate({kind:"guest",token:"guest"});
    await vi.waitFor(()=>expect(mocks.roster).toHaveBeenCalledOnce());
    expect(useGameScreenStore.getState().currentScreen).toBe("title");
    expect(useCharacterSelectStore.getState()).toMatchObject({accountOperation:"authenticate",isLoading:true,characters:[],rosterReady:false});
    reply.resolve(roster());await expect(run).resolves.toBe(true);
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");
    expect(useCharacterSelectStore.getState()).toMatchObject({accountId:7,accountGeneration:mocks.socket.sessionGeneration,rosterReady:true,accountOperation:null,isLoading:false});
    expect(mocks.connect).toHaveBeenCalledOnce();expect(mocks.guest).toHaveBeenCalledOnce();expect(mocks.listeners.size).toBe(0);
  });

  it("settles missing roster after JWT success and Retry only reads current characters",async()=>{
    useGameScreenStore.getState().setScreen("login");mocks.roster.mockRejectedValueOnce(new Error("lost list"));
    await expect(useCharacterSelectStore.getState().authenticate({kind:"login",email:"trainer@example.test",password:"secret"})).resolves.toBe(true);
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");
    expect(useCharacterSelectStore.getState()).toMatchObject({isLoading:false,accountOperation:null,rosterReady:false,rosterError:"Characters could not be loaded. Please try again."});
    await expect(useCharacterSelectStore.getState().refreshCharacters()).resolves.toBe(true);
    expect(mocks.login).toHaveBeenCalledOnce();expect(mocks.connect).toHaveBeenCalledOnce();expect(mocks.roster).toHaveBeenCalledTimes(2);
    expect(useCharacterSelectStore.getState()).toMatchObject({rosterReady:true,rosterError:null});
  });

  it("keeps authentication rejection on its form without publishing a roster",async()=>{
    useGameScreenStore.getState().setScreen("register");mocks.register.mockRejectedValueOnce(new Error("Email is already registered."));
    await expect(useCharacterSelectStore.getState().authenticate({kind:"register",email:"trainer@example.test",password:"secret"})).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({accountError:"Email is already registered.",accountOperation:null,isLoading:false,rosterReady:false});
    expect(useGameScreenStore.getState().currentScreen).toBe("register");expect(mocks.roster).not.toHaveBeenCalled();
  });

  it("admits one authentication even while connection establishment is pending",async()=>{
    useGameScreenStore.getState().setScreen("title");const connected=deferred<boolean>();mocks.connect.mockReturnValueOnce(connected.promise);
    const first=useCharacterSelectStore.getState().authenticate({kind:"guest",token:"guest"});
    await expect(useCharacterSelectStore.getState().authenticate({kind:"login",email:"other@example.test",password:"secret"})).resolves.toBe(false);
    await vi.waitFor(()=>expect(mocks.connect).toHaveBeenCalledOnce());connected.resolve(true);await first;
    expect(mocks.guest).toHaveBeenCalledOnce();expect(mocks.login).not.toHaveBeenCalled();
  });

  it("abandoned connection setup cannot send a later authentication request",async()=>{
    useGameScreenStore.getState().setScreen("title");const connected=deferred<boolean>();mocks.connect.mockReturnValueOnce(connected.promise);
    const run=useCharacterSelectStore.getState().authenticate({kind:"guest",token:"guest"});
    await vi.waitFor(()=>expect(mocks.connect).toHaveBeenCalledOnce());useGameScreenStore.getState().setScreen("register");
    connected.resolve(true);await expect(run).resolves.toBe(false);
    expect(mocks.socket.close).toHaveBeenCalledOnce();expect(mocks.guest).not.toHaveBeenCalled();expect(useGameScreenStore.getState().currentScreen).toBe("register");
  });

  it.each(["screen","transport"] as const)("late authenticated roster cannot hijack a replacement %s",async cause=>{
    useGameScreenStore.getState().setScreen("title");const reply=deferred<ReturnType<typeof roster>>();mocks.roster.mockReturnValueOnce(reply.promise);
    const run=useCharacterSelectStore.getState().authenticate({kind:"guest",token:"guest"});await vi.waitFor(()=>expect(mocks.roster).toHaveBeenCalledOnce());
    if(cause==="screen")useGameScreenStore.getState().setScreen("register");else retireTransport();
    useCharacterSelectStore.setState({accountId:8,accountGeneration:mocks.socket.sessionGeneration,characters:[other],selectedCharacter:other,rosterReady:true});
    reply.resolve(roster());await expect(run).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({accountId:8,characters:[other],selectedCharacter:other});
    expect(useGameScreenStore.getState().currentScreen).not.toBe("characterSelect");
  });

  it("a mismatched account roster cannot replace current characters",async()=>{
    ownAccount();mocks.roster.mockResolvedValueOnce(roster([],8));
    await expect(useCharacterSelectStore.getState().refreshCharacters()).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({accountId:7,characters:[character,other],rosterReady:false,rosterError:"Characters could not be loaded. Please try again."});
  });

  it("failed reads retain rows while a successful empty list is authoritative",async()=>{
    ownAccount();mocks.roster.mockRejectedValueOnce(new Error("database unavailable"));
    await expect(useCharacterSelectStore.getState().refreshCharacters()).resolves.toBe(false);expect(useCharacterSelectStore.getState().characters).toHaveLength(2);
    mocks.roster.mockResolvedValueOnce(roster([]));await expect(useCharacterSelectStore.getState().refreshCharacters()).resolves.toBe(true);
    expect(useCharacterSelectStore.getState()).toMatchObject({characters:[],selectedCharacter:null,rosterReady:true,rosterError:null});
  });

  it("keeps the selected stable ID and all rows when refreshing more than eight characters",async()=>{
    ownAccount();useCharacterSelectStore.getState().setSelectedCharacter(other);
    const many=Array.from({length:10},(_,index)=>({...character,id:100+index,name:"Trainer"+index}));
    mocks.roster.mockResolvedValueOnce(roster([...many,character,other]));await useCharacterSelectStore.getState().refreshCharacters();
    expect(useCharacterSelectStore.getState().characters).toHaveLength(12);expect(useCharacterSelectStore.getState().selectedCharacter?.id).toBe(other.id);
  });

  it("keeps pending deletion visible and blocks conflicting entry, creation and repeats",async()=>{
    ownAccount();const reply=deferred<ReturnType<typeof roster>&{characterId:number}>();mocks.delete.mockReturnValueOnce(reply.promise);
    const run=useCharacterSelectStore.getState().deleteCharacter(character.id);await vi.waitFor(()=>expect(mocks.delete).toHaveBeenCalledOnce());
    expect(mocks.delete.mock.calls[0][0]).toBe(character.id);
    expect(useCharacterSelectStore.getState()).toMatchObject({accountOperation:"delete",characters:[character,other],selectedCharacter:character});
    await expect(useCharacterSelectStore.getState().deleteCharacter(character.id)).resolves.toBe(false);
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(false);
    await expect(useCharacterSelectStore.getState().createCharacter(createPayload)).resolves.toBe(false);
    await expect(useCharacterSelectStore.getState().refreshCharacters()).resolves.toBe(false);
    reply.resolve({...roster([other]),characterId:character.id});await expect(run).resolves.toBe(true);
    expect(useCharacterSelectStore.getState()).toMatchObject({characters:[other],selectedCharacter:other,accountOperation:null});
    expect(mocks.enter).not.toHaveBeenCalled();expect(mocks.create).not.toHaveBeenCalled();expect(mocks.roster).not.toHaveBeenCalled();
  });

  it("explicit deletion rejection keeps its row without treating it as uncertainty",async()=>{
    ownAccount();mocks.delete.mockRejectedValueOnce(new CorrelatedResponseError({success:false,requestId:"delete",error:"Character is unavailable."}));
    await expect(useCharacterSelectStore.getState().deleteCharacter(character.id)).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({characters:[character,other],selectedCharacter:character,deleteError:"Character is unavailable.",accountOperation:null});
    expect(mocks.roster).not.toHaveBeenCalled();
  });

  it.each([true,false])("uncertain deletion reads once without resend; committed=%s",async committed=>{
    ownAccount();mocks.delete.mockRejectedValueOnce(new Error("lost acknowledgement"));mocks.roster.mockResolvedValueOnce(roster(committed?[other]:[character,other]));
    await expect(useCharacterSelectStore.getState().deleteCharacter(character.id)).resolves.toBe(committed);
    expect(mocks.delete).toHaveBeenCalledOnce();expect(mocks.roster).toHaveBeenCalledOnce();
    expect(useCharacterSelectStore.getState().characters.some(row=>row.id===character.id)).toBe(!committed);
    if(!committed)expect(useCharacterSelectStore.getState().deleteError).toContain("still listed");
  });

  it("failed deletion recovery blocks another mutation until explicit current read",async()=>{
    ownAccount();mocks.delete.mockRejectedValueOnce(new Error("lost acknowledgement"));mocks.roster.mockRejectedValueOnce(new Error("read unavailable"));
    await expect(useCharacterSelectStore.getState().deleteCharacter(character.id)).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({characters:[character,other],rosterReady:false});
    await expect(useCharacterSelectStore.getState().deleteCharacter(character.id)).resolves.toBe(false);
    await expect(useCharacterSelectStore.getState().enterWorld(character.name)).resolves.toBe(false);expect(mocks.delete).toHaveBeenCalledOnce();
    await expect(useCharacterSelectStore.getState().refreshCharacters()).resolves.toBe(true);expect(useCharacterSelectStore.getState().rosterReady).toBe(true);
  });

  it.each(["screen","transport"] as const)("old deletion cannot alter a replacement %s",async cause=>{
    ownAccount();const reply=deferred<ReturnType<typeof roster>&{characterId:number}>();mocks.delete.mockReturnValueOnce(reply.promise);
    const run=useCharacterSelectStore.getState().deleteCharacter(character.id);await vi.waitFor(()=>expect(mocks.delete).toHaveBeenCalledOnce());
    if(cause==="screen")useGameScreenStore.getState().setScreen("register");else retireTransport();
    useCharacterSelectStore.setState({accountId:8,characters:[other],selectedCharacter:other});
    reply.resolve({...roster([]),characterId:character.id});await expect(run).resolves.toBe(false);
    expect(useCharacterSelectStore.getState()).toMatchObject({accountId:8,characters:[other],selectedCharacter:other});expect(mocks.roster).not.toHaveBeenCalled();
  });

  it("recovery holds the same deletion admission until the current read settles",async()=>{
    ownAccount();mocks.delete.mockRejectedValueOnce(new Error("lost acknowledgement"));const reply=deferred<ReturnType<typeof roster>>();mocks.roster.mockReturnValueOnce(reply.promise);
    const run=useCharacterSelectStore.getState().deleteCharacter(character.id);await vi.waitFor(()=>expect(mocks.roster).toHaveBeenCalledOnce());
    expect(useCharacterSelectStore.getState().accountOperation).toBe("delete");
    await expect(useCharacterSelectStore.getState().deleteCharacter(other.id)).resolves.toBe(false);
    reply.resolve(roster([other]));await expect(run).resolves.toBe(true);expect(mocks.delete).toHaveBeenCalledOnce();
  });

  it("same-name replacement survives reconciliation of the deleted stable ID",async()=>{
    ownAccount();const replacement={...character,id:99};mocks.delete.mockRejectedValueOnce(new Error("lost acknowledgement"));mocks.roster.mockResolvedValueOnce(roster([replacement,other]));
    await expect(useCharacterSelectStore.getState().deleteCharacter(character.id)).resolves.toBe(true);
    expect(useCharacterSelectStore.getState().characters).toContainEqual(replacement);expect(mocks.delete).toHaveBeenCalledOnce();
  });

  it("creation waits for owned roster then selects the confirmed new trainer",async()=>{
    ownAccount();useGameScreenStore.getState().setScreen("characterCreate");mocks.create.mockResolvedValueOnce(true);
    const created={...character,id:99,name:createPayload.name};mocks.roster.mockResolvedValueOnce(roster([character,created]));
    await expect(useCharacterSelectStore.getState().createCharacter(createPayload)).resolves.toBe(true);
    expect(mocks.create).toHaveBeenCalledOnce();expect(mocks.roster).toHaveBeenCalledOnce();expect(useCharacterSelectStore.getState().selectedCharacter).toEqual(created);
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");
  });

  it("creation admission survives submit remount and retired completion cannot reset another screen",async()=>{
    ownAccount();useGameScreenStore.getState().setScreen("characterCreate");const reply=deferred<boolean>();mocks.create.mockReturnValueOnce(reply.promise);
    const first=useCharacterSelectStore.getState().createCharacter(createPayload);
    await expect(useCharacterSelectStore.getState().createCharacter(createPayload)).resolves.toBe(false);await vi.waitFor(()=>expect(mocks.create).toHaveBeenCalledOnce());
    useGameScreenStore.getState().setScreen("title");reply.resolve(true);await expect(first).resolves.toBe(false);
    expect(mocks.socket.close).toHaveBeenCalledOnce();expect(mocks.roster).not.toHaveBeenCalled();expect(useGameScreenStore.getState().currentScreen).toBe("title");
    expect(useCharacterSelectStore.getState().accountOperation).toBeNull();
  });

  it("creation rejection preserves its draft screen and exposes an error",async()=>{
    ownAccount();useGameScreenStore.getState().setScreen("characterCreate");mocks.create.mockResolvedValueOnce(false);
    await expect(useCharacterSelectStore.getState().createCharacter(createPayload)).resolves.toBe(false);
    expect(useGameScreenStore.getState().currentScreen).toBe("characterCreate");expect(useCharacterSelectStore.getState().createError).toContain("rejected");expect(mocks.roster).not.toHaveBeenCalled();
  });

  it("confirmed creation with missing roster offers a read retry instead of another create",async()=>{
    ownAccount();useGameScreenStore.getState().setScreen("characterCreate");mocks.create.mockResolvedValueOnce(true);mocks.roster.mockRejectedValueOnce(new Error("lost roster"));
    await expect(useCharacterSelectStore.getState().createCharacter(createPayload)).resolves.toBe(true);
    expect(useGameScreenStore.getState().currentScreen).toBe("characterSelect");expect(useCharacterSelectStore.getState()).toMatchObject({rosterReady:false,pendingSelectName:createPayload.name});
    const created={...character,id:99,name:createPayload.name};mocks.roster.mockResolvedValueOnce(roster([character,created]));await useCharacterSelectStore.getState().refreshCharacters();
    expect(useCharacterSelectStore.getState().selectedCharacter?.id).toBe(99);expect(mocks.create).toHaveBeenCalledOnce();expect(mocks.roster).toHaveBeenCalledTimes(2);
  });
});
