package world

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"capturequest/internal/content"
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
	Content          *content.Service `json:"-"`
	Items            *itemuse.Service `json:"-"`
	Economy          *economy.Service `json:"-"`
	sessionManager   *session.SessionManager
	globalRegistry   *HandlerRegistry
	characterOwners  characterOwners
	timeoutWorker    periodicWorker
	shutdownOnce     sync.Once
	shutdownDone     chan struct{}
	shutdownErr      error
	cleanupDraining  bool
	cleanupErrors    error
	cleanupMu        sync.Mutex
	cleanupWG        sync.WaitGroup
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
func NewWorldHandler(ctx context.Context, sessionManager *session.SessionManager) (*WorldHandler, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	registry := NewWorldOpCodeRegistry()
	wh := &WorldHandler{
		sessionManager: sessionManager,
		database:       db.GlobalWorldDB.DB,
		Content:        content.New(db.GlobalWorldDB.DB),
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
	if err := wh.ActorManager.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload actors: %w", err)
	}
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)
	wh.TrainerEncounter = NewTrainerEncounterManager(wh)
	if err := wh.TrainerEncounter.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload TrainerEncounter: %w", err)
	}
	wh.WildEncounter = NewWildEncounterManager(wh, wh.database)
	if err := wh.WildEncounter.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload WildEncounter: %w", err)
	}
	wh.EventFlags = NewEventFlagManager(db.GlobalWorldDB.DB)
	wh.CoordTriggers = NewCoordinateTriggerManager(db.GlobalWorldDB.DB)
	if err := wh.CoordTriggers.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload CoordTriggers: %w", err)
	}
	wh.MapScripts = NewMapScriptManager(db.GlobalWorldDB.DB)
	if err := wh.MapScripts.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload MapScripts: %w", err)
	}
	wh.Cutscenes = NewCutsceneManager(db.GlobalWorldDB.DB)
	if err := wh.Cutscenes.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload cutscenes: %w", err)
	}
	wh.SpinTiles = NewSpinTileManager(db.GlobalWorldDB.DB)
	if err := wh.SpinTiles.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload SpinTiles: %w", err)
	}
	wh.WarpTiles = NewWarpTileManager(db.GlobalWorldDB.DB)
	if err := wh.WarpTiles.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload WarpTiles: %w", err)
	}
	wh.phaserWarps = newPhaserWarpManager(db.GlobalWorldDB.DB)
	wh.phaserWarps.setActorManager(wh.ActorManager)
	if err := wh.phaserWarps.load(ctx); err != nil {
		return nil, fmt.Errorf("preload phaserWarps: %w", err)
	}
	wh.Safari = NewSafariZoneManager(wh.database)
	if err := wh.Safari.Load(ctx); err != nil {
		return nil, fmt.Errorf("preload Safari: %w", err)
	}
	LoadDisallowedWords()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	wh.ActorManager.Start()
	wh.PlayerMovement.Start()
	wh.StartSessionTimeoutChecker()
	return wh, nil
}

// HandlePacket processes incoming datagrams.
// All handlers are now at the world level - no zone routing needed.
func (wh *WorldHandler) HandlePacket(ses *session.Session, data []byte) {
	wh.globalRegistry.HandleWorldPacket(ses, data)
}

// RemoveSession cleans up session data.
func (wh *WorldHandler) RemoveSession(sessionID int) {
	// Claims and registration are serialized so shutdown cannot miss cleanup
	// already claimed by a transport callback before taking its own snapshot.
	wh.cleanupMu.Lock()
	ses, removed := wh.sessionManager.RemoveSession(sessionID)
	if removed {
		wh.cleanupWG.Add(1)
	}
	wh.cleanupMu.Unlock()
	if !removed {
		return
	}
	defer wh.cleanupWG.Done()
	log.Printf("[WORLD] Removing session %d", sessionID)
	ses.DrainCommands(func() {
		if err := wh.cleanupCharacterSession(context.Background(), ses); err != nil {
			wh.cleanupMu.Lock()
			if wh.cleanupDraining {
				wh.cleanupErrors = errors.Join(wh.cleanupErrors, fmt.Errorf("session %d: %w", ses.SessionID, err))
			}
			wh.cleanupMu.Unlock()
			log.Printf("[WORLD] Session %d cleanup persistence failed: %v", ses.SessionID, err)
		}
	})
}

