package pokebattle

import (
	"database/sql"
	"fmt"
	"math/rand"
)

// DBTX is the minimal query interface used by SQL DBs and transactions.
type DBTX interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
}

// HighCritMoves is the set of moves with high critical hit ratios in Gen 1.
var HighCritMoves = map[string]bool{
	"KARATE_CHOP": true,
	"RAZOR_LEAF":  true,
	"CRABHAMMER":  true,
	"SLASH":       true,
}

// LoadPokemonFromDB loads a Pokémon species from the database by ID.
func LoadPokemonFromDB(db DBTX, pokemonID int) (*Pokemon, error) {
	var name, type1 string
	var type2 sql.NullString
	var hp, atk, def, spd, spc, catchRate, baseExp int
	var growthRate string
	var dm1, dm2, dm3, dm4 sql.NullString
	var baseCry, cryPitch, cryLength sql.NullInt64
	var evolveLevel sql.NullInt64
	var evolvePokemon sql.NullString

	err := db.QueryRow(`
		SELECT name, type_1, type_2, hp, atk, def, spd, spc, catch_rate, base_exp, growth_rate,
		       default_move_1_id, default_move_2_id, default_move_3_id, default_move_4_id,
		       base_cry, cry_pitch, cry_length, evolve_level, evolve_pokemon
		FROM phaser_pokemon WHERE id = $1`, pokemonID).Scan(
		&name, &type1, &type2, &hp, &atk, &def, &spd, &spc, &catchRate, &baseExp, &growthRate,
		&dm1, &dm2, &dm3, &dm4,
		&baseCry, &cryPitch, &cryLength, &evolveLevel, &evolvePokemon,
	)
	if err != nil {
		return nil, fmt.Errorf("load pokemon %d: %w", pokemonID, err)
	}

	t1, _ := TypeFromString(type1)
	t2 := t1
	if type2.Valid && type2.String != "" {
		if parsed, ok := TypeFromString(type2.String); ok {
			t2 = parsed
		}
	}

	p := &Pokemon{
		ID:    pokemonID,
		Name:  name,
		Type1: t1,
		Type2: t2,
		BaseStats: BaseStats{
			HP:      hp,
			Attack:  atk,
			Defense: def,
			Special: spc,
			Speed:   spd,
		},
		CatchRate: catchRate,
		BaseSpeed: spd,
		BaseExp:   baseExp,
		GrowthRt:  GrowthRateFromString(growthRate),
	}

	if baseCry.Valid {
		p.CrySFX = fmt.Sprintf("SFX_CRY_%02X", baseCry.Int64)
	}
	if cryPitch.Valid {
		p.CryPitch = int(cryPitch.Int64)
	}
	if cryLength.Valid {
		p.CryLength = int(cryLength.Int64)
	}

	if evolveLevel.Valid {
		p.EvolveLevel = int(evolveLevel.Int64)
	}
	if evolvePokemon.Valid {
		p.EvolvePokemonName = evolvePokemon.String
	}

	// Resolve default move names to IDs
	defaultMoveNames := [4]string{}
	if dm1.Valid && dm1.String != "NO_MOVE" {
		defaultMoveNames[0] = dm1.String
	}
	if dm2.Valid && dm2.String != "NO_MOVE" {
		defaultMoveNames[1] = dm2.String
	}
	if dm3.Valid && dm3.String != "NO_MOVE" {
		defaultMoveNames[2] = dm3.String
	}
	if dm4.Valid && dm4.String != "NO_MOVE" {
		defaultMoveNames[3] = dm4.String
	}

	// Load default move IDs by name
	for i, mname := range defaultMoveNames {
		if mname == "" {
			continue
		}
		var moveID int
		err := db.QueryRow(`SELECT id FROM phaser_moves WHERE name = $1`, mname).Scan(&moveID)
		if err == nil {
			p.Moves[i], _ = LoadMoveSlotFromDB(db, moveID)
		}
	}

	return p, nil
}

