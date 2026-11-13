package world

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	db_character "capturequest/internal/db/character"
	db_currency "capturequest/internal/db/currency"
	model "capturequest/internal/db/models"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/zone/client"
)

// world-handlers.go now contains general character state synchronization
// and core helpers. Specific opcode handlers have been moved to:
// - world-auth-handlers.go
// - world-char-handlers.go
// - world-item-handlers.go
// - world-combat-handlers.go
// - world-query-handlers.go

func HandleCharacterQuitRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	// Gameplay position changes have already committed through their owners.
	// Replaying cached character coordinates here can undo an uncertain commit.
	if err := wh.cleanupCharacterSession(ses.CommandContext(), ses); err != nil {
		log.Printf("[WORLD] Camp cleanup: %v", err)
	}
	sendCharInfo(ses, wh, ses.AccountID)
	return false
}

// HandleHeartbeat echoes the heartbeat back to the client for latency calculation
func HandleHeartbeat(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	// Parse request to get timestamp for latency calculation
	var req struct {
		Timestamp *float64 `json:"timestamp"`
	}
	if decodePlayerMovement(payload, &req) != nil || req.Timestamp == nil || *req.Timestamp < 0 || ses.IsClosed() || ses.CommandContext().Err() != nil {
		return false
	}

	// Record heartbeat time for disconnect detection
	ses.RecordHeartbeat(time.Now())

	ses.SendStreamJSON(map[string]interface{}{"status": "ok", "timestamp": *req.Timestamp}, opcodes.Heartbeat)
	return false
}

