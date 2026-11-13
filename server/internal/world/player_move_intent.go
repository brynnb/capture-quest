package world

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/logutil"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

// A stream belongs to one movement registration. Strict sequence ordering fences
// already-sent predictions after a rejection, teleport, or ownership replacement.
// Tokens correlate durable results; they do not authorize client coordinates.
type playerMovementInput struct {
	mapID, x, y int
	direction   string
}

const maxPlayerMovementSequence = 1<<53 - 1

func (m *PlayerMovementManager) resetPlayerMovementStream(ses *session.Session) error {
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.players[int(ses.Client.CharData().ID)]
	if state == nil || state.SessionID != ses.SessionID {
		return fmt.Errorf("movement registration is absent")
	}
	state.movementStreamID = hex.EncodeToString(token)
	state.movementSequence = 0
	state.movementCommandToken = ""
	state.movementInput = playerMovementInput{}
	state.movementLedgeJump = false
	state.pendingStep = nil
	// Keep the pacing clock and server-owned path. A reset neither moves the
	// player nor permits a faster step or cancellation of a scripted route.
	return nil
}

func (m *PlayerMovementManager) retireOwnedMovementStream(ses *session.Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.players[int(ses.Client.CharData().ID)]
	if state != nil && state.SessionID == ses.SessionID {
		// The accepted legacy admission already installed its own pending step.
		state.movementStreamID = ""
	}
}

func (m *PlayerMovementManager) retirePlayerMovementStream(ses *session.Session, streamID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.players[int(ses.Client.CharData().ID)]
	if state != nil && state.SessionID == ses.SessionID && state.movementStreamID == streamID {
		state.movementStreamID = ""
		state.pendingStep = nil
	}
}

func handlePlayerMoveIntent(ses *session.Session, req protocol.PlayerStepRequest, wh *WorldHandler) bool {
	if wh.PlayerMovement == nil {
		sendPlayerStepError(ses, wh, req.RequestID, opcodes.PlayerStepResponse)
		return false
	}
	step, replayed, err := wh.PlayerMovement.commitPlayerMoveIntent(ses, req)
	if err != nil {
		logutil.Debugf("[PlayerMovement] Reject sequenced input %s: %v", req.RequestID, err)
		sendPlayerStepError(ses, wh, req.RequestID, opcodes.PlayerStepResponse)
		return false
	}
	response := playerStepResponse(step, req.RequestID)
	response.Committed = true
	response.Replayed = replayed
	response.StopPath = !replayed && !allowsPlayerStepContinuation(step.effects)
	ses.SendStreamJSON(response, opcodes.PlayerStepResponse)
	if !replayed {
		publishCompletedPlayerStep(ses, wh, step)
	}
	return false
}

// Called under the selected character command gate on both transports. Each
// locally started tile is one command: validate, pace, persist, then acknowledge.
// The browser can submit later predictions without waiting for a grant or reply.
func (m *PlayerMovementManager) commitPlayerMoveIntent(ses *session.Session, req protocol.PlayerStepRequest) (*issuedPlayerStep, bool, error) {
	if req.RequestID == "" || len(req.RequestID) > 64 || !validMovementToken(req.CommandToken) ||
		!validMovementToken(req.MovementStreamID) || req.Sequence == 0 || req.Sequence > maxPlayerMovementSequence {
		return nil, false, fmt.Errorf("invalid movement command")
	}
	charID := int(ses.Client.CharData().ID)
	m.mu.Lock()
	state := m.players[charID]
	if state == nil || state.SessionID != ses.SessionID || state.movementStreamID != req.MovementStreamID {
		m.mu.Unlock()
		return nil, false, fmt.Errorf("movement stream is stale")
	}
	isNext := req.Sequence == state.movementSequence+1
	direction := normalizeWarpDirection(req.Direction)
	if req.FromX == nil || req.FromY == nil || direction == "" || len(req.LookAhead) != 0 {
		if isNext {
			state.movementStreamID = ""
			state.pendingStep = nil
		}
		m.mu.Unlock()
		return nil, false, fmt.Errorf("invalid movement input")
	}
	input := playerMovementInput{mapID: req.MapID, x: *req.FromX, y: *req.FromY, direction: direction}
	replayed := req.Sequence == state.movementSequence && req.CommandToken == state.movementCommandToken && input == state.movementInput
	ledgeJump := state.movementLedgeJump
	if !isNext && !replayed {
		m.mu.Unlock()
		return nil, false, fmt.Errorf("movement sequence is stale")
	}
	if isNext && req.CommandToken == state.movementCommandToken {
		state.movementStreamID = ""
		state.pendingStep = nil
		m.mu.Unlock()
		return nil, false, fmt.Errorf("movement token was reused")
	}
	m.mu.Unlock()

	if replayed {
		receipt, err := loadMovementReceipt(ses.CommandContext(), m.wh.database, int64(charID), req.CommandToken)
		if err != nil {
			return nil, false, err
		}
		if receipt == nil {
			return nil, false, fmt.Errorf("movement receipt is absent")
		}
		return &issuedPlayerStep{token: receipt.StepToken, mapID: receipt.MapID, x: receipt.X, y: receipt.Y, direction: receipt.Direction, ledgeJump: ledgeJump}, true, nil
	}

	fail := func(err error) (*issuedPlayerStep, bool, error) {
		// Only the verified current stream is retired. Delayed commands from an
		// older stream cannot cancel its replacement, even at the same tile.
		m.retirePlayerMovementStream(ses, req.MovementStreamID)
		return nil, false, err
	}
	step, err := m.issuePlayerStep(ses, req)
	if err != nil {
		return fail(err)
	}
	m.mu.Lock()
	if m.players[charID] != state || state.SessionID != ses.SessionID ||
		state.movementStreamID != req.MovementStreamID || state.pendingStep != step || state.movementSequence+1 != req.Sequence {
		m.mu.Unlock()
		return fail(fmt.Errorf("movement owner changed"))
	}
	step.token = req.CommandToken
	duration := time.Duration(playerStepDistance(step.x-step.sourceX)+playerStepDistance(step.y-step.sourceY)) * state.MoveSpeed
	if state.movementSequence != 0 && !state.movementCommitStartedAt.IsZero() {
		// Anchor to the actual previous commit start. Adding a fresh duration to
		// every admission accumulates database/transport latency on every turn.
		step.completeAfter = state.movementCommitStartedAt.Add(duration)
	} else if next := state.movementCommitStartedAt.Add(duration); next.After(step.completeAfter) {
		step.completeAfter = next
	}
	m.mu.Unlock()

	completed, err := m.completePlayerStep(ses, req.CommandToken)
	if err != nil {
		return fail(err)
	}
	m.mu.Lock()
	if m.players[charID] == state && state.SessionID == ses.SessionID && state.movementStreamID == req.MovementStreamID {
		state.movementSequence = req.Sequence
		state.movementCommandToken = req.CommandToken
		state.movementInput = input
		state.movementLedgeJump = completed.ledgeJump
		state.movementCommitStartedAt = completed.completeAfter
		if !allowsPlayerStepContinuation(completed.effects) {
			state.movementStreamID = ""
			state.pendingStep = nil
		}
	}
	m.mu.Unlock()
	return completed, false, nil
}
