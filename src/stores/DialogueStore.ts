import { create } from "zustand";
import { QuestNPCInfo } from "@utils/questApi";

interface DialogueEntry {
  npcDialogue: string;
  playerQuestion?: string;
  isPlayer: boolean;
  systemMessages?: string[];
  ding?: boolean;
}

interface DialogueState {
  dialogueHistory: Record<string, DialogueEntry[]>;
  npcLatestInfos: Record<string, QuestNPCInfo | null>;
  currentNPC: string | null;
  currentNPCID: number | null;
  currentInfo: QuestNPCInfo | null;
  setCurrentInfo: (info: QuestNPCInfo | null) => void;
  setCurrentNPC: (npcName: string | null, npcId?: number) => void;
  addDialogueEntry: (npcName: string, entry: DialogueEntry) => void;
  getDialogueHistory: (npcName: string) => DialogueEntry[];
  isLoading: boolean;
  setIsLoading: (isLoading: boolean) => void;
  resetConversation: (npcName: string) => void;
}

const useDialogueStore = create<DialogueState>((set, get) => ({
  dialogueHistory: {},
  npcLatestInfos: {},
  currentNPC: null,
  currentNPCID: null,
  currentInfo: null,
  setCurrentInfo: (info) =>
    set((state) => {
      const nextState: Partial<DialogueState> = { currentInfo: info };
      if (state.currentNPC) {
        nextState.npcLatestInfos = {
          ...state.npcLatestInfos,
          [state.currentNPC]: info,
        };
      }
      return nextState;
    }),
  setCurrentNPC: (npcName, npcId) =>
    set((state) => ({
      currentNPC: npcName,
      currentNPCID: npcId !== undefined ? npcId : state.currentNPCID,
      currentInfo: npcName ? state.npcLatestInfos[npcName] || null : null,
    })),
  addDialogueEntry: (npcName, entry) =>
    set((state) => ({
      dialogueHistory: {
        ...state.dialogueHistory,
        [npcName]: [...(state.dialogueHistory[npcName] || []), entry],
      },
    })),
  getDialogueHistory: (npcName) => get().dialogueHistory[npcName] || [],
  isLoading: false,
  setIsLoading: (isLoading) => set({ isLoading }),
  resetConversation: (npcName) =>
    set((state) => {
      const newHistory = { ...state.dialogueHistory };
      const newLatest = { ...state.npcLatestInfos };
      delete newHistory[npcName];
      delete newLatest[npcName];
      return {
        dialogueHistory: newHistory,
        npcLatestInfos: newLatest,
        currentInfo: state.currentNPC === npcName ? null : state.currentInfo,
      };
    }),
}));

export default useDialogueStore;
