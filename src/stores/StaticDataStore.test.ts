import { beforeEach, expect, test, vi } from "vitest";
const api=vi.hoisted(()=>({staticData:vi.fn(),creation:vi.fn()}));
vi.mock("@/services/characterService",()=>({getStaticData:api.staticData,getCharCreateData:api.creation}));
import useStaticDataStore from "./StaticDataStore";
beforeEach(()=>{api.staticData.mockReset();useStaticDataStore.setState({isLoaded:false,isLoading:false,error:null,maps:[],classes:[],factions:[],homeTowns:[]});});
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
