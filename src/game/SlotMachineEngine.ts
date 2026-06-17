/**
 * Slot Machine Engine — faithful recreation of the original Game Boy
 * Pokémon Red/Blue slot machine mechanics.
 *
 * Source: engine/slots/slot_machine.asm, data/events/slot_machine_wheels.asm
 */

// Symbol IDs matching the original game constants
export enum SlotSymbol {
  SEVEN = 0,
  BAR = 1,
  CHERRY = 2,
  FISH = 3,    // Staryu/Goldeen
  BIRD = 4,    // Pidgey
  MOUSE = 5,   // Pikachu
}

// Display names and emoji for each symbol
export const SYMBOL_DISPLAY: Record<SlotSymbol, { name: string; emoji: string }> = {
  [SlotSymbol.SEVEN]:  { name: "7",       emoji: "7️⃣" },
  [SlotSymbol.BAR]:    { name: "BAR",     emoji: "🎰" },
  [SlotSymbol.CHERRY]: { name: "Cherry",  emoji: "🍒" },
  [SlotSymbol.FISH]:   { name: "Staryu",  emoji: "⭐" },
  [SlotSymbol.BIRD]:   { name: "Pidgey",  emoji: "🐦" },
  [SlotSymbol.MOUSE]:  { name: "Pikachu", emoji: "⚡" },
};

// Exact reel layouts from data/events/slot_machine_wheels.asm
// Each wheel has 18 symbols (indices 0-17), read as pairs of bytes
// but we only need the symbol identity.
export const WHEEL_1: SlotSymbol[] = [
  SlotSymbol.SEVEN,  SlotSymbol.MOUSE, SlotSymbol.FISH,
  SlotSymbol.BAR,    SlotSymbol.CHERRY, SlotSymbol.SEVEN,
  SlotSymbol.FISH,   SlotSymbol.BIRD,  SlotSymbol.BAR,
  SlotSymbol.CHERRY, SlotSymbol.SEVEN, SlotSymbol.MOUSE,
  SlotSymbol.BIRD,   SlotSymbol.BAR,   SlotSymbol.CHERRY,
  SlotSymbol.SEVEN,  SlotSymbol.MOUSE, SlotSymbol.FISH,
];

export const WHEEL_2: SlotSymbol[] = [
  SlotSymbol.SEVEN,  SlotSymbol.FISH,   SlotSymbol.CHERRY,
  SlotSymbol.BIRD,   SlotSymbol.MOUSE,  SlotSymbol.BAR,
  SlotSymbol.CHERRY, SlotSymbol.FISH,   SlotSymbol.BIRD,
  SlotSymbol.CHERRY, SlotSymbol.BAR,    SlotSymbol.FISH,
  SlotSymbol.BIRD,   SlotSymbol.CHERRY, SlotSymbol.MOUSE,
  SlotSymbol.SEVEN,  SlotSymbol.FISH,   SlotSymbol.CHERRY,
];

export const WHEEL_3: SlotSymbol[] = [
  SlotSymbol.SEVEN,  SlotSymbol.BIRD,  SlotSymbol.FISH,
  SlotSymbol.CHERRY, SlotSymbol.MOUSE, SlotSymbol.BIRD,
  SlotSymbol.FISH,   SlotSymbol.CHERRY, SlotSymbol.MOUSE,
  SlotSymbol.BIRD,   SlotSymbol.FISH,  SlotSymbol.CHERRY,
  SlotSymbol.MOUSE,  SlotSymbol.BIRD,  SlotSymbol.BAR,
  SlotSymbol.SEVEN,  SlotSymbol.BIRD,  SlotSymbol.FISH,
];

export const WHEELS = [WHEEL_1, WHEEL_2, WHEEL_3];
export const WHEEL_SIZE = 18;

/**
 * Get the 3 visible symbols for a wheel at a given offset.
 * Returns [bottom, middle, top] to match the original game's convention.
 * The offset wraps around the wheel.
 */
export function getVisibleSymbols(
  wheel: SlotSymbol[],
  offset: number,
): [SlotSymbol, SlotSymbol, SlotSymbol] {
  const size = wheel.length;
  const bottom = wheel[offset % size];
  const middle = wheel[(offset + 1) % size];
  const top = wheel[(offset + 2) % size];
  return [bottom, middle, top];
}

/**
 * Payout table from SlotRewardPointers in slot_machine.asm:
 *   Symbol index 0 (SEVEN):  300 coins
 *   Symbol index 1 (BAR):    100 coins
 *   Symbol index 2 (CHERRY):   8 coins
 *   Symbol index 3 (FISH):    15 coins
 *   Symbol index 4 (BIRD):    15 coins
 *   Symbol index 5 (MOUSE):   15 coins
 */
