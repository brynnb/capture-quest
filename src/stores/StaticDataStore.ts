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
  loadStaticData: async () => {
    const { isLoaded, isLoading } = get();
    if (isLoaded || isLoading) return;

    set({ isLoading: true, error: null });

    try {
      const data = await getStaticData();
      set({ isLoaded:true, isLoading:false, maps:data.maps, factions:data.factions, classes:data.classes, homeTowns:data.startCities });
    } catch (error) {
      // A failed read is retryable. Empty successful arrays come from the server;
      // cancellation or missing content must not become a loaded empty catalog.
      set({ isLoaded:false, isLoading:false, error:error instanceof Error ? error.message : "Static content unavailable" });
    }
  },

  loadCharCreateData: async () => {
    const { isCharCreateLoaded, isLoadingCharCreate } = get();
    if (isCharCreateLoaded || isLoadingCharCreate) return;

    set({ isLoadingCharCreate: true, error: null });

    try {
      const data = await getCharCreateData();

      set({
        isCharCreateLoaded: true,
        isLoadingCharCreate: false,
        ...data,
      });

      console.log(`[StaticData] Loaded Pokémon CharCreate theme`);
    } catch (error) {
      console.error("Failed to load character creation data:", error);
      set({
        isLoadingCharCreate: false,
        error: error instanceof Error ? error.message : "Unknown error",
      });
    }
  },

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

export default useStaticDataStore;
