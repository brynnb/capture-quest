// Import Script Candidates
//
// Reads neutral script_event_candidates rows from the extracted Pokemon SQLite
// artifact and materializes supported candidates as file-backed CaptureQuest
// scripted events. CaptureQuest-authored files remain authoritative overrides.
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"capturequest/internal/phaserdata"
	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

const (
	extractorSource                      = "extractor"
	capturequestSource                   = "capturequest"
	generatedObjectVisibilityFileName    = "object_visibility.generated.json"
	generatedEventTilesFileName          = "event_tile_overrides.generated.json"
	generatedConditionalDialogueFileName = "conditional_dialogue.generated.json"
)

type importOptions struct {
	SQLitePath      string
	OutputDir       string
	DiagnosticsPath string
	DryRun          bool
}

type importStats struct {
	Read                         int `json:"read"`
	Written                      int `json:"written"`
	Unchanged                    int `json:"unchanged"`
	SkippedOverrides             int `json:"skippedOverrides"`
	SkippedUnsupported           int `json:"skippedUnsupported"`
	TileOverrideRead             int `json:"tileOverrideRead"`
	TileOverrideRules            int `json:"tileOverrideRules"`
	TileOverrideWritten          int `json:"tileOverrideWritten"`
	TileOverrideUnchanged        int `json:"tileOverrideUnchanged"`
	TileOverrideSkippedOverrides int `json:"tileOverrideSkippedOverrides"`
	TileOverrideUnsupported      int `json:"tileOverrideUnsupported"`
	ObjectVisibilityRead         int `json:"objectVisibilityRead"`
	ObjectVisibilityRules        int `json:"objectVisibilityRules"`
	ObjectVisibilityWritten      int `json:"objectVisibilityWritten"`
	ObjectVisibilityUnchanged    int `json:"objectVisibilityUnchanged"`
	ObjectVisibilityUnsupported  int `json:"objectVisibilityUnsupported"`
	ConditionalDialogueRead      int `json:"conditionalDialogueRead"`
	ConditionalDialogueRules     int `json:"conditionalDialogueRules"`
	ConditionalDialogueWritten   int `json:"conditionalDialogueWritten"`
	ConditionalDialogueUnchanged int `json:"conditionalDialogueUnchanged"`
	ExtractorUnsupported         int `json:"extractorUnsupported"`
	ExtractorAmbiguous           int `json:"extractorAmbiguous"`
	ExtractorGenerated           int `json:"extractorGenerated"`
	ExtractorDiagnostics         int `json:"extractorDiagnostics"`
}

type scriptCandidate struct {
	Version     int                `json:"version"`
	Kind        string             `json:"kind"`
	MapName     string             `json:"mapName"`
	ScriptLabel string             `json:"scriptLabel"`
	Trigger     candidateTrigger   `json:"trigger"`
	Conditions  candidateCondition `json:"conditions"`
	Actions     []candidateAction  `json:"actions"`
	Confidence  string             `json:"confidence"`
}

type tileOverrideCandidate struct {
	Version      int                       `json:"version"`
	Kind         string                    `json:"kind"`
	MapName      string                    `json:"mapName"`
	ScriptLabel  string                    `json:"scriptLabel"`
	Replacements []tileOverrideReplacement `json:"replacements"`
	Confidence   string                    `json:"confidence"`
}

type conditionalDialogueCandidate struct {
	Version           int                `json:"version"`
	Kind              string             `json:"kind"`
	MapName           string             `json:"mapName"`
	ScriptLabel       string             `json:"scriptLabel"`
	SourceScriptLabel string             `json:"sourceScriptLabel"`
	TextConstant      string             `json:"textConstant"`
	Priority          int                `json:"priority"`
	Conditions        candidateCondition `json:"conditions"`
	DialogueLabels    []string           `json:"dialogueLabels"`
	Source            map[string]any     `json:"source"`
	Confidence        string             `json:"confidence"`
}

type objectVisibilityCandidate struct {
	Version       int            `json:"version"`
	Kind          string         `json:"kind"`
	MapName       string         `json:"mapName"`
	MapID         int            `json:"mapId"`
	ObjectName    string         `json:"objectName"`
	ObjectKey     string         `json:"objectKey"`
	Visible       bool           `json:"visible"`
	RequiresEvent string         `json:"requiresEvent"`
	Label         string         `json:"label"`
	SourceMapName string         `json:"sourceMapName"`
	ScriptLabel   string         `json:"scriptLabel"`
	Source        map[string]any `json:"source"`
	Confidence    string         `json:"confidence"`
}

type tileOverrideReplacement struct {
	BlockX              int    `json:"blockX"`
	BlockY              int    `json:"blockY"`
	BlockID             int    `json:"blockId"`
	RequiresEvent       string `json:"requiresEvent"`
	RequiresEventAbsent string `json:"requiresEventAbsent"`
	LabelPrefix         string `json:"labelPrefix"`
}

type candidateTrigger struct {
	Type        string                           `json:"type"`
	Label       string                           `json:"label"`
	Coordinates []scriptedevents.EventCoordinate `json:"coordinates"`
}

type candidateCondition struct {
	RequiresEvent         string   `json:"requiresEvent"`
	RequiresEventAbsent   string   `json:"requiresEventAbsent"`
	RequiresEvents        []string `json:"requiresEvents"`
	RequiresEventsAbsent  []string `json:"requiresEventsAbsent"`
	RequiresBadge         string   `json:"requiresBadge"`
	RequiresBadgeAbsent   string   `json:"requiresBadgeAbsent"`
	RequiresBadges        []string `json:"requiresBadges"`
	RequiresBadgesAbsent  []string `json:"requiresBadgesAbsent"`
	RequiresItem          string   `json:"requiresItem"`
	RequiresItemAbsent    string   `json:"requiresItemAbsent"`
	RequiresPokedexCaught int      `json:"requiresPokedexCaught"`
	RequiresMoney         int      `json:"requiresMoney"`
	RequiresMoneyBelow    int      `json:"requiresMoneyBelow"`
	RequiresCoins         int      `json:"requiresCoins"`
	RequiresCoinsBelow    int      `json:"requiresCoinsBelow"`
	RequiresPlayerFacing  string   `json:"requiresPlayerFacing"`
}

type candidateAction struct {
	Type              string                `json:"type"`
	Speaker           string                `json:"speaker"`
	Lines             []string              `json:"lines"`
	Prompt            string                `json:"prompt"`
	PromptLines       []string              `json:"promptLines"`
	YesLines          []string              `json:"yesLines"`
	NoLines           []string              `json:"noLines"`
	ContinueOnNo      bool                  `json:"continueOnNo"`
	StopOnYes         bool                  `json:"stopOnYes"`
	Destination       *candidateDestination `json:"destination"`
	Event             string                `json:"event"`
	Flag              string                `json:"flag"`
	ItemID            int                   `json:"itemId"`
	ItemName          string                `json:"itemName"`
	ItemConstant      string                `json:"itemConstant"`
	Quantity          int                   `json:"quantity"`
	PokemonID         int                   `json:"pokemonId"`
	SpeciesID         int                   `json:"speciesId"`
	PokemonName       string                `json:"pokemonName"`
	PokemonConstant   string                `json:"pokemonConstant"`
	SFXConstant       string                `json:"sfxConstant"`
	Volume            float64               `json:"volume"`
	Level             int                   `json:"level"`
	Message           string                `json:"message"`
	Money             int                   `json:"money"`
	Coins             int                   `json:"coins"`
	Actor             string                `json:"actor"`
	Movements         []string              `json:"movements"`
	MS                int                   `json:"ms"`
	MapID             int                   `json:"mapId"`
	X                 int                   `json:"x"`
	Y                 int                   `json:"y"`
	Direction         string                `json:"direction"`
	ObjectID          int                   `json:"objectId"`
	ObjectKey         string                `json:"objectKey"`
	ObjectMapName     string                `json:"objectMapName"`
	TriggerLabel      string                `json:"triggerLabel"`
	TextConstant      string                `json:"textConstant"`
	TrainerClass      string                `json:"trainerClass"`
	TrainerPartyIndex int                   `json:"partyIndex"`
	PartyByFlag       map[string]int        `json:"partyByFlag"`
	TrainerName       string                `json:"trainerName"`
	TrainerObjectID   int                   `json:"trainerObjectId"`
	WinFlag           string                `json:"winFlag"`
	LoseFlag          string                `json:"loseFlag"`
	LossMessage       string                `json:"lossMessage"`
	NoBlackoutOnLoss  bool                  `json:"noBlackoutOnLoss"`
	PostWinActions    []candidateAction     `json:"postWinActions"`
	PostLoseActions   []candidateAction     `json:"postLoseActions"`
	AllowedActions    []string              `json:"allowedActions"`
	GuaranteedCatch   bool                  `json:"guaranteedCatch"`
	PrizeWindow       int                   `json:"prizeWindow"`
}