// LoadMoveSlotFromDB loads a single move from the database.
func LoadMoveSlotFromDB(db DBTX, moveID int) (MoveSlot, error) {
	var name, shortName string
	var moveType sql.NullString
	var power, accuracy, pp sql.NullInt64
	var effect, battleSFX sql.NullString
	var battleSFXPitch, battleSFXTempo sql.NullInt64

	err := db.QueryRow(`
		SELECT name, short_name, type, power, accuracy, pp, effect,
		       battle_sound, battle_sound_pitch, battle_sound_tempo
		FROM phaser_moves WHERE id = $1`, moveID).Scan(
		&name, &shortName, &moveType, &power, &accuracy, &pp, &effect,
		&battleSFX, &battleSFXPitch, &battleSFXTempo,
	)
	if err != nil {
		return MoveSlot{}, fmt.Errorf("load move %d: %w", moveID, err)
	}

	mt := TypeNormal
	if moveType.Valid {
		if parsed, ok := TypeFromString(moveType.String); ok {
			mt = parsed
		}
	}

	slot := MoveSlot{
		ID:         moveID,
		Name:       shortName,
		Type:       mt,
		Power:      int(power.Int64),
		Accuracy:   int(accuracy.Int64),
		PP:         int(pp.Int64),
		MaxPP:      int(pp.Int64),
		BasePP:     int(pp.Int64),
		IsHighCrit: HighCritMoves[name],
	}
	if effect.Valid {
		slot.Effect = effect.String
	}
	if battleSFX.Valid && battleSFX.String != "NO_SOUND" {
		slot.BattleSFX = battleSFX.String
	}
	if battleSFXPitch.Valid {
		slot.SFXPitch = int(battleSFXPitch.Int64)
	}
	if battleSFXTempo.Valid {
		slot.SFXTempo = int(battleSFXTempo.Int64)
	}

	return slot, nil
}

// BuildWildPokemon creates a fully initialized wild Pokémon at a given level,
// with the correct moves for that level (default moves + learnset).
func BuildWildPokemon(db DBTX, pokemonID, level int) (*Pokemon, error) {
	p, err := LoadPokemonFromDB(db, pokemonID)
	if err != nil {
		return nil, err
	}
	p.Level = level
	p.IsWild = true
	p.Exp = ExpForLevel(p.GrowthRt, level)

	// Get default move IDs (from the moves already loaded)
	var defaultMoveIDs [4]int
	for i, m := range p.Moves {
		defaultMoveIDs[i] = m.ID
	}

	// Load learnset
	learnset, err := loadLearnset(db, pokemonID)
	if err != nil {
		return nil, err
	}

	// Determine which moves the Pokémon knows at this level
	moveIDs := GetMovesForLevel(level, defaultMoveIDs, learnset)

	// Load the actual move data
	for i, mid := range moveIDs {
		if mid > 0 {
			slot, err := LoadMoveSlotFromDB(db, mid)
			if err == nil {
				p.Moves[i] = slot
			}
		} else {
			p.Moves[i] = MoveSlot{}
		}
	}

	// Generate random IVs and calculate stats
	p.IVs = GenerateWildIVs(rand.Intn)
	p.EVs = EVs{} // Wild Pokémon have 0 EVs

	p.RecalculateStats()
	p.CurHP = p.MaxHP

	return p, nil
}

// GetMovesLearnedInRange returns all moves a Pokémon learns between (oldLevel, newLevel] inclusive.
// Each entry is (moveID, moveName). Used to check for new moves on level-up.
func GetMovesLearnedInRange(db DBTX, pokemonID, oldLevel, newLevel int) ([]LearnedMove, error) {
	rows, err := db.Query(`
		SELECT move_id, move_name FROM phaser_pokemon_learnset
		WHERE pokemon_id = $1 AND level > $2 AND level <= $3
		ORDER BY level ASC`, pokemonID, oldLevel, newLevel)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var moves []LearnedMove
	for rows.Next() {
		var m LearnedMove
		if err := rows.Scan(&m.MoveID, &m.MoveName); err != nil {
			continue
		}
		moves = append(moves, m)
	}
	return moves, nil
}

// LearnedMove represents a move learned from the learnset.
type LearnedMove struct {
	MoveID   int
	MoveName string
}

