package options

// OptionId represents the unique identifier for each game option
// These IDs must match between client and server
type OptionId int32

const (
	// Toggle options (value: 0 = disabled, 1 = enabled)
	OptionShowNetworkStats      OptionId = 13 // Display FPS and latency
	OptionAllowTrainerRebattles OptionId = 16 // Allow re-battles with defeated trainers

	// Dropdown options (value: 0, 1, 2, etc. for different selections)
	OptionChatFilter  OptionId = 100 // Chat filter level
	OptionSoundVolume OptionId = 102 // Sound volume level
	OptionMusicVolume OptionId = 103 // Music volume level
)
