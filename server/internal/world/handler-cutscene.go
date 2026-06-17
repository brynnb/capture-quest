package world

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

var pokemonLookupUnderscorePattern = regexp.MustCompile(`_+`)

// CutsceneEndRequest is sent when the client finishes playing a cutscene.
type CutsceneEndRequest struct {
	ScriptLabel string `json:"scriptLabel"` // The cutscene that was completed
}

type CutsceneAction struct {
	Type              string           `json:"type"`
	Speaker           string           `json:"speaker,omitempty"`
	Lines             []string         `json:"lines,omitempty"`
	Actor             string           `json:"actor,omitempty"`
	Sprite            string           `json:"sprite,omitempty"`
	Movements         []string         `json:"movements,omitempty"`
	PokemonID         int              `json:"pokemonId,omitempty"`
	SpeciesID         int              `json:"speciesId,omitempty"`
	PokemonName       string           `json:"pokemonName,omitempty"`
	PokemonConstant   string           `json:"pokemonConstant,omitempty"`
	SFXConstant       string           `json:"sfxConstant,omitempty"`
	MusicConstant     string           `json:"musicConstant,omitempty"`
	MusicPath         string           `json:"musicPath,omitempty"`
	Loop              *bool            `json:"loop,omitempty"`
	Volume            float64          `json:"volume,omitempty"`
	Level             int              `json:"level,omitempty"`
	Flag              string           `json:"flag,omitempty"`
	Message           string           `json:"message,omitempty"`
	Money             int              `json:"money,omitempty"`
	Coins             int              `json:"coins,omitempty"`
	ItemID            int              `json:"itemId,omitempty"`
	ItemName          string           `json:"itemName,omitempty"`
	Quantity          int              `json:"quantity,omitempty"`
	MapID             int              `json:"mapId,omitempty"`
	X                 int              `json:"x,omitempty"`
	Y                 int              `json:"y,omitempty"`
	Direction         string           `json:"direction,omitempty"`
	ObjectID          int              `json:"objectId,omitempty"`
	ObjectKey         string           `json:"objectKey,omitempty"`
	ObjectMapName     string           `json:"objectMapName,omitempty"`
	TriggerLabel      string           `json:"triggerLabel,omitempty"`
	TextConstant      string           `json:"textConstant,omitempty"`
	Prompt            string           `json:"prompt,omitempty"`
	YesLines          []string         `json:"yesLines,omitempty"`
	NoLines           []string         `json:"noLines,omitempty"`
	ContinueOnNo      bool             `json:"continueOnNo,omitempty"`
	StopOnYes         bool             `json:"stopOnYes,omitempty"`
	ActorID           int              `json:"actorId,omitempty"`
	TrainerClass      string           `json:"trainerClass,omitempty"`
	TrainerPartyIndex int              `json:"partyIndex,omitempty"`
	PartyByFlag       map[string]int   `json:"partyByFlag,omitempty"`
	TrainerName       string           `json:"trainerName,omitempty"`
	TrainerObjectID   int              `json:"trainerObjectId,omitempty"`
	WinFlag           string           `json:"winFlag,omitempty"`
	LoseFlag          string           `json:"loseFlag,omitempty"`
	LossMessage       string           `json:"lossMessage,omitempty"`
	NoBlackoutOnLoss  bool             `json:"noBlackoutOnLoss,omitempty"`
	PostWinActions    []CutsceneAction `json:"postWinActions,omitempty"`
	PostLoseActions   []CutsceneAction `json:"postLoseActions,omitempty"`
	AllowedActions    []string         `json:"allowedActions,omitempty"`
	GuaranteedCatch   bool             `json:"guaranteedCatch,omitempty"`
	PrizeWindow       int              `json:"prizeWindow,omitempty"`
	Actions           []CutsceneAction `json:"actions,omitempty"`
	MS                int              `json:"ms,omitempty"`
}

type CutsceneActionEffect struct {
	Type    string
	Detail  string
	Changed bool
}

type CutsceneActionContext struct {
	Session      *session.Session
	WorldHandler *WorldHandler
	EventFlags   *EventFlagManager
	Choice       *bool
	StopAtChoice bool
	state        *cutsceneActionState
}

type cutsceneActionState struct {
	LastGivenItemName string
}

// HandleCutsceneEndRequest processes the client's confirmation that a cutscene finished.
// Sets any event flags associated with the cutscene.
func HandleCutsceneEndRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req CutsceneEndRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Cutscene] Invalid CutsceneEndRequest: %v", err)
		return false
	}

	if !ses.HasValidClient() {
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	log.Printf("[Cutscene] Player %d completed cutscene %s", charID, req.ScriptLabel)

	if wh.Cutscenes == nil {
		return false
	}

	cs := wh.Cutscenes.GetByLabel(req.ScriptLabel)
	if cs == nil {
		log.Printf("[Cutscene] Unknown cutscene label: %s", req.ScriptLabel)
		return false
	}

	if cs.RequiresFlagAbst != nil && *cs.RequiresFlagAbst != "" && wh.EventFlags != nil && wh.EventFlags.CheckFlag(charID, *cs.RequiresFlagAbst) {
		log.Printf("[Cutscene] Ignoring duplicate completion for %s; absent flag %s is already set",
			req.ScriptLabel, *cs.RequiresFlagAbst)
		return false
	}

	if err := applyCutsceneServerActions(ses, cs, charID, wh); err != nil {
		log.Printf("[Cutscene] Failed to apply server actions for %s: %v", req.ScriptLabel, err)
		SendSystemMessage(ses, "That event could not be completed. Please try again.")
		return false
	}

	// Set completion flags
	if len(cs.SetsFlags) > 0 && wh.EventFlags != nil {
		if err := wh.EventFlags.SetFlagBatch(charID, cs.SetsFlags); err != nil {
			log.Printf("[Cutscene] Failed to set flags for %s: %v", req.ScriptLabel, err)
		} else {
			log.Printf("[Cutscene] Set %d flags for player %d: %v", len(cs.SetsFlags), charID, cs.SetsFlags)
			sendEventTileStatesForSession(ses, charID, cs.MapName, wh)
		}
	}

	if cutsceneAffectsTrainerCard(cs) {
		sendTrainerCardResponse(ses, wh)
	}

	// Post-cutscene warp: teleport the player to a different map
	if cs.WarpToMapID != nil && cs.WarpToX != nil && cs.WarpToY != nil {
		destMapID := *cs.WarpToMapID
		destX := *cs.WarpToX
		destY := *cs.WarpToY

		log.Printf("[Cutscene] Warping player %d to map %d (%d,%d) after %s",
			charID, destMapID, destX, destY, req.ScriptLabel)

		setCutscenePlayerPosition(ses, wh, charID, destMapID, destX, destY, "DOWN")

		// Send warp notification to client
		ses.SendStreamJSON(map[string]interface{}{
			"mapId": destMapID,
			"x":     destX,
			"y":     destY,
		}, opcodes.WarpTileTeleportNotify)
	}

	return false
}

