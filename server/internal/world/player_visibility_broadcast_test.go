package world

import (
	"database/sql"
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	model "capturequest/internal/db/models"
	"capturequest/internal/session"

	_ "modernc.org/sqlite"
)

func TestClientReportedMapChangeDespawnsOldMapAndUpdatesNewMap(t *testing.T) {
	setupPlayerVisibilityTestDB(t)

	wh, origin, oldMapMessenger, newMapMessenger, originMessenger := setupPlayerVisibilityWorld(t, 40, 63)
	wh.PlayerMovement.RegisterPlayer(origin, 7, 4, 4, 40, "DOWN")

	payload, err := json.Marshal(PhaserPlayerPositionUpdateRequest{
		X:         2,
		Y:         7,
		MapID:     63,
		Direction: "UP",
	})
	if err != nil {
		t.Fatalf("marshal position update: %v", err)
	}

	HandlePhaserPlayerPositionUpdate(origin, payload, wh)

	assertSinglePlayerDespawn(t, oldMapMessenger, wh.ActorRegistry.GetPhaserID(ActorTypePlayer, 7))
	assertSinglePlayerUpdate(t, newMapMessenger, wh.ActorRegistry.GetPhaserID(ActorTypePlayer, 7), 63, 2, 7)
	if got := len(originMessenger.streams); got != 0 {
		t.Fatalf("origin messages = %d, want no multiplayer echo", got)
	}
}

func TestServerTeleportedPlayerDespawnsOldMapAndUpdatesNewMap(t *testing.T) {
	setupPlayerVisibilityTestDB(t)

	wh, origin, oldMapMessenger, newMapMessenger, originMessenger := setupPlayerVisibilityWorld(t, 40, 63)
	wh.PlayerMovement.RegisterPlayer(origin, 7, 4, 4, 40, "DOWN")

	gotMapID := setServerTeleportedPlayerPosition(origin, wh, 63, 2, 7, "UP")
	if gotMapID != 63 {
		t.Fatalf("setServerTeleportedPlayerPosition mapID = %d, want 63", gotMapID)
	}

	assertSinglePlayerDespawn(t, oldMapMessenger, wh.ActorRegistry.GetPhaserID(ActorTypePlayer, 7))
	assertSinglePlayerUpdate(t, newMapMessenger, wh.ActorRegistry.GetPhaserID(ActorTypePlayer, 7), 63, 2, 7)
	if got := len(originMessenger.streams); got != 0 {
		t.Fatalf("origin messages = %d, want no multiplayer echo", got)
	}
}

func setupPlayerVisibilityWorld(t *testing.T, oldMapID, newMapID int) (*WorldHandler, *session.Session, *recordingMessenger, *recordingMessenger, *recordingMessenger) {
	t.Helper()

	sessionManager := session.NewSessionManager()
	oldMapMessenger := &recordingMessenger{}
	originMessenger := &recordingMessenger{}
	newMapMessenger := &recordingMessenger{}

	oldMapViewer := sessionManager.CreateSession(oldMapMessenger, 1, "old-map", nil)
	oldMapViewer.Authenticated = true
	oldMapViewer.MapID = oldMapID

	origin := sessionManager.CreateSession(originMessenger, 2, "origin", nil)
	origin.Authenticated = true
	origin.MapID = oldMapID
	origin.X = 4
	origin.Y = 4
	origin.Client = &testSessionClient{char: &model.CharacterData{
		ID:    7,
		Name:  "WarpingPlayer",
		MapID: uint32(oldMapID),
		X:     4,
		Y:     4,
	}}

	newMapViewer := sessionManager.CreateSession(newMapMessenger, 3, "new-map", nil)
	newMapViewer.Authenticated = true
	newMapViewer.MapID = newMapID

	wh := &WorldHandler{
		sessionManager: sessionManager,
		ActorRegistry:  NewActorRegistry(),
		CutTiles:       NewCutTileManager(),
	}
	wh.ActorManager = NewPhaserActorManager(wh)
	wh.PlayerMovement = NewPlayerMovementManager(wh, wh.ActorManager)

	return wh, origin, oldMapMessenger, newMapMessenger, originMessenger
}

func setupPlayerVisibilityTestDB(t *testing.T) {
	t.Helper()

	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		raw.Close()
	})

	if _, err := raw.Exec(`
		CREATE TABLE character_data (
			id INTEGER PRIMARY KEY,
			map_id INTEGER NOT NULL,
			x REAL NOT NULL,
			y REAL NOT NULL,
			z REAL NOT NULL DEFAULT 0,
			heading REAL NOT NULL DEFAULT 0
		);
		CREATE TABLE character_daycare (
			character_id INTEGER PRIMARY KEY,
			pokemon_row_id INTEGER NOT NULL,
			start_level INTEGER NOT NULL
		);
		INSERT INTO character_data (id, map_id, x, y, z, heading)
		VALUES (7, 40, 4, 4, 0, 0);
	`); err != nil {
		t.Fatalf("create test db schema: %v", err)
	}

	previous := db.GlobalWorldDB
	db.GlobalWorldDB = &db.WorldDB{DB: raw}
	t.Cleanup(func() {
		db.GlobalWorldDB = previous
	})
}

func assertSinglePlayerDespawn(t *testing.T, messenger *recordingMessenger, actorID int) {
	t.Helper()

	if got := len(messenger.streams); got != 1 {
		t.Fatalf("despawn messages = %d, want 1", got)
	}
	msg := messenger.streams[0]
	if got := msg.opcode; got != opcodes.PhaserActorDespawn {
		t.Fatalf("despawn opcode = %d, want %d", got, opcodes.PhaserActorDespawn)
	}
	var payload map[string]int
	if err := json.Unmarshal(msg.payload, &payload); err != nil {
		t.Fatalf("despawn payload JSON: %v", err)
	}
	if got := payload["id"]; got != actorID {
		t.Fatalf("despawn actor id = %d, want %d", got, actorID)
	}
}

func assertSinglePlayerUpdate(t *testing.T, messenger *recordingMessenger, actorID, mapID, x, y int) {
	t.Helper()

	if got := len(messenger.streams); got != 1 {
		t.Fatalf("update messages = %d, want 1", got)
	}
	msg := messenger.streams[0]
	if got := msg.opcode; got != opcodes.PhaserActorPositionUpdate {
		t.Fatalf("update opcode = %d, want %d", got, opcodes.PhaserActorPositionUpdate)
	}
	var actor PhaserActor
	if err := json.Unmarshal(msg.payload, &actor); err != nil {
		t.Fatalf("update payload JSON: %v", err)
	}
	if actor.ID != actorID || actor.MapID != mapID || actor.X == nil || actor.Y == nil || *actor.X != x || *actor.Y != y {
		t.Fatalf("actor update = id:%d map:%d pos:(%v,%v), want id:%d map:%d pos:(%d,%d)",
			actor.ID, actor.MapID, actor.X, actor.Y, actorID, mapID, x, y)
	}
}
