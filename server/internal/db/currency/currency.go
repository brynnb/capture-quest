package db_currency

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/db"
	model "capturequest/internal/db/models"
)

// AddPokedollars adds Pokemon money to a character.
func AddPokedollars(ctx context.Context, charID int32, amount int) error {
	if amount == 0 {
		return nil
	}
	query := `
		INSERT INTO character_wallet (character_id, pokedollars)
		VALUES ($1, GREATEST($2, 0))
		ON CONFLICT (character_id) DO UPDATE SET
			pokedollars = GREATEST(character_wallet.pokedollars + $2, 0)
	`
	if _, err := db.GlobalWorldDB.DB.ExecContext(ctx, query, charID, amount); err != nil {
		return fmt.Errorf("add pokedollars for char %d: %w", charID, err)
	}
	return nil
}

// GetCharacterWallet retrieves a character's Pokemon money.
func GetCharacterWallet(ctx context.Context, charID uint32) (*model.CharacterWallet, error) {
	var wallet model.CharacterWallet
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT character_id, pokedollars
		FROM character_wallet
		WHERE character_id = $1`,
		charID).Scan(&wallet.CharacterID, &wallet.Pokedollars)
	if err == sql.ErrNoRows {
		return &model.CharacterWallet{CharacterID: charID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get wallet for char %d: %w", charID, err)
	}
	return &wallet, nil
}

// SetPokedollars sets Pokemon money to an absolute value.
func SetPokedollars(ctx context.Context, charID int32, amount int64) error {
	if amount < 0 {
		amount = 0
	}
	query := `
		INSERT INTO character_wallet (character_id, pokedollars)
		VALUES ($1, $2)
		ON CONFLICT (character_id) DO UPDATE SET
			pokedollars = EXCLUDED.pokedollars
	`
	if _, err := db.GlobalWorldDB.DB.ExecContext(ctx, query, charID, amount); err != nil {
		return fmt.Errorf("set pokedollars for char %d: %w", charID, err)
	}
	return nil
}
