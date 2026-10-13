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
	result, err := fieldMovePermissionIn(db.GlobalWorldDB.DB, charID, moveName, func(flag string) (bool, error) {
		return characterHasEventFlag(charID, flag, efm), nil
	})
	if err != nil {
		result.Message = fmt.Sprintf("Failed to load field move eligibility: %v", err)
	}
	return result
}

// Durable effects supply their transaction and read badges from that same
// snapshot; presentation/simulator callers can retain their existing flag view.
func fieldMovePermissionIn(database db.DBTX, charID int64, moveName string, hasFlag func(string) (bool, error)) (FieldMovePermissionResult, error) {
	rule, ok := fieldMoveRuleForName(moveName)
	if !ok {
		return FieldMovePermissionResult{
			Allowed:  false,
			Message:  "Unknown field move.",
			MoveName: strings.ToUpper(strings.TrimSpace(moveName)),
		}, nil
	}

	result := FieldMovePermissionResult{
		MoveID:            rule.MoveID,
		MoveName:          rule.MoveName,
		RequiredBadgeFlag: rule.RequiredBadgeFlag,
	}

	knownBySpecies, knownByName, err := fieldMoveKnownByPartyIn(database, charID, rule)
	if err != nil {
		return result, err
	}
	if knownBySpecies == 0 {
		result.Message = "No POKEMON knows that move."
		return result, nil
	}
	result.KnownBySpeciesID = knownBySpecies
	result.KnownByName = knownByName

	if rule.RequiredBadgeFlag != "" {
		on, err := hasFlag(rule.RequiredBadgeFlag)
		if err != nil {
			return result, err
		}
		if !on {
			result.Message = NewBadgeRequiredMessage
			return result, nil
		}
	}

	result.Allowed = true
	return result, nil
}

// State activation joins the caller's character transaction. Permission and
// map-scoped activation must observe one durable owner for both consumers.
func prepareStateFieldMoveIn(tx db.DBTX, charID int64, mapID int, moveName string) (FieldMoveUseResult, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return FieldMoveUseResult{}, err
	}
	moveName = normalizeFieldMoveName(moveName)
	if moveName != "STRENGTH" && moveName != "FLASH" {
		return FieldMoveUseResult{}, fmt.Errorf("unsupported state field move %s", moveName)
	}
	permission, err := fieldMovePermissionIn(tx, charID, moveName, func(flag string) (bool, error) { return queryEventFlag(tx, charID, flag) })
	if err != nil {
		return FieldMoveUseResult{}, err
	}
	result := FieldMoveUseResult{Permission: permission, MoveName: permission.MoveName, MapID: mapID, Message: permission.Message}
	if !permission.Allowed {
		return result, nil
	}
	if err := recordFieldMoveStateIn(tx, charID, mapID, moveName); err != nil {
		return FieldMoveUseResult{}, err
	}
	result.Success = true
	result.Message = permission.MoveName + " is ready."
	if permission.KnownByName != "" {
		if moveName == "STRENGTH" {
			result.Message = fmt.Sprintf("%s used STRENGTH. %s can move boulders.", permission.KnownByName, permission.KnownByName)
		} else {
			result.Message = permission.KnownByName + " used FLASH."
		}
	}
	return result, nil
}

func recordFieldMoveStateIn(tx db.DBTX, charID int64, mapID int, moveName string) error {
	if err := db.RequireTransaction(tx); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT INTO character_field_move_state(character_id,move_name,map_id,active,updated_at) VALUES($1,$2,$3,1,CURRENT_TIMESTAMP) ON CONFLICT(character_id,move_name) DO UPDATE SET map_id=EXCLUDED.map_id,active=1,updated_at=CURRENT_TIMESTAMP`, charID, normalizeFieldMoveName(moveName), mapID)
	return err
}

func fieldMoveRuleForName(moveName string) (fieldMoveRule, bool) {
	rule, ok := fieldMoveRules[normalizeFieldMoveName(moveName)]
	return rule, ok
}

func fieldMoveKnownByPartyIn(database db.DBTX, charID int64, rule fieldMoveRule) (int, string, error) {
	party, err := pokebattle.LoadParty(database, charID)
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
