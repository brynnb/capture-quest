package pokebattle

import "fmt"

// RepairPokemonWithNoMoves restores persisted Pokemon that were created while
// default move constants were resolved against the display-name column. A
// battle-ready Pokemon having no moves is invalid in Gen 1, so the all-zero
// predicate is both narrowly targeted and safe to rerun.
func RepairPokemonWithNoMoves(db DBTX) (int, error) {
	rows, err := db.Query(`
		SELECT id, pokemon_id, level
		FROM character_pokemon
		WHERE move1_id = 0 AND move2_id = 0 AND move3_id = 0 AND move4_id = 0
		ORDER BY id`)
	if err != nil {
		return 0, fmt.Errorf("query pokemon with no moves: %w", err)
	}

	type persistedPokemon struct {
		rowID     int64
		speciesID int
		level     int
	}
	var candidates []persistedPokemon
	for rows.Next() {
		var candidate persistedPokemon
		if err := rows.Scan(&candidate.rowID, &candidate.speciesID, &candidate.level); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan pokemon with no moves: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close pokemon repair rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read pokemon with no moves: %w", err)
	}

	repaired := 0
	for _, candidate := range candidates {
		pokemon, err := BuildWildPokemon(db, candidate.speciesID, candidate.level)
		if err != nil {
			return repaired, fmt.Errorf("build moves for character_pokemon %d: %w", candidate.rowID, err)
		}
		if pokemon.Moves[0].ID == 0 && pokemon.Moves[1].ID == 0 && pokemon.Moves[2].ID == 0 && pokemon.Moves[3].ID == 0 {
			return repaired, fmt.Errorf("pokemon species %d level %d has no legal moves", candidate.speciesID, candidate.level)
		}

		result, err := db.Exec(`
			UPDATE character_pokemon
			SET move1_id = $1, move1_pp = $2,
			    move2_id = $3, move2_pp = $4,
			    move3_id = $5, move3_pp = $6,
			    move4_id = $7, move4_pp = $8,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = $9
			  AND move1_id = 0 AND move2_id = 0 AND move3_id = 0 AND move4_id = 0`,
			pokemon.Moves[0].ID, pokemon.Moves[0].PP,
			pokemon.Moves[1].ID, pokemon.Moves[1].PP,
			pokemon.Moves[2].ID, pokemon.Moves[2].PP,
			pokemon.Moves[3].ID, pokemon.Moves[3].PP,
			candidate.rowID,
		)
		if err != nil {
			return repaired, fmt.Errorf("repair character_pokemon %d: %w", candidate.rowID, err)
		}
		if affected, err := result.RowsAffected(); err == nil && affected > 0 {
			repaired++
		}
	}
	return repaired, nil
}
