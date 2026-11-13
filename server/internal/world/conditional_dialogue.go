package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"capturequest/internal/db"
)

// conditionalDialogueOverride is the result of a conditional dialogue check.
type conditionalDialogueOverride struct {
	label    string
	dialogue string
}

// checkConditionalDialogue checks if a text constant has a conditional override
// based on the player's event flags. Returns the highest-priority matching
// override, or nil if no conditions match (use default dialogue).
func checkConditionalDialogue(ctx context.Context, database db.ContextDBTX, textConstant string, charID int64, efm *EventFlagManager) (*conditionalDialogueOverride, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT id, requires_flag, requires_flag_absent, requires_flags, requires_flags_absent, override_dialogue
		FROM phaser_conditional_dialogue
		WHERE text_constant = $1
		ORDER BY priority DESC`, textConstant)
	if err != nil {
		return nil, fmt.Errorf("conditional dialogue %s query: %w", textConstant, err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var reqFlag, reqFlagAbsent sql.NullString
		var reqFlagsRaw, reqFlagsAbsentRaw []byte
		var dialogue string

		if err := rows.Scan(&id, &reqFlag, &reqFlagAbsent, &reqFlagsRaw, &reqFlagsAbsentRaw, &dialogue); err != nil {
			return nil, fmt.Errorf("conditional dialogue %s row %d: %w", textConstant, id, err)
		}

		requiredFlags, err := decodeConditionalFlagList(reqFlagsRaw)
		if err != nil {
			return nil, fmt.Errorf("conditional dialogue %s row %d requires_flags: %w", textConstant, id, err)
		}
		if reqFlag.Valid && strings.TrimSpace(reqFlag.String) != "" {
			requiredFlags = append(requiredFlags, strings.TrimSpace(reqFlag.String))
		}
		requiredAbsentFlags, err := decodeConditionalFlagList(reqFlagsAbsentRaw)
		if err != nil {
			return nil, fmt.Errorf("conditional dialogue %s row %d requires_flags_absent: %w", textConstant, id, err)
		}
		if reqFlagAbsent.Valid && strings.TrimSpace(reqFlagAbsent.String) != "" {
			requiredAbsentFlags = append(requiredAbsentFlags, strings.TrimSpace(reqFlagAbsent.String))
		}
		if !conditionalFlagsMatch(charID, efm, requiredFlags, requiredAbsentFlags) {
			continue
		}

		// All conditions met — return this override
		return &conditionalDialogueOverride{
			label:    textConstant + "_CONDITIONAL",
			dialogue: dialogue,
		}, nil
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("conditional dialogue %s iteration: %w", textConstant, err)
	}

	return nil, nil
}

func decodeConditionalFlagList(raw []byte) ([]string, error) {
	if len(raw) == 0 || strings.EqualFold(strings.TrimSpace(string(raw)), "null") {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("invalid conditional flag list: %w", err)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result, nil
}

func conditionalFlagsMatch(charID int64, efm *EventFlagManager, requiredFlags []string, requiredAbsentFlags []string) bool {
	if len(requiredFlags) > 0 && (charID == 0 || efm == nil) {
		return false
	}
	for _, flag := range requiredFlags {
		if !efm.CheckFlag(charID, flag) {
			return false
		}
	}
	if len(requiredAbsentFlags) > 0 && (charID == 0 || efm == nil) {
		return true
	}
	for _, flag := range requiredAbsentFlags {
		if efm.CheckFlag(charID, flag) {
			return false
		}
	}
	return true
}
