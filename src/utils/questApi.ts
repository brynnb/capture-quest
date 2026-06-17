import { WorldSocket, OpCodes } from "@/net";

export interface DialogueOption {
    text: string;
    keyword: string;
}

export interface QuestNPCInfo {
    success: boolean;
    error?: string;
    npcName: string;
    greeting: string;
    availableOptions: DialogueOption[];
    systemMessages?: string[];
    ding?: boolean;
}

export async function getQuestNPCInfo(npcId: number, zoneShortName: string): Promise<QuestNPCInfo | null> {
    try {
        if (!WorldSocket.isConnected) return null;

        const response = await WorldSocket.sendJsonRequest(
            OpCodes.QuestNPCInfoRequest,
            OpCodes.QuestNPCInfoResponse,
            { npcId, zoneShortName },
            5000
        ) as QuestNPCInfo;

        return response;
    } catch (error) {
        console.error("Error fetching quest NPC info:", error);
        return null;
    }
}

export async function sendQuestKeyword(npcId: number, zoneShortName: string, keyword: string): Promise<QuestNPCInfo | null> {
    try {
        if (!WorldSocket.isConnected) return null;

        const response = await WorldSocket.sendJsonRequest(
            OpCodes.QuestDialogueRequest,
            OpCodes.QuestDialogueResponse,
            { npcId, zoneShortName, keyword },
            5000
        ) as QuestNPCInfo;

        return response;
    } catch (error) {
        console.error("Error sending quest keyword:", error);
        return null;
    }
}
