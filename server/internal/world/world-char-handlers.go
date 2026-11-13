package world

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

type EnterWorldRequest struct {
	Name string `json:"name"`
}

type SimpleSuccessResponse struct {
	Value int32 `json:"value"`
}

func HandleEnterWorld(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if ses == nil || ses.IsClosed() {
		return false
	}
	loginStarted := time.Now()
	reject := func(closeOwner bool) bool {
		if !ses.IsClosed() {
			if err := ses.SendStreamJSON(SimpleSuccessResponse{Value: 0}, opcodes.PostEnterWorld); err != nil {
				log.Printf("[WORLD] Session %d: entry rejection failed: %v", ses.SessionID, err)
				closeOwner = true
			}
		}
		if closeOwner {
			// The transport's drain retires any partially installed owner before
			// a replacement session can acquire and load this character.
			ses.Close()
		}
		return false
	}
	if ses.HasValidClient() {
		return reject(false)
	}
	var req EnterWorldRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("failed to unmarshal EnterWorld JSON: %v", err)
		return reject(false)
	}
	if strings.TrimSpace(req.Name) == "" {
		return reject(false)
	}
	name := req.Name
	log.Printf("[WORLD] Session %d entering world as character %q (account %d)", ses.SessionID, name, ses.AccountID)
	if accountMatch, err := AccountHasCharacterName(ses.CommandContext(), wh.database, ses.AccountID, name); err != nil || !accountMatch {
		log.Printf("[WORLD] Session %d: Tried to log in unsuccessfully from account %d with character %q: %v", ses.SessionID, ses.AccountID, name, err)
		return reject(false)
	}
	if err := prepareCharacterEntry(ses, wh, name); err != nil {
		log.Printf("[WORLD] Session %d: character entry failed: %v", ses.SessionID, err)
		return reject(true)
	}

	// Resume/upgrade legacy Safari identity before the scene's read-only recovery.
	if wh.Safari != nil && ses.HasValidClient() {
		if _, err := wh.Safari.GetSession(ses.CommandContext(), int64(ses.Client.CharData().ID)); err != nil {
			log.Printf("[Safari] Character resume: %v", err)
			return reject(true)
		}
	}
	// Load event flags for this character
	if ses.HasValidClient() {
		charID := int64(ses.Client.CharData().ID)
		if err := wh.EventFlags.LoadFlagsContext(ses.CommandContext(), charID); err != nil {
			log.Printf("[WORLD] Failed to load event flags for char %d: %v", charID, err)
			return reject(true)
		}
	}

	// Stage battle publication until all restores and initial state reads succeed.
	var restoredBattle map[string]interface{}
	// Check for a saved battle from a previous session and restore it
	if ses.HasValidClient() {
		charID := int64(ses.Client.CharData().ID)
		restoreStarted := time.Now()
		restoreBudget := "none"
		if deadline, ok := ses.CommandContext().Deadline(); ok {
			restoreBudget = time.Until(deadline).String()
		}
		poolBefore := wh.database.Stats()
		battle, err := restoreBattleOnLogin(ses.CommandContext(), wh.database, charID)
		if err != nil {
			// The command deadline includes character loading and owner handoff.
			// Keep diagnostics on failure only; pool deltas cover all users of this DB
			// and are evidence of contention, not attribution to this character.
			poolAfter := wh.database.Stats()
			log.Printf("[PokeBattle] Restore failed for character %d (login_elapsed=%s restore_elapsed=%s db_open=%d db_in_use=%d pool_waits=%d pool_wait=%s restore_deadline_remaining_at_start=%s command_error=%v): %v",
				charID, time.Since(loginStarted), time.Since(restoreStarted), poolAfter.OpenConnections, poolAfter.InUse,
				poolAfter.WaitCount-poolBefore.WaitCount, poolAfter.WaitDuration-poolBefore.WaitDuration, restoreBudget, ses.CommandContext().Err(), err)
			return reject(true)
		}
		if battle != nil {
			// A terminal record still owns dismissal and any post-battle plan.
			// Let the scene's coherent gameplay read restore it; login must not
			// delete it or bypass the atomic close/plan transaction.
			if battle.IsOver() && battle.PendingMoveLearn == nil {
				log.Printf("[PokeBattle] Retained terminal battle for character %d pending scene recovery", charID)
			} else {
				// Preserve the committed phase, including a required faint switch.
				// Pending move choices use the existing move-learn presentation.
				resp := buildBattleStateResponse(battle)
				if battle.IsOver() && battle.PendingMoveLearn != nil {
					resp["phase"] = "move_learn_prompt"
					resp["events"] = []pokebattle.BattleEvent{{
						Type:        pokebattle.EventMoveLearnPrompt,
						Message:     fmt.Sprintf("%s wants to learn %s, but already knows 4 moves!", battle.PlayerParty[battle.PendingMoveLearn.PokemonIndex].Name, battle.PendingMoveLearn.MoveName),
						NewMoveID:   battle.PendingMoveLearn.MoveID,
						NewMoveName: battle.PendingMoveLearn.MoveName,
					}}
				}
				if battle.Trainer != nil {
					resp["trainerClass"] = battle.Trainer.ClassName
					resp["trainerName"] = battle.Trainer.Name
				}
				resp["restored"] = true
				restoredBattle = resp
			}
		}
	}

	initial, err := readCharacterEntryState(ses, wh)
	if err != nil {
		log.Printf("[WORLD] Session %d: initial character state failed: %v", ses.SessionID, err)
		return reject(true)
	}
	if err := sendCharacterEntryState(ses, initial); err != nil {
		log.Printf("[WORLD] Session %d: initial character publication failed: %v", ses.SessionID, err)
		return reject(true)
	}
	if ses.IsClosed() || ses.CommandContext().Err() != nil {
		return reject(true)
	}
	// Character data is available before the acknowledgement can mount gameplay.
	// Every required restore has completed; no failed restore can follow success.
	if err := ses.SendStreamJSON(SimpleSuccessResponse{Value: 1}, opcodes.PostEnterWorld); err != nil {
		log.Printf("[WORLD] Session %d: entry acknowledgement failed: %v", ses.SessionID, err)
		ses.Close()
		return false
	}
	notifyResourceChange(ses)
	if restoredBattle != nil {
		if err := ses.SendStreamJSON(restoredBattle, opcodes.PokeBattleStartResponse); err != nil {
			log.Printf("[WORLD] Session %d: restored battle publication failed: %v", ses.SessionID, err)
			ses.Close()
			return false
		}
	}
	SendPlayerSpawn(ses, wh)
	return false
}

