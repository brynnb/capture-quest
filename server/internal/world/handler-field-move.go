package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
)

type StateFieldResponse struct {
	Success     bool   `json:"success" tstype:"true"`
	RequestID   string `json:"requestId"`
	CharacterID int64  `json:"characterId"`
	MoveName    string `json:"moveName"`
	MapID       int    `json:"mapId"`
	Message     string `json:"message"`
}
type FieldMoveUseRequestPayload struct {
	RequestID       string `json:"requestId,omitempty"`
	CharacterID     int64  `json:"characterId,omitempty"`
	CommandRevision *int64 `json:"commandRevision,omitempty"`
	FromX           *int   `json:"fromX,omitempty"`
	FromY           *int   `json:"fromY,omitempty"`
	MoveName        string `json:"moveName"`
	TargetX         *int   `json:"targetX,omitempty"`
	TargetY         *int   `json:"targetY,omitempty"`
	MapID           *int   `json:"mapId,omitempty"`
	Direction       string `json:"direction,omitempty"`
}

type FieldMoveUseResponsePayload struct {
	Success  bool      `json:"success"`
	Error    string    `json:"error,omitempty"`
	Message  string    `json:"message,omitempty"`
	MoveName string    `json:"moveName,omitempty"`
	MapID    int       `json:"mapId,omitempty"`
	TargetX  int       `json:"targetX,omitempty"`
	TargetY  int       `json:"targetY,omitempty"`
	Tile     *TileEdit `json:"tile,omitempty"`
}

func HandleFieldMoveUse(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() {
		return false
	}

	var req FieldMoveUseRequestPayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &req); err != nil {
			sendFieldMoveUseError(ses, "Invalid field move request.")
			return false
		}
	}

	if move := normalizeFieldMoveName(req.MoveName); move != "STRENGTH" && move != "FLASH" {
		if req.RequestID != "" || req.CharacterID != 0 || req.CommandRevision != nil || req.FromX != nil || req.FromY != nil {
			sendInventoryCommandError(ses, req.RequestID, opcodes.FieldMoveUseResponse, "Tagged command is unsupported for this field move.")
			return false
		}
		if char := ses.Client.CharData(); char != nil {
			if battle := getBattle(int64(char.ID)); battle != nil && !battle.IsOver() {
				sendFieldMoveUseError(ses, "Field moves can't be used during a battle.")
				return false
			}
		}

	}

	switch normalizeFieldMoveName(req.MoveName) {
	case "CUT":
		return handleCutFieldMove(ses, req, wh)
	case "STRENGTH", "FLASH":
		if err := decodePlayerMovement(payload, &req); err != nil {
			sendInventoryCommandError(ses, req.RequestID, opcodes.FieldMoveUseResponse, "Invalid state field command.")
			return false
		}
		return handleStateFieldMove(ses, req, wh)
	case "DIG":
		return handleEscapeFieldMove(ses, req, wh)
	case "TELEPORT":
		return handleTeleportFieldMove(ses, req, wh)
	case "FLY":
		return handleFlyFieldMove(ses, req, wh)
	default:
		sendFieldMoveUseError(ses, "That move can't be used here.")
		return false
	}
}

