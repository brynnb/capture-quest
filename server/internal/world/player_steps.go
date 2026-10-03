package world

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/logutil"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

// One outstanding step belongs to the movement registration, not a transport
// token supplied by the client. Re-registration and teleports discard it.
type issuedPlayerStep struct {
	token                   string
	sourceX, sourceY, mapID int
	x, y                    int
	direction               string
	ledgeJump               bool
	issuedAt, completeAfter time.Time
}

func decodePlayerMovement(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("trailing movement request")
	}
	return nil
}

func sendPlayerStepError(ses *session.Session, wh *WorldHandler, requestID string, opcode opcodes.OpCode) {
	x, y, mapID := wh.ownedPlayerPosition(ses)
	direction := "DOWN"
	if wh.PlayerMovement != nil {
		if facing, ok := wh.PlayerMovement.GetDirection(int(ses.Client.CharData().ID)); ok {
			direction = facing
		}
	}
	ses.SendStreamJSON(protocol.PlayerStepError{RequestID: requestID, Error: "Movement was not accepted. Please try again.", MapID: mapID, X: x, Y: y, Direction: direction}, opcode)
}

func HandlePlayerStepRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PlayerStepRequest
	err := decodePlayerMovement(payload, &req)
	if err != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.FromX == nil || req.FromY == nil || wh.PlayerMovement == nil {
		sendPlayerStepError(ses, wh, req.RequestID, opcodes.PlayerStepResponse)
		return false
	}
	step, err := wh.PlayerMovement.issuePlayerStep(ses, req)
	if err != nil {
		logutil.Debugf("[PlayerMovement] Reject intent %s: %v", req.RequestID, err)
		sendPlayerStepError(ses, wh, req.RequestID, opcodes.PlayerStepResponse)
		return false
	}
	ses.SendStreamJSON(protocol.PlayerStepResponse{Success: true, RequestID: req.RequestID, StepToken: step.token, MapID: step.mapID, X: step.x, Y: step.y, Direction: step.direction, LedgeJump: step.ledgeJump}, opcodes.PlayerStepResponse)
	return false
}

// Called under the selected character's session command gate. Collision snapshots
// use the same overlays and NPC/boulder blockers as existing character pathfinding.
func (m *PlayerMovementManager) issuePlayerStep(ses *session.Session, req protocol.PlayerStepRequest) (*issuedPlayerStep, error) {
	charID := int(ses.Client.CharData().ID)
	direction := normalizeWarpDirection(req.Direction)
	if direction == "" || getBattle(int64(charID)) != nil {
		return nil, fmt.Errorf("movement is unavailable")
	}
	m.mu.RLock()
	state := m.players[charID]
	if state == nil {
		m.mu.RUnlock()
		return nil, fmt.Errorf("movement registration is absent")
	}
	if state.SessionID != ses.SessionID || state.MapID != req.MapID || state.CurrentX != *req.FromX || state.CurrentY != *req.FromY || len(state.Path) != 0 {
		err := fmt.Errorf("owned source session=%d map=%d x=%d y=%d path=%d; requested session=%d map=%d x=%d y=%d", state.SessionID, state.MapID, state.CurrentX, state.CurrentY, len(state.Path), ses.SessionID, req.MapID, *req.FromX, *req.FromY)
		m.mu.RUnlock()
		return nil, err
	}
	x, y, mapID, surfing, speed := state.CurrentX, state.CurrentY, state.MapID, state.IsSurfing, state.MoveSpeed
	pending := state.pendingStep
	m.mu.RUnlock()
	if m.actorManager == nil {
		return nil, fmt.Errorf("collision service is unavailable")
	}
	collision, raw, err := m.actorManager.characterCollision(ses.CommandContext(), m.wh.database, int64(charID), mapID, x, y, m.wh.EventFlags)
	if err != nil {
		return nil, err
	}
	dx, dy := 0, 0
	switch direction {
	case "UP":
		dy = -1
	case "DOWN":
		dy = 1
	case "LEFT":
		dx = -1
	case "RIGHT":
		dx = 1
	}
	next := PathNode{X: x + dx, Y: y + dy}
	collisionType, exists := collision[tileKey(next.X, next.Y)]
	ledge := false
	if !exists || !isPathableCollision(collisionType, pathfindOptions{AllowWater: surfing}) {
		next, ledge = ledgeLandingNeighbor(collision, raw, x, y, direction, dx, dy)
		if !ledge {
			return nil, fmt.Errorf("step is blocked")
		}
	}
	token := make([]byte, 16)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	now := time.Now()
	step := &issuedPlayerStep{token: hex.EncodeToString(token), sourceX: x, sourceY: y, mapID: mapID, x: next.X, y: next.Y, direction: direction, ledgeJump: ledge, issuedAt: now, completeAfter: now.Add(speed)}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.players[charID] != state || state.CurrentX != x || state.CurrentY != y || state.MapID != mapID || state.pendingStep != pending {
		return nil, fmt.Errorf("movement owner changed")
	}
	state.pendingStep = step
	return step, nil
}

