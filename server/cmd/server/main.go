package main

import (
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
	log.Printf("=== CaptureQuest Server Starting ===")
	log.Printf("Binary built at: %s", BuildTime)

	target, err := config.GetDatabaseTarget()
	if err != nil {
		log.Fatalf("failed to read database target: %v", err)
	}
	if err := db.InitWorldDB(target.DriverName, target.DSN); err != nil {
		log.Fatalf("failed to initialize db.WorldDB: %v", err)
	}
	if err := scriptedevents.SyncDefault(db.GlobalWorldDB.DB); err != nil {
		log.Fatalf("failed to sync scripted events: %v", err)
	}

	serverConfig, err := config.Get()
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}

	srv, err := server.NewServer(target.DSN, time.Duration(serverConfig.GracePeriod), serverConfig.Local)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	// _, err = nav.GetNavigation()

	// if err != nil {
	// 	log.Fatalf("Failed to create navigation %v", err)
	// }

	go srv.StartServer()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Received shutdown signal, shutting down...")

	srv.StopServer()
}
