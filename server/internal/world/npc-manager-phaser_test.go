package world

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"capturequest/internal/api/opcodes"
	model "capturequest/internal/db/models"
	"capturequest/internal/session"
)

type recordedStreamMessage struct {
	sessionID int
	opcode    opcodes.OpCode
	payload   []byte
}

type recordingMessenger struct {
	streams []recordedStreamMessage
}

func (m *recordingMessenger) SendDatagram(sessionID int, data []byte) error {
	return nil
}

func (m *recordingMessenger) SendStream(sessionID int, data []byte) error {
	m.streams = append(m.streams, recordedStreamMessage{
		sessionID: sessionID,
		opcode:    opcodes.OpCode(binary.LittleEndian.Uint16(data[4:6])),
		payload:   append([]byte(nil), data[6:]...),
	})
	return nil
}

type testSessionClient struct {
	char *model.CharacterData
}

func (c *testSessionClient) CharData() *model.CharacterData  { return c.char }
func (c *testSessionClient) ID() int                         { return int(c.char.ID) }
func (c *testSessionClient) Name() string                    { return c.char.Name }
func (c *testSessionClient) Say(string)                      {}
func (c *testSessionClient) ShowNetworkStatsEnabled() bool   { return false }
func (c *testSessionClient) SetShowNetworkStatsEnabled(bool) {}
func (c *testSessionClient) AllowTrainerRebattles() bool     { return false }
func (c *testSessionClient) SetAllowTrainerRebattlesEnabled(bool) {
}
func (c *testSessionClient) Options() interface{}              { return nil }
func (c *testSessionClient) SaveOptions() error                { return nil }
func (c *testSessionClient) SendSystemMessage(string)          {}
func (c *testSessionClient) SendSpecialMessage(string, string) {}
func (c *testSessionClient) SendStateUpdate()                  {}
func (c *testSessionClient) Shutdown()                         {}