type candidateDestination struct {
	MapName   string `json:"mapName"`
	MapID     int    `json:"mapId"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
}

type existingScript struct {
	Path        string
	Source      string
	ScriptLabel string
}

type existingScripts struct {
	ByLabel      map[string]existingScript
	ByTrigger    map[string]existingScript
	ByMapSetFlag map[string]existingScript
}

type importDecision struct {
	MapName     string          `json:"mapName"`
	ScriptLabel string          `json:"scriptLabel"`
	Status      string          `json:"status"`
	Reason      string          `json:"reason,omitempty"`
	Path        string          `json:"path,omitempty"`
	Details     json.RawMessage `json:"details,omitempty"`
}

type extractorDiagnostic struct {
	MapName     string          `json:"mapName"`
	ScriptLabel string          `json:"scriptLabel"`
	Status      string          `json:"status"`
	Reason      string          `json:"reason"`
	Details     json.RawMessage `json:"details"`
}

type importReport struct {
	SQLitePath           string                `json:"sqlitePath"`
	OutputDir            string                `json:"outputDir"`
	DryRun               bool                  `json:"dryRun"`
	Stats                importStats           `json:"stats"`
	Summary              importReportSummary   `json:"summary"`
	Decisions            []importDecision      `json:"decisions"`
	ExtractorDiagnostics []extractorDiagnostic `json:"extractorDiagnostics"`
}

type importReportSummary struct {
	DecisionsByStatus        map[string]int `json:"decisionsByStatus"`
	DecisionsByReason        map[string]int `json:"decisionsByReason"`
	ExtractorByStatus        map[string]int `json:"extractorByStatus"`
	ExtractorByReason        map[string]int `json:"extractorByReason"`
	GeneratedByAdapter       map[string]int `json:"generatedByAdapter"`
	SkippedOverridesByReason map[string]int `json:"skippedOverridesByReason"`
	UnsupportedByReason      map[string]int `json:"unsupportedByReason"`
}

func main() {
	opts := parseFlags()
	stats, err := run(opts)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Script candidate import complete: read=%d written=%d unchanged=%d skipped_overrides=%d skipped_unsupported=%d tile_override_candidates=%d tile_override_rules=%d conditional_dialogue_candidates=%d conditional_dialogue_rules=%d extractor_generated=%d extractor_unsupported=%d extractor_ambiguous=%d",
		stats.Read, stats.Written, stats.Unchanged, stats.SkippedOverrides, stats.SkippedUnsupported,
		stats.TileOverrideRead, stats.TileOverrideRules, stats.ConditionalDialogueRead, stats.ConditionalDialogueRules,
		stats.ExtractorGenerated, stats.ExtractorUnsupported, stats.ExtractorAmbiguous)
}

func parseFlags() importOptions {
	defaultSQLite := defaultSQLitePath()
	defaultOutput := defaultOutputDir()
	defaultDiagnostics := defaultDiagnosticsPath(defaultOutput)
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

	return importOptions{
		SQLitePath:      sqlitePath,
		OutputDir:       *outputFlag,
		DiagnosticsPath: *diagnosticsFlag,
		DryRun:          *dryRunFlag,
	}
}

func run(opts importOptions) (importStats, error) {
	if opts.SQLitePath == "" {
		return importStats{}, fmt.Errorf("missing SQLite path")
	}
	if opts.OutputDir == "" {
		return importStats{}, fmt.Errorf("missing output directory")
	}

	db, err := sql.Open("sqlite", opts.SQLitePath)
	if err != nil {
		return importStats{}, fmt.Errorf("open SQLite: %w", err)
	}
	defer db.Close()

	extractorDiagnostics, err := loadExtractorDiagnostics(db)
	if err != nil {
		return importStats{}, err
	}
	stats := statsFromExtractorDiagnostics(extractorDiagnostics)
	decisions := []importDecision{}

	candidates, err := loadCandidates(db)
	if err != nil {
		return importStats{}, err
	}
	stats.Read = len(candidates)

	coordResolver, err := newCoordinateResolver(db)
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
			log.Printf("[ScriptCandidates] Skipping %s: %v", candidate.ScriptLabel, err)
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
			log.Printf("[ScriptCandidates] Preserving CaptureQuest override for %s at %s", event.ScriptLabel, existingByLabel.Path)
			continue
		}

		if current := existing.ByTrigger[triggerKeyForEvent(event)]; existingByLabel.ScriptLabel == "" && current.ScriptLabel != "" && current.ScriptLabel != event.ScriptLabel && !canShareTriggerWithExistingExtractorBranch(event, current) {
			if merged, changed, err := mergeExtractorAudioIntoExisting(current, event, opts.DryRun); err != nil {
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
			log.Printf("[ScriptCandidates] Preserving existing trigger owner for %s at %s", event.ScriptLabel, current.Path)
			continue
		}

		if current, flag := existing.ownerForMapSetFlag(event); existingByLabel.ScriptLabel == "" && current.ScriptLabel != "" && current.ScriptLabel != event.ScriptLabel && !canShareMapSetFlagWithExistingExtractorBattle(event, current) {
			if merged, changed, err := mergeExtractorAudioIntoExisting(current, event, opts.DryRun); err != nil {
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
			log.Printf("[ScriptCandidates] Preserving existing map flag owner for %s (%s) at %s", event.ScriptLabel, flag, current.Path)
			continue
		}

		path := filepath.Join(opts.OutputDir, scriptFileName(event.ScriptLabel))
		if existingByLabel.Path != "" {
			path = existingByLabel.Path
		}
		changed, err := writeEventFile(path, event, opts.DryRun)
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

	tileDecisions, err := importTileOverrideCandidates(db, opts, &stats)
	if err != nil {
		return stats, err
	}
	decisions = append(decisions, tileDecisions...)

	visibilityDecisions, err := importObjectVisibilityCandidates(db, opts, &stats)
	if err != nil {
		return stats, err
	}
	decisions = append(decisions, visibilityDecisions...)

	conditionalDecisions, err := importConditionalDialogueCandidates(db, opts, &stats)
	if err != nil {
		return stats, err
	}
	decisions = append(decisions, conditionalDecisions...)

	if err := writeImportReport(opts, stats, decisions, extractorDiagnostics); err != nil {
		return stats, err
	}
	return stats, nil
}

func canShareTriggerWithExistingExtractorBranch(event scriptedevents.EventFile, current existingScript) bool {
	if current.Source == capturequestSource {
		return false
	}
	if current.ScriptLabel == "" || current.ScriptLabel == event.ScriptLabel {
		return false
	}
	return event.RequiresFlag != "" ||
		event.RequiresFlagAbsent != "" ||
		event.RequiresItemID != nil ||
		event.RequiresItemAbsentID != nil ||
		event.RequiresItemName != "" ||
		event.RequiresItemAbsentName != "" ||
		event.RequiresPokedexCaught != nil ||
		event.RequiresMoney != nil ||
		event.RequiresMoneyBelow != nil ||
		event.RequiresCoins != nil ||
		event.RequiresCoinsBelow != nil ||
		event.RequiresPlayerFacing != ""
}

func canShareMapSetFlagWithExistingExtractorBattle(event scriptedevents.EventFile, current existingScript) bool {
	if current.Source == capturequestSource || event.Trigger.Source != extractorSource {
		return false
	}
	for _, raw := range event.Actions {
		var action candidateAction
		if err := json.Unmarshal(raw, &action); err != nil {
			continue
		}
		if action.Type == "startTrainerBattle" && (action.WinFlag != "" || len(action.PostWinActions) > 0) {
			return true
		}
	}
	return false
}

func mergeExtractorAudioIntoExisting(current existingScript, generated scriptedevents.EventFile, dryRun bool) (bool, bool, error) {
	if current.Source == capturequestSource || current.Path == "" {
		return false, false, nil
	}
	sfxActions := playSFXActionsForEvent(generated)
	if len(sfxActions) == 0 || !eventHasActionType(generated, "giveItem") {
		return false, false, nil
	}

	raw, err := os.ReadFile(current.Path)
	if err != nil {
		return false, false, fmt.Errorf("read existing extractor script %s: %w", current.Path, err)
	}
	var existingEvent scriptedevents.EventFile
	if err := json.Unmarshal(raw, &existingEvent); err != nil {
		return false, false, fmt.Errorf("decode existing extractor script %s: %w", current.Path, err)
	}
	if existingEvent.Trigger.Source == capturequestSource || !eventHasActionType(existingEvent, "giveItem") {
		return false, false, nil
	}

	merged, changedActions := mergePlaySFXActions(existingEvent.Actions, sfxActions)
	if !changedActions {
		return true, false, nil
	}
	existingEvent.Actions = merged

	changed, err := writeEventFile(current.Path, existingEvent, dryRun)
	if err != nil {
		return false, false, err
	}
	return true, changed, nil
}

func mergePlaySFXActions(actions []json.RawMessage, wanted []candidateAction) ([]json.RawMessage, bool) {
	wantedCounts := map[string]int{}
	for _, action := range wanted {
		wantedCounts[playSFXActionKey(action)]++
	}

	removedCounts := map[string]int{}
	filtered := make([]json.RawMessage, 0, len(actions))
	for _, raw := range actions {
		var action candidateAction
		if err := json.Unmarshal(raw, &action); err == nil && action.Type == "playSFX" {
			key := playSFXActionKey(action)
			if removedCounts[key] < wantedCounts[key] {
				removedCounts[key]++
				continue
			}
		}
		filtered = append(filtered, raw)
	}

	toInsert := []candidateAction{}
	for _, action := range wanted {
		toInsert = append(toInsert, action)
	}
	if len(toInsert) == 0 {
		return actions, false
	}

	insertAt := rewardAudioInsertIndex(filtered)
	merged := make([]json.RawMessage, 0, len(filtered)+len(toInsert))
	merged = append(merged, filtered[:insertAt]...)
	for _, action := range toInsert {
		mapped := map[string]any{
			"type":        "playSFX",
			"sfxConstant": action.SFXConstant,
		}
		if action.Volume > 0 {
			mapped["volume"] = action.Volume
		}
		merged = append(merged, rawAction(mapped))
	}
	merged = append(merged, filtered[insertAt:]...)
	return merged, !rawActionSlicesEqual(actions, merged)
}

func rewardAudioInsertIndex(actions []json.RawMessage) int {
	giveItemAt := firstActionTypeIndex(actions, "giveItem")
	unlockAt := firstActionTypeIndex(actions, "unlockInput")
	if unlockAt >= 0 && (giveItemAt < 0 || unlockAt < giveItemAt) {
		return unlockAt
	}
	if giveItemAt >= 0 {
		return giveItemAt
	}
	setFlagAt := firstActionTypeIndex(actions, "setFlag")
	if setFlagAt >= 0 {
		return setFlagAt
	}
	return len(actions)
}

func rawActionSlicesEqual(a, b []json.RawMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func playSFXActionsForEvent(event scriptedevents.EventFile) []candidateAction {
	actions := []candidateAction{}
	for _, raw := range event.Actions {
		var action candidateAction
		if err := json.Unmarshal(raw, &action); err != nil || action.Type != "playSFX" || action.SFXConstant == "" {
			continue
		}
		actions = append(actions, action)
	}
	return actions
}

func playSFXActionKey(action candidateAction) string {
	return fmt.Sprintf("%s\x00%.4f", action.SFXConstant, action.Volume)
}

func eventHasActionType(event scriptedevents.EventFile, actionType string) bool {
	return firstActionTypeIndex(event.Actions, actionType) >= 0
}

func firstActionTypeIndex(actions []json.RawMessage, actionType string) int {
	for i, raw := range actions {
		var action candidateAction
		if err := json.Unmarshal(raw, &action); err != nil {
			continue
		}
		if action.Type == actionType {
			return i
		}
	}
	return -1
}

func loadCandidates(db *sql.DB) ([]scriptCandidate, error) {
	exists, err := sqliteTableExists(db, "script_event_candidates")
	if err != nil {
		return nil, fmt.Errorf("check script_event_candidates table: %w", err)
	}
	if !exists {
		log.Printf("[ScriptCandidates] SQLite has no script_event_candidates table; skipping")
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT candidate_json
		FROM script_event_candidates
		ORDER BY map_name, script_label, id`)
	if err != nil {
		return nil, fmt.Errorf("query script_event_candidates: %w", err)
	}
	defer rows.Close()

	candidates := []scriptCandidate{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var candidate scriptCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, fmt.Errorf("decode candidate JSON: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return candidates, nil
}

func loadTileOverrideCandidates(db *sql.DB) ([]tileOverrideCandidate, bool, error) {
	exists, err := sqliteTableExists(db, "script_event_tile_overrides")
	if err != nil {
		return nil, false, fmt.Errorf("check script_event_tile_overrides table: %w", err)
	}
	if !exists {
		log.Printf("[ScriptCandidates] SQLite has no script_event_tile_overrides table; skipping generated event tiles")
		return nil, false, nil
	}

	rows, err := db.Query(`
		SELECT candidate_json
		FROM script_event_tile_overrides
		ORDER BY map_name, script_label, id`)
	if err != nil {
		return nil, true, fmt.Errorf("query script_event_tile_overrides: %w", err)
	}
	defer rows.Close()

	candidates := []tileOverrideCandidate{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, true, err
		}
		var candidate tileOverrideCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, true, fmt.Errorf("decode tile override candidate JSON: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return candidates, true, nil
}

func importTileOverrideCandidates(db *sql.DB, opts importOptions, stats *importStats) ([]importDecision, error) {
	candidates, exists, err := loadTileOverrideCandidates(db)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	stats.TileOverrideRead = len(candidates)

	decisions := []importDecision{}
	if len(candidates) == 0 {
		changed, err := writeGeneratedEventTileOverrideFile(generatedEventTileOverridesPath(opts.OutputDir), nil, opts.DryRun)
		if err != nil {
			return nil, err
		}
		recordTileOverrideFileChange(stats, changed)
		return decisions, nil
	}

	resolver, err := newTileOverrideResolver(db)
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
	changed, err := writeGeneratedEventTileOverrideFile(generatedEventTileOverridesPath(opts.OutputDir), generatedRules, opts.DryRun)
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

func recordTileOverrideFileChange(stats *importStats, changed bool) {
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

func writeGeneratedEventTileOverrideFile(path string, rules []scriptedevents.EventTileOverrideRule, dryRun bool) (bool, error) {
	file := scriptedevents.EventTileOverrideFile{Tiles: rules}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode generated event tile overrides: %w", err)
	}
	raw = append(raw, '\n')

	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, raw) {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("read existing %s: %w", path, err)
	}
	if dryRun {
		log.Printf("[ScriptCandidates] Would write %s", path)
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("create generated event tile dir: %w", err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	log.Printf("[ScriptCandidates] Wrote %s", path)
	return true, nil
}

func loadObjectVisibilityCandidates(db *sql.DB) ([]objectVisibilityCandidate, bool, error) {
	exists, err := sqliteTableExists(db, "script_event_object_visibility")
	if err != nil {
		return nil, false, fmt.Errorf("check script_event_object_visibility table: %w", err)
	}
	if !exists {
		log.Printf("[ScriptCandidates] SQLite has no script_event_object_visibility table; skipping generated object visibility")
		return nil, false, nil
	}

	rows, err := db.Query(`
		SELECT rule_json
		FROM script_event_object_visibility
		ORDER BY map_name, object_name, requires_event, label`)
	if err != nil {
		return nil, true, fmt.Errorf("query script_event_object_visibility: %w", err)
	}
	defer rows.Close()

	candidates := []objectVisibilityCandidate{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, true, err
		}
		var candidate objectVisibilityCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, true, fmt.Errorf("decode object visibility candidate JSON: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return candidates, true, nil
}

func importObjectVisibilityCandidates(db *sql.DB, opts importOptions, stats *importStats) ([]importDecision, error) {
	candidates, exists, err := loadObjectVisibilityCandidates(db)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	stats.ObjectVisibilityRead = len(candidates)

	rules := []scriptedevents.ObjectVisibilityRule{}
	decisions := []importDecision{}
	for _, candidate := range candidates {
		rule, err := mapObjectVisibilityCandidate(candidate)
		if err != nil {
			stats.ObjectVisibilityUnsupported++
			decisions = append(decisions, importDecision{
				MapName:     candidate.MapName,
				ScriptLabel: candidate.ScriptLabel,
				Status:      "unsupported",
				Reason:      err.Error(),
			})
			continue
		}
		rules = append(rules, rule)
	}
	sortObjectVisibilityRulesForImport(rules)
	stats.ObjectVisibilityRules = len(rules)
	changed, err := writeGeneratedObjectVisibilityFile(generatedObjectVisibilityPath(opts.OutputDir), rules, opts.DryRun)
	if err != nil {
		return nil, err
	}
	if changed {
		stats.ObjectVisibilityWritten = 1
	} else {
		stats.ObjectVisibilityUnchanged = 1
	}
	status := "unchanged"
	if changed {
		status = "generated"
	}
	for _, rule := range rules {
		decisions = append(decisions, importDecision{
			MapName:     rule.MapName,
			ScriptLabel: rule.Label,
			Status:      status,
			Reason:      "object_visibility_candidate",
			Path:        generatedObjectVisibilityPath(opts.OutputDir),
			Details: rawDetails(map[string]any{
				"objectName":         rule.ObjectName,
				"visible":            rule.Visible,
				"requiresFlag":       rule.RequiresFlag,
				"requiresFlagAbsent": rule.RequiresFlagAbsent,
			}),
		})
	}
	return decisions, nil
}

func mapObjectVisibilityCandidate(candidate objectVisibilityCandidate) (scriptedevents.ObjectVisibilityRule, error) {
	if candidate.Kind != "" && candidate.Kind != "objectVisibility" {
		return scriptedevents.ObjectVisibilityRule{}, fmt.Errorf("unsupported object visibility kind %q", candidate.Kind)
	}
	if candidate.MapID == 0 || candidate.MapName == "" {
		return scriptedevents.ObjectVisibilityRule{}, fmt.Errorf("object visibility missing map identity")
	}
	if candidate.ObjectName == "" {
		return scriptedevents.ObjectVisibilityRule{}, fmt.Errorf("object visibility missing objectName")
	}
	if candidate.RequiresEvent == "" {
		return scriptedevents.ObjectVisibilityRule{}, fmt.Errorf("object visibility missing requiresEvent")
	}
	label := candidate.Label
	if label == "" {
		label = fmt.Sprintf("%s:%s", candidate.ScriptLabel, candidate.ObjectName)
	}
	return scriptedevents.ObjectVisibilityRule{
		MapID:        candidate.MapID,
		MapName:      candidate.MapName,
		ObjectName:   candidate.ObjectName,
		Visible:      candidate.Visible,
		RequiresFlag: candidate.RequiresEvent,
		Label:        label,
	}, nil
}

func generatedObjectVisibilityPath(outputDir string) string {
	return filepath.Join(filepath.Dir(outputDir), generatedObjectVisibilityFileName)
}

func writeGeneratedObjectVisibilityFile(path string, rules []scriptedevents.ObjectVisibilityRule, dryRun bool) (bool, error) {
	raw, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode generated object visibility: %w", err)
	}
	raw = append(raw, '\n')

	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, raw) {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("read existing %s: %w", path, err)
	}
	if dryRun {
		log.Printf("[ScriptCandidates] Would write %s", path)
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("create generated object visibility dir: %w", err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	log.Printf("[ScriptCandidates] Wrote %s", path)
	return true, nil
}

func sortObjectVisibilityRulesForImport(rules []scriptedevents.ObjectVisibilityRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].MapID != rules[j].MapID {
			return rules[i].MapID < rules[j].MapID
		}
		if rules[i].MapName != rules[j].MapName {
			return rules[i].MapName < rules[j].MapName
		}
		if rules[i].ObjectName != rules[j].ObjectName {
			return rules[i].ObjectName < rules[j].ObjectName
		}
		if rules[i].RequiresFlag != rules[j].RequiresFlag {
			return rules[i].RequiresFlag < rules[j].RequiresFlag
		}
		if rules[i].Visible != rules[j].Visible {
			return !rules[i].Visible
		}
		return rules[i].Label < rules[j].Label
	})
}

func loadConditionalDialogueCandidates(db *sql.DB) ([]conditionalDialogueCandidate, bool, error) {
	exists, err := sqliteTableExists(db, "script_event_conditional_dialogue")
	if err != nil {
		return nil, false, fmt.Errorf("check script_event_conditional_dialogue table: %w", err)
	}
	if !exists {
		log.Printf("[ScriptCandidates] SQLite has no script_event_conditional_dialogue table; skipping generated conditional dialogue")
		return nil, false, nil
	}

	rows, err := db.Query(`
		SELECT row_json
		FROM script_event_conditional_dialogue
		ORDER BY map_name, text_constant, priority DESC, id`)
	if err != nil {
		return nil, true, fmt.Errorf("query script_event_conditional_dialogue: %w", err)
	}
	defer rows.Close()

	candidates := []conditionalDialogueCandidate{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, true, err
		}
		var candidate conditionalDialogueCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, true, fmt.Errorf("decode conditional dialogue candidate JSON: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return candidates, true, nil
}

func importConditionalDialogueCandidates(db *sql.DB, opts importOptions, stats *importStats) ([]importDecision, error) {
	candidates, exists, err := loadConditionalDialogueCandidates(db)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	stats.ConditionalDialogueRead = len(candidates)

	rules := []scriptedevents.ConditionalDialogueRule{}
	decisions := []importDecision{}
	for _, candidate := range candidates {
		rule, err := mapConditionalDialogueCandidate(candidate)
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
		rules = append(rules, rule)
	}
	sortConditionalDialogueRulesForImport(rules)
	stats.ConditionalDialogueRules = len(rules)
	changed, err := writeGeneratedConditionalDialogueFile(generatedConditionalDialoguePath(opts.OutputDir), rules, opts.DryRun)
	if err != nil {
		return nil, err
	}
	if changed {
		stats.ConditionalDialogueWritten = 1
	} else {
		stats.ConditionalDialogueUnchanged = 1
	}
	status := "unchanged"
	if changed {
		status = "generated"
	}
	for _, rule := range rules {
		decisions = append(decisions, importDecision{
			MapName:     "",
			ScriptLabel: rule.TextConstant,
			Status:      status,
			Reason:      "conditional_dialogue_candidate",
			Path:        generatedConditionalDialoguePath(opts.OutputDir),
			Details: rawDetails(map[string]any{
				"priority":            rule.Priority,
				"requiresFlags":       rule.RequiresFlags,
				"requiresFlagsAbsent": rule.RequiresFlagsAbsent,
				"dialogueLabels":      rule.DialogueLabels,
			}),
		})
	}
	return decisions, nil
}

func mapConditionalDialogueCandidate(candidate conditionalDialogueCandidate) (scriptedevents.ConditionalDialogueRule, error) {
	if candidate.Kind != "" && candidate.Kind != "conditionalDialogue" {
		return scriptedevents.ConditionalDialogueRule{}, fmt.Errorf("unsupported conditional dialogue kind %q", candidate.Kind)
	}
	if candidate.TextConstant == "" {
		return scriptedevents.ConditionalDialogueRule{}, fmt.Errorf("conditional dialogue missing textConstant")
	}
	if len(candidate.DialogueLabels) == 0 {
		return scriptedevents.ConditionalDialogueRule{}, fmt.Errorf("conditional dialogue missing dialogueLabels")
	}
	requiresFlags, err := mapPositiveConditions(candidate.Conditions)
	if err != nil {
		return scriptedevents.ConditionalDialogueRule{}, err
	}
	requiresFlagsAbsent, err := mapAbsentConditions(candidate.Conditions)
	if err != nil {
		return scriptedevents.ConditionalDialogueRule{}, err
	}
	return scriptedevents.ConditionalDialogueRule{
		TextConstant:        candidate.TextConstant,
		Priority:            candidate.Priority,
		RequiresFlags:       requiresFlags,
		RequiresFlagsAbsent: requiresFlagsAbsent,
		DialogueLabels:      compactStrings(candidate.DialogueLabels),
		Source:              candidate.Source,
	}, nil
}

func generatedConditionalDialoguePath(outputDir string) string {
	return filepath.Join(filepath.Dir(outputDir), generatedConditionalDialogueFileName)
}

func writeGeneratedConditionalDialogueFile(path string, rules []scriptedevents.ConditionalDialogueRule, dryRun bool) (bool, error) {
	file := scriptedevents.ConditionalDialogueFile{Rows: rules}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode generated conditional dialogue: %w", err)
	}
	raw = append(raw, '\n')

	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, raw) {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("read existing %s: %w", path, err)
	}
	if dryRun {
		log.Printf("[ScriptCandidates] Would write %s", path)
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("create generated conditional dialogue dir: %w", err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	log.Printf("[ScriptCandidates] Wrote %s", path)
	return true, nil
}

func sortConditionalDialogueRulesForImport(rules []scriptedevents.ConditionalDialogueRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].TextConstant != rules[j].TextConstant {
			return rules[i].TextConstant < rules[j].TextConstant
		}
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		return strings.Join(rules[i].DialogueLabels, "\x00") < strings.Join(rules[j].DialogueLabels, "\x00")
	})
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func rawDetails(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}

type sourceMapMeta struct {
	ID        int
	Name      string
	TilesetID int
	Overworld bool
}

type coordinateOffset struct {
	X int
	Y int
}

type coordinateResolver struct {
	maps    map[string]sourceMapMeta
	offsets map[string]coordinateOffset
}

func newCoordinateResolver(db *sql.DB) (*coordinateResolver, error) {
	exists, err := sqliteTableExists(db, "maps")
	if err != nil {
		return nil, fmt.Errorf("check maps table: %w", err)
	}
	if !exists {
		return &coordinateResolver{
			maps:    map[string]sourceMapMeta{},
			offsets: map[string]coordinateOffset{},
		}, nil
	}
	maps, err := loadSourceMaps(db)
	if err != nil {
		return nil, err
	}
	offsets, err := loadOverworldMapOffsets(db)
	if err != nil {
		return nil, err
	}
	return &coordinateResolver{maps: maps, offsets: offsets}, nil
}

func (resolver *coordinateResolver) Normalize(coords []scriptedevents.EventCoordinate, mapName string) []scriptedevents.EventCoordinate {
	if len(coords) == 0 {
		return nil
	}
	normalized := make([]scriptedevents.EventCoordinate, len(coords))
	for i, coord := range coords {
		normalized[i] = coord
		if normalized[i].MapName == "" {
			normalized[i].MapName = mapName
		}
		coordMapName := mapNameToUpperSnake(normalized[i].MapName)
		if coordMapName == "" {
			coordMapName = mapName
		}
		normalized[i].MapName = coordMapName
		if resolver == nil {
			continue
		}
		meta, ok := resolver.maps[coordMapName]
		if !ok || !meta.Overworld {
			if normalized[i].MapID == 0 && ok {
				normalized[i].MapID = meta.ID
			}
			continue
		}
		if offset, ok := resolver.offsets[coordMapName]; ok {
			normalized[i].X += offset.X
			normalized[i].Y += offset.Y
			normalized[i].MapID = 9999
			normalized[i].MapName = coordMapName
		}
	}
	return normalized
}

type tileOverrideResolver struct {
	maps                   map[string]sourceMapMeta
	blocksets              map[int]map[int][]byte
	tilesetTiles           map[int]map[int][]byte
	collisionTiles         map[int]map[int]bool
	tileImageIDBySignature map[string]int
}

func newTileOverrideResolver(db *sql.DB) (*tileOverrideResolver, error) {
	for _, table := range []string{"maps", "blocksets", "tile_images", "tileset_tiles"} {
		exists, err := sqliteTableExists(db, table)
		if err != nil {
			return nil, fmt.Errorf("check %s table: %w", table, err)
		}
		if !exists {
			return nil, fmt.Errorf("missing required %s table", table)
		}
	}

	resolver := &tileOverrideResolver{}
	var err error
	if resolver.maps, err = loadSourceMaps(db); err != nil {
		return nil, err
	}
	if resolver.blocksets, err = loadSourceBlocksets(db); err != nil {
		return nil, err
	}
	if resolver.tilesetTiles, err = loadSourceTilesetTiles(db); err != nil {
		return nil, err
	}
	if resolver.collisionTiles, err = loadSourceCollisionTiles(db); err != nil {
		return nil, err
	}
	if resolver.tileImageIDBySignature, err = loadTileImageSignatures(db, resolver.tilesetTiles); err != nil {
		return nil, err
	}
	return resolver, nil
}

func (resolver *tileOverrideResolver) MapCandidate(candidate tileOverrideCandidate) ([]scriptedevents.EventTileOverrideRule, error) {
	if candidate.Kind != "" && candidate.Kind != "eventTileOverrideCandidate" {
		return nil, fmt.Errorf("unsupported tile override kind %q", candidate.Kind)
	}
	if candidate.MapName == "" || candidate.ScriptLabel == "" {
		return nil, fmt.Errorf("tile override candidate missing mapName or scriptLabel")
	}
	if len(candidate.Replacements) == 0 {
		return nil, fmt.Errorf("tile override candidate has no replacements")
	}

	mapName := mapNameToUpperSnake(candidate.MapName)
	meta, ok := resolver.maps[mapName]
	if !ok {
		return nil, fmt.Errorf("unknown map %s", mapName)
	}
	blocksetID := sourceBlocksetTilesetID(meta.TilesetID)
	rules := []scriptedevents.EventTileOverrideRule{}
	for _, replacement := range candidate.Replacements {
		if replacement.LabelPrefix == "" {
			return nil, fmt.Errorf("%s replacement missing labelPrefix", candidate.ScriptLabel)
		}
		requiresFlag := mapEventName(replacement.RequiresEvent)
		requiresFlagAbsent := mapEventName(replacement.RequiresEventAbsent)
		if requiresFlag != "" && requiresFlagAbsent != "" {
			return nil, fmt.Errorf("%s replacement %s has both requiresEvent and requiresEventAbsent", candidate.ScriptLabel, replacement.LabelPrefix)
		}
		blockData := resolver.blocksets[blocksetID][replacement.BlockID]
		if len(blockData) == 0 {
			return nil, fmt.Errorf("%s replacement %s references missing block %d for tileset %d", candidate.ScriptLabel, replacement.LabelPrefix, replacement.BlockID, blocksetID)
		}
		for position := 0; position < 4; position++ {
			signature, err := renderTileQuadrantSignature(blockData, position, blocksetID, resolver.tilesetTiles)
			if err != nil {
				return nil, fmt.Errorf("%s replacement %s position %d: %w", candidate.ScriptLabel, replacement.LabelPrefix, position, err)
			}
			tileImageID := resolver.tileImageIDBySignature[signature]
			if tileImageID == 0 {
				return nil, fmt.Errorf("%s replacement %s position %d has no matching tile image", candidate.ScriptLabel, replacement.LabelPrefix, position)
			}
			dx := position % 2
			dy := position / 2
			rules = append(rules, scriptedevents.EventTileOverrideRule{
				MapID:              meta.ID,
				MapName:            meta.Name,
				X:                  replacement.BlockX*2 + dx,
				Y:                  replacement.BlockY*2 + dy,
				TileImageID:        tileImageID,
				CollisionType:      resolver.collisionType(blockData, position, meta.TilesetID),
				RequiresFlag:       requiresFlag,
				RequiresFlagAbsent: requiresFlagAbsent,
				Label:              fmt.Sprintf("%s_%d_%d", replacement.LabelPrefix, dx, dy),
			})
		}
	}
	return rules, nil
}

func loadSourceMaps(db *sql.DB) (map[string]sourceMapMeta, error) {
	hasOverworld, err := sqliteColumnExists(db, "maps", "is_overworld")
	if err != nil {
		return nil, fmt.Errorf("check maps.is_overworld: %w", err)
	}
	query := `SELECT id, name, tileset_id, 0 FROM maps`
	if hasOverworld {
		query = `SELECT id, name, tileset_id, is_overworld FROM maps`
	}
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query maps: %w", err)
	}
	defer rows.Close()

	result := map[string]sourceMapMeta{}
	for rows.Next() {
		var meta sourceMapMeta
		var tilesetID sql.NullInt64
		var isOverworld sql.NullInt64
		if err := rows.Scan(&meta.ID, &meta.Name, &tilesetID, &isOverworld); err != nil {
			return nil, fmt.Errorf("scan map: %w", err)
		}
		if tilesetID.Valid {
			meta.TilesetID = int(tilesetID.Int64)
		}
		meta.Overworld = isOverworld.Valid && isOverworld.Int64 != 0
		result[mapNameToUpperSnake(meta.Name)] = meta
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read maps: %w", err)
	}
	return result, nil
}

