package world

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log"
	"time"

	"capturequest/internal/cache"
	"capturequest/internal/db"

	model "capturequest/internal/db/models"
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

	guestName := guestAccountName(guestToken)
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

func guestAccountName(guestToken string) string {
	digest := sha256.Sum256([]byte(guestToken))
	return fmt.Sprintf("Guest_%x", digest[:6])
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

func AccountHasCharacterName(ctx context.Context, database db.ContextDBTX, accountID int64, charName string) (bool, error) {
	// Never cache this - always keep up to date
	var exists bool
	if err := database.QueryRowContext(ctx, `
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
