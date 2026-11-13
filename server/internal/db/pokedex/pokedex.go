package pokedex

import (
	"fmt"

	"capturequest/internal/db"
)

func MarkCaught(database db.DBTX, charID int64, pokemonID int) error {
	if charID <= 0 || pokemonID < 1 || pokemonID > 151 {
		return fmt.Errorf("invalid pokedex identity character=%d pokemon=%d", charID, pokemonID)
	}
	_, err := database.Exec(`
		INSERT INTO character_pokedex (character_id, pokemon_id, seen, caught, first_seen_at, first_caught_at)
		VALUES ($1, $2, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT (character_id, pokemon_id) DO UPDATE SET
			seen = 1,
			caught = 1,
			first_seen_at = COALESCE(character_pokedex.first_seen_at, EXCLUDED.first_seen_at),
			first_caught_at = COALESCE(character_pokedex.first_caught_at, EXCLUDED.first_caught_at)`,
		charID, pokemonID)
	return err
}
