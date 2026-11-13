import {expect,test,vi} from "vitest";
const catalog=vi.hoisted(()=>({isLoaded:true,maps:[] as unknown[]}));
vi.mock("./StaticDataStore",()=>({default:{getState:()=>catalog}}));
vi.mock("@/net",async()=>({WorldSocket:{},OpCodes:await import("@/net/generated/opcodes")}));
import useGameStatusStore from "./GameStatusStore";
test("populated map view follows a replacement catalog and clears when its owner retires",async()=>{
 const old=[{id:1,name:"OLD",width:1,height:1,tilesetId:null,isOverworld:false,northConnection:null,southConnection:null,westConnection:null,eastConnection:null}];
 const current=[{...old[0],id:2,name:"CURRENT"}];
 useGameStatusStore.setState({maps:old});catalog.maps=current;
 await useGameStatusStore.getState().initializeMaps();expect(useGameStatusStore.getState().maps).toBe(current);
 catalog.isLoaded=false;catalog.maps=[];
 await useGameStatusStore.getState().initializeMaps();expect(useGameStatusStore.getState().maps).toEqual([]);
});
