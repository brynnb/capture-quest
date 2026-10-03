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

// Even valid catalog destinations cannot be established by the retired setter.
func TestRetiredPositionReportsPreservePositionAndVisibility(t *testing.T) {
	setupPlayerVisibilityTestDB(t)
	wh, origin, oldMapMessenger, newMapMessenger, originMessenger := setupPlayerVisibilityWorld(t, 40, 63)
	wh.PlayerMovement.RegisterPlayer(origin, 7, 4, 4, 40, "DOWN")
	registry := NewWorldOpCodeRegistry()
	registry.WH = wh
	for _, payload := range []string{
		`{"x":2,"y":7,"mapId":63,"direction":"UP"}`,
		`{"x":5,"y":4,"mapId":40,"direction":"RIGHT"}`,
		`{"x":4,"y":4,"mapId":40,"direction":"UP"}`,
		`{`,
	} {
		registry.HandleWorldPacket(origin, clientPacket(opcodes.PhaserPlayerPositionUpdate, payload))
	}
	for _, messenger := range []*recordingMessenger{oldMapMessenger, newMapMessenger, originMessenger} {
		if len(messenger.streams) != 0 {
			t.Fatalf("retired setter published %+v", messenger.streams)
		}
	}
	x, y, mapID, ok := wh.PlayerMovement.GetPosition(7)
	direction, _ := wh.PlayerMovement.GetDirection(7)
	if !ok || x != 4 || y != 4 || mapID != 40 || direction != "DOWN" || wh.PlayerMovement.players[7].positionDirty || wh.PlayerMovement.players[7].pendingStep != nil || len(wh.PlayerMovement.players[7].Path) != 0 {
		t.Fatal("retired setter changed owned movement")
	}
	char := origin.Client.CharData()
	if origin.MapID != 40 || origin.X != 4 || origin.Y != 4 || char.MapID != 40 || char.X != 4 || char.Y != 4 {
		t.Fatal("retired setter changed session location")
	}
	if err := wh.database.QueryRow(`SELECT x,y,map_id FROM character_data WHERE id=7`).Scan(&x, &y, &mapID); err != nil || x != 4 || y != 4 || mapID != 40 {
		t.Fatalf("retired setter changed durable location: %d %d %d %v", x, y, mapID, err)
	}
}

func TestServerTeleportedPlayerDespawnsOldMapAndUpdatesNewMap(t *testing.T) {
	setupPlayerVisibilityTestDB(t)

	wh, origin, oldMapMessenger, newMapMessenger, originMessenger := setupPlayerVisibilityWorld(t, 40, 63)
	wh.PlayerMovement.RegisterPlayer(origin, 7, 4, 4, 40, "DOWN")

	gotMapID, err := setServerTeleportedPlayerPosition(origin, wh, 63, 2, 7, "UP")
	if err != nil || gotMapID != 63 {
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

	sessionManager.ForEachSession(func(s *session.Session) { s.PublishPresence() })
	wh := &WorldHandler{
		sessionManager: sessionManager,
		database:       db.GlobalWorldDB.DB,
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
		CREATE TABLE phaser_maps(id INTEGER PRIMARY KEY);
		CREATE TABLE phaser_tiles(map_id INTEGER,x INTEGER,y INTEGER,collision_type INTEGER DEFAULT 0,raw_foot_tile_id INTEGER,is_tile_erased INTEGER DEFAULT 0);
		INSERT INTO phaser_maps(id) VALUES(63);
		INSERT INTO phaser_tiles(map_id,x,y) VALUES(63,2,7);
		CREATE TABLE character_safari_state(character_id INTEGER PRIMARY KEY,state_json TEXT NOT NULL,updated_at TEXT);
        CREATE TABLE character_cutscene_plans (character_id INTEGER,completion_token TEXT,resolution TEXT,map_id INTEGER,x INTEGER,y INTEGER);
 CREATE TABLE character_trainer_encounters(character_id INTEGER PRIMARY KEY,resolution TEXT,map_id INTEGER,x INTEGER,y INTEGER,updated_at TEXT);
		CREATE TABLE character_event_flags(character_id INTEGER,flag_name TEXT,PRIMARY KEY(character_id,flag_name));
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
