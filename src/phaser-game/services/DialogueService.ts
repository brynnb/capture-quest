/**
 * DialogueService — fetches dialogue text from the server via WebSocket
 * and parses it into displayable lines for the PokemonDialogueStore.
 *
 * Data flow: actor.text (TEXT_ constant) → PhaserDialogueRequest → server resolves
 * via text_pointers + dialogue_text → returns dialogue string → parsed into lines.
 */

import { correlatedRequest, readForCurrentCharacter } from "./CorrelatedRequest";
import * as PhaserNet from "./PhaserNetworkService";
import { resolveDialoguePlaceholders } from "@/utils/dialoguePlaceholders";
import { normalizeDialogueDisplayText } from "@/utils/dialogueText";

export interface DialogueResult {
  lines: string[];
  hasBranching: boolean;
  branchingPrompt: string | null;
}

/**
 * Fetch dialogue text for a TEXT_ constant from the server.
 * Returns parsed lines ready for the dialogue box.
 */
export async function fetchDialogue(textConstant: string, signal?:AbortSignal): Promise<string[]> {
 return (await fetchDialogueWithBranching(textConstant,signal)).lines;
}

// Each demand owns its subscription, cancellation and captured session identity.
// Failures reject so a cutscene cannot substitute inline text for a failed read.
export async function fetchDialogueWithBranching(textConstant:string,signal?:AbortSignal):Promise<DialogueResult> {
 return readForCurrentCharacter(async(characterId,ownedSignal)=>{
 const response=await correlatedRequest<import("@/net/generated/protocol").PhaserDialogueResponse>(PhaserNet.onDialogueRead, requestId=>PhaserNet.requestDialogueRead(requestId,textConstant), ownedSignal,5000);
 if(response.characterId!==characterId || response.textConstant!==textConstant)throw new Error("Dialogue response identity mismatch");
 if(!Array.isArray(response.dialogueEntries) || !response.dialogueEntries.every(entry=>entry && typeof entry.dialogue==="string" && typeof entry.label==="string" && typeof entry.sourceFile==="string" && Number.isSafeInteger(entry.isTrainer) && (entry.mapName===null || typeof entry.mapName==="string")) || typeof response.hasBranching!=="boolean" || (response.branchingPrompt!==null && typeof response.branchingPrompt!=="string") || (response.hasBranching && !response.branchingPrompt))throw new Error("Invalid dialogue response");
 const raw=response.dialogueEntries.map(entry=>entry.dialogue).filter(Boolean).join("\n\n");
 return {lines:parseDialogueText(raw),hasBranching:response.hasBranching,branchingPrompt:response.branchingPrompt ? parseDialogueText(response.branchingPrompt)[0]??null : null};
 },signal);
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
