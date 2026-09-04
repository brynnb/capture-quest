package scriptcandidateimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"capturequest/internal/scriptedevents"
)

func TestRunCheckModeReportsStaleOutputsWithoutWriting(t *testing.T) {
	candidate := safariCandidate("CheckCandidate", "", "")
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()
	diagnosticsPath := filepath.Join(filepath.Dir(outputDir), "diagnostics.json")
	opts := Options{SQLitePath: dbPath, OutputDir: outputDir, DiagnosticsPath: diagnosticsPath}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir, DiagnosticsPath: diagnosticsPath, Check: true}); err != nil {
		t.Fatalf("current output failed check: %v", err)
	}
	path := filepath.Join(outputDir, "check_candidate.json")
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	stale := append(append([]byte(nil), current...), '\n')
	if err := os.WriteFile(path, stale, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir, DiagnosticsPath: diagnosticsPath, Check: true}); !errors.Is(err, ErrChangesRequired) {
		t.Fatalf("stale output check error = %v, want ErrChangesRequired", err)
	}
	assertFileContent(t, path, string(stale))
}

func TestRunRejectsUnsupportedCandidateVersionBeforeWriting(t *testing.T) {
	dbPath := createSQLite(t, true, []scriptCandidate{safariCandidate("FutureCandidate", "", "")})
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE script_event_candidates SET candidate_json = json_set(candidate_json, '$.version', 99)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()
	_, err = Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err == nil || !strings.Contains(err.Error(), "unsupported candidate schema version 99") {
		t.Fatalf("Run error = %v", err)
	}
	if entries, readErr := os.ReadDir(outputDir); readErr != nil || len(entries) != 0 {
		t.Fatalf("output entries = %v, err = %v; want empty", entries, readErr)
	}
}