// TryLearnMove attempts to teach a move to a Pokémon.
// If the Pokémon has an empty slot, the move is learned automatically and the slot index is returned.
// If all 4 slots are full, returns -1 (caller must prompt the player).
// If the Pokémon already knows the move, returns -2 (skip).
func TryLearnMove(db DBTX, p *Pokemon, moveID int) int {
	// Check if already known
	for _, m := range p.Moves {
		if m.ID == moveID {
			return -2
		}
	}

	// Find empty slot
	for i, m := range p.Moves {
		if m.ID == 0 {
			slot, err := LoadMoveSlotFromDB(db, moveID)
			if err != nil {
				return -2 // Can't load move, skip
			}
			p.Moves[i] = slot
			return i
		}
	}

	// All slots full — caller must prompt
	return -1
}

// ApplyDayCareLevelUpMoves applies Gen 1 Day Care move learning for the levels
// gained while the Pokémon was stored. Day Care never prompts the player: when
// all four move slots are full, the oldest move is forgotten and the new move is
// placed in slot 4.
func ApplyDayCareLevelUpMoves(db DBTX, p *Pokemon, startLevel, endLevel int) ([]LearnedMove, error) {
	if p == nil {
		return nil, fmt.Errorf("pokemon is required")
	}
	if endLevel <= startLevel {
		return nil, nil
	}
	moves, err := GetMovesLearnedInRange(db, p.ID, startLevel, endLevel)
	if err != nil {
		return nil, err
	}
	learned := make([]LearnedMove, 0, len(moves))
	for _, move := range moves {
		if pokemonKnowsMove(p, move.MoveID) {
			continue
		}
		slot, err := LoadMoveSlotFromDB(db, move.MoveID)
		if err != nil {
			return learned, err
		}
		if empty := firstEmptyMoveSlot(p); empty >= 0 {
			p.Moves[empty] = slot
		} else {
			p.Moves[0] = p.Moves[1]
			p.Moves[1] = p.Moves[2]
			p.Moves[2] = p.Moves[3]
			p.Moves[3] = slot
		}
		learned = append(learned, move)
	}
	return learned, nil
}

func pokemonKnowsMove(p *Pokemon, moveID int) bool {
	for _, move := range p.Moves {
		if move.ID == moveID {
			return true
		}
	}
	return false
}

func firstEmptyMoveSlot(p *Pokemon) int {
	for i, move := range p.Moves {
		if move.ID == 0 {
			return i
		}
	}
	return -1
}

// ForgetAndLearnMove replaces a move at the given slot with a new move.
func ForgetAndLearnMove(db DBTX, p *Pokemon, forgetSlot, newMoveID int) error {
	if forgetSlot < 0 || forgetSlot >= 4 {
		return fmt.Errorf("invalid slot %d", forgetSlot)
	}
	slot, err := LoadMoveSlotFromDB(db, newMoveID)
	if err != nil {
		return err
	}
	p.Moves[forgetSlot] = slot
	return nil
}

