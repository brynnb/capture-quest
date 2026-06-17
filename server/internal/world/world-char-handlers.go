package world

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
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
	var req EnterWorldRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("failed to unmarshal EnterWorld JSON: %v", err)
		return false
	}
	name := req.Name
	log.Printf("[WORLD] Session %d entering world as character %q (account %d)", ses.SessionID, name, ses.AccountID)
	if accountMatch, err := AccountHasCharacterName(context.Background(), ses.AccountID, name); err != nil || !accountMatch {
		log.Printf("[WORLD] Session %d: Tried to log in unsuccessfully from account %d with character %q: %v", ses.SessionID, ses.AccountID, name, err)
		return false
	}
	ses.CharacterName = name
	log.Printf("[WORLD] Session %d: Character name set to %q", ses.SessionID, ses.CharacterName)

	// Send PostEnterWorld success
	ses.SendStreamJSON(SimpleSuccessResponse{Value: 1}, opcodes.PostEnterWorld)

	// For CaptureQuest, send character state immediately.
	// This is the initial load, so we create a fresh client from the database
	sendCharacterStateFromDB(ses, name)

	// Load event flags for this character
	if ses.HasValidClient() {
		charID := int64(ses.Client.CharData().ID)
		if err := wh.EventFlags.LoadFlags(charID); err != nil {
			log.Printf("[WORLD] Failed to load event flags for char %d: %v", charID, err)
		}
	}

	// Check for a saved battle from a previous session and restore it
	if ses.HasValidClient() {
		charID := int64(ses.Client.CharData().ID)
		if battle := restoreBattleOnLogin(charID); battle != nil {
			// If the battle is already over and there's no pending move learn,
			// the results (XP, party) were already saved — just clean up silently.
			if battle.IsOver() && battle.PendingMoveLearn == nil {
				log.Printf("[PokeBattle] Restored battle for char %d is already over with no pending action — cleaning up", charID)
				removeBattle(charID)
			} else {
				// For mid-battle restores, present as action_select so the client
				// shows the normal battle UI. For pending move learn, send as
				// move_learn_prompt so the client shows the move learn dialog.
				resp := buildBattleStateResponse(battle)
				if battle.IsOver() && battle.PendingMoveLearn != nil {
					resp["phase"] = "move_learn_prompt"
					resp["events"] = []pokebattle.BattleEvent{{
						Type:        pokebattle.EventMoveLearnPrompt,
						Message:     fmt.Sprintf("%s wants to learn %s, but already knows 4 moves!", battle.PlayerParty[battle.PendingMoveLearn.PokemonIndex].Name, battle.PendingMoveLearn.MoveName),
						NewMoveID:   battle.PendingMoveLearn.MoveID,
						NewMoveName: battle.PendingMoveLearn.MoveName,
					}}
				} else {
					resp["phase"] = "action_select"
				}
				if battle.Trainer != nil {
					resp["trainerClass"] = battle.Trainer.ClassName
					resp["trainerName"] = battle.Trainer.Name
				}
				resp["restored"] = true
				ses.SendStreamJSON(resp, opcodes.PokeBattleStartResponse)
			}
		}
	}

	return false
}

func HandleCharacterCreate(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req CharCreateRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("failed to unmarshal CharCreate JSON: %v", err)
		return false
	}

	name := req.Name
	if valid, _ := ValidateName(name); !valid {
		ses.SendStreamJSON(SimpleSuccessResponse{Value: 0}, opcodes.CharacterCreateResponse)
		return false
	}

	if !CharacterCreate(ses, ses.AccountID, req) {
		log.Printf("[CharacterCreate] Failed for account %d, name %s", ses.AccountID, req.Name)
		ses.SendStreamJSON(SimpleSuccessResponse{Value: 0}, opcodes.CharacterCreateResponse)
		return false
	}
	ses.SendStreamJSON(SimpleSuccessResponse{Value: 1}, opcodes.CharacterCreateResponse)

	sendCharInfo(ses, ses.AccountID)
	return false
}

type CharacterDeleteRequest struct {
	Value string `json:"value"`
}

func HandleCharacterDelete(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	log.Printf("HandleCharacterDelete called for session %d, payload len=%d", ses.SessionID, len(payload))
	var req CharacterDeleteRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("failed to unmarshal Delete JSON: %v", err)
		return false
	}

	ctx := context.Background()
	name := req.Value
	log.Printf("Deleting character: %s for account %d", name, ses.AccountID)
	if err := DeleteCharacter(ctx, ses.AccountID, name); err != nil {
		log.Printf("DeleteCharacter failed: %v", err)
		return false
	}
	log.Printf("Character %s deleted successfully, sending updated char info", name)
	sendCharInfo(ses, ses.AccountID)
	return false
}

const maxChatMessageLength = 256
const chatRateLimitMs = 500
const generalChatMessageType = "general"

var (
	chatRateLimits   = make(map[int]time.Time)
	chatRateLimitsMu sync.Mutex
)

type SendChatMessageRequest struct {
	Text string `json:"text"`
}

type ChatMessageBroadcast struct {
	SenderID    int    `json:"senderId,omitempty"`
	SenderName  string `json:"senderName"`
	Text        string `json:"text"`
	MessageType string `json:"messageType"`
}

func HandleSendChatMessage(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req SendChatMessageRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("failed to unmarshal SendChatMessage JSON: %v", err)
		return false
	}

	// Validation: trim and reject empty
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		return false
	}

	// Validation: enforce max length
	if len(req.Text) > maxChatMessageLength {
		req.Text = req.Text[:maxChatMessageLength]
	}

	senderName := "Unknown"
	charID := 0
	if ses.Client != nil && ses.Client.CharData() != nil {
		senderName = ses.Client.CharData().Name
		charID = int(ses.Client.CharData().ID)
	}

	// Rate limiting: 1 message per 500ms per session
	chatRateLimitsMu.Lock()
	if lastSent, ok := chatRateLimits[ses.SessionID]; ok {
		if time.Since(lastSent) < time.Duration(chatRateLimitMs)*time.Millisecond {
			chatRateLimitsMu.Unlock()
			return false
		}
	}
	chatRateLimits[ses.SessionID] = time.Now()
	chatRateLimitsMu.Unlock()

	// Check for slash commands before broadcasting
	if HandleChatCommand(ses, req.Text, wh) {
		return false
	}

	// Apply chat filter — censor disallowed words
	req.Text = CensorMessage(req.Text)

	log.Printf("[Chat] %s: %s", senderName, req.Text)

	// Persist to database (fire-and-forget)
	go func() {
		myDB := db.GlobalWorldDB.DB
		if myDB == nil {
			return
		}
		_, err := myDB.Exec(
			"INSERT INTO chat_messages (character_id, character_name, message_type, text, map_id) VALUES ($1, $2, $3, $4, $5)",
			charID, senderName, generalChatMessageType, req.Text, ses.MapID,
		)
		if err != nil {
			log.Printf("[Chat] failed to persist message: %v", err)
		}
	}()

	// Broadcast to all authenticated sessions
	sm := session.GetSessionManager()
	sm.ForEachSession(func(targetSes *session.Session) {
		if !targetSes.Authenticated {
			return
		}
		targetSes.SendJSON(ChatMessageBroadcast{
			SenderID:    charID,
			SenderName:  senderName,
			Text:        req.Text,
			MessageType: generalChatMessageType,
		}, opcodes.ChatMessageBroadcast)
	})

	return false
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
