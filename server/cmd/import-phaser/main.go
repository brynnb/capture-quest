// Import Phaser Database
//
// Imports public/phaser/pokemon.db (SQLite) data into the Postgres
// CaptureQuest database.
//
// Usage: go run ./cmd/import-phaser

package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"path/filepath"

	"capturequest/internal/config"
	"capturequest/internal/scriptedevents"

	_ "github.com/jackc/pgx/v4/stdlib"
	_ "modernc.org/sqlite"
)

func main() {
	defaultSQLite := defaultSQLitePath()
	sqliteFlag := flag.String("sqlite", defaultSQLite, "path to the extracted Pokemon SQLite database")
	flag.Parse()

	sqlitePath := *sqliteFlag
	if flag.NArg() > 1 {
		log.Fatalf("Usage: go run ./cmd/import-phaser [-sqlite path] [path]")
	}
	if flag.NArg() == 1 {
		if *sqliteFlag != defaultSQLite {
			log.Fatalf("Use either -sqlite or a positional SQLite path, not both")
		}
		sqlitePath = flag.Arg(0)
	}

	target, err := config.GetDatabaseTarget()
	if err != nil {
		log.Fatalf("Failed to load database target: %v", err)
	}
	if target.Dialect != config.DatabaseDialectPostgres {
		log.Fatalf("cmd/import-phaser is Postgres-only; got DB dialect %q from %s", target.Dialect, target.Source)
	}

	postgres, err := sql.Open(target.DriverName, target.DSN)
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer postgres.Close()
	if err := postgres.Ping(); err != nil {
		log.Fatalf("Failed to ping Postgres: %v", err)
	}

	sqlite, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		log.Fatalf("Failed to open SQLite database: %v", err)
	}
	defer sqlite.Close()

	if err := importPhaserToPostgres(sqlite, postgres); err != nil {
		log.Fatalf("Failed to import Phaser data into Postgres: %v", err)
	}
	if err := scriptedevents.SyncDefault(postgres); err != nil {
		log.Fatalf("Failed to sync scripted events from files: %v", err)
	}

	log.Println("Phaser database import complete.")
}

func defaultSQLitePath() string {
	candidates := []string{
		filepath.Join("..", "public", "phaser", "pokemon.db"),
		filepath.Join("public", "phaser", "pokemon.db"),
		filepath.Join("..", "..", "public", "phaser", "pokemon.db"),
		filepath.Join("..", "..", "..", "public", "phaser", "pokemon.db"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}
