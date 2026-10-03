package world

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/itemuse"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

func endSafariSessionIfLeavingMap(charID int64, sourceMapID, destMapID int, wh *WorldHandler) (bool, error) {
	if wh == nil || wh.Safari == nil || !IsInSafariZone(sourceMapID) || IsInSafariZone(destMapID) || destMapID == SafariZoneGateMapID {
		return false, nil
	}
	var ended bool
	err := db.Transaction(context.Background(), wh.Safari.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var err error
		ended, err = endSafariForDestinationIn(tx, charID, destMapID)
		return err
	})
	if err != nil {
		return false, err
	}
	if ended {
		refreshSafariFlags(wh, charID)
	}
	return ended, nil
}

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
	existing, err := wh.Safari.GetSession(charID)
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
	result, err := TryStartSafariZoneVisit(charID, wh.Safari)
	if err != nil {
		safariStorageError(ses, charID, err, opcodes.SafariZoneEnterResponse)
		return false
	}
	refreshSafariFlags(wh, charID)
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
func HandleSafariBattleAction(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		Action string `json:"action"` // "ball", "bait", "rock", "run"
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Safari] Invalid action request: %v", err)
		return false
	}

	char := ses.Client.CharData()
	if char == nil {
		return false
	}
	charID := int64(char.ID)

	_, _, mapID := wh.scriptPlayerPosition(ses)
	if !IsInSafariZone(mapID) || getBattle(charID) != nil {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "not in safari battle"}, opcodes.SafariBattleActionResponse)
		return false
	}

	result, err := wh.Safari.act(charID, req.Action)
	if err != nil {
		var rejection *itemuse.Rejection
		if errors.As(err, &rejection) {
			ses.SendStreamJSON(map[string]interface{}{"success": false, "error": rejection.Message}, opcodes.SafariBattleActionResponse)
		} else {
			safariStorageError(ses, charID, err, opcodes.SafariBattleActionResponse)
		}
		return false
	}
	safariSes, battle := result.Visit, result.Battle
	// Build response
	resp := map[string]interface{}{
		"success":   true,
		"events":    battle.Events,
		"ballsLeft": safariSes.BallsLeft,
		"stepsLeft": safariSes.StepsLeft,
		"isOver":    battle.IsOver(),
		"caught":    battle.Caught,
		"fled":      battle.Fled,
	}

	if battle.Caught {
		caughtPoke := battle.WildPokemon
		resp["caughtPokemon"] = map[string]interface{}{
			"id":    caughtPoke.ID,
			"name":  caughtPoke.Name,
			"level": caughtPoke.Level,
		}
		resp["sentToPC"] = result.SentToPC
		if result.SentToPC {
			resp["pcBox"] = result.PCBox + 1 // 1-indexed for display
		}
		log.Printf("[Safari] Player %d caught L%d %s", charID, caughtPoke.Level, caughtPoke.Name)
	}

	if !safariSes.Active {
		resp["safariOver"] = true
	}

	ses.SendStreamJSON(resp, opcodes.SafariBattleActionResponse)

	// If safari visit is over (out of balls), send exit notification to warp player back
	if !safariSes.Active {
		publishSafariExpiry(ses, wh, charID)
		ses.SendStreamJSON(map[string]interface{}{
			"stepsLeft": 0,
			"ballsLeft": 0,
			"message":   "PA: Ding-dong! Your SAFARI GAME is over!",
			"mapId":     SafariZoneGateMapID,
			"x":         SafariZoneGateReturnX,
			"y":         SafariZoneGateReturnY,
			"direction": "DOWN",
		}, opcodes.SafariZoneExitNotify)
	}

	return false
}

// CheckSafariStep is called from the movement tick when a player steps in a safari zone.
// It decrements the step counter, checks for encounters, and handles expiry.
func CheckSafariStep(charID int64, x, y, mapID int, ses *session.Session, wh *WorldHandler) bool {
	if !IsInSafariZone(mapID) {
		return false
	}

	var safariSes *SafariSession
	var expired bool
	err := wh.Safari.mutate(charID, func(tx db.DBTX, s *SafariSession) error {
		if s == nil || !s.Active || s.Battle != nil {
			return nil
		}
		safariSes = s
		expired = advanceSafariStep(s)
		if expired {
			return expireSafariVisitIn(tx, charID)
		}
		if wh.WildEncounter == nil {
			return nil
		}
		areaID := wh.WildEncounter.getEncounterAreaID(mapID, x, y)
		area := wh.WildEncounter.areas[areaID]
		if area == nil || area.EncounterRate == 0 || len(area.Slots) == 0 || rand.Intn(256) >= area.EncounterRate {
			return nil
		}
		pokemonID, level := wh.WildEncounter.selectEncounterPokemon(area)
		wild, err := pokebattle.BuildWildPokemon(tx, pokemonID, level)
		if err != nil {
			return err
		}
		s.Battle = pokebattle.NewSafariBattle(wild, s.BallsLeft, s.StepsLeft)
		return markPokemonSeen(tx, charID, pokemonID)
	})
	if err != nil {
		log.Printf("[Safari] Step for %d: %v", charID, err)
		return true
	}
	if safariSes == nil {
		return false
	}
	if expired {
		publishSafariExpiry(ses, wh, charID)
		ses.SendStreamJSON(map[string]interface{}{"stepsLeft": 0, "ballsLeft": safariSes.BallsLeft, "message": "PA: Ding-dong! Your SAFARI GAME is over!", "mapId": SafariZoneGateMapID, "x": SafariZoneGateReturnX, "y": SafariZoneGateReturnY, "direction": "DOWN"}, opcodes.SafariZoneExitNotify)
		return true
	}
	ses.SendStreamJSON(map[string]interface{}{"stepsLeft": safariSes.StepsLeft, "ballsLeft": safariSes.BallsLeft}, opcodes.SafariZoneStepUpdate)
	if safariSes.Battle == nil {
		return false
	}
	wild := safariSes.Battle.WildPokemon
	ses.SendStreamJSON(map[string]interface{}{"pokemon": map[string]interface{}{"id": wild.ID, "name": wild.Name, "level": wild.Level, "hp": wild.CurHP, "maxHp": wild.MaxHP, "spriteId": wild.ID, "catchRate": wild.CatchRate}, "ballsLeft": safariSes.BallsLeft, "stepsLeft": safariSes.StepsLeft}, opcodes.SafariBattleStartNotify)
	return true
}

func safariStorageError(ses *session.Session, charID int64, err error, opcode opcodes.OpCode) {
	log.Printf("[Safari] Character %d: %v", charID, err)
	ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Safari state could not be saved. Please try again.", "message": "Safari state is unavailable. Please try again."}, opcode)
}
func refreshSafariFlags(wh *WorldHandler, charID int64) {
	if wh != nil && wh.EventFlags != nil {
		if err := wh.EventFlags.LoadFlags(charID); err != nil {
			log.Printf("[Safari] Refresh flags for %d: %v", charID, err)
		}
	}
}

func publishSafariExpiry(ses *session.Session, wh *WorldHandler, charID int64) {
	refreshSafariFlags(wh, charID)
	applyServerTeleportedPlayerPosition(ses, wh, SafariZoneGateMapID, SafariZoneGateReturnX, SafariZoneGateReturnY, "DOWN", false)
}
