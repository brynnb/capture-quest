import { WorldSocket } from "@/net";
import { create } from "zustand";
import {
  getStaticData,
  getCharCreateData,
  ClassData,
  MapData,
  FactionData,
  HomeTownData,
} from "@/services/characterService";
import { displayLocationNameForMap } from "@utils/locationNames";

interface StaticDataStore {
  isLoaded: boolean;
  isLoading: boolean;
  isCharCreateLoaded: boolean;
  isLoadingCharCreate: boolean;
  error: string | null;
  maps: MapData[];
  classes: ClassData[];
  factions: FactionData[];
  homeTowns: HomeTownData[];
  areModelsPreloaded: boolean;
  modelPreloadProgress: number;
  setModelsPreloaded: (loaded: boolean) => void;
  setModelPreloadProgress: (progress: number) => void;
  loadStaticData: () => Promise<void>;
  loadCharCreateData: () => Promise<void>;
  getFactionById: (id: number) => FactionData | undefined;
  getClassById: (id: number) => ClassData | undefined;
  getMapById: (mapId: number) => MapData | undefined;
  getMapNameById: (mapId: number) => string | undefined;
}

const useStaticDataStore = create<StaticDataStore>()((set, get) => ({
  isLoaded: false,
  isLoading: false,
  isCharCreateLoaded: false,
  isLoadingCharCreate: false,
  error: null,
  maps: [],
  classes: [],
  factions: [],
  homeTowns: [],

  areModelsPreloaded: false,
  modelPreloadProgress: 0,
  setModelsPreloaded: (loaded: boolean) => set({ areModelsPreloaded: loaded }),
  setModelPreloadProgress: (progress: number) =>
    set({ modelPreloadProgress: progress }),
  loadStaticData: () => loadCatalog(false),
  loadCharCreateData: () => loadCatalog(true),

  getFactionById: (id: number) => {
    return get().factions.find((f) => f.id === id);
  },

  getClassById: (id: number) => {
    return get().classes.find((c) => c.id === id);
  },

  getMapById: (mapId: number) => {
    return get().maps.find((m) => m.id === mapId);
  },

  getMapNameById: (mapId: number) =>
    displayLocationNameForMap(mapId, get().maps),

}));

type CatalogFlight = { controller: AbortController; generation: number; promise: Promise<void> };
let flight: CatalogFlight | null = null;
let lifetimeBound = false;
function loadCatalog(creation: boolean): Promise<void> {
  // Catalog data is shared across screens/characters. Retire it on transport or
  // account replacement, not when one of several observing components unmounts.
  if (!lifetimeBound) {
    lifetimeBound = true;
    WorldSocket.subscribeSessionRetirement(() => {
      flight?.controller.abort(); flight = null;
      useStaticDataStore.setState({isLoaded:false,isCharCreateLoaded:false,isLoading:false,isLoadingCharCreate:false,error:null,maps:[],classes:[],factions:[],homeTowns:[]});
    });
  }
  if (flight) return flight.promise;
  if (useStaticDataStore.getState().isLoaded) return Promise.resolve();
  const owner: CatalogFlight = {controller:new AbortController(),generation:WorldSocket.sessionGeneration,promise:Promise.resolve()};
  flight = owner;
  owner.promise = (async () => {
    try {
      const data = creation ? await getCharCreateData(owner.controller.signal) : await getStaticData(owner.controller.signal);
      if (flight!==owner || owner.controller.signal.aborted || WorldSocket.sessionGeneration!==owner.generation) return;
      useStaticDataStore.setState({isLoaded:true,isCharCreateLoaded:true,maps:data.maps,classes:data.classes,factions:data.factions,homeTowns:"homeTowns" in data ? data.homeTowns : data.startCities});
    } catch (error) {
      if (flight===owner && !owner.controller.signal.aborted) useStaticDataStore.setState({error:error instanceof Error ? error.message : "Static content unavailable"});
    } finally {
      if (flight===owner) {flight=null;useStaticDataStore.setState({isLoading:false,isLoadingCharCreate:false});}
    }
  })();
  // Publish loading only after the real shared promise exists, including for
  // synchronous store subscribers that request the other endpoint.
  useStaticDataStore.setState({isLoading:true,isLoadingCharCreate:true,error:null});
  return owner.promise;
}

export default useStaticDataStore;
