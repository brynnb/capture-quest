package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultSQLitePathFindsTrackedPokemonDB(t *testing.T) {
	path := defaultSQLitePath()
	if filepath.Base(path) != "pokemon.db" {
		t.Fatalf("defaultSQLitePath() = %q, want pokemon.db path", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("defaultSQLitePath() returned missing path %q: %v", path, err)
	}
}
