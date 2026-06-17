package world

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"capturequest/internal/db"
)

func TryPushBoulderFromFacingAttempt(charID int64, mapID, playerX, playerY int, direction string, activateStrength bool, efm *EventFlagManager) (BoulderPushResult, bool, error) {
	if normalizeBoulderDirection(direction) == "" {
		return BoulderPushResult{}, false, nil
	}
	result, err := TryPushBoulder(charID, mapID, playerX, playerY, direction, activateStrength, efm)
	if err == nil && result.Message == boulderNoBoulderMessage {
		return result, false, nil
	}
	return result, true, err
}

func TryPushBoulderFromDestinationAttempt(charID int64, mapID, playerX, playerY, destX, destY int, activateStrength bool, efm *EventFlagManager) (BoulderPushResult, bool, error) {
	direction := directionFromAdjacentDestination(destX-playerX, destY-playerY)
	if direction == "" {
		return BoulderPushResult{}, false, nil
	}
	return TryPushBoulderFromFacingAttempt(charID, mapID, playerX, playerY, direction, activateStrength, efm)
}

func directionFromAdjacentDestination(dx, dy int) string {
	switch {
	case dx == 0 && dy == -1:
		return "UP"
	case dx == 0 && dy == 1:
		return "DOWN"
	case dx == -1 && dy == 0:
		return "LEFT"
	case dx == 1 && dy == 0:
		return "RIGHT"
	default:
		return ""
	}
}

func (m *PlayerMovementManager) tryPushBoulderFromFacingAttempt(charID int, mapID, playerX, playerY int, direction string) (BoulderPushResult, bool) {
	if m.wh == nil || m.wh.EventFlags == nil {
		return BoulderPushResult{}, false
	}
	result, attempted, err := TryPushBoulderFromFacingAttempt(int64(charID), mapID, playerX, playerY, direction, true, m.wh.EventFlags)
	if err != nil {
		log.Printf("[PlayerMovement] Boulder push from facing attempt failed for player %d: %v", charID, err)
		return result, attempted
	}
	if attempted && result.Success {
		m.broadcastBoulderPushResult(int64(charID), result)
	}
	return result, attempted
}

func (m *PlayerMovementManager) queueStepAfterBoulderPush(charID, startX, startY, mapID int, result BoulderPushResult) {
	if !result.Success {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.players[charID]
	if !ok {
		return
	}
	if state.MapID != mapID || state.CurrentX != startX || state.CurrentY != startY {
		return
	}

	state.Direction = result.Direction
	state.Path = []PathNode{{X: result.FromX, Y: result.FromY}}
	state.LastMoveTime = time.Now().Add(-state.MoveSpeed)
}

func (m *PlayerMovementManager) broadcastBoulderPushResult(charID int64, result BoulderPushResult) {
	if m.wh == nil || m.wh.ActorRegistry == nil || m.actorManager == nil {
		return
	}

	if result.ObjectID > 0 {
		runtimeID := m.wh.ActorRegistry.GetPhaserID(ActorTypeNPC, result.ObjectID)
		if result.Dropped {
			m.actorManager.broadcastActorDespawn(runtimeID, result.MapID)
		} else {
			actor := boulderActorFromPushResult(m.wh.ActorRegistry, result)
			m.actorManager.broadcastActorUpdate(&actor, 0)
		}
	}

	seenMaps := map[int]bool{result.MapID: true}
	for _, mapName := range result.AffectedMaps {
		mapID, err := mapIDForBoulderMapName(mapName)
		if err != nil || seenMaps[mapID] {
			continue
		}
		seenMaps[mapID] = true

		boulders, err := BoulderObjectsForCharacter(charID, mapID, m.wh.EventFlags)
		if err != nil {
			log.Printf("[PlayerMovement] Failed to load affected boulders for map %s: %v", mapName, err)
			continue
		}
		for _, boulder := range boulders {
			if !boulder.Visible {
				continue
			}
			actor := boulderActorFromState(m.wh.ActorRegistry, boulder, result.Direction)
			m.actorManager.broadcastActorUpdate(&actor, 0)
		}
	}
}

func boulderActorFromPushResult(registry *ActorRegistry, result BoulderPushResult) PhaserActor {
	x, y := result.ToX, result.ToY
	name := result.ObjectName
	spriteName := "SPRITE_BOULDER"
	actionType := "STAY"
	movementType := "BOTH"
	direction := result.Direction
	return PhaserActor{
		ID:              registry.GetPhaserID(ActorTypeNPC, result.ObjectID),
		X:               &x,
		Y:               &y,
		MapID:           result.MapID,
		ObjectType:      "npc",
		SpriteName:      &spriteName,
		Name:            &name,
		ActionType:      &actionType,
		ActionDirection: &direction,
		MovementType:    &movementType,
		MoveSpeed:       300,
	}
}

func boulderActorFromState(registry *ActorRegistry, state BoulderObjectState, direction string) PhaserActor {
	x, y := state.X, state.Y
	name := state.Name
	text := state.Text
	spriteName := "SPRITE_BOULDER"
	actionType := "STAY"
	movementType := "BOTH"
	normalizedDirection := normalizeBoulderDirection(direction)
	if normalizedDirection == "" {
		normalizedDirection = "DOWN"
	}
	return PhaserActor{
		ID:              registry.GetPhaserID(ActorTypeNPC, state.ObjectID),
		X:               &x,
		Y:               &y,
		MapID:           state.MapID,
		ObjectType:      "npc",
		SpriteName:      &spriteName,
		Name:            &name,
		ActionType:      &actionType,
		ActionDirection: &normalizedDirection,
		MovementType:    &movementType,
		Text:            &text,
		MoveSpeed:       300,
	}
}

func mapIDForBoulderMapName(mapName string) (int, error) {
	var mapID int
	err := db.GlobalWorldDB.DB.QueryRow(
		`SELECT id FROM phaser_maps WHERE name = $1`,
		strings.TrimSpace(mapName),
	).Scan(&mapID)
	if err == sql.ErrNoRows {
		return 0, err
	}
	return mapID, err
}
