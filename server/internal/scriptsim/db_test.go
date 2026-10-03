package scriptsim

import (
	"strings"
	"testing"

	"capturequest/internal/db"
)

func TestSimulatorRejectsApplicationDatabaseBeforeInitialization(t *testing.T) {
	previous := db.GlobalWorldDB
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	sentinel := &db.WorldDB{}
	db.GlobalWorldDB = sentinel
	t.Setenv("DATABASE_URL", "postgres://application-target.invalid/capturequest")
	for _, target := range []string{"", " \t\n"} {
		t.Setenv("CAPTUREQUEST_TEST_DATABASE_URL", target)
		err := InitDB()
		if err == nil || !strings.Contains(err.Error(), "CAPTUREQUEST_TEST_DATABASE_URL") {
			t.Fatalf("default target accepted: %v", err)
		}
		if db.GlobalWorldDB != sentinel {
			t.Fatal("simulator replaced the database before checking the test target")
		}
	}
}

func TestSimulatorRejectsMalformedExplicitTargetWithoutLeakingCredentials(t *testing.T) {
	previous := db.GlobalWorldDB
	t.Cleanup(func() { db.GlobalWorldDB = previous })
	sentinel := &db.WorldDB{}
	db.GlobalWorldDB = sentinel
	t.Setenv("CAPTUREQUEST_TEST_DATABASE_URL", "postgres://user:fixture-secret@%invalid/capturequest")
	err := InitDB()
	if err == nil || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("invalid target error=%v", err)
	}
	if db.GlobalWorldDB != sentinel {
		t.Fatal("invalid target changed the database")
	}
}
