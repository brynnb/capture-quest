package world

import (
	"database/sql"
	"fmt"
	"log"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

const dayCareSlot = 0

type DayCareStatus struct {
	Active      bool
	PokemonID   int
	Name        string
	Level       int
	StartLevel  int
	LevelsGrown int
	Cost        int
	Exp         int
}

type DayCareDepositResult struct {
	Success bool
	Message string
	Status  DayCareStatus
}

type DayCareWithdrawResult struct {
	Success      bool
	Message      string
	Money        int
	Cost         int
	PartySlot    int
	LearnedMoves []string
	Status       DayCareStatus
}

func LoadDayCareStatus(charID int64) (DayCareStatus, error) {
	return loadDayCareStatus(db.GlobalWorldDB.DB, charID)
}

func TryDepositDayCarePokemon(charID int64, partySlot int) DayCareDepositResult {
	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		return dayCareDepositFailure(charID, "Could not leave a Pokémon.")
	}
	defer tx.Rollback()

	if active, err := dayCareActive(tx, charID); err != nil {
		return dayCareDepositFailure(charID, "Could not leave a Pokémon.")
	} else if active {
		return dayCareDepositFailure(charID, "The Day Care already has a Pokémon.")
	}

	party, err := pokebattle.LoadParty(tx, charID)
	if err != nil {
		return dayCareDepositFailure(charID, "Could not read your party.")
	}
	if len(party) <= 1 {
		return dayCareDepositFailure(charID, "You have only one Pokémon.")
	}
	if partySlot < 0 || partySlot >= len(party) || party[partySlot] == nil {
		return dayCareDepositFailure(charID, "Choose a Pokémon from your party.")
	}
	pokemon := party[partySlot]
	if pokemonKnowsHMMove(tx, pokemon) {
		return dayCareDepositFailure(charID, "I can't accept a Pokémon that knows an HM move.")
	}

	var rowID int64
	var startLevel int
	if err := tx.QueryRow(`
		SELECT id, level
		FROM character_pokemon
		WHERE character_id = $1 AND box = $2 AND party_slot = $3
		FOR UPDATE`, charID, pokebattle.BoxParty, partySlot).Scan(&rowID, &startLevel); err != nil {
		return dayCareDepositFailure(charID, "Could not find that Pokémon.")
	}

	if _, err := tx.Exec(`
		UPDATE character_pokemon
		SET box = $1, box_slot = $2, party_slot = NULL
		WHERE id = $3 AND character_id = $4`,
		pokebattle.BoxDayCare, dayCareSlot, rowID, charID); err != nil {
		return dayCareDepositFailure(charID, "Could not leave that Pokémon.")
	}
	pokebattle.CompactPartySlots(tx, charID)

	if _, err := tx.Exec(`
		INSERT INTO character_daycare (character_id, pokemon_row_id, start_level)
		VALUES ($1, $2, $3)`,
		charID, rowID, startLevel); err != nil {
		return dayCareDepositFailure(charID, "Could not save Day Care state.")
	}
	if err := tx.Commit(); err != nil {
		return dayCareDepositFailure(charID, "Could not leave that Pokémon.")
	}

	status, _ := LoadDayCareStatus(charID)
	return DayCareDepositResult{
		Success: true,
		Message: "Come see me in a while.",
		Status:  status,
	}
}

func AdvanceDayCareSteps(charID int64, steps int) (DayCareStatus, bool, error) {
	if steps <= 0 {
		status, err := LoadDayCareStatus(charID)
		return status, false, err
	}
	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		return DayCareStatus{}, false, err
	}
	defer tx.Rollback()

	rowID, _, pokemon, err := loadDayCarePokemon(tx, charID, false)
	if err == sql.ErrNoRows {
		return DayCareStatus{}, false, nil
	}
	if err != nil {
		return DayCareStatus{}, false, err
	}
	oldExp := pokemon.Exp
	maxExp := pokebattle.ExpForLevel(pokemon.GrowthRt, 100)
	pokemon.Exp += steps
	if pokemon.Exp > maxExp {
		pokemon.Exp = maxExp
	}
	pokemon.Level = pokebattle.LevelForExp(pokemon.GrowthRt, pokemon.Exp)
	pokemon.RecalculateStats()
	if pokemon.CurHP > pokemon.MaxHP {
		pokemon.CurHP = pokemon.MaxHP
	}
	if err := pokebattle.SavePokemonRow(tx, rowID, pokemon); err != nil {
		return DayCareStatus{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return DayCareStatus{}, false, err
	}
	status, err := LoadDayCareStatus(charID)
	return status, pokemon.Exp != oldExp, err
}

