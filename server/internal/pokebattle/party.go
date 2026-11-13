package pokebattle

import (
	"database/sql"
	"fmt"
	"math/rand"
)

const (
	// BoxParty marks rows that are currently in the player's active party.
	BoxParty = -1
	// BoxDayCare marks the single Day Care storage slot for a character.
	BoxDayCare = -2
)

type persistedPokemonRow struct {
	dbID              int64
	slot              int
	pokemonID         int
	nickname          string
	level             int
	exp               int
	growthRate        string
	curHP             int
	maxHP             int
	ivAtk             int
	ivDef             int
	ivSpd             int
	ivSpc             int
	evHP              int
	evAtk             int
	evDef             int
	evSpd             int
	evSpc             int
	moveIDs           [4]int
	movePPs           [4]int
	movePPUps         [4]int
	status            int
	originalTrainerID sql.NullInt64
}

// LoadParty loads a player's Pokémon party from the character_pokemon table,
// fully hydrating each Pokémon with species data, moves, and computed stats.
func LoadParty(db DBTX, characterID int64) ([]*Pokemon, error) {
	rows, err := queryPersistedPokemonRows(db, `
		SELECT id, party_slot, pokemon_id, nickname, level, exp, growth_rate,
		       cur_hp, max_hp,
		       iv_atk, iv_def, iv_spd, iv_spc,
		       ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
			       move1_id, move1_pp, move2_id, move2_pp,
			       move3_id, move3_pp, move4_id, move4_pp,
			       move1_pp_up, move2_pp_up, move3_pp_up, move4_pp_up,
			       status, original_trainer_id
			FROM character_pokemon
			WHERE character_id = $1 AND box = $2
			ORDER BY party_slot ASC`, characterID, BoxParty)
	if err != nil {
		return nil, fmt.Errorf("query character_pokemon for char %d: %w", characterID, err)
	}
	var party []*Pokemon
	for _, row := range rows {
		p, err := pokemonFromPersistedRow(db, row)
		if err != nil {
			return nil, fmt.Errorf("load character %d pokemon row %d (species %d): %w", characterID, row.dbID, row.pokemonID, err)
		}
		party = append(party, p)
	}
	return party, nil
}

// LoadBox loads all Pokémon in a specific PC box for a character.
func LoadBox(db DBTX, characterID int64, box int) ([]*Pokemon, error) {
	rows, err := queryPersistedPokemonRows(db, `
		SELECT id, box_slot, pokemon_id, nickname, level, exp, growth_rate,
		       cur_hp, max_hp,
		       iv_atk, iv_def, iv_spd, iv_spc,
		       ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
			       move1_id, move1_pp, move2_id, move2_pp,
			       move3_id, move3_pp, move4_id, move4_pp,
			       move1_pp_up, move2_pp_up, move3_pp_up, move4_pp_up,
			       status, original_trainer_id
			FROM character_pokemon
			WHERE character_id = $1 AND box = $2
			ORDER BY box_slot ASC`, characterID, box)
	if err != nil {
		return nil, fmt.Errorf("query box %d for char %d: %w", box, characterID, err)
	}
	pokemon := make([]*Pokemon, 0, len(rows))
	for _, row := range rows {
		p, err := pokemonFromPersistedRow(db, row)
		if err != nil {
			return nil, fmt.Errorf("load character %d box %d pokemon row %d (species %d): %w", characterID, box, row.dbID, row.pokemonID, err)
		}
		p.BoxSlot = row.slot
		pokemon = append(pokemon, p)
	}
	return pokemon, nil
}

