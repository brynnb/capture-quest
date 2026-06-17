package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"capturequest/internal/config"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/session"

	"capturequest/internal/cache"
	"capturequest/internal/db"

	model "capturequest/internal/db/models"

	"github.com/google/uuid"
)

func cachedInt64(cacheKey string) (int64, bool) {
	val, found, err := cache.GetCache().Get(cacheKey)
	if err != nil || !found {
		return 0, false
	}
	switch v := val.(type) {
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case int:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v <= uint64(^uint(0)>>1) {
			return int64(v), true
		}
	}
	return 0, false
}

func isNoRows(err error) bool {
	return err == sql.ErrNoRows
}

func LoginIP(ctx context.Context, accountID int64, ip string) error {
	if _, err := db.GlobalWorldDB.DB.ExecContext(ctx, `
		INSERT INTO account_ip (accid, ip, count, lastused)
		VALUES ($1, $2, 1, CURRENT_TIMESTAMP)
		ON CONFLICT (accid, ip) DO UPDATE SET
			count = account_ip.count + 1,
			lastused = CURRENT_TIMESTAMP`,
		accountID, ip); err != nil {
		return fmt.Errorf("log account IP (accid=%d, ip=%s): %w", accountID, ip, err)
	}
	return nil
}

func GetVariable(ctx context.Context, name string) (model.Variables, error) {
	cacheKey := fmt.Sprintf("variables:%s", name)
	if val, found, err := cache.GetCache().Get(cacheKey); err == nil && found {
		if variable, ok := val.(model.Variables); ok {
			return variable, nil
		}
	}
	var variable model.Variables
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT id, varname, value, information, ts
		FROM variables
		WHERE varname = $1`, name).Scan(
		&variable.ID,
		&variable.Varname,
		&variable.Value,
		&variable.Information,
		&variable.Ts,
	)

	if err == nil {
		cache.GetCache().Set(cacheKey, variable)
		return variable, nil
	}
	return model.Variables{}, fmt.Errorf("GetVariable err: %w", err)
}

func GetOrCreateAccount(ctx context.Context, discordID string) (int64, error) {
	cacheKey := fmt.Sprintf("account:discord:%s", discordID)
	if accountID, ok := cachedInt64(cacheKey); ok {
		return accountID, nil
	}

	var id int64
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT id
		FROM account
		WHERE discord_id = $1`, discordID).Scan(&id)

	if err == nil {
		cache.GetCache().Set(cacheKey, id)
		return id, nil
	}
	if !isNoRows(err) {
		return 0, fmt.Errorf("lookup account by discord id: %w", err)
	}

	createdAt := time.Now().Unix()
	err = db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		INSERT INTO account (discord_id, name, time_creation)
		VALUES ($1, $2, $3)
		RETURNING id`,
		discordID, discordID, createdAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert account: %w", err)
	}
	cache.GetCache().Set(cacheKey, id)
	return id, nil
}

// GetOrCreateGuestAccount looks up or creates an account for a browser-specific guest token.
// Each unique guest token (UUID) gets its own account, allowing guests to persist across sessions.
func GetOrCreateGuestAccount(ctx context.Context, guestToken string) (int64, error) {
	cacheKey := fmt.Sprintf("account:guest:%s", guestToken)

	if accountID, ok := cachedInt64(cacheKey); ok {
		return accountID, nil
	}

	// Look up existing guest account by browser token.
	var id int64
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT id
		FROM account
		WHERE guest_token = $1`, guestToken).Scan(&id)

	if err == nil {
		cache.GetCache().Set(cacheKey, id)
		return id, nil
	}
	if !isNoRows(err) {
		return 0, fmt.Errorf("lookup guest account: %w", err)
	}

	guestName := "Guest_" + shortTokenLabel(guestToken)
	createdAt := time.Now().Unix()
	err = db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		INSERT INTO account (name, guest_token, discord_id, time_creation)
		VALUES ($1, $2, NULL, $3)
		RETURNING id`,
		guestName, guestToken, createdAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create guest account: %w", err)
	}

	log.Printf("Created new guest account %d for token %s", id, shortTokenLabel(guestToken))
	cache.GetCache().Set(cacheKey, id)
	return id, nil
}

// LoginLocalAccount authenticates a user by email/password for local development mode.
// Returns the account ID on success.
func LoginLocalAccount(ctx context.Context, email, password string) (int64, error) {
	var id int64
	var passwordHash string
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT id, password
		FROM account
		WHERE name = $1`, email).Scan(&id, &passwordHash)

	if err != nil {
		return 0, fmt.Errorf("Invalid email or password")
	}

	if err := verifyAccountPassword(passwordHash, password); err != nil {
		return 0, fmt.Errorf("Invalid email or password")
	}

	return id, nil
}