func loadOverworldMapOffsets(db *sql.DB) (map[string]coordinateOffset, error) {
	exists, err := sqliteTableExists(db, "overworld_map_positions")
	if err != nil {
		return nil, fmt.Errorf("check overworld_map_positions table: %w", err)
	}
	if !exists {
		return map[string]coordinateOffset{}, nil
	}
	rows, err := db.Query(`SELECT map_name, x_offset, y_offset FROM overworld_map_positions`)
	if err != nil {
		return nil, fmt.Errorf("query overworld map offsets: %w", err)
	}
	defer rows.Close()

	result := map[string]coordinateOffset{}
	for rows.Next() {
		var mapName string
		var x, y int
		if err := rows.Scan(&mapName, &x, &y); err != nil {
			return nil, fmt.Errorf("scan overworld map offset: %w", err)
		}
		result[mapNameToUpperSnake(mapName)] = coordinateOffset{X: x, Y: y}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read overworld map offsets: %w", err)
	}
	return result, nil
}

func loadSourceBlocksets(db *sql.DB) (map[int]map[int][]byte, error) {
	rows, err := db.Query(`SELECT tileset_id, block_index, block_data FROM blocksets`)
	if err != nil {
		return nil, fmt.Errorf("query blocksets: %w", err)
	}
	defer rows.Close()

	result := map[int]map[int][]byte{}
	for rows.Next() {
		var tilesetID, blockIndex int
		var blockData []byte
		if err := rows.Scan(&tilesetID, &blockIndex, &blockData); err != nil {
			return nil, fmt.Errorf("scan blockset: %w", err)
		}
		if result[tilesetID] == nil {
			result[tilesetID] = map[int][]byte{}
		}
		result[tilesetID][blockIndex] = blockData
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read blocksets: %w", err)
	}
	return result, nil
}

