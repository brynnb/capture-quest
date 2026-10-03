package world

import (
	"capturequest/internal/db"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"capturequest/internal/api/opcodes"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/options"
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

// HandleSetOption handles all game option changes from the client
func HandleSetOption(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req struct {
		OptionID options.OptionId `json:"optionId"`
		Value    int              `json:"value"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("HandleSetOption: failed to unmarshal JSON: %v", err)
		return false
	}

	log.Printf("SetOption for character %d: %d = %d", ses.Client.CharData().ID, req.OptionID, req.Value)

	enabled := req.Value == 1

	switch req.OptionID {
	case options.OptionShowNetworkStats:
		ses.Client.SetShowNetworkStatsEnabled(enabled)
	case options.OptionAllowTrainerRebattles:
		ses.Client.SetAllowTrainerRebattlesEnabled(enabled)
	default:
		log.Printf("SetOption: unknown option %d", req.OptionID)
		return false
	}

	// Persist all options to database as JSON
	if err := ses.Client.SaveOptions(); err != nil {
		log.Printf("SetOption: failed to persist options for character %d: %v", ses.Client.CharData().ID, err)
	}
	return false
}

// HandleCharacterQuitRequest saves player data before returning to character select.
func HandleCharacterQuitRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	charData := ses.Client.CharData()

	if err := db_character.UpdateCharacter(charData, ses.AccountID); err != nil {
		log.Printf("failed to save player data on camp: %v", err)
	}
	wh.cleanupCharacterSession(ses)
	sendCharInfo(ses, ses.AccountID)
	return false
}

// HandleHeartbeat echoes the heartbeat back to the client for latency calculation
func HandleHeartbeat(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	// Parse request to get timestamp for latency calculation
	var req struct {
		Timestamp float64 `json:"timestamp"`
	}
	// Ignore error, if empty payload timestamp will be 0
	_ = json.Unmarshal(payload, &req)

	// Record heartbeat time for disconnect detection
	ses.RecordHeartbeat(time.Now())

	ses.SendStreamJSON(map[string]interface{}{"status": "ok", "timestamp": req.Timestamp}, opcodes.Heartbeat)
	return false
}

// HandleValidateNameRequest handles name validation
func HandleValidateNameRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("HandleValidateNameRequest: failed to unmarshal JSON: %v", err)
		return false
	}

	valid, errorMsg := ValidateName(req.Name)
	available := false
	errorMessage := errorMsg

	if valid {
		// Check database for availability
		char, err := db_character.GetCharacterByName(req.Name)
		if err != nil {
			// If error is "not found", then it's available
			available = true
		} else if char == nil || char.ID == 0 {
			available = true
		} else {
			available = false
			errorMessage = "Name is already taken."
		}
	}

	ses.SendStreamJSON(map[string]interface{}{
		"valid":        valid,
		"available":    available,
		"errorMessage": errorMessage,
	}, opcodes.ValidateNameResponse)

	return false
}

// PerformMapChange centralizes all logic for moving a character between maps.
// It handles data updates, persistence, and messaging.
func (wh *WorldHandler) PerformMapChange(ses *session.Session, mapID, instanceID int, x, y, z, heading float64) {
	if !ses.HasValidClient() {
		return
	}

	charData := ses.Client.CharData()
	log.Printf("[Map] Character %s (%d) changing map: %d -> %d", charData.Name, charData.ID, ses.MapID, mapID)

	// Update session and character data
	normalizedMapID := mapID
	if wh.ActorManager.IsOverworld(mapID) {
		normalizedMapID = UnifiedOverworldMapID
	}

	ses.MapID = normalizedMapID
	ses.InstanceID = instanceID
	charData.MapID = uint32(normalizedMapID)
	charData.X = x
	charData.Y = y
	charData.Z = z
	charData.Heading = heading

	// Persist change to database so it stays across logins
	if err := db_character.UpdateCharacter(charData, ses.AccountID); err != nil {
		log.Printf("Failed to update character map in DB: %v", err)
	}

	// Send entry message
	SendSystemMessage(ses, fmt.Sprintf("You have entered %s.", mapEntryDisplayName(wh, mapID, x, y)))

	// Notify client of the new state (zone, position, etc.)
	buildAndSendCharacterState(ses)
}

func HandleMapChangeRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req struct {
		MapID      int     `json:"mapId"`
		LegacyZone int     `json:"zoneId"`
		InstanceID int     `json:"instanceId"`
		X          float64 `json:"x"`
		Y          float64 `json:"y"`
		Z          float64 `json:"z"`
		Heading    float64 `json:"heading"`
	}

	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("HandleMapChangeRequest: failed to unmarshal JSON: %v", err)
		return false
	}

	mapID := req.MapID
	if mapID == 0 {
		mapID = req.LegacyZone
	}

	wh.PerformMapChange(ses, mapID, req.InstanceID, req.X, req.Y, req.Z, req.Heading)
	return false
}

func (wh *WorldHandler) handleStateUpdate(ses *session.Session) {
	// Send the updated state to the client
	sendUpdatedCharacterState(ses)
}

// Core Helpers

func sendCharInfo(ses *session.Session, accountId int64) {
	ctx := ses.CommandContext()
	charInfo, err := GetCharSelectInfo(ses, ctx, accountId)
	if err != nil {
		log.Printf("failed to get character select info for accountID %d: %v", accountId, err)
		return
	}
	ses.SendStreamJSON(charInfo, opcodes.SendCharInfo)
}

func sendCharacterStateFromDB(ses *session.Session, wh *WorldHandler, characterName string) (err error) {
	if ses.HasValidClient() {
		return fmt.Errorf("session already has a character")
	}
	charData, err := db_character.GetCharacterByName(characterName)
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
		if err != nil {
			wh.characterOwners.release(charID, ses)
		}
	}()
	// The previous owner may have flushed position/playtime during the handoff.
	// Loading before that barrier would resurrect stale character state.
	charData, err = db_character.GetCharacterByName(characterName)
	if err != nil {
		return err
	}
	if int64(charData.ID) != charID || int64(charData.AccountID) != ses.AccountID {
		return fmt.Errorf("character changed during connection handoff")
	}
	ses.CharacterName = characterName

	if isInvalidZeroPlayerPosition(int(charData.X), int(charData.Y)) {
		log.Printf("[WORLD] Recovering invalid saved position for character %s (%d) to map %d (%d,%d)",
			charData.Name, charData.ID, RecoverySpawnMap, int(RecoverySpawnX), int(RecoverySpawnY))
		charData.MapID = RecoverySpawnMap
		charData.X = RecoverySpawnX
		charData.Y = RecoverySpawnY
		charData.Z = RecoverySpawnZ
		if err := commitPlayerPosition(ctx, wh.database, int64(charData.ID), RecoverySpawnMap, int(RecoverySpawnX), int(RecoverySpawnY)); err != nil {
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
	if err := db_character.UpdateCharacter(charData, ses.AccountID); err != nil {
		log.Printf("sendCharacterStateFromDB: failed to update last login for %s: %v", characterName, err)
	}
	applyLocalCharacterPrivileges(charData)

	ensureLocalDevFixtures(int64(charData.ID))

	// Create client for this character (loads inventory, etc.)
	ses.Client, err = client.NewClient(charData, func(text string) {
		SendSystemMessage(ses, text)
	}, func(text string, msgType string) {
		SendSpecialMessage(ses, text, msgType)
	}, func() {
		sendUpdatedCharacterState(ses)
	})
	if err != nil {
		return err
	}
	ses.StartPlaytime(playStartedAt, charData.TimePlayed, int32(charData.ID))

	ses.SendStreamJSON(SimpleSuccessResponse{Value: 1}, opcodes.PostEnterWorld)
	buildAndSendCharacterState(ses)
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

	sendCQInventorySnapshot(ses, int32(charData.ID))

	// Send bind data as its own persisted model stream.
	bind, _ := db_character.GetCharacterBind(ctx, charData.ID)
	ses.SendStreamJSON(bind, opcodes.CharacterBind)
}

func sendCQInventorySnapshot(ses *session.Session, charID int32) {
	items, err := cqitems.NewStore(db.GlobalWorldDB.DB).GetCharacterInventory(charID)
	if err != nil {
		log.Printf("[CQItems] Failed to load inventory snapshot for char %d: %v", charID, err)
		return
	}
	money, _ := cqitems.NewStore(db.GlobalWorldDB.DB).GetCharacterMoney(charID)
	log.Printf("[CQItems] Sending inventory snapshot for char %d: %d items", charID, len(items))
	ses.SendStreamJSON(map[string]interface{}{
		"success": true,
		"items":   items,
		"money":   money,
	}, opcodes.CQInventoryResponse)
}
