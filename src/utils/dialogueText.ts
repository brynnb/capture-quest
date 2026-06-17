export function normalizeDialogueDisplayText(text: string): string {
  if (!text) return text;

  return text
    .replace(/#MON/g, "POKÉMON")
    .replace(/# BALL/g, "POKÉ BALL")
    .replace(/#/g, "POKÉ")
    .replace(/POKéMON/g, "POKÉMON")
    .replace(/POKé/g, "POKÉ");
}
