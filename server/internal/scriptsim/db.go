package scriptsim

import (
	"context"
	"fmt"

	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/scriptedevents"
)

func InitDB() error {
	target, err := config.GetDatabaseTarget()
	if err != nil {
		return fmt.Errorf("read database target: %w", err)
	}
	if err := db.InitWorldDB(context.Background(), target.DriverName, target.DSN); err != nil {
		return err
	}
	if err := scriptedevents.SyncDefault(context.Background(), db.GlobalWorldDB.DB); err != nil {
		return fmt.Errorf("sync scripted events: %w", err)
	}
	return nil
}
