package main

import (
	"context"
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
	if err := db.InitWorldDB(target.DriverName, target.DSN); err != nil {
		log.Fatalf("failed to initialize db.WorldDB: %v", err)
	}
	if err := db.EnsureWorldTileMutationSchema(db.GlobalWorldDB.DB); err != nil {
		log.Fatalf("failed to upgrade world tile schema: %v", err)
	}
	if err := scriptedevents.SyncDefault(db.GlobalWorldDB.DB); err != nil {
		log.Fatalf("failed to sync scripted events: %v", err)
	}

	srv, err := server.NewServer(processCtx, target.DSN, time.Duration(serverConfig.GracePeriod), serverConfig.Local)
	if err != nil {
		if processCtx.Err() != nil {
			_ = db.GlobalWorldDB.DB.Close()
			log.Println("Startup cancelled")
			return
		}
		log.Fatalf("failed to create server: %v", err)
	}
	if processCtx.Err() != nil {
		srv.StopServer()
		return
	}

	// _, err = nav.GetNavigation()

	// if err != nil {
	// 	log.Fatalf("Failed to create navigation %v", err)
	// }

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

	srv.StopServer()
	if serveErr != nil {
		os.Exit(1)
	}
}
