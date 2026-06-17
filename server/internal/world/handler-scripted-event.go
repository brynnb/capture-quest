package world

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/session"
)

// ScriptedEventInteractRequest is sent when the client clicks an actor/object
// that may start a data-driven scripted event.
type ScriptedEventInteractRequest struct {
	ActorID int `json:"actorId"`
}

// ScriptedEventInteractResponse reports whether the clicked actor/object
// started a scripted event.
type ScriptedEventInteractResponse struct {
	Success     bool   `json:"success"`
	Started     bool   `json:"started"`
	ScriptLabel string `json:"scriptLabel,omitempty"`
	Error       string `json:"error,omitempty"`
}

// HandleScriptedEventInteract checks whether a clicked phaser object maps to
// an eligible npc_click cutscene and starts it.
func HandleScriptedEventInteract(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req ScriptedEventInteractRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[ScriptedEvent] Invalid interact request: %v", err)
		ses.SendStreamJSON(ScriptedEventInteractResponse{
			Success: false,
			Error:   "invalid request",
		}, opcodes.ScriptedEventInteractResponse)
		return false
	}

	if wh.Cutscenes == nil || wh.EventFlags == nil || wh.ActorRegistry == nil {
		ses.SendStreamJSON(ScriptedEventInteractResponse{
			Success: false,
			Error:   "scripted events unavailable",
		}, opcodes.ScriptedEventInteractResponse)
		return false
	}

	objectID := wh.ActorRegistry.GetOriginalID(ActorTypeNPC, req.ActorID)
	if objectID == 0 {
		ses.SendStreamJSON(ScriptedEventInteractResponse{
			Success: false,
			Error:   "unknown actor",
		}, opcodes.ScriptedEventInteractResponse)
		return false
	}

	mapName, triggerKeys, err := scriptedEventTriggerKeys(objectID)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[ScriptedEvent] Failed to load object %d: %v", objectID, err)
		}
		ses.SendStreamJSON(ScriptedEventInteractResponse{
			Success: true,
			Started: false,
		}, opcodes.ScriptedEventInteractResponse)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	if handled, err := tryHandleVermilionGymTrashClick(ses, wh, charID, mapName, triggerKeys); handled || err != nil {
		if err != nil {
			log.Printf("[ScriptedEvent] Vermilion Gym trash click failed: %v", err)
			ses.SendStreamJSON(ScriptedEventInteractResponse{
				Success: false,
				Error:   "trash puzzle unavailable",
			}, opcodes.ScriptedEventInteractResponse)
		}
		return false
	}
	if handled, err := tryHandleSilphCardKeyClick(ses, wh, charID, mapName, triggerKeys); handled || err != nil {
		if err != nil {
			log.Printf("[ScriptedEvent] Silph Card Key click failed: %v", err)
			ses.SendStreamJSON(ScriptedEventInteractResponse{
				Success: false,
				Error:   "card key door unavailable",
			}, opcodes.ScriptedEventInteractResponse)
		}
		return false
	}

	playerFacing := ""
	if wh.PlayerMovement != nil {
		playerFacing, _ = wh.PlayerMovement.GetDirection(int(charID))
	}
	cs := wh.Cutscenes.FindEligibleClickCutscene(mapName, triggerKeys, charID, wh.EventFlags, playerFacing)
	if cs == nil {
		ses.SendStreamJSON(ScriptedEventInteractResponse{
			Success: true,
			Started: false,
		}, opcodes.ScriptedEventInteractResponse)
		return false
	}

	log.Printf("[ScriptedEvent] Starting %s for char %d from actor %d/object %d",
		cs.ScriptLabel, charID, req.ActorID, objectID)
	ses.SendStreamJSON(ScriptedEventInteractResponse{
		Success:     true,
		Started:     true,
		ScriptLabel: cs.ScriptLabel,
	}, opcodes.ScriptedEventInteractResponse)
	SendCutsceneToPlayer(ses, cs, wh)
	return false
}

