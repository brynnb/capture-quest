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

// A bounded chain of issued steps belongs to the movement registration. Tokens
// authorize presentation only: each step still commits separately, in order,
// after collision/effect validation. Re-registration and teleports discard it.
type issuedPlayerStep struct {
	token                   string
	parentToken             string // a prepared continuation belongs to its prior token
	next                    *issuedPlayerStep
	retireOnParentReplay    bool
	sourceX, sourceY, mapID int
	x, y                    int
	direction               string
	ledgeJump               bool
	issuedAt, completeAfter time.Time
	effects                 movementStepResult
	sourceMapID             int
}

const playerStepLifetime = 10 * time.Second
const playerStepWindow = 8

func playerStepDistance(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// Call with the movement manager write lock after checking session ownership
// and any caller-provided source. Consumers retire expired authorization
// at the same edge.
func (state *PlayerMovementState) activePlayerStep(now time.Time) *issuedPlayerStep {
	if state.pendingStep != nil && now.Sub(state.pendingStep.issuedAt) >= playerStepLifetime {
		state.pendingStep = nil
	}
	return state.pendingStep
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

func (wh *WorldHandler) ownedPlayerSnapshot(ses *session.Session, requestID string) protocol.OwnedPlayerPositionResponse {
	char := ses.Client.CharData()
	p := protocol.OwnedPlayerPositionResponse{Success: true, RequestID: requestID, MapID: int(char.MapID), X: int(char.X), Y: int(char.Y), Direction: "DOWN"}
	if m := wh.PlayerMovement; m != nil {
		m.mu.RLock()
		if state := m.players[int(char.ID)]; state != nil && state.SessionID == ses.SessionID {
			p.MapID, p.X, p.Y, p.Direction = state.MapID, state.CurrentX, state.CurrentY, state.Direction
			p.ServerMovementPending = len(state.Path) != 0
			if state.movementStreamID != "" {
				p.MovementStreamID, p.MovementSequence = state.movementStreamID, state.movementSequence
			}
		}
		m.mu.RUnlock()
	}
	return p
}

func sendOwnedPlayerError(ses *session.Session, wh *WorldHandler, requestID string, opcode opcodes.OpCode, message string) {
	p := wh.ownedPlayerSnapshot(ses, requestID)
	ses.SendStreamJSON(protocol.PlayerStepError{RequestID: requestID, Error: message, MapID: p.MapID, X: p.X, Y: p.Y, Direction: p.Direction, ServerMovementPending: p.ServerMovementPending}, opcode)
}
func sendPlayerStepError(ses *session.Session, wh *WorldHandler, requestID string, opcode opcodes.OpCode) {
	sendOwnedPlayerError(ses, wh, requestID, opcode, "Movement was not accepted. Please try again.")
}

// Read under the selected character's command gate after preceding mutations.
// No arrival effects, position writes or client destination are part of recovery.
func HandleOwnedPlayerPositionRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.OwnedPlayerPositionRequest
	if decodePlayerMovement(payload, &req) != nil || req.RequestID == "" || len(req.RequestID) > 64 || (req.StepToken != "" && !validMovementToken(req.StepToken)) {
		sendOwnedPlayerError(ses, wh, req.RequestID, opcodes.OwnedPlayerPositionResponse, "Invalid position read.")
		return false
	}
	if req.ResetMovementStream {
		if wh.PlayerMovement == nil || wh.PlayerMovement.resetPlayerMovementStream(ses) != nil {
			sendOwnedPlayerError(ses, wh, req.RequestID, opcodes.OwnedPlayerPositionResponse, "Could not initialize movement.")
			return false
		}
	}
	snapshot := wh.ownedPlayerSnapshot(ses, req.RequestID)
	if req.StepToken != "" {
		receipt, err := loadMovementReceipt(ses.CommandContext(), wh.database, int64(ses.Client.CharData().ID), req.StepToken)
		if err != nil {
			logutil.Debugf("[PlayerMovement] Read receipt: %v", err)
			sendOwnedPlayerError(ses, wh, req.RequestID, opcodes.OwnedPlayerPositionResponse, "Could not read movement result.")
			return false
		}
		snapshot.CommittedStep = receipt
	}
	ses.SendStreamJSON(snapshot, opcodes.OwnedPlayerPositionResponse)
	trainerResumed := false
	if wh.TrainerEncounter != nil {
		var err error
		trainerResumed, err = wh.TrainerEncounter.resumePendingEncounter(ses, wh)
		if err != nil {
			logutil.Debugf("[TrainerEncounter] Recovery: %v", err)
		}
	}
	if !trainerResumed {
		if _, err := resumePendingCutscene(ses, wh); err != nil {
			logutil.Debugf("[Cutscene] Recovery: %v", err)
		}
	}
	return false
}

func HandlePlayerStepRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PlayerStepRequest
	err := decodePlayerMovement(payload, &req)
	if err == nil && (req.CommandToken != "" || req.MovementStreamID != "" || req.Sequence != 0) {
		return handlePlayerMoveIntent(ses, req, wh)
	}
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
	// Only an accepted legacy admission replaces an optimistic generation.
	wh.PlayerMovement.retireOwnedMovementStream(ses)
	response := playerStepResponse(step, req.RequestID)
	response.Buffered = true
	for next := step.next; next != nil; next = next.next {
		response.ContinuationSteps = append(response.ContinuationSteps, playerStepResponse(next, req.RequestID))
	}
	ses.SendStreamJSON(response, opcodes.PlayerStepResponse)
	return false
}

