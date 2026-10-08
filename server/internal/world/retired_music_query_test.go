package world

import (
	"capturequest/internal/api/opcodes"
	"capturequest/internal/db"
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"encoding/json"
	"testing"
)

func TestRetiredMapMusicQueryRejectsWithoutDatabaseDependency(t *testing.T) {
	old := db.GlobalWorldDB
	db.GlobalWorldDB = nil
	t.Cleanup(func() { db.GlobalWorldDB = old })
	messages := &recordingMessenger{}
	ses := &session.Session{Authenticated: true, Messenger: messages}
	HandlePhaserMapMusicRequest(ses, []byte(`{"mapId":0}`), &WorldHandler{})
	if len(messages.streams) != 1 || messages.streams[0].opcode != opcodes.PhaserMapMusicResponse {
		t.Fatal("missing reserved opcode rejection")
	}
	var reply protocol.ErrorResponse
	if err := json.Unmarshal(messages.streams[0].payload, &reply); err != nil || reply.Success || reply.Error == "" {
		t.Fatalf("retired query accepted: %+v %v", reply, err)
	}
}
