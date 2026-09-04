package scriptcandidateimport

import (
	"bytes"
	"encoding/json"
	"fmt"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

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

func mergeExtractorAudioIntoExisting(plan *outputPlan, current existingScript, generated scriptedevents.EventFile) (bool, bool, error) {
	if current.Source == capturequestSource || current.Path == "" {
		return false, false, nil
	}
	sfxActions := playSFXActionsForEvent(generated)
	if len(sfxActions) == 0 || !eventHasActionType(generated, "giveItem") {
		return false, false, nil
	}

	raw, err := plan.ReadFile(current.Path)
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

	changed, err := writeEventFile(plan, current.Path, existingEvent)
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
