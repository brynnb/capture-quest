import { create } from "zustand";
import {
  resolveDialoguePlaceholders,
  resolveDialogueSpeaker,
} from "@/utils/dialoguePlaceholders";
import { normalizeDialogueDisplayText } from "@/utils/dialogueText";

export interface PokemonDialogueState {
  isOpen: boolean;
  lines: string[]; // All lines/paragraphs to display
  currentLineIndex: number; // Which line we're currently showing
  displayedText: string; // Currently visible text (typewriter progress)
  isTyping: boolean; // Whether typewriter is still animating
  speakerName: string | null; // NPC/sign name (null for signs)
  actorId: number | null; // The actor that triggered dialogue
  onClose: (() => void) | null; // Optional callback when dialogue closes

  // Yes/No choice state
  isChoicePending: boolean; // Whether a YES/NO choice is being shown
  choicePrompt: string | null; // The question text
  onChoice: ((yes: boolean) => void) | null; // Callback with player's choice

  // Actions
  openDialogue: (lines: string[], speakerName?: string | null, actorId?: number, onClose?: () => void) => void;
  closeDialogue: () => void;
  advanceLine: () => void; // Press A / click to advance
  completeCurrentLine: () => void; // Skip typewriter, show full line
  setDisplayedText: (text: string) => void;
  setIsTyping: (typing: boolean) => void;
  showChoice: (prompt: string, onChoice: (yes: boolean) => void) => void;
  resolveChoice: (yes: boolean) => void;
  resetDialogueState: () => void;
}

const usePokemonDialogueStore = create<PokemonDialogueState>((set, get) => ({
  isOpen: false,
  lines: [],
  currentLineIndex: 0,
  displayedText: "",
  isTyping: false,
  speakerName: null,
  actorId: null,
  onClose: null,
  isChoicePending: false,
  choicePrompt: null,
  onChoice: null,

  openDialogue: (lines, speakerName = null, actorId, onClose) => {
    if (lines.length === 0) return;
    const resolvedLines = lines.map((line) =>
      normalizeDialogueDisplayText(resolveDialoguePlaceholders(line)),
    );
    set({
      isOpen: true,
      lines: resolvedLines,
      currentLineIndex: 0,
      displayedText: "",
      isTyping: true,
      speakerName: resolveDialogueSpeaker(speakerName),
      actorId: actorId ?? null,
      onClose: onClose ?? null,
      isChoicePending: false,
      choicePrompt: null,
      onChoice: null,
    });
  },

  closeDialogue: () => {
    const { onClose } = get();
    set({
      isOpen: false,
      lines: [],
      currentLineIndex: 0,
      displayedText: "",
      isTyping: false,
      speakerName: null,
      actorId: null,
      onClose: null,
      isChoicePending: false,
      choicePrompt: null,
      onChoice: null,
    });
    if (onClose) onClose();
  },

  advanceLine: () => {
    const { isTyping, currentLineIndex, lines } = get();
    if (isTyping) {
      // If still typing, complete the current line instantly
      get().completeCurrentLine();
      return;
    }
    // Move to next line
    const nextIndex = currentLineIndex + 1;
    if (nextIndex >= lines.length) {
      // No more lines — close dialogue
      get().closeDialogue();
    } else {
      set({
        currentLineIndex: nextIndex,
        displayedText: "",
        isTyping: true,
      });
    }
  },

  completeCurrentLine: () => {
    const { lines, currentLineIndex } = get();
    if (currentLineIndex < lines.length) {
      set({
        displayedText: lines[currentLineIndex],
        isTyping: false,
      });
    }
  },

  setDisplayedText: (text) => set({ displayedText: text }),
  setIsTyping: (typing) => set({ isTyping: typing }),

  showChoice: (prompt, onChoice) => {
    const resolvedPrompt = normalizeDialogueDisplayText(
      resolveDialoguePlaceholders(prompt),
    );
    set({
      isOpen: true,
      lines: [],
      currentLineIndex: 0,
      isChoicePending: true,
      choicePrompt: resolvedPrompt,
      displayedText: resolvedPrompt,
      isTyping: false,
      speakerName: null,
      actorId: null,
      onClose: null,
      onChoice,
    });
  },

  resolveChoice: (yes) => {
    const { onChoice } = get();
    set({
      isOpen: false,
      lines: [],
      currentLineIndex: 0,
      displayedText: "",
      isTyping: false,
      speakerName: null,
      actorId: null,
      onClose: null,
      isChoicePending: false,
      choicePrompt: null,
      onChoice: null,
    });
    if (onChoice) onChoice(yes);
  },

  resetDialogueState: () => {
    set({
      isOpen: false,
      lines: [],
      currentLineIndex: 0,
      displayedText: "",
      isTyping: false,
      speakerName: null,
      actorId: null,
      onClose: null,
      isChoicePending: false,
      choicePrompt: null,
      onChoice: null,
    });
  },
}));

export default usePokemonDialogueStore;
