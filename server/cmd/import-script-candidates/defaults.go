package main

import (
	"os"
	"path/filepath"
)

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

func defaultOutputDir() string {
	for _, candidate := range []string{
		filepath.Join("scripted_events", "scripts"),
		filepath.Join("server", "scripted_events", "scripts"),
		filepath.Join("..", "scripted_events", "scripts"),
		filepath.Join("..", "..", "server", "scripted_events", "scripts"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("scripted_events", "scripts")
}

func defaultDiagnosticsPath(outputDir string) string {
	return filepath.Join(filepath.Dir(outputDir), "script_candidate_import_diagnostics.json")
}