func queryPersistedPokemonRows(db DBTX, query string, args ...interface{}) ([]persistedPokemonRow, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pokemon []persistedPokemonRow
	for rows.Next() {
		var row persistedPokemonRow
		if err := rows.Scan(
			&row.dbID, &row.slot, &row.pokemonID, &row.nickname, &row.level, &row.exp, &row.growthRate,
			&row.curHP, &row.maxHP,
			&row.ivAtk, &row.ivDef, &row.ivSpd, &row.ivSpc,
			&row.evHP, &row.evAtk, &row.evDef, &row.evSpd, &row.evSpc,
			&row.moveIDs[0], &row.movePPs[0], &row.moveIDs[1], &row.movePPs[1],
			&row.moveIDs[2], &row.movePPs[2], &row.moveIDs[3], &row.movePPs[3],
			&row.movePPUps[0], &row.movePPUps[1], &row.movePPUps[2], &row.movePPUps[3],
			&row.status, &row.originalTrainerID,
		); err != nil {
			return nil, fmt.Errorf("scan character_pokemon row %d: %w", row.dbID, err)
		}
		pokemon = append(pokemon, row)
	}
	return pokemon, rows.Err()
}

func pokemonFromPersistedRow(db DBTX, row persistedPokemonRow) (*Pokemon, error) {
	p, err := LoadPokemonFromDB(db, row.pokemonID)
	if err != nil {
		return nil, err
	}
	p.RowID = row.dbID
	p.Nickname = row.nickname
	p.Level = row.level
	p.Exp = row.exp
	p.GrowthRt = GrowthRateFromString(row.growthRate)
	p.IsWild = false
	p.IVs = IVs{
		Attack:  row.ivAtk,
		Defense: row.ivDef,
		Speed:   row.ivSpd,
		Special: row.ivSpc,
	}
	p.EVs = EVs{
		HP:      row.evHP,
		Attack:  row.evAtk,
		Defense: row.evDef,
		Speed:   row.evSpd,
		Special: row.evSpc,
	}
	for i, mid := range row.moveIDs {
		if mid > 0 {
			slot, err := LoadMoveSlotFromDB(db, mid)
			if err != nil {
				return nil, fmt.Errorf("load move %d in slot %d for pokemon row %d: %w", mid, i, row.dbID, err)
			}
			slot.PPUps = row.movePPUps[i]
			slot.MaxPP = MaxPPWithUps(slot.MaxPP, slot.PPUps)
			slot.PP = row.movePPs[i]
			if slot.PP > slot.MaxPP {
				slot.PP = slot.MaxPP
			}
			p.Moves[i] = slot
		} else {
			p.Moves[i] = MoveSlot{}
		}
	}
	p.RecalculateStats()
	if row.curHP > p.MaxHP {
		p.CurHP = p.MaxHP
	} else {
		p.CurHP = row.curHP
	}
	p.Status = StatusCondition(row.status)
	if row.nickname != "" {
		p.Name = row.nickname
	}
	if row.originalTrainerID.Valid {
		p.OriginalTrainerID = row.originalTrainerID.Int64
	}
	return p, nil
}

// CreateStarterPokemon creates a starter Pokémon for a new character and inserts it
// into the character_pokemon table at party slot 0.
func CreateStarterPokemon(db DBTX, characterID int64, starterSpeciesID, starterLevel int) error {
	// Build the Pokémon using the same logic as wild Pokémon (random IVs, correct moves)
	p, err := BuildWildPokemon(db, starterSpeciesID, starterLevel)
	if err != nil {
		return fmt.Errorf("build starter pokemon %d L%d: %w", starterSpeciesID, starterLevel, err)
	}
	p.IsWild = false

	return withCharacterTransaction(db, characterID, func(tx DBTX) error {
		return insertPokemonRow(tx, characterID, 0, p)
	})
}

// AddPokemonToPartyOrPC creates a Pokémon and stores it in the first available
// party slot, or the player's PC if the party is full.
func AddPokemonToPartyOrPC(db DBTX, characterID int64, speciesID, level int) (addedToParty bool, pcBox int, pcSlot int, err error) {
	p, err := BuildWildPokemon(db, speciesID, level)
	if err != nil {
		return false, -1, -1, fmt.Errorf("build pokemon %d L%d: %w", speciesID, level, err)
	}
	p.IsWild = false

	return SavePreparedPokemonToPartyOrPC(db, characterID, p)
}