func HandleCharacterCreate(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req CharCreateRequest
	if err := json.Unmarshal(payload, &req); err != nil || !characterSelectSession(ses) {
		if ses != nil && !ses.IsClosed() {
			ses.SendStreamJSON(SimpleSuccessResponse{Value: 0}, opcodes.CharacterCreateResponse)
		}
		return false
	}
	if _, err := CharacterCreate(ses.CommandContext(), wh.database, ses.AccountID, req); err != nil {
		log.Printf("[CharacterCreate] Account %d name %q: %v", ses.AccountID, req.Name, err)
		ses.SendStreamJSON(SimpleSuccessResponse{Value: 0}, opcodes.CharacterCreateResponse)
		return false
	}
	ses.SendStreamJSON(SimpleSuccessResponse{Value: 1}, opcodes.CharacterCreateResponse)
	sendCharInfo(ses, wh, ses.AccountID)
	return false
}

const maxChatMessageLength = 256
const chatRateLimitMs = 500
const generalChatMessageType = "general"

type SendChatMessageRequest struct {
	Text string `json:"text"`
}

type ChatMessageBroadcast struct {
	SenderID    int    `json:"senderId,omitempty"`
	SenderName  string `json:"senderName"`
	Text        string `json:"text"`
	MessageType string `json:"messageType"`
}

func (wh *WorldHandler) SetPublicChatSink(sink func(ChatMessageBroadcast)) {
	if wh == nil {
		return
	}
	wh.publicChatSinkMu.Lock()
	wh.publicChatSink = sink
	wh.publicChatSinkMu.Unlock()
}

func (wh *WorldHandler) emitPublicChat(message ChatMessageBroadcast) {
	if wh == nil {
		return
	}
	wh.publicChatSinkMu.RLock()
	sink := wh.publicChatSink
	wh.publicChatSinkMu.RUnlock()
	if sink != nil {
		sink(message)
	}
}

