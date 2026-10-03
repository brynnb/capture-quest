package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/pokebattle"
)

type BlackoutResult struct {
	MapID     int
	X         int
	Y         int
	OldMoney  int
	NewMoney  int
	MoneyLost int
}

// CommitStandaloneBlackout shares recovery between runtime and simulator callers.
// Its destination, wallet and party are publishable only after successful return.
func CommitStandaloneBlackout(ctx context.Context, database db.DBTX, charID int64) (BlackoutResult, []*pokebattle.Pokemon, error) {
	var party []*pokebattle.Pokemon
	var result BlackoutResult
	err := db.Transaction(ctx, database, func(tx db.DBTX) (err error) {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		result, err = applyBlackoutInTransaction(tx, charID)
		if err != nil {
			return err
		}
		party, err = pokebattle.LoadParty(tx, charID)
		if err != nil {
			return err
		}
		HealPokemonParty(party)
		party, err = pokebattle.SavePartyInTransaction(tx, charID, party)
		return err
	})
	if err != nil {
		return BlackoutResult{}, nil, err
	}
	return result, party, nil
}

func applyBlackoutInTransaction(tx db.DBTX, charID int64) (BlackoutResult, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return BlackoutResult{}, err
	}
	opts := db_character.DefaultOptions()
	var raw sql.NullString
	if err := tx.QueryRow(`SELECT options FROM character_data WHERE id=$1`, charID).Scan(&raw); err != nil {
		return BlackoutResult{}, err
	}
	if raw.Valid && raw.String != "" {
		if err := json.Unmarshal([]byte(raw.String), opts); err != nil {
			return BlackoutResult{}, fmt.Errorf("decode blackout options for character %d: %w", charID, err)
		}
	}
	defaults := db_character.DefaultOptions()
	result := BlackoutResult{MapID: defaults.LastPokeCenterMapID, X: defaults.LastPokeCenterX, Y: defaults.LastPokeCenterY}
	// Zero in an older options record means no Pokemon Center was visited yet.
	if opts.LastPokeCenterMapID != 0 {
		result.MapID, result.X, result.Y = opts.LastPokeCenterMapID, opts.LastPokeCenterX, opts.LastPokeCenterY
	}
	if _, err := tx.Exec(`INSERT INTO character_wallet(character_id,pokedollars) VALUES($1,0) ON CONFLICT(character_id) DO NOTHING`, charID); err != nil {
		return result, err
	}
	if err := tx.QueryRow(`SELECT COALESCE(pokedollars,0) FROM character_wallet WHERE character_id=$1 FOR UPDATE`, charID).Scan(&result.OldMoney); err != nil {
		return result, err
	}
	result.NewMoney = result.OldMoney / 2
	result.MoneyLost = result.OldMoney - result.NewMoney
	if _, err := tx.Exec(`UPDATE character_wallet SET pokedollars=$1 WHERE character_id=$2`, result.NewMoney, charID); err != nil {
		return result, err
	}
	if err := saveFieldDestinationIn(tx, charID, result.MapID, result.X, result.Y); err != nil {
		return BlackoutResult{}, err
	}
	return result, nil
}
