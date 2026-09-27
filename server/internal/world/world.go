package world

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"capturequest/internal/db"
	db_character "capturequest/internal/db/character"
	"capturequest/internal/economy"
	"capturequest/internal/itemuse"
	"capturequest/internal/session"
)

const playtimeFlushInterval = time.Minute

// WorldHandler manages global game message routing.
type WorldHandler struct {
	database         *sql.DB
	Items            *itemuse.Service `json:"-"`
	Economy          *economy.Service `json:"-"`
	sessionManager   *session.SessionManager
	globalRegistry   *HandlerRegistry
	characterOwners  characterOwners
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
	publicChatSinkMu sync.RWMutex
	publicChatSink   func(ChatMessageBroadcast)
}

// NewWorldHandler creates a new WorldHandler.
func NewWorldHandler(sessionManager *session.SessionManager) *WorldHandler {
	registry := NewWorldOpCodeRegistry()
	wh := &WorldHandler{
		sessionManager: sessionManager,
		database:       db.GlobalWorldDB.DB,
		Economy:        economy.New(db.GlobalWorldDB.DB),
		Items:          itemuse.New(db.GlobalWorldDB.DB),
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
	wh.globalRegistry.HandleWorldPacket(ses, data)
}

// RemoveSession cleans up session data.
func (wh *WorldHandler) RemoveSession(sessionID int) {
	ses, removed := wh.sessionManager.RemoveSession(sessionID)
	if !removed {
		return
	}
	log.Printf("[WORLD] Removing session %d", sessionID)
	ses.DrainCommands(func() { wh.cleanupCharacterSession(ses) })
}

func (wh *WorldHandler) cleanupCharacterSession(ses *session.Session) {
	ses.IssuedCutscenes.Clear()
	if !ses.HasValidClient() {
		return
	}
	char := ses.Client.CharData()
	charID := int(char.ID)
	// A delayed disconnect may retire its local client, but must never evict
	// state belonging to a replacement connection.
	defer func() {
		ses.StopPlaytime()
		ses.Client.Shutdown()
		ses.Client = nil
		ses.CharacterName = ""
		ses.MapID = -1
		wh.characterOwners.release(int64(charID), ses)
	}()
	if !wh.characterOwners.owns(int64(charID), ses) {
		return
	}
	log.Printf("[WORLD] Flushing position for character %d (%s) from session %d", charID, char.Name, ses.SessionID)
	wh.PlayerMovement.FlushPlayerPosition(charID)
	wh.PlayerMovement.UnregisterPlayer(charID)
	wh.TrainerEncounter.ClearPlayer(int64(charID))
	wh.WildEncounter.ClearPlayer(int64(charID))
	wh.EventFlags.UnloadFlags(int64(charID))
	saveBattleOnDisconnect(int64(charID))
	wh.persistSessionPlaytime(ses, time.Now())

	// Notify other Phaser clients to remove this actor.
	phaserID := wh.ActorRegistry.GetPhaserID(ActorTypePlayer, charID)
	log.Printf("[WORLD] Despawning Phaser actor %d for character %s", phaserID, char.Name)
	wh.ActorManager.broadcastActorDespawn(phaserID, ses.MapID)
}

func (wh *WorldHandler) persistSessionPlaytime(ses *session.Session, now time.Time) {
	_, err := ses.PersistPlaytime(now, func(characterID int32, seconds uint32) error {
		return db_character.AddCharacterPlaytime(characterID, ses.AccountID, seconds)
	})
	if err != nil {
		log.Printf("[WORLD] Failed to persist playtime for session %d: %v", ses.SessionID, err)
	}
}

// Shutdown flushes active playtime before the database connection closes.
func (wh *WorldHandler) Shutdown() {
	now := time.Now()
	wh.sessionManager.ForEachSession(func(ses *session.Session) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ses.ExecuteCommand(ctx, func() { wh.persistSessionPlaytime(ses, now) })
	})
}

func (wh *WorldHandler) StartSessionTimeoutChecker() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		nextPlaytimeFlush := time.Now().Add(playtimeFlushInterval)

		for range ticker.C {
			now := time.Now()
			var timedOutSessions []int

			wh.sessionManager.ForEachSession(func(ses *session.Session) {
				lastHeartbeat := ses.LastHeartbeat()
				if !lastHeartbeat.IsZero() && now.Sub(lastHeartbeat) > 15*time.Second {
					log.Printf("[WORLD] Session %d timed out (last heartbeat: %v)", ses.SessionID, lastHeartbeat)
					timedOutSessions = append(timedOutSessions, ses.SessionID)
				}
			})

			for _, sessionID := range timedOutSessions {
				wh.RemoveSession(sessionID)
			}

			if !now.Before(nextPlaytimeFlush) {
				wh.sessionManager.ForEachSession(func(ses *session.Session) {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					_ = ses.ExecuteCommand(ctx, func() { wh.persistSessionPlaytime(ses, now) })
				})
				nextPlaytimeFlush = now.Add(playtimeFlushInterval)
			}
		}
	}()
}
