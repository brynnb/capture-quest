package world

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
)

type BlackoutResult struct {
	MapID     int
	X         int
	Y         int
	OldMoney  int
	NewMoney  int
	MoneyLost int
}

func ApplyBlackoutForCharacter(charID int64) (BlackoutResult, error) {
	result := BlackoutResult{
		MapID: 41,
		X:     3,
		Y:     4,
	}
	if opts, err := db_character.LoadOptions(context.Background(), int32(charID)); err == nil {
		if opts.LastPokeCenterMapID != 0 {
			result.MapID = opts.LastPokeCenterMapID
			result.X = opts.LastPokeCenterX
			result.Y = opts.LastPokeCenterY
		}
	}

	tx, err := db.GlobalWorldDB.DB.Begin()
	if err != nil {
		return result, fmt.Errorf("begin blackout money update: %w", err)
	}
	defer tx.Rollback()

	var money sql.NullInt64
	err = tx.QueryRow(`SELECT pokedollars FROM character_wallet WHERE character_id = $1 FOR UPDATE`, charID).Scan(&money)
	if err != nil && err != sql.ErrNoRows {
		return result, fmt.Errorf("load blackout money: %w", err)
	}
	if err == nil && money.Valid {
		result.OldMoney = int(money.Int64)
	}
	result.NewMoney = result.OldMoney / 2
	result.MoneyLost = result.OldMoney - result.NewMoney
	if err == nil {
		if _, err := tx.Exec(`
			UPDATE character_wallet
			SET pokedollars = $1
			WHERE character_id = $2`, result.NewMoney, charID); err != nil {
			return result, fmt.Errorf("save blackout money: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit blackout money update: %w", err)
	}
	return result, nil
}
