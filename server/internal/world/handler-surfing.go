package world

import (
	"log"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/logutil"
	"capturequest/internal/session"
)

type PokeSurfingRequestPayload struct {
	TargetX   *int   `json:"targetX,omitempty"`
	TargetY   *int   `json:"targetY,omitempty"`
	MapID     *int   `json:"mapId,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// SURF entry delegates gameplay permission and effects to the movement owner.
// Targetless encounter generation is retired; ordinary water steps already run
// the authoritative movement encounter policy.
func HandlePokeSurfing(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PokeSurfingRequestPayload
	reject := func(message string) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": message}, opcodes.PokeSurfingResponse)
	}
	if !ses.HasValidClient() || wh == nil || wh.PlayerMovement == nil {
		reject("SURF movement owner unavailable.")
		return false
	}
	if err := decodePlayerMovement(payload, &req); err != nil || req.TargetX == nil || req.TargetY == nil {
		reject("Select adjacent water to SURF.")
		return false
	}
	source := wh.ownedPlayerSnapshot(ses, "")
	return handlePokeSurfingTarget(ses, wh, int64(ses.Client.CharData().ID), source.MapID, source.X, source.Y, req)
}

func handlePokeSurfingTarget(
	ses *session.Session,
	wh *WorldHandler,
	charID int64,
	currentMapID int,
	playerX int,
	playerY int,
	req PokeSurfingRequestPayload,
) bool {
	if wh == nil || wh.ActorManager == nil || wh.PlayerMovement == nil {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You can't SURF here.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	targetMapID := currentMapID
	if req.MapID != nil {
		targetMapID = *req.MapID
	}
	if wh.ActorManager.IsOverworld(targetMapID) {
		targetMapID = UnifiedOverworldMapID
	}
	if targetMapID != currentMapID {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You can't SURF here.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	targetX, targetY := *req.TargetX, *req.TargetY
	if abs(targetX-playerX)+abs(targetY-playerY) != 1 {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You need to be next to the water.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	surfable, err := isSurfableWaterTile(ses.CommandContext(), wh, targetMapID, targetX, targetY)
	if err != nil {
		logutil.Debugf("[Surfing] Water read map=%d target=(%d,%d): %v", targetMapID, targetX, targetY, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Unable to read water."}, opcodes.PokeSurfingResponse)
		return false
	}
	if !surfable {
		ses.SendStreamJSON(map[string]interface{}{
			"success": false,
			"error":   "You can't SURF here.",
		}, opcodes.PokeSurfingResponse)
		return false
	}

	direction := normalizeWarpDirection(req.Direction)
	expectedDirection := directionFromAdjacentTiles(playerX, playerY, targetX, targetY)
	if strings.TrimSpace(req.Direction) != "" && (direction == "" || direction != expectedDirection) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "SURF facing disagrees with target."}, opcodes.PokeSurfingResponse)
		return false
	}
	direction = expectedDirection

	result, err := wh.PlayerMovement.SurfTo(ses.CommandContext(), ses, targetX, targetY, targetMapID, direction)
	if err != nil {
		log.Printf("[Surf] Commit for %d: %v", charID, err)
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "You can't SURF here."}, opcodes.PokeSurfingResponse)
		return false
	}
	encounter := result.Wild.Battle != nil || result.Wild.Blackout != nil

	ses.SendStreamJSON(map[string]interface{}{
		"success":   true,
		"encounter": encounter,
		"blackout":  result.Wild.Blackout != nil,
		"message":   "You're surfing!",
		"mapId":     targetMapID,
		"x":         targetX,
		"y":         targetY,
		"direction": direction,
	}, opcodes.PokeSurfingResponse)
	publishMovementStepEffects(ses, wh, charID, result)
	return false
}

func directionFromAdjacentTiles(fromX, fromY, toX, toY int) string {
	switch {
	case toY < fromY:
		return "UP"
	case toY > fromY:
		return "DOWN"
	case toX < fromX:
		return "LEFT"
	case toX > fromX:
		return "RIGHT"
	default:
		return "DOWN"
	}
}
