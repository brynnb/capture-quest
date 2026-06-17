package world

import (
	"context"
	"database/sql"
	"encoding/json"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/session"
)

type FieldMoveUseRequestPayload struct {
	MoveName  string `json:"moveName"`
	TargetX   *int   `json:"targetX,omitempty"`
	TargetY   *int   `json:"targetY,omitempty"`
	MapID     *int   `json:"mapId,omitempty"`
	Direction string `json:"direction,omitempty"`
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

	switch normalizeFieldMoveName(req.MoveName) {
	case "CUT":
		return handleCutFieldMove(ses, req, wh)
	case "STRENGTH", "FLASH":
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

	state, err := baseEventTileState(targetMapID, targetX, targetY)
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
	charData := ses.Client.CharData()
	if charData == nil {
		return false
	}

	moveName := normalizeFieldMoveName(req.MoveName)
	charID := int64(charData.ID)
	mapID := int(charData.MapID)
	if wh != nil && wh.ActorManager != nil && wh.ActorManager.IsOverworld(mapID) {
		mapID = UnifiedOverworldMapID
	}
	if wh != nil && wh.PlayerMovement != nil {
		if _, _, movementMapID, ok := wh.PlayerMovement.GetPosition(int(charData.ID)); ok {
			mapID = movementMapID
		}
	}

	permission := CanUseFieldMove(charID, moveName, eventFlagsForFieldMove(wh))
	if !permission.Allowed {
		sendFieldMoveUseError(ses, permission.Message)
		return false
	}
	if err := recordFieldMoveState(charID, mapID, moveName); err != nil {
		sendFieldMoveUseError(ses, "That move can't be used here.")
		return false
	}

	message := permission.MoveName + " is ready."
	if permission.KnownByName != "" {
		switch moveName {
		case "STRENGTH":
			message = permission.KnownByName + " used STRENGTH. " + permission.KnownByName + " can move boulders."
		case "FLASH":
			message = permission.KnownByName + " used FLASH."
		}
	}

	ses.SendStreamJSON(FieldMoveUseResponsePayload{
		Success:  true,
		Message:  message,
		MoveName: permission.MoveName,
		MapID:    mapID,
	}, opcodes.FieldMoveUseResponse)
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
	teleportPlayerTo(ses, wh, destMapID, destX, destY)

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
	teleportPlayerTo(ses, wh, destMapID, destX, destY)

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

	permission := CanUseFieldMove(int64(charData.ID), "FLY", eventFlagsForFieldMove(wh))
	if !permission.Allowed {
		sendFieldMoveUseError(ses, permission.Message)
		return false
	}
	if req.MapID == nil || req.TargetX == nil || req.TargetY == nil {
		sendFieldMoveUseError(ses, "Choose where to FLY.")
		return false
	}

	destMapID := *req.MapID
	destX := *req.TargetX
	destY := *req.TargetY
	teleportPlayerTo(ses, wh, destMapID, destX, destY)

	message := "Flew to destination."
	if permission.KnownByName != "" {
		message = permission.KnownByName + " used FLY!"
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

func recordFieldMoveState(charID int64, mapID int, moveName string) error {
	_, err := db.GlobalWorldDB.DB.Exec(`
		INSERT INTO character_field_move_state (character_id, move_name, map_id, active, updated_at)
		VALUES ($1, $2, $3, 1, CURRENT_TIMESTAMP)
		ON CONFLICT (character_id, move_name) DO UPDATE SET
			map_id = EXCLUDED.map_id,
			active = 1,
			updated_at = CURRENT_TIMESTAMP`,
		charID,
		normalizeFieldMoveName(moveName),
		mapID,
	)
	return err
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
