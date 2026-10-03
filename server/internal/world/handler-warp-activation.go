package world

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

// Normal warps accept source identity and activation intent, never destinations.
func HandlePhaserWarpActivateRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserWarpActivateRequest
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	if err == nil && decoder.Decode(new(any)) != io.EOF {
		err = fmt.Errorf("trailing warp request")
	}
	fail := func(err error) {
		log.Printf("[Warp] Character %d activation: %v", ses.Client.CharData().ID, err)
		ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Could not activate this warp. Please try again."}, opcodes.PhaserWarpActivateResponse)
	}
	if err != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.WarpID <= 0 || (req.InputSource != "click" && req.InputSource != "keyboard") || normalizeWarpDirection(req.Direction) == "" {
		fail(fmt.Errorf("invalid warp request: %v", err))
		return false
	}
	x, y, mapID := wh.ownedPlayerPosition(ses)
	charID := int64(ses.Client.CharData().ID)
	if getBattle(charID) != nil {
		fail(fmt.Errorf("normal warp unavailable during battle"))
		return false
	}
	result, err := commitNormalWarp(ses.CommandContext(), wh.database, charID, req, mapID, x, y, ses.PreviousMapID, wh.ActorManager, wh.EventFlags != nil)
	if err != nil {
		fail(err)
		return false
	}
	if wh.EventFlags != nil {
		if err := wh.EventFlags.LoadFlagsContext(ses.CommandContext(), charID); err != nil {
			log.Printf("[Warp] Refresh committed flags for %d: %v", charID, err)
		}
	}
	publishCommittedPlayerPosition(ses, wh, result.PlayerMapID, result.X, result.Y, result.Direction)
	result.RequestID = req.RequestID
	ses.SendStreamJSON(result, opcodes.PhaserWarpActivateResponse)
	return false
}

func commitNormalWarp(ctx context.Context, database db.DBTX, charID int64, req protocol.PhaserWarpActivateRequest, mapID, x, y, previousMapID int, actorManager *PhaserActorManager, applyEffects bool) (protocol.PhaserWarpActivateResponse, error) {
	var result protocol.PhaserWarpActivateResponse
	err := db.Transaction(ctx, database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		var catalog PhaserWarp
		err := tx.QueryRow(`SELECT id,source_map_id,x,y,destination_map_id,destination_map,destination_x,destination_y,
   COALESCE(destination_kind,''),COALESCE(destination_warp_id,0),COALESCE(warp_type,'door'),COALESCE(warp_direction,'')
   FROM phaser_warps WHERE id=$1`, req.WarpID).Scan(&catalog.ID, &catalog.SourceMapID, &catalog.X, &catalog.Y, &catalog.DestinationMapID, &catalog.DestinationMap, &catalog.DestinationX, &catalog.DestinationY, &catalog.DestinationKind, &catalog.DestinationWarpID, &catalog.WarpType, &catalog.WarpDirection)
		if err != nil {
			return err
		}
		if catalog.WarpType == "inactive" || catalog.WarpType == "elevator" {
			return fmt.Errorf("warp %d is not a normal warp", req.WarpID)
		}
		catalog, err = resolvePhaserWarpForPlayer(tx, previousMapID, catalog)
		if err != nil {
			return err
		}
		if catalog.DestinationMapID == nil || catalog.DestinationX == nil || catalog.DestinationY == nil {
			return fmt.Errorf("warp %d has incomplete destination", req.WarpID)
		}
		var sourceOverworld, destOverworld int
		if err := tx.QueryRow(`SELECT is_overworld FROM phaser_maps WHERE id=$1`, catalog.SourceMapID).Scan(&sourceOverworld); err != nil {
			return err
		}
		if err := tx.QueryRow(`SELECT is_overworld FROM phaser_maps WHERE id=$1`, *catalog.DestinationMapID).Scan(&destOverworld); err != nil {
			return err
		}
		// Keep source map identity for eligibility, including the unified overworld view.
		eligibilityMap := mapID
		if mapID == UnifiedOverworldMapID && sourceOverworld != 0 {
			eligibilityMap = catalog.SourceMapID
		}
		warpDirection := ""
		if catalog.WarpDirection != nil {
			warpDirection = *catalog.WarpDirection
		}
		warp := phaserMapWarp{ID: catalog.ID, SourceMapID: catalog.SourceMapID, X: catalog.X, Y: catalog.Y, WarpType: catalog.WarpType, WarpDirection: warpDirection}
		allowed := warp.canActivateByClick(eligibilityMap, x, y, actorManager)
		direction := normalizeWarpDirection(req.Direction)
		if req.InputSource == "keyboard" {
			allowed = allowed && ((x == warp.X && y == warp.Y) || warp.canActivateByDirection(eligibilityMap, x, y, direction, actorManager))
			if required := normalizeWarpDirection(warp.WarpDirection); required != "" && required != direction {
				allowed = false
			}
		}
		if !allowed {
			return fmt.Errorf("warp %d is not reachable from %d (%d,%d)", req.WarpID, mapID, x, y)
		}
		if catalog.SourceMapID == SafariZoneGateMapID && IsInSafariZone(*catalog.DestinationMapID) {
			visit, err := safariSessionIn(tx, charID)
			if err != nil {
				return err
			}
			if visit == nil || !visit.Active {
				return fmt.Errorf("Safari entry requires an active visit")
			}
		}
		direction = warp.activationFacingDirection(mapID, x, y, direction, actorManager)
		result = protocol.PhaserWarpActivateResponse{Success: true, MapID: *catalog.DestinationMapID, PlayerMapID: *catalog.DestinationMapID, X: *catalog.DestinationX, Y: *catalog.DestinationY, Direction: direction}
		if destOverworld != 0 {
			result.PlayerMapID = UnifiedOverworldMapID
		}
		// Preserve the shipped building-exit step; it is committed here rather than
		// added to a browser-provided destination. Animation starts at the source exit.
		if destOverworld != 0 && sourceOverworld == 0 && direction == "DOWN" {
			startX, startY := result.X, result.Y
			result.AnimationStartX, result.AnimationStartY = &startX, &startY
			result.AnimateExitStep = true
			result.Y++
		}
		_, err = commitMapLoad(ctx, tx, charID, mapLoadArrival{MapID: result.PlayerMapID, X: result.X, Y: result.Y, ValidateCatalog: true, ApplyEffects: applyEffects})
		return err
	})
	if err != nil {
		return protocol.PhaserWarpActivateResponse{}, err
	}
	return result, nil
}

