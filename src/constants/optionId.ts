// OptionId values must match the server-side options.go constants

export enum OptionId {
  // Toggle options (value: 0 = disabled, 1 = enabled)
  AllowTrainerRebattles = 16, // Allow re-battles with defeated trainers

  // Dropdown options (value: 0, 1, 2, etc. for different selections)
  ChatFilter = 100, // Chat filter level
  SoundVolume = 102, // Sound volume level
  MusicVolume = 103, // Music volume level
}