// CanLearnTMHM checks if a Pokémon can learn a move via TM/HM.
// Returns true if the pokemon_id + move_id combination exists in phaser_pokemon_tmhm.
func CanLearnTMHM(db DBTX, pokemonID, moveID int) bool {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM phaser_pokemon_tmhm
		WHERE pokemon_id = $1 AND move_id = $2`, pokemonID, moveID).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

// IsHMMove checks if a move is an HM move (can't be forgotten).
func IsHMMove(db DBTX, moveID int) bool {
	var isHM bool
	err := db.QueryRow(`
		SELECT is_hm FROM phaser_pokemon_tmhm
		WHERE move_id = $1 LIMIT 1`, moveID).Scan(&isHM)
	if err != nil {
		return false
	}
	return isHM
}

func loadLearnset(db DBTX, pokemonID int) ([][2]int, error) {
	rows, err := db.Query(`
		SELECT level, move_id FROM phaser_pokemon_learnset
		WHERE pokemon_id = $1 ORDER BY level ASC`, pokemonID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var learnset [][2]int
	for rows.Next() {
		var level, moveID int
		if err := rows.Scan(&level, &moveID); err != nil {
			continue
		}
		learnset = append(learnset, [2]int{level, moveID})
	}
	return learnset, nil
}

// SelectWildEncounter picks a random wild Pokémon for a map based on encounter data.
// Returns pokemonID and level, or an error if no encounters exist.
// encounterType should be "grass" or "water".
func SelectWildEncounter(db DBTX, mapID int, encounterType string) (pokemonID, level int, err error) {
	// Join wild encounters with encounter slots (by slot_index) and pokemon table (by name→id).
	// The encounter_slots table has 10 slots with percentage probabilities.
	// Wild encounters have slot_index 1-10 per version; we pick Red version slots (1-10).
	rows, err := db.Query(`
		SELECT pp.id, we.level, es.probability
		FROM phaser_wild_encounters we
		JOIN phaser_encounter_slots es ON we.slot_index = es.slot_index
		JOIN phaser_pokemon pp ON we.pokemon_name = pp.name
		WHERE we.map_id = $1 AND we.encounter_type = $2
		  AND we.slot_index <= 10
		  AND (we.version = 'red' OR we.version = 'both')
		ORDER BY es.slot_index ASC`, mapID, encounterType)
	if err != nil {
		return 0, 0, fmt.Errorf("query wild encounters for map %d: %w", mapID, err)
	}
	defer rows.Close()

	type encounter struct {
		pokemonID   int
		level       int
		probability float64
	}
	var encounters []encounter
	for rows.Next() {
		var e encounter
		if err := rows.Scan(&e.pokemonID, &e.level, &e.probability); err != nil {
			continue
		}
		encounters = append(encounters, e)
	}

	if len(encounters) == 0 {
		return 0, 0, fmt.Errorf("no wild encounters for map %d (type %s)", mapID, encounterType)
	}

	// Roll based on cumulative probability (percentages that sum to ~100)
	roll := rand.Float64() * 100.0
	cumulative := 0.0
	for _, e := range encounters {
		cumulative += e.probability
		if roll < cumulative {
			return e.pokemonID, e.level, nil
		}
	}

	// Fallback to last entry
	last := encounters[len(encounters)-1]
	return last.pokemonID, last.level, nil
}

// SelectFishingEncounter picks a random Pokémon for a fishing rod on a given map.
// rodType: "old_rod", "good_rod", or "super_rod".
// Old Rod always returns Magikarp L5 (hardcoded, Gen 1 behavior).
// Good Rod uses a global table (map_id IS NULL).
// Super Rod uses map-specific tables with slot probability weighting.
func SelectFishingEncounter(db DBTX, mapID int, rodType string) (pokemonID, level int, err error) {
	// Old Rod: always Magikarp L5
	if rodType == "old_rod" {
		var magikarpID int
		err := db.QueryRow(`SELECT id FROM phaser_pokemon WHERE name = 'MAGIKARP'`).Scan(&magikarpID)
		if err != nil {
			return 0, 0, fmt.Errorf("magikarp not found: %w", err)
		}
		return magikarpID, 5, nil
	}

	// Good Rod: global table (map_id IS NULL), equal probability
	if rodType == "good_rod" {
		rows, err := db.Query(`
			SELECT pp.id, we.level
			FROM phaser_wild_encounters we
			JOIN phaser_pokemon pp ON we.pokemon_name = pp.name
			WHERE we.encounter_type = 'good_rod' AND we.map_id IS NULL
			  AND (we.version = 'red' OR we.version = 'both')
			ORDER BY we.slot_index ASC`)
		if err != nil {
			return 0, 0, fmt.Errorf("query good_rod encounters: %w", err)
		}
		defer rows.Close()

		type enc struct {
			pokemonID int
			level     int
		}
		var encounters []enc
		for rows.Next() {
			var e enc
			if err := rows.Scan(&e.pokemonID, &e.level); err != nil {
				continue
			}
			encounters = append(encounters, e)
		}
		if len(encounters) == 0 {
			return 0, 0, fmt.Errorf("no good_rod encounters")
		}
		pick := encounters[rand.Intn(len(encounters))]
		return pick.pokemonID, pick.level, nil
	}

	// Super Rod: map-specific, uses encounter_slots probability weighting
	return SelectWildEncounter(db, mapID, "super_rod")
}

// GetEncounterRate returns the encounter rate for a map and encounter type.
// Returns 0 if no encounters exist for this map/type.
// The encounter rate is out of 256 (Gen 1 style): each step, roll rand(256) < rate.
func GetEncounterRate(db DBTX, mapID int, encounterType string) int {
	var rate int
	err := db.QueryRow(`
		SELECT DISTINCT encounter_rate
		FROM phaser_wild_encounters
		WHERE map_id = $1 AND encounter_type = $2
		LIMIT 1`, mapID, encounterType).Scan(&rate)
	if err != nil {
		return 0
	}
	return rate
}
