package scriptcandidateimport

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	"capturequest/internal/extractorcontract"

	_ "modernc.org/sqlite"
)

func Run(ctx context.Context, opts Options) (Stats, error) {
	if opts.SQLitePath == "" {
		return Stats{}, fmt.Errorf("missing SQLite path")
	}
	if opts.OutputDir == "" {
		return Stats{}, fmt.Errorf("missing output directory")
	}
	lock, err := acquireImportLock(ctx, opts.OutputDir)
	if err != nil {
		return Stats{}, err
	}
	defer lock.Close()

	db, err := sql.Open("sqlite", opts.SQLitePath)
	if err != nil {
		return Stats{}, fmt.Errorf("open SQLite: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return Stats{}, fmt.Errorf("connect to SQLite: %w", err)
	}
	release := opts.Release
	if release == "" {
		release = "red"
	}
	if _, err := extractorcontract.Negotiate(ctx, db, release); err != nil {
		return Stats{}, fmt.Errorf("negotiate extractor source: %w", err)
	}
	plan := newOutputPlan(opts.Report)

	extractorDiagnostics, err := loadExtractorDiagnostics(ctx, db)
	if err != nil {
		return Stats{}, err
	}
	if err := validateUnsupportedDiagnosticBudget(extractorDiagnostics); err != nil {
		return Stats{}, err
	}
	stats := statsFromExtractorDiagnostics(extractorDiagnostics)
	decisions := []importDecision{}

	candidates, err := loadCandidates(ctx, db)
	if err != nil {
		return Stats{}, err
	}
	stats.Read = len(candidates)

	coordResolver, err := newCoordinateResolver(ctx, db)
	if err != nil {
		return stats, err
	}

	existing, err := loadExistingScripts(opts.OutputDir)
	if err != nil {
		return stats, err
	}

	for _, candidate := range candidates {
		event, err := mapCandidateWithResolver(candidate, coordResolver)
		if err != nil {
			stats.SkippedUnsupported++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "unsupported",
				Reason:      err.Error(),
			})
			continue
		}

		existingByLabel := existing.ByLabel[event.ScriptLabel]
		if existingByLabel.Source == capturequestSource {
			stats.SkippedOverrides++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "skipped_override",
				Reason:      "capturequest source owns this scriptLabel",
				Path:        existingByLabel.Path,
			})
			continue
		}

		if current := existing.ByTrigger[triggerKeyForEvent(event)]; existingByLabel.ScriptLabel == "" && current.ScriptLabel != "" && current.ScriptLabel != event.ScriptLabel && !canShareTriggerWithExistingExtractorBranch(event, current) {
			if merged, changed, err := mergeExtractorAudioIntoExisting(plan, current, event); err != nil {
				return stats, err
			} else if merged {
				if changed {
					stats.Written++
				} else {
					stats.Unchanged++
				}
				status := "unchanged"
				if changed {
					status = "generated"
				}
				decisions = append(decisions, importDecision{
					MapName:     candidate.MapName,
					ScriptLabel: candidate.ScriptLabel,
					Status:      status,
					Reason:      "merged source audio into existing extractor reward",
					Path:        current.Path,
				})
				continue
			}
			reason := "existing file owns this trigger"
			if current.Source == capturequestSource {
				reason = "capturequest source owns this trigger"
			}
			stats.SkippedOverrides++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "skipped_override",
				Reason:      reason,
				Path:        current.Path,
			})
			continue
		}

		if current, flag := existing.ownerForMapSetFlag(event); existingByLabel.ScriptLabel == "" && current.ScriptLabel != "" && current.ScriptLabel != event.ScriptLabel && !canShareMapSetFlagWithExistingExtractorBattle(event, current) {
			if merged, changed, err := mergeExtractorAudioIntoExisting(plan, current, event); err != nil {
				return stats, err
			} else if merged {
				if changed {
					stats.Written++
				} else {
					stats.Unchanged++
				}
				status := "unchanged"
				if changed {
					status = "generated"
				}
				decisions = append(decisions, importDecision{
					MapName:     candidate.MapName,
					ScriptLabel: candidate.ScriptLabel,
					Status:      status,
					Reason:      fmt.Sprintf("merged source audio into existing extractor reward for map flag %s", flag),
					Path:        current.Path,
				})
				continue
			}
			stats.SkippedOverrides++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "skipped_override",
				Reason:      fmt.Sprintf("existing file owns map flag %s", flag),
				Path:        current.Path,
			})
			continue
		}

		path := filepath.Join(opts.OutputDir, scriptFileName(event.ScriptLabel))
		if existingByLabel.Path != "" {
			path = existingByLabel.Path
		}
		changed, err := writeEventFile(plan, path, event)
		if err != nil {
			return stats, err
		}
		if changed {
			stats.Written++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "generated",
				Path:        path,
			})
		} else {
			stats.Unchanged++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "unchanged",
				Path:        path,
			})
		}
	}

	tileDecisions, err := importTileOverrideCandidates(ctx, db, opts, &stats, plan)
	if err != nil {
		return stats, err
	}
	decisions = append(decisions, tileDecisions...)

	visibilityDecisions, err := importObjectVisibilityCandidates(ctx, db, opts, &stats, plan)
	if err != nil {
		return stats, err
	}
	decisions = append(decisions, visibilityDecisions...)

	conditionalDecisions, err := importConditionalDialogueCandidates(ctx, db, opts, &stats, plan)
	if err != nil {
		return stats, err
	}
	decisions = append(decisions, conditionalDecisions...)

	if err := writeImportReport(plan, opts, stats, decisions, extractorDiagnostics); err != nil {
		return stats, err
	}
	if err := ctx.Err(); err != nil {
		return stats, fmt.Errorf("script candidate import cancelled before publication: %w", err)
	}
	if err := plan.Apply(opts.DryRun || opts.Check); err != nil {
		return stats, err
	}
	if opts.Check && plan.HasChanges() {
		return stats, ErrChangesRequired
	}
	return stats, nil
}