func waitPlayerStep(ctx context.Context, until time.Time) error {
	remaining := time.Until(until)
	if remaining <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(remaining)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func HandlePlayerStepCompleteRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PlayerStepCompleteRequest
	err := decodePlayerMovement(payload, &req)
	fail := func() { sendPlayerStepError(ses, wh, req.RequestID, opcodes.PlayerStepCompleteResponse) }
	if err != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.StepToken == "" || wh.PlayerMovement == nil {
		fail()
		return false
	}
	step, err := wh.PlayerMovement.completePlayerStep(ses, req.StepToken)
	if err != nil {
		logutil.Debugf("[PlayerMovement] Reject completion %s: %v", req.RequestID, err)
		wh.PlayerMovement.rejectPlayerStep(ses, req.StepToken)
		fail()
		return false
	}
	ses.SendStreamJSON(protocol.PlayerStepCompleteResponse{Success: true, RequestID: req.RequestID, MapID: step.mapID, X: step.x, Y: step.y, Direction: step.direction}, opcodes.PlayerStepCompleteResponse)
	// Effects run only for a successfully committed issued step. Duplicate/stale
	// acknowledgements never re-run encounters, Safari counters or script triggers.
	broadcastCommittedPlayerStep(ses, wh, step.x, step.y, step.mapID, step.direction, step.mapID)
	handleClientReportedStepEffects(ses, wh, int64(ses.Client.CharData().ID), step.x, step.y, step.mapID, step.direction, false, step.mapID)
	return false
}

func (m *PlayerMovementManager) completePlayerStep(ses *session.Session, token string) (*issuedPlayerStep, error) {
	charID := int(ses.Client.CharData().ID)
	m.mu.RLock()
	state := m.players[charID]
	var step *issuedPlayerStep
	if state != nil && state.SessionID == ses.SessionID {
		step = state.pendingStep
	}
	m.mu.RUnlock()
	if step == nil || step.token != token || time.Since(step.issuedAt) > 10*time.Second || getBattle(int64(charID)) != nil {
		return nil, fmt.Errorf("step is stale")
	}
	if err := waitPlayerStep(ses.CommandContext(), step.completeAfter); err != nil {
		return nil, err
	}
	// Revalidate dynamic collision at completion: an NPC or puzzle can change while
	// the browser animates. Use the same rule as acceptance, never trust its target.
	m.mu.Lock()
	if m.players[charID] != state || state.pendingStep != step || state.CurrentX != step.sourceX || state.CurrentY != step.sourceY || state.MapID != step.mapID {
		m.mu.Unlock()
		return nil, fmt.Errorf("step owner changed")
	}
	// Temporarily leave issuance unchanged; collision is checked without holding the
	// shared player lock or replacing the issued token.
	surfing := state.IsSurfing
	m.mu.Unlock()
	collision, raw, err := m.actorManager.characterCollision(ses.CommandContext(), m.wh.database, int64(charID), step.mapID, step.sourceX, step.sourceY, m.wh.EventFlags)
	if err != nil {
		return nil, err
	}
	if step.ledgeJump {
		dx, dy := (step.x-step.sourceX)/2, (step.y-step.sourceY)/2
		landing, ok := ledgeLandingNeighbor(collision, raw, step.sourceX, step.sourceY, step.direction, dx, dy)
		if !ok || landing.X != step.x || landing.Y != step.y {
			return nil, fmt.Errorf("ledge changed")
		}
	} else if value, exists := collision[tileKey(step.x, step.y)]; !exists || !isPathableCollision(value, pathfindOptions{AllowWater: surfing}) {
		return nil, fmt.Errorf("step became blocked")
	}
	if err := commitClientPlayerPosition(ses.CommandContext(), m.wh.database, int64(charID), step.mapID, step.x, step.y); err != nil {
		return nil, err
	}
	// The owning session gate excludes command/tick position writers across commit.
	m.UpdateReportedPosition(charID, step.x, step.y, step.mapID, step.direction)
	publishCommittedPlayerLocation(ses, m.wh, step.mapID, step.x, step.y)
	return step, nil
}

