package world

import (
	"encoding/binary"
	"log"
	"time"

	"capturequest/internal/db"
	"capturequest/internal/session"
)

// WorldHandler manages global game message routing.
type WorldHandler struct {
	sessionManager   *session.SessionManager
	globalRegistry   *HandlerRegistry
	ActorManager     *PhaserActorManager       `json:"actorManager,omitempty"`
	PlayerMovement   *PlayerMovementManager    `json:"playerMovement,omitempty"`
	ActorRegistry    *ActorRegistry            `json:"actorRegistry,omitempty"`
	TrainerEncounter *TrainerEncounterManager  `json:"trainerEncounter,omitempty"`
	WildEncounter    *WildEncounterManager     `json:"wildEncounter,omitempty"`
	EventFlags       *EventFlagManager         `json:"eventFlags,omitempty"`
	CoordTriggers    *CoordinateTriggerManager `json:"coordTriggers,omitempty"`
	MapScripts       *MapScriptManager         `json:"mapScripts,omitempty"`
	Cutscenes        *CutsceneManager          `json:"cutscenes,omitempty"`
	SpinTiles        *SpinTileManager          `json:"spinTiles,omitempty"`
	WarpTiles        *WarpTileManager          `json:"warpTiles,omitempty"`
	phaserWarps      *phaserWarpManager
	Safari           *SafariZoneManager `json:"-"`
	CutTiles         *CutTileManager    `json:"-"`
}

// NewWorldHandler creates a new WorldHandler.
func NewWorldHandler(sessionManager *session.SessionManager) *WorldHandler {
	registry := NewWorldOpCodeRegistry()
	wh := &WorldHandler{
		sessionManager: sessionManager,
		globalRegistry: registry,
		ActorManager:   nil, // Will be set below
		PlayerMovement: nil, // Will be set below
		ActorRegistry:  NewActorRegistry(),
		CutTiles:       NewCutTileManager(),
	}
	registry.WH = wh
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.ActorManager.Start()
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.PlayerMovement.Start()
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	wh.TrainerEncounter.Load()
	wh.WildEncounter = NewWildEncounterManager(wh)
	wh.WildEncounter.Load()
	wh.EventFlags = NewEventFlagManager(db.GlobalWorldDB.DB)
	wh.CoordTriggers = NewCoordinateTriggerManager(db.GlobalWorldDB.DB)
	wh.CoordTriggers.Load()
	wh.MapScripts = NewMapScriptManager(db.GlobalWorldDB.DB)
	wh.MapScripts.Load()
	wh.Cutscenes = NewCutsceneManager(db.GlobalWorldDB.DB)
	wh.Cutscenes.Load()
	wh.SpinTiles = NewSpinTileManager(db.GlobalWorldDB.DB)
	wh.SpinTiles.Load()
	wh.WarpTiles = NewWarpTileManager(db.GlobalWorldDB.DB)
	wh.WarpTiles.Load()
	wh.phaserWarps = newPhaserWarpManager(db.GlobalWorldDB.DB)
	wh.phaserWarps.setActorManager(wh.ActorManager)
	wh.phaserWarps.load()
	wh.Safari = NewSafariZoneManager()
	LoadDisallowedWords()
	wh.StartSessionTimeoutChecker()
	return wh
}

// HandlePacket processes incoming datagrams.
// All handlers are now at the world level - no zone routing needed.
func (wh *WorldHandler) HandlePacket(ses *session.Session, data []byte) {
	if len(data) < 2 {
		return
	}

	// All opcodes are handled globally now
	if wh.globalRegistry.ShouldHandleGlobally(data) {
		wh.globalRegistry.HandleWorldPacket(ses, data)
		return
	}

	// Unknown opcode
	if !ses.Authenticated {
		op := binary.LittleEndian.Uint16(data[:2])
		log.Printf("unauthenticated opcode %d from session %d – dropping", op, ses.SessionID)
		return
	}

	op := binary.LittleEndian.Uint16(data[:2])
	log.Printf("unhandled opcode %d from session %d", op, ses.SessionID)
}

// RemoveSession cleans up session data.
func (wh *WorldHandler) RemoveSession(sessionID int) {
	log.Printf("[WORLD] Removing session %d", sessionID)
	// Flush and unregister player from movement manager if they have a character
	if ses, ok := wh.sessionManager.GetSession(sessionID); ok {
		if ses.HasValidClient() {
			char := ses.Client.CharData()
			charID := int(char.ID)
			log.Printf("[WORLD] Flushing position for character %d (%s) from session %d", charID, char.Name, sessionID)
			wh.PlayerMovement.FlushPlayerPosition(charID)
			wh.PlayerMovement.UnregisterPlayer(charID)
			wh.TrainerEncounter.ClearPlayer(int64(charID))
			wh.WildEncounter.ClearPlayer(int64(charID))
			wh.EventFlags.UnloadFlags(int64(charID))
			saveBattleOnDisconnect(int64(charID))

			// Stop the client's regen goroutine to prevent leaks
			ses.Client.Shutdown()

			// Notify other Phaser clients to remove this actor
			phaserID := wh.ActorRegistry.GetPhaserID(ActorTypePlayer, charID)
			log.Printf("[WORLD] Despawning Phaser actor %d for character %s", phaserID, char.Name)
			wh.ActorManager.broadcastActorDespawn(phaserID, ses.MapID)
		}
	}

	wh.sessionManager.RemoveSession(sessionID)
}

// Shutdown is a no-op now - no zone instances to stop.
func (wh *WorldHandler) Shutdown() {
	// No-op: No zone instances to shut down
}

func (wh *WorldHandler) StartSessionTimeoutChecker() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now()
			var timedOutSessions []int

			wh.sessionManager.ForEachSession(func(ses *session.Session) {
				// Only check sessions that have received at least one heartbeat
				if !ses.LastHeartbeat.IsZero() && now.Sub(ses.LastHeartbeat) > 15*time.Second {
					log.Printf("[WORLD] Session %d timed out (last heartbeat: %v)", ses.SessionID, ses.LastHeartbeat)
					timedOutSessions = append(timedOutSessions, ses.SessionID)
				}
			})

			for _, sessionID := range timedOutSessions {
				wh.RemoveSession(sessionID)
			}
		}
	}()
}
