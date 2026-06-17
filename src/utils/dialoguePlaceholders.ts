import usePlayerCharacterStore from "@/stores/PlayerCharacterStore";

const DEFAULT_PLAYER_NAME = "Red";
const DEFAULT_RIVAL_NAME = "Gary";

type DialogueOptions = {
  rivalName?: string;
};

function parseOptions(options: unknown): DialogueOptions {
  if (!options) return {};
  if (typeof options === "string") {
    try {
      const parsed = JSON.parse(options);
      return typeof parsed === "object" && parsed !== null
        ? (parsed as DialogueOptions)
        : {};
    } catch {
      return {};
    }
  }
  return typeof options === "object" ? (options as DialogueOptions) : {};
}

function cleanName(value: unknown, fallback: string): string {
  return typeof value === "string" && value.trim() ? value.trim() : fallback;
}

export function getDialogueNames(): {
  playerName: string;
  rivalName: string;
} {
  const profile = usePlayerCharacterStore.getState().characterProfile;
  const options = parseOptions(profile?.options);

  return {
    playerName: cleanName(profile?.name, DEFAULT_PLAYER_NAME),
    rivalName: cleanName(options.rivalName, DEFAULT_RIVAL_NAME),
  };
}

export function resolveDialoguePlaceholders(text: string): string {
  if (!text) return text;
  const { playerName, rivalName } = getDialogueNames();
  return text
    .replace(/\{PLAYER\}|\(PLAYER\)/g, playerName)
    .replace(/\{RIVAL\}|\(RIVAL\)/g, rivalName);
}

export function resolveDialogueSpeaker(
  speaker: string | null | undefined,
): string | null {
  if (!speaker) return speaker ?? null;
  const trimmed = speaker.trim();
  if (!trimmed) return null;

  const { playerName, rivalName } = getDialogueNames();
  if (/^(?:\{PLAYER\}|\(PLAYER\)|PLAYER)$/i.test(trimmed)) {
    return playerName;
  }
  if (/^(?:\{RIVAL\}|\(RIVAL\)|RIVAL)$/i.test(trimmed)) {
    return rivalName;
  }
  return resolveDialoguePlaceholders(trimmed);
}
