package scriptcandidateimport

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"capturequest/internal/scriptedevents"

	_ "modernc.org/sqlite"
)

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
		"winFlag":          mapEventName(action.WinFlag),
		"loseFlag":         mapEventName(action.LoseFlag),
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
		"winFlag":        mapEventName(action.WinFlag),
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
