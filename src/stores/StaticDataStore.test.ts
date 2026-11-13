import { beforeEach, expect, test, vi } from "vitest";
const api=vi.hoisted(()=>({staticData:vi.fn(),creation:vi.fn(),generation:0,retire:new Set<()=>void>()}));
vi.mock("@/net",()=>({WorldSocket:{get sessionGeneration(){return api.generation;},subscribeSessionRetirement:(fn:()=>void)=>{api.retire.add(fn);return()=>api.retire.delete(fn);}}}));
vi.mock("@/services/characterService",()=>({getStaticData:api.staticData,getCharCreateData:api.creation}));
import useStaticDataStore from "./StaticDataStore";
beforeEach(()=>{api.staticData.mockReset();api.creation.mockReset();useStaticDataStore.setState({isLoaded:false,isLoading:false,error:null,maps:[],classes:[],factions:[],homeTowns:[]});});
test("failed static read remains retryable rather than becoming a loaded empty catalog",async()=>{
 api.staticData.mockRejectedValueOnce(new Error("cancelled read"));
 await useStaticDataStore.getState().loadStaticData();
 expect(useStaticDataStore.getState().isLoaded).toBe(false);
 expect(useStaticDataStore.getState().error).toBe("cancelled read");
 const maps=[{id:1,name:"PALLET_TOWN",width:20,height:20,tilesetId:null,isOverworld:true,northConnection:null,southConnection:null,westConnection:null,eastConnection:null}];
 api.staticData.mockResolvedValueOnce({maps,classes:[],factions:[],startCities:[]});
 await useStaticDataStore.getState().loadStaticData();
 expect(useStaticDataStore.getState().maps).toBe(maps);
 expect(useStaticDataStore.getState().isLoaded).toBe(true);
 expect(useStaticDataStore.getState().error).toBeNull();
});

test("both catalog consumers await one request and retirement fences a late old read",async()=>{
 let resolveOld!:(value:unknown)=>void;
 api.staticData.mockReturnValueOnce(new Promise(resolve=>{resolveOld=resolve;}));
 const old=useStaticDataStore.getState().loadStaticData();
 const same=useStaticDataStore.getState().loadCharCreateData();
 expect(same).toBe(old);expect(api.staticData).toHaveBeenCalledTimes(1);
 const signal=api.staticData.mock.calls[0][0] as AbortSignal;
 api.generation++;api.retire.forEach(fn=>fn());expect(signal.aborted).toBe(true);
 const maps=[{id:2,name:"CURRENT",width:1,height:1,tilesetId:null,isOverworld:false,northConnection:null,southConnection:null,westConnection:null,eastConnection:null}];
 api.staticData.mockResolvedValueOnce({maps,classes:[],factions:[],startCities:[]});
 await useStaticDataStore.getState().loadStaticData();
 resolveOld({maps:[],classes:[],factions:[],startCities:[]});await old;
 expect(useStaticDataStore.getState().maps).toBe(maps);
 expect(useStaticDataStore.getState().isLoaded).toBe(true);
 expect(useStaticDataStore.getState().isCharCreateLoaded).toBe(true);
});

test("a creation-initiated read supplies the complete catalog to both consumers",async()=>{
 api.creation.mockResolvedValueOnce({maps:[],classes:[],factions:[],homeTowns:[]});
 const creation=useStaticDataStore.getState().loadCharCreateData();
 const general=useStaticDataStore.getState().loadStaticData();
 expect(general).toBe(creation);await general;
 expect(api.creation).toHaveBeenCalledTimes(1);expect(api.staticData).not.toHaveBeenCalled();
 expect(useStaticDataStore.getState().isLoaded).toBe(true);
});
