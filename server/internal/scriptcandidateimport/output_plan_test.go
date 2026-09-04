package scriptcandidateimport

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOutputPlanPublishesOnlyWhenApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event.json")
	if err := os.WriteFile(path, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	plan := newOutputPlan()
	changed, err := plan.Stage(path, []byte("new\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("Stage reported unchanged content")
	}
	assertFileContent(t, path, "old\n")

	if err := plan.Apply(false); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, path, "new\n")
}

func TestOutputPlanDryRunDoesNotPublish(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "event.json")
	if err := os.WriteFile(path, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	plan := newOutputPlan()
	if _, err := plan.Stage(path, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(true); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, path, "old\n")
}

func TestOutputPlanMakesStagedContentVisibleToLaterMerges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "event.json")
	plan := newOutputPlan()
	if changed, err := plan.Stage(path, []byte("first\n")); err != nil || !changed {
		t.Fatalf("first Stage changed=%t err=%v", changed, err)
	}
	if changed, err := plan.Stage(path, []byte("first\n")); err != nil || changed {
		t.Fatalf("identical Stage changed=%t err=%v", changed, err)
	}
	if changed, err := plan.Stage(path, []byte("second\n")); err != nil || !changed {
		t.Fatalf("second Stage changed=%t err=%v", changed, err)
	}
	raw, err := plan.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "second\n" {
		t.Fatalf("ReadFile = %q, want latest staged content", raw)
	}
}

func TestOutputPlanPreparationFailureLeavesEarlierFilesUntouched(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "a", "event.json")
	blocker := filepath.Join(dir, "z-blocker")
	blockedPath := filepath.Join(blocker, "event.json")
	if err := os.MkdirAll(filepath.Dir(goodPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(goodPath, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	plan := newOutputPlan()
	if _, err := plan.Stage(goodPath, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Stage(blockedPath, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(blocker, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(false); err == nil {
		t.Fatal("Apply succeeded despite an invalid output directory")
	}
	assertFileContent(t, goodPath, "old\n")
}

func TestRunDoesNotPublishScriptsWhenLaterCandidateFamilyFails(t *testing.T) {
	dbPath := createSQLite(t, true, []scriptCandidate{safariCandidate("PlannedEvent", "", "")})
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		CREATE TABLE script_event_object_visibility (
			id INTEGER PRIMARY KEY,
			map_name TEXT NOT NULL,
			map_id INTEGER NOT NULL,
			object_name TEXT NOT NULL,
			object_key TEXT NOT NULL,
			script_label TEXT NOT NULL,
			requires_event TEXT NOT NULL,
			visible INTEGER NOT NULL,
			label TEXT NOT NULL,
			rule_json TEXT NOT NULL
		);
		INSERT INTO script_event_object_visibility VALUES
			(1, 'TEST_MAP', 1, 'NPC', 'NPC_1', 'Broken', 'EVENT_TEST', 1, 'broken', '{');
	`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	outputDir := t.TempDir()
	if _, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir}); err == nil {
		t.Fatal("Run succeeded with malformed later-family JSON")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "planned_event.json")); !os.IsNotExist(err) {
		t.Fatalf("planned script was published before later validation failed: %v", err)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != want {
		t.Fatalf("%s = %q, want %q", path, raw, want)
	}
}
