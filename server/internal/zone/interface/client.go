package entity

import model "capturequest/internal/db/models"

// Client defines the minimal methods Session needs.
type Client interface {
	CharData() *model.CharacterData
	ID() int
	Name() string
	Say(msg string)

	// UI options
	ShowNetworkStatsEnabled() bool
	SetShowNetworkStatsEnabled(enabled bool)
	AllowTrainerRebattles() bool
	SetAllowTrainerRebattlesEnabled(enabled bool)

	Options() interface{}
	SaveOptions() error

	SendSystemMessage(text string)
	SendSpecialMessage(text string, msgType string)

	SendStateUpdate()

	// Shutdown is called when a client disconnects or switches characters.
	Shutdown()
}
