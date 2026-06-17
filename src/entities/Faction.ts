export default interface Faction {
  id: number;
  name: string;
  shortName?: string;
  lore?: string;
  isPlayable?: boolean;
  isStarting?: boolean;
}
