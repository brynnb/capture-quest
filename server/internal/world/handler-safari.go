package world

import (
	"encoding/json"
	"errors"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/itemuse"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

// HandleSafariZoneEnter handles the player entering the Safari Zone.
// If payload contains "statusOnly":true, it only returns existing session data
// (used on reconnect/warp) and never creates a new session.
// Otherwise it creates a new session (used by gate NPC interaction).
func HandleSafariZoneEnter(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		StatusOnly bool `json:"statusOnly"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	// Check if already in safari
	existing, err := wh.Safari.GetSession(ses.CommandContext(), charID)
	if err != nil {
		safariStorageError(ses, charID, err, opcodes.SafariZoneEnterResponse)
		return false
	}
	if existing != nil && existing.Active {
		ses.SendStreamJSON(map[string]interface{}{
			"success":   true,
			"ballsLeft": existing.BallsLeft,
			"stepsLeft": existing.StepsLeft,
		}, opcodes.SafariZoneEnterResponse)
		// Also send step update so the HUD shows on reconnect
		ses.SendStreamJSON(map[string]interface{}{
			"stepsLeft": existing.StepsLeft,
			"ballsLeft": existing.BallsLeft,
		}, opcodes.SafariZoneStepUpdate)
		// If there's an active battle, re-send it so the encounter UI resumes
		if existing.Battle != nil && !existing.Battle.IsOver() {
			wild := existing.Battle.WildPokemon
			ses.SendStreamJSON(map[string]interface{}{
				"battleId": existing.Battle.BattleID, "revision": existing.Battle.Revision,
				"pokemon": map[string]interface{}{
					"id":    wild.ID,
					"name":  wild.Name,
					"level": wild.Level,
					"hp":    wild.CurHP,
					"maxHp": wild.MaxHP,
				},
				"ballsLeft": existing.BallsLeft,
				"stepsLeft": existing.StepsLeft,
			}, opcodes.SafariBattleStartNotify)
		}
		return false
	}

	// Status-only check: no existing session, so just report no active safari
	if req.StatusOnly {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"message": "no active safari session",
		}, opcodes.SafariZoneEnterResponse)
		return false
	}

	if getBattle(charID) != nil || wh.authorizeSourceInteraction(ses, SafariZoneGateMapID, "TEXT_SAFARIZONEGATE_SAFARI_ZONE_WORKER1") != nil {
		sendSafariEntryFailure(ses, SafariEntryResult{Message: "Please check in at the counter first."})
		return false
	}
	result, err := TryStartSafariZoneVisit(ses.CommandContext(), charID, wh.Safari)
	if err != nil {
		safariStorageError(ses, charID, err, opcodes.SafariZoneEnterResponse)
		return false
	}
	refreshSafariFlags(ses, wh, charID)
	if !result.Success {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"message": result.Message,
			"money":   result.Money,
		}, opcodes.SafariZoneEnterResponse)
		return false
	}

	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"ballsLeft": result.BallsLeft,
		"stepsLeft": result.StepsLeft,
		"money":     result.Money,
	}, opcodes.SafariZoneEnterResponse)

	// Also send a step update so the HUD shows immediately
	ses.SendStreamJSON(map[string]interface{}{
		"stepsLeft": result.StepsLeft,
		"ballsLeft": result.BallsLeft,
	}, opcodes.SafariZoneStepUpdate)

	return false
}

// HandleSafariBattleAction processes a safari battle action (ball, bait, rock, run).
type SafariBattleActionRequest struct {
	RequestID string                `json:"requestId"`
	Battle    BattleCommandIdentity `json:"battle"`
	Action    string                `json:"action"`
}
type SafariBattleActionResponse struct {
	ExitMessage string                               `json:"exitMessage,omitempty"`
	PlayerParty []PokemonDTO                         `json:"playerParty,omitempty"`
	Success     bool                                 `json:"success" tstype:"true"`
	RequestID   string                               `json:"requestId"`
	BattleID    string                               `json:"battleId"`
	Revision    int64                                `json:"revision"`
	Position    protocol.OwnedPlayerPositionResponse `json:"position" tstype:"import(\"./protocol\").OwnedPlayerPositionResponse"`
	Events      []pokebattle.SafariBattleEvent       `json:"events" tstype:"import(\"./battle_events\").SafariBattleEvent[]"`
	BallsLeft   int                                  `json:"ballsLeft"`
	StepsLeft   int                                  `json:"stepsLeft"`
	IsOver      bool                                 `json:"isOver"`
	Caught      bool                                 `json:"caught"`
	Fled        bool                                 `json:"fled"`
	Closed      bool                                 `json:"closed,omitempty"`
	SentToPC    bool                                 `json:"sentToPC,omitempty"`
	PCBox       int                                  `json:"pcBox,omitempty"`
	SafariOver  bool                                 `json:"safariOver,omitempty"`
}

func HandleSafariBattleAction(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req SafariBattleActionRequest
	if err := decodePlayerMovement(payload, &req); err != nil || !validBattleRequestID(req.RequestID) {
		sendBattleCommandError(ses, req.RequestID, opcodes.SafariBattleActionResponse, "Invalid Safari action request")
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	_, _, mapID := wh.ownedPlayerPosition(ses)
	if (!IsInSafariZone(mapID) && !(req.Action == "close" && mapID == SafariZoneGateMapID)) || getBattle(charID) != nil {
		sendBattleCommandError(ses, req.RequestID, opcodes.SafariBattleActionResponse, "not in safari battle")
		return false
	}

	result, err := wh.Safari.act(ses.CommandContext(), charID, req.Action, req.Battle)
	if err != nil {
		var rejection *itemuse.Rejection
		if errors.As(err, &rejection) {
			sendBattleCommandError(ses, req.RequestID, opcodes.SafariBattleActionResponse, rejection.Message)
		} else {
			log.Printf("[Safari] Character %d action commit: %v", charID, err)
			sendBattleCommandError(ses, req.RequestID, opcodes.SafariBattleActionResponse, "Could not save Safari action. Reconnect to recover its current state.")
		}
		return false
	}
	safariSes, battle := result.Visit, result.Battle
	// Build response
	// Project exhaustion before taking the committed response's owned position.
	if !safariSes.Active && !result.Closed {
		publishSafariExpiry(ses, wh, charID)
	}
	resp := SafariBattleActionResponse{Success: true, RequestID: req.RequestID, BattleID: battle.BattleID, Revision: battle.Revision, Position: wh.ownedPlayerSnapshot(ses, req.RequestID), Events: battle.Events, BallsLeft: safariSes.BallsLeft, StepsLeft: safariSes.StepsLeft, IsOver: battle.IsOver(), Caught: battle.Caught, Fled: battle.Fled, Closed: result.Closed, SafariOver: !safariSes.Active}
	if result.Closed || resp.Events == nil {
		resp.Events = []pokebattle.SafariBattleEvent{}
	}
	for _, p := range result.Party {
		resp.PlayerParty = append(resp.PlayerParty, pokemonToDTO(p))
	}
	if resp.SafariOver {
		resp.ExitMessage = SafariExpiryMessage
	}

	if battle.Caught {
		caughtPoke := battle.WildPokemon
		resp.SentToPC = result.SentToPC
		if result.SentToPC {
			resp.PCBox = result.PCBox + 1 // 1-indexed for display
		}
		log.Printf("[Safari] Player %d caught L%d %s", charID, caughtPoke.Level, caughtPoke.Name)
	}

	ses.SendStreamJSON(resp, opcodes.SafariBattleActionResponse)

	return false
}

// CheckSafariStep is called from the movement tick when a player steps in a safari zone.
// It decrements the step counter, checks for encounters, and handles expiry.
type safariStepResult struct {
	Visit   *SafariSession
	Expired bool
}

func prepareSafariStepIn(tx db.DBTX, charID int64, x, y, mapID int, wh *WorldHandler) (safariStepResult, error) {
	var result safariStepResult
	s, err := safariSessionIn(tx, charID)
	if err != nil {
		return result, err
	}
	err = func() error {
		if s == nil || !s.Active || s.Battle != nil {
			return nil
		}
		result.Visit = s
		result.Expired = advanceSafariStep(s)
		if result.Expired {
			return expireSafariVisitIn(tx, charID)
		}
		if wh.WildEncounter == nil {
			return nil
		}
		area, err := wh.WildEncounter.encounterAreaIn(tx, mapID, x, y)
		if err != nil {
			return err
		}
		if area == nil || area.EncounterRate == 0 || len(area.Slots) == 0 || wh.WildEncounter.encounterRoll(256) >= area.EncounterRate {
			return nil
		}
		pokemonID, level := wh.WildEncounter.selectEncounterPokemon(area)
		wild, err := pokebattle.BuildWildPokemon(tx, pokemonID, level)
		if err != nil {
			return err
		}
		s.Battle = pokebattle.NewSafariBattle(wild, s.BallsLeft, s.StepsLeft)
		return markPokemonSeen(tx, charID, pokemonID)
	}()
	if err != nil {
		return safariStepResult{}, err
	}
	if err := saveSafariSessionIn(tx, charID, s); err != nil {
		return safariStepResult{}, err
	}
	return result, nil
}

func CheckSafariStep(charID int64, x, y, mapID int, ses *session.Session, wh *WorldHandler) bool {
	if !IsInSafariZone(mapID) {
		return false
	}
	var result safariStepResult
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var err error
		result, err = prepareSafariStepIn(tx, charID, x, y, mapID, wh)
		return err
	})
	if err != nil {
		log.Printf("[Safari] Step for %d: %v", charID, err)
		return true
	}
	return publishSafariStep(ses, wh, charID, result)
}

func publishSafariStep(ses *session.Session, wh *WorldHandler, charID int64, result safariStepResult) bool {
	if result.Visit == nil {
		return false
	}
	if result.Expired {
		publishSafariExpiry(ses, wh, charID)
		sendCommittedSafariExit(ses, result.Visit.BallsLeft)
		return true
	}
	ses.SendStreamJSON(map[string]interface{}{"stepsLeft": result.Visit.StepsLeft, "ballsLeft": result.Visit.BallsLeft}, opcodes.SafariZoneStepUpdate)
	if result.Visit.Battle == nil {
		return false
	}
	wild := result.Visit.Battle.WildPokemon
	ses.SendStreamJSON(map[string]interface{}{"battleId": result.Visit.Battle.BattleID, "revision": result.Visit.Battle.Revision, "pokemon": map[string]interface{}{"id": wild.ID, "name": wild.Name, "level": wild.Level, "hp": wild.CurHP, "maxHp": wild.MaxHP, "spriteId": wild.ID, "catchRate": wild.CatchRate}, "ballsLeft": result.Visit.BallsLeft, "stepsLeft": result.Visit.StepsLeft}, opcodes.SafariBattleStartNotify)
	return true
}

func safariStorageError(ses *session.Session, charID int64, err error, opcode opcodes.OpCode) {
	log.Printf("[Safari] Character %d: %v", charID, err)
	ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Safari state could not be saved. Please try again.", "message": "Safari state is unavailable. Please try again."}, opcode)
}
func refreshSafariFlags(ses *session.Session, wh *WorldHandler, charID int64) {
	if wh != nil && wh.EventFlags != nil {
		if err := wh.EventFlags.LoadFlagsContext(ses.CommandContext(), charID); err != nil {
			log.Printf("[Safari] Refresh flags for %d: %v", charID, err)
		}
	}
}

func publishSafariExpiry(ses *session.Session, wh *WorldHandler, charID int64) {
	refreshSafariFlags(ses, wh, charID)
	publishCommittedPlayerPosition(ses, wh, SafariZoneGateMapID, SafariZoneGateReturnX, SafariZoneGateReturnY, "DOWN")
}

func sendCommittedSafariExit(ses *session.Session, ballsLeft int) {
	ses.SendStreamJSON(protocol.SafariZoneExitNotify{
		WarpTileTeleportNotify: protocol.WarpTileTeleportNotify{MapID: SafariZoneGateMapID, X: SafariZoneGateReturnX, Y: SafariZoneGateReturnY, Direction: "DOWN"},
		StepsLeft:              0, BallsLeft: ballsLeft, Message: SafariExpiryMessage,
	}, opcodes.SafariZoneExitNotify)
}
