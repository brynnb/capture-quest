package pokebattle

import (
	"context"
	"fmt"

	"capturequest/internal/db"
)

// ReorderPartyInTransaction changes ordering only. Resolve the complete owned
// set by stable row ID while locked, then reuse the authoritative party writer
// so slots, metadata and rollback keep the same rules as other party changes.
func ReorderPartyInTransaction(tx db.DBTX, characterID int64, ids []int64) ([]*Pokemon, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return nil, err
	}
	if len(ids) < 1 || len(ids) > 6 {
		return nil, fmt.Errorf("invalid party size")
	}
	if err := db.LockCharacter(tx, characterID); err != nil {
		return nil, err
	}
	party, err := LoadParty(tx, characterID)
	if err != nil {
		return nil, err
	}
	if len(party) != len(ids) {
		return nil, fmt.Errorf("party membership changed")
	}
	owned := make(map[int64]*Pokemon, len(party))
	for _, pokemon := range party {
		owned[pokemon.RowID] = pokemon
	}
	ordered := make([]*Pokemon, len(ids))
	for i, id := range ids {
		pokemon := owned[id]
		if id <= 0 || pokemon == nil {
			return nil, fmt.Errorf("pokemon row %d is not a unique current party member", id)
		}
		ordered[i] = pokemon
		delete(owned, id)
	}
	return SavePartyInTransaction(tx, characterID, ordered)
}

// All party/storage membership changes share this lock, including empty parties.
// Nested operations join the caller's transaction and propagate failures.
func withCharacterTransaction(database DBTX, characterID int64, operation func(DBTX) error) error {
	return db.Transaction(context.Background(), database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, characterID); err != nil {
			return err
		}
		return operation(tx)
	})
}

// DepositToPC moves a party Pokémon to the first available slot in the given box.
// Returns the box_slot assigned, or -1 if the box is full.
func DepositToPC(database DBTX, characterID int64, partySlot int, box int) (int, error) {
	result := -1
	err := withCharacterTransaction(database, characterID, func(tx DBTX) (err error) {
		result, err = depositToPC(tx, characterID, partySlot, box)
		return err
	})
	if err != nil {
		return -1, err
	}
	return result, nil
}

func depositToPC(db DBTX, characterID int64, partySlot int, box int) (int, error) {
	if box < 0 || box >= 12 || partySlot < 0 || partySlot >= 6 {
		return -1, fmt.Errorf("invalid party slot %d or PC box %d", partySlot, box)
	}
	var partySize int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_pokemon WHERE character_id=$1 AND box=-1`, characterID).Scan(&partySize); err != nil {
		return -1, err
	}
	if partySize <= 1 {
		return -1, fmt.Errorf("cannot deposit the last party pokemon")
	}
	// Find first available slot in the box (0-19)
	var usedSlots []int
	rows, err := db.Query(`SELECT box_slot FROM character_pokemon WHERE character_id = $1 AND box = $2`, characterID, box)
	if err != nil {
		return -1, err
	}
	defer rows.Close()
	for rows.Next() {
		var s int
		if err := rows.Scan(&s); err != nil {
			return -1, err
		}
		usedSlots = append(usedSlots, s)
	}

	if err := rows.Err(); err != nil {
		return -1, err
	}
	rows.Close()

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
	affected, err := result.RowsAffected()
	if err != nil {
		return -1, err
	}
	if affected == 0 {
		return -1, fmt.Errorf("no Pokémon found in party slot %d", partySlot)
	}

	// Compact remaining party slots
	if err := compactPartySlots(db, characterID); err != nil {
		return -1, err
	}
	return freeSlot, nil
}

// WithdrawFromPC moves a PC Pokémon to the party at the next available slot.
// Returns the party_slot assigned, or -1 if the party is full.
func WithdrawFromPC(database DBTX, characterID int64, box int, boxSlot int) (int, error) {
	result := -1
	err := withCharacterTransaction(database, characterID, func(tx DBTX) (err error) {
		result, err = withdrawFromPC(tx, characterID, box, boxSlot)
		return err
	})
	if err != nil {
		return -1, err
	}
	return result, nil
}

func withdrawFromPC(db DBTX, characterID int64, box int, boxSlot int) (int, error) {
	if err := validatePCSlot(box, boxSlot); err != nil {
		return -1, err
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
	rows, err := result.RowsAffected()
	if err != nil {
		return -1, err
	}
	if rows == 0 {
		return -1, fmt.Errorf("no Pokémon found in box %d slot %d", box, boxSlot)
	}

	return partySlot, nil
}

// ReleasePokemon permanently deletes a Pokémon from PC storage.
func ReleasePokemon(database DBTX, characterID int64, box int, boxSlot int) error {
	if err := validatePCSlot(box, boxSlot); err != nil {
		return err
	}
	return withCharacterTransaction(database, characterID, func(tx DBTX) error {
		result, err := tx.Exec(`DELETE FROM character_pokemon WHERE character_id=$1 AND box=$2 AND box_slot=$3`, characterID, box, boxSlot)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("no pokemon found in box %d slot %d", box, boxSlot)
		}
		return nil
	})
}

func validatePCSlot(box, slot int) error {
	if box < 0 || box >= 12 || slot < 0 || slot >= 20 {
		return fmt.Errorf("invalid PC box %d slot %d", box, slot)
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
func compactPartySlots(tx DBTX, characterID int64) error {
	rows, err := tx.Query(`SELECT id FROM character_pokemon WHERE character_id=$1 AND box=-1 ORDER BY party_slot ASC`, characterID)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// Compact in ascending order: each destination is either the same slot or
	// an earlier slot already vacated by the removal or preceding update.
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE character_pokemon SET party_slot=$1, box_slot=$1 WHERE id=$2`, i, id); err != nil {
			return err
		}
	}
	return nil
}

// CompactPartySlots re-numbers active party slots within the caller's transaction.
func CompactPartySlots(database DBTX, characterID int64) error {
	return withCharacterTransaction(database, characterID, func(tx DBTX) error {
		return compactPartySlots(tx, characterID)
	})
}
