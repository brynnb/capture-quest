package db_character

import (
	"context"
	"database/sql"
	"fmt"

	"capturequest/internal/cache"
	"capturequest/internal/db"
	db_currency "capturequest/internal/db/currency"
	model "capturequest/internal/db/models"
)

type rowScanner interface {
	Scan(dest ...any) error
}

const characterDataRuntimeColumns = `
	id, account_id, name, last_name, title, suffix,
	map_id, x, y, z, heading,
	gender, faction_id, class,
	birthday, last_login, time_played, gm,
	deleted_at, CAST(options AS text)
`

func scanCharacterData(scanner rowScanner) (*model.CharacterData, error) {
	var character model.CharacterData
	var deletedAt sql.NullTime
	var options sql.NullString
	if err := scanner.Scan(
		&character.ID,
		&character.AccountID,
		&character.Name,
		&character.LastName,
		&character.Title,
		&character.Suffix,
		&character.MapID,
		&character.X,
		&character.Y,
		&character.Z,
		&character.Heading,
		&character.Gender,
		&character.FactionID,
		&character.Class,
		&character.Birthday,
		&character.LastLogin,
		&character.TimePlayed,
		&character.Gm,
		&deletedAt,
		&options,
	); err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		character.DeletedAt = &deletedAt.Time
	}
	if options.Valid {
		character.Options = &options.String
	}
	return &character, nil
}

// GetCharacterByName loads character data from the database.
func GetCharacterByName(name string) (*model.CharacterData, error) {
	ctx := context.Background()
	character, err := scanCharacterData(db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT `+characterDataRuntimeColumns+`
		FROM character_data
		WHERE name = $1
		  AND deleted_at IS NULL`, name))
	if err != nil {
		return nil, fmt.Errorf("query character_data: %w", err)
	}

	return character, nil
}

// GetCharacterByID loads character data from the database by ID.
func GetCharacterByID(id int32) (*model.CharacterData, error) {
	ctx := context.Background()
	character, err := scanCharacterData(db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT `+characterDataRuntimeColumns+`
		FROM character_data
		WHERE id = $1
		  AND deleted_at IS NULL`, id))
	if err != nil {
		return nil, fmt.Errorf("query character_data by id: %w", err)
	}

	return character, nil
}

// UpdateCharacter saves character data to the database.
func UpdateCharacter(charData *model.CharacterData, accountID int64) error {
	charSelectCacheKey := fmt.Sprintf("account:characters:%d", accountID)
	cache.GetCache().Delete(charSelectCacheKey)

	if _, err := db.GlobalWorldDB.DB.Exec(`
		UPDATE character_data
		SET map_id = $1,
		    x = $2,
		    y = $3,
		    z = $4,
		    heading = $5,
		    time_played = $6,
		    last_login = $7
		WHERE id = $8`,
		charData.MapID,
		charData.X,
		charData.Y,
		charData.Z,
		charData.Heading,
		charData.TimePlayed,
		charData.LastLogin,
		charData.ID,
	); err != nil {
		return fmt.Errorf("failed to update character: %v", err)
	}
	return nil
}

// UpdateCharacterPosition updates only the position-related fields of a character.
func UpdateCharacterPosition(charID int32, mapID uint32, x, y, z, heading float64) error {
	if _, err := db.GlobalWorldDB.DB.Exec(`
		UPDATE character_data
		SET map_id = $1,
		    x = $2,
		    y = $3,
		    z = $4,
		    heading = $5
		WHERE id = $6`,
		mapID, x, y, z, heading, charID); err != nil {
		return fmt.Errorf("failed to update character position: %v", err)
	}
	return nil
}

// GetCharacterBind retrieves the character's bind point
func GetCharacterBind(ctx context.Context, charID uint32) (*model.CharacterBind, error) {
	var bind model.CharacterBind
	if err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT id, slot, map_id, x, y, z, heading
		FROM character_bind
		WHERE id = $1
		  AND slot = 0`,
		charID).Scan(
		&bind.ID,
		&bind.Slot,
		&bind.MapID,
		&bind.X,
		&bind.Y,
		&bind.Z,
		&bind.Heading,
	); err != nil {
		return nil, fmt.Errorf("query character bind: %w", err)
	}
	return &bind, nil
}

// UpdateCharacterBind updates the character's primary bind point
func UpdateCharacterBind(ctx context.Context, charID uint32, mapID uint32, x, y, z, heading float64) error {
	if _, err := db.GlobalWorldDB.DB.ExecContext(ctx, `
		INSERT INTO character_bind (id, slot, map_id, x, y, z, heading)
		VALUES ($1, 0, $2, $3, $4, $5, $6)
		ON CONFLICT (id, slot) DO UPDATE SET
			map_id = EXCLUDED.map_id,
			x = EXCLUDED.x,
			y = EXCLUDED.y,
			z = EXCLUDED.z,
			heading = EXCLUDED.heading`,
		charID, mapID, x, y, z, heading); err != nil {
		return fmt.Errorf("update character bind: %w", err)
	}
	return nil
}

// AddPokedollars adds Pokemon money.
func AddPokedollars(ctx context.Context, charID int32, amount int) error {
	return db_currency.AddPokedollars(ctx, charID, amount)
}

// GetCharacterWallet retrieves a character's Pokemon money.
func GetCharacterWallet(ctx context.Context, charID uint32) (*model.CharacterWallet, error) {
	return db_currency.GetCharacterWallet(ctx, charID)
}

// SetPokedollars sets Pokemon money.
func SetPokedollars(ctx context.Context, charID int32, amount int64) error {
	return db_currency.SetPokedollars(ctx, charID, amount)
}
