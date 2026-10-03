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
	if accountMatch, err := AccountHasCharacterName(ses.CommandContext(), ses.AccountID, name); err != nil || !accountMatch {
		log.Printf("[WORLD] Session %d: Tried to log in unsuccessfully from account %d with character %q: %v", ses.SessionID, ses.AccountID, name, err)
		return false
	}
	if err := sendCharacterStateFromDB(ses, wh, name); err != nil {
		log.Printf("[WORLD] Session %d: character entry failed: %v", ses.SessionID, err)
		ses.SendStreamJSON(SimpleSuccessResponse{Value: 0}, opcodes.PostEnterWorld)
		return false
	}

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
		battle, err := restoreBattleOnLogin(wh.database, charID)
		if err != nil {
			log.Printf("[PokeBattle] Restore failed for character %d: %v", charID, err)
			ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Could not restore your battle. Please reconnect."}, opcodes.PokeBattleStartResponse)
			ses.Close()
			return false
		}
		if battle != nil {
			// If the battle is already over and there's no pending move learn,
			// the results (XP, party) were already saved — just clean up silently.
			if battle.IsOver() && battle.PendingMoveLearn == nil {
				log.Printf("[PokeBattle] Restored battle for char %d is already over with no pending action — cleaning up", charID)
				if err := pokebattle.CloseBattle(ses.CommandContext(), wh.database, charID, battle); err != nil {
					log.Printf("[PokeBattle] Close restored battle for character %d: %v", charID, err)
				} else {
					forgetBattle(charID, battle)
				}
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

	ctx := ses.CommandContext()
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
func (wh *WorldHandler) BroadcastExternalChat(senderName, text string) error {
	senderName = strings.Join(strings.Fields(senderName), " ")
	text = strings.Join(strings.Fields(text), " ")
	if senderName == "" || text == "" {
		return fmt.Errorf("sender and message are required")
	}
	textRunes := []rune(text)
	if len(textRunes) > maxChatMessageLength {
		text = string(textRunes[:maxChatMessageLength])
	}
	senderName = CensorMessage(senderName)
	text = CensorMessage(text)
	message := ChatMessageBroadcast{
		SenderName: senderName, Text: text, MessageType: generalChatMessageType,
	}
	log.Printf("[Chat] %s: %s (source: discord)", senderName, text)
	wh.persistChatMessage(0, senderName, text, nil)
	wh.broadcastGeneralChat(message)
	return nil
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

	// Persist within this command so its lifecycle includes the database write.
	wh.persistChatMessage(charID, senderName, req.Text, ses.MapID)

	wh.broadcastGeneralChat(ChatMessageBroadcast{
		SenderID: charID, SenderName: senderName, Text: req.Text, MessageType: generalChatMessageType,
	})

	return false
}

func (wh *WorldHandler) persistChatMessage(characterID int, name, text string, mapID any) {
	if wh.database == nil {
		return
	}
	// Keep persistence in the owning command so disconnect/shutdown drains it.
	// Its query deadline bounds a slow database without detached goroutines.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := wh.database.ExecContext(ctx,
		"INSERT INTO chat_messages (character_id, character_name, message_type, text, map_id) VALUES ($1, $2, $3, $4, $5)",
		characterID, name, generalChatMessageType, text, mapID,
	); err != nil {
		log.Printf("[Chat] failed to persist message: %v", err)
	}
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
