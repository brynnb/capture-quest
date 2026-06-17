package world

import (
	"database/sql"
	"encoding/json"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
)

// DialogueChoiceRequest is sent when the player makes a YES/NO choice
type DialogueChoiceRequest struct {
	TextConstant string `json:"textConstant"` // The original text constant that prompted the choice
	Choice       bool   `json:"choice"`       // true = YES, false = NO
	ActorID      int    `json:"actorId"`      // The NPC actor ID (for context)
}

// BranchingDialogue represents a dialogue with YES/NO options
type BranchingDialogue struct {
	ID                 int             `json:"id"`
	MapName            sql.NullString  `json:"-"`
	PromptTextConstant string          `json:"promptTextConstant"`
	PromptText         string          `json:"promptText"`
	YesTextConstant    sql.NullString  `json:"-"`
	NoTextConstant     sql.NullString  `json:"-"`
	YesDialogue        sql.NullString  `json:"-"`
	NoDialogue         sql.NullString  `json:"-"`
	RequiresEventFlag  sql.NullString  `json:"-"`
	SetsEventFlag      sql.NullString  `json:"-"`
	YesActions         json.RawMessage `json:"-"`
	NoActions          json.RawMessage `json:"-"`
}

type DialogueChoiceResult struct {
	Choice               bool            `json:"choice"`
	FollowUpDialogue     string          `json:"followUpDialogue"`
	FollowUpTextConstant string          `json:"followUpTextConstant"`
	MapName              string          `json:"mapName"`
	Actions              json.RawMessage `json:"actions"`
}

