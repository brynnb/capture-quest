package world

import (
	"fmt"
	"strings"
	"unicode"

	"capturequest/internal/db"
	"capturequest/internal/pokebattle"
)

const NewBadgeRequiredMessage = "No! A new BADGE is required."

type FieldMovePermissionResult struct {
	Allowed           bool
	Message           string
	MoveID            int
	MoveName          string
	RequiredBadgeFlag string
	KnownBySpeciesID  int
	KnownByName       string
}

type FieldMoveUseResult struct {
	Permission FieldMovePermissionResult
	Success    bool
	Message    string
	MoveName   string
	MapID      int
}

type fieldMoveRule struct {
	MoveID            int
	MoveName          string
	RequiredBadgeFlag string
}

// Gen 1 start-menu field move gates from engine/menus/start_sub_menus.asm.
var fieldMoveRules = map[string]fieldMoveRule{
	"CUT":        {MoveID: 15, MoveName: "CUT", RequiredBadgeFlag: "EVENT_GOT_CASCADEBADGE"},
	"FLY":        {MoveID: 19, MoveName: "FLY", RequiredBadgeFlag: "EVENT_GOT_THUNDERBADGE"},
	"SURF":       {MoveID: 57, MoveName: "SURF", RequiredBadgeFlag: "EVENT_GOT_SOULBADGE"},
	"STRENGTH":   {MoveID: 70, MoveName: "STRENGTH", RequiredBadgeFlag: "EVENT_GOT_RAINBOWBADGE"},
	"FLASH":      {MoveID: 148, MoveName: "FLASH", RequiredBadgeFlag: "EVENT_GOT_BOULDERBADGE"},
	"DIG":        {MoveID: 91, MoveName: "DIG"},
	"TELEPORT":   {MoveID: 100, MoveName: "TELEPORT"},
	"SOFTBOILED": {MoveID: 135, MoveName: "SOFTBOILED"},
}

func CanUseFieldMove(charID int64, moveName string, efm *EventFlagManager) FieldMovePermissionResult {
	rule, ok := fieldMoveRuleForName(moveName)
	if !ok {
		return FieldMovePermissionResult{
			Allowed:  false,
			Message:  "Unknown field move.",
			MoveName: strings.ToUpper(strings.TrimSpace(moveName)),
		}
	}

	result := FieldMovePermissionResult{
		MoveID:            rule.MoveID,
		MoveName:          rule.MoveName,
		RequiredBadgeFlag: rule.RequiredBadgeFlag,
	}

	knownBySpecies, knownByName, err := fieldMoveKnownByParty(charID, rule)
	if err != nil {
		result.Message = fmt.Sprintf("Failed to load party: %v", err)
		return result
	}
	if knownBySpecies == 0 {
		result.Message = "No POKEMON knows that move."
		return result
	}
	result.KnownBySpeciesID = knownBySpecies
	result.KnownByName = knownByName

	if rule.RequiredBadgeFlag != "" && !characterHasEventFlag(charID, rule.RequiredBadgeFlag, efm) {
		result.Message = NewBadgeRequiredMessage
		return result
	}

	result.Allowed = true
	return result
}

func TryUseFieldMove(charID int64, mapID int, moveName string, efm *EventFlagManager) FieldMoveUseResult {
	permission := CanUseFieldMove(charID, moveName, efm)
	result := FieldMoveUseResult{
		Permission: permission,
		MoveName:   permission.MoveName,
		MapID:      mapID,
	}
	if !permission.Allowed {
		result.Message = permission.Message
		return result
	}

	switch normalizeFieldMoveName(permission.MoveName) {
	case "STRENGTH":
		if _, err := db.GlobalWorldDB.DB.Exec(`
			INSERT INTO character_field_move_state (character_id, move_name, map_id, active)
			VALUES ($1, 'STRENGTH', $2, 1)
			ON CONFLICT (character_id, move_name) DO UPDATE SET
				map_id = EXCLUDED.map_id,
				active = 1`,
			charID,
			mapID,
		); err != nil {
			result.Message = fmt.Sprintf("Failed to use STRENGTH: %v", err)
			return result
		}
		result.Success = true
		result.Message = fmt.Sprintf("%s used STRENGTH. %s can move boulders.", permission.KnownByName, permission.KnownByName)
	default:
		result.Success = true
		result.Message = permission.MoveName + " is ready."
	}
	return result
}

func IsFieldMoveActive(charID int64, mapID int, moveName string) bool {
	var active int
	if err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT active
		FROM character_field_move_state
		WHERE character_id = $1 AND map_id = $2 AND move_name = $3`,
		charID,
		mapID,
		normalizeFieldMoveName(moveName),
	).Scan(&active); err != nil {
		return false
	}
	return active != 0
}

func fieldMoveRuleForName(moveName string) (fieldMoveRule, bool) {
	rule, ok := fieldMoveRules[normalizeFieldMoveName(moveName)]
	return rule, ok
}

func fieldMoveKnownByParty(charID int64, rule fieldMoveRule) (int, string, error) {
	party, err := pokebattle.LoadParty(db.GlobalWorldDB.DB, charID)
	if err != nil {
		return 0, "", err
	}
	for _, pokemon := range party {
		if pokemon == nil {
			continue
		}
		for _, move := range pokemon.Moves {
			if move.ID == 0 {
				continue
			}
			if move.ID == rule.MoveID || normalizeFieldMoveName(move.Name) == normalizeFieldMoveName(rule.MoveName) {
				return pokemon.ID, pokemon.Name, nil
			}
		}
	}
	return 0, "", nil
}

func normalizeFieldMoveName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
