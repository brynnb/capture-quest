# Makefile for Go daemon project
.PHONY: all build up down clean lint server start s bootstrap import-phaser quest

# Variables
BIN_DIR = ./bin
SERVER_SRC = ./cmd/server
SERVER_NAME = $(BIN_DIR)/server
DAEMON_SRC = ./cmd/daemon
IMPORT_PHASER_SRC = ./cmd/import-phaser
DAEMON_NAME = $(BIN_DIR)/daemon

PID_FILE = server.pid
LOG_FILE = server.log

# Default target
all: build

server:
	@echo "Starting server..."
	@go run $(SERVER_SRC)
start: server
s: server

bootstrap:
	@echo "Bootstrapping Postgres database..."
	@./scripts/bootstrap_postgres.sh --create --reset

import-phaser:
	@echo "Importing Phaser data into Postgres..."
	@go run $(IMPORT_PHASER_SRC) ../public/phaser/pokemon.db

# Build the daemon binary
build:
	@echo "Building $(SERVER_NAME) and $(DAEMON_NAME)..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(DAEMON_NAME) $(DAEMON_SRC)
	@go build -o $(SERVER_NAME) $(SERVER_SRC)

# Start the daemon
up: build
	@echo "Starting daemon..."
	sudo ./$(DAEMON_NAME) &

# Stop the daemon
down:
	@echo "Stopping daemon..."
	@if [ -f $(PID_FILE) ]; then \
		kill `cat $(PID_FILE)` && rm -f $(PID_FILE); \
		echo "Daemon stopped"; \
	else \
		echo "No PID file found, daemon may not be running"; \
	fi

# Clean up generated files
clean:
	@echo "Cleaning up..."
	@rm -f $(DAEMON_NAME) $(PID_FILE) $(LOG_FILE)
	@rm -f $(SERVER_NAME)

# Run linter (optional, requires golangci-lint)
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

quest:
	@echo "Running quest tests..."
	@echo "Running NATIVE..."
	@go test -bench=. -benchmem ./internal/quest/registry
	@echo "Running YAEGI..."
	@go test -bench=. -benchmem -tags=dev ./internal/quest/registry
	@echo "Quest tests completed."