func shortTokenLabel(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8]
}

// RegisterLocalAccount creates a new account with the given email and password.
// Returns the new account ID on success.
// If guestToken is provided, the browser guest account's characters move to it.
func RegisterLocalAccount(ctx context.Context, email, password, guestToken string) (int64, error) {
	// Check if email already exists
	var existingID int64
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT id
		FROM account
		WHERE name = $1`, email).Scan(&existingID)
	switch {
	case err == nil:
		return 0, fmt.Errorf("email already taken")
	case !isNoRows(err):
		return 0, fmt.Errorf("check existing account: %w", err)
	}

	passwordHash, err := hashAccountPassword(password)
	if err != nil {
		return 0, fmt.Errorf("failed to secure password")
	}

	// Create the new account
	createdAt := time.Now().Unix()
	var id int64
	err = db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		INSERT INTO account (name, password, discord_id, guest_token, time_creation)
		VALUES ($1, $2, NULL, NULL, $3)
		RETURNING id`,
		email, passwordHash, createdAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to create account: %w", err)
	}

	if guestToken != "" {
		guestAccountID, err := lookupGuestAccountID(ctx, guestToken)
		if err != nil {
			log.Printf("Failed to look up guest account for token %s: %v", shortTokenLabel(guestToken), err)
		} else if guestAccountID != 0 && guestAccountID != id {
			if err := TransferGuestCharactersToAccount(ctx, guestAccountID, id); err != nil {
				log.Printf("Failed to transfer guest characters from account %d to %d: %v", guestAccountID, id, err)
			}
		}
	}

	return id, nil
}

func lookupGuestAccountID(ctx context.Context, guestToken string) (int64, error) {
	cacheKey := fmt.Sprintf("account:guest:%s", guestToken)
	if accountID, ok := cachedInt64(cacheKey); ok {
		return accountID, nil
	}

	var id int64
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT id
		FROM account
		WHERE guest_token = $1`, guestToken).Scan(&id)
	if err == nil {
		cache.GetCache().Set(cacheKey, id)
		return id, nil
	}
	if isNoRows(err) {
		return 0, nil
	}
	return 0, fmt.Errorf("lookup guest account: %w", err)
}

// TransferGuestCharactersToAccount transfers active characters from a guest
// account to a newly registered account, provided the target has no characters.
func TransferGuestCharactersToAccount(ctx context.Context, guestAccountID, targetAccountID int64) error {
	if guestAccountID == 0 || targetAccountID == 0 || guestAccountID == targetAccountID {
		return nil
	}

	// Check if target account has any characters
	targetCharCount, err := CountAccountCharacters(ctx, targetAccountID)
	if err != nil {
		return fmt.Errorf("failed to count target account characters: %w", err)
	}
	if targetCharCount > 0 {
		// Target already has characters, don't transfer
		return nil
	}

	// Check if guest account has any characters
	guestCharCount, err := CountAccountCharacters(ctx, guestAccountID)
	if err != nil {
		return fmt.Errorf("failed to count guest account characters: %w", err)
	}
	if guestCharCount == 0 {
		// No guest characters to transfer
		return nil
	}

	// Transfer all guest characters to the new account
	result, err := db.GlobalWorldDB.DB.ExecContext(ctx, `
		UPDATE character_data
		SET account_id = $1
		WHERE account_id = $2
		  AND deleted_at IS NULL`,
		targetAccountID, guestAccountID)
	if err != nil {
		return fmt.Errorf("failed to transfer characters: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("Transferred %d guest characters from account %d to account %d", rowsAffected, guestAccountID, targetAccountID)

	// Invalidate caches for both accounts
	cache.GetCache().Delete(fmt.Sprintf("account:characters:%d", guestAccountID))
	cache.GetCache().Delete(fmt.Sprintf("account:characters:%d", targetAccountID))

	return nil
}

// CountAccountCharacters returns the number of active (non-deleted) characters for an account.
func CountAccountCharacters(ctx context.Context, accountID int64) (int, error) {
	var count int
	if err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM character_data
		WHERE account_id = $1
		  AND deleted_at IS NULL`,
		accountID).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count characters: %w", err)
	}

	return count, nil
}

