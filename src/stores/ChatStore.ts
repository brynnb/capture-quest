import { create } from "zustand";
import {
  WorldSocket,
  OpCodes,
} from "@/net";

export enum MessageType {
  SYSTEM = "SYSTEM",
  GENERAL_CHAT = "GENERAL_CHAT",
  LOOT = "LOOT",
  QUEST_UPDATE = "QUEST_UPDATE",
  ZONE_ANNOUNCEMENT = "ZONE_ANNOUNCEMENT",
  ZONE_ENTRY = "ZONE_ENTRY",
  DEATH = "DEATH",
  RESURRECTION = "RESURRECTION",
  SYSTEM_ERROR = "SYSTEM_ERROR",
}

interface ChatMessage {
  id: number;
  text: string;
  timestamp: number;
  type: MessageType;
}

interface ChatStore {
  messages: ChatMessage[];
  addMessage: (text: string, type: MessageType) => void;
  sendMessage: (text: string) => Promise<void>;
  clearMessages: () => void;
  nextId: number;
  isConnected: boolean;
  connectionError: string | null;
  initializeWebTransport: () => Promise<void>;
  handleChatMessage: (data: any) => void;
}

const mapMessageType = (messageType: string): MessageType => {
  switch (String(messageType || "general").toLowerCase()) {
    case "general":
      return MessageType.GENERAL_CHAT;
    case "system":
      return MessageType.SYSTEM;
    case "loot":
      return MessageType.LOOT;
    case "quest_update":
      return MessageType.QUEST_UPDATE;
    case "zone_announcement":
      return MessageType.ZONE_ANNOUNCEMENT;
    case "zone_entry":
      return MessageType.ZONE_ENTRY;
    case "death":
      return MessageType.DEATH;
    case "resurrection":
      return MessageType.RESURRECTION;
    case "system_error":
      return MessageType.SYSTEM_ERROR;
    default:
      return MessageType.GENERAL_CHAT;
  }
};

const useChatStore = create<ChatStore>()(
  (set, get) => ({
    messages: [],
    nextId: 1,
    isConnected: false,
    connectionError: null,

    addMessage: (text: string, type: MessageType) =>
      set((state) => ({
        messages: [
          ...state.messages,
          { id: state.nextId, text, timestamp: Date.now(), type },
        ].slice(-500),
        nextId: state.nextId + 1,
      })),

    sendMessage: async (text: string) => {
      try {
        await WorldSocket.sendJsonMessage(OpCodes.SendChatMessage, {
          text,
        });
      } catch (error) {
        console.error("Failed to send chat message:", error);
        set({ connectionError: `Failed to send message: ${error}` });
      }
    },

    handleChatMessage: (data: any) => {
      const type = mapMessageType(data.messageType);
      const displayText = type === MessageType.GENERAL_CHAT && data.senderName
        ? `${data.senderName}: ${data.text}`
        : data.text;
      get().addMessage(displayText, type);
    },

    initializeWebTransport: async () => {
      try {
        set({ connectionError: null });
        if (WorldSocket.isConnected) {
          set({ isConnected: true, connectionError: null });
          import("./StaticDataStore").then((module) => {
            module.default.getState().loadStaticData();
          });
        } else {
          set({ isConnected: false, connectionError: null });
        }
      } catch (error) {
        console.error("Failed to initialize chat:", error);
        set({ isConnected: false, connectionError: `Chat initialization failed: ${error}` });
      }
    },

    clearMessages: () => set({ messages: [], nextId: 1 }),
  })
);

export default useChatStore;