func cutsceneAffectsTrainerCard(cs *CutsceneScript) bool {
	if cs == nil {
		return false
	}
	for _, flag := range cs.SetsFlags {
		if isBadgeFlag(flag) {
			return true
		}
	}
	actions, err := DecodeCutsceneActions(cs.Actions)
	if err != nil {
		return false
	}
	return cutsceneActionListAffectsTrainerCard(actions)
}

func cutsceneActionListAffectsTrainerCard(actions []CutsceneAction) bool {
	for _, action := range actions {
		switch action.Type {
		case "setFlag", "resetFlag", "toggleFlag":
			if isBadgeFlag(action.Flag) {
				return true
			}
		case "parallel":
			if cutsceneActionListAffectsTrainerCard(action.Actions) {
				return true
			}
		}
	}
	return false
}

func isBadgeFlag(flag string) bool {
	for _, badgeFlag := range badgeFlags {
		if flag == badgeFlag {
			return true
		}
	}
	return false
}

// SendCutsceneToPlayer sends a cutscene action sequence to a specific player.
func SendCutsceneToPlayer(ses *session.Session, cs *CutsceneScript, handlers ...*WorldHandler) {
	actions := cs.Actions
	if len(handlers) > 0 && handlers[0] != nil {
		if annotated, err := annotateCutsceneActionsForClient(cs, handlers[0]); err != nil {
			log.Printf("[Cutscene] Failed to annotate client actions for %s: %v", cs.ScriptLabel, err)
		} else if len(annotated) > 0 {
			actions = annotated
		}
	}
	payload := map[string]interface{}{
		"scriptLabel": cs.ScriptLabel,
		"mapName":     cs.MapName,
		"actions":     json.RawMessage(actions),
	}
	ses.SendStreamJSON(payload, opcodes.CutsceneStartNotify)
	log.Printf("[Cutscene] Sent cutscene %s to player", cs.ScriptLabel)
}

func annotateCutsceneActionsForClient(cs *CutsceneScript, wh *WorldHandler) (json.RawMessage, error) {
	if cs == nil || wh == nil || wh.ActorRegistry == nil {
		return nil, nil
	}
	actions, err := DecodeCutsceneActions(cs.Actions)
	if err != nil {
		return nil, err
	}
	changed := annotateCutsceneActionListForClient(actions, cs.MapName, wh)
	if !changed {
		return nil, nil
	}
	return json.Marshal(actions)
}

func annotateCutsceneActionListForClient(actions []CutsceneAction, mapName string, wh *WorldHandler) bool {
	changed := false
	for i := range actions {
		actionMapName := mapName
		if actions[i].ObjectMapName != "" {
			actionMapName = actions[i].ObjectMapName
		}
		switch actions[i].Type {
		case "hideObject", "showObject":
			if actions[i].ActorID != 0 {
				continue
			}
			objectIDs, err := resolveCutsceneObjectIDs(actionMapName, actions[i])
			if err != nil || len(objectIDs) == 0 {
				continue
			}
			actions[i].ActorID = wh.ActorRegistry.GetPhaserID(ActorTypeNPC, objectIDs[0])
			changed = true
		case "parallel":
			if annotateCutsceneActionListForClient(actions[i].Actions, mapName, wh) {
				changed = true
			}
		}
	}
	return changed
}

func applyCutsceneServerActions(ses *session.Session, cs *CutsceneScript, charID int64, wh *WorldHandler) error {
	return applyCutsceneServerActionList(ses, cs.MapName, cs.Actions, charID, wh)
}

func applyScriptedTrainerPostWinActions(ses *session.Session, trainer *pokebattle.TrainerMeta, charID int64, wh *WorldHandler) error {
	if trainer == nil || len(trainer.PostWinActions) == 0 || string(trainer.PostWinActions) == "null" {
		return nil
	}
	return applyCutsceneServerActionList(ses, trainer.PostWinMapName, trainer.PostWinActions, charID, wh)
}

func applyScriptedTrainerPostLoseActions(ses *session.Session, trainer *pokebattle.TrainerMeta, charID int64, wh *WorldHandler) error {
	if trainer == nil || len(trainer.PostLoseActions) == 0 || string(trainer.PostLoseActions) == "null" {
		return nil
	}
	return applyCutsceneServerActionList(ses, trainer.PostLoseMapName, trainer.PostLoseActions, charID, wh)
}

func applyScriptedWildPostWinActions(ses *session.Session, battle *pokebattle.BattleState, charID int64, wh *WorldHandler) error {
	if battle == nil || len(battle.WildPostWinActions) == 0 || string(battle.WildPostWinActions) == "null" {
		return nil
	}
	return applyCutsceneServerActionList(ses, battle.WildPostWinMapName, battle.WildPostWinActions, charID, wh)
}

