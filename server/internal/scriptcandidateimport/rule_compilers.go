package scriptcandidateimport

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"capturequest/internal/scriptedevents"
)

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalJSON(left, right any) bool {
	return reflect.DeepEqual(left, right)
}

func loadObjectVisibilityCandidates(ctx context.Context, db *sql.DB) ([]objectVisibilityCandidate, bool, error) {
	exists, err := sqliteTableExists(ctx, db, "script_event_object_visibility")
	if err != nil {
		return nil, false, fmt.Errorf("check script_event_object_visibility table: %w", err)
	}
	if !exists {
		return nil, false, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, map_name, map_id, object_name, object_key, script_label,
		       requires_event, visible, label, rule_json
		FROM script_event_object_visibility
		ORDER BY map_name, object_name, requires_event, label`)
	if err != nil {
		return nil, true, fmt.Errorf("query script_event_object_visibility: %w", err)
	}
	defer rows.Close()

	candidates := []objectVisibilityCandidate{}
	for rows.Next() {
		var id int64
		var mapName, objectName, objectKey, scriptLabel, requiresEvent, label, raw string
		var mapID int
		var visible bool
		if err := rows.Scan(&id, &mapName, &mapID, &objectName, &objectKey, &scriptLabel, &requiresEvent, &visible, &label, &raw); err != nil {
			return nil, true, err
		}
		var candidate objectVisibilityCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, true, fmt.Errorf("decode script_event_object_visibility row %d JSON: %w", id, err)
		}
		if err := requireCandidateVersion("script_event_object_visibility", id, candidate.Version); err != nil {
			return nil, true, err
		}
		if candidate.MapName != mapName || candidate.MapID != mapID || candidate.ObjectName != objectName || candidate.ObjectKey != objectKey || candidate.ScriptLabel != scriptLabel || candidate.RequiresEvent != requiresEvent || candidate.Visible != visible || candidate.Label != label {
			return nil, true, fmt.Errorf("script_event_object_visibility row %d JSON disagrees with relational columns", id)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return candidates, true, nil
}

func importObjectVisibilityCandidates(ctx context.Context, db *sql.DB, opts Options, stats *Stats, plan *outputPlan) ([]importDecision, error) {
	candidates, exists, err := loadObjectVisibilityCandidates(ctx, db)
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
	changed, err := writeGeneratedObjectVisibilityFile(plan, generatedObjectVisibilityPath(opts.OutputDir), rules)
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

func writeGeneratedObjectVisibilityFile(plan *outputPlan, path string, rules []scriptedevents.ObjectVisibilityRule) (bool, error) {
	raw, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode generated object visibility: %w", err)
	}
	raw = append(raw, '\n')

	return plan.Stage(path, raw)
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

func loadConditionalDialogueCandidates(ctx context.Context, db *sql.DB) ([]conditionalDialogueCandidate, bool, error) {
	exists, err := sqliteTableExists(ctx, db, "script_event_conditional_dialogue")
	if err != nil {
		return nil, false, fmt.Errorf("check script_event_conditional_dialogue table: %w", err)
	}
	if !exists {
		return nil, false, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT id, text_constant, map_name, script_label, priority,
		       requires_flags_json, requires_flags_absent_json,
		       dialogue_labels_json, source_json, row_json
		FROM script_event_conditional_dialogue
		ORDER BY map_name, text_constant, priority DESC, id`)
	if err != nil {
		return nil, true, fmt.Errorf("query script_event_conditional_dialogue: %w", err)
	}
	defer rows.Close()

	candidates := []conditionalDialogueCandidate{}
	for rows.Next() {
		var id int64
		var textConstant, mapName, scriptLabel string
		var priority int
		var requires, requiresAbsent, dialogueLabels, source, raw string
		if err := rows.Scan(&id, &textConstant, &mapName, &scriptLabel, &priority, &requires, &requiresAbsent, &dialogueLabels, &source, &raw); err != nil {
			return nil, true, err
		}
		var candidate conditionalDialogueCandidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, true, fmt.Errorf("decode script_event_conditional_dialogue row %d JSON: %w", id, err)
		}
		if err := requireCandidateVersion("script_event_conditional_dialogue", id, candidate.Version); err != nil {
			return nil, true, err
		}
		var relationalRequires, relationalRequiresAbsent, relationalDialogue []string
		var relationalSource map[string]any
		if err := json.Unmarshal([]byte(requires), &relationalRequires); err != nil {
			return nil, true, fmt.Errorf("decode script_event_conditional_dialogue row %d requires_flags_json: %w", id, err)
		}
		if err := json.Unmarshal([]byte(requiresAbsent), &relationalRequiresAbsent); err != nil {
			return nil, true, fmt.Errorf("decode script_event_conditional_dialogue row %d requires_flags_absent_json: %w", id, err)
		}
		if err := json.Unmarshal([]byte(dialogueLabels), &relationalDialogue); err != nil {
			return nil, true, fmt.Errorf("decode script_event_conditional_dialogue row %d dialogue_labels_json: %w", id, err)
		}
		if err := json.Unmarshal([]byte(source), &relationalSource); err != nil {
			return nil, true, fmt.Errorf("decode script_event_conditional_dialogue row %d source_json: %w", id, err)
		}
		if candidate.TextConstant != textConstant || candidate.MapName != mapName || candidate.ScriptLabel != scriptLabel || candidate.Priority != priority || !equalStrings(candidate.Conditions.RequiresEvents, relationalRequires) || !equalStrings(candidate.Conditions.RequiresEventsAbsent, relationalRequiresAbsent) || !equalStrings(candidate.DialogueLabels, relationalDialogue) || !equalJSON(candidate.Source, relationalSource) {
			return nil, true, fmt.Errorf("script_event_conditional_dialogue row %d JSON disagrees with relational columns", id)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, true, err
	}
	return candidates, true, nil
}

func importConditionalDialogueCandidates(ctx context.Context, db *sql.DB, opts Options, stats *Stats, plan *outputPlan) ([]importDecision, error) {
	candidates, exists, err := loadConditionalDialogueCandidates(ctx, db)
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
	changed, err := writeGeneratedConditionalDialogueFile(plan, generatedConditionalDialoguePath(opts.OutputDir), rules)
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

func writeGeneratedConditionalDialogueFile(plan *outputPlan, path string, rules []scriptedevents.ConditionalDialogueRule) (bool, error) {
	file := scriptedevents.ConditionalDialogueFile{Rows: rules}
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return false, fmt.Errorf("encode generated conditional dialogue: %w", err)
	}
	raw = append(raw, '\n')

	return plan.Stage(path, raw)
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
