package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/logutil"
	"capturequest/internal/pokebattle"
	"capturequest/internal/session"
)

type PokeFishingRequestPayload struct {
	ItemID    int32  `json:"itemId"` // Catalog rod item ID; ownership and short name are resolved on the server.
	RodType   string `json:"rodType,omitempty"`
	MapID     *int   `json:"mapId,omitempty"`
	X         *int   `json:"x,omitempty"`
	Y         *int   `json:"y,omitempty"`
	Direction string `json:"direction,omitempty"`
}

// HandlePokeFishing handles a fishing rod use request from the client.
// The client sends the rod item ID. The server checks if the player is facing
// water, selects an encounter, and starts a wild battle.
func HandlePokeFishing(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() || wh == nil || wh.database == nil {
		return false
	}

	var req PokeFishingRequestPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		log.Printf("[Fishing] Failed to unmarshal fishing request: %v", err)
		return false
	}

	charID := int64(ses.Client.CharData().ID)
	source := wh.ownedPlayerSnapshot(ses, "")
	source.MapID = normalizedVisiblePlayerMapID(wh, source.MapID)
	if wh.PlayerMovement == nil {
		source.Direction = directionFromCharacterHeading(ses.Client.CharData().Heading)
	}
	if source.ServerMovementPending || (req.MapID != nil && *req.MapID != source.MapID) || (req.X != nil && *req.X != source.X) || (req.Y != nil && *req.Y != source.Y) || (req.Direction != "" && normalizeWarpDirection(req.Direction) != source.Direction) {
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": "Fishing source changed."}, opcodes.PokeFishingResponse)
		return false
	}
	var battle *pokebattle.BattleState
	var nibble bool
	err := db.Transaction(ses.CommandContext(), wh.database, func(tx db.DBTX) error {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		if err := requireNoOwnedBattleIn(tx, charID); err != nil {
			return err
		}
		var mapID, x, y int
		if err := tx.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=$1`, charID).Scan(&mapID, &x, &y); err != nil {
			return err
		}
		if normalizedVisiblePlayerMapID(wh, mapID) != source.MapID || x != source.X || y != source.Y {
			return fmt.Errorf("fishing source changed")
		}
		if wh.PlayerMovement != nil {
			wh.PlayerMovement.mu.Lock()
			state := wh.PlayerMovement.players[int(charID)]
			valid := state != nil && state.SessionID == ses.SessionID && state.MapID == source.MapID && state.CurrentX == x && state.CurrentY == y && state.Direction == source.Direction && len(state.Path) == 0 && state.activePlayerStep(time.Now()) == nil
			wh.PlayerMovement.mu.Unlock()
			if !valid {
				return fmt.Errorf("fishing movement owner changed")
			}
		}
		id := req.ItemID
		if id <= 0 {
			name := normalizeFishingRodName(req.RodType)
			if name == "" {
				return fmt.Errorf("not a fishing rod")
			}
			if err := tx.QueryRow(`SELECT id FROM cq_items WHERE short_name=$1`, strings.ToUpper(name)).Scan(&id); err != nil {
				return err
			}
		}
		owned, err := cqitems.NewStore(tx).FindInventoryItemByItemID(int32(charID), id)
		if errors.Is(err, sql.ErrNoRows) {
			return &itemuse.Rejection{Message: "You don't own that fishing rod."}
		}
		if err != nil {
			return fmt.Errorf("owned fishing rod: %w", err)
		}
		if owned == nil {
			return fmt.Errorf("fishing rod not owned")
		}
		rod := normalizeFishingRodName(owned.Item.ShortName)
		if rod == "" || (req.RodType != "" && rod != normalizeFishingRodName(req.RodType)) {
			return fmt.Errorf("fishing rod identity disagrees with catalog")
		}
		targetX, targetY, ok := fishingTargetTile(source.X, source.Y, source.Direction)
		if !ok {
			return fmt.Errorf("invalid fishing facing")
		}
		water, err := isSurfableWaterTileIn(ses.CommandContext(), tx.(db.ContextDBTX), wh, source.MapID, targetX, targetY)
		if err != nil {
			return err
		}
		if !water {
			return &itemuse.Rejection{Message: "You can't fish here."}
		}
		if rod != "old_rod" && rand.Intn(2) == 0 {
			nibble = true
			return nil
		}
		pokemonID, level, err := pokebattle.SelectFishingEncounter(tx, source.MapID, rod)
		if errors.Is(err, pokebattle.ErrNoEncounter) {
			nibble = true
			return nil
		}
		if err != nil {
			return err
		}
		battle, _, err = prepareScriptedWildBattle(tx, charID, ScriptedWildBattleSpec{PokemonID: pokemonID, Level: level})
		return err
	})
	if err != nil {
		logutil.Debugf("[Fishing] Character %d: %v", charID, err)
		message := "Unable to read fishing state."
		var rejection *itemuse.Rejection
		if errors.As(err, &rejection) {
			message = rejection.Message
		}
		ses.SendStreamJSON(map[string]interface{}{"success": false, "error": message}, opcodes.PokeFishingResponse)
		return false
	}
	if nibble {
		ses.SendStreamJSON(map[string]interface{}{"success": true, "hooked": false, "message": "Not even a nibble!"}, opcodes.PokeFishingResponse)
		return false
	}
	setBattle(charID, battle)
	ses.SendStreamJSON(map[string]interface{}{"success": true, "hooked": true, "message": "Oh! A bite!"}, opcodes.PokeFishingResponse)
	ses.SendStreamJSON(buildBattleStateResponse(battle), opcodes.PokeBattleStartResponse)
	return false
}

func normalizeFishingRodName(name string) string {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "OLD_ROD", "OLD ROD", "OLD":
		return "old_rod"
	case "GOOD_ROD", "GOOD ROD", "GOOD":
		return "good_rod"
	case "SUPER_ROD", "SUPER ROD", "SUPER":
		return "super_rod"
	}
	return ""
}

func isFacingFishableWater(ctx context.Context, wh *WorldHandler, mapID, playerX, playerY int, direction string) (bool, error) {
	targetX, targetY, ok := fishingTargetTile(playerX, playerY, direction)
	if !ok {
		return false, nil
	}
	return isSurfableWaterTile(ctx, wh, mapID, targetX, targetY)
}

func fishingTargetTile(playerX, playerY int, direction string) (int, int, bool) {
	switch normalizeWarpDirection(direction) {
	case "UP":
		return playerX, playerY - 1, true
	case "DOWN":
		return playerX, playerY + 1, true
	case "LEFT":
		return playerX - 1, playerY, true
	case "RIGHT":
		return playerX + 1, playerY, true
	default:
		return playerX, playerY, false
	}
}

func directionFromCharacterHeading(heading float64) string {
	normalized := int(heading) % 360
	if normalized < 0 {
		normalized += 360
	}
	switch normalized {
	case 90:
		return "RIGHT"
	case 180:
		return "DOWN"
	case 270:
		return "LEFT"
	default:
		return "UP"
	}
}
