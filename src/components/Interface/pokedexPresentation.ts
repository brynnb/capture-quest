export interface PokedexPresentation {
  displayName: string;
  canOpen: boolean;
  isCaught: boolean;
}

export function getPokedexPresentation(
  speciesName: string | undefined,
  seen: boolean,
  caught: boolean,
): PokedexPresentation {
  // Owning a Pokémon always implies it has been seen, even while an older save
  // is waiting for the server-side reconciliation pass.
  const isSeen = seen || caught;
  return {
    displayName: isSeen && speciesName ? speciesName.toLowerCase() : "???",
    canOpen: isSeen && speciesName != null,
    isCaught: caught,
  };
}
