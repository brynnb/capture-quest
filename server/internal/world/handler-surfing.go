package world

import (
	"log"

	"capturequest/internal/api/opcodes"
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
	SurfIdentity    `tstype:",extends"`
	CommandRevision *int64 `json:"commandRevision" tstype:"number,required"`
	FromX           *int   `json:"fromX" tstype:"number,required"`
	FromY           *int   `json:"fromY" tstype:"number,required"`
	TargetX         *int   `json:"targetX" tstype:"number,required"`
	TargetY         *int   `json:"targetY" tstype:"number,required"`
	MapID           *int   `json:"mapId" tstype:"number,required"`
	Direction       string `json:"direction,omitempty"`
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
	if err := decodePlayerMovement(payload, &req); err != nil || req.TargetX == nil || req.TargetY == nil || req.FromX == nil || req.FromY == nil || req.MapID == nil || req.CommandRevision == nil || req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != int64(ses.Client.CharData().ID) {
		reject("Select adjacent water to SURF.")
		return false
	}
	result, outcome, replay, err := wh.PlayerMovement.SurfTo(ses.CommandContext(), ses, req)
	if err != nil {
		log.Printf("[Surf] Commit for %d: %v", req.CharacterID, err)
		reject("SURF state changed or is unavailable. Recover before trying again.")
		return false
	}
	ses.SendStreamJSON(outcome, opcodes.PokeSurfingResponse)
	if !replay {
		publishMovementStepEffects(ses, wh, req.CharacterID, result)
	}
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
