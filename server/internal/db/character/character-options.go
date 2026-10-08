package db_character

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"capturequest/internal/db"
)

const DefaultRivalName = "Gary"

// CharacterOptions contains all player preferences stored as JSON
// All fields use camelCase for JSON to match frontend conventions
type CharacterOptions struct {
	// UI options
	PreferenceRevision    int64 `json:"preferenceRevision"`
	ShowNetworkStats      bool  `json:"showNetworkStats"`
	AllowTrainerRebattles bool  `json:"allowTrainerRebattles"`

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
	return LoadOptionsFrom(ctx, db.GlobalWorldDB.DB, charID)
}

func LoadOptionsFrom(ctx context.Context, database db.ContextDBTX, charID int32) (*CharacterOptions, error) {
	query := `SELECT options FROM character_data WHERE id = $1`

	var optionsJSON sql.NullString
	err := database.QueryRowContext(ctx, query, charID).Scan(&optionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to load options for character %d: %w", charID, err)
	}

	// If options is NULL, return defaults
	if !optionsJSON.Valid || optionsJSON.String == "" {
		return DefaultOptions(), nil
	}

	if !strings.HasPrefix(strings.TrimSpace(optionsJSON.String), "{") {
		return nil, fmt.Errorf("options for character %d must be an object", charID)
	}
	opts := DefaultOptions() // Start with defaults so missing fields get default values
	if err := json.Unmarshal([]byte(optionsJSON.String), opts); err != nil {
		return nil, fmt.Errorf("parse options for character %d: %w", charID, err)
	}
	if opts.PreferenceRevision < 0 || opts.PreferenceRevision >= 9007199254740991 {
		return nil, fmt.Errorf("invalid preference revision for character %d", charID)
	}
	opts.RivalName = NormalizeRivalName(opts.RivalName)

	return opts, nil
}

// SetBooleanOption owns one desired preference key. Newer center/story keys and
// unknown options remain in the same authoritative JSON object.
func SetBooleanOption(ctx context.Context, database *sql.DB, charID int32, key string, enabled bool, expectedRevision int64) (*CharacterOptions, error) {
	if key != "showNetworkStats" && key != "allowTrainerRebattles" {
		return nil, fmt.Errorf("unsupported preference key %q", key)
	}
	var snapshot *CharacterOptions
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, int64(charID)); err != nil {
			return err
		}
		// Use the same strict reader as entry/current preference reads. The shared
		// transaction wrapper implements ContextDBTX and preserves its own deadline.
		opts, err := LoadOptionsFrom(ctx, tx.(db.ContextDBTX), charID)
		if err != nil {
			return err
		}
		if expectedRevision < 0 || opts.PreferenceRevision != expectedRevision || expectedRevision >= 9007199254740990 {
			return fmt.Errorf("preference revision changed; read current preferences")
		}
		_, err = tx.Exec(`UPDATE character_data SET options=COALESCE(options,'{}'::jsonb)||jsonb_build_object($2::text,$3::boolean,'preferenceRevision',$4::bigint) WHERE id=$1`, charID, key, enabled, expectedRevision+1)
		if err != nil {
			return err
		}
		opts.PreferenceRevision = expectedRevision + 1
		if key == "showNetworkStats" {
			opts.ShowNetworkStats = enabled
		} else {
			opts.AllowTrainerRebattles = enabled
		}
		snapshot = opts
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Never return a pre-commit snapshot when the owning transaction fails.
	return snapshot, nil
}