// GetBranchingDialogue fetches branching dialogue data for a text constant
func GetBranchingDialogue(textConstant string) (*BranchingDialogue, error) {
	var bd BranchingDialogue
	var yesActions, noActions sql.NullString
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, map_name, prompt_text_constant, prompt_text, yes_text_constant, no_text_constant,
			yes_dialogue, no_dialogue, requires_event_flag, sets_event_flag, yes_actions, no_actions
		FROM phaser_branching_dialogue
		WHERE prompt_text_constant = $1`, textConstant).Scan(
		&bd.ID, &bd.MapName, &bd.PromptTextConstant, &bd.PromptText,
		&bd.YesTextConstant, &bd.NoTextConstant,
		&bd.YesDialogue, &bd.NoDialogue,
		&bd.RequiresEventFlag, &bd.SetsEventFlag, &yesActions, &noActions)
	if err != nil {
		return nil, err
	}
	if yesActions.Valid && yesActions.String != "" {
		bd.YesActions = json.RawMessage(yesActions.String)
	}
	if noActions.Valid && noActions.String != "" {
		bd.NoActions = json.RawMessage(noActions.String)
	}
	return &bd, nil
}

func ResolveDialogueChoice(textConstant string, choice bool) (*DialogueChoiceResult, error) {
	bd, err := GetBranchingDialogue(textConstant)
	if err != nil {
		return nil, err
	}

	result := &DialogueChoiceResult{Choice: choice}
	if bd.MapName.Valid {
		result.MapName = bd.MapName.String
	}

	if choice {
		if bd.YesDialogue.Valid && bd.YesDialogue.String != "" {
			result.FollowUpDialogue = bd.YesDialogue.String
		} else if bd.YesTextConstant.Valid && bd.YesTextConstant.String != "" {
			result.FollowUpTextConstant = bd.YesTextConstant.String
			result.FollowUpDialogue = fetchDialogueText(result.FollowUpTextConstant)
		}
		result.Actions, err = dialogueChoiceActions(bd.YesActions, bd.SetsEventFlag)
	} else {
		if bd.NoDialogue.Valid && bd.NoDialogue.String != "" {
			result.FollowUpDialogue = bd.NoDialogue.String
		} else if bd.NoTextConstant.Valid && bd.NoTextConstant.String != "" {
			result.FollowUpTextConstant = bd.NoTextConstant.String
			result.FollowUpDialogue = fetchDialogueText(result.FollowUpTextConstant)
		}
		result.Actions, err = dialogueChoiceActions(bd.NoActions, sql.NullString{})
	}
	if err != nil {
		return nil, err
	}

	return result, nil
}

func dialogueChoiceActions(raw json.RawMessage, legacyYesFlag sql.NullString) (json.RawMessage, error) {
	var actions []CutsceneAction
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &actions); err != nil {
			return nil, err
		}
	}
	if legacyYesFlag.Valid && legacyYesFlag.String != "" {
		actions = append(actions, CutsceneAction{Type: "setFlag", Flag: legacyYesFlag.String})
	}
	if len(actions) == 0 {
		return nil, nil
	}
	return json.Marshal(actions)
}

// HandleDialogueChoiceRequest processes the player's YES/NO choice
func HandleDialogueChoiceRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req DialogueChoiceRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[DialogueChoice] Invalid request: %v", err)
		return false
	}

	log.Printf("[DialogueChoice] Player chose %v for %s", req.Choice, req.TextConstant)

	if handleInGameTradeDialogueChoice(ses, req) {
		return false
	}

	result, err := ResolveDialogueChoice(req.TextConstant, req.Choice)
	if err != nil {
		log.Printf("[DialogueChoice] No branching dialogue for %s: %v", req.TextConstant, err)
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "no branching dialogue found",
		}, opcodes.DialogueChoiceResponse)
		return false
	}

	res := map[string]interface{}{
		"success":              true,
		"choice":               result.Choice,
		"followUpDialogue":     result.FollowUpDialogue,
		"followUpTextConstant": result.FollowUpTextConstant,
	}
	ses.SendStreamJSON(res, opcodes.DialogueChoiceResponse)

	if len(result.Actions) > 0 && ses.HasValidClient() {
		charID := int64(ses.Client.CharData().ID)
		mapName := dialogueChoiceActionMapName(req, ses, result.MapName)
		if err := applyCutsceneServerActionList(ses, mapName, result.Actions, charID, wh); err != nil {
			log.Printf("[DialogueChoice] Failed to apply choice actions for %s: %v", req.TextConstant, err)
			SendSystemMessage(ses, "That choice could not be completed. Please try again.")
		}
	}

	log.Printf("[DialogueChoice] Sent follow-up dialogue for choice=%v", req.Choice)
	return false
}

func dialogueChoiceActionMapName(req DialogueChoiceRequest, ses *session.Session, fallback string) string {
	if req.ActorID > 0 {
		var mapName string
		err := db.GlobalWorldDB.DB.QueryRow(`
			SELECT m.name
			FROM phaser_objects o
			JOIN phaser_maps m ON m.id = o.map_id
			WHERE o.id = $1`, req.ActorID).Scan(&mapName)
		if err == nil && mapName != "" {
			return mapName
		}
	}
	if fallback != "" {
		return fallback
	}
	if ses != nil && ses.MapID > 0 {
		var mapName string
		if err := db.GlobalWorldDB.DB.QueryRow(`SELECT name FROM phaser_maps WHERE id = $1`, ses.MapID).Scan(&mapName); err == nil {
			return mapName
		}
	}
	return ""
}

// fetchDialogueText resolves a text constant to dialogue text
func fetchDialogueText(textConstant string) string {
	var dialogue string
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT dt.dialogue
		FROM phaser_text_pointers tp
		LEFT JOIN phaser_dialogue_text dt ON dt.label = tp.dialogue_label
		WHERE tp.text_constant = $1
		LIMIT 1`, textConstant).Scan(&dialogue)
	if err != nil {
		log.Printf("[DialogueChoice] Could not fetch dialogue for %s: %v", textConstant, err)
		return ""
	}
	return dialogue
}

// CheckForBranchingDialogue checks if a text constant has YES/NO branching.
// If the branching requires an event flag, charID must be provided to check it.
// Pass charID=0 and efm=nil to skip the flag check (e.g., from the dialogue response handler).
func CheckForBranchingDialogue(textConstant string) *BranchingDialogue {
	bd, err := GetBranchingDialogue(textConstant)
	if err != nil {
		return nil
	}
	return bd
}

// CheckForBranchingDialogueWithFlags checks if a text constant has YES/NO branching
// and verifies the player meets the requires_event_flag condition.
func CheckForBranchingDialogueWithFlags(textConstant string, charID int64, efm *EventFlagManager) *BranchingDialogue {
	bd, err := GetBranchingDialogue(textConstant)
	if err != nil {
		return nil
	}
	// If branching requires an event flag, check it
	if bd.RequiresEventFlag.Valid && bd.RequiresEventFlag.String != "" {
		if efm == nil || charID == 0 {
			return nil // Can't check — skip branching
		}
		if !efm.CheckFlag(charID, bd.RequiresEventFlag.String) {
			return nil // Player doesn't have the required flag
		}
	}
	return bd
}