// Called under the selected character's session command gate. Collision snapshots
// use the same overlays and NPC/boulder blockers as existing character pathfinding.
func (m *PlayerMovementManager) issuePlayerStep(ses *session.Session, req protocol.PlayerStepRequest) (*issuedPlayerStep, error) {
	charID := int(ses.Client.CharData().ID)
	direction := normalizeWarpDirection(req.Direction)
	if direction == "" || len(req.LookAhead) >= playerStepWindow || getBattle(int64(charID)) != nil {
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
	var trainerPending bool
	if err := m.wh.database.QueryRowContext(ses.CommandContext(), `SELECT EXISTS(SELECT 1 FROM character_trainer_encounters WHERE character_id=$1 AND resolution='pending') OR EXISTS(SELECT 1 FROM character_cutscene_plans WHERE character_id=$1 AND resolution='pending')`, charID).Scan(&trainerPending); err != nil {
		return nil, err
	}
	if trainerPending {
		return nil, fmt.Errorf("gameplay presentation is pending")
	}
	if m.actorManager == nil {
		return nil, fmt.Errorf("collision service is unavailable")
	}
	collision, raw, err := m.actorManager.characterCollision(ses.CommandContext(), m.wh.database, int64(charID), mapID, x, y, m.wh.EventFlags)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	step, err := buildPlayerStepChain(collision, raw, x, y, mapID, surfing, append([]string{direction}, req.LookAhead...), now, now.Add(speed), speed)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.players[charID] != state || state.CurrentX != x || state.CurrentY != y || state.MapID != mapID || state.pendingStep != pending {
		return nil, fmt.Errorf("movement owner changed")
	}
	state.pendingStep = step
	return step, nil
}

// Responses contain a flat window; recursive pointers never enter the protocol.
func playerStepResponse(step *issuedPlayerStep, requestID string) protocol.PlayerStepResponse {
	return protocol.PlayerStepResponse{Success: true, RequestID: requestID, StepToken: step.token, MapID: step.mapID, X: step.x, Y: step.y, Direction: step.direction, LedgeJump: step.ledgeJump}
}

// Validate each proposed tile with server collision. A blocked future tile ends
// the window; it cannot undo the usable prefix or authorize movement through it.
func buildPlayerStepChain(collision map[string]int, raw map[string]int, x, y, mapID int, surfing bool, directions []string, issuedAt, deadline time.Time, speed time.Duration) (*issuedPlayerStep, error) {
	var head, tail *issuedPlayerStep
	for _, proposed := range directions {
		direction := normalizeWarpDirection(proposed)
		if direction == "" {
			return nil, fmt.Errorf("invalid movement direction")
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
		value, exists := collision[tileKey(next.X, next.Y)]
		ledge := false
		if !exists || !isPathableCollision(value, pathfindOptions{AllowWater: surfing}) {
			next, ledge = ledgeLandingNeighbor(collision, raw, x, y, direction, dx, dy)
			if !ledge {
				if head == nil {
					return nil, fmt.Errorf("step is blocked")
				}
				break
			}
		}
		token := make([]byte, 16)
		if _, err := rand.Read(token); err != nil {
			return nil, err
		}
		if ledge {
			deadline = deadline.Add(speed)
		}
		step := &issuedPlayerStep{token: hex.EncodeToString(token), sourceX: x, sourceY: y, mapID: mapID, x: next.X, y: next.Y, direction: direction, ledgeJump: ledge, issuedAt: issuedAt, completeAfter: deadline}
		if tail == nil {
			head = step
		} else {
			tail.next = step
			step.parentToken = tail.token
		}
		tail = step
		x, y = step.x, step.y
		deadline = deadline.Add(speed)
	}
	return head, nil
}

// Append only to the exact tail observed by the browser. A late refill cannot
// replace a retargeted route, and the total pending window remains bounded.
func (m *PlayerMovementManager) appendPlayerSteps(ses *session.Session, completed *issuedPlayerStep, afterToken string, directions []string) []protocol.PlayerStepResponse {
	if len(directions) == 0 || len(directions) >= playerStepWindow {
		return nil
	}
	charID := int(ses.Client.CharData().ID)
	m.mu.RLock()
	state := m.players[charID]
	if state == nil || state.SessionID != ses.SessionID {
		m.mu.RUnlock()
		return nil
	}
	tail, count := state.pendingStep, 0
	for next := tail; next != nil; next = next.next {
		tail = next
		count++
	}
	base := tail
	if base == nil {
		base = completed
	}
	if base.token != afterToken || count >= playerStepWindow {
		m.mu.RUnlock()
		return nil
	}
	surfing, speed := state.IsSurfing, state.MoveSpeed
	m.mu.RUnlock()
	if len(directions) > playerStepWindow-count {
		directions = directions[:playerStepWindow-count]
	}
	collision, raw, err := m.actorManager.characterCollision(ses.CommandContext(), m.wh.database, int64(charID), base.mapID, base.x, base.y, m.wh.EventFlags)
	if err != nil {
		return nil
	}
	first, err := buildPlayerStepChain(collision, raw, base.x, base.y, base.mapID, surfing, directions, time.Now(), base.completeAfter.Add(speed), speed)
	if err != nil || first == nil {
		return nil
	}
	first.parentToken = base.token
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.players[charID] != state || (tail == nil && state.pendingStep != nil) || (tail != nil && tail.next != nil) {
		return nil
	}
	if tail == nil {
		state.pendingStep = first
	} else {
		tail.next = first
	}
	var responses []protocol.PlayerStepResponse
	for next := first; next != nil; next = next.next {
		responses = append(responses, playerStepResponse(next, ""))
	}
	return responses
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
	if err != nil || req.RequestID == "" || len(req.RequestID) > 64 || !validMovementToken(req.StepToken) || wh.PlayerMovement == nil ||
		(req.NextDirection != "" && normalizeWarpDirection(req.NextDirection) == "") || len(req.LookAhead) >= playerStepWindow || (len(req.LookAhead) != 0 && !validMovementToken(req.AfterStepToken)) || (req.Cancel && (req.NextDirection != "" || len(req.LookAhead) != 0)) {
		fail()
		return false
	}
	if req.Cancel {
		// Idempotent retirement of one exact authorization. A late cancellation
		// cannot consume a replacement token or undo an already committed step.
		wh.PlayerMovement.rejectPlayerStep(ses, req.StepToken)
		owned := wh.ownedPlayerSnapshot(ses, req.RequestID)
		ses.SendStreamJSON(protocol.PlayerStepCompleteResponse{Success: true, Cancelled: true, RequestID: req.RequestID, MapID: owned.MapID, X: owned.X, Y: owned.Y, Direction: owned.Direction}, opcodes.PlayerStepCompleteResponse)
		return false
	}
	receipt, err := loadMovementReceipt(ses.CommandContext(), wh.database, int64(ses.Client.CharData().ID), req.StepToken)
	if err != nil {
		logutil.Debugf("[PlayerMovement] Read completion receipt: %v", err)
		fail()
		return false
	}
	if receipt != nil {
		// A lost publication may have prepared an unobserved continuation. Retire
		// only that receipt's child, never a newer independently issued token.
		wh.PlayerMovement.retirePreparedContinuation(ses, req.StepToken)
		// Historical success acknowledges the lost reply only. Never rewind live
		// state, republish triggers, or replay counters/battles on a duplicate.
		ses.SendStreamJSON(protocol.PlayerStepCompleteResponse{Success: true, RequestID: req.RequestID, MapID: receipt.MapID, X: receipt.X, Y: receipt.Y, Direction: receipt.Direction, Replayed: true}, opcodes.PlayerStepCompleteResponse)
		return false
	}
	step, err := wh.PlayerMovement.completePlayerStep(ses, req.StepToken)
	if err != nil {
		logutil.Debugf("[PlayerMovement] Reject completion %s: %v", req.RequestID, err)
		wh.PlayerMovement.rejectPlayerStep(ses, req.StepToken)
		fail()
		return false
	}
	response := protocol.PlayerStepCompleteResponse{Success: true, RequestID: req.RequestID, MapID: step.mapID, X: step.x, Y: step.y, Direction: step.direction, StopPath: !allowsPlayerStepContinuation(step.effects)}
	if req.NextDirection != "" && !response.StopPath {
		// The previous move is already committed. An unavailable continuation is
		// not a failure of that commit; ordinary input can revalidate next time.
		next, nextErr := wh.PlayerMovement.issuePlayerStep(ses, protocol.PlayerStepRequest{MapID: step.mapID, FromX: &step.x, FromY: &step.y, Direction: req.NextDirection, RequestID: req.RequestID})
		if nextErr == nil {
			wh.PlayerMovement.mu.Lock()
			next.parentToken = req.StepToken
			next.retireOnParentReplay = true
			wh.PlayerMovement.mu.Unlock()
			response.NextStep = &protocol.PlayerStepResponse{Success: true, RequestID: req.RequestID, StepToken: next.token, MapID: next.mapID, X: next.x, Y: next.y, Direction: next.direction, LedgeJump: next.ledgeJump}
		}
	}
	if !response.StopPath {
		if len(req.LookAhead) != 0 {
			wh.PlayerMovement.appendPlayerSteps(ses, step, req.AfterStepToken, req.LookAhead)
		}
		// Republish the bounded pending window with every receipt. A later child
		// response repairs a lost refill reply without replaying an old commit.
		wh.PlayerMovement.mu.RLock()
		state := wh.PlayerMovement.players[int(ses.Client.CharData().ID)]
		if state != nil && state.SessionID == ses.SessionID {
			for next := state.pendingStep; next != nil; next = next.next {
				response.ContinuationSteps = append(response.ContinuationSteps, playerStepResponse(next, req.RequestID))
			}
		}
		wh.PlayerMovement.mu.RUnlock()
	}
	ses.SendStreamJSON(response, opcodes.PlayerStepCompleteResponse)
	publishCompletedPlayerStep(ses, wh, step)
	return false
}

func publishCompletedPlayerStep(ses *session.Session, wh *WorldHandler, step *issuedPlayerStep) {
	// Effects run only for a successfully committed issued step. Duplicate/stale
	// acknowledgements never re-run encounters, Safari counters or script triggers.
	broadcastCommittedPlayerStep(ses, wh, step.x, step.y, step.mapID, step.direction, step.sourceMapID)
	publishMovementStepEffects(ses, wh, int64(ses.Client.CharData().ID), step.effects)
	if len(step.effects.ForcedPath) > 0 {
		// The committed source point starts the server route; this notification
		// retires the client's future user path before its next prediction.
		wh.PlayerMovement.mu.RLock()
		state := wh.PlayerMovement.players[int(ses.Client.CharData().ID)]
		if state != nil && state.SessionID == ses.SessionID && len(state.Path) > 0 {
			snapshot := wh.PlayerMovement.snapshotForState(state, 0)
			wh.PlayerMovement.mu.RUnlock()
			wh.PlayerMovement.broadcastSnapshot(snapshot, true)
		} else {
			wh.PlayerMovement.mu.RUnlock()
		}
	}
}

func (m *PlayerMovementManager) completePlayerStep(ses *session.Session, token string) (*issuedPlayerStep, error) {
	charID := int(ses.Client.CharData().ID)
	m.mu.Lock()
	state := m.players[charID]
	var step *issuedPlayerStep
	if state != nil && state.SessionID == ses.SessionID {
		step = state.activePlayerStep(time.Now())
	}
	m.mu.Unlock()
	if step == nil || step.token != token || getBattle(int64(charID)) != nil {
		return nil, fmt.Errorf("step is stale")
	}
	if err := waitPlayerStep(ses.CommandContext(), step.completeAfter); err != nil {
		return nil, err
	}
	startedAt := time.Now()
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
	effects, err := commitMovementStep(ses.CommandContext(), m.wh, int64(charID), movementStepCandidate{StepToken: token, Surfing: surfing, SourceMap: step.mapID, SourceX: step.sourceX, SourceY: step.sourceY, MapID: step.mapID, X: step.x, Y: step.y, Direction: step.direction})
	if err != nil {
		return nil, err
	}
	completed := *step
	// An empty-window refill starts from the actual paced commit, not an old
	// issuance deadline that may already be behind after transport delay.
	completed.completeAfter = startedAt
	completed.effects = effects
	completed.sourceMapID = step.mapID
	completed.mapID, completed.x, completed.y, completed.direction = effects.MapID, effects.X, effects.Y, effects.Direction
	step = &completed
	// Consume the issued token even when recovery returns to the same source tile.
	// The owning session gate excludes command/tick position writers across commit.
	m.projectCommittedPosition(charID, step.x, step.y, step.mapID, step.direction, effects.Surfing)
	m.mu.Lock()
	if m.players[charID] == state && state.SessionID == ses.SessionID && allowsPlayerStepContinuation(effects) {
		state.pendingStep = completed.next
		if state.pendingStep != nil {
			state.pendingStep.completeAfter = startedAt.Add(time.Duration(playerStepDistance(state.pendingStep.x-state.pendingStep.sourceX)+playerStepDistance(state.pendingStep.y-state.pendingStep.sourceY)) * state.MoveSpeed)
		}
	}
	m.mu.Unlock()
	publishCommittedPlayerLocation(ses, m.wh, step.mapID, step.x, step.y)
	if len(effects.ForcedPath) > 0 {
		m.mu.Lock()
		if current := m.players[charID]; current == state && current.SessionID == ses.SessionID {
			current.Path = append([]PathNode(nil), effects.ForcedPath...)
			current.LastMoveTime = time.Now()
		}
		m.mu.Unlock()
	}
	return step, nil
}

func allowsPlayerStepContinuation(result movementStepResult) bool {
	return !result.StopPath && !result.Teleport && len(result.ForcedPath) == 0 && result.Trainer == nil && result.Cutscene == nil && result.Wild.Battle == nil
}

func (m *PlayerMovementManager) retirePreparedContinuation(ses *session.Session, parentToken string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.players[int(ses.Client.CharData().ID)]
	if state != nil && state.SessionID == ses.SessionID && state.pendingStep != nil && state.pendingStep.parentToken == parentToken && state.pendingStep.retireOnParentReplay {
		state.pendingStep = nil
	}
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
	ses.SendStreamJSON(protocol.PlayerFacingResponse{Success: true, RequestID: req.RequestID, MapID: facing.mapID, X: facing.x, Y: facing.y, Direction: facing.direction, ServerMovementPending: facing.serverMovementPending}, opcodes.PlayerFacingResponse)
	broadcastCommittedPlayerStep(ses, wh, facing.x, facing.y, facing.mapID, facing.direction, facing.mapID)
	return false
}

type ownedPlayerFacing struct {
	serverMovementPending bool
	x, y, mapID           int
	direction             string
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
	if state == nil || state.SessionID != ses.SessionID || state.MapID != req.MapID || state.CurrentX != *req.FromX || state.CurrentY != *req.FromY || len(state.Path) != 0 || state.activePlayerStep(time.Now()) != nil {
		m.mu.Unlock()
		return ownedPlayerFacing{}, fmt.Errorf("facing source is stale or moving")
	}
	state.Direction = direction
	x, y, mapID := state.CurrentX, state.CurrentY, state.MapID
	m.mu.Unlock()
	result, attempted, err := m.tryPushBoulderFromFacingAttempt(ses, charID, mapID, x, y, direction)
	if err != nil {
		return ownedPlayerFacing{}, err
	}
	if attempted && result.Success {
		m.queueStepAfterBoulderPush(charID, x, y, mapID, result)
	}
	m.mu.RLock()
	pending := m.players[charID] == state && len(state.Path) != 0
	m.mu.RUnlock()
	return ownedPlayerFacing{x: x, y: y, mapID: mapID, direction: direction, serverMovementPending: pending}, nil
}
