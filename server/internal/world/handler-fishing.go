package world

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/db/cqitems"
	"capturequest/internal/itemuse"
	"capturequest/internal/logutil"
	"capturequest/internal/pokebattle"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
)

type PokeFishingRequestPayload struct {
	CommandRevision *int64 `json:"commandRevision,omitempty" tstype:"number"`
	RequestID       string `json:"requestId"`
	CharacterID     int64  `json:"characterId"`
	InstanceID      int32  `json:"instanceId"`
	ItemID          int32  `json:"itemId"` // Catalog rod item ID; ownership and short name are resolved on the server.
	RodType         string `json:"rodType,omitempty"`
	MapID           *int   `json:"mapId,omitempty"`
	X               *int   `json:"x,omitempty"`
	Y               *int   `json:"y,omitempty"`
	Direction       string `json:"direction,omitempty"`
}

// HandlePokeFishing handles a fishing rod use request from the client.
// The client sends the rod item ID. The server checks if the player is facing
// water, selects an encounter, and starts a wild battle.
func HandlePokeFishing(ses *session.Session, payload []byte, wh *WorldHandler) bool {
	if !ses.HasValidClient() || wh == nil || wh.database == nil {
		return false
	}

	var req PokeFishingRequestPayload
	err := json.Unmarshal(payload, &req)
	charID := int64(ses.Client.CharData().ID)
	identity := protocol.FishingIdentity{RequestID: req.RequestID, CharacterID: charID, InstanceID: req.InstanceID}
	reject := func(message string) {
		ses.SendStreamJSON(protocol.FishingError{FishingIdentity: identity, Error: message}, opcodes.PokeFishingResponse)
	}
	if err != nil || (req.CharacterID != 0 && req.CharacterID != charID) || (req.RequestID != "" && (req.CharacterID != charID || req.InstanceID <= 0 || req.CommandRevision == nil)) {
		reject("Invalid fishing request.")
		return false
	}
	source := wh.ownedPlayerSnapshot(ses, "")
	source.MapID = normalizedVisiblePlayerMapID(wh, source.MapID)
	if wh.PlayerMovement == nil {
		source.Direction = directionFromCharacterHeading(ses.Client.CharData().Heading)
	}
	var battle *pokebattle.BattleState
	var nibble bool
	input, _ := json.Marshal(req)
	var replay bool
	var encoded []byte
	encoded, replay, err = db.ExecuteFieldCommand(ses.CommandContext(), wh.database, charID, "fishing", req.RequestID, req.CommandRevision, input, func(tx db.DBTX) ([]byte, error) {
		applyErr := func() error {
			if source.ServerMovementPending || (req.MapID != nil && *req.MapID != source.MapID) || (req.X != nil && *req.X != source.X) || (req.Y != nil && *req.Y != source.Y) || (req.Direction != "" && normalizeWarpDirection(req.Direction) != source.Direction) {
				return &itemuse.Rejection{Message: "Fishing source changed."}
			}

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
			var owned *cqitems.CQInventoryItem
			var err error
			store := cqitems.NewStore(tx)
			if req.InstanceID > 0 {
				owned, err = store.FindInventoryItemByInstanceID(int32(charID), req.InstanceID)
			} else {
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
				owned, err = store.FindInventoryItemByItemID(int32(charID), id)
			}
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
			if (req.ItemID > 0 && req.ItemID != owned.Item.ID) || rod == "" || (req.RodType != "" && rod != normalizeFishingRodName(req.RodType)) {
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
		}()
		if applyErr != nil {
			return nil, applyErr
		}
		message := "Oh! A bite!"
		if nibble {
			message = "Not even a nibble!"
		}
		return json.Marshal(protocol.FishingResponse{FishingIdentity: identity, Success: true, Hooked: !nibble, Message: message})
	})
	if err != nil {
		logutil.Debugf("[Fishing] Character %d: %v", charID, err)
		message := "Unable to read fishing state."
		var rejection *itemuse.Rejection
		if errors.As(err, &rejection) {
			message = rejection.Message
		}
		reject(message)
		return false
	}
	var outcome protocol.FishingResponse
	if err := json.Unmarshal(encoded, &outcome); err != nil || !outcome.Success || outcome.FishingIdentity != identity {
		reject("Invalid committed fishing receipt.")
		return false
	}
	if !replay && battle != nil {
		setBattle(charID, battle)
	}
	ses.SendStreamJSON(outcome, opcodes.PokeFishingResponse)
	if !replay && battle != nil {
		ses.SendStreamJSON(buildBattleStateResponse(battle), opcodes.PokeBattleStartResponse)
	}

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
