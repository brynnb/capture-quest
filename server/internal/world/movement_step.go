package world

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"capturequest/internal/db"
	"capturequest/internal/session"
)

// A candidate is resolved by an owned command or timer, never from client X/Y.
// Surf entry preserves its wild-only effect policy. Walking and forced paths
// resolve spin/current rules; only forced routes resolve automatic warp continuation.
// Ordinary warps use 183/184.
type movementStepCandidate struct {
	StepToken                          string
	RemainingPath                      []PathNode
	Surfing                            bool
	SourceMap, SourceX, SourceY        int
	MapID, X, Y                        int
	Direction                          string
	SurfEntry, Forced, PathDestination bool
}

type movementStepResult struct {
	MapID, X, Y                      int
	Direction                        string
	Trainer                          *pendingEncounter
	Cutscene                         *durableCutscene
	Safari                           safariStepResult
	Wild                             wildStepResult
	ForcedPath                       []PathNode
	StopPath, Teleport, FlagsChanged bool
	SafariEntryBlocked               bool
	Flags                            map[string]bool
}

// Position, durable counters, selected battle and forced destinations have one
// commit. Returned presentation plans stay private until the outer commit.
func commitMovementStep(ctx context.Context, wh *WorldHandler, charID int64, c movementStepCandidate) (movementStepResult, error) {
	result := movementStepResult{MapID: c.MapID, X: c.X, Y: c.Y, Direction: c.Direction}
	var rejection error
	err := db.Transaction(ctx, wh.database, func(tx db.DBTX) (err error) {
		if err := db.LockCharacter(tx, charID); err != nil {
			return err
		}
		// Cached availability is only an early hint. Durable battle ownership
		// fences all position/effect writers under the same character lock.
		if err := requireNoOwnedBattleIn(tx, charID); err != nil {
			if errors.Is(err, errBattleOwnership) {
				rejection = err
				return saveMovementRouteIn(tx, charID, 0, 0, 0, nil, false)
			}
			return err
		}
		var trainerPending bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM character_trainer_encounters WHERE character_id=$1 AND resolution='pending') OR EXISTS(SELECT 1 FROM character_cutscene_plans WHERE character_id=$1 AND resolution='pending')`, charID).Scan(&trainerPending); err != nil {
			return err
		}
		if trainerPending {
			return fmt.Errorf("gameplay presentation is pending")
		}
		var sourceMap, sourceX, sourceY int
		if err := tx.QueryRow(`SELECT map_id,x,y FROM character_data WHERE id=$1`, charID).Scan(&sourceMap, &sourceX, &sourceY); err != nil {
			return err
		}
		sourceMap = normalizedVisiblePlayerMapID(wh, sourceMap)
		if sourceMap != c.SourceMap || sourceX != c.SourceX || sourceY != c.SourceY {
			return fmt.Errorf("movement saved source changed: map=%d x=%d y=%d", sourceMap, sourceX, sourceY)
		}
		if err := validateClientDestinationIn(tx, c.MapID, c.X, c.Y); err != nil {
			return err
		}
		route, err := loadMovementRouteIn(tx, charID)
		if err != nil {
			return err
		}
		remaining := c.RemainingPath
		if route != nil {
			if !c.Forced || route.MapID != c.SourceMap || route.X != c.SourceX || route.Y != c.SourceY || route.Path[0].X != c.X || route.Path[0].Y != c.Y {
				return fmt.Errorf("movement route ownership changed")
			}
			remaining = route.Path[1:]
		}
		if err := saveFieldDestinationIn(tx, charID, c.MapID, c.X, c.Y); err != nil {
			return err
		}
		flags, err := eventFlagSnapshotIn(tx, charID)
		if err != nil {
			return err
		}
		result.Flags = flags.flags[charID]
		defer func() {
			if err == nil {
				path := remaining
				if !c.Forced || result.StopPath || result.Teleport {
					path = nil
				}
				if len(result.ForcedPath) > 0 {
					path = result.ForcedPath
				}
				surfing := false
				if c.Surfing {
					surfing, err = isSurfableWaterTileIn(ctx, tx.(db.ContextDBTX), wh, result.MapID, result.X, result.Y)
				}
				if err == nil {
					err = saveMovementRouteIn(tx, charID, result.MapID, result.X, result.Y, path, surfing)
				}
			}
			if err == nil && result.FlagsChanged {
				var final *EventFlagManager
				final, err = eventFlagSnapshotIn(tx, charID)
				if err == nil {
					result.Flags = final.flags[charID]
				}
			}
			if err == nil && c.StepToken != "" {
				err = saveMovementReceiptIn(tx, charID, c.StepToken, result)
			}
		}()
		if c.SurfEntry {
			permission, err := fieldMovePermissionIn(tx, charID, "SURF", func(flag string) (bool, error) { return queryEventFlag(tx, charID, flag) })
			if err != nil {
				return err
			}
			if !permission.Allowed {
				return fmt.Errorf("SURF is unavailable: %s", permission.Message)
			}
			if c.MapID != c.SourceMap || abs(c.X-c.SourceX)+abs(c.Y-c.SourceY) != 1 {
				return fmt.Errorf("SURF target is not adjacent")
			}
			collision, _, err := wh.ActorManager.characterCollision(ctx, tx.(db.ReadDBTX), charID, c.MapID, c.SourceX, c.SourceY, flags)
			if err != nil {
				return err
			}
			if collision[tileKey(c.X, c.Y)] != collisionWater {
				return fmt.Errorf("SURF target is not available water")
			}
			if wh.phaserWarps != nil && wh.phaserWarps.warpAt(c.MapID, c.X, c.Y) != nil {
				return fmt.Errorf("SURF target is a warp")
			}
			if wh.Cutscenes != nil && SeafoamSurfBlocked(charID, wh.Cutscenes.MapNameForID(c.MapID), c.X, c.Y, flags) {
				return fmt.Errorf("SURF current blocks entry")
			}
		}
		if !c.SurfEntry {
			if wh.TrainerEncounter != nil {
				var trainer *trainerSightData
				trainer, err = wh.TrainerEncounter.planPositionEncounter(ctx, tx, charID, c.X, c.Y, c.MapID, flags)
				if err != nil {
					return err
				}
				if trainer != nil {
					result.Trainer, err = savePendingTrainerIn(tx, charID, c.MapID, c.X, c.Y, trainer)
					if err != nil {
						return err
					}
					result.StopPath = true
					return nil
				}
			}
			if _, _, err := advanceDayCareStepsIn(tx, charID, 1); err != nil {
				return err
			}
			if wh.Safari != nil && IsInSafariZone(c.MapID) {
				result.Safari, err = prepareSafariStepIn(tx, charID, c.X, c.Y, c.MapID, wh)
				if err != nil {
					return err
				}
				result.StopPath = result.Safari.Expired || (result.Safari.Visit != nil && result.Safari.Visit.Battle != nil)
				if result.Safari.Expired {
					result.MapID, result.X, result.Y, result.Direction = SafariZoneGateMapID, SafariZoneGateReturnX, SafariZoneGateReturnY, "DOWN"
					result.Teleport = true
					result.FlagsChanged = true
				}
				return nil
			}
			if wh.CoordTriggers != nil && wh.Cutscenes != nil && wh.EventFlags != nil {
				for _, trigger := range wh.CoordTriggers.CheckTileTriggers(c.MapID, c.X, c.Y) {
					script, err := wh.Cutscenes.findEligibleCoordCutsceneIn(tx, trigger, charID, flags, c.Direction)
					if err != nil {
						return err
					}
					if script != nil {
						result.Cutscene, err = issueCutsceneIn(tx, charID, issuedCutscene{Script: *script, MapID: c.MapID, X: c.X, Y: c.Y})
						if err != nil {
							return err
						}
						result.StopPath = true
						return nil
					}
				}
			}
		}
		suppressed := false
		if wh.Cutscenes != nil && wh.EventFlags != nil {
			name := wh.Cutscenes.MapNameForID(c.MapID)
			if strings.EqualFold(name, "POKEMON_TOWER_5F") {
				suppressed = isPokemonTower5FPurifiedZone(name, c.X, c.Y)
				if !suppressed && flags.CheckFlag(charID, "EVENT_IN_PURIFIED_ZONE") {
					if err := writeEventFlag(tx, charID, "EVENT_IN_PURIFIED_ZONE", false); err != nil {
						return err
					}
					result.FlagsChanged = true
				}
			}
		}
		testSuppressed := os.Getenv("CAPTUREQUEST_TEST_MODE") == "true" && os.Getenv("CAPTUREQUEST_TEST_RANDOM_ENCOUNTERS") != "true"
		if wh.WildEncounter != nil && !suppressed && (!testSuppressed || c.SurfEntry) {
			result.Wild, err = wh.WildEncounter.prepareWildStepIn(ctx, tx, charID, c.X, c.Y, c.MapID)
			if err != nil {
				return err
			}
			if result.Wild.Battle != nil {
				result.StopPath = true
				return nil
			}
			if b := result.Wild.Blackout; b != nil {
				result.MapID, result.X, result.Y, result.Direction = normalizedVisiblePlayerMapID(wh, b.MapID), b.X, b.Y, "DOWN"
				result.StopPath = true
				result.Teleport = true
				result.FlagsChanged = true
				return nil
			}
		}
		if c.SurfEntry {
			return nil
		}
		name := ""
		if wh.Cutscenes != nil {
			name = wh.Cutscenes.MapNameForID(c.MapID)
		}
		if wh.SpinTiles != nil && name != "" {
			if spin := wh.SpinTiles.CheckTile(name, c.X, c.Y); spin != nil {
				result.ForcedPath = SeafoamCurrentPath(c.X, c.Y, spin.Movements)
			}
		}
		if wh.EventFlags != nil && name != "" {
			if current, ok := SeafoamCurrentAt(charID, name, c.X, c.Y, flags); ok {
				result.ForcedPath = SeafoamCurrentPath(c.X, c.Y, current.Movements)
				return nil
			}
		}
		if c.Forced && c.PathDestination && wh.WarpTiles != nil {
			if warp := wh.WarpTiles.CheckTile(c.MapID, c.X, c.Y); warp != nil {
				if c.MapID == SafariZoneGateMapID && IsInSafariZone(warp.DestMapID) {
					visit, err := safariSessionIn(tx, charID)
					if err != nil {
						return err
					}
					if visit == nil || !visit.Active {
						result.SafariEntryBlocked = true
						result.StopPath = true
						return nil
					}
				}
				result.MapID = normalizedVisiblePlayerMapID(wh, warp.DestMapID)
				result.X, result.Y, result.Direction = warp.DestX, warp.DestY, "DOWN"
				if err := validateClientDestinationIn(tx, result.MapID, result.X, result.Y); err != nil {
					return err
				}
				if err := saveFieldDestinationIn(tx, charID, result.MapID, result.X, result.Y); err != nil {
					return err
				}
				result.Teleport = true
				result.StopPath = true
				result.FlagsChanged = true
			}
		}
		return nil
	})
	if err != nil {
		return movementStepResult{}, err
	}
	if rejection != nil {
		return movementStepResult{}, rejection
	}
	return result, nil
}

// This function contains publication only. It cannot create transactions or
// replay effects after an unknown outcome. The owning session gate is held.
func publishMovementStepEffects(ses *session.Session, wh *WorldHandler, charID int64, result movementStepResult) {
	if wh.EventFlags != nil && result.Flags != nil {
		wh.EventFlags.publishCommittedFlags(charID, result.Flags)
	}
	if result.SafariEntryBlocked {
		SendSystemMessage(ses, "Please check in at the counter first.")
	}
	if result.Trainer != nil {
		wh.TrainerEncounter.publishPositionEncounter(result.Trainer, ses)
		return
	}
	if result.Cutscene != nil {
		publishCutscenePlan(ses, result.Cutscene, wh)
		return
	}
	if result.Safari.Visit != nil {
		publishSafariStep(ses, wh, charID, result.Safari)
		return
	}
	if wh.WildEncounter != nil {
		wh.WildEncounter.publishWildStep(ses, charID, result.Wild)
	}
	if result.Teleport && result.Wild.Blackout == nil {
		sendCommittedWarpNotification(ses, result.MapID, result.X, result.Y, result.Direction)
	}
}
