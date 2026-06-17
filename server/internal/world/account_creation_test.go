package world

import (
	"context"
	"database/sql"
	"testing"

	"capturequest/internal/db"

	_ "modernc.org/sqlite"
)

func TestAccountCreationIgnoresLegacyDiscordIDDefault(t *testing.T) {
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })

	_, err = raw.Exec(`
		CREATE TABLE account (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			time_creation INTEGER NOT NULL DEFAULT 0,
			discord_id TEXT DEFAULT '' UNIQUE,
			guest_token TEXT DEFAULT NULL UNIQUE
		);
		CREATE TABLE character_data (
			account_id INTEGER NOT NULL,
			deleted_at TIMESTAMP DEFAULT NULL
		);
		INSERT INTO account (name, discord_id) VALUES ('legacy-empty-discord', '');
	`)
	if err != nil {
		t.Fatal(err)
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() { db.GlobalWorldDB = previous })

	ctx := context.Background()
	guestID, err := GetOrCreateGuestAccount(ctx, "guest_abcdefgh123456")
	if err != nil {
		t.Fatalf("GetOrCreateGuestAccount failed with legacy discord_id default: %v", err)
	}
	assertNullableAccountIdentity(t, raw, guestID, true)

	registeredID, err := RegisterLocalAccount(ctx, "new@example.test", "correct horse battery staple", "")
	if err != nil {
		t.Fatalf("RegisterLocalAccount failed with legacy discord_id default: %v", err)
	}
	assertNullableAccountIdentity(t, raw, registeredID, false)
}

func TestRegisterLocalAccountTransfersMatchingGuestCharacters(t *testing.T) {
	raw := setupAccountCreationDB(t)
	ctx := context.Background()

	const guestToken = "guest_abcdefgh123456"
	guestID, err := GetOrCreateGuestAccount(ctx, guestToken)
	if err != nil {
		t.Fatalf("GetOrCreateGuestAccount failed: %v", err)
	}
	if _, err := raw.Exec(`
		INSERT INTO character_data (account_id, deleted_at) VALUES ($1, NULL)`,
		guestID); err != nil {
		t.Fatal(err)
	}

	registeredID, err := RegisterLocalAccount(ctx, "new@example.test", "correct horse battery staple", guestToken)
	if err != nil {
		t.Fatalf("RegisterLocalAccount failed: %v", err)
	}

	assertAccountCharacterCount(t, guestID, 0)
	assertAccountCharacterCount(t, registeredID, 1)
}

func TestRegisterLocalAccountDoesNotTransferDifferentGuestCharacters(t *testing.T) {
	raw := setupAccountCreationDB(t)
	ctx := context.Background()

	guestID, err := GetOrCreateGuestAccount(ctx, "guest_abcdefgh123456")
	if err != nil {
		t.Fatalf("GetOrCreateGuestAccount failed: %v", err)
	}
	if _, err := raw.Exec(`
		INSERT INTO character_data (account_id, deleted_at) VALUES ($1, NULL)`,
		guestID); err != nil {
		t.Fatal(err)
	}

	registeredID, err := RegisterLocalAccount(ctx, "new@example.test", "correct horse battery staple", "guest_other123456")
	if err != nil {
		t.Fatalf("RegisterLocalAccount failed: %v", err)
	}

	assertAccountCharacterCount(t, guestID, 1)
	assertAccountCharacterCount(t, registeredID, 0)
}

func TestGuestAccountNamesDoNotCollideForMatchingTokenPrefixes(t *testing.T) {
	raw := setupAccountCreationDB(t)
	ctx := context.Background()

	firstID, err := GetOrCreateGuestAccount(ctx, "guest_abcdefgh123456")
	if err != nil {
		t.Fatalf("first GetOrCreateGuestAccount failed: %v", err)
	}
	secondID, err := GetOrCreateGuestAccount(ctx, "guest_abcdefgh999999")
	if err != nil {
		t.Fatalf("second GetOrCreateGuestAccount failed: %v", err)
	}
	if firstID == secondID {
		t.Fatalf("guest accounts reused id %d for different tokens", firstID)
	}

	var firstName, secondName string
	if err := raw.QueryRow(`SELECT name FROM account WHERE id = $1`, firstID).Scan(&firstName); err != nil {
		t.Fatal(err)
	}
	if err := raw.QueryRow(`SELECT name FROM account WHERE id = $1`, secondID).Scan(&secondName); err != nil {
		t.Fatal(err)
	}
	if firstName == secondName {
		t.Fatalf("guest account names collided: %q", firstName)
	}
}

func setupAccountCreationDB(t *testing.T) *sql.DB {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })

	_, err = raw.Exec(`
		CREATE TABLE account (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			time_creation INTEGER NOT NULL DEFAULT 0,
			discord_id TEXT DEFAULT NULL UNIQUE,
			guest_token TEXT DEFAULT NULL UNIQUE
		);
		CREATE TABLE character_data (
			account_id INTEGER NOT NULL,
			deleted_at TIMESTAMP DEFAULT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() { db.GlobalWorldDB = previous })

	return raw
}

func assertAccountCharacterCount(t *testing.T, accountID int64, want int) {
	t.Helper()

	got, err := CountAccountCharacters(context.Background(), accountID)
	if err != nil {
		t.Fatalf("CountAccountCharacters(%d): %v", accountID, err)
	}
	if got != want {
		t.Fatalf("CountAccountCharacters(%d) = %d, want %d", accountID, got, want)
	}
}

func assertNullableAccountIdentity(t *testing.T, raw *sql.DB, accountID int64, wantGuestToken bool) {
	t.Helper()

	var discordID sql.NullString
	var guestToken sql.NullString
	if err := raw.QueryRow(`
		SELECT discord_id, guest_token
		FROM account
		WHERE id = $1`, accountID).Scan(&discordID, &guestToken); err != nil {
		t.Fatal(err)
	}
	if discordID.Valid {
		t.Fatalf("account %d discord_id = %q, want NULL", accountID, discordID.String)
	}
	if guestToken.Valid != wantGuestToken {
		t.Fatalf("account %d guest_token valid = %t, want %t", accountID, guestToken.Valid, wantGuestToken)
	}
}