export const PAYOUT_TABLE: Record<SlotSymbol, number> = {
  [SlotSymbol.SEVEN]: 300,
  [SlotSymbol.BAR]: 100,
  [SlotSymbol.CHERRY]: 8,
  [SlotSymbol.FISH]: 15,
  [SlotSymbol.BIRD]: 15,
  [SlotSymbol.MOUSE]: 15,
};

export interface MatchResult {
  symbol: SlotSymbol;
  payout: number;
  line: string; // "middle", "top", "bottom", "diagonal-up", "diagonal-down"
}

/**
 * Check for matches given the visible symbols on each wheel.
 * Bet determines which lines are active:
 *   1 coin: middle row only
 *   2 coins: middle + top + bottom rows
 *   3 coins: middle + top + bottom + both diagonals
 *
 * From SlotMachine_CheckForMatches in slot_machine.asm
 */
export function checkMatches(
  wheel1: [SlotSymbol, SlotSymbol, SlotSymbol],
  wheel2: [SlotSymbol, SlotSymbol, SlotSymbol],
  wheel3: [SlotSymbol, SlotSymbol, SlotSymbol],
  bet: number,
): MatchResult | null {
  // [bottom=0, middle=1, top=2]

  // 3 coin bet: check diagonals first (highest priority in ASM)
  if (bet >= 3) {
    // Diagonal: bottom-left to top-right
    if (wheel1[0] === wheel2[1] && wheel2[1] === wheel3[2]) {
      return {
        symbol: wheel1[0],
        payout: PAYOUT_TABLE[wheel1[0]],
        line: "diagonal-up",
      };
    }
    // Diagonal: top-left to bottom-right
    if (wheel1[2] === wheel2[1] && wheel2[1] === wheel3[0]) {
      return {
        symbol: wheel1[2],
        payout: PAYOUT_TABLE[wheel1[2]],
        line: "diagonal-down",
      };
    }
  }

  // 2 coin bet: check top and bottom rows
  if (bet >= 2) {
    // Top row
    if (wheel1[2] === wheel2[2] && wheel2[2] === wheel3[2]) {
      return {
        symbol: wheel1[2],
        payout: PAYOUT_TABLE[wheel1[2]],
        line: "top",
      };
    }
    // Bottom row
    if (wheel1[0] === wheel2[0] && wheel2[0] === wheel3[0]) {
      return {
        symbol: wheel1[0],
        payout: PAYOUT_TABLE[wheel1[0]],
        line: "bottom",
      };
    }
  }

  // 1 coin bet: middle row (always active)
  if (wheel1[1] === wheel2[1] && wheel2[1] === wheel3[1]) {
    return {
      symbol: wheel1[1],
      payout: PAYOUT_TABLE[wheel1[1]],
      line: "middle",
    };
  }

  return null;
}

/**
 * Determine if the current spin should be allowed to win.
 * From SlotMachine_SetFlags in slot_machine.asm:
 *
 * Lucky machine: sevenAndBarModeChance = 250 (vs 253 for normal)
 * Random byte (0-255):
 *   - If 0: set allowMatchesCounter to 60 (guaranteed wins for 60 spins)
 *   - If < sevenAndBarModeChance: allow 7/BAR matches
 *   - If < 210: allow any match (~21.5% chance)
 *   - Otherwise: no match allowed
 */
export function determineWinFlags(isLucky: boolean): {
  canWin: boolean;
  canWinSevenOrBar: boolean;
} {
  const rand = Math.floor(Math.random() * 256);
  const sevenBarChance = isLucky ? 250 : 253;

  if (rand === 0) {
    // ~0.4% chance: guaranteed win mode
    return { canWin: true, canWinSevenOrBar: true };
  }

  if (rand < sevenBarChance) {
    // Can win with 7 or BAR
    return { canWin: true, canWinSevenOrBar: true };
  }

  if (rand < 210) {
    // Can win with any symbol (~21.5% effective chance)
    return { canWin: true, canWinSevenOrBar: false };
  }

  // No win allowed this spin
  return { canWin: false, canWinSevenOrBar: false };
}

/**
 * Generate random final reel positions for a spin.
 * Each position is 0 to WHEEL_SIZE-1.
 */
export function generateSpinResult(): [number, number, number] {
  return [
    Math.floor(Math.random() * WHEEL_SIZE),
    Math.floor(Math.random() * WHEEL_SIZE),
    Math.floor(Math.random() * WHEEL_SIZE),
  ];
}
