package pokebattle

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
)

const (
	// BoxParty marks rows that are currently in the player's active party.
	BoxParty = -1
	// BoxDayCare marks the single Day Care storage slot for a character.
	BoxDayCare = -2
)

type persistedPokemonRow struct {
	dbID              int
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
			log.Printf("[Party] Failed to load species %d for char pokemon %d: %v", row.pokemonID, row.dbID, err)
			continue
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
			log.Printf("[PC] Failed to load species %d: %v", row.pokemonID, err)
			continue
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
			log.Printf("[Party] Error scanning character_pokemon row: %v", err)
			continue
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
				log.Printf("[Party] Failed to load move %d: %v", mid, err)
				continue
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

// DepositToPC moves a party Pokémon to the first available slot in the given box.
// Returns the box_slot assigned, or -1 if the box is full.
func DepositToPC(db DBTX, characterID int64, partySlot int, box int) (int, error) {
	// Find first available slot in the box (0-19)
	var usedSlots []int
	rows, err := db.Query(`SELECT box_slot FROM character_pokemon WHERE character_id = $1 AND box = $2`, characterID, box)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	for rows.Next() {
		var s int
		rows.Scan(&s)
		usedSlots = append(usedSlots, s)
	}

	usedSet := make(map[int]bool)
	for _, s := range usedSlots {
		usedSet[s] = true
	}
	freeSlot := -1
	for i := 0; i < 20; i++ {
		if !usedSet[i] {
			freeSlot = i
			break
		}
	}
	if freeSlot == -1 {
		return -1, fmt.Errorf("box %d is full", box)
	}

	// Move the party Pokémon to the PC box (NULL party_slot for PC Pokémon)
	result, err := db.Exec(`
		UPDATE character_pokemon
		SET box = $1, box_slot = $2, party_slot = NULL
		WHERE character_id = $3 AND party_slot = $4 AND box = -1`,
		box, freeSlot, characterID, partySlot)
	if err != nil {
		return -1, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return -1, fmt.Errorf("no Pokémon found in party slot %d", partySlot)
	}

	// Compact remaining party slots
	compactPartySlots(db, characterID)

	return freeSlot, nil
}

// WithdrawFromPC moves a PC Pokémon to the party at the next available slot.
// Returns the party_slot assigned, or -1 if the party is full.
func WithdrawFromPC(db DBTX, characterID int64, box int, boxSlot int) (int, error) {
	// Count current party size
	var partySize int
	db.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id = $1 AND box = -1`, characterID).Scan(&partySize)
	if partySize >= 6 {
		return -1, fmt.Errorf("party is full")
	}
	partySlot, err := nextOpenPartySlot(db, characterID)
	if err != nil {
		return -1, err
	}

	// Move from PC to party (box_slot mirrors party_slot for party uniqueness)
	result, err := db.Exec(`
		UPDATE character_pokemon
		SET box = -1, box_slot = $1, party_slot = $2
		WHERE character_id = $3 AND box = $4 AND box_slot = $5`,
		partySlot, partySlot, characterID, box, boxSlot)
	if err != nil {
		return -1, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return -1, fmt.Errorf("no Pokémon found in box %d slot %d", box, boxSlot)
	}

	return partySlot, nil
}

// ReleasePokemon permanently deletes a Pokémon from PC storage.
func ReleasePokemon(db DBTX, characterID int64, box int, boxSlot int) error {
	result, err := db.Exec(`
		DELETE FROM character_pokemon
		WHERE character_id = $1 AND box = $2 AND box_slot = $3`,
		characterID, box, boxSlot)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no Pokémon found in box %d slot %d", box, boxSlot)
	}
	return nil
}

func nextOpenPartySlot(db DBTX, characterID int64) (int, error) {
	rows, err := db.Query(`
		SELECT party_slot
		FROM character_pokemon
		WHERE character_id = $1 AND box = -1 AND party_slot IS NOT NULL`, characterID)
	if err != nil {
		return -1, err
	}
	defer rows.Close()

	usedSlots := make(map[int]bool)
	for rows.Next() {
		var slot int
		if err := rows.Scan(&slot); err != nil {
			return -1, err
		}
		usedSlots[slot] = true
	}
	if err := rows.Err(); err != nil {
		return -1, err
	}
	for slot := 0; slot < 6; slot++ {
		if !usedSlots[slot] {
			return slot, nil
		}
	}
	return -1, fmt.Errorf("party is full")
}

// compactPartySlots re-numbers party slots to be contiguous (0, 1, 2, ...).
func compactPartySlots(db DBTX, characterID int64) {
	rows, err := db.Query(`
		SELECT id FROM character_pokemon
		WHERE character_id = $1 AND box = -1
		ORDER BY party_slot ASC`, characterID)
	if err != nil {
		return
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		ids = append(ids, id)
	}
	for i, id := range ids {
		db.Exec(`UPDATE character_pokemon SET party_slot = $1, box_slot = $2 WHERE id = $3`, i, i, id)
	}
}

// CompactPartySlots re-numbers active party slots to be contiguous.
func CompactPartySlots(db DBTX, characterID int64) {
	compactPartySlots(db, characterID)
}

// SaveParty writes the full party state to the database, replacing all existing party rows.
// Only deletes party Pokémon (box = -1), preserving PC box storage.
func SaveParty(db DBTX, characterID int64, party []*Pokemon) error {
	// Delete existing party only (box = -1)
	_, err := db.Exec(`DELETE FROM character_pokemon WHERE character_id = $1 AND box = -1`, characterID)
	if err != nil {
		return fmt.Errorf("delete old party for char %d: %w", characterID, err)
	}

	for slot, p := range party {
		if p == nil {
			continue
		}
		if err := insertPokemonRow(db, characterID, slot, p); err != nil {
			return fmt.Errorf("insert party slot %d for char %d: %w", slot, characterID, err)
		}
	}
	return nil
}

// SavePokemonAfterBattle persists the battle-modified state of the player's party.
// This updates HP, PP, XP, level, EVs, and status for each Pokémon.
func SavePokemonAfterBattle(db DBTX, characterID int64, party []*Pokemon) error {
	// Use SaveParty for simplicity — full replace is fine for up to 6 rows
	return SaveParty(db, characterID, party)
}

func originalTrainerIDForSave(characterID int64, p *Pokemon) int64 {
	if p != nil && p.OriginalTrainerID > 0 {
		return p.OriginalTrainerID
	}
	return characterID
}

// SavePokemonRow updates one persisted Pokémon row in place. It is used by
// storage systems such as Day Care that need to preserve the row's party/box
// location while changing level, EXP, HP, PP, EVs, IVs, moves, or status.
func SavePokemonRow(db DBTX, rowID int64, p *Pokemon) error {
	if p == nil {
		return fmt.Errorf("pokemon is required")
	}
	_, err := db.Exec(`
		UPDATE character_pokemon
		SET pokemon_id = $1,
			level = $2,
			exp = $3,
			growth_rate = $4,
			cur_hp = $5,
			max_hp = $6,
			iv_atk = $7,
			iv_def = $8,
			iv_spd = $9,
			iv_spc = $10,
			ev_hp = $11,
			ev_atk = $12,
			ev_def = $13,
			ev_spd = $14,
			ev_spc = $15,
			move1_id = $16,
			move1_pp = $17,
			move2_id = $18,
			move2_pp = $19,
			move3_id = $20,
			move3_pp = $21,
			move4_id = $22,
			move4_pp = $23,
			move1_pp_up = $24,
			move2_pp_up = $25,
			move3_pp_up = $26,
			move4_pp_up = $27,
				status = $28,
				original_trainer_id = CASE WHEN $29 > 0 THEN $30 ELSE original_trainer_id END
			WHERE id = $31`,
		p.ID,
		p.Level,
		p.Exp,
		p.GrowthRt.String(),
		p.CurHP,
		p.MaxHP,
		p.IVs.Attack,
		p.IVs.Defense,
		p.IVs.Speed,
		p.IVs.Special,
		p.EVs.HP,
		p.EVs.Attack,
		p.EVs.Defense,
		p.EVs.Speed,
		p.EVs.Special,
		p.Moves[0].ID,
		p.Moves[0].PP,
		p.Moves[1].ID,
		p.Moves[1].PP,
		p.Moves[2].ID,
		p.Moves[2].PP,
		p.Moves[3].ID,
		p.Moves[3].PP,
		p.Moves[0].PPUps,
		p.Moves[1].PPUps,
		p.Moves[2].PPUps,
		p.Moves[3].PPUps,
		int(p.Status),
		p.OriginalTrainerID,
		p.OriginalTrainerID,
		rowID,
	)
	return err
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

	return insertPokemonRow(db, characterID, 0, p)
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
func SavePreparedPokemonToPartyOrPC(db DBTX, characterID int64, p *Pokemon) (addedToParty bool, pcBox int, pcSlot int, err error) {
	if p == nil {
		return false, -1, -1, fmt.Errorf("pokemon is required")
	}
	p.IsWild = false

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
func SavePokemonToPC(db DBTX, characterID int64, pokemon *Pokemon) (int, int, error) {
	const boxCount = 12
	const boxSize = 20

	// Get the player's current box preference (start searching from there)
	startBox := 0
	db.QueryRow(`SELECT current_box FROM character_pc_state WHERE character_id = $1`, characterID).Scan(&startBox)

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
		rows.Scan(&b, &s)
		occupied[slot{b, s}] = true
	}

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
func SavePokemonToStorageSlot(db DBTX, characterID int64, box, boxSlot int, pokemon *Pokemon) error {
	if boxSlot < 0 {
		return fmt.Errorf("invalid storage slot %d", boxSlot)
	}
	if pokemon == nil {
		return fmt.Errorf("pokemon is required")
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

func insertPokemonPCRow(db DBTX, characterID int64, box, boxSlot int, pokemon *Pokemon) error {
	_, err := db.Exec(`
		INSERT INTO character_pokemon (
			character_id, party_slot, box, box_slot, pokemon_id, nickname,
			level, exp, growth_rate, cur_hp, max_hp,
			iv_atk, iv_def, iv_spd, iv_spc,
			ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
			move1_id, move1_pp, move2_id, move2_pp,
			move3_id, move3_pp, move4_id, move4_pp,
			move1_pp_up, move2_pp_up, move3_pp_up, move4_pp_up,
			status, original_trainer_id
		) VALUES ($1, NULL, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33)`,
		characterID, box, boxSlot, pokemon.ID, "",
		pokemon.Level, pokemon.Exp, pokemon.GrowthRt.String(), pokemon.CurHP, pokemon.MaxHP,
		pokemon.IVs.Attack, pokemon.IVs.Defense, pokemon.IVs.Speed, pokemon.IVs.Special,
		pokemon.EVs.HP, pokemon.EVs.Attack, pokemon.EVs.Defense, pokemon.EVs.Speed, pokemon.EVs.Special,
		pokemon.Moves[0].ID, pokemon.Moves[0].PP, pokemon.Moves[1].ID, pokemon.Moves[1].PP,
		pokemon.Moves[2].ID, pokemon.Moves[2].PP, pokemon.Moves[3].ID, pokemon.Moves[3].PP,
		pokemon.Moves[0].PPUps, pokemon.Moves[1].PPUps, pokemon.Moves[2].PPUps, pokemon.Moves[3].PPUps,
		int(pokemon.Status), originalTrainerIDForSave(characterID, pokemon),
	)
	return err
}

// insertPokemonRow inserts a single Pokémon into the character_pokemon table.
// For party Pokémon, box_slot mirrors party_slot to keep the unique index valid.
func insertPokemonRow(db DBTX, characterID int64, slot int, p *Pokemon) error {
	_, err := db.Exec(`
		INSERT INTO character_pokemon (
			character_id, party_slot, box_slot, pokemon_id, nickname,
			level, exp, growth_rate, cur_hp, max_hp,
			iv_atk, iv_def, iv_spd, iv_spc,
			ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
			move1_id, move1_pp, move2_id, move2_pp,
			move3_id, move3_pp, move4_id, move4_pp,
			move1_pp_up, move2_pp_up, move3_pp_up, move4_pp_up,
			status, original_trainer_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33)`,
		characterID, slot, slot, p.ID, "", // box_slot = slot for party uniqueness
		p.Level, p.Exp, p.GrowthRt.String(), p.CurHP, p.MaxHP,
		p.IVs.Attack, p.IVs.Defense, p.IVs.Speed, p.IVs.Special,
		p.EVs.HP, p.EVs.Attack, p.EVs.Defense, p.EVs.Speed, p.EVs.Special,
		p.Moves[0].ID, p.Moves[0].PP, p.Moves[1].ID, p.Moves[1].PP,
		p.Moves[2].ID, p.Moves[2].PP, p.Moves[3].ID, p.Moves[3].PP,
		p.Moves[0].PPUps, p.Moves[1].PPUps, p.Moves[2].PPUps, p.Moves[3].PPUps,
		int(p.Status), originalTrainerIDForSave(characterID, p),
	)
	return err
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
func EnsureStarterExists(db DBTX, characterID int64, starterSpeciesID, starterLevel int) error {
	if HasParty(db, characterID) {
		return nil
	}
	log.Printf("[Party] Character %d has no party, creating starter Pokémon (species %d, L%d)",
		characterID, starterSpeciesID, starterLevel)
	return CreateStarterPokemon(db, characterID, starterSpeciesID, starterLevel)
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
