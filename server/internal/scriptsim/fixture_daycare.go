package scriptsim

import (
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

func seedFixtureDayCare(charID int64, fixture DayCareFixture) error {
	if fixture.Pokemon.SpeciesID <= 0 {
		return fmt.Errorf("dayCare fixture requires pokemon.speciesId")
	}
	level := fixture.Pokemon.Level
	if level <= 0 {
		level = 5
	}
	pokemon, err := pokebattle.BuildWildPokemon(db.GlobalWorldDB.DB, fixture.Pokemon.SpeciesID, level)
	if err != nil {
		return fmt.Errorf("build Day Care pokemon %d L%d: %w", fixture.Pokemon.SpeciesID, level, err)
	}
	pokemon.IsWild = false
	if err := applyFixturePokemonMoves(pokemon, fixture.Pokemon); err != nil {
		return fmt.Errorf("seed Day Care pokemon moves: %w", err)
	}
	if err := applyFixturePokemonDetails(pokemon, fixture.Pokemon); err != nil {
		return fmt.Errorf("seed Day Care pokemon details: %w", err)
	}
	if err := pokebattle.SavePokemonToStorageSlot(db.GlobalWorldDB.DB, charID, pokebattle.BoxDayCare, 0, pokemon); err != nil {
		return fmt.Errorf("save Day Care pokemon: %w", err)
	}

	var rowID int64
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id
		FROM character_pokemon
		WHERE character_id = $1 AND box = $2 AND box_slot = 0`,
		charID, pokebattle.BoxDayCare).Scan(&rowID); err != nil {
		return fmt.Errorf("lookup Day Care pokemon row: %w", err)
	}
	startLevel := fixture.StartLevel
	if startLevel <= 0 {
		startLevel = level
	}
	if _, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_daycare (character_id, pokemon_row_id, start_level)
		VALUES ($1, $2, $3)`,
		charID, rowID, startLevel); err != nil {
		return fmt.Errorf("seed Day Care metadata: %w", err)
	}
	return nil
}
