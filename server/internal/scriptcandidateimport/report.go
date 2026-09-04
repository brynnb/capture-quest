package scriptcandidateimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

func loadExtractorDiagnostics(ctx context.Context, db *sql.DB) ([]extractorDiagnostic, error) {
	exists, err := sqliteTableExists(ctx, db, "script_event_candidate_diagnostics")
	if err != nil {
		return nil, fmt.Errorf("check script_event_candidate_diagnostics table: %w", err)
	}
	if !exists {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT map_name, script_label, status, reason, details_json
		FROM script_event_candidate_diagnostics
		ORDER BY status, map_name, script_label, id`)
	if err != nil {
		return nil, fmt.Errorf("query script_event_candidate_diagnostics: %w", err)
	}
	defer rows.Close()

	diagnostics := []extractorDiagnostic{}
	for rows.Next() {
		var diagnostic extractorDiagnostic
		var details string
		if err := rows.Scan(&diagnostic.MapName, &diagnostic.ScriptLabel, &diagnostic.Status, &diagnostic.Reason, &details); err != nil {
			return nil, err
		}
		if json.Valid([]byte(details)) {
			diagnostic.Details = json.RawMessage(details)
		} else {
			diagnostic.Details = json.RawMessage(`{}`)
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return diagnostics, nil
}

func statsFromExtractorDiagnostics(diagnostics []extractorDiagnostic) Stats {
	stats := Stats{ExtractorDiagnostics: len(diagnostics)}
	for _, diagnostic := range diagnostics {
		switch diagnostic.Status {
		case "generated":
			stats.ExtractorGenerated++
		case "unsupported":
			stats.ExtractorUnsupported++
		case "ambiguous":
			stats.ExtractorAmbiguous++
		}
	}
	return stats
}

func writeImportReport(plan *outputPlan, opts Options, stats Stats, decisions []importDecision, extractorDiagnostics []extractorDiagnostic) error {
	if opts.DiagnosticsPath == "" {
		return nil
	}
	report := importReport{
		SQLitePath:           opts.SQLitePath,
		OutputDir:            opts.OutputDir,
		DryRun:               opts.DryRun,
		Stats:                stats,
		Summary:              buildImportReportSummary(decisions, extractorDiagnostics),
		Decisions:            decisions,
		ExtractorDiagnostics: extractorDiagnostics,
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode diagnostics report: %w", err)
	}
	raw = append(raw, '\n')
	_, err = plan.Stage(opts.DiagnosticsPath, raw)
	return err
}

func buildImportReportSummary(decisions []importDecision, extractorDiagnostics []extractorDiagnostic) importReportSummary {
	summary := importReportSummary{
		DecisionsByStatus:        map[string]int{},
		DecisionsByReason:        map[string]int{},
		ExtractorByStatus:        map[string]int{},
		ExtractorByReason:        map[string]int{},
		GeneratedByAdapter:       map[string]int{},
		SkippedOverridesByReason: map[string]int{},
		UnsupportedByReason:      map[string]int{},
	}
	for _, decision := range decisions {
		increment(summary.DecisionsByStatus, decision.Status)
		increment(summary.DecisionsByReason, decision.Reason)
		if decision.Status == "skipped_override" {
			increment(summary.SkippedOverridesByReason, decision.Reason)
		}
		if decision.Status == "unsupported" {
			increment(summary.UnsupportedByReason, decision.Reason)
		}
	}
	for _, diagnostic := range extractorDiagnostics {
		increment(summary.ExtractorByStatus, diagnostic.Status)
		increment(summary.ExtractorByReason, diagnostic.Reason)
		if diagnostic.Status == "generated" {
			increment(summary.GeneratedByAdapter, diagnostic.Reason)
		}
		if diagnostic.Status == "unsupported" || diagnostic.Status == "ambiguous" {
			increment(summary.UnsupportedByReason, diagnostic.Reason)
		}
	}
	return summary
}

func increment(counts map[string]int, key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "(none)"
	}
	counts[key]++
}

func sqliteTableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return name == table, nil
}

func sqliteColumnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}