// HandleValidateNameRequest handles name validation
func HandleValidateNameRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.NameValidationRequest
	sendError := func(err error) {
		ses.SendStreamJSON(protocol.NameValidationError{RequestID: req.RequestID, Error: err.Error()}, opcodes.ValidateNameResponse)
	}
	if err := decodePlayerMovement(payload, &req); err != nil {
		sendError(err)
		return false
	}
	if req.RequestID != "" && !validBattleRequestID(req.RequestID) {
		sendError(fmt.Errorf("invalid name validation request identity"))
		return false
	}
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	valid, message, err := ValidateNameContext(ctx, wh.database, req.Name)
	if err != nil {
		sendError(err)
		return false
	}
	response := protocol.NameValidationResponse{Success: true, RequestID: req.RequestID, Name: req.Name, Valid: valid, ErrorMessage: message}
	if valid {
		// The schema's UNIQUE(name) includes soft-deleted rows. Match that authority,
		// and never turn a query failure into an available name.
		if err := wh.database.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM character_data WHERE name=$1)`, req.Name).Scan(&response.Available); err != nil {
			sendError(err)
			return false
		}
		if !response.Available {
			response.ErrorMessage = "Name is already taken."
		}
	}
	ses.SendStreamJSON(response, opcodes.ValidateNameResponse)
	return false
}

func (wh *WorldHandler) handleStateUpdate(ses *session.Session) {
	// Send the updated state to the client
	sendUpdatedCharacterState(ses)
}

// Core Helpers

func sendCharInfo(ses *session.Session, wh *WorldHandler, accountID int64) {
	characters, err := readCharacterRoster(ses.CommandContext(), wh.database, accountID)
	if err != nil {
		log.Printf("failed to get character select info for accountID %d: %v", accountID, err)
		return
	}
	// Legacy publications share the authoritative reader. Current clients use
	// the correlated account read, so these cannot settle a pending request.
	ses.SendStreamJSON(CharacterListResponse{Success: true, AccountID: accountID, Characters: characters}, opcodes.SendCharInfo)
}

// prepareCharacterEntry installs the serialized character owner without publishing
// entry success or initial state. Required restores still belong to the caller.
func prepareCharacterEntry(ses *session.Session, wh *WorldHandler, characterName string) (err error) {
	if ses.HasValidClient() {
		return fmt.Errorf("session already has a character")
	}
	charData, err := db_character.GetCharacterByNameContext(ses.CommandContext(), wh.database, characterName)
	if err != nil {
		return err
	}
	if int64(charData.AccountID) != ses.AccountID {
		return fmt.Errorf("character does not belong to authenticated account")
	}
	charID := int64(charData.ID)
	ctx, cancel := context.WithTimeout(ses.CommandContext(), 5*time.Second)
	defer cancel()
	if err = wh.characterOwners.acquire(ctx, charID, ses, wh.cleanupCharacterSession); err != nil {
		return err
	}
	defer func() {
		if err != nil && !ses.HasValidClient() {
			wh.characterOwners.release(charID, ses)
			ses.CharacterName = ""
			ses.MapID = -1
		}
		// After Client is installed, disconnect cleanup owns movement, cached
		// flags/battle and the registry. Releasing here would let that cleanup
		// mistake a partially prepared character for a stale connection.
	}()
	// The previous owner may have flushed position/playtime during the handoff.
	// Loading before that barrier would resurrect stale character state.
	charData, err = db_character.GetCharacterByNameContext(ctx, wh.database, characterName)
	if err != nil {
		return err
	}
	if int64(charData.ID) != charID || int64(charData.AccountID) != ses.AccountID {
		return fmt.Errorf("character changed during connection handoff")
	}
	ses.CharacterName = characterName

	if isInvalidZeroPlayerPosition(int(charData.X), int(charData.Y)) {
		destination := recoverySpawnDestination()
		log.Printf("[WORLD] Recovering invalid saved position for character %s (%d) to map %d (%d,%d)",
			charData.Name, charData.ID, destination.MapID, destination.X, destination.Y)
		charData.MapID = uint32(destination.MapID)
		charData.X = RecoverySpawnX
		charData.Y = RecoverySpawnY
		charData.Z = RecoverySpawnZ
		if err := commitRecoveryPlayerPosition(ctx, wh.database, int64(charData.ID), charData.Heading); err != nil {
			return err
		}

	}

	// Initialize session map from character data
	ses.MapID = int(charData.MapID)
	ses.X = float32(charData.X)
	ses.Y = float32(charData.Y)
	ses.InstanceID = 0

	// Update last login and begin the active play interval from the same instant.
	playStartedAt := time.Now()
	charData.LastLogin = uint32(playStartedAt.Unix())
	if err := db_character.SetCharacterLastLogin(ctx, wh.database, charData.ID, ses.AccountID, charData.LastLogin); err != nil {
		return fmt.Errorf("update last login for %s: %w", characterName, err)
	}
	applyLocalCharacterPrivileges(charData)

	// Create client for this character (loads inventory, etc.)
	ses.Client, err = client.NewClient(ses.CommandContext(), wh.database, charData, func(text string) {
		SendSystemMessage(ses, text)
	}, func(text string, msgType string) {
		SendSpecialMessage(ses, text, msgType)
	}, func() {
		sendUpdatedCharacterState(ses)
	})
	if err != nil {
		return err
	}
	// Movement ownership must exist before gameplay starts. Previously the first
	// arbitrary browser position report created it, so fresh entry had no owner.
	if wh.PlayerMovement != nil && wh.ActorManager != nil {
		if err := RegisterPlayerForMovement(ses, wh); err != nil {
			return fmt.Errorf("restore movement ownership: %w", err)
		}
	}
	ses.StartPlaytime(playStartedAt, charData.TimePlayed, int32(charData.ID))

	return nil
}

type characterEntryState struct {
	character protocol.CharacterData
	wallet    *model.CharacterWallet
	bind      *model.CharacterBind
}

// Read every required initial stream before publishing any of them. Entry must
// fail when storage is unavailable, rather than enter with a null/stale wallet.
func readCharacterEntryState(ses *session.Session, wh *WorldHandler) (characterEntryState, error) {
	var state characterEntryState
	if !ses.HasValidClient() || ses.IsClosed() {
		return state, session.ErrSessionClosed
	}
	char := ses.Client.CharData()
	ctx := ses.CommandContext()
	wallet, err := db_currency.GetCharacterWalletContext(ctx, wh.database, char.ID)
	if err != nil {
		return state, err
	}
	bind, err := db_character.GetCharacterBindContext(ctx, wh.database, char.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return state, err
	}
	if err := ctx.Err(); err != nil {
		return state, err
	}
	state.character = protocol.CharacterData{CharacterData: *char, Options: ses.Client.Options()}
	state.wallet, state.bind = wallet, bind
	return state, nil
}

func sendCharacterEntryState(ses *session.Session, state characterEntryState) error {
	for _, message := range []struct {
		opcode opcodes.OpCode
		data   interface{}
	}{
		{opcodes.CharacterData, state.character},
		{opcodes.CharacterWallet, state.wallet},
		{opcodes.CharacterBind, state.bind},
	} {
		if ses.IsClosed() {
			return session.ErrSessionClosed
		}
		if err := ses.CommandContext().Err(); err != nil {
			return err
		}
		if err := ses.SendStreamJSON(message.data, message.opcode); err != nil {
			return fmt.Errorf("initial character stream %d: %w", message.opcode, err)
		}
	}
	return nil
}

func sendUpdatedCharacterState(ses *session.Session) {
	if !ses.HasValidClient() {
		return
	}
	buildAndSendCharacterState(ses)
}

func buildAndSendCharacterState(ses *session.Session) {
	if ses == nil || ses.Client == nil {
		return
	}

	charData := ses.Client.CharData()
	ctx := ses.CommandContext()

	ses.SendStreamJSON(protocol.CharacterData{
		CharacterData: *charData,
		Options:       ses.Client.Options(),
	}, opcodes.CharacterData)

	// Send wallet as its own persisted model stream.
	wallet, _ := db_character.GetCharacterWallet(ctx, charData.ID)
	ses.SendStreamJSON(wallet, opcodes.CharacterWallet)

	notifyResourceChange(ses)

	// Send bind data as its own persisted model stream.
	bind, _ := db_character.GetCharacterBind(ctx, charData.ID)
	ses.SendStreamJSON(bind, opcodes.CharacterBind)
}