func TestRunRejectsCandidateJSONRelationalMismatchBeforeWriting(t *testing.T) {
	dbPath := createSQLite(t, true, []scriptCandidate{safariCandidate("MismatchCandidate", "", "")})
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE script_event_candidates SET map_name = 'WRONG_MAP'`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()
	_, err = Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err == nil || !strings.Contains(err.Error(), "JSON disagrees with relational columns") {
		t.Fatalf("Run error = %v", err)
	}
	if entries, readErr := os.ReadDir(outputDir); readErr != nil || len(entries) != 0 {
		t.Fatalf("output entries = %v, err = %v; want empty", entries, readErr)
	}
}

func TestUnsupportedDiagnosticBudgetRejectsNewReasonAndRegression(t *testing.T) {
	if err := validateUnsupportedDiagnosticBudget([]extractorDiagnostic{{Status: "unsupported", Reason: "new_unreviewed_reason"}}); err == nil || !strings.Contains(err.Error(), "unreviewed") {
		t.Fatalf("new reason error = %v", err)
	}
	diagnostics := make([]extractorDiagnostic, reviewedUnsupportedDiagnosticBudget["text_asm_multi_text_branch"]+1)
	for index := range diagnostics {
		diagnostics[index] = extractorDiagnostic{Status: "unsupported", Reason: "text_asm_multi_text_branch"}
	}
	if err := validateUnsupportedDiagnosticBudget(diagnostics); err == nil || !strings.Contains(err.Error(), "increased") {
		t.Fatalf("increased count error = %v", err)
	}
}

func TestRunNoopsWhenCandidateTableMissing(t *testing.T) {
	dbPath := createSQLite(t, false, nil)
	outputDir := t.TempDir()

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Read != 0 || stats.Written != 0 {
		t.Fatalf("stats = %+v, want no-op", stats)
	}
	if entries, err := os.ReadDir(outputDir); err != nil || len(entries) != 0 {
		t.Fatalf("output entries = %v, err = %v; want empty", entries, err)
	}
}

func TestRunHonorsCancelledContextBeforePlanning(t *testing.T) {
	dbPath := createSQLite(t, true, []scriptCandidate{safariCandidate("CancelledEvent", "", "")})
	outputDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := Run(ctx, Options{SQLitePath: dbPath, OutputDir: outputDir}); err == nil {
		t.Fatal("Run succeeded with a cancelled context")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "cancelled_event.json")); !os.IsNotExist(err) {
		t.Fatalf("cancelled run published output: %v", err)
	}
}

func TestRunWritesSupportedCandidate(t *testing.T) {
	candidate := safariCandidate("SafariZoneGateEntryOffer", "EVENT_IN_SAFARI_ZONE", "")
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	outputDir := t.TempDir()

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Read != 1 || stats.Written != 1 || stats.Unchanged != 0 {
		t.Fatalf("stats = %+v, want one write", stats)
	}

	path := filepath.Join(outputDir, "safari_zone_gate_entry_offer.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var event scriptedevents.EventFile
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatal(err)
	}
	if event.ScriptLabel != "SafariZoneGateEntryOffer" {
		t.Fatalf("scriptLabel = %q", event.ScriptLabel)
	}
	if event.MapName != "SAFARI_ZONE_GATE" {
		t.Fatalf("mapName = %q", event.MapName)
	}
	if event.Trigger.Source != extractorSource || event.Trigger.Label != "SafariZoneGateEntryOffer" {
		t.Fatalf("trigger = %+v", event.Trigger)
	}
	if event.RequiresFlagAbsent != "EVENT_IN_SAFARI_ZONE" {
		t.Fatalf("requiresFlagAbsent = %q", event.RequiresFlagAbsent)
	}
	if len(event.Actions) != 6 {
		t.Fatalf("actions len = %d, want 6: %s", len(event.Actions), string(raw))
	}
}

func TestRunWritesDiagnosticsReport(t *testing.T) {
	candidate := safariCandidate("SafariZoneGateEntryOffer", "EVENT_IN_SAFARI_ZONE", "")
	dbPath := createSQLite(t, true, []scriptCandidate{candidate})
	insertExtractorDiagnostic(t, dbPath, extractorDiagnostic{
		MapName:     "BikeShop",
		ScriptLabel: "BikeShopClerkText",
		Status:      "unsupported",
		Reason:      "text_asm_multi_text_branch",
		Details:     json.RawMessage(`{"features":{"hasGiveItem":true}}`),
	})
	insertExtractorDiagnostic(t, dbPath, extractorDiagnostic{
		MapName:     "Museum1F",
		ScriptLabel: "Museum1FScript",
		Status:      "ambiguous",
		Reason:      "choice,item_reward,event_flags",
		Details:     json.RawMessage(`{"features":{"hasChoice":true}}`),
	})
	outputDir := t.TempDir()
	diagnosticsPath := filepath.Join(t.TempDir(), "diagnostics.json")

	stats, err := Run(context.Background(), Options{SQLitePath: dbPath, OutputDir: outputDir, DiagnosticsPath: diagnosticsPath})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Written != 1 || stats.ExtractorUnsupported != 1 || stats.ExtractorAmbiguous != 1 {
		t.Fatalf("stats = %+v, want written=1 unsupported=1 ambiguous=1", stats)
	}

	raw, err := os.ReadFile(diagnosticsPath)
	if err != nil {
		t.Fatal(err)
	}
	var report importReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.Stats.ExtractorUnsupported != 1 || report.Stats.ExtractorAmbiguous != 1 {
		t.Fatalf("report stats = %+v", report.Stats)
	}
	if report.Summary.DecisionsByStatus["generated"] != 1 {
		t.Fatalf("decision summary = %+v, want one generated decision", report.Summary.DecisionsByStatus)
	}
	if report.Summary.ExtractorByStatus["unsupported"] != 1 || report.Summary.ExtractorByStatus["ambiguous"] != 1 {
		t.Fatalf("extractor status summary = %+v", report.Summary.ExtractorByStatus)
	}
	if report.Summary.ExtractorByReason["text_asm_multi_text_branch"] != 1 {
		t.Fatalf("extractor reason summary = %+v, want text_asm_multi_text_branch", report.Summary.ExtractorByReason)
	}
	if report.Summary.UnsupportedByReason["text_asm_multi_text_branch"] != 1 {
		t.Fatalf("unsupported reason summary = %+v, want text_asm_multi_text_branch", report.Summary.UnsupportedByReason)
	}
	if len(report.Decisions) != 1 || report.Decisions[0].Status != "generated" {
		t.Fatalf("decisions = %+v, want one generated decision", report.Decisions)
	}
	if len(report.ExtractorDiagnostics) != 2 {
		t.Fatalf("extractor diagnostics len = %d, want 2", len(report.ExtractorDiagnostics))
	}
}
