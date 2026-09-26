package pokebattle

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/db"
)

// SaveParty persists one complete party atomically while preserving row identity
// and storage metadata. New IDs are copied back only after the commit succeeds.
// Operations with other durable effects must use SavePartyInTransaction and
// publish its returned snapshot only after their outer transaction commits.
func SaveParty(database *sql.DB, characterID int64, party []*Pokemon) error {
	var saved []*Pokemon
	err := db.Transaction(context.Background(), database, func(tx db.DBTX) (err error) {
		saved, err = SavePartyInTransaction(tx, characterID, party)
		return err
	})
	if err != nil {
		return err
	}
	for i := range party {
		party[i].RowID = saved[i].RowID
	}
	return nil
}

// SavePartyInTransaction validates membership under the character lock, updates
// existing rows, and inserts new acquisitions. It never releases omitted rows.
// The caller owns the transaction and must discard the returned snapshot on any
// later failure. The supplied Pokemon objects are never mutated here.
func SavePartyInTransaction(tx db.DBTX, characterID int64, party []*Pokemon) ([]*Pokemon, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return nil, err
	}
	if len(party) > 6 {
		return nil, fmt.Errorf("party has %d pokemon, maximum is 6", len(party))
	}
	if err := db.LockCharacter(tx, characterID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT id, party_slot, box_slot FROM character_pokemon WHERE character_id=$1 AND box=$2`, characterID, BoxParty)
	if err != nil {
		return nil, err
	}
	existing := make(map[int64]bool)
	for rows.Next() {
		var id int64
		var slot, boxSlot int
		if err := rows.Scan(&id, &slot, &boxSlot); err != nil {
			rows.Close()
			return nil, err
		}
		if slot < 0 || slot >= 6 || boxSlot != slot {
			rows.Close()
			return nil, fmt.Errorf("pokemon row %d has invalid party_slot=%d box_slot=%d", id, slot, boxSlot)
		}
		existing[id] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	saved := make([]*Pokemon, len(party))
	seen := make(map[int64]bool)
	for slot, p := range party {
		if p == nil {
			return nil, fmt.Errorf("party slot %d is empty", slot)
		}
		if p.RowID != 0 {
			if !existing[p.RowID] || seen[p.RowID] {
				return nil, fmt.Errorf("pokemon row %d is not a unique member of character %d's current party", p.RowID, characterID)
			}
			seen[p.RowID] = true
		}
		copy := *p // Pokemon contains only value fields, including its move array.
		saved[slot] = &copy
	}
	if len(seen) != len(existing) {
		return nil, fmt.Errorf("party snapshot omits an existing pokemon for character %d", characterID)
	}
	// Both slot constraints are immediate. Park existing rows in disjoint negative
	// slots before applying a permutation; an ordinary swap otherwise violates
	// uniqueness even in a transaction. Readers cannot see the temporary slots.
	if _, err := tx.Exec(`UPDATE character_pokemon SET party_slot=NULL, box_slot=-box_slot-1 WHERE character_id=$1 AND box=$2`, characterID, BoxParty); err != nil {
		return nil, err
	}
	for slot, p := range saved {
		if p.RowID == 0 {
			p.RowID, err = insertStoredPokemonRow(tx, characterID, slot, BoxParty, slot, p)
		} else {
			err = SavePokemonRow(tx, p.RowID, p)
			if err == nil {
				_, err = tx.Exec(`UPDATE character_pokemon SET party_slot=$1, box_slot=$1 WHERE id=$2 AND character_id=$3`, slot, p.RowID, characterID)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("save character %d party slot %d: %w", characterID, slot, err)
		}
	}
	return saved, nil
}

// SavePokemonAfterBattle persists HP, PP, XP, level, EVs, moves, and status.
func SavePokemonAfterBattle(database *sql.DB, characterID int64, party []*Pokemon) error {
	return SaveParty(database, characterID, party)
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
	result, err := db.Exec(`
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
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("pokemon row %d does not exist", rowID)
	}
	return nil
}

func insertPokemonPCRow(database DBTX, characterID int64, box, boxSlot int, pokemon *Pokemon) error {
	_, err := insertStoredPokemonRow(database, characterID, nil, box, boxSlot, pokemon)
	return err
}

func insertPokemonRow(database DBTX, characterID int64, slot int, pokemon *Pokemon) error {
	_, err := insertStoredPokemonRow(database, characterID, slot, BoxParty, slot, pokemon)
	return err
}

func insertStoredPokemonRow(db DBTX, characterID int64, partySlot any, box, boxSlot int, pokemon *Pokemon) (int64, error) {
	var rowID int64
	err := db.QueryRow(`
		INSERT INTO character_pokemon (
			character_id, party_slot, box, box_slot, pokemon_id, nickname,
			level, exp, growth_rate, cur_hp, max_hp,
			iv_atk, iv_def, iv_spd, iv_spc,
			ev_hp, ev_atk, ev_def, ev_spd, ev_spc,
			move1_id, move1_pp, move2_id, move2_pp,
			move3_id, move3_pp, move4_id, move4_pp,
			move1_pp_up, move2_pp_up, move3_pp_up, move4_pp_up,
			status, original_trainer_id
		) VALUES ($1, $34, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33) RETURNING id`,
		characterID, box, boxSlot, pokemon.ID, pokemon.Nickname,
		pokemon.Level, pokemon.Exp, pokemon.GrowthRt.String(), pokemon.CurHP, pokemon.MaxHP,
		pokemon.IVs.Attack, pokemon.IVs.Defense, pokemon.IVs.Speed, pokemon.IVs.Special,
		pokemon.EVs.HP, pokemon.EVs.Attack, pokemon.EVs.Defense, pokemon.EVs.Speed, pokemon.EVs.Special,
		pokemon.Moves[0].ID, pokemon.Moves[0].PP, pokemon.Moves[1].ID, pokemon.Moves[1].PP,
		pokemon.Moves[2].ID, pokemon.Moves[2].PP, pokemon.Moves[3].ID, pokemon.Moves[3].PP,
		pokemon.Moves[0].PPUps, pokemon.Moves[1].PPUps, pokemon.Moves[2].PPUps, pokemon.Moves[3].PPUps,
		int(pokemon.Status), originalTrainerIDForSave(characterID, pokemon), partySlot,
	).Scan(&rowID)
	return rowID, err
}