func TestShouldSendActorToSessionMapKeepsOverworldNPCsWarmIndoors(t *testing.T) {
	mgr := &PhaserActorManager{
		overworldMapIds: map[int]bool{1: true, 2: true},
	}

	tests := []struct {
		name         string
		actor        PhaserActor
		sessionMapID int
		want         bool
	}{
		{
			name:         "overworld NPC reaches interior session",
			actor:        PhaserActor{ID: 10, MapID: 1, ObjectType: "npc"},
			sessionMapID: 59,
			want:         true,
		},
		{
			name:         "overworld player stays scoped to overworld viewers",
			actor:        PhaserActor{ID: 11, MapID: 1, ObjectType: "player"},
			sessionMapID: 59,
			want:         false,
		},
		{
			name:         "same interior map still reaches session",
			actor:        PhaserActor{ID: 12, MapID: 59, ObjectType: "npc"},
			sessionMapID: 59,
			want:         true,
		},
		{
			name:         "different interior map stays hidden",
			actor:        PhaserActor{ID: 13, MapID: 60, ObjectType: "npc"},
			sessionMapID: 59,
			want:         false,
		},
		{
			name:         "legacy overworld viewers still receive legacy overworld actors",
			actor:        PhaserActor{ID: 14, MapID: 1, ObjectType: "player"},
			sessionMapID: 2,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.shouldSendActorToSessionMap(&tt.actor, tt.sessionMapID)
			if got != tt.want {
				t.Fatalf("shouldSendActorToSessionMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBroadcastActorDespawnExceptRemovesPlayerFromPreviousMapViewers(t *testing.T) {
	sessionManager := session.NewSessionManager()
	oldMapMessenger := &recordingMessenger{}
	originMessenger := &recordingMessenger{}
	newMapMessenger := &recordingMessenger{}

	oldMapViewer := sessionManager.CreateSession(oldMapMessenger, 1, "old-map", nil)
	oldMapViewer.Authenticated = true
	oldMapViewer.MapID = 40

	origin := sessionManager.CreateSession(originMessenger, 2, "origin", nil)
	origin.Authenticated = true
	origin.MapID = 63

	newMapViewer := sessionManager.CreateSession(newMapMessenger, 3, "new-map", nil)
	newMapViewer.Authenticated = true
	newMapViewer.MapID = 63

	mgr := &PhaserActorManager{
		wh:              &WorldHandler{sessionManager: sessionManager},
		overworldMapIds: map[int]bool{},
	}

	mgr.broadcastActorDespawnExcept(1007, 40, origin.SessionID)

	if got := len(oldMapMessenger.streams); got != 1 {
		t.Fatalf("old map despawn messages = %d, want 1", got)
	}
	if got := oldMapMessenger.streams[0].opcode; got != opcodes.PhaserActorDespawn {
		t.Fatalf("old map opcode = %d, want %d", got, opcodes.PhaserActorDespawn)
	}
	var payload map[string]int
	if err := json.Unmarshal(oldMapMessenger.streams[0].payload, &payload); err != nil {
		t.Fatalf("despawn payload JSON: %v", err)
	}
	if got := payload["id"]; got != 1007 {
		t.Fatalf("despawn actor id = %d, want 1007", got)
	}
	if got := len(originMessenger.streams); got != 0 {
		t.Fatalf("origin despawn messages = %d, want 0", got)
	}
	if got := len(newMapMessenger.streams); got != 0 {
		t.Fatalf("new map despawn messages = %d, want 0", got)
	}
}

func TestBroadcastPlayerVisibleMapChangeDespawnsOldMapAndUpdatesNewMap(t *testing.T) {
	sessionManager := session.NewSessionManager()
	oldMapMessenger := &recordingMessenger{}
	originMessenger := &recordingMessenger{}
	newMapMessenger := &recordingMessenger{}

	oldMapViewer := sessionManager.CreateSession(oldMapMessenger, 1, "old-map", nil)
	oldMapViewer.Authenticated = true
	oldMapViewer.MapID = 40

	origin := sessionManager.CreateSession(originMessenger, 2, "origin", nil)
	origin.Authenticated = true
	origin.MapID = 63
	origin.Client = &testSessionClient{char: &model.CharacterData{ID: 7, Name: "WarpingPlayer", MapID: 63}}

	newMapViewer := sessionManager.CreateSession(newMapMessenger, 3, "new-map", nil)
	newMapViewer.Authenticated = true
	newMapViewer.MapID = 63

	wh := &WorldHandler{sessionManager: sessionManager}
	mgr := &PhaserActorManager{
		wh:              wh,
		overworldMapIds: map[int]bool{},
	}
	wh.ActorManager = mgr

	x, y := 2, 7
	sprite := "SPRITE_BLUE"
	name := "WarpingPlayer"
	actor := &PhaserActor{
		ID:         1007,
		InternalID: 7,
		X:          &x,
		Y:          &y,
		MapID:      63,
		ObjectType: "player",
		SpriteName: &sprite,
		Name:       &name,
	}

	broadcastPlayerActorVisibleMapChange(origin, wh, 40, actor)

	if got := len(oldMapMessenger.streams); got != 1 {
		t.Fatalf("old map messages = %d, want 1 despawn", got)
	}
	if got := oldMapMessenger.streams[0].opcode; got != opcodes.PhaserActorDespawn {
		t.Fatalf("old map opcode = %d, want %d", got, opcodes.PhaserActorDespawn)
	}
	if got := len(newMapMessenger.streams); got != 1 {
		t.Fatalf("new map messages = %d, want 1 position update", got)
	}
	if got := newMapMessenger.streams[0].opcode; got != opcodes.PhaserActorPositionUpdate {
		t.Fatalf("new map opcode = %d, want %d", got, opcodes.PhaserActorPositionUpdate)
	}
	if got := len(originMessenger.streams); got != 0 {
		t.Fatalf("origin messages = %d, want no multiplayer echo", got)
	}
}

func TestApplyRuntimeActorStateOverlaysMovingNPCFields(t *testing.T) {
	x, y := 14, 16
	dir := "LEFT"
	moveType := "WALK"
	mgr := &PhaserActorManager{
		walkingActors: map[int]*PhaserActor{
			42: {
				ID:              42,
				X:               &x,
				Y:               &y,
				ActionDirection: &dir,
				MovementType:    &moveType,
				MoveSpeed:       300,
			},
		},
	}

	dbX, dbY := 12, 16
	text := "MTMOON_TRAINER_TEXT"
	actor := PhaserActor{
		ID:        42,
		X:         &dbX,
		Y:         &dbY,
		MapID:     1,
		Text:      &text,
		MoveSpeed: 0,
	}

	mgr.applyRuntimeActorState(&actor)

	if actor.X == nil || actor.Y == nil || *actor.X != 14 || *actor.Y != 16 {
		t.Fatalf("runtime position = (%v,%v), want (14,16)", actor.X, actor.Y)
	}
	if actor.ActionDirection == nil || *actor.ActionDirection != "LEFT" {
		t.Fatalf("runtime direction = %v, want LEFT", actor.ActionDirection)
	}
	if actor.MovementType == nil || *actor.MovementType != "WALK" {
		t.Fatalf("runtime movement type = %v, want WALK", actor.MovementType)
	}
	if actor.MoveSpeed != 300 {
		t.Fatalf("runtime move speed = %d, want 300", actor.MoveSpeed)
	}
	if actor.Text == nil || *actor.Text != text {
		t.Fatalf("static actor text should be preserved, got %v", actor.Text)
	}
}

func TestCollisionMapWithBlockedPositionsStopsPathThroughNPC(t *testing.T) {
	collisionMap := map[string]int{
		"0,0": collisionLand,
		"1,0": collisionLand,
		"2,0": collisionLand,
	}

	if path := findPathOnCollisionMap(collisionMap, nil, 0, 0, 2, 0); len(path) != 2 {
		t.Fatalf("unblocked path length = %d, want 2", len(path))
	}

	blocked := collisionMapWithBlockedPositions(collisionMap, []tilePosition{{X: 1, Y: 0}}, 0, 0)
	if path := findPathOnCollisionMap(blocked, nil, 0, 0, 2, 0); len(path) != 0 {
		t.Fatalf("blocked path = %#v, want no path", path)
	}

	startBlocked := collisionMapWithBlockedPositions(collisionMap, []tilePosition{{X: 0, Y: 0}}, 0, 0)
	if path := findPathOnCollisionMap(startBlocked, nil, 0, 0, 2, 0); len(path) != 2 {
		t.Fatalf("path from occupied start length = %d, want 2", len(path))
	}
}
