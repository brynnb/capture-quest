import type { ClassData } from "@/services/characterService";

export function findDefaultCharacterClass(
  classes: ClassData[],
): ClassData | undefined {
  return classes.find(
    (characterClass) => characterClass.name.trim().toLowerCase() === "bug catcher",
  );
}
