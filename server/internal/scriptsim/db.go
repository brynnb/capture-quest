package scriptsim

import (
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
	if err := db.InitWorldDB(target.DriverName, target.DSN); err != nil {
		return err
	}
	if err := scriptedevents.SyncDefault(db.GlobalWorldDB.DB); err != nil {
		return fmt.Errorf("sync scripted events: %w", err)
	}
	return nil
}