func loadSourceTilesetTiles(db *sql.DB) (map[int]map[int][]byte, error) {
	rows, err := db.Query(`SELECT tileset_id, tile_index, tile_data FROM tileset_tiles`)
	if err != nil {
		return nil, fmt.Errorf("query tileset_tiles: %w", err)
	}
	defer rows.Close()

	result := map[int]map[int][]byte{}
	for rows.Next() {
		var tilesetID, tileIndex int
		var tileData []byte
		if err := rows.Scan(&tilesetID, &tileIndex, &tileData); err != nil {
			return nil, fmt.Errorf("scan tileset tile: %w", err)
		}
		if result[tilesetID] == nil {
			result[tilesetID] = map[int][]byte{}
		}
		result[tilesetID][tileIndex] = tileData
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tileset tiles: %w", err)
	}
	return result, nil
}

func loadSourceCollisionTiles(db *sql.DB) (map[int]map[int]bool, error) {
	exists, err := sqliteTableExists(db, "collision_tiles")
	if err != nil {
		return nil, fmt.Errorf("check collision_tiles table: %w", err)
	}
	result := map[int]map[int]bool{}
	if !exists {
		return result, nil
	}
	rows, err := db.Query(`SELECT tileset_id, tile_id FROM collision_tiles`)
	if err != nil {
		return nil, fmt.Errorf("query collision_tiles: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tilesetID, tileID int
		if err := rows.Scan(&tilesetID, &tileID); err != nil {
			return nil, fmt.Errorf("scan collision tile: %w", err)
		}
		if result[tilesetID] == nil {
			result[tilesetID] = map[int]bool{}
		}
		result[tilesetID][tileID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read collision tiles: %w", err)
	}
	return result, nil
}

func loadTileImageSignatures(db *sql.DB, tilesetTiles map[int]map[int][]byte) (map[string]int, error) {
	rows, err := db.Query(`
		SELECT ti.id, ti.tileset_id, ti.position, bs.block_data
		FROM tile_images ti
		JOIN blocksets bs
		  ON bs.tileset_id = ti.tileset_id AND bs.block_index = ti.block_index
		ORDER BY ti.id`)
	if err != nil {
		return nil, fmt.Errorf("query tile image signatures: %w", err)
	}
	defer rows.Close()

	result := map[string]int{}
	for rows.Next() {
		var id, tilesetID, position int
		var blockData []byte
		if err := rows.Scan(&id, &tilesetID, &position, &blockData); err != nil {
			return nil, fmt.Errorf("scan tile image signature: %w", err)
		}
		signature, err := renderTileQuadrantSignature(blockData, position, tilesetID, tilesetTiles)
		if err != nil {
			return nil, fmt.Errorf("tile image %d: %w", id, err)
		}
		if result[signature] == 0 {
			result[signature] = id
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tile image signatures: %w", err)
	}
	return result, nil
}

func (resolver *tileOverrideResolver) collisionType(blockData []byte, position int, rawTilesetID int) int {
	for _, index := range quadrantSubTileIndices(position) {
		if index < len(blockData) && (blockData[index] == 0x14 || blockData[index] == 0x48) {
			return 2
		}
	}
	footTileID, ok := phaserdata.RawFootTileIDFromBlockData(blockData, position)
	if !ok {
		return 0
	}
	if resolver.collisionTiles[rawTilesetID][footTileID] {
		return 1
	}
	return 0
}

func renderTileQuadrantSignature(blockData []byte, position int, tilesetID int, tilesetTiles map[int]map[int][]byte) (string, error) {
	indices := quadrantSubTileIndices(position)
	if len(indices) == 0 {
		return "", fmt.Errorf("invalid block quadrant position %d", position)
	}
	pixels := make([]byte, 16*16)
	for i, blockIndex := range indices {
		if blockIndex >= len(blockData) {
			return "", fmt.Errorf("block data has %d bytes, need index %d", len(blockData), blockIndex)
		}
		tileID := int(blockData[blockIndex])
		tileData := tilesetTiles[tilesetID][tileID]
		if len(tileData) < 16 {
			return "", fmt.Errorf("missing 2bpp tile data for tileset %d tile %#x", tilesetID, tileID)
		}
		offsetX := (i % 2) * 8
		offsetY := (i / 2) * 8
		writeDecodedTilePixels(pixels, offsetX, offsetY, tileData)
	}
	return string(pixels), nil
}

func writeDecodedTilePixels(dest []byte, offsetX, offsetY int, tileData []byte) {
	for row := 0; row < 8; row++ {
		lo := tileData[row*2]
		hi := tileData[row*2+1]
		for bit := 0; bit < 8; bit++ {
			shift := 7 - bit
			value := ((hi >> shift) & 1) << 1
			value |= (lo >> shift) & 1
			dest[(offsetY+row)*16+offsetX+bit] = value
		}
	}
}

func quadrantSubTileIndices(position int) []int {
	switch position {
	case 0:
		return []int{0, 1, 4, 5}
	case 1:
		return []int{2, 3, 6, 7}
	case 2:
		return []int{8, 9, 12, 13}
	case 3:
		return []int{10, 11, 14, 15}
	default:
		return nil
	}
}

func sourceBlocksetTilesetID(tilesetID int) int {
	switch tilesetID {
	case 2:
		return 6
	case 5:
		return 7
	default:
		return tilesetID
	}
}

func sortEventTileRulesForImport(rules []scriptedevents.EventTileOverrideRule) {
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].MapID != rules[j].MapID {
			return rules[i].MapID < rules[j].MapID
		}
		if rules[i].MapName != rules[j].MapName {
			return rules[i].MapName < rules[j].MapName
		}
		if rules[i].X != rules[j].X {
			return rules[i].X < rules[j].X
		}
		if rules[i].Y != rules[j].Y {
			return rules[i].Y < rules[j].Y
		}
		if rules[i].RequiresFlag != rules[j].RequiresFlag {
			return rules[i].RequiresFlag < rules[j].RequiresFlag
		}
		if rules[i].RequiresFlagAbsent != rules[j].RequiresFlagAbsent {
			return rules[i].RequiresFlagAbsent < rules[j].RequiresFlagAbsent
		}
		return rules[i].Label < rules[j].Label
	})
}

