package scriptsim

import (
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

func seedFixturePCPokemon(charID int64, fixture FixturePCPokemon) error {
	if fixture.SpeciesID <= 0 {
		return fmt.Errorf("pc pokemon fixture requires speciesId")
	}
	level := fixture.Level
	if level <= 0 {
		level = 5
	}
	pokemon, err := pokebattle.BuildWildPokemon(db.GlobalWorldDB.DB, fixture.SpeciesID, level)
	if err != nil {
		return fmt.Errorf("build PC pokemon %d L%d: %w", fixture.SpeciesID, level, err)
	}
	pokemon.IsWild = false
	if err := pokebattle.SavePokemonToPCSlot(db.GlobalWorldDB.DB, charID, fixture.Box, fixture.Slot, pokemon); err != nil {
		return fmt.Errorf("seed PC pokemon %d box=%d slot=%d: %w", fixture.SpeciesID, fixture.Box, fixture.Slot, err)
	}
	return nil
}

func seedFixturePartyPokemonDetails(charID int64, fixtures []FixturePokemon) error {
	if !hasFixturePartyDetails(fixtures) {
		return nil
	}
	party, err := pokebattle.LoadParty(db.GlobalWorldDB.DB, charID)
	if err != nil {
		return fmt.Errorf("load seeded party: %w", err)
	}
	for slot, fixture := range fixtures {
		if !hasFixturePokemonDetails(fixture) {
			continue
		}
		if slot >= len(party) || party[slot] == nil {
			return fmt.Errorf("fixture party slot %d was not seeded", slot)
		}
		if err := applyFixturePokemonDetails(party[slot], fixture); err != nil {
			return err
		}
	}
	if err := pokebattle.SaveParty(db.GlobalWorldDB.DB, charID, party); err != nil {
		return fmt.Errorf("save seeded party details: %w", err)
	}
	return nil
}

func hasFixturePartyDetails(fixtures []FixturePokemon) bool {
	for _, fixture := range fixtures {
		if hasFixturePokemonDetails(fixture) {
			return true
		}
	}
	return false
}

func hasFixturePokemonDetails(fixture FixturePokemon) bool {
	return len(fixture.MovePP) > 0 ||
		fixture.CurHP != nil ||
		fixture.Status != "" ||
		fixture.Exp != nil ||
		fixture.IVs != nil ||
		fixture.EVs != nil
}

func parseFixturePokemonStatus(status string) (pokebattle.StatusCondition, error) {
	switch normalizeFixtureMoveName(status) {
	case "", "NONE", "OK":
		return pokebattle.StatusNone, nil
	case "BRN", "BURN":
		return pokebattle.StatusBurn, nil
	case "FRZ", "FREEZE", "FROZEN":
		return pokebattle.StatusFreeze, nil
	case "PAR", "PARALYZE", "PARALYZED":
		return pokebattle.StatusParalyze, nil
	case "PSN", "POISON":
		return pokebattle.StatusPoison, nil
	case "TOX", "BADPOISON", "TOXIC":
		return pokebattle.StatusBadPoison, nil
	case "SLP", "SLEEP", "ASLEEP":
		return pokebattle.StatusSleep, nil
	default:
		return pokebattle.StatusNone, fmt.Errorf("unknown pokemon status %q", status)
	}
}

func seedFixturePokemonMoves(charID int64, pokemon FixturePokemon) error {
	if len(pokemon.MoveIDs) == 0 && len(pokemon.Moves) == 0 {
		return nil
	}
	ids, pps, _, err := fixtureMoveIDsAndPP(pokemon)
	if err != nil {
		return err
	}

	var pokemonRowID int64
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT id FROM character_pokemon WHERE character_id = $1 ORDER BY id DESC LIMIT 1`,
		charID,
	).Scan(&pokemonRowID); err != nil {
		return fmt.Errorf("lookup seeded pokemon row: %w", err)
	}

	if _, err := db.GlobalWorldDB.DB.Exec(`
		UPDATE character_pokemon
		SET move1_id = $1, move1_pp = $2,
		    move2_id = $3, move2_pp = $4,
		    move3_id = $5, move3_pp = $6,
		    move4_id = $7, move4_pp = $8
		WHERE id = $9`,
		ids[0], pps[0],
		ids[1], pps[1],
		ids[2], pps[2],
		ids[3], pps[3],
		pokemonRowID,
	); err != nil {
		return fmt.Errorf("update seeded pokemon moves: %w", err)
	}
	return nil
}

func applyFixturePokemonMoves(pokemon *pokebattle.Pokemon, fixture FixturePokemon) error {
	if len(fixture.MoveIDs) == 0 && len(fixture.Moves) == 0 {
		return nil
	}
	moveIDs, _, count, err := fixtureMoveIDsAndPP(fixture)
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		moveID := moveIDs[i]
		slot, err := pokebattle.LoadMoveSlotFromDB(db.GlobalWorldDB.DB, moveID)
		if err != nil {
			return err
		}
		pokemon.Moves[i] = slot
	}
	for i := count; i < len(pokemon.Moves); i++ {
		pokemon.Moves[i] = pokebattle.MoveSlot{}
	}
	return nil
}

func applyFixturePokemonDetails(pokemon *pokebattle.Pokemon, fixture FixturePokemon) error {
	recalculated := false
	if fixture.IVs != nil {
		pokemon.IVs = pokebattle.IVs{
			Attack:  fixture.IVs.Attack,
			Defense: fixture.IVs.Defense,
			Speed:   fixture.IVs.Speed,
			Special: fixture.IVs.Special,
		}
		recalculated = true
	}
	if fixture.EVs != nil {
		pokemon.EVs = pokebattle.EVs{
			HP:      fixture.EVs.HP,
			Attack:  fixture.EVs.Attack,
			Defense: fixture.EVs.Defense,
			Speed:   fixture.EVs.Speed,
			Special: fixture.EVs.Special,
		}
		recalculated = true
	}
	if fixture.Exp != nil {
		pokemon.Exp = *fixture.Exp
	}
	if recalculated {
		pokemon.RecalculateStats()
		if fixture.CurHP == nil {
			pokemon.CurHP = pokemon.MaxHP
		}
	}
	if fixture.CurHP != nil {
		pokemon.CurHP = clampInt(*fixture.CurHP, 0, pokemon.MaxHP)
	}
	if fixture.Status != "" {
		status, err := parseFixturePokemonStatus(fixture.Status)
		if err != nil {
			return err
		}
		pokemon.Status = status
	}
	for i, pp := range fixture.MovePP {
		if i >= len(pokemon.Moves) {
			return fmt.Errorf("fixture movePp slot %d is out of range", i)
		}
		if pokemon.Moves[i].ID == 0 {
			return fmt.Errorf("fixture movePp slot %d has no move", i)
		}
		pokemon.Moves[i].PP = clampInt(pp, 0, pokemon.Moves[i].MaxPP)
	}
	return nil
}

func fixtureMoveIDsAndPP(pokemon FixturePokemon) ([4]int, [4]int, int, error) {
	var ids [4]int
	var pps [4]int
	moveIDs := append([]int{}, pokemon.MoveIDs...)
	for _, moveName := range pokemon.Moves {
		moveID, err := resolveFixtureMoveID(moveName)
		if err != nil {
			return ids, pps, 0, err
		}
		moveIDs = append(moveIDs, moveID)
	}
	if len(moveIDs) > 4 {
		return ids, pps, 0, fmt.Errorf("pokemon fixture supports at most 4 moves, got %d", len(moveIDs))
	}
	for i, moveID := range moveIDs {
		pp, err := fixtureMovePP(moveID)
		if err != nil {
			return ids, pps, 0, err
		}
		ids[i] = moveID
		pps[i] = pp
	}
	return ids, pps, len(moveIDs), nil
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func resolveFixtureMoveID(moveName string) (int, error) {
	normalized := normalizeFixtureMoveName(moveName)
	if normalized == "" {
		return 0, fmt.Errorf("move name is required")
	}
	var id int
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id
		FROM phaser_moves
		WHERE REPLACE(REPLACE(UPPER(name), '-', ''), ' ', '') = $1
		   OR REPLACE(REPLACE(UPPER(short_name), '-', ''), ' ', '') = $2
		LIMIT 1`,
		normalized,
		normalized,
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("lookup move %s: %w", moveName, err)
	}
	return id, nil
}

func fixtureMovePP(moveID int) (int, error) {
	var pp int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT COALESCE(pp, 0) FROM phaser_moves WHERE id = $1`,
		moveID,
	).Scan(&pp); err != nil {
		return 0, fmt.Errorf("lookup move %d pp: %w", moveID, err)
	}
	return pp, nil
}

func moveNameForID(moveID int) (string, error) {
	var name string
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT short_name FROM phaser_moves WHERE id = $1`,
		moveID,
	).Scan(&name); err != nil {
		return "", fmt.Errorf("lookup move %d name: %w", moveID, err)
	}
	return name, nil
}

func normalizeFixtureMoveName(name string) string {
	normalized := ""
	for _, ch := range name {
		if ch >= 'a' && ch <= 'z' {
			normalized += string(ch - 32)
			continue
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			normalized += string(ch)
		}
	}
	return normalized
}
