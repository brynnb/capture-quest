package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRootFromPackageDir(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "server", "schema", "postgres_runtime_schema.sql")); err != nil {
		t.Fatalf("findRepoRoot() = %q without schema: %v", root, err)
	}
}

func TestCountScriptEventFiles(t *testing.T) {
	count, root, err := countScriptEventFiles()
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatalf("countScriptEventFiles() = 0 under %s", root)
	}
}

func TestTableNamePatternRejectsUnsafeNames(t *testing.T) {
	for _, table := range []string{"phaser_maps", "character_data_2"} {
		if !tableNamePattern.MatchString(table) {
			t.Fatalf("tableNamePattern rejected safe table %q", table)
		}
	}
	for _, table := range []string{"phaser_maps;drop table account", "public.account", "Account"} {
		if tableNamePattern.MatchString(table) {
			t.Fatalf("tableNamePattern accepted unsafe table %q", table)
		}
	}
}