func handleCutFieldMove(ses *session.Session, req FieldMoveUseRequestPayload, wh *WorldHandler) bool {
	if wh == nil || wh.CutTiles == nil {
		sendFieldMoveUseError(ses, "CUT can't be used here.")
		return false
	}

	charData := ses.Client.CharData()
	if charData == nil {
		return false
	}
	charID := int64(charData.ID)

	mapID := int(charData.MapID)
	if wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		mapID = UnifiedOverworldMapID
	}
	playerX, playerY := int(charData.X), int(charData.Y)
	if wh.PlayerMovement != nil {
		if x, y, movementMapID, ok := wh.PlayerMovement.GetPosition(int(charData.ID)); ok {
			playerX, playerY, mapID = x, y, movementMapID
		}
	}

	targetMapID := mapID
	if req.MapID != nil {
		targetMapID = *req.MapID
	}
	if wh.ActorManager != nil && wh.ActorManager.IsOverworld(targetMapID) {
		targetMapID = UnifiedOverworldMapID
	}
	if targetMapID != mapID {
		sendFieldMoveUseError(ses, "CUT can't be used here.")
		return false
	}

	targetX, targetY, ok := fieldMoveTargetTile(playerX, playerY, req)
	if !ok {
		sendFieldMoveUseError(ses, "CUT can't be used here.")
		return false
	}
	if abs(targetX-playerX)+abs(targetY-playerY) != 1 {
		sendFieldMoveUseError(ses, "You need to be next to it.")
		return false
	}

	state, err := db.ReadSnapshot(ses.CommandContext(), wh.database, func(_ context.Context, q db.ReadDBTX) (EventTileState, error) {
		return baseEventTileStateIn(q, targetMapID, targetX, targetY)
	})
	if err != nil || state.RawFootTileID == nil || *state.RawFootTileID != cuttableTreeRawFootTileID {
		sendFieldMoveUseError(ses, "There isn't anything to CUT.")
		return false
	}

	permission := CanUseFieldMove(charID, "CUT", wh.EventFlags)
	if !permission.Allowed {
		sendFieldMoveUseError(ses, permission.Message)
		return false
	}

	wh.CutTiles.MarkCut(charID, targetMapID, targetX, targetY)

	replacement := cutTreeReplacementForTileImage(state.TileImageID)
	tile := TileEdit{
		X:             targetX,
		Y:             targetY,
		TileImageID:   replacement.TileImageID,
		CollisionType: replacement.CollisionType,
		RawFootTileID: replacement.RawFootTileID,
		TalkOverTile:  replacement.TalkOverTile,
	}
	message := "Used CUT!"
	if permission.KnownByName != "" {
		message = permission.KnownByName + " used CUT!"
	}

	ses.SendStreamJSON(FieldMoveUseResponsePayload{
		Success:  true,
		Message:  message,
		MoveName: "CUT",
		MapID:    targetMapID,
		TargetX:  targetX,
		TargetY:  targetY,
		Tile:     &tile,
	}, opcodes.FieldMoveUseResponse)
	return false
}