// SavePreparedPokemonToPartyOrPC stores an already-built Pokémon in the first
// open party slot, or in PC storage if the party is full.
func SavePreparedPokemonToPartyOrPC(database DBTX, characterID int64, p *Pokemon) (bool, int, int, error) {
	var party bool
	box, slot := -1, -1
	err := withCharacterTransaction(database, characterID, func(tx DBTX) (err error) {
		party, box, slot, err = savePreparedPokemonToPartyOrPC(tx, characterID, p)
		return err
	})
	if err != nil {
		return false, -1, -1, err
	}
	return party, box, slot, nil
}

func savePreparedPokemonToPartyOrPC(db DBTX, characterID int64, p *Pokemon) (bool, int, int, error) {
	if p == nil || p.RowID != 0 {
		return false, -1, -1, fmt.Errorf("a new, unpersisted pokemon is required")
	}

	usedSlots := make(map[int]bool)
	rows, err := db.Query(`
		SELECT party_slot
		FROM character_pokemon
		WHERE character_id = $1 AND box = -1 AND party_slot IS NOT NULL`, characterID)
	if err != nil {
		return false, -1, -1, fmt.Errorf("query party slots for char %d: %w", characterID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var slot int
		if scanErr := rows.Scan(&slot); scanErr != nil {
			return false, -1, -1, scanErr
		}
		usedSlots[slot] = true
	}
	if err := rows.Err(); err != nil {
		return false, -1, -1, err
	}

	for slot := 0; slot < 6; slot++ {
		if usedSlots[slot] {
			continue
		}
		if err := insertPokemonRow(db, characterID, slot, p); err != nil {
			return false, -1, -1, fmt.Errorf("insert party pokemon in slot %d for char %d: %w", slot, characterID, err)
		}
		return true, -1, -1, nil
	}

	box, slot, err := SavePokemonToPC(db, characterID, p)
	if err != nil {
		return false, -1, -1, err
	}
	return false, box, slot, nil
}

// SavePokemonToPC saves a caught Pokémon directly to the player's PC storage.
// It scans all 12 boxes (starting from the player's current box) for the first
// available slot. Returns (box, boxSlot, nil) on success.
func SavePokemonToPC(database DBTX, characterID int64, pokemon *Pokemon) (int, int, error) {
	box, slot := -1, -1
	err := withCharacterTransaction(database, characterID, func(tx DBTX) (err error) {
		box, slot, err = savePokemonToPC(tx, characterID, pokemon)
		return err
	})
	if err != nil {
		return -1, -1, err
	}
	return box, slot, nil
}

func savePokemonToPC(db DBTX, characterID int64, pokemon *Pokemon) (int, int, error) {
	if pokemon == nil || pokemon.RowID != 0 {
		return -1, -1, fmt.Errorf("a new, unpersisted pokemon is required")
	}
	const boxCount = 12
	const boxSize = 20

	// Get the player's current box preference (start searching from there)
	startBox := 0
	if err := db.QueryRow(`SELECT current_box FROM character_pc_state WHERE character_id = $1`, characterID).Scan(&startBox); err != nil && err != sql.ErrNoRows {
		return -1, -1, err
	}
	if startBox < 0 || startBox >= boxCount {
		return -1, -1, fmt.Errorf("character %d has invalid current_box=%d", characterID, startBox)
	}

	// Build a set of all occupied (box, box_slot) pairs
	type slot struct {
		box     int
		boxSlot int
	}
	occupied := make(map[slot]bool)
	rows, err := db.Query(`SELECT box, box_slot FROM character_pokemon WHERE character_id = $1 AND box >= 0`, characterID)
	if err != nil {
		return -1, -1, fmt.Errorf("query PC slots for char %d: %w", characterID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var b, s int
		if err := rows.Scan(&b, &s); err != nil {
			return -1, -1, err
		}
		occupied[slot{b, s}] = true
	}

	if err := rows.Err(); err != nil {
		return -1, -1, err
	}
	rows.Close()

	// Find first free slot, starting from current box
	for i := 0; i < boxCount; i++ {
		box := (startBox + i) % boxCount
		for s := 0; s < boxSize; s++ {
			if !occupied[slot{box, s}] {
				if err := insertPokemonPCRow(db, characterID, box, s, pokemon); err != nil {
					return -1, -1, fmt.Errorf("insert to PC box %d slot %d: %w", box, s, err)
				}
				return box, s, nil
			}
		}
	}

	return -1, -1, fmt.Errorf("all PC boxes are full (12×20 = 240 Pokémon)")
}

// SavePokemonToPCSlot saves a Pokémon to an exact PC box slot. It is primarily
// used by tests and tools that need deterministic PC fixture state.
func SavePokemonToPCSlot(db DBTX, characterID int64, box, boxSlot int, pokemon *Pokemon) error {
	if box < 0 || box >= 12 {
		return fmt.Errorf("invalid PC box %d", box)
	}
	if boxSlot < 0 || boxSlot >= 20 {
		return fmt.Errorf("invalid PC slot %d", boxSlot)
	}
	return SavePokemonToStorageSlot(db, characterID, box, boxSlot, pokemon)
}

// SavePokemonToStorageSlot saves a Pokémon to an explicit non-party storage
// slot. PC callers should use SavePokemonToPCSlot so PC bounds remain enforced;
// other owned storage systems such as Day Care use this lower-level helper.
func SavePokemonToStorageSlot(database DBTX, characterID int64, box, boxSlot int, pokemon *Pokemon) error {
	return withCharacterTransaction(database, characterID, func(tx DBTX) error {
		return savePokemonToStorageSlot(tx, characterID, box, boxSlot, pokemon)
	})
}

func savePokemonToStorageSlot(db DBTX, characterID int64, box, boxSlot int, pokemon *Pokemon) error {
	if boxSlot < 0 || box == BoxParty {
		return fmt.Errorf("invalid storage slot %d", boxSlot)
	}
	if pokemon == nil || pokemon.RowID != 0 {
		return fmt.Errorf("a new, unpersisted pokemon is required")
	}
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM character_pokemon WHERE character_id = $1 AND box = $2 AND box_slot = $3`,
		characterID, box, boxSlot,
	).Scan(&count); err != nil {
		return fmt.Errorf("check PC slot %d/%d: %w", box, boxSlot, err)
	}
	if count > 0 {
		return fmt.Errorf("storage box %d slot %d is occupied", box, boxSlot)
	}
	return insertPokemonPCRow(db, characterID, box, boxSlot, pokemon)
}

// HasParty returns true if the character has at least one Pokémon in their party.
func HasParty(db DBTX, characterID int64) bool {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id = $1`, characterID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// EnsureStarterExists checks if a character has a party, and if not, creates a starter.
// This is a safety net for existing characters created before Pokémon persistence was added.
func EnsureStarterExists(database DBTX, characterID int64, starterSpeciesID, starterLevel int) error {
	return withCharacterTransaction(database, characterID, func(tx DBTX) error {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id=$1`, characterID).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		return CreateStarterPokemon(tx, characterID, starterSpeciesID, starterLevel)
	})
}

// AddEVsFromDefeated adds EVs to a Pokémon based on the defeated Pokémon's base stats.
// In Gen 1, you gain EVs equal to the defeated Pokémon's base stats, capped at 65535 each.
func AddEVsFromDefeated(winner *Pokemon, defeated *Pokemon) {
	winner.EVs.HP = clampEV(winner.EVs.HP + defeated.BaseStats.HP)
	winner.EVs.Attack = clampEV(winner.EVs.Attack + defeated.BaseStats.Attack)
	winner.EVs.Defense = clampEV(winner.EVs.Defense + defeated.BaseStats.Defense)
	winner.EVs.Speed = clampEV(winner.EVs.Speed + defeated.BaseStats.Speed)
	winner.EVs.Special = clampEV(winner.EVs.Special + defeated.BaseStats.Special)
}

func clampEV(v int) int {
	if v > 65535 {
		return 65535
	}
	return v
}

// GenerateStarterIVs generates IVs for a starter Pokémon (same as wild).
func GenerateStarterIVs() IVs {
	return GenerateWildIVs(rand.Intn)
}
