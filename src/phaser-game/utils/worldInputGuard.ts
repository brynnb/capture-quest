import usePokeBattleStore from "@/stores/PokeBattleStore";
import usePokemonDialogueStore from "@/stores/PokemonDialogueStore";
import useCQInventoryStore from "@/stores/CQInventoryStore";
import { isCutscenePlaying } from "../services/CutsceneService";

export type WorldInputFreezeReason = "battle" | "dialogue" | "shop" | "cutscene";

const DEFAULT_CONSUMED_INPUT_FREEZE_MS = 250;
let worldInputSuppressedUntil = 0;

export interface WorldInputFreezeOptions {
  cutsceneInputLocked?: boolean;
  includeBattle?: boolean;
  includeDialogue?: boolean;
  includeCutscene?: boolean;
}

export function getWorldInputFreezeReason(
  options: WorldInputFreezeOptions = {},
): WorldInputFreezeReason | null {
  const includeBattle = options.includeBattle ?? true;
  const includeDialogue = options.includeDialogue ?? true;
  const includeCutscene = options.includeCutscene ?? true;

  if (includeBattle && usePokeBattleStore.getState().isInBattle) {
    return "battle";
  }

  if (includeDialogue && Date.now() < worldInputSuppressedUntil) {
    return "dialogue";
  }

  if (includeDialogue) {
    const dialogue = usePokemonDialogueStore.getState();
    if (dialogue.isOpen || dialogue.isChoicePending) {
      return "dialogue";
    }
  }

  if (useCQInventoryStore.getState().shopOpen) {
    return "shop";
  }

  if (
    includeCutscene &&
    (options.cutsceneInputLocked || isCutscenePlaying())
  ) {
    return "cutscene";
  }

  return null;
}

export function isWorldInputFrozen(
  options: WorldInputFreezeOptions = {},
): boolean {
  return getWorldInputFreezeReason(options) !== null;
}

export function suppressWorldInputFor(
  durationMs = DEFAULT_CONSUMED_INPUT_FREEZE_MS,
): void {
  worldInputSuppressedUntil = Math.max(
    worldInputSuppressedUntil,
    Date.now() + durationMs,
  );
}
