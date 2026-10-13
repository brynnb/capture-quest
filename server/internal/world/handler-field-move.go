package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/itemuse"
	"capturequest/internal/logutil"
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
type CutFieldResponse struct {
	StateFieldResponse `tstype:",extends"`
	CutScope           string `json:"cutScope"`
	TargetX            int    `json:"targetX"`
	TargetY            int    `json:"targetY"`
}
type FieldMoveUseRequestPayload struct {
	CutScope        string `json:"cutScope,omitempty"`
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

	if move := normalizeFieldMoveName(req.MoveName); move != "STRENGTH" && move != "FLASH" && move != "CUT" {
		if req.RequestID != "" || req.CharacterID != 0 || req.CommandRevision != nil || req.FromX != nil || req.FromY != nil || req.CutScope != "" {
			sendInventoryCommandError(ses, req.RequestID, opcodes.FieldMoveUseResponse, "Tagged command is unsupported for this field move.")
			return false
		}
		if move == "FLY" {
			if char := ses.Client.CharData(); char != nil {
				if battle := getBattle(int64(char.ID)); battle != nil && !battle.IsOver() {
					sendFieldMoveUseError(ses, "Field moves can't be used during a battle.")
					return false
				}
			}
		}
	}

	switch normalizeFieldMoveName(req.MoveName) {
	case "CUT":
		if err := decodePlayerMovement(payload, &req); err != nil {
			sendInventoryCommandError(ses, req.RequestID, opcodes.FieldMoveUseResponse, "Invalid CUT command.")
			return false
		}
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
	reject := func(message string) {
		sendInventoryCommandError(ses, req.RequestID, opcodes.FieldMoveUseResponse, message)
	}
	if wh == nil || wh.database == nil || wh.CutTiles == nil {
		reject("CUT state unavailable.")
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	if req.RequestID == "" || len(req.RequestID) > 64 || req.CharacterID != charID || req.CommandRevision == nil || req.MapID == nil || req.FromX == nil || req.FromY == nil || req.TargetX == nil || req.TargetY == nil || !wh.CutTiles.MatchesScope(charID, *req.MapID, req.CutScope) {
		reject("Invalid or retired CUT command.")
		return false
	}
	var mapID, targetX, targetY int
	cutScope := req.CutScope
	var permission FieldMovePermissionResult
	var replacement TileEdit
	input, _ := json.Marshal(req)
	encoded, replay, err := db.ExecuteFieldCommand(ses.CommandContext(), wh.database, charID, "cut", req.RequestID, req.CommandRevision, input, func(tx db.DBTX) ([]byte, error) {
		prepareErr := func() error {
			if err := db.LockCharacter(tx, charID); err != nil {
				return err
			}
			if err := requireNoOwnedBattleIn(tx, charID); err != nil {
				return err
			}
			var x, y int
			if err := tx.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=$1`, charID).Scan(&mapID, &x, &y); err != nil {
				return err
			}
			mapID = normalizedVisiblePlayerMapID(wh, mapID)
			if !wh.CutTiles.MatchesScope(charID, mapID, cutScope) || mapID != *req.MapID || x != *req.FromX || y != *req.FromY {
				return &itemuse.Rejection{Message: "CUT source retired or changed."}
			}
			if req.MapID != nil && normalizedVisiblePlayerMapID(wh, *req.MapID) != mapID {
				return &itemuse.Rejection{Message: "CUT can't be used here."}
			}
			var ok bool
			targetX, targetY, ok = fieldMoveTargetTile(x, y, req)
			if !ok || abs(targetX-x)+abs(targetY-y) != 1 {
				return &itemuse.Rejection{Message: "You need to be next to it."}
			}
			state, err := baseEventTileStateIn(tx, mapID, targetX, targetY)
			if err != nil {
				return fmt.Errorf("CUT map=%d target=(%d,%d): %w", mapID, targetX, targetY, err)
			}
			tiles, err := applyEventTileOverridesContext(ses.CommandContext(), tx.(db.ReadDBTX), charID, mapID, []PhaserTile{{X: targetX, Y: targetY, TileImageID: state.TileImageID, CollisionType: state.CollisionType, RawFootTileID: state.RawFootTileID, TalkOverTile: state.TalkOverTile}})
			if err != nil {
				return err
			}
			if tiles[0].RawFootTileID == nil || *tiles[0].RawFootTileID != cuttableTreeRawFootTileID {
				return &itemuse.Rejection{Message: "There isn't anything to CUT."}
			}
			permission, err = fieldMovePermissionIn(tx, charID, "CUT", func(flag string) (bool, error) { return queryEventFlag(tx, charID, flag) })
			if err != nil {
				return err
			}
			if !permission.Allowed {
				return &itemuse.Rejection{Message: permission.Message}
			}
			replacement, err = cutTreeReplacementIn(tx, tiles[0].TileImageID)
			return err
		}()
		if prepareErr != nil {
			return nil, prepareErr
		}
		message := "Used CUT!"
		if permission.KnownByName != "" {
			message = permission.KnownByName + " used CUT!"
		}
		return json.Marshal(CutFieldResponse{StateFieldResponse: StateFieldResponse{Success: true, RequestID: req.RequestID, CharacterID: charID, MoveName: "CUT", MapID: mapID, Message: message}, CutScope: cutScope, TargetX: targetX, TargetY: targetY})
	})
	if err != nil {
		var denied *itemuse.Rejection
		if errors.As(err, &denied) {
			reject(denied.Message)
		} else {
			logutil.Debugf("[CUT] Prepare character=%d: %v", charID, err)
			reject("Unable to read CUT state.")
		}
		return false
	}

	var reply CutFieldResponse
	if err := json.Unmarshal(encoded, &reply); err != nil || !reply.Success || reply.CharacterID != charID || reply.RequestID != req.RequestID || reply.CutScope != cutScope || reply.MapID != *req.MapID || reply.TargetX != *req.TargetX || reply.TargetY != *req.TargetY || reply.MoveName != "CUT" || reply.Message == "" {
		reject("Invalid committed CUT result.")
		return false
	}
	if replay {
		if !wh.CutTiles.HasCut(charID, reply.MapID, cutScope, reply.TargetX, reply.TargetY) {
			reject("CUT visit no longer contains this outcome. Refresh the current map.")
			return false
		}
	} else {
		replacement.X, replacement.Y = targetX, targetY
		if !wh.CutTiles.MarkCut(charID, mapID, cutScope, replacement) {
			reject("CUT source retired. Refresh the current map.")
			return false
		}
	}
	ses.SendStreamJSON(reply, opcodes.FieldMoveUseResponse)
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
	return handleTravelFieldMove(ses, wh, "DIG")
}
func handleTeleportFieldMove(ses *session.Session, req FieldMoveUseRequestPayload, wh *WorldHandler) bool {
	return handleTravelFieldMove(ses, wh, "TELEPORT")
}

func handleTravelFieldMove(ses *session.Session, wh *WorldHandler, moveName string) bool {
	if wh == nil || wh.database == nil {
		sendFieldMoveUseError(ses, "Field travel unavailable.")
		return false
	}
	source := wh.ownedPlayerSnapshot(ses, "")
	result, err := useTravelFieldMove(ses.CommandContext(), wh.database, int64(ses.Client.CharData().ID), moveName, source.MapID, source.X, source.Y, func(mapID int) int { return normalizedVisiblePlayerMapID(wh, mapID) })
	if err != nil {
		var denied *itemuse.Rejection
		if errors.As(err, &denied) {
			sendFieldMoveUseError(ses, denied.Message)
		} else {
			logutil.Debugf("[FieldTravel] character=%d move=%s: %v", ses.Client.CharData().ID, moveName, err)
			sendFieldMoveUseError(ses, "Could not read or save field travel. Please try again.")
		}
		return false
	}
	publishCommittedTeleport(ses, wh, result.MapID, result.X, result.Y)
	message := "Dug out of the dungeon."
	if moveName == "TELEPORT" {
		message = "Teleported to safety."
	}
	if result.Permission.KnownByName != "" {
		message = result.Permission.KnownByName + " used " + moveName + "!"
	}
	ses.SendStreamJSON(FieldMoveUseResponsePayload{Success: true, Message: message, MoveName: moveName, MapID: result.MapID, TargetX: result.X, TargetY: result.Y}, opcodes.FieldMoveUseResponse)
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

// This retains the existing single-square approximation. Native CUT replaces
// blocks via CutTreeBlockSwaps; placed block provenance is not yet imported.
// Numeric image IDs are catalog-local, so missing metadata cannot use tile 25.
func cutTreeReplacementIn(tx db.DBTX, tileImageID int) (TileEdit, error) {
	if err := db.RequireTransaction(tx); err != nil {
		return TileEdit{}, err
	}
	var id, raw int
	var talkOver bool
	err := tx.QueryRow(`SELECT replacement.id,replacement.raw_foot_tile_id,replacement.talk_over_tile FROM phaser_tile_images source JOIN phaser_tile_images replacement ON replacement.tileset_id=source.tileset_id AND replacement.raw_foot_tile_id=$2 WHERE source.id=$1 ORDER BY replacement.id LIMIT 1`, tileImageID, cutTreeReplacementRawFoot).Scan(&id, &raw, &talkOver)
	if err != nil {
		return TileEdit{}, fmt.Errorf("CUT source image %d replacement foot %d: %w", tileImageID, cutTreeReplacementRawFoot, err)
	}
	// CutTileManager authorizes cleared ground as land. Palette preferences must
	// not make the client disagree with that transient gameplay collision rule.
	return TileEdit{TileImageID: id, CollisionType: collisionLand, RawFootTileID: &raw, TalkOverTile: talkOver}, nil
}

func eventFlagsForFieldMove(wh *WorldHandler) *EventFlagManager {
	if wh == nil {
		return nil
	}
	return wh.EventFlags
}

func sendFieldMoveUseError(ses *session.Session, message string) {
	ses.SendStreamJSON(FieldMoveUseResponsePayload{
		Success: false,
		Error:   message,
	}, opcodes.FieldMoveUseResponse)
}