// Instant Warp deliberately permits arbitrary catalog tiles for ordinary players.
// Keep that policy separate from normal warp source reach and walking reports.
func HandlePhaserInstantWarpRequest(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	var req protocol.PhaserInstantWarpRequest
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&req)
	if err == nil && decoder.Decode(new(any)) != io.EOF {
		err = fmt.Errorf("trailing Instant Warp request")
	}
	fail := func(err error) {
		log.Printf("[Warp] Character %d Instant Warp: %v", ses.Client.CharData().ID, err)
		ses.SendStreamJSON(protocol.PhaserMapRequestError{RequestID: req.RequestID, Error: "Could not warp to that tile. Please try again."}, opcodes.PhaserInstantWarpResponse)
	}
	if err != nil || req.RequestID == "" || len(req.RequestID) > 64 || req.MapID <= 0 || req.X == nil || req.Y == nil || normalizeWarpDirection(req.Direction) == "" {
		fail(fmt.Errorf("invalid Instant Warp request: %v", err))
		return false
	}
	charID := int64(ses.Client.CharData().ID)
	if getBattle(charID) != nil {
		fail(fmt.Errorf("Instant Warp unavailable during battle"))
		return false
	}
	mapID := req.MapID
	err = db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if mapID != UnifiedOverworldMapID {
			var isOverworld int
			if err := tx.QueryRow(`SELECT is_overworld FROM phaser_maps WHERE id=$1`, mapID).Scan(&isOverworld); err != nil {
				return err
			}
			if isOverworld != 0 {
				mapID = UnifiedOverworldMapID
			}
		}
		_, err := commitMapLoad(ses.CommandContext(), tx, charID, mapLoadArrival{MapID: mapID, X: *req.X, Y: *req.Y, ValidateCatalog: true, ApplyEffects: wh.EventFlags != nil})
		return err
	})
	if err != nil {
		fail(err)
		return false
	}
	if wh.EventFlags != nil {
		if err := wh.EventFlags.LoadFlagsContext(ses.CommandContext(), charID); err != nil {
			log.Printf("[Warp] Refresh committed Instant Warp flags for %d: %v", charID, err)
		}
	}
	direction := normalizeWarpDirection(req.Direction)
	publishCommittedPlayerPosition(ses, wh, mapID, *req.X, *req.Y, direction)
	ses.SendStreamJSON(protocol.PhaserInstantWarpResponse{Success: true, RequestID: req.RequestID, MapID: mapID, X: *req.X, Y: *req.Y, Direction: direction}, opcodes.PhaserInstantWarpResponse)
	return false
}
