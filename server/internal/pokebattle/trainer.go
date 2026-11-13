package pokebattle

import "fmt"

// BuildTrainerParty creates a fully initialized party of Pokémon for a trainer battle.
// It looks up the trainer's party by class name and party index, then builds each Pokémon
// at the specified level with correct moves.
func BuildTrainerParty(db DBTX, trainerClass string, partyIndex int) ([]*Pokemon, error) {
	// Get trainer class ID
	var classID int
	err := db.QueryRow(`SELECT id FROM phaser_trainer_classes WHERE constant_name = $1`, trainerClass).Scan(&classID)
	if err != nil {
		return nil, fmt.Errorf("trainer class %q not found: %w", trainerClass, err)
	}

	// Get party Pokémon
	rows, err := db.Query(`
		SELECT tpp.pokemon_name, tpp.level
		FROM phaser_trainer_party_pokemon tpp
		JOIN phaser_trainer_parties tp ON tpp.trainer_party_id = tp.id
		WHERE tp.trainer_class_id = $1 AND tp.party_index = $2
		ORDER BY tpp.slot_index`, classID, partyIndex)
	if err != nil {
		return nil, fmt.Errorf("query trainer party: %w", err)
	}
	defer rows.Close()

	type partyEntry struct {
		name  string
		level int
	}
	var entries []partyEntry
	for rows.Next() {
		var entry partyEntry
		if err := rows.Scan(&entry.name, &entry.level); err != nil {
			return nil, fmt.Errorf("scan trainer party: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read trainer party: %w", err)
	}
	// A transaction owns one PostgreSQL connection. Exhaust and close its party
	// rows before nested species/move reads; otherwise pgx reports a busy connection.
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close trainer party: %w", err)
	}
	var party []*Pokemon
	for _, entry := range entries {

		// Look up Pokémon ID by name
		var pokemonID int
		err := db.QueryRow(`SELECT id FROM phaser_pokemon WHERE name = $1`, entry.name).Scan(&pokemonID)
		if err != nil {
			return nil, fmt.Errorf("pokemon %q not found: %w", entry.name, err)
		}

		p, err := BuildWildPokemon(db, pokemonID, entry.level)
		if err != nil {
			return nil, fmt.Errorf("build trainer pokemon %q L%d: %w", entry.name, entry.level, err)
		}
		p.IsWild = false
		party = append(party, p)
	}

	if len(party) == 0 {
		return nil, fmt.Errorf("empty party for trainer %s/%d", trainerClass, partyIndex)
	}

	return party, nil
}
