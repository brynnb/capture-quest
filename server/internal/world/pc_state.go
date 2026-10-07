package world

import (
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

type PCInteractionSource struct {
	ID        int    `json:"id"`
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
}

type PCStorageSnapshot struct {
	CurrentBox int                   `json:"currentBox"`
	BoxCount   int                   `json:"boxCount"`
	BoxSize    int                   `json:"boxSize"`
	Box        []PokemonDTO          `json:"box"`
	Sources    []PCInteractionSource `json:"sources"`
}

// Read PC membership and access metadata through the same locked character
// transaction as party/revision recovery. A missing preference means the first
// box; query failures or malformed preferences never become an empty box.
func readPCStorageIn(tx db.DBTX, charID int64, mapID int) (PCStorageSnapshot, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return PCStorageSnapshot{}, err
	}
	result := PCStorageSnapshot{BoxCount: pcBoxCount, BoxSize: pcBoxSize, Box: []PokemonDTO{}, Sources: []PCInteractionSource{}}
	err := tx.QueryRow(`SELECT current_box FROM character_pc_state WHERE character_id=$1`, charID).Scan(&result.CurrentBox)
	if err != nil && err != sql.ErrNoRows {
		return PCStorageSnapshot{}, fmt.Errorf("read PC preference: %w", err)
	}
	if result.CurrentBox < 0 || result.CurrentBox >= pcBoxCount {
		return PCStorageSnapshot{}, fmt.Errorf("invalid current PC box %d for character %d", result.CurrentBox, charID)
	}
	box, err := pokebattle.LoadBox(tx, charID, result.CurrentBox)
	if err != nil {
		return PCStorageSnapshot{}, err
	}
	for _, pokemon := range box {
		if pokemon.BoxSlot < 0 || pokemon.BoxSlot >= pcBoxSize {
			return PCStorageSnapshot{}, fmt.Errorf("invalid PC slot %d for pokemon %d", pokemon.BoxSlot, pokemon.RowID)
		}
		result.Box = append(result.Box, pokemonToDTO(pokemon))
	}
	// These are original hidden-object triggers, not synthesized NPC actors.
	// Their IDs and native map coordinates must survive into command intent.
	rows, err := tx.Query(`SELECT id,map_id,x,y,item_or_direction FROM phaser_hidden_objects WHERE map_id=$1 AND routine='OpenPokemonCenterPC' AND object_type='pc' ORDER BY id`, mapID)
	if err != nil {
		return PCStorageSnapshot{}, fmt.Errorf("read PC source triggers: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var source PCInteractionSource
		var facing string
		if err := rows.Scan(&source.ID, &source.MapID, &source.X, &source.Y, &facing); err != nil {
			return PCStorageSnapshot{}, err
		}
		if source.ID <= 0 || source.X < 0 || source.Y < 0 || facing != "SPRITE_FACING_UP" {
			return PCStorageSnapshot{}, fmt.Errorf("unsupported PC source record %d: map=%d x=%d y=%d facing=%q", source.ID, source.MapID, source.X, source.Y, facing)
		}
		source.Direction = "UP"
		result.Sources = append(result.Sources, source)
	}
	if err := rows.Err(); err != nil {
		return PCStorageSnapshot{}, err
	}
	return result, nil
}
