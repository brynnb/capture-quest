package world

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/db/pokedex"
	"capturequest/internal/pokebattle"
	"capturequest/internal/scriptedactions"
	"capturequest/internal/session"
)

var pokemonLookupUnderscorePattern = regexp.MustCompile(`_+`)

// CutsceneEndRequest is sent when the client finishes playing a cutscene.
type CutsceneEndRequest struct {
	CompletionToken string `json:"completionToken"`
	ScriptLabel     string `json:"scriptLabel"` // The cutscene that was completed
}

type CutsceneAction = scriptedactions.Action

type CutsceneActionEffect struct {
	Type    string
	Detail  string
	Changed bool
}

type CutsceneActionContext struct {
	Database     *sql.DB
	mutation     *cutsceneMutation
	Session      *session.Session
	WorldHandler *WorldHandler
	EventFlags   *EventFlagManager
	Choice       *bool
	StopAtChoice bool
	issuedSource *cutscenePosition
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

	if ses.IsClosed() {
		return false
	}
	issued, ok := ses.IssuedCutscenes.Claim(charID, req.ScriptLabel, req.CompletionToken)
	if !ok {
		return false
	}
	committed := false
	defer func() { ses.IssuedCutscenes.Finish(req.CompletionToken, committed) }()
	var event issuedCutscene
	if err := json.Unmarshal(issued, &event); err != nil {
		return false
	}

	cs := event.Script
	_, completed, err := ApplyCutsceneScript(ses.CommandContext(), CutsceneActionContext{Session: ses, WorldHandler: wh, EventFlags: wh.EventFlags, issuedSource: &cutscenePosition{mapID: event.MapID, x: event.X, y: event.Y}}, &cs, charID)
	if err != nil {
		log.Printf("[Cutscene] Failed to apply script %s for character %d: %v", cs.ScriptLabel, charID, err)
		SendSystemMessage(ses, "That event could not be completed. Please try again.")
		return false
	}
	committed = true
	if completed {
		sendEventTileStatesForSession(ses, charID, cs.MapName, wh)
		if cutsceneAffectsTrainerCard(&cs) {
			sendTrainerCardResponse(ses, wh)
		}
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

// issuedCutscene is private authorization state, never a client-supplied position.
// Playback is presentation-only; relative actions start from this owned source.
type issuedCutscene struct {
	Script CutsceneScript `json:"script"`
	MapID  int            `json:"mapId"`
	X      int            `json:"x"`
	Y      int            `json:"y"`
}

// SendCutsceneToPlayer sends a cutscene action sequence to a specific player.
func SendCutsceneToPlayer(ses *session.Session, cs *CutsceneScript, handlers ...*WorldHandler) {
	if cs == nil || !ses.HasValidClient() || ses.IsClosed() {
		return
	}
	x, y, mapID, err := currentCutscenePlayerPosition(ses, int64(ses.Client.CharData().ID))
	if err != nil {
		return
	}
	if len(handlers) > 0 && handlers[0] != nil {
		x, y, mapID = handlers[0].ownedPlayerPosition(ses)
	}
	snapshot, err := json.Marshal(issuedCutscene{Script: *cs, MapID: mapID, X: x, Y: y})
	if err != nil {
		log.Printf("[Cutscene] Encode issued event: %v", err)
		return
	}
	token, err := ses.IssuedCutscenes.Issue(int64(ses.Client.CharData().ID), cs.ScriptLabel, snapshot)
	if err != nil {
		log.Printf("[Cutscene] Issue event: %v", err)
		return
	}
	actions := cs.Actions
	if len(handlers) > 0 && handlers[0] != nil {
		if annotated, err := annotateCutsceneActionsForClient(cs, handlers[0]); err != nil {
			log.Printf("[Cutscene] Failed to annotate client actions for %s: %v", cs.ScriptLabel, err)
		} else if len(annotated) > 0 {
			actions = annotated
		}
	}
	payload := map[string]interface{}{
		"scriptLabel":     cs.ScriptLabel,
		"completionToken": token,
		"mapName":         cs.MapName,
		"actions":         json.RawMessage(actions),
	}
	if err := ses.SendStreamJSON(payload, opcodes.CutsceneStartNotify); err != nil {
		ses.IssuedCutscenes.Finish(token, true)
		log.Printf("[Cutscene] Send issued event: %v", err)
		return
	}
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
			objectIDs, err := resolveCutsceneObjectIDs(db.GlobalWorldDB.DB, actionMapName, actions[i])
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

func applyCutsceneActionList(ctx CutsceneActionContext, mapName string, rawActions json.RawMessage, charID int64) ([]CutsceneActionEffect, bool, error) {
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
			nestedEffects, completed, err := applyCutsceneActionList(ctx, mapName, nestedRaw, charID)
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
			detail, err := applyMovePlayerAction(ctx, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "givePokemon":
			detail, err := applyGivePokemonAction(ctx, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "setFlag", "resetFlag", "toggleFlag":
			if action.Flag == "" {
				return effects, false, fmt.Errorf("%s missing flag", action.Type)
			}
			on := action.Type != "resetFlag"
			if action.Type == "toggleFlag" {
				previous, err := queryEventFlag(ctx.mutation.database, charID, action.Flag)
				if err != nil {
					return effects, false, err
				}
				on = !previous
			}
			if err := ctx.mutation.setFlag(action.Flag, on); err != nil {
				return effects, false, err
			}
			ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(publishCtx CutsceneActionContext) error {
				sendCutsceneEventTileStates(publishCtx, charID, mapName)
				return nil
			})
			effect.Changed = true
			if action.Type == "toggleFlag" {
				effect.Detail = fmt.Sprintf("%s=%t", action.Flag, on)
			}
		case "hideObject":
			detail, err := applyObjectVisibilityAction(ctx, mapName, action, charID, true)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "showObject":
			detail, err := applyObjectVisibilityAction(ctx, mapName, action, charID, false)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "giveItem":
			detail, itemName, err := applyGiveItemAction(ctx, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
			ctx.state.LastGivenItemName = itemName
		case "takeItem":
			detail, err := applyTakeItemAction(ctx, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "dialogue":
			action.Lines = fillBufferedItemDialogueLines(action.Lines, ctx.state.LastGivenItemName)
			effect.Detail = CutsceneActionSummary(action)
		case "takeMoney":
			detail, err := applyTakeMoneyAction(ctx, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "giveCoins":
			detail, err := applyGiveCoinsAction(ctx, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "gameCornerPrizeVendor":
			ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error {
				_, err := sendGameCornerPrizeList(p.Session, charID, action.PrizeWindow)
				return err
			})
		case "healParty":
			detail, err := applyHealPartyAction(ctx, charID)
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
			detail, err := applyStartTrainerBattleAction(ctx, mapName, action, charID)
			if err != nil {
				return effects, false, err
			}
			effect.Detail = detail
			effect.Changed = true
		case "startWildBattle":
			detail, err := applyStartWildBattleAction(ctx, mapName, action, charID)
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

func applyMovePlayerAction(ctx CutsceneActionContext, action CutsceneAction, charID int64) (string, error) {
	if len(action.Movements) == 0 {
		return "", fmt.Errorf("movePlayer missing movements")
	}
	x, y, mapID, err := ctx.mutation.currentPosition(ctx)
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
	if err := ctx.mutation.movePlayer(ctx, mapID, x, y, direction, false); err != nil {
		return "", err
	}
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
		}
		return
	}

	publishCommittedPlayerPosition(ses, wh, mapID, x, y, direction)
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

func applyGiveItemAction(ctx CutsceneActionContext, action CutsceneAction, charID int64) (string, string, error) {
	itemID, name, err := resolveCutsceneItem(ctx.mutation.database, action)
	if err != nil {
		return "", "", err
	}
	quantity := action.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	if quantity > 65535 {
		return "", "", fmt.Errorf("giveItem quantity %d exceeds inventory range", quantity)
	}
	if _, err := cqitems.NewStore(ctx.mutation.database).AddItemToInventory(int32(charID), int32(itemID), uint16(quantity)); err != nil {
		return "", "", err
	}
	ctx.mutation.messages = append(ctx.mutation.messages, fmt.Sprintf("Received %s!", name))
	ctx.mutation.inventoryChanged = true
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

func applyTakeItemAction(ctx CutsceneActionContext, action CutsceneAction, charID int64) (string, error) {
	itemID, name, err := resolveCutsceneItem(ctx.mutation.database, action)
	if err != nil {
		return "", err
	}
	quantity := action.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	if quantity > 65535 {
		return "", fmt.Errorf("takeItem quantity %d exceeds inventory range", quantity)
	}
	store := cqitems.NewStore(ctx.mutation.database)
	for i := 0; i < quantity; i++ {
		found, err := store.FindInventoryItemByItemID(int32(charID), int32(itemID))
		if err != nil {
			return "", err
		}
		if _, err := store.DecrementItemQuantity(int32(charID), found.Instance.ID); err != nil {
			return "", err
		}
	}
	ctx.mutation.inventoryChanged = true
	return fmt.Sprintf("%s x%d removed", name, quantity), nil
}

func applyTakeMoneyAction(ctx CutsceneActionContext, action CutsceneAction, charID int64) (string, error) {
	if action.Money <= 0 {
		return "", fmt.Errorf("takeMoney missing money")
	}
	var remaining int
	if err := ctx.mutation.database.QueryRow(`UPDATE character_wallet SET pokedollars=pokedollars-$1 WHERE character_id=$2 AND pokedollars>=$1 RETURNING pokedollars`, action.Money, charID).Scan(&remaining); err != nil {
		return "", err
	}
	ctx.mutation.messages = append(ctx.mutation.messages, fmt.Sprintf("Spent %d Pokedollars.", action.Money))
	ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error {
		if p.Session != nil {
			p.Session.SendStreamJSON(map[string]interface{}{"characterId": charID, "pokedollars": remaining}, opcodes.CharacterWallet)
		}
		return nil
	})
	return fmt.Sprintf("spent=%d money=%d", action.Money, remaining), nil
}

func applyGiveCoinsAction(ctx CutsceneActionContext, action CutsceneAction, charID int64) (string, error) {
	if action.Coins <= 0 {
		return "", fmt.Errorf("giveCoins missing coins")
	}
	total, err := addCoinsInTransaction(ctx.mutation.database, charID, action.Coins)
	if err != nil {
		return "", err
	}
	ctx.mutation.messages = append(ctx.mutation.messages, fmt.Sprintf("Received %d coins!", action.Coins))
	return fmt.Sprintf("coins=%d total=%d", action.Coins, total), nil
}

func applyHealPartyAction(ctx CutsceneActionContext, charID int64) (string, error) {
	party, err := ctx.mutation.loadParty()
	if err != nil {
		return "", err
	}
	HealPokemonParty(party)
	ctx.mutation.partyDirty = true
	return fmt.Sprintf("healed %d pokemon", len(party)), nil
}

func applyStartSafariSessionAction(ctx CutsceneActionContext, action CutsceneAction, charID int64) (string, bool, error) {
	result, err := startSafariVisitIn(ctx.mutation.database, charID)
	if err != nil {
		return "", false, err
	}
	if !result.Success {
		ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error { sendSafariEntryFailure(p.Session, result); return nil })
		return fmt.Sprintf("success=false money=%d message=%q", result.Money, result.Message), false, nil
	}
	if err := ctx.mutation.setFlag(EventInSafariZone, true); err != nil {
		return "", false, err
	}
	if err := ctx.mutation.setFlag(EventSafariGameOver, false); err != nil {
		return "", false, err
	}
	ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error { sendSafariEntrySuccess(p.Session, result); return nil })
	mapID, x, y := safariEntryDestination(action)
	direction := normalizeWarpDirection(action.Direction)
	if direction == "" {
		direction = "UP"
	}
	if err := ctx.mutation.movePlayer(ctx, mapID, x, y, direction, true); err != nil {
		return "", false, err
	}
	return fmt.Sprintf("success=true money=%d balls=%d steps=%d warp=%d(%d,%d)", result.Money, result.BallsLeft, result.StepsLeft, mapID, x, y), true, nil
}

func applyEndSafariSessionAction(ctx CutsceneActionContext, charID int64) (string, bool, error) {
	previous, err := safariSessionIn(ctx.mutation.database, charID)
	if err != nil {
		return "", false, err
	}
	if err := ctx.mutation.setFlag(EventInSafariZone, false); err != nil {
		return "", false, err
	}
	if err := ctx.mutation.setFlag(EventSafariGameOver, false); err != nil {
		return "", false, err
	}
	if err := saveSafariSessionIn(ctx.mutation.database, charID, nil); err != nil {
		return "", false, err
	}
	ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error {
		sendSafariManualExit(p.Session)
		return nil
	})
	if previous != nil {
		return fmt.Sprintf("active=%t balls=%d steps=%d ended=true", previous.Active, previous.BallsLeft, previous.StepsLeft), true, nil
	}
	return "ended=true", true, nil
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

func resolveCutsceneItem(database db.DBTX, action CutsceneAction) (int, string, error) {
	if action.ItemID > 0 {
		var name string
		if err := database.QueryRow(`SELECT name FROM cq_items WHERE id=$1`, action.ItemID).Scan(&name); err != nil {
			return 0, "", fmt.Errorf("lookup item %d: %w", action.ItemID, err)
		}
		return action.ItemID, name, nil
	}
	if action.ItemName == "" {
		return 0, "", fmt.Errorf("%s action missing itemId/itemName", action.Type)
	}
	var id int
	var name string
	if err := database.QueryRow(
		`SELECT id, name FROM cq_items WHERE name = $1 OR short_name = $2 LIMIT 1`,
		action.ItemName, action.ItemName).Scan(&id, &name); err != nil {
		return 0, "", fmt.Errorf("lookup item %s: %w", action.ItemName, err)
	}
	return id, name, nil
}

func applyStartTrainerBattleAction(ctx CutsceneActionContext, mapName string, action CutsceneAction, charID int64) (string, error) {
	partyIndex, err := resolveCutsceneTrainerPartyIndex(ctx.mutation.database, action, charID)
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
	if ctx.mutation.boundBattle {
		return "", fmt.Errorf("cannot start another battle inside an unsettled battle turn")
	}
	if ctx.mutation.battle != nil {
		return "", fmt.Errorf("script starts more than one battle")
	}
	if err := ctx.mutation.saveParty(); err != nil {
		return "", err
	}
	battle, events, err := prepareScriptedTrainerBattle(ctx.mutation.database, charID, ScriptedTrainerBattleSpec{
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

	ctx.mutation.battle = battle
	ctx.mutation.party = &battle.PlayerParty
	ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error {
		setBattle(charID, battle)
		if p.Session == nil {
			return nil
		}
		resp := buildBattleStateResponse(battle)
		resp["trainerClass"] = action.TrainerClass
		if battle.Trainer != nil {
			resp["trainerName"] = battle.Trainer.Name
		}
		resp["events"] = events
		p.Session.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
		return nil
	})
	enemy := make([]string, 0, len(battle.EnemyParty))
	for _, pokemon := range battle.EnemyParty {
		if pokemon != nil {
			enemy = append(enemy, fmt.Sprintf("#%d %s L%d", pokemon.ID, pokemon.Name, pokemon.Level))
		}
	}
	return fmt.Sprintf("%s party=%d enemy=%v winFlag=%s", action.TrainerClass, partyIndex, enemy, action.WinFlag), nil
}

func applyStartWildBattleAction(ctx CutsceneActionContext, mapName string, action CutsceneAction, charID int64) (string, error) {
	pokemonID, _, err := resolveCutscenePokemonSpecies(ctx.mutation.database, action)
	if err != nil {
		return "", fmt.Errorf("startWildBattle: %w", err)
	}
	postWinActions, err := json.Marshal(action.PostWinActions)
	if err != nil {
		return "", fmt.Errorf("marshal post-win actions: %w", err)
	}
	if ctx.mutation.boundBattle {
		return "", fmt.Errorf("cannot start another battle inside an unsettled battle turn")
	}
	if ctx.mutation.battle != nil {
		return "", fmt.Errorf("script starts more than one battle")
	}
	if err := ctx.mutation.saveParty(); err != nil {
		return "", err
	}
	battle, events, err := prepareScriptedWildBattle(ctx.mutation.database, charID, ScriptedWildBattleSpec{
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

	ctx.mutation.battle = battle
	ctx.mutation.party = &battle.PlayerParty
	ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error {
		setBattle(charID, battle)
		if p.Session == nil {
			return nil
		}
		resp := buildBattleStateResponse(battle)
		resp["events"] = events
		p.Session.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
		return nil
	})
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

func resolveCutsceneTrainerPartyIndex(database db.DBTX, action CutsceneAction, charID int64) (int, error) {
	if action.TrainerPartyIndex > 0 {
		return action.TrainerPartyIndex, nil
	}
	selected := 0
	for flag, index := range action.PartyByFlag {
		on, err := queryEventFlag(database, charID, flag)
		if err != nil {
			return 0, err
		}
		if on {
			if selected != 0 && selected != index {
				return 0, fmt.Errorf("ambiguous trainer party flags")
			}
			selected = index
		}
	}
	if selected <= 0 {
		return 0, fmt.Errorf("startTrainerBattle action missing partyIndex or matching partyByFlag")
	}
	return selected, nil
}

func applyObjectVisibilityAction(ctx CutsceneActionContext, mapName string, action CutsceneAction, charID int64, hide bool) (string, error) {
	objectMapName := mapName
	if action.ObjectMapName != "" {
		objectMapName = action.ObjectMapName
	}
	objectIDs, err := resolveCutsceneObjectIDs(ctx.mutation.database, objectMapName, action)
	if err != nil {
		return "", err
	}
	if len(objectIDs) == 0 {
		return "", fmt.Errorf("%s action could not resolve an object on map %s", action.Type, objectMapName)
	}
	for _, objectID := range objectIDs {
		if hide {
			_, err = ctx.mutation.database.Exec(`INSERT INTO character_collected_items(character_id,object_id) VALUES($1,$2) ON CONFLICT(character_id,object_id) DO NOTHING`, charID, objectID)
		} else {
			_, err = ctx.mutation.database.Exec(`DELETE FROM character_collected_items WHERE character_id=$1 AND object_id=$2`, charID, objectID)
		}
		if err != nil {
			return "", err
		}
		if err := setCharacterObjectVisibilityOverride(ctx.mutation.database, charID, objectID, !hide, "CutsceneAction:"+action.Type); err != nil {
			return "", err
		}
		ctx.mutation.publishActions = append(ctx.mutation.publishActions, func(p CutsceneActionContext) error {
			if p.Session == nil || p.WorldHandler == nil {
				return nil
			}
			if hide && p.WorldHandler.ActorRegistry != nil {
				p.Session.SendStreamJSON(map[string]interface{}{"id": p.WorldHandler.ActorRegistry.GetPhaserID(ActorTypeNPC, objectID)}, opcodes.PhaserActorDespawn)
			}
			if !hide && p.WorldHandler.ActorManager != nil {
				return p.WorldHandler.ActorManager.SendObjectActorToSession(objectID, p.Session)
			}
			return nil
		})
	}
	return describeCutsceneObjectIDs(ctx.mutation.database, objectIDs), nil
}

func describeCutsceneObjectIDs(database db.DBTX, ids []int) string {
	labels := make([]string, 0, len(ids))
	for _, id := range ids {
		labels = append(labels, cutsceneObjectDisplayLabel(database, id))
	}
	return fmt.Sprintf("objects=[%s]", strings.Join(labels, ", "))
}

func cutsceneObjectDisplayLabel(database db.DBTX, id int) string {
	var name, text string
	err := database.QueryRow(`
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

func resolveCutsceneObjectIDs(database db.DBTX, mapName string, action CutsceneAction) ([]int, error) {
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
		resolved, err := resolveCutsceneObjectKey(database, mapName, key)
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
	return resolveCutsceneObjectKey(db.GlobalWorldDB.DB, mapName, key)
}

func resolveCutsceneObjectKey(database db.DBTX, mapName string, key string) ([]int, error) {
	var explicitID int
	if _, err := fmt.Sscanf(key, "object:%d", &explicitID); err == nil && explicitID > 0 {
		return []int{explicitID}, nil
	}
	if _, err := fmt.Sscanf(key, "phaser_object:%d", &explicitID); err == nil && explicitID > 0 {
		return []int{explicitID}, nil
	}

	ids, err := queryCutsceneObjectIDsByMapName(database, mapName, key)
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		return ids, nil
	}

	if strings.HasPrefix(key, "HS_") {
		ids, err = queryCutsceneObjectIDsByMissableConstant(database, key)
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}

	if !isCutsceneOverworldMapName(database, mapName) {
		return ids, nil
	}

	rows, err := database.Query(`
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

func queryCutsceneObjectIDsByMapName(database db.DBTX, mapName string, key string) ([]int, error) {
	rows, err := database.Query(`
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

func queryCutsceneObjectIDsByMissableConstant(database db.DBTX, hsConstant string) ([]int, error) {
	rows, err := database.Query(`
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

func isCutsceneOverworldMapName(database db.DBTX, mapName string) bool {
	var isOverworld bool
	if err := database.QueryRow(
		`SELECT is_overworld FROM phaser_maps WHERE name = $1`, mapName).Scan(&isOverworld); err != nil {
		return false
	}
	return isOverworld
}

func applyGivePokemonAction(ctx CutsceneActionContext, action CutsceneAction, charID int64) (string, error) {
	speciesID, name, err := resolveCutscenePokemonSpecies(ctx.mutation.database, action)
	if err != nil {
		return "", err
	}
	level := action.Level
	if level <= 0 {
		level = 5
	}
	pokemon, err := pokebattle.BuildWildPokemon(ctx.mutation.database, speciesID, level)
	if err != nil {
		return "", err
	}
	pokemon.IsWild = false
	pokemon.OriginalTrainerID = charID
	party, err := ctx.mutation.loadParty()
	if err != nil {
		return "", err
	}
	if err := pokedex.MarkCaught(ctx.mutation.database, charID, speciesID); err != nil {
		return "", err
	}
	message := action.Message
	if message == "" {
		message = fmt.Sprintf("Received %s!", name)
	}
	if len(party) < 6 {
		*ctx.mutation.party = append(party, pokemon)
		ctx.mutation.partyDirty = true
		ctx.mutation.messages = append(ctx.mutation.messages, message)
		return fmt.Sprintf("%s L%d added to party", name, level), nil
	}
	box, slot, err := pokebattle.SavePokemonToPC(ctx.mutation.database, charID, pokemon)
	if err != nil {
		return "", err
	}
	ctx.mutation.messages = append(ctx.mutation.messages, fmt.Sprintf("%s Sent to BOX %d.", message, box+1))
	return fmt.Sprintf("%s L%d sent to PC box=%d slot=%d", name, level, box, slot), nil
}

func resolveCutscenePokemonSpecies(database db.DBTX, action CutsceneAction) (int, string, error) {
	speciesID := action.PokemonID
	if speciesID == 0 {
		speciesID = action.SpeciesID
	}
	if speciesID > 0 {
		var name string
		if err := database.QueryRow(`SELECT name FROM phaser_pokemon WHERE id=$1`, speciesID).Scan(&name); err != nil {
			return 0, "", fmt.Errorf("lookup pokemon %d: %w", speciesID, err)
		}
		return speciesID, name, nil
	}

	name := action.PokemonName
	if name == "" {
		name = action.PokemonConstant
	}
	lookup := normalizePokemonLookupName(name)
	if lookup == "" {
		return 0, "", fmt.Errorf("missing pokemonId/speciesId/pokemonName/pokemonConstant")
	}
	if database == nil {
		return 0, "", fmt.Errorf("database is not initialized")
	}

	var resolvedID int
	var resolvedName string
	err := database.QueryRow(`
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