func handleStateFieldMove(ses *session.Session, req FieldMoveUseRequestPayload, wh *WorldHandler) bool {
	reject := func(message string) {
		sendInventoryCommandError(ses, req.RequestID, opcodes.FieldMoveUseResponse, message)
	}
	charID := int64(ses.Client.CharData().ID)
	if wh == nil || wh.database == nil || req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != charID || req.CommandRevision == nil || req.MapID == nil || req.FromX == nil || req.FromY == nil {
		reject("Invalid state field command.")
		return false
	}
	input, _ := json.Marshal(req)
	encoded, _, err := db.ExecuteFieldCommand(ses.CommandContext(), wh.database, charID, "field-state", req.RequestID, req.CommandRevision, input, func(tx db.DBTX) ([]byte, error) {
		if err := requireNoOwnedBattleIn(tx, charID); err != nil {
			return nil, err
		}
		var mapID, x, y int
		if err := tx.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=$1`, charID).Scan(&mapID, &x, &y); err != nil {
			return nil, err
		}
		mapID = normalizedVisiblePlayerMapID(wh, mapID)
		if mapID != *req.MapID || x != *req.FromX || y != *req.FromY {
			return nil, fmt.Errorf("field move source changed")
		}
		result, err := prepareStateFieldMoveIn(tx, charID, mapID, req.MoveName)
		if err != nil {
			return nil, err
		}
		if !result.Success {
			return nil, &itemuse.Rejection{Message: result.Message}
		}
		return json.Marshal(StateFieldResponse{Success: true, RequestID: req.RequestID, CharacterID: charID, MoveName: result.MoveName, MapID: mapID, Message: result.Message})
	})
	if err != nil {
		var denied *itemuse.Rejection
		if errors.As(err, &denied) {
			reject(denied.Message)
		} else {
			reject("Field move state changed or could not be read. Recover before trying again.")
		}
		return false
	}
	var reply StateFieldResponse
	if err := json.Unmarshal(encoded, &reply); err != nil || !reply.Success || reply.CharacterID != charID || reply.RequestID != req.RequestID || reply.MoveName != normalizeFieldMoveName(req.MoveName) || reply.MapID != *req.MapID || reply.Message == "" {
		reject("Invalid committed field state result.")
		return false
	}
	ses.SendStreamJSON(reply, opcodes.FieldMoveUseResponse)
	return false
}

func handleEscapeFieldMove(ses *session.Session, req FieldMoveUseRequestPayload, wh *WorldHandler) bool {
	charData := ses.Client.CharData()
	if charData == nil {
		return false
	}

	permission := CanUseFieldMove(int64(charData.ID), "DIG", eventFlagsForFieldMove(wh))
	if !permission.Allowed {
		sendFieldMoveUseError(ses, permission.Message)
		return false
	}

	destMapID, destX, destY, err := escapeRopeDestination(ses, wh)
	if err != nil {
		sendFieldMoveUseError(ses, err.Error())
		return false
	}
	if err := teleportPlayerTo(ses, wh, destMapID, destX, destY); err != nil {
		log.Printf("[FieldMove] Teleport for %d: %v", charData.ID, err)
		sendFieldMoveUseError(ses, "Could not save the destination. Please try again.")
		return false
	}

	message := "Dug out of the dungeon."
	if permission.KnownByName != "" {
		message = permission.KnownByName + " used DIG!"
	}
	ses.SendStreamJSON(FieldMoveUseResponsePayload{
		Success:  true,
		Message:  message,
		MoveName: permission.MoveName,
		MapID:    destMapID,
		TargetX:  destX,
		TargetY:  destY,
	}, opcodes.FieldMoveUseResponse)
	return false
}

func handleTeleportFieldMove(ses *session.Session, req FieldMoveUseRequestPayload, wh *WorldHandler) bool {
	charData := ses.Client.CharData()
	if charData == nil {
		return false
	}

	permission := CanUseFieldMove(int64(charData.ID), "TELEPORT", eventFlagsForFieldMove(wh))
	if !permission.Allowed {
		sendFieldMoveUseError(ses, permission.Message)
		return false
	}

	destMapID, destX, destY, err := characterBindDestination(charData.ID)
	if err != nil {
		sendFieldMoveUseError(ses, "TELEPORT can't be used here.")
		return false
	}
	if err := teleportPlayerTo(ses, wh, destMapID, destX, destY); err != nil {
		log.Printf("[FieldMove] Teleport for %d: %v", charData.ID, err)
		sendFieldMoveUseError(ses, "Could not save the destination. Please try again.")
		return false
	}

	message := "Teleported to safety."
	if permission.KnownByName != "" {
		message = permission.KnownByName + " used TELEPORT!"
	}
	ses.SendStreamJSON(FieldMoveUseResponsePayload{
		Success:  true,
		Message:  message,
		MoveName: permission.MoveName,
		MapID:    destMapID,
		TargetX:  destX,
		TargetY:  destY,
	}, opcodes.FieldMoveUseResponse)
	return false
}

func handleFlyFieldMove(ses *session.Session, req FieldMoveUseRequestPayload, wh *WorldHandler) bool {
	charData := ses.Client.CharData()
	if charData == nil {
		return false
	}

	if req.MapID == nil || req.TargetX == nil || req.TargetY == nil {
		sendFieldMoveUseError(ses, "Choose where to FLY.")
		return false
	}

	result, err := useFly(ses.CommandContext(), wh.database, int64(charData.ID), *req.MapID, *req.TargetX, *req.TargetY, func(mapID int) int {
		return normalizedVisiblePlayerMapID(wh, mapID)
	})
	if err != nil {
		message := "Could not FLY. Please try again."
		var rejection *itemuse.Rejection
		if errors.As(err, &rejection) {
			message = rejection.Message
		} else {
			log.Printf("[FieldMove] FLY failed for character %d: %v", charData.ID, err)
		}
		sendFieldMoveUseError(ses, message)
		return false
	}
	publishCommittedTeleport(ses, wh, result.MapID, result.X, result.Y)

	message := "Flew to destination."
	if result.Permission.KnownByName != "" {
		message = result.Permission.KnownByName + " used FLY!"
	}
	ses.SendStreamJSON(FieldMoveUseResponsePayload{
		Success:  true,
		Message:  message,
		MoveName: result.Permission.MoveName,
		MapID:    result.MapID,
		TargetX:  result.X,
		TargetY:  result.Y,
	}, opcodes.FieldMoveUseResponse)
	return false
}

func fieldMoveTargetTile(playerX, playerY int, req FieldMoveUseRequestPayload) (int, int, bool) {
	if req.TargetX != nil && req.TargetY != nil {
		return *req.TargetX, *req.TargetY, true
	}
	switch normalizeWarpDirection(req.Direction) {
	case "UP":
		return playerX, playerY - 1, true
	case "DOWN":
		return playerX, playerY + 1, true
	case "LEFT":
		return playerX - 1, playerY, true
	case "RIGHT":
		return playerX + 1, playerY, true
	default:
		return 0, 0, false
	}
}

func cutTreeReplacementForTileImage(tileImageID int) TileEdit {
	rawFoot := cutTreeReplacementRawFoot
	fallback := TileEdit{
		TileImageID:   cutTreeReplacementTileImage,
		CollisionType: collisionLand,
		RawFootTileID: &rawFoot,
	}
	if db.GlobalWorldDB == nil || db.GlobalWorldDB.DB == nil {
		return fallback
	}

	var (
		replacementTileImageID int
		collisionType          int
		rawFootTileID          sql.NullInt64
		talkOverTile           bool
	)
	err := db.GlobalWorldDB.DB.QueryRow(`
		SELECT replacement.id, COALESCE(tp.collision_type, $2), replacement.raw_foot_tile_id, COALESCE(replacement.talk_over_tile, FALSE)
		FROM phaser_tile_images source
		JOIN phaser_tile_images replacement
		  ON replacement.tileset_id = source.tileset_id
		 AND replacement.raw_foot_tile_id = $3
		LEFT JOIN phaser_tile_properties tp ON tp.tile_image_id = replacement.id
		WHERE source.id = $1
		ORDER BY replacement.id
		LIMIT 1`,
		tileImageID,
		collisionLand,
		cutTreeReplacementRawFoot,
	).Scan(&replacementTileImageID, &collisionType, &rawFootTileID, &talkOverTile)
	if err != nil {
		return fallback
	}

	replacement := TileEdit{
		TileImageID:   replacementTileImageID,
		CollisionType: collisionType,
		TalkOverTile:  talkOverTile,
	}
	if rawFootTileID.Valid {
		v := int(rawFootTileID.Int64)
		replacement.RawFootTileID = &v
	}
	return replacement
}

func eventFlagsForFieldMove(wh *WorldHandler) *EventFlagManager {
	if wh == nil {
		return nil
	}
	return wh.EventFlags
}

func characterBindDestination(charID uint32) (int, int, int, error) {
	bind, err := db_character.GetCharacterBind(context.Background(), charID)
	if err != nil {
		return RecoverySpawnMap, int(RecoverySpawnX), int(RecoverySpawnY), nil
	}
	return int(bind.MapID), int(bind.X), int(bind.Y), nil
}

func sendFieldMoveUseError(ses *session.Session, message string) {
	ses.SendStreamJSON(FieldMoveUseResponsePayload{
		Success: false,
		Error:   message,
	}, opcodes.FieldMoveUseResponse)
}
