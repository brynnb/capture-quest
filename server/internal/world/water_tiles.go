package world

import (
	"capturequest/internal/db"
	"context"
	"fmt"
)

func isSurfableWaterTile(ctx context.Context, wh *WorldHandler, mapID, x, y int) (bool, error) {
	if wh == nil || wh.ActorManager == nil {
		return false, fmt.Errorf("water collision service unavailable")
	}
	collision, _, err := wh.ActorManager.baseCollision(ctx, wh.database, mapID, true)
	if err != nil {
		return false, err
	}
	value, exists := collision[tileKey(x, y)]
	return surfableWaterCollision(wh, mapID, x, y, value, exists), nil
}

func isSurfableWaterTileIn(ctx context.Context, q db.ContextDBTX, wh *WorldHandler, mapID, x, y int) (bool, error) {
	if wh == nil || wh.ActorManager == nil {
		return false, fmt.Errorf("water collision service unavailable")
	}
	collision, _, err := wh.ActorManager.baseCollision(ctx, q, mapID, false)
	if err != nil {
		return false, err
	}
	value, exists := collision[tileKey(x, y)]
	return surfableWaterCollision(wh, mapID, x, y, value, exists), nil
}

func surfableWaterCollision(wh *WorldHandler, mapID, x, y, collisionType int, exists bool) bool {
	if !exists || collisionType != collisionWater {
		return false
	}
	if wh.phaserWarps != nil && wh.phaserWarps.warpAt(mapID, x, y) != nil {
		return false
	}
	return true
}
