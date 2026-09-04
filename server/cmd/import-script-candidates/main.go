// Import Script Candidates materializes neutral extractor candidates as
// file-backed CaptureQuest scripted events.
package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"capturequest/internal/scriptcandidateimport"
)

func main() {
	opts := parseFlags()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	stats, err := scriptcandidateimport.Run(ctx, opts)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Script candidate import complete: read=%d written=%d unchanged=%d skipped_overrides=%d skipped_unsupported=%d tile_override_candidates=%d tile_override_rules=%d conditional_dialogue_candidates=%d conditional_dialogue_rules=%d extractor_generated=%d extractor_unsupported=%d extractor_ambiguous=%d",
		stats.Read, stats.Written, stats.Unchanged, stats.SkippedOverrides, stats.SkippedUnsupported,
		stats.TileOverrideRead, stats.TileOverrideRules, stats.ConditionalDialogueRead, stats.ConditionalDialogueRules,
		stats.ExtractorGenerated, stats.ExtractorUnsupported, stats.ExtractorAmbiguous)
}

func parseFlags() scriptcandidateimport.Options {
	defaultSQLite := scriptcandidateimport.DefaultSQLitePath()
	defaultOutput := scriptcandidateimport.DefaultOutputDir()
	defaultDiagnostics := scriptcandidateimport.DefaultDiagnosticsPath(defaultOutput)
	sqliteFlag := flag.String("sqlite", defaultSQLite, "path to extracted Pokemon SQLite database")
	outputFlag := flag.String("output", defaultOutput, "CaptureQuest scripted event scripts directory")
	diagnosticsFlag := flag.String("diagnostics", defaultDiagnostics, "write importer/extractor diagnostics JSON report")
	dryRunFlag := flag.Bool("dry-run", false, "report generated files without writing")
	flag.Parse()

	if flag.NArg() > 1 {
		log.Fatalf("Usage: go run ./cmd/import-script-candidates [-sqlite path] [-output dir] [path]")
	}
	sqlitePath := *sqliteFlag
	if flag.NArg() == 1 {
		if *sqliteFlag != defaultSQLite {
			log.Fatalf("Use either -sqlite or a positional SQLite path, not both")
		}
		sqlitePath = flag.Arg(0)
	}

	return scriptcandidateimport.Options{
		SQLitePath:      sqlitePath,
		OutputDir:       *outputFlag,
		DiagnosticsPath: *diagnosticsFlag,
		DryRun:          *dryRunFlag,
	}
}
