package db_character

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"unicode"

	"capturequest/internal/db"
)

const DefaultRivalName = "Gary"

// CharacterOptions contains all player preferences stored as JSON
// All fields use camelCase for JSON to match frontend conventions
type CharacterOptions struct {
	// UI options
	ShowNetworkStats      bool `json:"showNetworkStats"`
	AllowTrainerRebattles bool `json:"allowTrainerRebattles"`

	// Pokémon Center tracking (last visited center for blackout warp)
	LastPokeCenterMapID int `json:"lastPokeCenterMapId"`
	LastPokeCenterX     int `json:"lastPokeCenterX"`
	LastPokeCenterY     int `json:"lastPokeCenterY"`

	// Pokémon story options
	RivalName string `json:"rivalName"`
}

// DefaultOptions returns the default options for new characters
func DefaultOptions() *CharacterOptions {
	return &CharacterOptions{
		ShowNetworkStats: true,
		// Default blackout warp: Viridian City Pokémon Center (map 41, entrance at 3,4)
		LastPokeCenterMapID: 41,
		LastPokeCenterX:     3,
		LastPokeCenterY:     4,
		RivalName:           DefaultRivalName,
	}
}

func NormalizeRivalName(name string) string {
	var letters []rune
	for _, r := range strings.TrimSpace(name) {
		if unicode.IsLetter(r) {
			letters = append(letters, r)
		}
	}
	if len(letters) == 0 {
		return DefaultRivalName
	}
	if len(letters) > 12 {
		letters = letters[:12]
	}
	letters[0] = unicode.ToUpper(letters[0])
	for i := 1; i < len(letters); i++ {
		letters[i] = unicode.ToLower(letters[i])
	}
	return string(letters)
}

// LoadOptions loads character options from the database
func LoadOptions(ctx context.Context, charID int32) (*CharacterOptions, error) {
	query := `SELECT options FROM character_data WHERE id = $1`

	var optionsJSON sql.NullString
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, query, charID).Scan(&optionsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return DefaultOptions(), nil
		}
		return nil, fmt.Errorf("failed to load options for character %d: %w", charID, err)
	}

	// If options is NULL, return defaults
	if !optionsJSON.Valid || optionsJSON.String == "" {
		return DefaultOptions(), nil
	}

	opts := DefaultOptions() // Start with defaults so missing fields get default values
	if err := json.Unmarshal([]byte(optionsJSON.String), opts); err != nil {
		log.Printf("Warning: failed to parse options JSON for character %d, using defaults: %v", charID, err)
		return DefaultOptions(), nil
	}
	opts.RivalName = NormalizeRivalName(opts.RivalName)

	return opts, nil
}

// SaveOptions saves character options to the database
func SaveOptions(ctx context.Context, charID int32, opts *CharacterOptions) error {
	jsonBytes, err := json.Marshal(opts)
	if err != nil {
		return fmt.Errorf("failed to marshal options to JSON: %w", err)
	}

	query := `UPDATE character_data SET options = $1 WHERE id = $2`
	_, err = db.GlobalWorldDB.DB.ExecContext(ctx, query, string(jsonBytes), charID)
	if err != nil {
		return fmt.Errorf("failed to save options for character %d: %w", charID, err)
	}

	return nil
}