func eventTileRuleKeyForImport(rule scriptedevents.EventTileOverrideRule) string {
	return fmt.Sprintf("%d|%s|%d|%d|%s|%s", rule.MapID, rule.MapName, rule.X, rule.Y, rule.RequiresFlag, rule.RequiresFlagAbsent)
}

func loadExtractorDiagnostics(db *sql.DB) ([]extractorDiagnostic, error) {
	exists, err := sqliteTableExists(db, "script_event_candidate_diagnostics")
	if err != nil {
		return nil, fmt.Errorf("check script_event_candidate_diagnostics table: %w", err)
	}
	if !exists {
		return nil, nil
	}

	rows, err := db.Query(`
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

func statsFromExtractorDiagnostics(diagnostics []extractorDiagnostic) importStats {
	stats := importStats{ExtractorDiagnostics: len(diagnostics)}
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

func writeImportReport(opts importOptions, stats importStats, decisions []importDecision, extractorDiagnostics []extractorDiagnostic) error {
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
	if opts.DryRun {
		log.Printf("[ScriptCandidates] Dry run; not writing diagnostics report %s", opts.DiagnosticsPath)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(opts.DiagnosticsPath), 0755); err != nil {
		return fmt.Errorf("create diagnostics dir: %w", err)
	}
	if err := os.WriteFile(opts.DiagnosticsPath, raw, 0644); err != nil {
		return fmt.Errorf("write diagnostics report %s: %w", opts.DiagnosticsPath, err)
	}
	log.Printf("[ScriptCandidates] Wrote diagnostics report %s", opts.DiagnosticsPath)
	return nil
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

func sqliteTableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return name == table, nil
}

func sqliteColumnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
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

func mapCandidate(candidate scriptCandidate) (scriptedevents.EventFile, error) {
	return mapCandidateWithResolver(candidate, nil)
}

func mapCandidateWithResolver(candidate scriptCandidate, coordResolver *coordinateResolver) (scriptedevents.EventFile, error) {
	if candidate.Kind != "" && candidate.Kind != "scriptEventCandidate" {
		return scriptedevents.EventFile{}, fmt.Errorf("unsupported kind %q", candidate.Kind)
	}
	if candidate.MapName == "" || candidate.ScriptLabel == "" {
		return scriptedevents.EventFile{}, fmt.Errorf("missing mapName or scriptLabel")
	}
	if candidate.Trigger.Type == "" || candidate.Trigger.Label == "" {
		return scriptedevents.EventFile{}, fmt.Errorf("missing trigger type or label")
	}
	if candidate.Trigger.Type != "coord" && candidate.Trigger.Type != "map_script" && candidate.Trigger.Type != "npc_click" {
		return scriptedevents.EventFile{}, fmt.Errorf("unsupported trigger type %q", candidate.Trigger.Type)
	}

	actionsSource, warp, err := splitTopLevelWarpAction(candidate.Actions)
	if err != nil {
		return scriptedevents.EventFile{}, err
	}
	actions, err := mapActions(actionsSource)
	if err != nil {
		return scriptedevents.EventFile{}, err
	}
	if len(actions) == 0 {
		return scriptedevents.EventFile{}, fmt.Errorf("candidate has no supported actions")
	}

	mapName := mapNameToUpperSnake(candidate.MapName)
	coordinates := normalizeCandidateCoordinates(candidate.Trigger.Coordinates, mapName, coordResolver)
	requiresFlags, err := mapPositiveConditions(candidate.Conditions)
	if err != nil {
		return scriptedevents.EventFile{}, err
	}
	requiresFlagsAbsent, err := mapAbsentConditions(candidate.Conditions)
	if err != nil {
		return scriptedevents.EventFile{}, err
	}
	requiresFlag, requiresFlagList := splitScalarCondition(requiresFlags)
	requiresFlagAbsent, requiresFlagAbsentList := splitScalarCondition(requiresFlagsAbsent)
	event := scriptedevents.EventFile{
		ScriptLabel: candidate.ScriptLabel,
		MapName:     mapName,
		Trigger: scriptedevents.EventTrigger{
			Type:        candidate.Trigger.Type,
			Source:      extractorSource,
			Label:       candidate.Trigger.Label,
			Coordinates: coordinates,
		},
		RequiresFlag:           requiresFlag,
		RequiresFlagAbsent:     requiresFlagAbsent,
		RequiresFlags:          requiresFlagList,
		RequiresFlagsAbsent:    requiresFlagAbsentList,
		RequiresItemName:       candidate.Conditions.RequiresItem,
		RequiresItemAbsentName: candidate.Conditions.RequiresItemAbsent,
		RequiresPlayerFacing:   normalizeCandidateDirection(candidate.Conditions.RequiresPlayerFacing),
		Actions:                actions,
		Warp:                   warp,
	}
	if candidate.Conditions.RequiresPlayerFacing != "" && event.RequiresPlayerFacing == "" {
		return scriptedevents.EventFile{}, fmt.Errorf("unsupported requiresPlayerFacing %q", candidate.Conditions.RequiresPlayerFacing)
	}
	if candidate.Conditions.RequiresPokedexCaught > 0 {
		event.RequiresPokedexCaught = &candidate.Conditions.RequiresPokedexCaught
	}
	if candidate.Conditions.RequiresMoney > 0 {
		event.RequiresMoney = &candidate.Conditions.RequiresMoney
	}
	if candidate.Conditions.RequiresMoneyBelow > 0 {
		event.RequiresMoneyBelow = &candidate.Conditions.RequiresMoneyBelow
	}
	if candidate.Conditions.RequiresCoins > 0 {
		event.RequiresCoins = &candidate.Conditions.RequiresCoins
	}
	if candidate.Conditions.RequiresCoinsBelow > 0 {
		event.RequiresCoinsBelow = &candidate.Conditions.RequiresCoinsBelow
	}
	return event, nil
}

func splitTopLevelWarpAction(actions []candidateAction) ([]candidateAction, *scriptedevents.EventWarp, error) {
	result := make([]candidateAction, 0, len(actions))
	var warp *scriptedevents.EventWarp
	for _, action := range actions {
		if action.Type != "warp" {
			result = append(result, action)
			continue
		}
		if warp != nil {
			return nil, nil, fmt.Errorf("candidate has multiple warp actions")
		}
		if action.MapID <= 0 {
			return nil, nil, fmt.Errorf("warp missing mapId")
		}
		warp = &scriptedevents.EventWarp{
			MapID: action.MapID,
			X:     action.X,
			Y:     action.Y,
		}
	}
	return result, warp, nil
}

func normalizeCandidateCoordinates(coords []scriptedevents.EventCoordinate, mapName string, coordResolver *coordinateResolver) []scriptedevents.EventCoordinate {
	return coordResolver.Normalize(coords, mapName)
}

func normalizeCandidateDirection(direction string) string {
	switch strings.ToUpper(strings.TrimSpace(direction)) {
	case "UP", "DOWN", "LEFT", "RIGHT":
		return strings.ToUpper(strings.TrimSpace(direction))
	default:
		return ""
	}
}

func mapActions(actions []candidateAction) ([]json.RawMessage, error) {
	result := []json.RawMessage{}
	for _, action := range actions {
		switch action.Type {
		case "lockInput", "unlockInput", "endSafariSession", "healParty":
			result = append(result, rawAction(map[string]any{"type": action.Type}))
		case "delay", "screenFade":
			result = append(result, rawAction(map[string]any{
				"type": action.Type,
				"ms":   action.MS,
			}))
		case "dialogue":
			if len(action.Lines) == 0 {
				continue
			}
			result = append(result, rawAction(map[string]any{
				"type":    "dialogue",
				"speaker": action.Speaker,
				"lines":   action.Lines,
			}))
		case "dialogueText":
			if action.TextConstant == "" {
				return nil, fmt.Errorf("dialogueText missing textConstant")
			}
			result = append(result, rawAction(map[string]any{
				"type":         "dialogueText",
				"speaker":      action.Speaker,
				"textConstant": action.TextConstant,
			}))
		case "playSFX":
			if action.SFXConstant == "" {
				return nil, fmt.Errorf("playSFX missing sfxConstant")
			}
			mapped := map[string]any{
				"type":        "playSFX",
				"sfxConstant": action.SFXConstant,
			}
			if action.Volume > 0 {
				mapped["volume"] = action.Volume
			}
			result = append(result, rawAction(mapped))
		case "playCry":
			if action.PokemonName == "" && action.PokemonConstant == "" && action.SFXConstant == "" {
				return nil, fmt.Errorf("playCry missing pokemonName/pokemonConstant/sfxConstant")
			}
			mapped := map[string]any{
				"type":            "playCry",
				"pokemonName":     action.PokemonName,
				"pokemonConstant": action.PokemonConstant,
				"sfxConstant":     action.SFXConstant,
			}
			if action.Volume > 0 {
				mapped["volume"] = action.Volume
			}
			result = append(result, rawAction(mapped))
		case "choice":
			prompt, prelude := splitPromptLines(action)
			if len(prelude) > 0 {
				result = append(result, rawAction(map[string]any{
					"type":    "dialogue",
					"speaker": action.Speaker,
					"lines":   prelude,
				}))
			}
			choice := map[string]any{
				"type":         "choice",
				"speaker":      action.Speaker,
				"prompt":       prompt,
				"textConstant": action.TextConstant,
			}
			if len(action.NoLines) > 0 {
				choice["noLines"] = action.NoLines
			}
			if len(action.YesLines) > 0 && action.StopOnYes {
				choice["yesLines"] = action.YesLines
			}
			if action.ContinueOnNo {
				choice["continueOnNo"] = true
			}
			if action.StopOnYes {
				choice["stopOnYes"] = true
			}
			result = append(result, rawAction(choice))
			if len(action.YesLines) > 0 && !action.StopOnYes {
				result = append(result, rawAction(map[string]any{
					"type":    "dialogue",
					"speaker": action.Speaker,
					"lines":   action.YesLines,
				}))
			}
		case "setEvent", "setFlag":
			flag := actionFlag(action)
			if flag == "" {
				return nil, fmt.Errorf("%s missing event/flag", action.Type)
			}
			result = append(result, rawAction(map[string]any{"type": "setFlag", "flag": flag}))
		case "resetEvent", "resetFlag":
			flag := actionFlag(action)
			if flag == "" {
				return nil, fmt.Errorf("%s missing event/flag", action.Type)
			}
			result = append(result, rawAction(map[string]any{"type": "resetFlag", "flag": flag}))
		case "toggleEvent", "toggleFlag":
			flag := actionFlag(action)
			if flag == "" {
				return nil, fmt.Errorf("%s missing event/flag", action.Type)
			}
			result = append(result, rawAction(map[string]any{"type": "toggleFlag", "flag": flag}))
		case "giveItem", "takeItem":
			mapped, err := mapItemAction(action)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		case "takeMoney":
			if action.Money <= 0 {
				return nil, fmt.Errorf("takeMoney missing money")
			}
			result = append(result, rawAction(map[string]any{
				"type":  "takeMoney",
				"money": action.Money,
			}))
		case "giveCoins":
			if action.Coins <= 0 {
				return nil, fmt.Errorf("giveCoins missing coins")
			}
			result = append(result, rawAction(map[string]any{
				"type":  "giveCoins",
				"coins": action.Coins,
			}))
		case "gameCornerPrizeVendor":
			if action.PrizeWindow <= 0 {
				return nil, fmt.Errorf("gameCornerPrizeVendor missing prizeWindow")
			}
			result = append(result, rawAction(map[string]any{
				"type":         "gameCornerPrizeVendor",
				"textConstant": action.TextConstant,
				"prizeWindow":  action.PrizeWindow,
			}))
		case "givePokemon":
			mapped, err := mapGivePokemonAction(action)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		case "hideObject", "showObject":
			mapped, err := mapObjectAction(action)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		case "move", "movePlayer":
			if len(action.Movements) == 0 {
				return nil, fmt.Errorf("%s missing movements", action.Type)
			}
			mapped := map[string]any{
				"type":      action.Type,
				"actor":     action.Actor,
				"movements": action.Movements,
			}
			result = append(result, rawAction(mapped))
		case "showActor":
			if action.Actor == "" {
				return nil, fmt.Errorf("showActor missing actor")
			}
			result = append(result, rawAction(map[string]any{
				"type":  "showActor",
				"actor": action.Actor,
				"x":     action.X,
				"y":     action.Y,
			}))
		case "hideActor":
			if action.Actor == "" {
				return nil, fmt.Errorf("hideActor missing actor")
			}
			result = append(result, rawAction(map[string]any{
				"type":  "hideActor",
				"actor": action.Actor,
			}))
		case "facePlayer":
			if action.Actor == "" || action.Direction == "" {
				return nil, fmt.Errorf("facePlayer missing actor/direction")
			}
			result = append(result, rawAction(map[string]any{
				"type":      "facePlayer",
				"actor":     action.Actor,
				"direction": action.Direction,
			}))
		case "startTrainerBattle":
			mapped, err := mapStartTrainerBattleAction(action)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		case "startWildBattle":
			mapped, err := mapStartWildBattleAction(action)
			if err != nil {
				return nil, err
			}
			result = append(result, mapped)
		case "startSafariSession":
			if len(action.Lines) > 0 {
				result = append(result, rawAction(map[string]any{
					"type":    "dialogue",
					"speaker": action.Speaker,
					"lines":   action.Lines,
				}))
			}
			if action.Destination == nil {
				return nil, fmt.Errorf("startSafariSession missing destination")
			}
			result = append(result, rawAction(map[string]any{
				"type":      "startSafariSession",
				"mapId":     action.Destination.MapID,
				"x":         action.Destination.X,
				"y":         action.Destination.Y,
				"direction": action.Destination.Direction,
			}))
		default:
			return nil, fmt.Errorf("unsupported action type %q", action.Type)
		}
	}
	return result, nil
}

func actionFlag(action candidateAction) string {
	if action.Flag != "" {
		return mapEventName(action.Flag)
	}
	return mapEventName(action.Event)
}

func mapPositiveConditions(conditions candidateCondition) ([]string, error) {
	return mapConditionList(
		conditions.RequiresEvent,
		conditions.RequiresEvents,
		conditions.RequiresBadge,
		conditions.RequiresBadges,
	)
}

func mapAbsentConditions(conditions candidateCondition) ([]string, error) {
	return mapConditionList(
		conditions.RequiresEventAbsent,
		conditions.RequiresEventsAbsent,
		conditions.RequiresBadgeAbsent,
		conditions.RequiresBadgesAbsent,
	)
}

func mapConditionList(eventFlag string, eventFlags []string, badgeFlag string, badgeFlags []string) ([]string, error) {
	flags := []string{}
	if mapped := mapEventName(eventFlag); mapped != "" {
		flags = append(flags, mapped)
	}
	for _, event := range eventFlags {
		if mapped := mapEventName(event); mapped != "" {
			flags = append(flags, mapped)
		}
	}
	if mapped := mapBadgeName(badgeFlag); mapped != "" {
		flags = append(flags, mapped)
	}
	for _, badge := range badgeFlags {
		if mapped := mapBadgeName(badge); mapped != "" {
			flags = append(flags, mapped)
		}
	}
	flags = uniqueStrings(flags)
	sort.Strings(flags)
	return flags, nil
}

func splitScalarCondition(flags []string) (string, []string) {
	if len(flags) == 1 {
		return flags[0], nil
	}
	return "", flags
}

func mapEventName(event string) string {
	switch strings.TrimSpace(event) {
	case "EVENT_BEAT_MT_MOON_EXIT_SUPER_NERD":
		return "EVENT_BEAT_MT_MOON_SUPER_NERD"
	case "EVENT_BEAT_ROUTE22_RIVAL_1ST_BATTLE":
		return "EVENT_ROUTE22_RIVAL_1"
	case "EVENT_BEAT_ROUTE22_RIVAL_2ND_BATTLE":
		return "EVENT_ROUTE22_RIVAL_2"
	case "EVENT_PASSED_CASCADEBADGE_CHECK":
		return "EVENT_ROUTE23_BADGE2_CHECKED"
	case "EVENT_PASSED_THUNDERBADGE_CHECK":
		return "EVENT_ROUTE23_BADGE3_CHECKED"
	case "EVENT_PASSED_RAINBOWBADGE_CHECK":
		return "EVENT_ROUTE23_BADGE4_CHECKED"
	case "EVENT_PASSED_SOULBADGE_CHECK":
		return "EVENT_ROUTE23_BADGE5_CHECKED"
	case "EVENT_PASSED_MARSHBADGE_CHECK":
		return "EVENT_ROUTE23_BADGE6_CHECKED"
	case "EVENT_PASSED_VOLCANOBADGE_CHECK":
		return "EVENT_ROUTE23_BADGE7_CHECKED"
	case "EVENT_PASSED_EARTHBADGE_CHECK":
		return "EVENT_ROUTE23_BADGE8_CHECKED"
	default:
		return strings.TrimSpace(event)
	}
}

func mapBadgeName(badge string) string {
	badge = strings.TrimSpace(badge)
	if badge == "" {
		return ""
	}
	return "EVENT_GOT_" + badge
}

func mapItemAction(action candidateAction) (json.RawMessage, error) {
	itemName := action.ItemName
	if itemName == "" {
		itemName = action.ItemConstant
	}
	if action.ItemID <= 0 && itemName == "" {
		return nil, fmt.Errorf("%s missing itemId/itemName/itemConstant", action.Type)
	}
	quantity := action.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	mapped := map[string]any{
		"type":     action.Type,
		"quantity": quantity,
	}
	if action.ItemID > 0 {
		mapped["itemId"] = action.ItemID
	} else {
		mapped["itemName"] = itemName
	}
	return rawAction(mapped), nil
}

func mapGivePokemonAction(action candidateAction) (json.RawMessage, error) {
	pokemonID := action.PokemonID
	if pokemonID <= 0 {
		pokemonID = action.SpeciesID
	}
	pokemonName := action.PokemonName
	if pokemonName == "" {
		pokemonName = action.PokemonConstant
	}
	if pokemonID <= 0 && pokemonName == "" {
		return nil, fmt.Errorf("givePokemon missing pokemonId/speciesId/pokemonName/pokemonConstant")
	}
	level := action.Level
	if level <= 0 {
		level = 5
	}
	mapped := map[string]any{
		"type":    "givePokemon",
		"level":   level,
		"message": action.Message,
	}
	if pokemonID > 0 {
		mapped["pokemonId"] = pokemonID
	} else {
		mapped["pokemonName"] = pokemonName
	}
	return rawAction(mapped), nil
}

func mapObjectAction(action candidateAction) (json.RawMessage, error) {
	if action.ObjectID <= 0 && action.ObjectKey == "" && action.TriggerLabel == "" && action.TextConstant == "" {
		return nil, fmt.Errorf("%s missing objectId/objectKey/triggerLabel/textConstant", action.Type)
	}
	return rawAction(map[string]any{
		"type":          action.Type,
		"objectId":      action.ObjectID,
		"objectKey":     action.ObjectKey,
		"objectMapName": action.ObjectMapName,
		"triggerLabel":  action.TriggerLabel,
		"textConstant":  action.TextConstant,
	}), nil
}

func mapStartTrainerBattleAction(action candidateAction) (json.RawMessage, error) {
	if action.TrainerClass == "" {
		return nil, fmt.Errorf("startTrainerBattle missing trainerClass")
	}
	if action.TrainerPartyIndex <= 0 && len(action.PartyByFlag) == 0 {
		return nil, fmt.Errorf("startTrainerBattle missing partyIndex/partyByFlag")
	}
	postWinActions, err := mapActions(action.PostWinActions)
	if err != nil {
		return nil, fmt.Errorf("postWinActions: %w", err)
	}
	postLoseActions, err := mapActions(action.PostLoseActions)
	if err != nil {
		return nil, fmt.Errorf("postLoseActions: %w", err)
	}
	return rawAction(map[string]any{
		"type":             "startTrainerBattle",
		"trainerClass":     action.TrainerClass,
		"partyIndex":       action.TrainerPartyIndex,
		"partyByFlag":      action.PartyByFlag,
		"trainerName":      action.TrainerName,
		"trainerObjectId":  action.TrainerObjectID,
		"winFlag":          action.WinFlag,
		"loseFlag":         action.LoseFlag,
		"lossMessage":      action.LossMessage,
		"noBlackoutOnLoss": action.NoBlackoutOnLoss,
		"postWinActions":   postWinActions,
		"postLoseActions":  postLoseActions,
	}), nil
}

func mapStartWildBattleAction(action candidateAction) (json.RawMessage, error) {
	pokemonID := action.PokemonID
	if pokemonID <= 0 {
		pokemonID = action.SpeciesID
	}
	pokemonName := action.PokemonName
	if pokemonName == "" {
		pokemonName = action.PokemonConstant
	}
	if pokemonID <= 0 && pokemonName == "" {
		return nil, fmt.Errorf("startWildBattle missing pokemonId/speciesId/pokemonName/pokemonConstant")
	}
	if action.Level <= 0 {
		return nil, fmt.Errorf("startWildBattle missing level")
	}
	postWinActions, err := mapActions(action.PostWinActions)
	if err != nil {
		return nil, fmt.Errorf("postWinActions: %w", err)
	}
	mapped := map[string]any{
		"type":           "startWildBattle",
		"level":          action.Level,
		"winFlag":        action.WinFlag,
		"postWinActions": postWinActions,
	}
	if len(action.AllowedActions) > 0 {
		mapped["allowedActions"] = action.AllowedActions
	}
	if action.GuaranteedCatch {
		mapped["guaranteedCatch"] = true
	}
	if pokemonID > 0 {
		mapped["pokemonId"] = pokemonID
	} else {
		mapped["pokemonName"] = pokemonName
	}
	return rawAction(mapped), nil
}

func splitPromptLines(action candidateAction) (string, []string) {
	if action.Prompt != "" {
		return action.Prompt, nil
	}
	lines := compactLines(action.PromptLines)
	if len(lines) == 0 {
		lines = compactLines(action.Lines)
	}
	if len(lines) == 0 {
		return "Do you want this?", nil
	}
	if len(lines) == 1 {
		return lines[0], nil
	}
	if len(lines) >= 2 && strings.HasSuffix(lines[len(lines)-2], "to") {
		prompt := strings.TrimSpace(lines[len(lines)-2] + " " + lines[len(lines)-1])
		return prompt, lines[:len(lines)-2]
	}
	return lines[len(lines)-1], lines[:len(lines)-1]
}

func compactLines(lines []string) []string {
	result := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func rawAction(value map[string]any) json.RawMessage {
	for key, field := range value {
		switch typed := field.(type) {
		case nil:
			delete(value, key)
		case string:
			if typed == "" {
				delete(value, key)
			}
		case []string:
			if len(typed) == 0 {
				delete(value, key)
			}
		case []json.RawMessage:
			if len(typed) == 0 {
				delete(value, key)
			}
		case map[string]int:
			if len(typed) == 0 {
				delete(value, key)
			}
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(raw)
}

func loadExistingScripts(outputDir string) (existingScripts, error) {
	result := existingScripts{
		ByLabel:      map[string]existingScript{},
		ByTrigger:    map[string]existingScript{},
		ByMapSetFlag: map[string]existingScript{},
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return existingScripts{}, fmt.Errorf("read output dir %s: %w", outputDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(outputDir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return existingScripts{}, fmt.Errorf("read %s: %w", path, err)
		}
		var event scriptedevents.EventFile
		if err := json.Unmarshal(raw, &event); err != nil {
			return existingScripts{}, fmt.Errorf("decode %s: %w", path, err)
		}
		if event.ScriptLabel == "" {
			continue
		}
		record := existingScript{
			Path:        path,
			Source:      event.Trigger.Source,
			ScriptLabel: event.ScriptLabel,
		}
		result.ByLabel[event.ScriptLabel] = record
		if key := triggerKeyForEvent(event); key != "" {
			if current, ok := result.ByTrigger[key]; !ok || current.Source != capturequestSource || record.Source == capturequestSource {
				result.ByTrigger[key] = record
			}
		}
		for _, flag := range setFlagsForEvent(event) {
			if key := mapSetFlagKey(event.MapName, flag); key != "" {
				if current, ok := result.ByMapSetFlag[key]; !ok || current.Source != capturequestSource || record.Source == capturequestSource {
					result.ByMapSetFlag[key] = record
				}
			}
		}
	}
	return result, nil
}

func (scripts existingScripts) ownerForMapSetFlag(event scriptedevents.EventFile) (existingScript, string) {
	for _, flag := range setFlagsForEvent(event) {
		if current := scripts.ByMapSetFlag[mapSetFlagKey(event.MapName, flag)]; current.ScriptLabel != "" {
			return current, flag
		}
	}
	return existingScript{}, ""
}

func setFlagsForEvent(event scriptedevents.EventFile) []string {
	flags := append([]string{}, event.SetsFlags...)
	for _, raw := range event.Actions {
		var action candidateAction
		if err := json.Unmarshal(raw, &action); err != nil {
			continue
		}
		flags = append(flags, setFlagsForAction(action)...)
	}
	return uniqueStrings(flags)
}

func setFlagsForAction(action candidateAction) []string {
	flags := []string{}
	switch action.Type {
	case "setFlag", "setEvent":
		flag := action.Flag
		if flag == "" {
			flag = action.Event
		}
		if flag != "" {
			flags = append(flags, flag)
		}
	case "startTrainerBattle", "startWildBattle":
		if action.WinFlag != "" {
			flags = append(flags, action.WinFlag)
		}
		for _, nested := range action.PostWinActions {
			flags = append(flags, setFlagsForAction(nested)...)
		}
	}
	return flags
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func triggerKeyForEvent(event scriptedevents.EventFile) string {
	if event.MapName == "" || event.Trigger.Type == "" || event.Trigger.Label == "" {
		return ""
	}
	return triggerKey(event.MapName, event.Trigger.Type, event.Trigger.Label)
}

func mapSetFlagKey(mapName, flag string) string {
	mapName = mapNameToUpperSnake(mapName)
	flag = strings.TrimSpace(flag)
	if mapName == "" || flag == "" {
		return ""
	}
	return mapName + "\x00" + flag
}

func triggerKey(mapName, triggerType, triggerLabel string) string {
	parts := []string{
		mapNameToUpperSnake(mapName),
		strings.ToLower(strings.TrimSpace(triggerType)),
		strings.TrimSpace(triggerLabel),
	}
	for _, part := range parts {
		if part == "" {
			return ""
		}
	}
	return strings.Join(parts, "\x00")
}

func writeEventFile(path string, event scriptedevents.EventFile, dryRun bool) (bool, error) {
	raw, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode %s: %w", event.ScriptLabel, err)
	}
	raw = append(raw, '\n')

	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, raw) {
		return false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("read existing %s: %w", path, err)
	}
	if dryRun {
		log.Printf("[ScriptCandidates] Would write %s", path)
		return true, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	log.Printf("[ScriptCandidates] Wrote %s", path)
	return true, nil
}

func scriptFileName(label string) string {
	return camelToSnake(label) + ".json"
}

func camelToUpperSnake(value string) string {
	return strings.ToUpper(camelToSnake(value))
}

func mapNameToUpperSnake(value string) string {
	converted := camelToUpperSnake(value)
	converted = floorSuffixPattern.ReplaceAllString(converted, "_${1}F")
	return basementFloorPattern.ReplaceAllString(converted, "_B${1}F")
}

var acronymBoundaryPattern = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)
var lowerUpperBoundaryPattern = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var alphaDigitBoundaryPattern = regexp.MustCompile(`([A-Za-z])([0-9])`)
var nonIdentifierPattern = regexp.MustCompile(`[^A-Za-z0-9]+`)
var floorSuffixPattern = regexp.MustCompile(`_((?:B)?\d+)_F\b`)
var basementFloorPattern = regexp.MustCompile(`_B_(\d+)F\b`)

func camelToSnake(value string) string {
	value = strings.TrimSpace(value)
	value = nonIdentifierPattern.ReplaceAllString(value, "_")
	value = acronymBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	value = lowerUpperBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	value = alphaDigitBoundaryPattern.ReplaceAllString(value, `${1}_${2}`)
	value = strings.Trim(value, "_")
	return strings.ToLower(value)
}

func defaultSQLitePath() string {
	candidates := []string{
		filepath.Join("..", "public", "phaser", "pokemon.db"),
		filepath.Join("public", "phaser", "pokemon.db"),
		filepath.Join("..", "..", "public", "phaser", "pokemon.db"),
		filepath.Join("..", "..", "..", "public", "phaser", "pokemon.db"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func defaultOutputDir() string {
	for _, candidate := range []string{
		filepath.Join("scripted_events", "scripts"),
		filepath.Join("server", "scripted_events", "scripts"),
		filepath.Join("..", "scripted_events", "scripts"),
		filepath.Join("..", "..", "server", "scripted_events", "scripts"),
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return filepath.Join("scripted_events", "scripts")
}

func defaultDiagnosticsPath(outputDir string) string {
	return filepath.Join(filepath.Dir(outputDir), "script_candidate_import_diagnostics.json")
}