func AccountHasCharacterName(ctx context.Context, accountID int64, charName string) (bool, error) {
	// Never cache this - always keep up to date
	var exists bool
	if err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM character_data
			WHERE account_id = $1
			  AND name = $2
			  AND deleted_at IS NULL
		)`,
		accountID, charName).Scan(&exists); err != nil {
		return false, fmt.Errorf("query character_data: %w", err)
	}

	return exists, nil
}

func GetCharSelectInfo(ses *session.Session, ctx context.Context, accountID int64) (map[string]interface{}, error) {

	const limit = 8

	var chars []model.CharacterData
	// Select only the runtime character-list columns instead of AllColumns,
	// which may expect fields outside the current CaptureQuest schema.
	rows, err := db.GlobalWorldDB.DB.QueryContext(ctx, `
		SELECT id, name, map_id, gender, faction_id, class, last_login
		FROM character_data
		WHERE account_id = $1
		  AND deleted_at IS NULL
		ORDER BY last_login DESC
		LIMIT $2`,
		accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("query character_data: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c model.CharacterData
		if err := rows.Scan(
			&c.ID,
			&c.Name,
			&c.MapID,
			&c.Gender,
			&c.FactionID,
			&c.Class,
			&c.LastLogin,
		); err != nil {
			return nil, fmt.Errorf("scan character select row: %w", err)
		}
		chars = append(chars, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read character select rows: %w", err)
	}

	log.Printf("GetCharSelectInfo: Found %d characters for account %d", len(chars), accountID)

	jsonChars := make([]map[string]interface{}, 0, len(chars))

	for _, c := range chars {
		jsonChars = append(jsonChars, characterSelectEntryFromData(c))
	}

	return map[string]interface{}{
		"characterCount": int32(len(chars)),
		"characters":     jsonChars,
	}, nil
}

func characterSelectEntryFromData(c model.CharacterData) map[string]interface{} {
	return map[string]interface{}{
		"id":        c.ID,
		"name":      c.Name,
		"factionId": c.FactionID,
		"class":     c.Class,
		"mapId":     c.MapID,
		"gender":    c.Gender,
		"lastLogin": c.LastLogin,
	}
}

func GetOrCreateCharacterID(ctx context.Context, accountId int64, profile *CharacterCreateProfile) (int64, bool, error) {
	name := profile.Name
	cacheKey := fmt.Sprintf("account:character:%d:%s", accountId, name)
	if characterID, ok := cachedInt64(cacheKey); ok {
		return characterID, false, nil
	}

	var id int64
	err := db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		SELECT id
		FROM character_data
		WHERE account_id = $1
		  AND name = $2
		  AND deleted_at IS NULL`,
		accountId, name).Scan(&id)

	if err == nil {
		cache.GetCache().Set(cacheKey, id)
		return id, false, nil
	}
	if !isNoRows(err) {
		return 0, false, fmt.Errorf("lookup character: %w", err)
	}

	gmLevel := 0
	serverConfig, _ := config.Get()
	if serverConfig.Local && accountId == 1 {
		gmLevel = 1
	}

	now := time.Now().Unix()
	err = db.GlobalWorldDB.DB.QueryRowContext(ctx, `
		INSERT INTO character_data (account_id, name, gm, birthday, last_login)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		accountId, name, gmLevel, now, now).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("insert character: %w", err)
	}
	cache.GetCache().Set(cacheKey, id)
	return id, true, nil
}

// SaveCharacterCreate saves the character creation data to the database
func SaveCharacterCreate(ctx context.Context, accountID int64, profile *CharacterCreateProfile) bool {
	// Get or create character ID
	charID, _, err := GetOrCreateCharacterID(ctx, accountID, profile)
	if err != nil {
		log.Printf("Failed to get or create character ID for %d: %v", accountID, err)
		return false
	}
	name := profile.Name
	options := profile.Options
	if options == nil {
		options = db_character.DefaultOptions()
	}
	options.RivalName = db_character.NormalizeRivalName(options.RivalName)
	optionsJSON, err := json.Marshal(options)
	if err != nil {
		log.Printf("[DB] Character creation FAILED at options JSON for %s: %v", name, err)
		return false
	}

	// Start a transaction
	tx, err := db.GlobalWorldDB.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Failed to start transaction: %v", err)
		return false
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE character_data
		SET map_id = $1,
		    x = $2,
		    y = $3,
		    z = $4,
		    heading = $5,
		    faction_id = $6,
		    class = $7,
		    gender = $8,
		    last_login = $9,
		    birthday = $10,
		    options = $11::jsonb
		WHERE id = $12`,
		profile.MapID,
		profile.X,
		profile.Y,
		profile.Z,
		profile.Heading,
		profile.FactionID,
		profile.CharClass,
		profile.Gender,
		profile.LastLogin,
		profile.Birthday,
		string(optionsJSON),
		charID,
	); err != nil {
		log.Printf("[DB] Character creation FAILED at CharacterData update for %s: %v", name, err)
		return false
	}

	// Save bind points
	for i, bind := range profile.Binds {
		if bind.MapID == 0 {
			continue // Skip unset binds
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO character_bind (id, map_id, x, y, z, heading, slot)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id, slot) DO UPDATE SET
				map_id = EXCLUDED.map_id,
				x = EXCLUDED.x,
				y = EXCLUDED.y,
				z = EXCLUDED.z,
				heading = EXCLUDED.heading`,
			charID,
			bind.MapID,
			bind.X,
			bind.Y,
			bind.Z,
			bind.Heading,
			i,
		); err != nil {
			log.Printf("[DB] Character creation FAILED at Bind point %d for %s: %v", i, name, err)
			return false
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit transaction for %s: %v", name, err)
		return false
	}

	// Invalidate character select cache
	cacheKey := fmt.Sprintf("account:characters:%d", accountID)
	cache.GetCache().Delete(cacheKey)

	log.Printf("Character creation succeeded for %s (ID: %d)", name, charID)
	return true
}

func DeleteCharacter(ctx context.Context, accountID int64, characterName string) error {
	uuidStr := uuid.New().String()
	newNameSuffix := fmt.Sprintf("-DELETED-%s", uuidStr)

	tx, err := db.GlobalWorldDB.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Only select the columns we need to avoid schema mismatch issues
	var currentChar model.CharacterData
	var deletedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT id, name, account_id, deleted_at
		FROM character_data
		WHERE name = $1
		  AND account_id = $2
		  AND deleted_at IS NULL
		FOR UPDATE`,
		characterName, accountID).Scan(
		&currentChar.ID,
		&currentChar.Name,
		&currentChar.AccountID,
		&deletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to retrieve character name (name=%s): %w", characterName, err)
	}
	if deletedAt.Valid {
		currentChar.DeletedAt = &deletedAt.Time
	}

	log.Printf("DeleteCharacter: Found character to delete: ID=%d, Name='%s', DeletedAt='%v'", currentChar.ID, currentChar.Name, currentChar.DeletedAt)

	// Append -DELETED-<uuid> to the name
	newName := currentChar.Name + newNameSuffix

	// Soft-delete the character and update name
	result, err := tx.ExecContext(ctx, `
		UPDATE character_data
		SET name = $1,
		    deleted_at = CURRENT_TIMESTAMP
		WHERE name = $2
		  AND account_id = $3
		  AND deleted_at IS NULL`,
		newName, characterName, accountID)
	if err != nil {
		return fmt.Errorf("failed to delete character (id=%s): %w", characterName, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no character found with id %s for account %d or already deleted", characterName, accountID)
	}

	log.Printf("DeleteCharacter: Successfully marked %d rows as deleted (name=%s -> %s)", rowsAffected, characterName, newName)

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Invalidate character select cache
	cacheKey := fmt.Sprintf("account:characters:%d", accountID)
	if err := cache.GetCache().Delete(cacheKey); err != nil {
		log.Printf("Failed to delete cache key %s: %v", cacheKey, err)
		// Log but don't fail, as the database operation succeeded
	}

	log.Printf("Successfully deleted character (name=%s, new name=%s) for account %d", characterName, newName, accountID)
	return nil
}