func applyCutsceneServerActionList(ses *session.Session, mapName string, rawActions json.RawMessage, charID int64, wh *WorldHandler) error {
	var efm *EventFlagManager
	if wh != nil {
		efm = wh.EventFlags
	}
	_, _, err := ApplyCutsceneActionList(CutsceneActionContext{
		Session:      ses,
		WorldHandler: wh,
		EventFlags:   efm,
	}, mapName, rawActions, charID)
	return err
}

func ApplyCutsceneActionList(ctx CutsceneActionContext, mapName string, rawActions json.RawMessage, charID int64) ([]CutsceneActionEffect, bool, error) {
	actions, err := DecodeCutsceneActions(rawActions)
	if err != nil {
		return nil, false, err
	}
	if ctx.state == nil {
		ctx.state = &cutsceneActionState{}
	}

	effects := make([]CutsceneActionEffect, 0, len(actions))
	for _, action := range actions {
		effect := CutsceneActionEffect{Type: action.Type, Detail: CutsceneActionSummary(action)}
		switch action.Type {
		case "parallel":
			effects = append(effects, effect)
			nestedRaw, err := json.Marshal(action.Actions)
			if err != nil {
				return effects, false, fmt.Errorf("marshal parallel actions: %w", err)
			}
			nestedEffects, completed, err := ApplyCutsceneActionList(ctx, mapName, nestedRaw, charID)
			effects = append(effects, nestedEffects...)
			if err != nil || !completed {
				return effects, completed, err
			}
			continue
		case "choice":
			if ctx.StopAtChoice {
				if ctx.Choice == nil {
					effects = append(effects, effect)
					return effects, false, nil
				}
				effect.Detail = fmt.Sprintf("%s choice=%t", CutsceneActionSummary(action), *ctx.Choice)
				effects = append(effects, effect)
				if *ctx.Choice {
					if len(action.YesLines) > 0 {
						effects = append(effects, cutsceneChoiceDialogueEffect(action, action.YesLines, ctx.state.LastGivenItemName))
					}
					if action.StopOnYes {
						return effects, false, nil
					}
				} else {
					if len(action.NoLines) > 0 {
						effects = append(effects, cutsceneChoiceDialogueEffect(action, action.NoLines, ctx.state.LastGivenItemName))
					}
					if !action.ContinueOnNo {
						return effects, false, nil
					}
				}
				continue
			}
		case "movePlayer":
			detail, err := applyMovePlayerAction(ctx.Session, action, charID, ctx.WorldHandler)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "givePokemon":
			detail, err := applyGivePokemonAction(ctx.Session, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "setFlag":
			if action.Flag != "" && ctx.EventFlags != nil {
				if err := ctx.EventFlags.SetFlag(charID, action.Flag); err != nil {
					return effects, false, err
				}
				sendCutsceneEventTileStates(ctx, charID, mapName)
				effect.Changed = true
			}
		case "resetFlag":
			if action.Flag != "" && ctx.EventFlags != nil {
				if err := ctx.EventFlags.ResetFlag(charID, action.Flag); err != nil {
					return effects, false, err
				}
				sendCutsceneEventTileStates(ctx, charID, mapName)
				effect.Changed = true
			}
		case "toggleFlag":
			if action.Flag != "" && ctx.EventFlags != nil {
				on, err := ctx.EventFlags.ToggleFlag(charID, action.Flag)
				if err != nil {
					return effects, false, err
				}
				sendCutsceneEventTileStates(ctx, charID, mapName)
				effect.Detail = fmt.Sprintf("%s=%t", action.Flag, on)
				effect.Changed = true
			}
		case "hideObject":
			detail, err := applyObjectVisibilityAction(ctx.Session, mapName, action, charID, ctx.WorldHandler, true)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "showObject":
			detail, err := applyObjectVisibilityAction(ctx.Session, mapName, action, charID, ctx.WorldHandler, false)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "giveItem":
			detail, itemName, err := applyGiveItemAction(ctx.Session, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
			ctx.state.LastGivenItemName = itemName
		case "takeItem":
			detail, err := applyTakeItemAction(ctx.Session, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "dialogue":
			action.Lines = fillBufferedItemDialogueLines(action.Lines, ctx.state.LastGivenItemName)
			effect.Detail = CutsceneActionSummary(action)
		case "takeMoney":
			detail, err := applyTakeMoneyAction(ctx.Session, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "giveCoins":
			detail, err := applyGiveCoinsAction(ctx.Session, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "gameCornerPrizeVendor":
			detail, err := sendGameCornerPrizeList(ctx.Session, charID, action.PrizeWindow)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
		case "healParty":
			detail, err := applyHealPartyAction(ctx.Session, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "startSafariSession":
			detail, changed, err := applyStartSafariSessionAction(ctx, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = changed
		case "endSafariSession":
			detail, changed, err := applyEndSafariSessionAction(ctx, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = changed
		case "startTrainerBattle":
			detail, err := applyStartTrainerBattleAction(ctx.Session, mapName, action, charID, ctx.EventFlags)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "startWildBattle":
			detail, err := applyStartWildBattleAction(ctx.Session, mapName, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		}
		effects = append(effects, effect)
	}

	return effects, true, nil
}

func DecodeCutsceneActions(rawActions json.RawMessage) ([]CutsceneAction, error) {
	if len(rawActions) == 0 || string(rawActions) == "null" {
		return nil, nil
	}
	var actions []CutsceneAction
	if err := json.Unmarshal(rawActions, &actions); err != nil {
		return nil, fmt.Errorf("parse actions: %w", err)
	}
	return actions, nil
}

func sendCutsceneEventTileStates(ctx CutsceneActionContext, charID int64, mapName string) {
	if ctx.Session != nil && ctx.WorldHandler != nil {
		sendEventTileStatesForSession(ctx.Session, charID, mapName, ctx.WorldHandler)
	}
}

func applyMovePlayerAction(ses *session.Session, action CutsceneAction, charID int64, wh *WorldHandler) (string, error) {
	if len(action.Movements) == 0 {
		return "", fmt.Errorf("movePlayer missing movements")
	}

	x, y, mapID, err := currentCutscenePlayerPosition(ses, charID)
	if err != nil {
		return "", err
	}
	startX, startY := x, y
	direction := ""
	for _, movement := range action.Movements {
		step := normalizeCutsceneMovement(movement)
		switch step {
		case "UP":
			y--
		case "DOWN":
			y++
		case "LEFT":
			x--
		case "RIGHT":
			x++
		default:
			return "", fmt.Errorf("unsupported movePlayer movement %q", movement)
		}
		direction = step
	}

	setCutscenePlayerPosition(ses, wh, charID, mapID, x, y, direction)
	return fmt.Sprintf("from=(%d,%d) to=(%d,%d) steps=%v", startX, startY, x, y, action.Movements), nil
}

func normalizeCutsceneMovement(movement string) string {
	step := strings.ToUpper(strings.TrimSpace(movement))
	step = strings.TrimPrefix(step, "NPC_MOVEMENT_")
	return step
}

func currentCutscenePlayerPosition(ses *session.Session, charID int64) (int, int, int, error) {
	if ses != nil {
		x := int(ses.X)
		y := int(ses.Y)
		mapID := ses.MapID
		if ses.Client != nil {
			if char := ses.Client.CharData(); char != nil {
				x = int(char.X)
				y = int(char.Y)
				mapID = int(char.MapID)
			}
		}
		return x, y, mapID, nil
	}

	var x, y, mapID int
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT CAST(x AS INTEGER), CAST(y AS INTEGER), map_id FROM character_data WHERE id = $1`,
		charID).Scan(&x, &y, &mapID); err != nil {
		return 0, 0, 0, fmt.Errorf("load player position: %w", err)
	}
	return x, y, mapID, nil
}

func setCutscenePlayerPosition(ses *session.Session, wh *WorldHandler, charID int64, mapID, x, y int, direction string) {
	if direction == "" {
		direction = "DOWN"
	}

	sessionMapID := mapID
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		sessionMapID = UnifiedOverworldMapID
	}
	if ses == nil || !ses.HasValidClient() {
		if wh != nil && wh.PlayerMovement != nil {
			wh.PlayerMovement.UpdatePosition(int(charID), x, y, sessionMapID, direction)
			wh.PlayerMovement.FlushPlayerPosition(int(charID))
		}
		return
	}

	setServerTeleportedPlayerPosition(ses, wh, mapID, x, y, direction)
}

func sendCutsceneSystemMessage(ses *session.Session, message string) {
	if ses != nil {
		SendSystemMessage(ses, message)
	}
}

func sendCutsceneInventorySnapshot(ses *session.Session, charID int32) {
	if ses != nil {
		sendCQInventorySnapshot(ses, charID)
	}
}

func sendCutscenePartyUpdate(ses *session.Session) {
	if ses != nil {
		sendPartyUpdate(ses)
	}
}

func applyGiveItemAction(ses *session.Session, action CutsceneAction, charID int64) (string, string, error) {
	itemID, name, err := resolveCutsceneItem(action)
	if err != nil {
		return "", "", err
	}
	quantity := action.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	_, err = cqitems.AddItemToInventory(int32(charID), int32(itemID), uint16(quantity))
	if err != nil {
		return "", "", fmt.Errorf("give item %s x%d: %w", name, quantity, err)
	}
	sendCutsceneSystemMessage(ses, fmt.Sprintf("Received %s!", name))
	sendCutsceneInventorySnapshot(ses, int32(charID))
	return fmt.Sprintf("%s x%d added", name, quantity), name, nil
}

func fillBufferedItemDialogueLines(lines []string, itemName string) []string {
	if itemName == "" || len(lines) == 0 {
		return lines
	}
	filled := make([]string, len(lines))
	copy(filled, lines)
	for i, line := range filled {
		filled[i] = fillBufferedItemDialogueLine(line, itemName)
	}
	return filled
}

func fillBufferedItemDialogueLine(line, itemName string) string {
	replacer := strings.NewReplacer(
		"received\n!", "received\n"+itemName+"!",
		"received\na \n!", "received\na "+itemName+"!",
		"received\nan \n!", "received\nan "+itemName+"!",
		"received\n \n!", "received\n"+itemName+"!",
	)
	line = replacer.Replace(line)
	if strings.HasPrefix(line, "contains\n") && itemName != "" {
		return itemName + " " + line
	}
	return line
}

func cutsceneChoiceDialogueEffect(action CutsceneAction, lines []string, lastItemName string) CutsceneActionEffect {
	dialogue := CutsceneAction{
		Type:    "dialogue",
		Speaker: action.Speaker,
		Lines:   fillBufferedItemDialogueLines(lines, lastItemName),
	}
	return CutsceneActionEffect{Type: "dialogue", Detail: CutsceneActionSummary(dialogue)}
}

func applyTakeItemAction(ses *session.Session, action CutsceneAction, charID int64) (string, error) {
	itemID, name, err := resolveCutsceneItem(action)
	if err != nil {
		return "", err
	}
	quantity := action.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	for i := 0; i < quantity; i++ {
		found, err := cqitems.FindInventoryItemByItemID(int32(charID), int32(itemID))
		if err != nil {
			return "", fmt.Errorf("take item %s: %w", name, err)
		}
		if _, err := cqitems.DecrementItemQuantity(int32(charID), found.Instance.ID); err != nil {
			return "", err
		}
	}
	sendCutsceneInventorySnapshot(ses, int32(charID))
	return fmt.Sprintf("%s x%d removed", name, quantity), nil
}

func applyTakeMoneyAction(ses *session.Session, action CutsceneAction, charID int64) (string, error) {
	if action.Money <= 0 {
		return "", fmt.Errorf("takeMoney missing money")
	}
	var remaining int
	err := db.GlobalWorldDB.DB.QueryRow(`
		UPDATE character_wallet
		SET pokedollars = pokedollars - $1
		WHERE character_id = $2 AND pokedollars >= $1
		RETURNING pokedollars`, action.Money, charID).Scan(&remaining)
	if err != nil {
		return "", fmt.Errorf("take money %d: %w", action.Money, err)
	}
	sendCutsceneSystemMessage(ses, fmt.Sprintf("Spent %d Pokedollars.", action.Money))
	return fmt.Sprintf("spent=%d money=%d", action.Money, remaining), nil
}

func applyGiveCoinsAction(ses *session.Session, action CutsceneAction, charID int64) (string, error) {
	if action.Coins <= 0 {
		return "", fmt.Errorf("giveCoins missing coins")
	}
	total, err := addCoins(charID, action.Coins)
	if err != nil {
		return "", fmt.Errorf("give coins %d: %w", action.Coins, err)
	}
	sendCutsceneSystemMessage(ses, fmt.Sprintf("Received %d coins!", action.Coins))
	return fmt.Sprintf("coins=%d total=%d", action.Coins, total), nil
}

func applyHealPartyAction(ses *session.Session, charID int64) (string, error) {
	party, err := HealCharacterParty(charID)
	if err != nil {
		return "", fmt.Errorf("heal party: %w", err)
	}
	sendCutscenePartyUpdate(ses)
	return fmt.Sprintf("healed %d pokemon", len(party)), nil
}

func applyStartSafariSessionAction(ctx CutsceneActionContext, action CutsceneAction, charID int64) (string, bool, error) {
	safari := safariManagerForCutscene(ctx.WorldHandler)
	result := TryStartSafariZoneVisit(charID, safari)
	detail := fmt.Sprintf("success=%t money=%d", result.Success, result.Money)
	if result.Message != "" {
		detail = fmt.Sprintf("%s message=%q", detail, result.Message)
	}
	if !result.Success {
		sendSafariEntryFailure(ctx.Session, result)
		return detail, false, nil
	}

	if ctx.EventFlags != nil {
		if err := ctx.EventFlags.SetFlag(charID, EventInSafariZone); err != nil {
			return detail, false, err
		}
		if err := ctx.EventFlags.ResetFlag(charID, EventSafariGameOver); err != nil {
			return detail, false, err
		}
	}

	detail = fmt.Sprintf("%s balls=%d steps=%d", detail, result.BallsLeft, result.StepsLeft)
	sendSafariEntrySuccess(ctx.Session, result)

	destMapID, destX, destY := safariEntryDestination(action)
	direction := normalizeWarpDirection(action.Direction)
	if direction == "" {
		direction = "UP"
	}
	setCutscenePlayerPosition(ctx.Session, ctx.WorldHandler, charID, destMapID, destX, destY, direction)
	sendCutsceneWarp(ctx.Session, destMapID, destX, destY, direction)
	detail = fmt.Sprintf("%s warp=%d(%d,%d)", detail, destMapID, destX, destY)
	return detail, true, nil
}

func applyEndSafariSessionAction(ctx CutsceneActionContext, charID int64) (string, bool, error) {
	detail := "active=false"
	changed := false
	if ctx.WorldHandler != nil && ctx.WorldHandler.Safari != nil {
		if session := ctx.WorldHandler.Safari.GetSession(charID); session != nil {
			detail = fmt.Sprintf("active=%t balls=%d steps=%d", session.Active, session.BallsLeft, session.StepsLeft)
			changed = true
		}
		ctx.WorldHandler.Safari.EndSession(charID)
	}

	if ctx.EventFlags != nil {
		if err := ctx.EventFlags.ResetFlag(charID, EventInSafariZone); err != nil {
			return detail, changed, err
		}
		if err := ctx.EventFlags.ResetFlag(charID, EventSafariGameOver); err != nil {
			return detail, changed, err
		}
		changed = true
	}

	sendSafariManualExit(ctx.Session)
	return fmt.Sprintf("%s ended=true", detail), changed, nil
}

func safariManagerForCutscene(wh *WorldHandler) *SafariZoneManager {
	if wh != nil {
		if wh.Safari == nil {
			wh.Safari = NewSafariZoneManager()
		}
		return wh.Safari
	}
	return NewSafariZoneManager()
}

func safariEntryDestination(action CutsceneAction) (int, int, int) {
	mapID := action.MapID
	if mapID <= 0 {
		mapID = SafariZoneCenterMapID
	}
	x, y := action.X, action.Y
	if x == 0 && y == 0 {
		x, y = SafariZoneDefaultEntryX, SafariZoneDefaultEntryY
	}
	return mapID, x, y
}

func sendSafariEntryFailure(ses *session.Session, result SafariEntryResult) {
	if ses == nil {
		return
	}
	message := result.Message
	if message == "not enough money" {
		message = "Oops! Not enough money!"
	}
	ses.SendStreamJSON(map[string]interface{}{
		"success": false,
		"message": message,
		"money":   result.Money,
	}, opcodes.SafariZoneEnterResponse)
	SendSystemMessage(ses, message)
}

func sendSafariEntrySuccess(ses *session.Session, result SafariEntryResult) {
	if ses == nil {
		return
	}
	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"ballsLeft": result.BallsLeft,
		"stepsLeft": result.StepsLeft,
		"money":     result.Money,
	}, opcodes.SafariZoneEnterResponse)
	ses.SendStreamJSON(map[string]interface{}{
		"stepsLeft": result.StepsLeft,
		"ballsLeft": result.BallsLeft,
	}, opcodes.SafariZoneStepUpdate)
	if result.AlreadyActive {
		SendSystemMessage(ses, "Safari Zone visit already active.")
	} else {
		SendSystemMessage(ses, "Received 30 SAFARI BALLs!")
	}
}

func sendSafariManualExit(ses *session.Session) {
	if ses == nil {
		return
	}
	ses.SendStreamJSON(map[string]interface{}{
		"success": false,
		"message": "Safari Zone visit ended.",
	}, opcodes.SafariZoneEnterResponse)
}

func sendCutsceneWarp(ses *session.Session, mapID, x, y int, direction string) {
	if ses == nil {
		return
	}
	ses.SendStreamJSON(map[string]interface{}{
		"mapId":     mapID,
		"x":         x,
		"y":         y,
		"direction": direction,
	}, opcodes.WarpTileTeleportNotify)
}

func resolveCutsceneItem(action CutsceneAction) (int, string, error) {
	if action.ItemID > 0 {
		name := cutsceneItemName(action.ItemID)
		return action.ItemID, name, nil
	}
	if action.ItemName == "" {
		return 0, "", fmt.Errorf("%s action missing itemId/itemName", action.Type)
	}
	var id int
	var name string
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT id, name FROM cq_items WHERE name = $1 OR short_name = $2 LIMIT 1`,
		action.ItemName, action.ItemName).Scan(&id, &name); err != nil {
		return 0, "", fmt.Errorf("lookup item %s: %w", action.ItemName, err)
	}
	return id, name, nil
}

func cutsceneItemName(itemID int) string {
	var name string
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT name FROM cq_items WHERE id = $1`, itemID).Scan(&name); err != nil || name == "" {
		return fmt.Sprintf("ITEM #%d", itemID)
	}
	return name
}

func applyStartTrainerBattleAction(ses *session.Session, mapName string, action CutsceneAction, charID int64, efm *EventFlagManager) (string, error) {
	partyIndex, err := resolveCutsceneTrainerPartyIndex(action, charID, efm)
	if err != nil {
		return "", err
	}
	postWinActions, err := json.Marshal(action.PostWinActions)
	if err != nil {
		return "", fmt.Errorf("marshal post-win actions: %w", err)
	}
	postLoseActions, err := json.Marshal(action.PostLoseActions)
	if err != nil {
		return "", fmt.Errorf("marshal post-lose actions: %w", err)
	}
	battle, events, err := StartScriptedTrainerBattle(charID, ScriptedTrainerBattleSpec{
		TrainerClass:     action.TrainerClass,
		PartyIndex:       partyIndex,
		TrainerName:      action.TrainerName,
		TrainerObjectID:  action.TrainerObjectID,
		WinFlag:          action.WinFlag,
		LoseFlag:         action.LoseFlag,
		LossMessage:      action.LossMessage,
		NoBlackoutOnLoss: action.NoBlackoutOnLoss,
		PostWinMapName:   mapName,
		PostWinActions:   postWinActions,
		PostLoseMapName:  mapName,
		PostLoseActions:  postLoseActions,
	})
	if err != nil {
		return "", err
	}

	if ses != nil {
		resp := buildBattleStateResponse(battle)
		resp["trainerClass"] = action.TrainerClass
		if battle.Trainer != nil {
			resp["trainerName"] = battle.Trainer.Name
		}
		resp["events"] = events
		ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
	}
	enemy := make([]string, 0, len(battle.EnemyParty))
	for _, pokemon := range battle.EnemyParty {
		if pokemon != nil {
			enemy = append(enemy, fmt.Sprintf("#%d %s L%d", pokemon.ID, pokemon.Name, pokemon.Level))
		}
	}
	return fmt.Sprintf("%s party=%d enemy=%v winFlag=%s", action.TrainerClass, partyIndex, enemy, action.WinFlag), nil
}

func applyStartWildBattleAction(ses *session.Session, mapName string, action CutsceneAction, charID int64) (string, error) {
	pokemonID, _, err := resolveCutscenePokemonSpecies(action)
	if err != nil {
		return "", fmt.Errorf("startWildBattle: %w", err)
	}
	postWinActions, err := json.Marshal(action.PostWinActions)
	if err != nil {
		return "", fmt.Errorf("marshal post-win actions: %w", err)
	}
	battle, events, err := StartScriptedWildBattle(charID, ScriptedWildBattleSpec{
		PokemonID:       pokemonID,
		Level:           action.Level,
		WinFlag:         action.WinFlag,
		PostWinMapName:  mapName,
		PostWinActions:  postWinActions,
		AllowedActions:  action.AllowedActions,
		GuaranteedCatch: action.GuaranteedCatch,
	})
	if err != nil {
		return "", err
	}

	if ses != nil {
		resp := buildBattleStateResponse(battle)
		resp["events"] = events
		ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
	}
	wild := battle.GetEnemyPokemon()
	detail := fmt.Sprintf("#%d %s L%d winFlag=%s", wild.ID, wild.Name, wild.Level, action.WinFlag)
	if len(action.AllowedActions) > 0 {
		detail += fmt.Sprintf(" allowedActions=%v", action.AllowedActions)
	}
	if action.GuaranteedCatch {
		detail += " guaranteedCatch=true"
	}
	return detail, nil
}

func resolveCutsceneTrainerPartyIndex(action CutsceneAction, charID int64, efm *EventFlagManager) (int, error) {
	if action.TrainerPartyIndex > 0 {
		return action.TrainerPartyIndex, nil
	}
	if efm != nil {
		for flag, partyIndex := range action.PartyByFlag {
			if efm.CheckFlag(charID, flag) {
				return partyIndex, nil
			}
		}
	}
	return 0, fmt.Errorf("startTrainerBattle action missing partyIndex or matching partyByFlag")
}

func applyObjectVisibilityAction(ses *session.Session, mapName string, action CutsceneAction, charID int64, wh *WorldHandler, hide bool) (string, error) {
	objectMapName := mapName
	if action.ObjectMapName != "" {
		objectMapName = action.ObjectMapName
	}
	objectIDs, err := resolveCutsceneObjectIDs(objectMapName, action)
	if err != nil {
		return "", err
	}
	if len(objectIDs) == 0 {
		return "", fmt.Errorf("%s action could not resolve an object on map %s", action.Type, objectMapName)
	}

	for _, objectID := range objectIDs {
		if hide {
			if _, err := db.GlobalWorldDB.DB.Exec(`
				INSERT INTO character_collected_items (character_id, object_id)
				VALUES ($1, $2)
				ON CONFLICT (character_id, object_id) DO NOTHING`, charID, objectID); err != nil {
				return "", fmt.Errorf("hide object %d: %w", objectID, err)
			}
			if err := SetCharacterObjectVisibilityOverride(charID, objectID, false, "CutsceneAction:hideObject"); err != nil {
				return "", fmt.Errorf("hide object visibility override %d: %w", objectID, err)
			}
			if ses != nil && wh != nil && wh.ActorRegistry != nil {
				phaserID := wh.ActorRegistry.GetPhaserID(ActorTypeNPC, objectID)
				ses.SendStreamJSON(map[string]interface{}{"id": phaserID}, opcodes.PhaserActorDespawn)
			}
			log.Printf("[Cutscene] Hid object %d for char %d", objectID, charID)
			continue
		}

		if _, err := db.GlobalWorldDB.DB.Exec(`
			DELETE FROM character_collected_items
			WHERE character_id = $1 AND object_id = $2`, charID, objectID); err != nil {
			return "", fmt.Errorf("show object %d: %w", objectID, err)
		}
		if err := SetCharacterObjectVisibilityOverride(charID, objectID, true, "CutsceneAction:showObject"); err != nil {
			return "", fmt.Errorf("show object visibility override %d: %w", objectID, err)
		}
		if ses != nil && wh != nil && wh.ActorManager != nil {
			if err := wh.ActorManager.SendObjectActorToSession(objectID, ses); err != nil {
				return "", fmt.Errorf("show object actor %d: %w", objectID, err)
			}
		}
		log.Printf("[Cutscene] Restored object %d for char %d", objectID, charID)
	}

	return describeCutsceneObjectIDs(objectIDs), nil
}

func describeCutsceneObjectIDs(ids []int) string {
	labels := make([]string, 0, len(ids))
	for _, id := range ids {
		labels = append(labels, cutsceneObjectDisplayLabel(id))
	}
	return fmt.Sprintf("objects=[%s]", strings.Join(labels, ", "))
}

func cutsceneObjectDisplayLabel(id int) string {
	var name, text string
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT COALESCE(name, ''), COALESCE(text, '')
		FROM phaser_objects
		WHERE id = $1`, id).Scan(&name, &text)
	if err != nil {
		return fmt.Sprintf("object:%d", id)
	}
	if text != "" {
		return text
	}
	if name != "" {
		return name
	}
	return fmt.Sprintf("object:%d", id)
}

func resolveCutsceneObjectIDs(mapName string, action CutsceneAction) ([]int, error) {
	if action.ObjectID > 0 {
		return []int{action.ObjectID}, nil
	}

	keys := []string{}
	for _, key := range []string{action.ObjectKey, action.TriggerLabel, action.TextConstant} {
		if key != "" {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%s action missing objectId/objectKey/triggerLabel/textConstant", action.Type)
	}

	ids := make([]int, 0, len(keys))
	seen := make(map[int]bool)
	for _, key := range keys {
		resolved, err := ResolveCutsceneObjectKey(mapName, key)
		if err != nil {
			return nil, err
		}
		for _, id := range resolved {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}

	return ids, nil
}

func ResolveCutsceneObjectKey(mapName string, key string) ([]int, error) {
	var explicitID int
	if _, err := fmt.Sscanf(key, "object:%d", &explicitID); err == nil && explicitID > 0 {
		return []int{explicitID}, nil
	}
	if _, err := fmt.Sscanf(key, "phaser_object:%d", &explicitID); err == nil && explicitID > 0 {
		return []int{explicitID}, nil
	}

	ids, err := queryCutsceneObjectIDsByMapName(mapName, key)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		return ids, nil
	}

	if strings.HasPrefix(key, "HS_") {
		ids, err = queryCutsceneObjectIDsByMissableConstant(key)
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}

	if !isCutsceneOverworldMapName(mapName) {
		return ids, nil
	}

	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id
		FROM phaser_objects po
		WHERE po.map_id = $1 AND (po.text = $2 OR po.name = $3)
		ORDER BY po.id`, UnifiedOverworldMapID, key, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids = []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func queryCutsceneObjectIDsByMapName(mapName string, key string) ([]int, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id
		FROM phaser_objects po
		JOIN phaser_maps pm ON pm.id = po.map_id
		WHERE pm.name = $1 AND (po.text = $2 OR po.name = $3)
		ORDER BY po.id`, mapName, key, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func queryCutsceneObjectIDsByMissableConstant(hsConstant string) ([]int, error) {
	rows, err := db.GlobalWorldDB.DB.Query(`
		SELECT po.id
		FROM phaser_missable_objects mo
		JOIN phaser_objects po
			ON po.map_id = mo.map_id
			AND (
				(mo.object_name IS NOT NULL AND mo.object_name <> '' AND po.name = mo.object_name)
				OR po.text = mo.object_constant
			)
		WHERE mo.hs_constant = $1
		ORDER BY po.id`, hsConstant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func isCutsceneOverworldMapName(mapName string) bool {
	var isOverworld bool
	if err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT is_overworld FROM phaser_maps WHERE name = $1`, mapName).Scan(&isOverworld); err != nil {
		return false
	}
	return isOverworld
}

func applyGivePokemonAction(ses *session.Session, action CutsceneAction, charID int64) (string, error) {
	speciesID, speciesName, err := resolveCutscenePokemonSpecies(action)
	if err != nil {
		return "", fmt.Errorf("givePokemon: %w", err)
	}
	level := action.Level
	if level <= 0 {
		level = 5
	}

	myDB := db.GlobalWorldDB.DB
	addedToParty, box, slot, err := pokebattle.AddPokemonToPartyOrPC(myDB, charID, speciesID, level)
	if err != nil {
		return "", fmt.Errorf("give pokemon %d L%d: %w", speciesID, level, err)
	}

	name := speciesName
	if name == "" {
		name = pokemonSpeciesName(speciesID)
	}
	message := action.Message
	if message == "" {
		message = fmt.Sprintf("Received %s!", name)
	}
	if addedToParty {
		log.Printf("[Cutscene] Added %s L%d to char %d party", name, level, charID)
		sendCutsceneSystemMessage(ses, message)
		sendCutscenePartyUpdate(ses)
		return fmt.Sprintf("%s L%d added to party", name, level), nil
	}

	log.Printf("[Cutscene] Sent %s L%d to char %d PC box %d slot %d", name, level, charID, box, slot)
	sendCutsceneSystemMessage(ses, fmt.Sprintf("%s Sent to BOX %d.", message, box+1))
	sendCutscenePartyUpdate(ses)
	return fmt.Sprintf("%s L%d sent to PC box=%d slot=%d", name, level, box, slot), nil
}

func pokemonSpeciesName(speciesID int) string {
	var name string
	if err := db.GlobalWorldDB.DB.QueryRow(`SELECT name FROM phaser_pokemon WHERE id = $1`, speciesID).Scan(&name); err != nil || name == "" {
		return fmt.Sprintf("POKEMON #%d", speciesID)
	}
	return name
}

func resolveCutscenePokemonSpecies(action CutsceneAction) (int, string, error) {
	speciesID := action.PokemonID
	if speciesID == 0 {
		speciesID = action.SpeciesID
	}
	if speciesID > 0 {
		return speciesID, pokemonSpeciesName(speciesID), nil
	}

	name := action.PokemonName
	if name == "" {
		name = action.PokemonConstant
	}
	lookup := normalizePokemonLookupName(name)
	if lookup == "" {
		return 0, "", fmt.Errorf("missing pokemonId/speciesId/pokemonName/pokemonConstant")
	}
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		return 0, "", fmt.Errorf("database is not initialized")
	}

	var resolvedID int
	var resolvedName string
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT id, name
		FROM phaser_pokemon
		WHERE UPPER(name) = $1
		ORDER BY id
		LIMIT 1`, lookup).Scan(&resolvedID, &resolvedName)
	if err != nil {
		return 0, "", fmt.Errorf("pokemon %q not found: %w", name, err)
	}
	return resolvedID, resolvedName, nil
}

func normalizePokemonLookupName(name string) string {
	name = strings.TrimSpace(strings.ToUpper(name))
	if name == "" {
		return ""
	}
	name = strings.ReplaceAll(name, "POKéMON", "POKEMON")
	name = strings.ReplaceAll(name, "POKÉMON", "POKEMON")
	name = strings.ReplaceAll(name, "MR. MIME", "MR_MIME")
	name = strings.ReplaceAll(name, "MR MIME", "MR_MIME")
	name = strings.ReplaceAll(name, "FARFETCH'D", "FARFETCHD")
	name = strings.ReplaceAll(name, "NIDORAN♂", "NIDORAN_M")
	name = strings.ReplaceAll(name, "NIDORAN♀", "NIDORAN_F")
	name = strings.ReplaceAll(name, "NIDORAN_MALE", "NIDORAN_M")
	name = strings.ReplaceAll(name, "NIDORAN_FEMALE", "NIDORAN_F")
	name = strings.NewReplacer(" ", "_", "-", "_", ".", "", "'", "").Replace(name)
	name = pokemonLookupUnderscorePattern.ReplaceAllString(name, "_")
	return strings.Trim(name, "_")
}

func CutsceneActionSummary(action CutsceneAction) string {
	switch action.Type {
	case "dialogue":
		return fmt.Sprintf("%s %v", action.Speaker, action.Lines)
	case "dialogueText":
		return fmt.Sprintf("%s %s", action.Speaker, action.TextConstant)
	case "choice":
		return fmt.Sprintf("%s %q", action.TextConstant, action.Prompt)
	case "playSFX":
		return action.SFXConstant
	case "playMusic":
		if action.MusicConstant != "" {
			return action.MusicConstant
		}
		return action.MusicPath
	case "playCry":
		if action.PokemonName != "" {
			return action.PokemonName
		}
		return action.PokemonConstant
	case "move", "movePlayer":
		return fmt.Sprintf("%s %v", action.Actor, action.Movements)
	case "parallel":
		return fmt.Sprintf("%d actions", len(action.Actions))
	case "delay":
		return fmt.Sprintf("%dms", action.MS)
	case "setFlag", "resetFlag", "toggleFlag":
		return action.Flag
	case "takeMoney":
		return fmt.Sprintf("%d", action.Money)
	case "giveCoins":
		return fmt.Sprintf("%d", action.Coins)
	case "gameCornerPrizeVendor":
		return fmt.Sprintf("%s window=%d", action.TextConstant, action.PrizeWindow)
	case "hideObject", "showObject":
		mapName := action.ObjectMapName
		if mapName == "" {
			mapName = "current"
		}
		return fmt.Sprintf("%s %s %s %s", mapName, action.ObjectKey, action.TriggerLabel, action.TextConstant)
	case "startWildBattle":
		speciesID := action.PokemonID
		if speciesID == 0 {
			speciesID = action.SpeciesID
		}
		speciesName := action.PokemonName
		if speciesName == "" {
			speciesName = action.PokemonConstant
		}
		if speciesName != "" {
			return fmt.Sprintf("%s L%d winFlag=%s", speciesName, action.Level, action.WinFlag)
		}
		return fmt.Sprintf("#%d L%d winFlag=%s", speciesID, action.Level, action.WinFlag)
	default:
		return ""
	}
}
