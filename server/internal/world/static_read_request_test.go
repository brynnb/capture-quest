package world

import (
	"capturequest/internal/protocol"
	"capturequest/internal/session"
	"capturequest/internal/testdb"
	"context"
	"encoding/json"
	"testing"
)

func TestStaticEndpointsEchoCorrelationAndRejectLegacyReads(t *testing.T) {
	database := testdb.Postgres(t)
	wh := &WorldHandler{database: database}
	for _, handler := range []func(*session.Session, []byte, *WorldHandler) bool{HandleStaticDataRequest, HandleCharCreateDataRequest} {
		messages := &recordingMessenger{}
		ses := &session.Session{Authenticated: true, Messenger: messages}
		if err := ses.ExecuteCommand(context.Background(), func() { handler(ses, []byte(`{"requestId":"catalog:1"}`), wh) }); err != nil {
			t.Fatal(err)
		}
		var response StaticDataResponse
		if len(messages.streams) != 1 {
			t.Fatal("missing static reply")
		}
		if err := json.Unmarshal(messages.streams[0].payload, &response); err != nil || !response.Success || response.RequestID != "catalog:1" || response.Maps == nil || response.Classes == nil {
			t.Fatalf("uncorrelated/incomplete reply: %+v %v", response, err)
		}
		messages.streams = nil
		handler(ses, []byte(`{}`), wh)
		var failed protocol.PlayerStepError
		if len(messages.streams) != 1 {
			t.Fatal("missing legacy rejection")
		}
		if err := json.Unmarshal(messages.streams[0].payload, &failed); err != nil || failed.Success || failed.Error == "" {
			t.Fatalf("legacy read accepted: %+v %v", failed, err)
		}
	}
	testdb.Exec(t, database, `ALTER TABLE phaser_maps RENAME TO unavailable_maps`)
	messages := &recordingMessenger{}
	ses := &session.Session{Authenticated: true, Messenger: messages}
	HandleStaticDataRequest(ses, []byte(`{"requestId":"catalog:failure"}`), wh)
	var failed protocol.PlayerStepError
	if len(messages.streams) != 1 {
		t.Fatal("missing read failure")
	}
	if err := json.Unmarshal(messages.streams[0].payload, &failed); err != nil || failed.Success || failed.RequestID != "catalog:failure" || failed.Error == "" {
		t.Fatalf("failure lost correlation: %+v %v", failed, err)
	}
}
