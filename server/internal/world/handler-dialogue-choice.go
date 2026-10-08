package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"

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

func getBranchingDialogueContext(ctx context.Context, database db.ContextDBTX, textConstant string) (*BranchingDialogue, error) {
	var bd BranchingDialogue
	var yesActions, noActions sql.NullString
	err := database.QueryRowContext(ctx, `
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	bd, err := getBranchingDialogueContext(ctx, db.GlobalWorldDB.DB, textConstant)
	if err != nil {
		return nil, err
	}

	return resolveDialogueChoiceContext(ctx, db.GlobalWorldDB.DB, bd, choice)
}

func resolveDialogueChoiceContext(ctx context.Context, database db.ContextDBTX, bd *BranchingDialogue, choice bool) (*DialogueChoiceResult, error) {
	var err error
	result := &DialogueChoiceResult{Choice: choice}
	if bd.MapName.Valid {
		result.MapName = bd.MapName.String
	}

	if choice {
		if bd.YesDialogue.Valid && bd.YesDialogue.String != "" {
			result.FollowUpDialogue = bd.YesDialogue.String
		} else if bd.YesTextConstant.Valid && bd.YesTextConstant.String != "" {
			result.FollowUpTextConstant = bd.YesTextConstant.String
			result.FollowUpDialogue, err = fetchDialogueTextContext(ctx, database, result.FollowUpTextConstant)
			if err != nil {
				return nil, err
			}
		}
		result.Actions, err = dialogueChoiceActions(bd.YesActions, bd.SetsEventFlag)
	} else {
		if bd.NoDialogue.Valid && bd.NoDialogue.String != "" {
			result.FollowUpDialogue = bd.NoDialogue.String
		} else if bd.NoTextConstant.Valid && bd.NoTextConstant.String != "" {
			result.FollowUpTextConstant = bd.NoTextConstant.String
			result.FollowUpDialogue, err = fetchDialogueTextContext(ctx, database, result.FollowUpTextConstant)
			if err != nil {
				return nil, err
			}
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

	if !ses.HasValidClient() || wh == nil || wh.ActorRegistry == nil {
		return false
	}
	objectID := wh.ActorRegistry.GetOriginalID(ActorTypeNPC, req.ActorID)
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	actor, mapName, err := wh.scriptInteractionTargetContext(ctx, ses, objectID)
	if err != nil || actor.Text == nil || *actor.Text != req.TextConstant {
		if err != nil && !errors.Is(err, errScriptInteractionDenied) {
			log.Printf("[DialogueChoice] Authorize actor %d: %v", req.ActorID, err)
		}
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "dialogue unavailable or out of reach"}, opcodes.DialogueChoiceResponse)
		return false
	}

	if handleInGameTradeDialogueChoice(ctx, ses, req, wh, mapName) {
		return false
	}

	// Catalog choices belonging to issued scripts must use their completion token.
	if wh.Cutscenes != nil && wh.Cutscenes.HasClickCutsceneForTriggerLabel(req.TextConstant) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "dialogue requires its scripted event"}, opcodes.DialogueChoiceResponse)
		return false
	}
	bd, err := getBranchingDialogueContext(ctx, wh.database, req.TextConstant)
	if err == nil && bd.MapName.Valid && bd.MapName.String != "" && !sameMapName(bd.MapName.String, mapName) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "dialogue belongs to another map"}, opcodes.DialogueChoiceResponse)
		return false
	}
	var result *DialogueChoiceResult
	if err == nil {
		result, err = resolveDialogueChoiceContext(ctx, wh.database, bd, req.Choice)
	}
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

	if len(result.Actions) > 0 || bd.RequiresEventFlag.Valid {
		charID := int64(ses.Client.CharData().ID)
		script := &CutsceneScript{MapName: mapName, Actions: result.Actions}
		if bd.RequiresEventFlag.Valid {
			script.RequiresFlag = &bd.RequiresEventFlag.String
		}
		_, completed, err := ApplyCutsceneScript(ses.CommandContext(), CutsceneActionContext{Session: ses, WorldHandler: wh, EventFlags: wh.EventFlags}, script, charID)
		if err != nil || !completed {
			if err != nil {
				log.Printf("[DialogueChoice] Failed to apply choice actions for %s: %v", req.TextConstant, err)
			}
			ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "That choice could not be completed. Please try again."}, opcodes.DialogueChoiceResponse)
			return false
		}
	}

	ses.SendStreamJSON(res, opcodes.DialogueChoiceResponse)
	log.Printf("[DialogueChoice] Sent follow-up dialogue for choice=%v", req.Choice)
	return false
}

// fetchDialogueText resolves a text constant to dialogue text
func fetchDialogueTextContext(ctx context.Context, database db.ContextDBTX, textConstant string) (string, error) {
	var dialogue string
	err := database.QueryRowContext(ctx, `
		SELECT dt.dialogue
		FROM phaser_text_pointers tp
		LEFT JOIN phaser_dialogue_text dt ON dt.label = tp.dialogue_label
		WHERE tp.text_constant = $1
		LIMIT 1`, textConstant).Scan(&dialogue)
	return dialogue, err
}

// Branch prerequisites are evaluated against the caller's flag snapshot.
func checkBranchingDialogueWithFlags(ctx context.Context, database db.ContextDBTX, textConstant string, charID int64, efm *EventFlagManager) (*BranchingDialogue, error) {
	bd, err := getBranchingDialogueContext(ctx, database, textConstant)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if bd.RequiresEventFlag.Valid && bd.RequiresEventFlag.String != "" {
		if efm == nil || charID == 0 || !efm.CheckFlag(charID, bd.RequiresEventFlag.String) {
			return nil, nil
		}
	}
	return bd, nil
}
