package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/scriptedevents"
	"capturequest/internal/server"
)

// Build timestamp - set at compile time via ldflags
var BuildTime = "unknown"

func main() {
	processCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	log.Printf("=== CaptureQuest Server Starting ===")
	log.Printf("Binary built at: %s", BuildTime)

	serverConfig, err := config.Get()
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}
	if err := serverConfig.ValidateServerSecurity(); err != nil {
		log.Fatalf("unsafe server configuration: %v", err)
	}

	target, err := config.GetDatabaseTarget()
	if err != nil {
		log.Fatalf("failed to read database target: %v", err)
	}
	// One startup budget covers database bootstrap and world preload. The process
	// context remains independent so expiration cannot stop an already ready server.
	startupCtx, cancelStartup := context.WithTimeout(processCtx, time.Minute)
	defer cancelStartup()
	if err := initializeWorldDatabase(startupCtx, target.DriverName, target.DSN); err != nil {
		if processCtx.Err() != nil {
			log.Println("Startup cancelled")
			return
		}
		log.Fatalf("failed to initialize world database: %v", err)
	}

	srv, err := server.NewServer(startupCtx, target.DSN, time.Duration(serverConfig.GracePeriod)*time.Second, serverConfig.Local)
	if err != nil {
		_ = db.GlobalWorldDB.DB.Close()
		if processCtx.Err() != nil {
			log.Println("Startup cancelled")
			return
		}
		log.Fatalf("failed to create server: %v", err)
	}
	cancelStartup()

	if processCtx.Err() != nil {
		srv.StopServer()
		return
	}

	if err := srv.StartServer(); err != nil {
		srv.StopServer()
		log.Fatalf("failed to start server: %v", err)
	}

	var serveErr error
	select {
	case <-processCtx.Done():
		log.Println("Received shutdown signal, shutting down...")
	case serveErr = <-srv.Errors():
		log.Printf("Listener failed, shutting down: %v", serveErr)
	}

	shutdownBudget := time.Duration(serverConfig.GracePeriod) * time.Second
	if shutdownBudget <= 0 {
		shutdownBudget = 30 * time.Second
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownBudget)
	defer cancelShutdown()
	shutdownErr := srv.StopServerContext(shutdownCtx)
	if shutdownErr != nil {
		log.Printf("Shutdown failed: %v", shutdownErr)
	}
	if serveErr != nil || shutdownErr != nil {
		os.Exit(1)
	}
}

// No database consumers or listeners run until all bootstrap stages succeed.
// A failed or cancelled stage relinquishes the opened handle before returning.
func initializeWorldDatabase(ctx context.Context, driverName, dsn string) error {
	if err := db.InitWorldDB(ctx, driverName, dsn); err != nil {
		return err
	}
	database := db.GlobalWorldDB.DB
	if err := db.EnsureWorldTileMutationSchema(ctx, database); err != nil {
		_ = database.Close()
		return fmt.Errorf("upgrade world tile schema: %w", err)
	}
	if err := scriptedevents.SyncDefault(ctx, database); err != nil {
		_ = database.Close()
		return fmt.Errorf("sync scripted events: %w", err)
	}
	return nil
}
