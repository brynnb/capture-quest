import { create } from "zustand";
import {
  FactionData,
  ClassData,
  HomeTownData,
} from "@/services/characterService";
import { createCharacter } from "@/services/authService";

export interface CharacterCreatorStore {
  selectedFaction: FactionData | null;
  selectedClass: ClassData | null;
  selectedHomeTown: HomeTownData | null;
  selectedGender: number;
  characterName: string;
  rivalName: string;
  nameValidation: {
    isValid: boolean;
    isAvailable: boolean;
    errorMessage: string;
    isValidating: boolean;
  };
  currentStep: number;

  setSelectedFaction: (faction: FactionData | null) => void;
  setSelectedClass: (charClass: ClassData | null) => void;
  setSelectedHomeTown: (homeTown: HomeTownData | null) => void;
  setSelectedGender: (gender: number) => void;
  setCharacterName: (name: string) => void;
  setRivalName: (name: string) => void;
  setNameValidation: (validation: {
    isValid: boolean;
    isAvailable: boolean;
    errorMessage: string;
    isValidating: boolean;
  }) => void;
  setCurrentStep: (step: number) => void;

  initializeDefaults: () => void;
  resetStore: () => void;
  canProceedToNextStep: () => boolean;
  createCharacter: () => Promise<boolean>;
}

const useCharacterCreatorStore = create<CharacterCreatorStore>()(
  (set, get) => ({
    selectedHomeTown: null,
    selectedFaction: null,
    selectedClass: null,
    selectedGender: 0,
    characterName: "",
    rivalName: "Gary",
    currentStep: 1,
    nameValidation: {
      isValid: false,
      isAvailable: false,
      errorMessage: "",
      isValidating: false,
    },

    setSelectedFaction: (faction) => set({ selectedFaction: faction }),
    setSelectedClass: (charClass) => set({ selectedClass: charClass }),
    setSelectedHomeTown: (homeTown) => set({ selectedHomeTown: homeTown }),
    setSelectedGender: (gender) => set({ selectedGender: gender }),
    setCharacterName: (name) => set({ characterName: name }),
    setRivalName: (name) => set({ rivalName: name }),
    setNameValidation: (validation) => set({ nameValidation: validation }),
    setCurrentStep: (step) => set({ currentStep: step }),

    initializeDefaults: () => {
      set({
        selectedFaction: null,
        selectedClass: null,
      });
    },

    resetStore: () => {
      set({
        selectedHomeTown: null,
        selectedFaction: null,
        selectedClass: null,
        selectedGender: 0,
        characterName: "",
        rivalName: "Gary",
        nameValidation: {
          isValid: false,
          isAvailable: false,
          errorMessage: "",
          isValidating: false,
        },
        currentStep: 1,
      });
    },

    canProceedToNextStep: () => {
      const state = get();
      switch (state.currentStep) {
        case 1:
          return !!(
            state.characterName &&
            state.nameValidation.isValid &&
            state.nameValidation.isAvailable &&
            !state.nameValidation.isValidating &&
            state.rivalName.trim().length > 0 &&
            state.selectedFaction
          );
        case 2:
          return !!state.selectedClass;
        case 3:
          return state.selectedHomeTown !== null;
        default:
          return false;
      }
    },

    createCharacter: async () => {
      const state = get();
      if (
        !state.selectedFaction ||
        !state.selectedClass ||
        !state.selectedHomeTown
      ) {
        return false;
      }

      return await createCharacter({
        name: state.characterName,
        rivalName: state.rivalName,
        factionId: state.selectedFaction.id,
        charClass: state.selectedClass.id,
        homeTownMapId: state.selectedHomeTown.mapId,
        gender: state.selectedGender,
        tutorial: 0,
      });
    },
  }),
);

export default useCharacterCreatorStore;
