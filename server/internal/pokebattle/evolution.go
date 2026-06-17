package pokebattle

import (
	"fmt"
	"log"
)

// CheckEvolution checks if a Pokémon should evolve based on its current level.
// Returns the evolved species ID and name, or (0, "") if no evolution is triggered.
func CheckEvolution(db DBTX, p *Pokemon) (int, string) {
	if p.EvolveLevel <= 0 || p.EvolvePokemonName == "" {
		return 0, ""
	}
	if p.Level < p.EvolveLevel {
		return 0, ""
	}
	if db == nil {
		log.Printf("[Evolution] Cannot look up evolved species %s without a database", p.EvolvePokemonName)
		return 0, ""
	}
	// Look up the evolved species ID by name
	var evolvedID int
	err := db.QueryRow(`SELECT id FROM phaser_pokemon WHERE name = $1`, p.EvolvePokemonName).Scan(&evolvedID)
	if err != nil {
		log.Printf("[Evolution] Failed to look up evolved species %s: %v", p.EvolvePokemonName, err)
		return 0, ""
	}
	return evolvedID, p.EvolvePokemonName
}

// EvolvePokemon transforms a Pokémon into its evolved form in-place.
// It loads the new species data from the DB, preserving the Pokémon's
// level, exp, IVs, EVs, moves, PP, status, and current HP ratio.
func EvolvePokemon(db DBTX, p *Pokemon, evolvedID int) error {
	evolved, err := LoadPokemonFromDB(db, evolvedID)
	if err != nil {
		return fmt.Errorf("load evolved species %d: %w", evolvedID, err)
	}

	oldName := p.Name
	oldMaxHP := p.MaxHP

	// Update species data
	p.ID = evolved.ID
	p.Name = evolved.Name
	p.Type1 = evolved.Type1
	p.Type2 = evolved.Type2
	p.BaseStats = evolved.BaseStats
	p.CatchRate = evolved.CatchRate
	p.BaseSpeed = evolved.BaseSpeed
	p.BaseExp = evolved.BaseExp
	p.EvolveLevel = evolved.EvolveLevel
	p.EvolvePokemonName = evolved.EvolvePokemonName

	// Recalculate stats with new base stats (keeps IVs, EVs, level)
	p.RecalculateStats()

	// Increase current HP by the same amount max HP increased (Gen 1 behavior)
	hpIncrease := p.MaxHP - oldMaxHP
	if hpIncrease > 0 {
		p.CurHP += hpIncrease
	}
	// Clamp to new max
	if p.CurHP > p.MaxHP {
		p.CurHP = p.MaxHP
	}

	log.Printf("[Evolution] %s evolved into %s (ID %d → %d)", oldName, p.Name, evolvedID, p.ID)
	return nil
}
