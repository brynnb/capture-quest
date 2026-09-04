package scriptcandidateimport

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

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
		Reason:      "item_reward,event_flags",
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
	if report.Summary.ExtractorByReason["item_reward,event_flags"] != 1 {
		t.Fatalf("extractor reason summary = %+v, want item_reward,event_flags", report.Summary.ExtractorByReason)
	}
	if report.Summary.UnsupportedByReason["item_reward,event_flags"] != 1 {
		t.Fatalf("unsupported reason summary = %+v, want item_reward,event_flags", report.Summary.UnsupportedByReason)
	}
	if len(report.Decisions) != 1 || report.Decisions[0].Status != "generated" {
		t.Fatalf("decisions = %+v, want one generated decision", report.Decisions)
	}
	if len(report.ExtractorDiagnostics) != 2 {
		t.Fatalf("extractor diagnostics len = %d, want 2", len(report.ExtractorDiagnostics))
	}
}