func TryWithdrawDayCarePokemon(charID int64) DayCareWithdrawResult {
	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		return dayCareWithdrawFailure(charID, "Could not get your Pokémon.")
	}
	defer tx.Rollback()

	rowID, startLevel, pokemon, err := loadDayCarePokemon(tx, charID, true)
	if err == sql.ErrNoRows {
		return dayCareWithdrawFailure(charID, "No Pokémon is in Day Care.")
	}
	if err != nil {
		return dayCareWithdrawFailure(charID, "Could not get your Pokémon.")
	}

	partySize, err := partySizeForUpdate(tx, charID)
	if err != nil {
		return dayCareWithdrawFailure(charID, "Could not read your party.")
	}
	if partySize >= 6 {
		return dayCareWithdrawFailure(charID, "No room for Pokémon.")
	}

	level := pokebattle.LevelForExp(pokemon.GrowthRt, pokemon.Exp)
	if level > 100 {
		level = 100
	}
	levelsGrown := level - startLevel
	if levelsGrown < 0 {
		levelsGrown = 0
	}
	cost := DayCareCost(levelsGrown)
	money := dayCareMoneyForUpdate(tx, charID)
	if money < cost {
		return DayCareWithdrawResult{
			Success: false,
			Message: "Not enough money.",
			Money:   money,
			Cost:    cost,
			Status:  statusFromPokemon(pokemon, startLevel),
		}
	}

	pokemon.Level = level
	pokemon.Exp = minInt(pokemon.Exp, pokebattle.ExpForLevel(pokemon.GrowthRt, 100))
	learned, err := pokebattle.ApplyDayCareLevelUpMoves(tx, pokemon, startLevel, level)
	if err != nil {
		return dayCareWithdrawFailure(charID, "Could not update Pokémon moves.")
	}
	pokemon.RecalculateStats()
	pokemon.CurHP = pokemon.MaxHP
	if err := pokebattle.SavePokemonRow(tx, rowID, pokemon); err != nil {
		return dayCareWithdrawFailure(charID, "Could not update Pokémon.")
	}

	if _, err := tx.Exec(`
		UPDATE character_pokemon
		SET box = $1, box_slot = $2, party_slot = $3
		WHERE id = $4 AND character_id = $5`,
		pokebattle.BoxParty, partySize, partySize, rowID, charID); err != nil {
		return dayCareWithdrawFailure(charID, "Could not return Pokémon.")
	}
	if _, err := tx.Exec(`DELETE FROM character_daycare WHERE character_id = $1`, charID); err != nil {
		return dayCareWithdrawFailure(charID, "Could not clear Day Care state.")
	}
	if _, err := tx.Exec(`
		UPDATE character_wallet
		SET pokedollars = pokedollars - $1
		WHERE character_id = $2`, cost, charID); err != nil {
		return dayCareWithdrawFailure(charID, "Could not pay Day Care fee.")
	}
	if err := tx.Commit(); err != nil {
		return dayCareWithdrawFailure(charID, "Could not get your Pokémon.")
	}

	learnedNames := make([]string, 0, len(learned))
	for _, move := range learned {
		learnedNames = append(learnedNames, move.MoveName)
	}
	return DayCareWithdrawResult{
		Success:      true,
		Message:      "Here's your Pokémon.",
		Money:        money - cost,
		Cost:         cost,
		PartySlot:    partySize,
		LearnedMoves: learnedNames,
	}
}

func DayCareCost(levelsGrown int) int {
	if levelsGrown < 0 {
		levelsGrown = 0
	}
	return (levelsGrown + 1) * 100
}

