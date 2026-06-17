package world

func isSurfableWaterTile(wh *WorldHandler, mapID, x, y int) bool {
	if wh == nil || wh.ActorManager == nil {
		return false
	}
	collisionType, exists := wh.ActorManager.CollisionTypeAt(mapID, x, y)
	if !exists || collisionType != collisionWater {
		return false
	}
	if wh.phaserWarps != nil && wh.phaserWarps.warpAt(mapID, x, y) != nil {
		return false
	}
	return true
}
