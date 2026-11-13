import {expect,test,vi} from "vitest";
vi.mock("phaser",()=>({Scene:class {},default:{Scene:class {}}}));
const mock=vi.hoisted(()=>({read:vi.fn()}));
vi.mock("@/phaser-game/services/GameplayRecoveryService",()=>({readCurrentGameplayState:mock.read,applyGameplayResourceSnapshot:vi.fn(),applyGameplaySnapshot:vi.fn()}));
import {TileViewer} from "../TileViewer";
import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";

test("CUT projection checks visit after delayed texture completion",async()=>{
 usePlayerCharacterStore.getState().setCharacterProfile({id:42});
 let release!:()=>void;
 const add=vi.fn(),collision=vi.fn();
 const scene=Object.create(TileViewer.prototype) as any;
 scene.sys={isActive:()=>true};scene.playerMovementController={getCurrentMapId:()=>50,updateCollisionTile:collision};
 scene.mapLoader={invalidateOwnedTileProjection:vi.fn()};scene.mapDataService={fetchTilesInBounds:vi.fn(async()=>[{x:1,y:0,tileImageId:200,collisionType:1,rawFootTileId:82,talkOverTile:false}])};
 scene.mapRenderer={loadTileTextureIfNeeded:vi.fn(()=>new Promise<void>(done=>{release=done;})),addTile:add};scene.updateTileEditorLookup=vi.fn();
 mock.read.mockResolvedValue({cutScope:"new-visit"});
 const run=scene.reconcileCutTile(50,1,0,"old-visit",new AbortController().signal);
 await Promise.resolve();expect(mock.read).not.toHaveBeenCalled();release();expect(await run).toBe(false);
 expect(mock.read).toHaveBeenCalledOnce();expect(add).not.toHaveBeenCalled();expect(collision).not.toHaveBeenCalled();
});
