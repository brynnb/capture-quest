package db

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v4/stdlib"
)

type WorldDB struct {
	DB *sql.DB
}

var GlobalWorldDB *WorldDB

func InitWorldDB(driverName, dsn string) error {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}
	fmt.Printf("Connected to %s database successfully\n", driverName)
	GlobalWorldDB = &WorldDB{DB: db}
	return nil
}