func (m *PlayerMovementManager) rejectPlayerStep(ses *session.Session, token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.players[int(ses.Client.CharData().ID)]
	if state != nil && state.SessionID == ses.SessionID && state.pendingStep != nil && state.pendingStep.token == token {
		state.pendingStep = nil
	}
}

// Facing shares the character command gate, but never persists client coordinates
// or runs completed-step effects. The legacy boulder interaction remains attached
// to a facing attempt until its field-command migration.
func HandlePlayerFacingRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PlayerFacingRequest
	if decodePlayerMovement(payload, &req) != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.FromX == nil || req.FromY == nil || wh.PlayerMovement == nil {
		sendPlayerStepError(ses, wh, req.RequestID, opcodes.PlayerFacingResponse)
		return false
	}
	facing, err := wh.PlayerMovement.facePlayer(ses, req)
	if err != nil {
		logutil.Debugf("[PlayerMovement] Reject facing %s: %v", req.RequestID, err)
		sendPlayerStepError(ses, wh, req.RequestID, opcodes.PlayerFacingResponse)
		return false
	}
	ses.SendStreamJSON(protocol.PlayerFacingResponse{Success: true, RequestID: req.RequestID, MapID: facing.mapID, X: facing.x, Y: facing.y, Direction: facing.direction}, opcodes.PlayerFacingResponse)
	broadcastCommittedPlayerStep(ses, wh, facing.x, facing.y, facing.mapID, facing.direction, facing.mapID)
	return false
}

type ownedPlayerFacing struct {
	x, y, mapID int
	direction   string
}

func (m *PlayerMovementManager) facePlayer(ses *session.Session, req protocol.PlayerFacingRequest) (ownedPlayerFacing, error) {
	direction := normalizeWarpDirection(req.Direction)
	if direction == "" || getBattle(int64(ses.Client.CharData().ID)) != nil {
		return ownedPlayerFacing{}, fmt.Errorf("facing is unavailable")
	}
	if err := ses.CommandContext().Err(); err != nil {
		return ownedPlayerFacing{}, err
	}
	charID := int(ses.Client.CharData().ID)
	m.mu.Lock()
	state := m.players[charID]
	if state == nil || state.SessionID != ses.SessionID || state.MapID != req.MapID || state.CurrentX != *req.FromX || state.CurrentY != *req.FromY || len(state.Path) != 0 || (state.pendingStep != nil && time.Since(state.pendingStep.issuedAt) < 10*time.Second) {
		m.mu.Unlock()
		return ownedPlayerFacing{}, fmt.Errorf("facing source is stale or moving")
	}
	state.pendingStep = nil // Expired acceptance cannot later complete after a turn.
	state.Direction = direction
	x, y, mapID := state.CurrentX, state.CurrentY, state.MapID
	m.mu.Unlock()
	if result, attempted := m.tryPushBoulderFromFacingAttempt(charID, mapID, x, y, direction); attempted && result.Success {
		m.queueStepAfterBoulderPush(charID, x, y, mapID, result)
	}
	return ownedPlayerFacing{x: x, y: y, mapID: mapID, direction: direction}, nil
}