func (wh *WorldHandler) broadcastGeneralChat(message ChatMessageBroadcast) {
	session.GetSessionManager().ForEachSession(func(targetSes *session.Session) {
		if !targetSes.Presence().Authenticated {
			return
		}
		targetSes.SendJSON(message, opcodes.ChatMessageBroadcast)
	})
	wh.emitPublicChat(message)
}

// BroadcastExternalChat inserts a verified Discord message into CaptureQuest's
// authoritative global player-chat stream.
func (wh *WorldHandler) BroadcastExternalChat(ctx context.Context, senderName, text string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	senderName = strings.Join(strings.Fields(senderName), " ")
	text = strings.Join(strings.Fields(text), " ")
	if senderName == "" || text == "" {
		return fmt.Errorf("sender and message are required")
	}
	text = boundedChatText(text)

	senderName = CensorMessage(senderName)
	text = CensorMessage(text)
	message := ChatMessageBroadcast{
		SenderName: senderName, Text: text, MessageType: generalChatMessageType,
	}
	log.Printf("[Chat] %s: %s (source: discord)", senderName, text)
	if err := wh.persistChatMessage(ctx, 0, senderName, text, nil); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		log.Printf("[Chat] History persistence failed: %v", err)
	}
	wh.broadcastGeneralChat(message)
	return nil
}

func HandleSendChatMessage(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() || ses.CommandContext().Err() != nil {
		return false
	}
	var req SendChatMessageRequest
	if err := decodePlayerMovement(payload, &req); err != nil {
		return false
	}

	// Validation: trim and reject empty
	req.Text = boundedChatText(req.Text)
	if req.Text == "" {
		return false
	}

	senderName := "Unknown"
	charID := 0
	if ses.Client != nil && ses.Client.CharData() != nil {
		senderName = ses.Client.CharData().Name
		charID = int(ses.Client.CharData().ID)
	}

	if !ses.AllowChatMessage(time.Now(), time.Duration(chatRateLimitMs)*time.Millisecond) {
		return false
	}

	// Check for slash commands before broadcasting
	if HandleChatCommand(ses, req.Text, wh) {
		return false
	}

	// Apply chat filter — censor disallowed words
	req.Text = CensorMessage(req.Text)

	log.Printf("[Chat] %s: %s", senderName, req.Text)

	// Persist within this command so its lifecycle includes the database write.
	if err := wh.persistChatMessage(ses.CommandContext(), charID, senderName, req.Text, ses.MapID); err != nil {
		if ses.CommandContext().Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		// Chat history remains best effort. A non-cancellation storage failure
		// does not suppress a live message or invite an automatic resend.
		log.Printf("[Chat] History persistence failed: %v", err)
	}

	wh.broadcastGeneralChat(ChatMessageBroadcast{
		SenderID: charID, SenderName: senderName, Text: req.Text, MessageType: generalChatMessageType,
	})

	return false
}

func (wh *WorldHandler) persistChatMessage(ctx context.Context, characterID int, name, text string, mapID any) error {
	if wh.database == nil {
		return nil
	}
	// Keep persistence in the owning command so disconnect/shutdown drains it.
	// Its query deadline bounds a slow database without detached goroutines.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := wh.database.ExecContext(ctx,
		"INSERT INTO chat_messages (character_id, character_name, message_type, text, map_id) VALUES ($1, $2, $3, $4, $5)",
		characterID, name, generalChatMessageType, text, mapID,
	); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

func boundedChatText(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > maxChatMessageLength {
		runes = runes[:maxChatMessageLength]
	}
	return string(runes)
}

func SendSystemMessage(ses *session.Session, text string) {
	ses.SendStreamJSON(ChatMessageBroadcast{
		Text:        text,
		MessageType: "system",
	}, opcodes.ChatMessageBroadcast)
}

func SendSpecialMessage(ses *session.Session, text string, msgType string) {
	ses.SendStreamJSON(ChatMessageBroadcast{
		Text:        text,
		MessageType: msgType,
	}, opcodes.ChatMessageBroadcast)
}
