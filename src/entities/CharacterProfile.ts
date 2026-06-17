import CharacterClass from "./CharacterClass";
import Faction from "./Faction";

// Updated to match Go server's CharacterData model
export default interface CharacterProfile {
  // Core character data (matching Go server exactly)
  id?: number; // ID (primary key)
  accountId?: number; // AccountID
  name?: string; // Name
  lastName?: string; // LastName
  title?: string; // Title
  suffix?: string; // Suffix
  mapId?: number; // MapID
  zoneId?: number; // Legacy client alias while older generated payloads settle.
  zoneInstance?: number; // ZoneInstance

  // Position data
  x?: number; // X
  y?: number; // Y
  z?: number; // Z
  heading?: number; // Heading

  // Basic character info
  gender?: number; // Gender
  faction?: Faction; // Faction object
  class?: CharacterClass; // Class object
  birthday?: number; // Birthday
  lastLogin?: number; // LastLogin
  timePlayed?: number; // TimePlayed
  gm?: number; // Gm

  // Special states
  deletedAt?: string | null; // DeletedAt (timestamp)

  options?: string | {
    rivalName?: string;
    allowTrainerRebattles?: boolean;
    showNetworkStats?: boolean;
    lastPokeCenterMapId?: number;
    lastPokeCenterX?: number;
    lastPokeCenterY?: number;
  };
  pokedollars?: number;

  // Bind position (Source of truth: CharacterBind model)
  bind?: {
    mapId?: number;
    x?: number;
    y?: number;
    z?: number;
    heading?: number;
  };

  position?: {
    x?: number;
    y?: number;
    z?: number;
    heading?: number;
  };
}