func (wh *WorldHandler) cleanupCharacterSession(ctx context.Context, ses *session.Session) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ses.IssuedCutscenes.Clear()
	ses.GameCorner.Clear()
	if !ses.HasValidClient() {
		return nil
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
		return nil
	}
	log.Printf("[WORLD] Flushing position for character %d (%s) from session %d", charID, char.Name, ses.SessionID)
	positionErr := wh.PlayerMovement.FlushPlayerPosition(ctx, charID)
	wh.PlayerMovement.UnregisterPlayer(charID)
	wh.TrainerEncounter.ClearPlayer(int64(charID))
	wh.EventFlags.UnloadFlags(int64(charID))
	saveBattleOnDisconnect(int64(charID))
	playtimeErr := wh.persistSessionPlaytime(ctx, ses, time.Now())

	// Notify other Phaser clients to remove this actor.
	phaserID := wh.ActorRegistry.GetPhaserID(ActorTypePlayer, charID)
	log.Printf("[WORLD] Despawning Phaser actor %d for character %s", phaserID, char.Name)
	wh.ActorManager.broadcastActorDespawn(phaserID, ses.MapID)
	if positionErr != nil {
		positionErr = fmt.Errorf("final position: %w", positionErr)
	}
	if playtimeErr != nil {
		playtimeErr = fmt.Errorf("final playtime: %w", playtimeErr)
	}
	return errors.Join(positionErr, playtimeErr)
}

func (wh *WorldHandler) persistSessionPlaytime(ctx context.Context, ses *session.Session, now time.Time) error {
	_, err := ses.PersistPlaytime(now, func(characterID int32, seconds uint32) error {
		return db_character.AddCharacterPlaytime(ctx, wh.database, characterID, ses.AccountID, seconds)
	})
	return err
}

// Shutdown flushes active playtime before the database connection closes.
func (wh *WorldHandler) Shutdown() {
	if err := wh.ShutdownContext(context.Background()); err != nil {
		log.Printf("[WORLD] Shutdown: %v", err)
	}
}

// ShutdownContext bounds the caller wait, not the lifetime of owned work.
// A timeout leaves one drain running; callers can join it again before closing storage.
func (wh *WorldHandler) ShutdownContext(ctx context.Context) error {
	wh.shutdownOnce.Do(func() {
		wh.shutdownDone = make(chan struct{})
		go func() {
			wh.shutdown()
			close(wh.shutdownDone)
		}()
	})
	select {
	case <-wh.shutdownDone:
		return wh.shutdownErr
	case <-ctx.Done():
		return fmt.Errorf("world drain unfinished: %w", ctx.Err())
	}
}

func (wh *WorldHandler) shutdown() {
	wh.cleanupMu.Lock()
	wh.cleanupDraining = true
	wh.cleanupMu.Unlock()
	wh.sessionManager.Seal()
	wh.sessionManager.ForEachSession(func(ses *session.Session) { ses.Close() })
	wh.timeoutWorker.stop()
	if wh.PlayerMovement != nil {
		wh.PlayerMovement.Stop()
	}
	if wh.ActorManager != nil {
		wh.ActorManager.Stop()
	}
	wh.sessionManager.ForEachSession(func(ses *session.Session) { wh.RemoveSession(ses.SessionID) })
	// The sealed manager is now empty. No later removal can register more work.
	wh.cleanupMu.Lock()
	wh.cleanupMu.Unlock()
	wh.cleanupWG.Wait()
	wh.cleanupMu.Lock()
	wh.shutdownErr = wh.cleanupErrors
	wh.cleanupMu.Unlock()
}

func (wh *WorldHandler) StartSessionTimeoutChecker() {
	nextPlaytimeFlush := time.Now().Add(playtimeFlushInterval)
	wh.timeoutWorker.start(5*time.Second, nil, func() {
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
				_ = ses.ExecuteCommand(ctx, func() {
					if err := wh.persistSessionPlaytime(ses.CommandContext(), ses, now); err != nil && !ses.IsClosed() {
						log.Printf("[WORLD] Playtime for session %d: %v", ses.SessionID, err)
					}
				})
			})
			nextPlaytimeFlush = now.Add(playtimeFlushInterval)
		}
	})
}
