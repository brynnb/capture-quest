/**
 * DialogueService — fetches dialogue text from the server via WebSocket
 * and parses it into displayable lines for the PokemonDialogueStore.
 *
 * Data flow: actor.text (TEXT_ constant) → PhaserDialogueRequest → server resolves
 * via text_pointers + dialogue_text → returns dialogue string → parsed into lines.
 */

import { WorldSocket } from "@/net/index";
import * as OpCodes from "@/net/generated/opcodes";
import { resolveDialoguePlaceholders } from "@/utils/dialoguePlaceholders";
import { normalizeDialogueDisplayText } from "@/utils/dialogueText";

interface DialogueEntry {
  label: string;
  sourceFile: string;
  dialogue: string;
  isTrainer: number;
  mapName: string | null;
}

interface DialogueResponse {
  success: boolean;
  error?: string;
  textConstant: string;
  dialogueEntries: DialogueEntry[];
  hasBranching?: boolean;
  branchingPrompt?: string;
}

export interface DialogueResult {
  lines: string[];
  hasBranching: boolean;
  branchingPrompt: string | null;
}

/**
 * Fetch dialogue text for a TEXT_ constant from the server.
 * Returns parsed lines ready for the dialogue box.
 */
export async function fetchDialogue(textConstant: string): Promise<string[]> {
  const result = await fetchDialogueWithBranching(textConstant);
  return result.lines;
}

/**
 * Fetch dialogue text with branching (YES/NO) metadata.
 * Returns lines + whether a YES/NO choice should be shown after the dialogue.
 */
export async function fetchDialogueWithBranching(
  textConstant: string,
): Promise<DialogueResult> {
  const empty: DialogueResult = {
    lines: [],
    hasBranching: false,
    branchingPrompt: null,
  };

  if (!WorldSocket.isConnected) {
    console.warn("[DialogueService] Not connected");
    return empty;
  }

  try {
    const response = (await WorldSocket.sendJsonRequest(
      OpCodes.PhaserDialogueRequest,
      OpCodes.PhaserDialogueResponse,
      { textConstant },
      5000,
    )) as DialogueResponse;

    if (!response.success || !response.dialogueEntries?.length) {
      console.warn(
        `[DialogueService] No dialogue found for ${textConstant}`,
        response.error,
      );
      return empty;
    }

    // Combine all dialogue entries (usually just one for signs/NPCs)
    const rawText = response.dialogueEntries
      .map((e) => e.dialogue)
      .filter(Boolean)
      .join("\n\n");

    return {
      lines: parseDialogueText(rawText),
      hasBranching: response.hasBranching ?? false,
      branchingPrompt: response.branchingPrompt
        ? parseDialogueText(response.branchingPrompt)[0] ?? null
        : null,
    };
  } catch (error) {
    console.error(
      `[DialogueService] Error fetching dialogue for ${textConstant}:`,
      error,
    );
    return empty;
  }
}

/**
 * Parse raw dialogue text into display lines.
 *
 * Pokémon Red/Blue dialogue conventions:
 * - \n\n = paragraph break (press A to continue)
 * - \n = line break within same text box
 * - {PLAYER} / (PLAYER) = player's name
 * - {RIVAL} / (RIVAL) = rival's name
 * - POKé/# tokens = Pokemon-era text glyphs normalized to plain ASCII
 * - @ at end = text terminator (strip)
 */
export function parseDialogueText(raw: string): string[] {
  if (!raw) return [];

  let text = raw;

  // Strip text terminator artifacts
  text = text.replace(/@$/g, "").trim();

  text = resolveDialoguePlaceholders(text);
  text = normalizeDialogueDisplayText(text);

  // Split on double newlines for paragraph breaks (each becomes a separate "page")
  const paragraphs = text
    .split(/\n\n+/)
    .map((p) => p.trim())
    .filter(Boolean);

  // If no paragraph breaks, treat the whole thing as one page
  if (paragraphs.length === 0) return [text];

  return paragraphs;
}
