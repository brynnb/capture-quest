package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Files below are no longer embedded because they are gitignored and would cause CI builds to fail.
// We read them from disk at runtime or use environment variables.
var keyPEMData = "internal/config/key.pem"
var configFileData = "internal/config/capturequest_config.json"

const (
	DatabaseDialectPostgres = "postgres"
)

func GetCert() (string, error) {
	// 1. Try environment variable
	if cert := os.Getenv("SSL_CERT_PEM"); cert != "" {
		return cert, nil
	}

	// 2. Try reading from file
	data, err := os.ReadFile(keyPEMData)
	if err != nil {
		return "", fmt.Errorf("cert not found in SSL_CERT_PEM env or %s: %w", keyPEMData, err)
	}
	return strings.TrimSpace(string(data)), nil
}

type Config struct {
	DatabaseURL string `json:"database_url"`
	DBDriver    string `json:"db_driver"`
	DBHost      string `json:"db_host"`
	DBPort      int    `json:"db_port"`
	DBUser      string `json:"db_user"`
	DBPass      string `json:"db_pass"`
	DBName      string `json:"db_name"`
	DBSSLMode   string `json:"db_sslmode"`
	Local       bool   `json:"local"`
	LocalQuests bool   `json:"localQuests"`
	GracePeriod int    `json:"gracePeriod"`
	OpenAIKey   string `json:"openai_key"`
	HTTPPort    int    `json:"http_port"`
	AdminKey    string `json:"admin_key"`
}

var config *Config

func Get() (*Config, error) {
	if config != nil {
		return config, nil
	}

	// Initialize with default values
	config = &Config{
		DBDriver:    "postgres",
		DBHost:      "127.0.0.1",    // Default host
		DBPort:      5432,           // Default Postgres port
		DBUser:      "postgres",     // Default Postgres user
		DBPass:      "",             // Default empty password
		DBName:      "capturequest", // Default database name
		DBSSLMode:   "disable",      // Default local Postgres setting
		Local:       true,           // Default local setting
		LocalQuests: false,          // Default local setting
		GracePeriod: 5,              // Default local setting
	}

	// Load default config from file (if exists)
	data, err := os.ReadFile(configFileData)
	if err == nil {
		_ = json.Unmarshal(data, config)
	}

	// Optionally override from local file on disk (not embedded, gitignored)
	// This allows developers to keep real credentials in capturequest_config.local.json
	// without committing them to the public repository.
	if localData, err := os.ReadFile("internal/config/capturequest_config.local.json"); err == nil {
		_ = json.Unmarshal(localData, config)
	}

	// Environment variable overrides (Standard for production/Fly.io)
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		config.DatabaseURL = databaseURL
	}
	if driver := os.Getenv("DB_DRIVER"); driver != "" {
		config.DBDriver = driver
	}
	if host := os.Getenv("DB_HOST"); host != "" {
		config.DBHost = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.DBPort = p
		}
	}
	if user := os.Getenv("DB_USER"); user != "" {
		config.DBUser = user
	}
	if pass := os.Getenv("DB_PASS"); pass != "" {
		config.DBPass = pass
	}
	if name := os.Getenv("DB_NAME"); name != "" {
		config.DBName = name
	}
	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		config.DBSSLMode = sslMode
	}
	if local := os.Getenv("LOCAL"); local != "" {
		config.Local = local == "true"
	}
	if port := os.Getenv("HTTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			config.HTTPPort = p
		}
	}
	if key := os.Getenv("OPENAI_KEY"); key != "" {
		config.OpenAIKey = key
	}
	if key := os.Getenv("ADMIN_KEY"); key != "" {
		config.AdminKey = key
	}

	return config, nil
}

type DatabaseTarget struct {
	DriverName string
	Dialect    string
	DSN        string
	Source     string
}

func GetDatabaseTarget() (DatabaseTarget, error) {
	cfg, err := Get()
	if err != nil {
		return DatabaseTarget{}, err
	}

	if cfg.DatabaseURL != "" {
		driverName, dialect, err := databaseDriverForURL(cfg.DatabaseURL)
		if err != nil {
			return DatabaseTarget{}, err
		}
		return DatabaseTarget{
			DriverName: driverName,
			Dialect:    dialect,
			DSN:        cfg.DatabaseURL,
			Source:     "DATABASE_URL",
		}, nil
	}

	driverName, dialect, err := normalizeDBDriver(cfg.DBDriver)
	if err != nil {
		return DatabaseTarget{}, err
	}
	if cfg.DBHost == "" || cfg.DBUser == "" || cfg.DBName == "" {
		return DatabaseTarget{}, fmt.Errorf("database connection details are incomplete")
	}

	if dialect != DatabaseDialectPostgres {
		return DatabaseTarget{}, fmt.Errorf("unsupported database dialect %q", dialect)
	}
	return DatabaseTarget{
		DriverName: driverName,
		Dialect:    dialect,
		DSN:        buildPostgresDSN(cfg),
		Source:     "config",
	}, nil
}

func databaseDriverForURL(rawURL string) (driverName, dialect string, err error) {
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return "", "", fmt.Errorf("parse DATABASE_URL: %w", parseErr)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "postgres", "postgresql":
		return "pgx", DatabaseDialectPostgres, nil
	default:
		return "", "", fmt.Errorf("unsupported DATABASE_URL scheme %q", parsed.Scheme)
	}
}

func normalizeDBDriver(driver string) (driverName, dialect string, err error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "postgres", "postgresql", "pgx":
		return "pgx", DatabaseDialectPostgres, nil
	default:
		return "", "", fmt.Errorf("unsupported DB_DRIVER %q", driver)
	}
}

func buildPostgresDSN(cfg *Config) string {
	user := url.User(cfg.DBUser)
	if cfg.DBPass != "" {
		user = url.UserPassword(cfg.DBUser, cfg.DBPass)
	}
	dsn := url.URL{
		Scheme: "postgres",
		User:   user,
		Host:   net.JoinHostPort(cfg.DBHost, strconv.Itoa(cfg.DBPort)),
		Path:   cfg.DBName,
	}
	query := dsn.Query()
	if cfg.DBSSLMode != "" {
		query.Set("sslmode", cfg.DBSSLMode)
	}
	dsn.RawQuery = query.Encode()
	return dsn.String()
}
