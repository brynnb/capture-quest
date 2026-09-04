package scriptcandidateimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

func importTileOverrideCandidates(ctx context.Context, db *sql.DB, opts Options, stats *Stats, plan *outputPlan) ([]importDecision, error) {
	candidates, exists, err := loadTileOverrideCandidates(ctx, db)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	stats.TileOverrideRead = len(candidates)

	decisions := []importDecision{}
	if len(candidates) == 0 {
		changed, err := writeGeneratedEventTileOverrideFile(plan, generatedEventTileOverridesPath(opts.OutputDir), nil)
		if err != nil {
			return nil, err
		}
		recordTileOverrideFileChange(stats, changed)
		return decisions, nil
	}

	resolver, err := newTileOverrideResolver(ctx, db)
	if err != nil {
		for _, candidate := range candidates {
			stats.TileOverrideUnsupported++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "unsupported",
				Reason:      fmt.Sprintf("event tile resolver unavailable: %v", err),
			})
		}
		return decisions, nil
	}

	manualKeys, err := loadManualEventTileKeys(opts.OutputDir)
	if err != nil {
		return nil, err
	}

	generatedRules := []scriptedevents.EventTileOverrideRule{}
	successful := []tileOverrideCandidate{}
	for _, candidate := range candidates {
		rules, err := resolver.MapCandidate(candidate)
		if err != nil {
			stats.TileOverrideUnsupported++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "unsupported",
				Reason:      err.Error(),
			})
			continue
		}
		if key, path := firstManualEventTileConflict(rules, manualKeys); key != "" {
			stats.TileOverrideSkippedOverrides++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "skipped_override",
				Reason:      fmt.Sprintf("manual event tile file owns tile key %s", key),
				Path:        path,
				Details: rawDetails(map[string]any{
					"manualTileKey":      key,
					"manualPath":         path,
					"generatedRules":     rules,
					"sourceReplacements": candidate.Replacements,
				}),
			})
			continue
		}
		generatedRules = append(generatedRules, rules...)
		successful = append(successful, candidate)
	}

	sortEventTileRulesForImport(generatedRules)
	stats.TileOverrideRules = len(generatedRules)
	if len(generatedRules) == 0 && stats.TileOverrideUnsupported == len(candidates) {
		log.Printf("[ScriptCandidates] Preserving existing generated event tile overrides because all %d candidates failed to resolve", len(candidates))
		return decisions, nil
	}
	changed, err := writeGeneratedEventTileOverrideFile(plan, generatedEventTileOverridesPath(opts.OutputDir), generatedRules)
	if err != nil {
		return nil, err
	}
	recordTileOverrideFileChange(stats, changed)

	status := "unchanged"
	if changed {
		status = "generated"
	}
	for _, candidate := range successful {
		decisions = append(decisions, importDecision{
			MapName:     candidate.MapName,
			ScriptLabel: candidate.ScriptLabel,
			Status:      status,
			Reason:      "event_tile_override_candidate",
			Path:        generatedEventTileOverridesPath(opts.OutputDir),
			Details: rawDetails(map[string]any{
				"replacements": len(candidate.Replacements),
			}),
		})
	}
	return decisions, nil
}

func recordTileOverrideFileChange(stats *Stats, changed bool) {
	if changed {
		stats.TileOverrideWritten = 1
	} else {
		stats.TileOverrideUnchanged = 1
	}
}

func loadManualEventTileKeys(outputDir string) (map[string]string, error) {
	path := filepath.Join(filepath.Dir(outputDir), "event_tile_overrides.json")
	rules, err := scriptedevents.LoadEventTileOverridesFile(path)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]string, len(rules))
	for _, rule := range rules {
		keys[eventTileRuleKeyForImport(rule)] = path
	}
	return keys, nil
}

func firstManualEventTileConflict(rules []scriptedevents.EventTileOverrideRule, manualKeys map[string]string) (string, string) {
	for _, rule := range rules {
		key := eventTileRuleKeyForImport(rule)
		if path := manualKeys[key]; path != "" {
			return key, path
		}
	}
	return "", ""
}

func generatedEventTileOverridesPath(outputDir string) string {
	return filepath.Join(filepath.Dir(outputDir), generatedEventTilesFileName)
}

func writeGeneratedEventTileOverrideFile(plan *outputPlan, path string, rules []scriptedevents.EventTileOverrideRule) (bool, error) {
	file := scriptedevents.EventTileOverrideFile{Tiles: rules}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode generated event tile overrides: %w", err)
	}
	raw = append(raw, '\n')

	return plan.Stage(path, raw)
}