func tryHandleVermilionGymTrashClick(ses *session.Session, wh *WorldHandler, charID int64, mapName string, triggerKeys []string) (bool, error) {
	if mapName != "VERMILION_GYM" {
		return false, nil
	}
	for _, key := range triggerKeys {
		canIndex, ok := VermilionGymTrashCanIndexForTextConstant(key)
		if !ok {
			continue
		}
		outcome, err := HandleVermilionGymTrashCan(charID, canIndex, wh.EventFlags)
		if err != nil {
			return true, err
		}
		actions, err := dynamicDialogueActions(outcome.Dialogue)
		if err != nil {
			return true, err
		}

		ses.SendStreamJSON(ScriptedEventInteractResponse{
			Success:     true,
			Started:     true,
			ScriptLabel: "VermilionGymTrash",
		}, opcodes.ScriptedEventInteractResponse)
		SendCutsceneToPlayer(ses, &CutsceneScript{
			ScriptLabel: "VermilionGymTrash",
			MapName:     mapName,
			TriggerType: "npc_click",
			Actions:     actions,
		}, wh)
		if outcome.OpenedSecond {
			sendEventTileStatesForSession(ses, charID, mapName, wh)
		}
		return true, nil
	}
	return false, nil
}

func tryHandleSilphCardKeyClick(ses *session.Session, wh *WorldHandler, charID int64, mapName string, triggerKeys []string) (bool, error) {
	for _, key := range triggerKeys {
		door, ok := SilphCardKeyDoorForTextConstant(key)
		if !ok {
			continue
		}
		if door.MapName != mapName {
			return true, fmt.Errorf("card key door %s belongs to %s, got %s", key, door.MapName, mapName)
		}
		outcome, err := HandleSilphCardKeyDoor(charID, key, wh.EventFlags)
		if err != nil {
			return true, err
		}
		actions, err := dynamicDialogueActions(outcome.Dialogue)
		if err != nil {
			return true, err
		}

		ses.SendStreamJSON(ScriptedEventInteractResponse{
			Success:     true,
			Started:     true,
			ScriptLabel: SilphCardKeyScriptLabel(),
		}, opcodes.ScriptedEventInteractResponse)
		SendCutsceneToPlayer(ses, &CutsceneScript{
			ScriptLabel: SilphCardKeyScriptLabel(),
			MapName:     mapName,
			TriggerType: "npc_click",
			Actions:     actions,
		}, wh)
		if outcome.Opened {
			sendEventTileStatesForSession(ses, charID, mapName, wh)
		}
		return true, nil
	}
	return false, nil
}

func dynamicDialogueActions(lines []string) (json.RawMessage, error) {
	return json.Marshal([]map[string]interface{}{
		{"type": "lockInput"},
		{"type": "dialogue", "lines": lines},
		{"type": "unlockInput"},
	})
}

func scriptedEventTriggerKeys(objectID int) (string, []string, error) {
	var (
		objectType sql.NullString
		name       sql.NullString
		text       sql.NullString
		mapName    string
	)
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT pm.name, po.object_type, po.name, po.text
		FROM phaser_objects po
		JOIN phaser_maps pm ON pm.id = po.map_id
		WHERE po.id = $1`, objectID).Scan(&mapName, &objectType, &name, &text)
	if err != nil {
		return "", nil, err
	}

	keys := []string{
		fmt.Sprintf("object:%d", objectID),
		fmt.Sprintf("phaser_object:%d", objectID),
	}
	if text.Valid && text.String != "" {
		keys = append(keys, text.String)
	}
	if name.Valid && name.String != "" {
		keys = append(keys, name.String)
	}
	if objectType.Valid && objectType.String != "" {
		keys = append(keys, fmt.Sprintf("%s:%d", objectType.String, objectID))
	}
	return mapName, keys, nil
}