func TickDayCareStep(charID int64) {
	if _, _, err := AdvanceDayCareSteps(charID, 1); err != nil {
		log.Printf("[DayCare] Failed to advance Day Care EXP for char %d: %v", charID, err)
	}
}

func dayCareDepositFailure(charID int64, message string) DayCareDepositResult {
	status, _ := LoadDayCareStatus(charID)
	return DayCareDepositResult{Success: false, Message: message, Status: status}
}

func dayCareWithdrawFailure(charID int64, message string) DayCareWithdrawResult {
	status, _ := LoadDayCareStatus(charID)
	return DayCareWithdrawResult{
		Success: false,
		Message: message,
		Money:   dayCareMoney(charID),
		Status:  status,
	}
}

func loadDayCareStatus(q pokebattle.DBTX, charID int64) (DayCareStatus, error) {
	_, startLevel, pokemon, err := loadDayCarePokemon(q, charID, false)
	if err == sql.ErrNoRows {
		return DayCareStatus{Active: false}, nil
	}
	if err != nil {
		return DayCareStatus{}, err
	}
	return statusFromPokemon(pokemon, startLevel), nil
}

func loadDayCarePokemon(q pokebattle.DBTX, charID int64, forUpdate bool) (int64, int, *pokebattle.Pokemon, error) {
	query := `SELECT pokemon_row_id, start_level FROM character_daycare WHERE character_id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var rowID int64
	var startLevel int
	if err := q.QueryRow(query, charID).Scan(&rowID, &startLevel); err != nil {
		return 0, 0, nil, err
	}
	pokemon, err := pokebattle.LoadBox(q, charID, pokebattle.BoxDayCare)
	if err != nil {
		return 0, 0, nil, err
	}
	if len(pokemon) != 1 {
		return 0, 0, nil, fmt.Errorf("expected one Day Care Pokémon for char %d, got %d", charID, len(pokemon))
	}
	return rowID, startLevel, pokemon[0], nil
}

func statusFromPokemon(pokemon *pokebattle.Pokemon, startLevel int) DayCareStatus {
	level := pokebattle.LevelForExp(pokemon.GrowthRt, pokemon.Exp)
	if level > 100 {
		level = 100
	}
	levelsGrown := level - startLevel
	if levelsGrown < 0 {
		levelsGrown = 0
	}
	return DayCareStatus{
		Active:      true,
		PokemonID:   pokemon.ID,
		Name:        pokemon.Name,
		Level:       level,
		StartLevel:  startLevel,
		LevelsGrown: levelsGrown,
		Cost:        DayCareCost(levelsGrown),
		Exp:         minInt(pokemon.Exp, pokebattle.ExpForLevel(pokemon.GrowthRt, 100)),
	}
}

func dayCareActive(q sqlQueryer, charID int64) (bool, error) {
	var exists int
	if err := q.QueryRow(
		`SELECT 1 FROM character_daycare WHERE character_id = $1`, charID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func pokemonKnowsHMMove(q pokebattle.DBTX, pokemon *pokebattle.Pokemon) bool {
	if pokemon == nil {
		return false
	}
	for _, move := range pokemon.Moves {
		if move.ID > 0 && pokebattle.IsHMMove(q, move.ID) {
			return true
		}
	}
	return false
}

func partySizeForUpdate(q pokebattle.DBTX, charID int64) (int, error) {
	rows, err := q.Query(
		`SELECT id FROM character_pokemon WHERE character_id = $1 AND box = $2 FOR UPDATE`,
		charID, pokebattle.BoxParty)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

func dayCareMoneyForUpdate(q sqlQueryer, charID int64) int {
	var money sql.NullInt64
	if err := q.QueryRow(
		`SELECT pokedollars FROM character_wallet WHERE character_id = $1 FOR UPDATE`, charID).Scan(&money); err != nil {
		return 0
	}
	if !money.Valid {
		return 0
	}
	return int(money.Int64)
}

func dayCareMoney(charID int64) int {
	var money sql.NullInt64
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT pokedollars FROM character_wallet WHERE character_id = $1`, charID).Scan(&money); err != nil {
		return 0
	}
	if !money.Valid {
		return 0
	}
	return int(money.Int64)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
