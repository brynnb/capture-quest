package scriptsim

import (
	"context"
	"fmt"
	"os"
	"strings"

	"capturequest/internal/db"
	"capturequest/internal/scriptedevents"
	"github.com/jackc/pgx/v4"
)

func InitDB(ctx context.Context) error {
	// Simulator initialization syncs scripts and fixtures replace character
	// state. Never inherit a server target from DATABASE_URL or local config.
	// Use the same explicit disposable-database boundary as Go integration tests.
	dsn := strings.TrimSpace(os.Getenv("CAPTUREQUEST_TEST_DATABASE_URL"))
	if dsn == "" {
		return fmt.Errorf("script simulator requires CAPTUREQUEST_TEST_DATABASE_URL pointing to a disposable PostgreSQL database; use scripts/testing/run-isolated-script-sim.sh")
	}
	if _, err := pgx.ParseConfig(dsn); err != nil {
		// Parser errors can include the supplied URL and credentials.
		return fmt.Errorf("invalid CAPTUREQUEST_TEST_DATABASE_URL")
	}
	if err := db.InitWorldDB(ctx, "pgx", dsn); err != nil {
		return err
	}
	if err := scriptedevents.SyncDefault(ctx, db.GlobalWorldDB.DB); err != nil {
		return fmt.Errorf("sync scripted events: %w", err)
	}
	return nil
}
