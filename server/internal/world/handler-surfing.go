package world

import (
	"log"
	"strings"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/logutil"
	"capturequest/internal/session"
)

type SurfIdentity struct {
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
}
type SurfResponse struct {
	SurfIdentity `tstype:",extends"`
	Success      bool `json:"success" tstype:"true"`
	Encounter    bool `json:"encounter"`
	Blackout     bool `json:"blackout"`
}
type SurfError struct {
	SurfIdentity `tstype:",extends"`
	Success      bool   `json:"success" tstype:"false"`
	Error        string `json:"error"`
}
type PokeSurfingRequestPayload struct {
	SurfIdentity `tstype:",extends"`
	FromX        *int   `json:"fromX" tstype:"number,required"`
	FromY        *int   `json:"fromY" tstype:"number,required"`
	TargetX      *int   `json:"targetX" tstype:"number,required"`
	TargetY      *int   `json:"targetY" tstype:"number,required"`
	MapID        *int   `json:"mapId" tstype:"number,required"`
	Direction    string `json:"direction,omitempty"`
}

// SURF entry delegates gameplay permission and effects to the movement owner.
// Targetless encounter generation is retired; ordinary water steps already run
// the authoritative movement encounter policy.
func HandlePokeSurfing(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req PokeSurfingRequestPayload
	reject := func(message string) {
		sendSurfError(ses, req.SurfIdentity, message)
	}
	if !ses.HasValidClient() || wh == nil || wh.PlayerMovement == nil {
		reject("SURF movement owner unavailable.")
		return false
	}
	if err := decodePlayerMovement(payload, &req); err != nil || req.TargetX == nil || req.TargetY == nil || req.FromX == nil || req.FromY == nil || req.MapID == nil || req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != int64(ses.Client.CharData().ID) {
		reject("Select adjacent water to SURF.")
		return false
	}
	source := wh.ownedPlayerSnapshot(ses, "")
	if source.MapID != *req.MapID || source.X != *req.FromX || source.Y != *req.FromY {
		reject("SURF source changed.")
		return false
	}
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
		sendSurfError(ses, req.SurfIdentity, "You can't SURF here.")
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
		sendSurfError(ses, req.SurfIdentity, "You can't SURF here.")
		return false
	}

	targetX, targetY := *req.TargetX, *req.TargetY
	if abs(targetX-playerX)+abs(targetY-playerY) != 1 {
		sendSurfError(ses, req.SurfIdentity, "You need to be next to the water.")
		return false
	}

	surfable, err := isSurfableWaterTile(ses.CommandContext(), wh, targetMapID, targetX, targetY)
	if err != nil {
		logutil.Debugf("[Surfing] Water read map=%d target=(%d,%d): %v", targetMapID, targetX, targetY, err)
		sendSurfError(ses, req.SurfIdentity, "Unable to read water.")
		return false
	}
	if !surfable {
		sendSurfError(ses, req.SurfIdentity, "You can't SURF here.")
		return false
	}

	direction := normalizeWarpDirection(req.Direction)
	expectedDirection := directionFromAdjacentTiles(playerX, playerY, targetX, targetY)
	if strings.TrimSpace(req.Direction) != "" && (direction == "" || direction != expectedDirection) {
		sendSurfError(ses, req.SurfIdentity, "SURF facing disagrees with target.")
		return false
	}
	direction = expectedDirection

	result, err := wh.PlayerMovement.SurfTo(ses.CommandContext(), ses, targetX, targetY, targetMapID, direction)
	if err != nil {
		log.Printf("[Surf] Commit for %d: %v", charID, err)
		sendSurfError(ses, req.SurfIdentity, "You can't SURF here.")
		return false
	}
	encounter := result.Wild.Battle != nil || result.Wild.Blackout != nil

	ses.SendStreamJSON(SurfResponse{SurfIdentity: req.SurfIdentity, Success: true, Encounter: encounter, Blackout: result.Wild.Blackout != nil}, opcodes.PokeSurfingResponse)
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

func sendSurfError(ses *session.Session, identity SurfIdentity, message string) {
	ses.SendStreamJSON(SurfError{SurfIdentity: identity, Error: message}, opcodes.PokeSurfingResponse)
}
