// Package protocol owns transport payload shapes. JSON tags are authoritative
// for both encoding/json and generated TypeScript; no field-name conversion runs.
package protocol

import (
	db_character "capturequest/internal/db/character"
	"capturequest/internal/db/models"
)

// CharacterData is the character state stream. Persistence keeps options as a
// JSON string, while this view sends the parsed preferences beside base fields.
type CharacterData struct {
	models.CharacterData `tstype:",extends"`
	Options              *db_character.CharacterOptions `json:"options,omitempty"`
}

type NameValidationRequest struct {
	RequestID string `json:"requestId"`
	Name      string `json:"name"`
}

type NameValidationResponse struct {
	Success      bool   `json:"success" tstype:"true"`
	RequestID    string `json:"requestId"`
	Name         string `json:"name"`
	Valid        bool   `json:"valid"`
	Available    bool   `json:"available"`
	ErrorMessage string `json:"errorMessage"`
}

type NameValidationError struct {
	Success   bool   `json:"success" tstype:"false"`
	RequestID string `json:"requestId"`
	Error     string `json:"error"`
}
